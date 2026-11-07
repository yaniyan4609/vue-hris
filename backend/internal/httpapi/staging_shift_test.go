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

func TestStagingShiftTemplates(t *testing.T) {
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
	target := 480
	item := shiftTemplate{Code: code, Name: "QA Shift", StartTime: "09:00", EndTime: "17:00", ClockInWindowStartTime: "08:00", ClockInWindowEndTime: "10:00", ClockOutWindowStartTime: "16:00", ClockOutWindowEndTime: "19:00", TargetWorkMinutes: &target, IsActive: true, ColorHex: "#123ABC"}
	var created struct {
		Data shiftTemplate `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/shift-templates", item, 201, &created)
	idsToClean := []string{created.Data.ID}
	defer func() {
		for _, id := range idsToClean {
			if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='shift_templates' AND entity_id=?`, state.SelectedTenantID, id); err != nil {
				t.Errorf("cleanup shift audit: %v", err)
			}
			if _, err := db.Exec(`DELETE FROM shift_templates WHERE tenant_id=? AND id=?`, state.SelectedTenantID, id); err != nil {
				t.Errorf("cleanup shift: %v", err)
			}
		}
	}()
	if created.Data.ID == "" || created.Data.StartTime != "09:00:00" {
		t.Fatalf("created: %#v", created)
	}
	var detail struct {
		Data shiftTemplate `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/shift-templates/"+created.Data.ID, nil, 200, &detail)
	if detail.Data.Code != code {
		t.Fatal("wrong shift")
	}
	var list struct {
		Data []shiftTemplate `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/shift-templates?q="+code, nil, 200, &list)
	if list.Meta.Total != 1 || len(list.Data) != 1 {
		t.Fatalf("shift list: %#v", list)
	}
	order := 32000
	doJSON(t, client, "PATCH", server.URL+"/api/v1/admin/shift-templates/"+created.Data.ID, map[string]any{"is_pinned": true, "pin_order": order}, 200, &detail)
	if !detail.Data.IsPinned || detail.Data.PinOrder == nil || *detail.Data.PinOrder != order {
		t.Fatal("pin not saved")
	}
	var duplicate struct {
		Data shiftTemplate `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/shift-templates/"+created.Data.ID+"/duplicate", map[string]string{"code": code + "_COPY", "name": "QA Copy"}, 201, &duplicate)
	idsToClean = append(idsToClean, duplicate.Data.ID)
	if duplicate.Data.IsPinned || duplicate.Data.Code != code+"_COPY" {
		t.Fatal("duplicate is invalid")
	}
	doJSON(t, client, "DELETE", server.URL+"/api/v1/admin/shift-templates/"+created.Data.ID, nil, 204, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/shift-templates/"+created.Data.ID, nil, 200, &detail)
	if detail.Data.IsActive || detail.Data.IsPinned {
		t.Fatal("shift not deactivated")
	}
}
