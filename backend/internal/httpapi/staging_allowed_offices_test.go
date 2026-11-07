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

func TestStagingAllowedOfficesRevocation(t *testing.T) {
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
	var adminState, essState auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &adminState)
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &essState)
	var employeeID, regionID string
	if err := db.QueryRow(`SELECT e.id,a.region_id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.user_id=?`, adminState.SelectedTenantID, essState.User.ID).Scan(&employeeID, &regionID); err != nil {
		t.Fatal(err)
	}
	officeID, _ := ids.New()
	if _, err := db.Exec(`INSERT INTO offices (tenant_id,id,region_id,code,name,timezone) VALUES (?,?,?,?,?,'Asia/Jakarta')`, adminState.SelectedTenantID, officeID, regionID, "QA_"+officeID, "QA Allowed Office"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(`DELETE FROM offices WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, officeID); err != nil {
			t.Errorf("cleanup office: %v", err)
		}
	}()
	var created struct {
		Data allowedOffice `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/employees/"+employeeID+"/allowed-offices", map[string]any{"office_id": officeID, "valid_from": "2000-01-01", "reason": "QA"}, 201, &created)
	defer func() {
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employee_allowed_offices' AND entity_id=?`, adminState.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM employee_allowed_offices WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup allowed office: %v", err)
		}
	}()
	if created.Data.OfficeID != officeID {
		t.Fatalf("created: %#v", created)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/employees/"+employeeID+"/allowed-offices", map[string]any{"office_id": officeID, "valid_from": "2020-01-01"}, 409, nil)
	var today struct {
		Data struct {
			AllowedOffices []struct {
				ID string `json:"id"`
			} `json:"allowed_offices"`
		} `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/attendance/today", nil, 200, &today)
	contains := func() bool {
		for _, o := range today.Data.AllowedOffices {
			if o.ID == officeID {
				return true
			}
		}
		return false
	}
	if !contains() {
		t.Fatal("allowed office missing from ESS")
	}
	doJSON(t, admin, "DELETE", server.URL+"/api/v1/admin/employees/"+employeeID+"/allowed-offices/"+created.Data.ID, nil, 204, nil)
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/attendance/today", nil, 200, &today)
	if contains() {
		t.Fatal("revoked office still available in ESS")
	}
	var list struct {
		Data []allowedOffice `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/employees/"+employeeID+"/allowed-offices", nil, 200, &list)
	found := false
	for _, item := range list.Data {
		if item.ID == created.Data.ID {
			found = item.RevokedAt != nil
		}
	}
	if !found {
		t.Fatal("revocation history missing")
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/admin/employees/"+employeeID+"/allowed-offices", nil, 403, nil)
}
