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

func TestStagingContractDraftAndActivation(t *testing.T) {
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
	base := server.URL + "/api/v1/admin/contracts"
	doJSON(t, admin, "POST", base, map[string]any{"employee_id": employeeID, "contract_type": "PKWT", "start_date": "2099-12-31", "end_date": "2099-01-01"}, 400, nil)
	var active struct {
		Data contract `json:"data"`
	}
	doJSON(t, admin, "POST", base, map[string]any{"employee_id": employeeID, "contract_type": "PKWT", "start_date": "2099-01-01", "end_date": "2099-12-31"}, 201, &active)
	defer func() {
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='contracts' AND entity_id=?`, adminState.SelectedTenantID, active.Data.ID); err != nil {
			t.Errorf("cleanup audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM contracts WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, active.Data.ID); err != nil {
			t.Errorf("cleanup contract: %v", err)
		}
	}()
	if active.Data.Status != "draft" {
		t.Fatalf("created: %#v", active.Data)
	}
	doJSON(t, admin, "PATCH", base+"/"+active.Data.ID, map[string]any{"status": "active"}, 200, &active)
	if active.Data.Status != "active" {
		t.Fatalf("activated: %#v", active.Data)
	}
	doJSON(t, admin, "DELETE", base+"/"+active.Data.ID, nil, 409, nil)
	var draft struct {
		Data contract `json:"data"`
	}
	doJSON(t, admin, "POST", base, map[string]any{"employee_id": employeeID, "contract_type": "PKWT", "start_date": "2099-06-01", "end_date": "2099-06-30"}, 201, &draft)
	defer func() {
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='contracts' AND entity_id=?`, adminState.SelectedTenantID, draft.Data.ID); err != nil {
			t.Errorf("cleanup second audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM contracts WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, draft.Data.ID); err != nil {
			t.Errorf("cleanup second contract: %v", err)
		}
	}()
	doJSON(t, admin, "PATCH", base+"/"+draft.Data.ID, map[string]any{"status": "active"}, 409, nil)
	doJSON(t, admin, "PATCH", base+"/"+draft.Data.ID, map[string]any{"end_date": "2099-07-15"}, 200, &draft)
	if draft.Data.EndDate == nil || *draft.Data.EndDate != "2099-07-15" {
		t.Fatalf("draft update: %#v", draft.Data)
	}
	doJSON(t, admin, "DELETE", base+"/"+draft.Data.ID, nil, 204, nil)
	doJSON(t, ess, "POST", base, map[string]any{"employee_id": employeeID, "contract_type": "PKWTT", "start_date": "2099-01-01"}, 403, nil)
}
