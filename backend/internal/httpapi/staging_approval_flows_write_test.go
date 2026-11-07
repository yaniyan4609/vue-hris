package httpapi

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
)

func TestStagingApprovalFlowLifecycle(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var state auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &state)
	var positionID, roleID string
	if err := db.QueryRow(`SELECT id FROM positions WHERE tenant_id=? AND deleted_at IS NULL LIMIT 1`, state.SelectedTenantID).Scan(&positionID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM roles WHERE tenant_id=? LIMIT 1`, state.SelectedTenantID).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	kind := ""
	for _, candidate := range []string{"profile_change", "shift_swap", "attendance_correction", "promotion"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM approval_flows WHERE tenant_id=? AND request_type=? AND is_default=TRUE`, state.SelectedTenantID, candidate).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			kind = candidate
			break
		}
	}
	if kind == "" {
		t.Skip("every candidate request type already has a default; set-default would alter existing staging data")
	}
	base := server.URL + "/api/v1/admin/approval-flows"
	var created struct {
		Data approvalFlow `json:"data"`
	}
	doJSON(t, client, "POST", base, map[string]any{"request_type": kind, "name": "QA Approval Flow", "valid_from": "2000-01-01"}, 201, &created)
	defer func() {
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='approval_flows' AND entity_id=?`, state.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM approval_flow_steps WHERE tenant_id=? AND flow_id=?`, state.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup steps: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM approval_flows WHERE tenant_id=? AND id=?`, state.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup flow: %v", err)
		}
	}()
	if created.Data.Version != 1 || !created.Data.IsActive {
		t.Fatalf("created: %#v", created.Data)
	}
	steps := []map[string]any{{"level_no": 1, "approver_kind": "reports_to", "sla_hours": 24}, {"level_no": 2, "approver_kind": "role", "approver_ref": roleID}, {"level_no": 3, "approver_kind": "position", "approver_ref": positionID}, {"level_no": 4, "approver_kind": "membership", "approver_ref": state.User.ID}}
	var replaced struct {
		Data struct {
			Version uint64             `json:"version"`
			Steps   []approvalFlowStep `json:"steps"`
		} `json:"data"`
	}
	doJSON(t, client, "PUT", base+"/"+created.Data.ID+"/steps", map[string]any{"version": 1, "steps": steps}, 200, &replaced)
	if replaced.Data.Version != 2 || len(replaced.Data.Steps) != 4 {
		t.Fatalf("steps: %#v", replaced.Data)
	}
	doJSON(t, client, "PUT", base+"/"+created.Data.ID+"/steps", map[string]any{"version": 1, "steps": steps}, 409, nil)
	var detail struct {
		Data approvalFlow `json:"data"`
	}
	doJSON(t, client, "GET", base+"/"+created.Data.ID, nil, 200, &detail)
	if len(detail.Data.Steps) != 4 || detail.Data.Steps[0].ApproverKind != "reports_to" || detail.Data.Steps[2].ApproverKind != "position" {
		t.Fatalf("detail steps: %#v", detail.Data.Steps)
	}
	doJSON(t, client, "PATCH", base+"/"+created.Data.ID, map[string]any{"version": 2, "name": "QA Updated Flow"}, 200, &detail)
	if detail.Data.Version != 3 || detail.Data.Name != "QA Updated Flow" {
		t.Fatalf("patch: %#v", detail.Data)
	}
	doJSON(t, client, "POST", base+"/"+created.Data.ID+"/set-default", map[string]any{}, 200, &detail)
	if !detail.Data.IsDefault || detail.Data.Version != 4 {
		t.Fatalf("default: %#v", detail.Data)
	}
	subject, decision, err := (access.Store{DB: db}).Decide(ctx, state.SelectedTenantID, state.User.ID, "Asia/Jakarta", "karyawan.lihat")
	if err != nil {
		t.Fatal(err)
	}
	filter, filterArgs := access.EmployeeWhere(subject, decision)
	params := append([]any{state.SelectedTenantID}, filterArgs...)
	var employeeID string
	if err := db.QueryRow(`SELECT e.id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND `+filter+` LIMIT 1`, params...).Scan(&employeeID); err != nil {
		if err := db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? LIMIT 1`, state.SelectedTenantID).Scan(&employeeID); err != nil {
			t.Fatal(err)
		}
		doJSON(t, client, "POST", base+"/simulate", map[string]any{"request_type": kind, "employee_id": employeeID}, 404, nil)
	} else {
		var simulation struct {
			Data struct {
				FlowID string           `json:"flow_id"`
				Levels []map[string]any `json:"levels"`
			} `json:"data"`
		}
		doJSON(t, client, "POST", base+"/simulate", map[string]any{"request_type": kind, "employee_id": employeeID}, 200, &simulation)
		if simulation.Data.FlowID != created.Data.ID || len(simulation.Data.Levels) != 4 {
			t.Fatalf("simulation: %#v", simulation.Data)
		}
	}
	doJSON(t, client, "DELETE", base+"/"+created.Data.ID, nil, 204, nil)
	doJSON(t, client, "GET", base+"/"+created.Data.ID, nil, 200, &detail)
	if detail.Data.IsActive || detail.Data.IsDefault {
		t.Fatalf("deactivation: %#v", detail.Data)
	}
	doJSON(t, client, "PUT", base+"/"+created.Data.ID+"/steps", map[string]any{"version": 5, "steps": steps}, 409, nil)
}
