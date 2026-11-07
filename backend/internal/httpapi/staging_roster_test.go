package httpapi

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
)

func TestStagingDraftRosterScopeAndVersion(t *testing.T) {
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
	var employeeID, officeID string
	if err := db.QueryRow(`SELECT e.id,a.office_id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.status='active' LIMIT 1`, state.SelectedTenantID).Scan(&employeeID, &officeID); err != nil {
		t.Fatal(err)
	}
	qaID, _ := ids.New()
	code := "QA_" + qaID
	var shift struct {
		Data shiftTemplate `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/shift-templates", shiftTemplate{Code: code, Name: "QA Roster Shift", StartTime: "09:00", EndTime: "17:00", ClockInWindowStartTime: "08:00", ClockInWindowEndTime: "10:00", ClockOutWindowStartTime: "16:00", ClockOutWindowEndTime: "19:00", IsActive: true}, 201, &shift)
	defer func() {
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='shift_templates' AND entity_id=?`, state.SelectedTenantID, shift.Data.ID); err != nil {
			t.Errorf("cleanup shift audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM shift_templates WHERE tenant_id=? AND id=?`, state.SelectedTenantID, shift.Data.ID); err != nil {
			t.Errorf("cleanup shift: %v", err)
		}
	}()
	var created struct {
		Data workSchedule `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/work-schedules", map[string]any{"employee_id": employeeID, "work_date": "2099-11-07", "day_type": "shift", "shift_template_id": shift.Data.ID, "office_id": officeID}, 201, &created)
	defer func() {
		if _, err := db.Exec(`DELETE FROM work_schedule_revisions WHERE tenant_id=? AND work_schedule_id=?`, state.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup revisions: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='work_schedules' AND entity_id=?`, state.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup roster audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM work_schedules WHERE tenant_id=? AND id=?`, state.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup roster: %v", err)
		}
	}()
	if created.Data.ID == "" || created.Data.Version != 1 {
		t.Fatalf("created: %#v", created)
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/work-schedules", map[string]any{"employee_id": employeeID, "work_date": "2099-11-07", "day_type": "shift", "shift_template_id": shift.Data.ID, "office_id": officeID}, 409, nil)
	var list struct {
		Data []workSchedule `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/work-schedules?employee_id="+employeeID+"&from=2099-11-07&to=2099-11-07", nil, 200, &list)
	if len(list.Data) != 1 || list.Data[0].ID != created.Data.ID {
		t.Fatalf("roster list: %#v", list)
	}
	var changed struct {
		Data workSchedule `json:"data"`
	}
	doJSON(t, client, "PATCH", server.URL+"/api/v1/admin/work-schedules/"+created.Data.ID, map[string]any{"version": 1, "work_date": "2099-11-08", "reason": "QA edit"}, 200, &changed)
	if changed.Data.WorkDate != "2099-11-08" || changed.Data.Version != 2 {
		t.Fatal("roster update failed")
	}
	doJSON(t, client, "PATCH", server.URL+"/api/v1/admin/work-schedules/"+created.Data.ID, map[string]any{"version": 1, "work_date": "2099-11-09"}, 409, nil)
	doJSON(t, client, "DELETE", server.URL+"/api/v1/admin/work-schedules/"+created.Data.ID, nil, 204, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/work-schedules?employee_id="+employeeID+"&from=2099-11-08&to=2099-11-08", nil, 200, &list)
	if len(list.Data) != 0 {
		t.Fatal("cancelled roster visible by default")
	}
}
