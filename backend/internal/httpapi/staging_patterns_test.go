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

func TestStagingMonthlyPatternSixthWeekAndVersion(t *testing.T) {
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
	qaID, _ := ids.New()
	code := "QA_" + qaID
	var shift struct {
		Data shiftTemplate `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/shift-templates", shiftTemplate{Code: code, Name: "QA Pattern Shift", StartTime: "09:00", EndTime: "17:00", ClockInWindowStartTime: "08:00", ClockInWindowEndTime: "10:00", ClockOutWindowStartTime: "16:00", ClockOutWindowEndTime: "19:00", IsActive: true}, 201, &shift)
	patternIDs := []string{}
	defer func() {
		for _, id := range patternIDs {
			for _, query := range []string{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='schedule_patterns' AND entity_id=?`, `DELETE FROM schedule_pattern_slots WHERE tenant_id=? AND pattern_id=?`, `DELETE FROM schedule_patterns WHERE tenant_id=? AND id=?`} {
				if _, err := db.Exec(query, state.SelectedTenantID, id); err != nil {
					t.Errorf("cleanup pattern: %v", err)
				}
			}
		}
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='shift_templates' AND entity_id=?`, state.SelectedTenantID, shift.Data.ID); err != nil {
			t.Errorf("cleanup shift audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM shift_templates WHERE tenant_id=? AND id=?`, state.SelectedTenantID, shift.Data.ID); err != nil {
			t.Errorf("cleanup shift: %v", err)
		}
	}()
	slots := make([]patternSlot, 0, 35)
	for week := 1; week <= 5; week++ {
		for day := 1; day <= 7; day++ {
			slot := patternSlot{WeekNo: week, Weekday: day, DayType: "off"}
			if week == 5 && day == 1 {
				slot.DayType = "shift"
				slot.ShiftTemplateID = &shift.Data.ID
			}
			slots = append(slots, slot)
		}
	}
	var created struct {
		Data schedulePattern `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/schedule-patterns", schedulePattern{Code: code, Name: "QA Monthly", PatternType: "monthly", Slots: slots, IsActive: true}, 201, &created)
	patternIDs = append(patternIDs, created.Data.ID)
	if created.Data.Version != 1 || len(created.Data.Slots) != 35 {
		t.Fatalf("pattern: %#v", created)
	}
	var preview struct {
		Data struct {
			Days []struct {
				WorkDate        string  `json:"work_date"`
				WeekNo          int     `json:"week_no"`
				ShiftTemplateID *string `json:"shift_template_id"`
			} `json:"days"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/schedule-patterns/"+created.Data.ID+"/preview", map[string]string{"month": "2026-08"}, 200, &preview)
	found := false
	for _, day := range preview.Data.Days {
		if day.WorkDate == "2026-08-31" {
			found = true
			if day.WeekNo != 5 || day.ShiftTemplateID == nil || *day.ShiftTemplateID != shift.Data.ID {
				t.Fatalf("week six mapping: %#v", day)
			}
		}
	}
	if !found {
		t.Fatal("August 31 missing")
	}
	var replaced struct {
		Data struct {
			Version int `json:"version"`
		} `json:"data"`
	}
	doJSON(t, client, "PUT", server.URL+"/api/v1/admin/schedule-patterns/"+created.Data.ID+"/slots", map[string]any{"version": 1, "slots": slots}, 200, &replaced)
	if replaced.Data.Version != 2 {
		t.Fatal("version did not advance")
	}
	doJSON(t, client, "PUT", server.URL+"/api/v1/admin/schedule-patterns/"+created.Data.ID+"/slots", map[string]any{"version": 1, "slots": slots}, 409, nil)
	var duplicate struct {
		Data schedulePattern `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/schedule-patterns/"+created.Data.ID+"/duplicate", map[string]string{"code": code + "_COPY", "name": "QA Copy"}, 201, &duplicate)
	patternIDs = append(patternIDs, duplicate.Data.ID)
	if duplicate.Data.Version != 1 || len(duplicate.Data.Slots) != 35 {
		t.Fatal("duplicate pattern invalid")
	}
	var employeeID string
	if err := db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? AND status='active' LIMIT 1`, state.SelectedTenantID).Scan(&employeeID); err != nil {
		t.Fatal(err)
	}
	bulkBody := map[string]any{"employee_ids": []string{employeeID}, "pattern_id": created.Data.ID, "from": "2099-11-09", "to": "2099-11-10", "dry_run": true}
	var dryRun struct {
		Data struct {
			Proposed  []workSchedule       `json:"proposed"`
			Conflicts []rosterBulkConflict `json:"conflicts"`
			CanApply  bool                 `json:"can_apply"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/work-schedules/bulk", bulkBody, 200, &dryRun)
	if len(dryRun.Data.Proposed) != 2 || len(dryRun.Data.Conflicts) != 0 || !dryRun.Data.CanApply {
		t.Fatalf("bulk preview: %#v", dryRun)
	}
	bulkBody["dry_run"] = false
	var applied struct {
		Data struct {
			Created []workSchedule `json:"created"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/work-schedules/bulk", bulkBody, 201, &applied)
	defer func() {
		for _, item := range applied.Data.Created {
			if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='work_schedules' AND entity_id=?`, state.SelectedTenantID, item.ID); err != nil {
				t.Errorf("cleanup bulk audit: %v", err)
			}
			if _, err := db.Exec(`DELETE FROM work_schedules WHERE tenant_id=? AND id=?`, state.SelectedTenantID, item.ID); err != nil {
				t.Errorf("cleanup bulk roster: %v", err)
			}
		}
	}()
	if len(applied.Data.Created) != 2 {
		t.Fatal("bulk apply did not create two rows")
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/work-schedules/bulk", bulkBody, 409, nil)
	doJSON(t, client, "DELETE", server.URL+"/api/v1/admin/schedule-patterns/"+created.Data.ID, nil, 204, nil)
}
