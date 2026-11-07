package httpapi

import (
	"context"
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

func TestStagingPublishRosterCreatesSnapshot(t *testing.T) {
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
	essJar, _ := cookiejar.New(nil)
	essClient := &http.Client{Jar: essJar, Timeout: 10 * time.Second}
	var essState auth.State
	doJSON(t, essClient, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &essState)
	var employeeID, officeID string
	if err := db.QueryRow(`SELECT e.id,a.office_id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.user_id=? AND e.status='active' LIMIT 1`, state.SelectedTenantID, essState.User.ID).Scan(&employeeID, &officeID); err != nil {
		t.Fatal(err)
	}
	qaID, _ := ids.New()
	code := "QA_" + qaID
	var shift struct {
		Data shiftTemplate `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/shift-templates", shiftTemplate{Code: code, Name: "QA Night", StartTime: "22:00", EndTime: "06:00", CrossesMidnight: true, ClockInWindowStartTime: "21:30", ClockInWindowEndTime: "22:30", ClockOutWindowStartTime: "05:30", ClockOutWindowStartDayOffset: 1, ClockOutWindowEndTime: "07:00", ClockOutWindowEndDayOffset: 1, RequireSelfie: true, IsActive: true}, 201, &shift)
	defer func() {
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='shift_templates' AND entity_id=?`, state.SelectedTenantID, shift.Data.ID); err != nil {
			t.Errorf("cleanup shift audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM shift_templates WHERE tenant_id=? AND id=?`, state.SelectedTenantID, shift.Data.ID); err != nil {
			t.Errorf("cleanup shift: %v", err)
		}
	}()
	var roster struct {
		Data workSchedule `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/work-schedules", map[string]any{"employee_id": employeeID, "work_date": "2099-11-11", "day_type": "shift", "shift_template_id": shift.Data.ID, "office_id": officeID}, 201, &roster)
	defer func() {
		for _, query := range []string{`DELETE FROM notifications WHERE tenant_id=? AND dedupe_key=?`, `DELETE FROM shift_instances WHERE tenant_id=? AND work_schedule_id=?`, `DELETE FROM work_schedule_revisions WHERE tenant_id=? AND work_schedule_id=?`, `DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='work_schedules' AND entity_id=?`, `DELETE FROM work_schedules WHERE tenant_id=? AND id=?`} {
			value := roster.Data.ID
			if strings.Contains(query, "dedupe_key") {
				value = "roster:" + value
			}
			if _, err := db.Exec(query, state.SelectedTenantID, value); err != nil {
				t.Errorf("cleanup publish: %v", err)
			}
		}
	}()
	var published struct {
		Data struct {
			Published []publishedInstance `json:"published"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/work-schedules/publish", map[string]any{"from": "2099-11-11", "to": "2099-11-11", "employee_ids": []string{employeeID}}, 200, &published)
	if len(published.Data.Published) != 1 || published.Data.Published[0].InstanceID == nil {
		t.Fatalf("publish: %#v", published)
	}
	var instancePage struct {
		Data []shiftInstance `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/shift-instances?employee_id="+employeeID+"&from=2099-11-11&to=2099-11-11", nil, 200, &instancePage)
	if len(instancePage.Data) != 1 || instancePage.Data[0].ID != *published.Data.Published[0].InstanceID {
		t.Fatalf("instance list: %#v", instancePage.Data)
	}
	var instanceDetail struct {
		Data shiftInstance `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/shift-instances/"+instancePage.Data[0].ID, nil, 200, &instanceDetail)
	if instanceDetail.Data.WorkScheduleID != roster.Data.ID {
		t.Fatalf("instance detail: %#v", instanceDetail.Data)
	}
	var essDays struct {
		Data []essScheduleDay `json:"data"`
	}
	doJSON(t, essClient, "GET", server.URL+"/api/v1/ess/schedule?from=2099-11-11&to=2099-11-11", nil, 200, &essDays)
	if len(essDays.Data) != 1 || essDays.Data[0].RosterID != roster.Data.ID || essDays.Data[0].InstanceID == nil {
		t.Fatalf("ESS schedule: %#v", essDays.Data)
	}
	doJSON(t, essClient, "GET", server.URL+"/api/v1/admin/shift-instances/"+instancePage.Data[0].ID, nil, 403, nil)
	var expectedStart time.Time
	var snapshot string
	if err := db.QueryRow(`SELECT expected_start,rule_snapshot FROM shift_instances WHERE tenant_id=? AND work_schedule_id=?`, state.SelectedTenantID, roster.Data.ID).Scan(&expectedStart, &snapshot); err != nil {
		t.Fatal(err)
	}
	if expectedStart.UTC().Format(time.RFC3339) != "2099-11-11T15:00:00Z" || !strings.Contains(snapshot, `"require_selfie":true`) {
		t.Fatalf("snapshot: start=%s body=%s", expectedStart.Format(time.RFC3339), snapshot)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE tenant_id=? AND dedupe_key=?`, state.SelectedTenantID, "roster:"+roster.Data.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("notification count=%d err=%v", count, err)
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/work-schedules/publish", map[string]any{"from": "2099-11-11", "to": "2099-11-11", "employee_ids": []string{employeeID}}, 404, nil)
}
