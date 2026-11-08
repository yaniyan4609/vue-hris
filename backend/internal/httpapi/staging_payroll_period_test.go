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

func TestStagingPayrollPeriodFinanceAccess(t *testing.T) {
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
	var tenantID, roleID string
	if err = db.QueryRow(`SELECT r.tenant_id,r.id FROM roles r JOIN tenants t ON t.id=r.tenant_id WHERE t.slug='client-pertama' AND r.code='FINANCE_PAYROLL'`).Scan(&tenantID, &roleID); err != nil {
		t.Fatal(err)
	}
	userID, _ := ids.New()
	roleAssignmentID, _ := ids.New()
	email := "qa-payroll-" + userID + "@example.invalid"
	password := "Qa-Payroll-Password-29"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO users (id,email,email_normalized,display_name,password_hash) VALUES (?,?,?,'QA Payroll',?)`, userID, email, email, hash); err != nil {
		t.Fatal(err)
	}
	periodID := ""
	t.Cleanup(func() {
		if periodID != "" {
			_, _ = db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='payroll_periods' AND entity_id=?`, tenantID, periodID)
			if _, e := db.Exec(`DELETE FROM payroll_periods WHERE tenant_id=? AND id=?`, tenantID, periodID); e != nil {
				t.Errorf("cleanup payroll period: %v", e)
			}
		}
		for _, step := range []struct {
			query string
			args  []any
		}{
			{`DELETE FROM login_events WHERE user_id=?`, []any{userID}},
			{`DELETE FROM sessions WHERE user_id=?`, []any{userID}},
			{`DELETE FROM user_roles WHERE tenant_id=? AND user_id=?`, []any{tenantID, userID}},
			{`DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, []any{tenantID, userID}},
			{`DELETE FROM users WHERE id=?`, []any{userID}},
		} {
			if _, e := db.Exec(step.query, step.args...); e != nil {
				t.Errorf("cleanup finance user: %v", e)
			}
		}
	})
	if _, err = db.Exec(`INSERT INTO tenant_memberships (tenant_id,user_id,status) VALUES (?,?,'active')`, tenantID, userID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO user_roles (tenant_id,id,user_id,role_id,valid_from) VALUES (?,?,?,?,CURRENT_DATE())`, tenantID, roleAssignmentID, userID, roleID); err != nil {
		t.Fatal(err)
	}
	year, month := 2099, 12
	for ; month > 0; month-- {
		var count int
		if err = db.QueryRow(`SELECT COUNT(*) FROM payroll_periods WHERE tenant_id=? AND period_year=? AND period_month=?`, tenantID, year, month).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			break
		}
	}
	if month == 0 {
		t.Skip("no unused QA payroll period in 2099")
	}
	server := httptest.NewServer(Router(db, Options{CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/login", map[string]string{"email": email, "password": password}, 200, nil)
	var created struct {
		Data payrollPeriodView `json:"data"`
	}
	input := map[string]int{"year": year, "month": month}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/payroll/periods", input, 201, &created)
	periodID = created.Data.ID
	if created.Data.Year != year || created.Data.Month != month || created.Data.Status != "open" {
		t.Fatalf("created period: %#v", created.Data)
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/payroll/periods", input, 409, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/payroll/periods/"+periodID, nil, 200, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/payroll/periods/"+userID, nil, 404, nil)
}
