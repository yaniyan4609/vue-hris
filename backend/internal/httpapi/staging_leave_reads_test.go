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

func TestStagingEssLeaveReads(t *testing.T) {
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
	t.Cleanup(func() { db.Close() })
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	ess := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var state auth.State
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &state)
	var employeeID string
	if err := db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? AND user_id=?`, state.SelectedTenantID, state.User.ID).Scan(&employeeID); err != nil {
		t.Fatal(err)
	}
	typeID, _ := ids.New()
	ledgerID, _ := ids.New()
	refID, _ := ids.New()
	if _, err := db.Exec(`INSERT INTO leave_types (tenant_id,id,code,name,is_paid,requires_attachment,is_active) VALUES (?,?,?,'QA Leave Read',TRUE,FALSE,TRUE)`, state.SelectedTenantID, typeID, "QA-"+typeID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM leave_balance_ledger WHERE tenant_id=? AND id=?`, state.SelectedTenantID, ledgerID); err != nil {
			t.Errorf("cleanup leave ledger: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM leave_types WHERE tenant_id=? AND id=?`, state.SelectedTenantID, typeID); err != nil {
			t.Errorf("cleanup leave type: %v", err)
		}
	})
	if _, err := db.Exec(`INSERT INTO leave_balance_ledger (tenant_id,id,employee_id,leave_type_id,entry_type,days,effective_date,ref_type,ref_id) VALUES (?,?,?,?,'accrual',1.25,CURRENT_DATE(),'qa',?)`, state.SelectedTenantID, ledgerID, employeeID, typeID, refID); err != nil {
		t.Fatal(err)
	}
	var types struct {
		Data []essLeaveType `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/leave/types", nil, 200, &types)
	found := false
	for _, item := range types.Data {
		if item.ID == typeID {
			found = true
		}
	}
	if !found {
		t.Fatal("leave type missing")
	}
	var balances struct {
		Data []struct {
			LeaveTypeID string `json:"leave_type_id"`
			BalanceDays string `json:"balance_days"`
		} `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/leave/balances", nil, 200, &balances)
	found = false
	for _, item := range balances.Data {
		if item.LeaveTypeID == typeID {
			found = item.BalanceDays == "1.25"
		}
	}
	if !found {
		t.Fatalf("leave balance missing: %#v", balances.Data)
	}
	var ledger struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/leave/balances/"+typeID+"/ledger", nil, 200, &ledger)
	if len(ledger.Data) != 1 || ledger.Data[0]["days"] != "1.25" {
		t.Fatalf("leave ledger: %#v", ledger.Data)
	}
}
