package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
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

func TestStagingPayrollSettingsVersion(t *testing.T) {
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
	tenant := state.SelectedTenantID
	var roleID string
	if err = db.QueryRowContext(ctx, `SELECT id FROM roles WHERE tenant_id=? AND code='FINANCE_PAYROLL'`, tenant).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	userID, _ := ids.New()
	assignmentID, _ := ids.New()
	email := "qa-settings-" + userID + "@example.invalid"
	password := "Qa-Settings-Password-29"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO users (id,email,email_normalized,display_name,password_hash) VALUES (?,?,?,'QA Payroll Settings',?)`, userID, email, email, hash); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, query := range []string{`DELETE FROM login_events WHERE user_id=?`, `DELETE FROM sessions WHERE user_id=?`, `DELETE FROM user_roles WHERE user_id=?`, `DELETE FROM tenant_memberships WHERE user_id=?`, `DELETE FROM users WHERE id=?`} {
			if _, e := db.Exec(query, userID); e != nil {
				t.Errorf("cleanup finance user: %v", e)
			}
		}
	})
	if _, err = db.ExecContext(ctx, `INSERT INTO tenant_memberships (tenant_id,user_id,status) VALUES (?,?,'active')`, tenant, userID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO user_roles (tenant_id,id,user_id,role_id,valid_from) VALUES (?,?,?,?,CURRENT_DATE())`, tenant, assignmentID, userID, roleID); err != nil {
		t.Fatal(err)
	}
	financeJar, _ := cookiejar.New(nil)
	finance := &http.Client{Jar: financeJar, Timeout: 10 * time.Second}
	doJSON(t, finance, "POST", server.URL+"/api/v1/auth/login", map[string]string{"email": email, "password": password}, 200, nil)
	var original []byte
	var originalActor sql.NullString
	var originalUpdated time.Time
	err = db.QueryRowContext(ctx, `SELECT value_json,updated_by,updated_at FROM tenant_settings WHERE tenant_id=? AND setting_key=?`, tenant, payrollSettingsKey).Scan(&original, &originalActor, &originalUpdated)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	hadSetting := err == nil
	var auditID string
	t.Cleanup(func() {
		if hadSetting {
			var actor any
			if originalActor.Valid {
				actor = originalActor.String
			}
			_, err = db.Exec(`UPDATE tenant_settings SET value_json=?,updated_by=?,updated_at=? WHERE tenant_id=? AND setting_key=?`, original, actor, originalUpdated, tenant, payrollSettingsKey)
		} else {
			_, err = db.Exec(`DELETE FROM tenant_settings WHERE tenant_id=? AND setting_key=?`, tenant, payrollSettingsKey)
		}
		if err != nil {
			t.Errorf("restore settings: %v", err)
		}
		if auditID != "" {
			if _, err = db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND id=?`, tenant, auditID); err != nil {
				t.Errorf("cleanup audit: %v", err)
			}
		}
	})
	var response struct {
		Data payrollSettings `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/payroll/settings", nil, 200, &response)
	version := response.Data.Version
	input := map[string]any{"version": version, "pay_day": 25, "rounding_mode": "none", "currency": "IDR", "proration_basis": "published_roster_workdays"}
	doJSON(t, admin, "PUT", server.URL+"/api/v1/admin/payroll/settings", input, 403, nil)
	doJSON(t, finance, "PUT", server.URL+"/api/v1/admin/payroll/settings", input, 200, &response)
	if response.Data.Version != version+1 || response.Data.PayDay == nil || *response.Data.PayDay != 25 {
		t.Fatalf("settings after update: %+v", response.Data)
	}
	if err = db.QueryRowContext(ctx, `SELECT id FROM audit_logs WHERE tenant_id=? AND entity_type='payroll_settings' AND entity_id=? ORDER BY created_at DESC,id DESC LIMIT 1`, tenant, tenant).Scan(&auditID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, finance, "PUT", server.URL+"/api/v1/admin/payroll/settings", input, 409, nil)
	input["version"] = version + 1
	input["pay_day"] = 32
	doJSON(t, finance, "PUT", server.URL+"/api/v1/admin/payroll/settings", input, 400, nil)
	var periodID, runID string
	t.Cleanup(func() {
		if runID != "" {
			_, _ = db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='payroll_runs' AND entity_id=?`, tenant, runID)
			if _, e := db.Exec(`DELETE FROM payroll_runs WHERE tenant_id=? AND id=?`, tenant, runID); e != nil {
				t.Errorf("cleanup run: %v", e)
			}
		}
		if periodID != "" {
			_, _ = db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='payroll_periods' AND entity_id=?`, tenant, periodID)
			if _, e := db.Exec(`DELETE FROM payroll_periods WHERE tenant_id=? AND id=?`, tenant, periodID); e != nil {
				t.Errorf("cleanup period: %v", e)
			}
		}
	})
	year, month := 2098, 12
	for ; month > 0; month-- {
		var exists int
		if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM payroll_periods WHERE tenant_id=? AND period_year=? AND period_month=?`, tenant, year, month).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists == 0 {
			break
		}
	}
	if month == 0 {
		t.Skip("no unused QA payroll period")
	}
	var period struct {
		Data payrollPeriodView `json:"data"`
	}
	doJSON(t, finance, "POST", server.URL+"/api/v1/admin/payroll/periods", map[string]int{"year": year, "month": month}, 201, &period)
	periodID = period.Data.ID
	runURL := server.URL + "/api/v1/admin/payroll/periods/" + periodID + "/runs"
	key := "c0261109-4f64-4d76-8e45-aef92a93846e"
	postRun := func(key string, want int) payrollRunView {
		req, e := http.NewRequest("POST", runURL, bytes.NewReader([]byte("{}")))
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrfToken(cookieFromJar(t, finance, server.URL)))
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		response, e := finance.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		raw, e := io.ReadAll(response.Body)
		if e != nil {
			t.Fatal(e)
		}
		if response.StatusCode != want {
			t.Fatalf("create draft status=%d want=%d body=%s", response.StatusCode, want, raw)
		}
		var result struct {
			Data payrollRunView `json:"data"`
		}
		if want < 300 {
			if e = json.Unmarshal(raw, &result); e != nil {
				t.Fatal(e)
			}
		}
		return result.Data
	}
	postRun("", 400)
	created := postRun(key, 202)
	runID = created.ID
	if created.Status != "draft" || created.CalculationState != "blocked" || created.TotalGross != nil || created.TotalNet != nil {
		t.Fatalf("draft leaked calculated amount: %+v", created)
	}
	replayed := postRun(key, 200)
	if replayed.ID != runID {
		t.Fatalf("replay run id=%s want=%s", replayed.ID, runID)
	}
	doJSON(t, finance, "GET", server.URL+"/api/v1/admin/payroll/runs/"+runID, nil, 200, nil)
	doJSON(t, finance, "GET", server.URL+"/api/v1/admin/payroll/runs?period_id="+periodID, nil, 200, nil)
	var validation struct {
		Data payrollValidationView `json:"data"`
	}
	doJSON(t, finance, "GET", server.URL+"/api/v1/admin/payroll/runs/"+runID+"/validation", nil, 200, &validation)
	if validation.Data.Ready {
		t.Fatal("unverified tax engine must block payroll")
	}
	found := false
	for _, issue := range validation.Data.Issues {
		if issue.Code == "TAX_ENGINE_UNVERIFIED" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing tax blocker: %+v", validation.Data.Issues)
	}
	doJSON(t, admin, "POST", runURL, map[string]any{}, 403, nil)
}
