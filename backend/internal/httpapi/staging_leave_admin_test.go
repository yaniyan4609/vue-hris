package httpapi

import (
	"context"
	"fmt"
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

func TestStagingAdminLeaveConfiguration(t *testing.T) {
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
	admin := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var state auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &state)
	var employeeID string
	if err = db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? ORDER BY id LIMIT 1`, state.SelectedTenantID).Scan(&employeeID); err != nil {
		t.Fatal(err)
	}
	typeID, policyID, ledgerID := "", "", ""
	t.Cleanup(func() {
		tenant := state.SelectedTenantID
		if ledgerID != "" {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='leave_balance_ledger' AND entity_id=?`, tenant, ledgerID); e != nil {
				t.Errorf("cleanup adjustment audit: %v", e)
			}
			if _, e := db.Exec(`DELETE FROM leave_balance_ledger WHERE tenant_id=? AND id=?`, tenant, ledgerID); e != nil {
				t.Errorf("cleanup adjustment: %v", e)
			}
		}
		if typeID != "" {
			if _, e := db.Exec(`DELETE FROM leave_balances WHERE tenant_id=? AND employee_id=? AND leave_type_id=?`, tenant, employeeID, typeID); e != nil {
				t.Errorf("cleanup balance: %v", e)
			}
		}
		if policyID != "" {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='leave_policies' AND entity_id=?`, tenant, policyID); e != nil {
				t.Errorf("cleanup policy audit: %v", e)
			}
			if _, e := db.Exec(`DELETE FROM leave_policies WHERE tenant_id=? AND id=?`, tenant, policyID); e != nil {
				t.Errorf("cleanup policy: %v", e)
			}
		}
		if typeID != "" {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='leave_types' AND entity_id=?`, tenant, typeID); e != nil {
				t.Errorf("cleanup type audit: %v", e)
			}
			if _, e := db.Exec(`DELETE FROM leave_types WHERE tenant_id=? AND id=?`, tenant, typeID); e != nil {
				t.Errorf("cleanup type: %v", e)
			}
		}
	})
	codeID, _ := ids.New()
	code := "QA_" + codeID
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-types", map[string]any{"code": code, "name": "QA Leave Admin", "is_paid": true, "requires_attachment": false}, 201, &created)
	typeID = created.Data.ID
	doJSON(t, admin, "PATCH", server.URL+"/api/v1/admin/leave-types/"+typeID, map[string]any{"code": code, "name": "QA Leave Admin Updated", "is_paid": true, "requires_attachment": false, "is_active": true}, 200, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/leave-types", nil, 200, nil)
	var policy struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-policies", map[string]any{"leave_type_id": typeID, "accrual_type": "manual", "quota_days": 12, "valid_from": "2000-01-01"}, 201, &policy)
	policyID = policy.Data.ID
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-policies", map[string]any{"leave_type_id": typeID, "accrual_type": "manual", "valid_from": "2020-01-01"}, 409, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/leave-policies?leave_type_id="+typeID, nil, 200, nil)
	doJSON(t, admin, "PATCH", server.URL+"/api/v1/admin/leave-policies/"+policyID, map[string]any{"valid_to": "2030-12-31"}, 200, nil)
	year := time.Now().Year() + 3
	date := fmt.Sprintf("%04d-01-02", year)
	adjust := map[string]any{"employee_id": employeeID, "leave_type_id": typeID, "days": 1.25, "effective_date": date, "reason": "QA saldo awal"}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-balances/adjustments", adjust, 403, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "password"}, 200, nil)
	var adjustment struct {
		Data struct {
			ID           string `json:"id"`
			BalanceAfter string `json:"balance_after"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-balances/adjustments", adjust, 201, &adjustment)
	ledgerID = adjustment.Data.ID
	if adjustment.Data.BalanceAfter != "1.25" {
		t.Fatalf("balance after: %s", adjustment.Data.BalanceAfter)
	}
	adjust["days"] = -2.0
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-balances/adjustments", adjust, 409, nil)
	doJSON(t, admin, "DELETE", server.URL+"/api/v1/admin/leave-policies/"+policyID, nil, 204, nil)
	doJSON(t, admin, "DELETE", server.URL+"/api/v1/admin/leave-types/"+typeID, nil, 204, nil)
}
