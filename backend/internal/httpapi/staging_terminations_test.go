package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
)

func TestStagingTerminationWorkflow(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	newClient := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, Timeout: 10 * time.Second}
	}
	admin, qa := newClient(), newClient()
	var state auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &state)
	tenantID := state.SelectedTenantID
	qaUser, _ := ids.New()
	qaEmployee, _ := ids.New()
	qaAssignment, _ := ids.New()
	qaAdminRole, _ := ids.New()
	qaEssRole, _ := ids.New()
	flowID, _ := ids.New()
	stepID, _ := ids.New()
	var adminRoleID, essRoleID, seedEmployee string
	if err = db.QueryRow(`SELECT id FROM roles WHERE tenant_id=? AND code='ADMIN_TENANT'`, tenantID).Scan(&adminRoleID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT id FROM roles WHERE tenant_id=? AND code='KARYAWAN'`, tenantID).Scan(&essRoleID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? AND user_id IS NOT NULL LIMIT 1`, tenantID).Scan(&seedEmployee); err != nil {
		t.Fatal(err)
	}
	termIDs := []string{}
	requestIDs := []string{}
	t.Cleanup(func() {
		for _, id := range termIDs {
			var request sql.NullString
			_ = db.QueryRow(`SELECT request_id FROM terminations WHERE tenant_id=? AND id=?`, tenantID, id).Scan(&request)
			if request.Valid {
				requestIDs = append(requestIDs, request.String)
			}
		}
		for _, id := range termIDs {
			for _, q := range []string{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='terminations' AND entity_id=?`, `DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='termination' AND aggregate_id=?`, `DELETE FROM termination_checklist WHERE tenant_id=? AND termination_id=?`} {
				if _, e := db.Exec(q, tenantID, id); e != nil {
					t.Error(e)
				}
			}
			if _, e := db.Exec(`DELETE FROM terminations WHERE tenant_id=? AND id=?`, tenantID, id); e != nil {
				t.Error(e)
			}
		}
		seenRequests := map[string]bool{}
		for _, id := range requestIDs {
			if id == "" || seenRequests[id] {
				continue
			}
			seenRequests[id] = true
			var instanceID string
			_ = db.QueryRow(`SELECT id FROM approval_instances WHERE tenant_id=? AND request_id=?`, tenantID, id).Scan(&instanceID)
			if instanceID != "" {
				for _, q := range []string{`DELETE FROM approval_actions WHERE tenant_id=? AND instance_id=?`, `DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id=?`, `DELETE FROM approval_instances WHERE tenant_id=? AND id=?`} {
					if _, e := db.Exec(q, tenantID, instanceID); e != nil {
						t.Error(e)
					}
				}
			}
			for _, q := range []string{`DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='request' AND aggregate_id=?`, `DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='requests' AND entity_id=?`, `DELETE FROM request_revisions WHERE tenant_id=? AND request_id=?`} {
				if _, e := db.Exec(q, tenantID, id); e != nil {
					t.Error(e)
				}
			}
			if _, e := db.Exec(`DELETE FROM requests WHERE tenant_id=? AND id=?`, tenantID, id); e != nil {
				t.Error(e)
			}
		}
		for _, entry := range []struct {
			q    string
			args []any
		}{
			{`UPDATE tenant_memberships SET employee_id=NULL WHERE tenant_id=? AND user_id=?`, []any{tenantID, qaUser}},
			{`DELETE FROM employee_assignments WHERE tenant_id=? AND id=?`, []any{tenantID, qaAssignment}},
			{`DELETE FROM employees WHERE tenant_id=? AND id=?`, []any{tenantID, qaEmployee}},
			{`DELETE FROM approval_flow_steps WHERE tenant_id=? AND id=?`, []any{tenantID, stepID}},
			{`DELETE FROM approval_flows WHERE tenant_id=? AND id=?`, []any{tenantID, flowID}},
			{`DELETE FROM user_roles WHERE tenant_id=? AND user_id=?`, []any{tenantID, qaUser}},
			{`DELETE FROM sessions WHERE user_id=?`, []any{qaUser}},
			{`DELETE FROM login_events WHERE user_id=?`, []any{qaUser}},
			{`DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, []any{tenantID, qaUser}},
			{`DELETE FROM users WHERE id=?`, []any{qaUser}},
		} {
			if _, e := db.Exec(entry.q, entry.args...); e != nil {
				t.Error(e)
			}
		}
	})
	email := "qa-termination-" + qaUser + "@example.invalid"
	password := "Qa-Termination-Password-29"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO users (id,email,email_normalized,display_name,password_hash) VALUES (?,?,?,'QA Termination',?)`, qaUser, email, email, hash); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO tenant_memberships (tenant_id,user_id,status) VALUES (?,?,'active')`, tenantID, qaUser); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO user_roles (tenant_id,id,user_id,role_id,valid_from) VALUES (?,?,?,?,?),(?,?,?,?,?)`, tenantID, qaAdminRole, qaUser, adminRoleID, "2000-01-01", tenantID, qaEssRole, qaUser, essRoleID, "2000-01-01"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO employees (tenant_id,id,user_id,employee_number,display_name,status,base_office_id,position_id) SELECT ?,?,?,?,'QA Termination Employee','active',base_office_id,position_id FROM employees WHERE tenant_id=? AND id=?`, tenantID, qaEmployee, qaUser, "QA-"+qaEmployee, tenantID, seedEmployee); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE tenant_memberships SET employee_id=? WHERE tenant_id=? AND user_id=?`, qaEmployee, tenantID, qaUser); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,valid_from,current_flag,created_by) SELECT ?,?,?,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,UTC_TIMESTAMP(6),1,? FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=1`, tenantID, qaAssignment, qaEmployee, qaUser, tenantID, seedEmployee); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO approval_flows (tenant_id,id,request_type,name,is_default,valid_from) VALUES (?,?,'termination','QA Termination Flow',TRUE,'2000-01-01')`, tenantID, flowID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO approval_flow_steps (tenant_id,id,flow_id,level_no,approver_kind,approver_ref_id,required_permission) VALUES (?,?,?,1,'role',?,'terminasi.approve')`, tenantID, stepID, flowID, adminRoleID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, qa, "POST", server.URL+"/api/v1/auth/login", map[string]string{"email": email, "password": password}, 200, nil)
	today := tenantToday("Asia/Jakarta")
	var essResult struct {
		Data struct {
			ID        string `json:"id"`
			RequestID string `json:"request_id"`
			Status    string `json:"status"`
			Version   uint64 `json:"version"`
		} `json:"data"`
	}
	doJSON(t, qa, "POST", server.URL+"/api/v1/ess/resignation", map[string]any{"effective_date": today, "reason": "Pengunduran diri QA untuk pengujian alur"}, 200, &essResult)
	termIDs = append(termIDs, essResult.Data.ID)
	requestIDs = append(requestIDs, essResult.Data.RequestID)
	if essResult.Data.Status != "in_review" || essResult.Data.RequestID == "" {
		t.Fatalf("ESS resignation %#v", essResult)
	}
	doJSON(t, qa, "GET", server.URL+"/api/v1/ess/resignation", nil, 200, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/terminations/"+essResult.Data.ID, nil, 200, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+essResult.Data.RequestID+"/actions", map[string]any{"action": "return", "reason": "Perbaiki uraian dan tanggal pengunduran diri QA", "level_no": 1, "version": 1}, 200, nil)
	var returned struct {
		Data termination `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/terminations/"+essResult.Data.ID, nil, 200, &returned)
	if returned.Data.Status != "returned" {
		t.Fatalf("expected returned: %#v", returned.Data)
	}
	oldRequestID := essResult.Data.RequestID
	doJSON(t, qa, "POST", server.URL+"/api/v1/ess/resignation", map[string]any{"effective_date": today, "reason": "Pengunduran diri QA sesudah uraian diperbaiki"}, 200, &essResult)
	if essResult.Data.RequestID != oldRequestID || essResult.Data.Status != "in_review" {
		t.Fatalf("resubmission changed request: %#v", essResult.Data)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/terminations/"+essResult.Data.ID+"/cancel", map[string]any{"version": essResult.Data.Version, "reason": "Membatalkan pengujian resign QA"}, 403, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "password"}, 200, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/terminations/"+essResult.Data.ID+"/cancel", map[string]any{"version": essResult.Data.Version, "reason": "Membatalkan pengujian resign QA"}, 200, nil)
	var made struct {
		Data struct {
			ID      string `json:"id"`
			Version uint64 `json:"version"`
		} `json:"data"`
	}
	doJSON(t, qa, "POST", server.URL+"/api/v1/admin/terminations", map[string]any{"employee_id": qaEmployee, "termination_type": "dismissal", "effective_date": today, "reason": "Pemutusan QA untuk pengujian alur"}, 201, &made)
	termIDs = append(termIDs, made.Data.ID)
	doJSON(t, qa, "POST", server.URL+"/api/v1/admin/terminations", map[string]any{"employee_id": qaEmployee, "termination_type": "dismissal", "effective_date": today, "reason": "Duplikat proses terminasi QA"}, 409, nil)
	doJSON(t, qa, "PATCH", server.URL+"/api/v1/admin/terminations/"+made.Data.ID, map[string]any{"effective_date": today, "reason": "Alasan pemutusan QA yang telah dikoreksi", "version": made.Data.Version}, 200, nil)
	check := map[string]any{"version": uint64(2), "items": []map[string]any{{"code": "assets_returned", "is_done": true, "note": "Aset QA kembali"}, {"code": "handover_complete", "is_done": true, "note": "Serah terima QA selesai"}}}
	doJSON(t, qa, "PUT", server.URL+"/api/v1/admin/terminations/"+made.Data.ID+"/checklist", check, 200, nil)
	var submitted struct {
		Data struct {
			RequestID       string `json:"request_id"`
			ApprovalVersion uint64 `json:"approval_version"`
		} `json:"data"`
	}
	doJSON(t, qa, "POST", server.URL+"/api/v1/admin/terminations/"+made.Data.ID+"/submit", nil, 200, &submitted)
	requestIDs = append(requestIDs, submitted.Data.RequestID)
	doJSON(t, qa, "PATCH", server.URL+"/api/v1/admin/terminations/"+made.Data.ID, map[string]any{"effective_date": today, "reason": "Koreksi setelah submit dilarang", "version": 4}, 409, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+submitted.Data.RequestID+"/actions", map[string]any{"action": "approve", "level_no": 1, "version": submitted.Data.ApprovalVersion}, 200, nil)
	processed, err := applyDueTermination(ctx, db, tenantID, made.Data.ID, "Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("approved termination was not processed")
	}
	var employeeStatus, memberStatus, termStatus string
	var activeRoles, activeSessions, currentAssignments, finalEvents int
	if err = db.QueryRow(`SELECT status FROM employees WHERE tenant_id=? AND id=?`, tenantID, qaEmployee).Scan(&employeeStatus); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT status FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, tenantID, qaUser).Scan(&memberStatus); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT status FROM terminations WHERE tenant_id=? AND id=?`, tenantID, made.Data.ID).Scan(&termStatus); err != nil {
		t.Fatal(err)
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM user_roles WHERE tenant_id=? AND user_id=? AND revoked_at IS NULL`, tenantID, qaUser).Scan(&activeRoles)
	_ = db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id=? AND revoked_at IS NULL`, qaUser).Scan(&activeSessions)
	_ = db.QueryRow(`SELECT COUNT(*) FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=1`, tenantID, qaEmployee).Scan(&currentAssignments)
	_ = db.QueryRow(`SELECT COUNT(*) FROM notification_outbox WHERE tenant_id=? AND aggregate_type='termination' AND aggregate_id=? AND event_type='payroll.final_requested'`, tenantID, made.Data.ID).Scan(&finalEvents)
	if employeeStatus != "terminated" || memberStatus != "inactive" || termStatus != "processed" || activeRoles != 0 || activeSessions != 0 || currentAssignments != 0 || finalEvents != 1 {
		t.Fatalf("effects: employee=%s member=%s term=%s roles=%d sessions=%d assignment=%d payroll=%d", employeeStatus, memberStatus, termStatus, activeRoles, activeSessions, currentAssignments, finalEvents)
	}
	doJSON(t, qa, "GET", server.URL+"/api/v1/auth/me", nil, 401, nil)
	processed, err = applyDueTermination(ctx, db, tenantID, made.Data.ID, "Asia/Jakarta")
	if err != nil || processed {
		t.Fatalf("retry: %v %v", processed, err)
	}
	var auditReason sql.NullString
	if err = db.QueryRow(`SELECT CAST(GROUP_CONCAT(COALESCE(after_json,'') SEPARATOR ' ') AS CHAR) FROM audit_logs WHERE tenant_id=? AND entity_type='terminations' AND entity_id=?`, tenantID, made.Data.ID).Scan(&auditReason); err != nil {
		t.Fatal(err)
	}
	if auditReason.Valid && strings.Contains(auditReason.String, "Alasan pemutusan QA") {
		t.Fatal("reason leaked to audit")
	}
}
