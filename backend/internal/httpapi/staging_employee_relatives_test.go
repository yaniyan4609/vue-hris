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

func TestStagingEmployeeRelatives(t *testing.T) {
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
	var employeeID string
	if err := db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? AND user_id=?`, adminState.SelectedTenantID, essState.User.ID).Scan(&employeeID); err != nil {
		t.Fatal(err)
	}
	base := server.URL + "/api/v1/admin/employees/" + employeeID
	var family struct {
		Data employeeFamily `json:"data"`
	}
	doJSON(t, admin, "POST", base+"/families", map[string]any{"relation": "anak", "name": "QA Family", "birth_date": "2010-05-11", "is_dependent": true}, 201, &family)
	defer func() {
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employee_families' AND entity_id=?`, adminState.SelectedTenantID, family.Data.ID); err != nil {
			t.Errorf("cleanup family audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM employee_families WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, family.Data.ID); err != nil {
			t.Errorf("cleanup family: %v", err)
		}
	}()
	if family.Data.ID == "" || !family.Data.IsDependent {
		t.Fatalf("family: %#v", family.Data)
	}
	var families struct {
		Data []employeeFamily `json:"data"`
	}
	doJSON(t, admin, "GET", base+"/families", nil, 200, &families)
	found := false
	for _, item := range families.Data {
		if item.ID == family.Data.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("family missing")
	}
	doJSON(t, admin, "PATCH", base+"/families/"+family.Data.ID, map[string]any{"birth_date": nil, "is_dependent": false}, 200, &family)
	if family.Data.BirthDate != nil || family.Data.IsDependent {
		t.Fatalf("family update: %#v", family.Data)
	}
	doJSON(t, ess, "GET", base+"/families", nil, 403, nil)
	doJSON(t, admin, "DELETE", base+"/families/"+family.Data.ID, nil, 204, nil)
	doJSON(t, admin, "PATCH", base+"/families/"+family.Data.ID, map[string]any{"name": "No"}, 404, nil)
	var contact struct {
		Data emergencyContact `json:"data"`
	}
	doJSON(t, admin, "POST", base+"/emergency-contacts", map[string]any{"name": "QA Contact", "relation": "saudara", "phone": "+628111234567"}, 201, &contact)
	defer func() {
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employee_emergency_contacts' AND entity_id=?`, adminState.SelectedTenantID, contact.Data.ID); err != nil {
			t.Errorf("cleanup contact audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM employee_emergency_contacts WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, contact.Data.ID); err != nil {
			t.Errorf("cleanup contact: %v", err)
		}
	}()
	if contact.Data.ID == "" {
		t.Fatal("contact id missing")
	}
	var contacts struct {
		Data []emergencyContact `json:"data"`
	}
	doJSON(t, admin, "GET", base+"/emergency-contacts", nil, 200, &contacts)
	found = false
	for _, item := range contacts.Data {
		if item.ID == contact.Data.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("contact missing")
	}
	doJSON(t, admin, "PATCH", base+"/emergency-contacts/"+contact.Data.ID, map[string]any{"phone": "081122233344"}, 200, &contact)
	if contact.Data.Phone != "081122233344" {
		t.Fatalf("contact update: %#v", contact.Data)
	}
	doJSON(t, ess, "GET", base+"/emergency-contacts", nil, 403, nil)
	doJSON(t, admin, "DELETE", base+"/emergency-contacts/"+contact.Data.ID, nil, 204, nil)
	doJSON(t, admin, "PATCH", base+"/emergency-contacts/"+contact.Data.ID, map[string]any{"name": "No"}, 404, nil)
}
