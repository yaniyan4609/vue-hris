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
)

func TestStagingHolidays(t *testing.T) {
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
	newClient := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, Timeout: 10 * time.Second}
	}
	admin, ess := newClient(), newClient()
	var state auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &state)
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, nil)
	var created struct {
		Data holiday `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/holidays", map[string]any{"holiday_date": "2099-04-01", "name": "QA Temporary", "holiday_type": "company"}, 201, &created)
	defer func() {
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='holidays' AND entity_id=?`, state.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup holiday audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM holidays WHERE tenant_id=? AND id=?`, state.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup holiday: %v", err)
		}
	}()
	if created.Data.ID == "" {
		t.Fatal("holiday missing ID")
	}
	var list struct {
		Data []holiday `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/holidays?year=2099&per_page=100", nil, 200, &list)
	found := false
	for _, item := range list.Data {
		if item.ID == created.Data.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("admin holiday missing")
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/holidays?year=2099", nil, 200, &list)
	found = false
	for _, item := range list.Data {
		if item.ID == created.Data.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("ESS holiday missing")
	}
	var changed struct {
		Data holiday `json:"data"`
	}
	doJSON(t, admin, "PATCH", server.URL+"/api/v1/admin/holidays/"+created.Data.ID, map[string]any{"name": "QA Updated"}, 200, &changed)
	if changed.Data.Name != "QA Updated" {
		t.Fatal("holiday patch failed")
	}
	doJSON(t, admin, "DELETE", server.URL+"/api/v1/admin/holidays/"+created.Data.ID, nil, 204, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/holidays?year=2099", nil, 200, &list)
	for _, item := range list.Data {
		if item.ID == created.Data.ID {
			t.Fatal("deleted holiday visible")
		}
	}
}
