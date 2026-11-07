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

func TestStagingEmployeeAssignmentAndContractReads(t *testing.T) {
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
	var assignments struct {
		Data []employeeAssignment `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/employees/"+employeeID+"/assignments", nil, 200, &assignments)
	if len(assignments.Data) == 0 || !assignments.Data[0].Current {
		t.Fatalf("assignments: %#v", assignments.Data)
	}
	id, _ := ids.New()
	zone, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Now().In(zone)
	end := today.AddDate(0, 0, 10).Format("2006-01-02")
	if _, err := db.Exec(`INSERT INTO contracts (tenant_id,id,employee_id,contract_type,start_date,end_date,status) VALUES (?,?,?,'PKWT',? ,?,'active')`, adminState.SelectedTenantID, id, employeeID, today.AddDate(0, -1, 0).Format("2006-01-02"), end); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(`DELETE FROM contracts WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, id); err != nil {
			t.Errorf("cleanup contract: %v", err)
		}
	}()
	var page struct {
		Data []contract `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/contracts?employee_id="+employeeID, nil, 200, &page)
	found := false
	for _, item := range page.Data {
		if item.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatal("contract missing from list")
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/contracts/expiring?days=30", nil, 200, &page)
	found = false
	for _, item := range page.Data {
		if item.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatal("contract missing from expiry")
	}
	var detail struct {
		Data contract `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/contracts/"+id, nil, 200, &detail)
	if detail.Data.EmployeeID != employeeID {
		t.Fatalf("detail: %#v", detail.Data)
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/employees/"+employeeID+"/contracts", nil, 200, &page)
	found = false
	for _, item := range page.Data {
		if item.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatal("contract missing from employee list")
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/admin/contracts/"+id, nil, 403, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/contracts/expiring?days=366", nil, 400, nil)
}
