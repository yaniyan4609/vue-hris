package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
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

func TestStagingBPJSWageBasisPublication(t *testing.T) {
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
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var demo auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &demo)
	tenant := demo.SelectedTenantID
	var roleID string
	if err = db.QueryRowContext(ctx, `SELECT id FROM roles WHERE tenant_id=? AND code='FINANCE_PAYROLL'`, tenant).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	userID, _ := ids.New()
	roleAssignmentID, _ := ids.New()
	componentID, _ := ids.New()
	email := "qa-bpjs-basis-" + userID + "@example.invalid"
	password := "Qa-BPJS-Basis-29"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO users (id,email,email_normalized,display_name,password_hash) VALUES (?,?,?,'QA BPJS Basis',?)`, userID, email, email, hash); err != nil {
		t.Fatal(err)
	}
	var policyID, previousID string
	var previousTo sql.NullTime
	t.Cleanup(func() {
		if policyID == "" {
			_ = db.QueryRow(`SELECT id FROM bpjs_wage_basis_policies WHERE tenant_id=? AND created_by=? ORDER BY effective_from DESC LIMIT 1`, tenant, userID).Scan(&policyID)
		}
		if policyID != "" {
			_, _ = db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='bpjs_wage_basis_policies' AND entity_id=?`, tenant, policyID)
			if _, e := db.Exec(`DELETE FROM bpjs_wage_basis_policies WHERE tenant_id=? AND id=?`, tenant, policyID); e != nil {
				t.Errorf("cleanup wage basis: %v", e)
			}
			if previousID != "" {
				var original any
				if previousTo.Valid {
					original = previousTo.Time
				}
				if _, e := db.Exec(`UPDATE bpjs_wage_basis_policies SET effective_to=? WHERE tenant_id=? AND id=?`, original, tenant, previousID); e != nil {
					t.Errorf("restore prior wage basis: %v", e)
				}
			}
		}
		_, _ = db.Exec(`DELETE FROM pay_components WHERE tenant_id=? AND id=?`, tenant, componentID)
		for _, query := range []string{`DELETE FROM login_events WHERE user_id=?`, `DELETE FROM sessions WHERE user_id=?`, `DELETE FROM user_roles WHERE user_id=?`, `DELETE FROM tenant_memberships WHERE user_id=?`, `DELETE FROM users WHERE id=?`} {
			if _, e := db.Exec(query, userID); e != nil {
				t.Errorf("cleanup finance user: %v", e)
			}
		}
	})
	if _, err = db.ExecContext(ctx, `INSERT INTO tenant_memberships (tenant_id,user_id,status) VALUES (?,?,'active')`, tenant, userID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO user_roles (tenant_id,id,user_id,role_id,valid_from) VALUES (?,?,?,?,CURRENT_DATE())`, tenant, roleAssignmentID, userID, roleID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO pay_components (tenant_id,id,code,name,component_type,taxable,calc_type,is_system,is_active,version) VALUES (?,?,?,'QA Gaji Pokok','earning',TRUE,'fixed',FALSE,TRUE,1)`, tenant, componentID, "QA_"+componentID); err != nil {
		t.Fatal(err)
	}
	financeJar, _ := cookiejar.New(nil)
	finance := &http.Client{Jar: financeJar, Timeout: 10 * time.Second}
	doJSON(t, finance, "POST", server.URL+"/api/v1/auth/login", map[string]string{"email": email, "password": password}, 200, nil)
	doJSON(t, finance, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": password}, 200, nil)
	var original struct {
		ID string
		To sql.NullTime
	}
	err = db.QueryRowContext(ctx, `SELECT id,effective_to FROM bpjs_wage_basis_policies WHERE tenant_id=? ORDER BY effective_from DESC LIMIT 1`, tenant).Scan(&original.ID, &original.To)
	if err != nil && err != sql.ErrNoRows {
		t.Fatal(err)
	}
	if err == nil {
		previousID, previousTo = original.ID, original.To
	}
	var response struct {
		Data struct {
			Configured        bool              `json:"configured"`
			Policy            bpjsWageBasisView `json:"policy"`
			SuggestedPrograms []bpjsWageProgram `json:"suggested_programs"`
		} `json:"data"`
	}
	url := server.URL + "/api/v1/admin/payroll/bpjs-wage-basis"
	doJSON(t, finance, "GET", url+"?date=2100-01-01", nil, 200, &response)
	if response.Data.Configured || len(response.Data.SuggestedPrograms) != 5 {
		t.Fatalf("unexpected default wage basis: %+v", response.Data)
	}
	minimums := []bpjsOfficeMinimum{}
	rows, err := db.QueryContext(ctx, `SELECT id FROM offices WHERE tenant_id=? AND is_active=TRUE AND deleted_at IS NULL`, tenant)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		minimums = append(minimums, bpjsOfficeMinimum{OfficeID: id, Amount: json.Number("3000000")})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil || len(minimums) == 0 {
		t.Fatalf("QA offices: %v, %d", err, len(minimums))
	}
	programs := []bpjsWageProgram{}
	for _, program := range requiredPayrollBPJSPrograms {
		programs = append(programs, bpjsWageProgram{Program: program, ComponentIDs: []string{componentID}})
	}
	input := map[string]any{"effective_from": "2100-01-01", "source_reference": "QA fixture only", "programs": programs, "office_minimums": minimums}
	var published struct {
		Data bpjsWageBasisView `json:"data"`
	}
	doJSON(t, finance, "POST", url, input, 201, &published)
	policyID = published.Data.ID
	if policyID == "" || len(published.Data.Programs) != 5 || len(published.Data.OfficeMinimums) != len(minimums) {
		t.Fatalf("published policy incomplete: %+v", published.Data)
	}
	doJSON(t, finance, "GET", url+"?date=2100-01-01", nil, 200, &response)
	if !response.Data.Configured || response.Data.Policy.ID != policyID {
		t.Fatalf("published policy not readable: %+v", response.Data)
	}
	doJSON(t, finance, "DELETE", server.URL+"/api/v1/admin/payroll/components/"+componentID, nil, 409, nil)
	doJSON(t, finance, "POST", url, input, 409, nil)
}
