package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
)

func TestStagingEmployeeTaxBPJSVersionAndEncryption(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	if _, _, err := tenantMasterKey(); err != nil {
		t.Fatal("persistent tenant master key required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var tenantID, sourceID string
	if err = db.QueryRow(`SELECT e.tenant_id,e.id FROM employees e JOIN tenants t ON t.id=e.tenant_id WHERE t.slug='client-pertama' AND e.status='active' LIMIT 1`).Scan(&tenantID, &sourceID); err != nil {
		t.Fatal(err)
	}
	employeeID, _ := ids.New()
	assignmentID, _ := ids.New()
	if _, err = db.Exec(`INSERT INTO employees (tenant_id,id,employee_number,display_name,status,base_office_id,position_id) SELECT ?,?,?, 'QA Tax BPJS','active',base_office_id,position_id FROM employees WHERE tenant_id=? AND id=?`, tenantID, employeeID, "QA-"+employeeID, tenantID, sourceID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, query := range []string{
			`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employee_tax_bpjs' AND (entity_id=? OR JSON_UNQUOTE(JSON_EXTRACT(after_json,'$.employee_id'))=?)`,
			`DELETE FROM employee_tax_bpjs WHERE tenant_id=? AND employee_id=?`,
			`DELETE FROM employee_assignments WHERE tenant_id=? AND employee_id=?`,
			`DELETE FROM employees WHERE tenant_id=? AND id=?`,
		} {
			var e error
			if query == `DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employee_tax_bpjs' AND (entity_id=? OR JSON_UNQUOTE(JSON_EXTRACT(after_json,'$.employee_id'))=?)` {
				_, e = db.Exec(query, tenantID, employeeID, employeeID)
			} else {
				_, e = db.Exec(query, tenantID, employeeID)
			}
			if e != nil {
				t.Errorf("cleanup QA tax profile: %v", e)
			}
		}
	})
	if _, err = db.Exec(`INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,valid_from,current_flag) SELECT ?,?,?,region_id,office_id,division_id,department_id,position_id,UTC_TIMESTAMP(6),1 FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=TRUE`, tenantID, assignmentID, employeeID, tenantID, sourceID); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(Router(db, Options{CookieSecure: false, DemoLogin: true}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	url := server.URL + "/api/v1/admin/employees/" + employeeID + "/tax-bpjs"
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, nil)
	doJSON(t, client, "PUT", url, map[string]any{"version": "", "valid_from": "2098-01-01", "ptkp_status": "TK/0"}, 403, nil)
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "password"}, 200, nil)
	input := map[string]any{"version": "", "valid_from": "2098-01-01", "ptkp_status": "TK/0", "npwp": "1234567890123456", "bpjs_kes_no": "1234567890123", "bpjs_tk_no": "9876543210123"}
	var written struct {
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	doJSON(t, client, "PUT", url, input, 200, &written)
	if written.Data.Version == "" {
		t.Fatal("missing profile version")
	}
	var ciphertext []byte
	if err = db.QueryRow(`SELECT npwp_enc FROM employee_tax_bpjs WHERE tenant_id=? AND employee_id=?`, tenantID, employeeID).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if string(ciphertext) == input["npwp"] {
		t.Fatal("NPWP stored in plaintext")
	}
	var detail struct {
		Data struct {
			Version  string `json:"version"`
			Profiles []struct {
				ValidTo *string `json:"valid_to"`
				NPWP    *string `json:"npwp"`
			} `json:"profiles"`
		} `json:"data"`
	}
	doJSON(t, client, "GET", url, nil, 200, &detail)
	if detail.Data.Version != written.Data.Version || len(detail.Data.Profiles) != 1 || detail.Data.Profiles[0].NPWP == nil || *detail.Data.Profiles[0].NPWP != input["npwp"] {
		t.Fatalf("tax profile round trip failed: %#v", detail.Data)
	}
	doJSON(t, client, "PUT", url, input, 409, nil)
	input["version"] = written.Data.Version
	input["valid_from"] = "2098-02-01"
	input["npwp"] = nil
	doJSON(t, client, "PUT", url, input, 200, &written)
	doJSON(t, client, "GET", url, nil, 200, &detail)
	if len(detail.Data.Profiles) != 2 || detail.Data.Profiles[0].NPWP != nil || detail.Data.Profiles[1].ValidTo == nil || *detail.Data.Profiles[1].ValidTo != "2098-01-31" {
		t.Fatalf("tax profile versioning failed: %#v", detail.Data)
	}
	masterKey := os.Getenv("TENANT_MASTER_KEY")
	defer os.Setenv("TENANT_MASTER_KEY", masterKey)
	if err = os.Unsetenv("TENANT_MASTER_KEY"); err != nil {
		t.Fatal(err)
	}
	doJSON(t, client, "GET", url, nil, 503, nil)
	if err = os.Setenv("TENANT_MASTER_KEY", masterKey); err != nil {
		t.Fatal(err)
	}
	var leaked int
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE tenant_id=? AND entity_type='employee_tax_bpjs' AND CAST(after_json AS CHAR) LIKE '%1234567890123456%'`, tenantID).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("sensitive number leaked to audit: count=%d err=%v", leaked, err)
	}
	essJar, _ := cookiejar.New(nil)
	ess := &http.Client{Jar: essJar, Timeout: 10 * time.Second}
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, nil)
	doJSON(t, ess, "GET", url, nil, 403, nil)
	var periodID string
	err = db.QueryRow(`SELECT id FROM payroll_periods WHERE tenant_id=? AND period_year=2098 AND period_month=1`, tenantID).Scan(&periodID)
	if errors.Is(err, sql.ErrNoRows) {
		periodID, _ = ids.New()
		if _, err = db.Exec(`INSERT INTO payroll_periods (tenant_id,id,period_year,period_month,start_date,end_date) VALUES (?,?,2098,1,'2098-01-01','2098-01-31')`, tenantID, periodID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, e := db.Exec(`DELETE FROM payroll_periods WHERE tenant_id=? AND id=?`, tenantID, periodID); e != nil {
				t.Errorf("cleanup QA payroll period: %v", e)
			}
		})
	} else if err != nil {
		t.Fatal(err)
	}
	var preflight struct {
		Data struct {
			Checks map[string]bool `json:"checks"`
		} `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/payroll/periods/"+periodID+"/preflight?employee_id="+employeeID, nil, 200, &preflight)
	if len(preflight.Data.Checks) != 7 || !preflight.Data.Checks["profile_covered"] || !preflight.Data.Checks["npwp_recorded"] {
		t.Fatalf("incomplete tax/BPJS preflight: %#v", preflight.Data.Checks)
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/payroll/periods/"+periodID+"/preflight?employee_id="+written.Data.Version, nil, 404, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/employees/"+written.Data.Version+"/tax-bpjs", nil, 404, nil)
}
