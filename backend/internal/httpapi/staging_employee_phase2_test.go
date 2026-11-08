package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
)

func TestStagingEmployeePhase2LifecycleAndSensitiveData(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	if _, _, err := tenantMasterKey(); err != nil {
		t.Fatal("persistent tenant key required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
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
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, nil)
	var tenantID, officeID, positionID string
	err = db.QueryRow(`SELECT e.tenant_id,e.base_office_id,e.position_id FROM employees e JOIN tenants t ON t.id=e.tenant_id WHERE t.slug='client-pertama' AND e.deleted_at IS NULL LIMIT 1`).Scan(&tenantID, &officeID, &positionID)
	if err != nil {
		t.Fatal(err)
	}
	qaID, err := ids.New()
	if err != nil {
		t.Fatal(err)
	}
	code := "QA-" + qaID
	nik := "3173010909901234"
	root := server.URL + "/api/v1/admin/employees"
	var created employee
	doJSON(t, client, "POST", root, map[string]any{"employeeNumber": code, "displayName": "QA Phase2", "status": "active", "baseOfficeId": officeID, "positionId": positionID, "nickname": "QA", "join_date": "2026-01-01"}, 201, &created)
	id := created.ID
	var accountID string
	var bankID string
	t.Cleanup(func() {
		if accountID != "" {
			for _, step := range []string{
				`DELETE FROM password_reset_tokens WHERE user_id=?`,
				`DELETE FROM sessions WHERE user_id=?`,
				`DELETE FROM user_roles WHERE user_id=?`,
			} {
				if _, e := db.Exec(step, accountID); e != nil {
					t.Errorf("cleanup account: %v", e)
				}
			}
			if _, e := db.Exec(`UPDATE employees SET user_id=NULL WHERE tenant_id=? AND id=?`, tenantID, id); e != nil {
				t.Error(e)
			}
			if _, e := db.Exec(`DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, tenantID, accountID); e != nil {
				t.Error(e)
			}
			if _, e := db.Exec(`DELETE FROM users WHERE id=?`, accountID); e != nil {
				t.Error(e)
			}
		}
		for _, step := range []string{
			`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employees' AND entity_id=?`,
			`DELETE FROM employee_bank_accounts WHERE tenant_id=? AND employee_id=?`,
			`DELETE FROM employee_assignments WHERE tenant_id=? AND employee_id=?`,
			`DELETE FROM employee_assignment_history WHERE tenant_id=? AND employee_id=?`,
			`DELETE FROM employees WHERE tenant_id=? AND id=?`,
		} {
			if _, e := db.Exec(step, tenantID, id); e != nil {
				t.Errorf("cleanup employee: %v", e)
			}
		}
		if bankID != "" {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employee_bank_accounts' AND entity_id IN (?,?)`, tenantID, bankID, id); e != nil {
				t.Errorf("cleanup bank audit: %v", e)
			}
		}
	})
	if id == "" {
		t.Fatal("missing employee ID")
	}
	essJar, _ := cookiejar.New(nil)
	essClient := &http.Client{Jar: essJar, Timeout: 10 * time.Second}
	doJSON(t, essClient, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, nil)
	doJSON(t, essClient, "GET", root+"/"+id, nil, 403, nil)
	doJSON(t, essClient, "GET", root+"/"+id+"/bank-accounts", nil, 403, nil)
	doJSON(t, client, "GET", root+"/"+qaID, nil, 404, nil)
	var detail map[string]any
	doJSON(t, client, "GET", root+"/"+id, nil, 200, &detail)
	if detail["nickname"] != "QA" {
		t.Fatalf("personal detail: %#v", detail)
	}
	doJSON(t, client, "PATCH", root+"/"+id, map[string]any{"nik": nik}, 403, nil)
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "password"}, 200, nil)
	doJSON(t, client, "PATCH", root+"/"+id, map[string]any{"nik": nik, "mobile": "08123456789"}, 200, nil)
	var cipher []byte
	if err = db.QueryRow(`SELECT nik_enc FROM employees WHERE tenant_id=? AND id=?`, tenantID, id).Scan(&cipher); err != nil || string(cipher) == nik {
		t.Fatalf("NIK plaintext or missing: %v", err)
	}
	doJSON(t, client, "GET", root+"/"+id, nil, 200, &detail)
	if detail["nik"] != nik || detail["mobile"] != "08123456789" {
		t.Fatalf("personal roundtrip: %#v", detail)
	}
	var leaked int
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE tenant_id=? AND entity_type='employees' AND entity_id=? AND (CAST(before_json AS CHAR) LIKE ? OR CAST(after_json AS CHAR) LIKE ?)`, tenantID, id, "%"+nik+"%", "%"+nik+"%").Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("NIK leaked to audit: %d %v", leaked, err)
	}
	bankURL := root + "/" + id + "/bank-accounts"
	var bankWrite struct {
		Data struct {
			ID      string `json:"id"`
			Version uint64 `json:"version"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", bankURL, map[string]any{"bank_code": "BCA", "account_no": "123456789012", "holder_name": "QA Holder", "is_primary": true, "valid_from": "2026-01-01"}, 201, &bankWrite)
	bankID = bankWrite.Data.ID
	if bankID == "" {
		t.Fatal("missing bank ID")
	}
	doJSON(t, essClient, "GET", bankURL, nil, 403, nil)
	if err = db.QueryRow(`SELECT account_no_enc FROM employee_bank_accounts WHERE tenant_id=? AND id=?`, tenantID, bankID).Scan(&cipher); err != nil || string(cipher) == "123456789012" {
		t.Fatalf("bank plaintext or missing: %v", err)
	}
	var bankList struct {
		Data struct {
			Accounts []employeeBankAccount `json:"accounts"`
		} `json:"data"`
	}
	doJSON(t, client, "GET", bankURL, nil, 200, &bankList)
	if len(bankList.Data.Accounts) != 1 || bankList.Data.Accounts[0].AccountNo == nil || *bankList.Data.Accounts[0].AccountNo != "123456789012" {
		t.Fatalf("bank detail: %#v", bankList)
	}
	doJSON(t, client, "PATCH", bankURL+"/"+bankID, map[string]any{"version": 1, "holder_name": "QA Updated"}, 200, &bankWrite)
	doJSON(t, client, "PATCH", bankURL+"/"+bankID, map[string]any{"version": 1, "holder_name": "QA Again"}, 409, nil)
	doJSON(t, client, "DELETE", bankURL+"/"+bankID, nil, 204, nil)
	var assignments struct {
		Data any `json:"data"`
	}
	doJSON(t, client, "GET", root+"/"+id+"/assignments", nil, 200, &assignments)
	var currentAssignment string
	if err = db.QueryRow(`SELECT id FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=1`, tenantID, id).Scan(&currentAssignment); err != nil {
		t.Fatal(err)
	}
	today := tenantToday("Asia/Jakarta")
	doJSON(t, client, "POST", root+"/"+id+"/assignments", map[string]any{"current_assignment_id": currentAssignment, "effective_from": today, "office_id": officeID, "position_id": positionID, "reason": "Koreksi data QA staging"}, 201, nil)
	var assignmentCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM employee_assignments WHERE tenant_id=? AND employee_id=?`, tenantID, id).Scan(&assignmentCount); err != nil || assignmentCount != 2 {
		t.Fatalf("assignment history: %d %v", assignmentCount, err)
	}
	doJSON(t, client, "GET", root+"/"+id+"/summary", nil, 200, nil)
	doJSON(t, client, "GET", root+"/"+id+"/timeline", nil, 200, nil)
	var next struct {
		Data struct {
			Suggestion string `json:"suggestion"`
		} `json:"data"`
	}
	doJSON(t, client, "GET", root+"/next-number", nil, 200, &next)
	if !strings.HasPrefix(next.Data.Suggestion, "EMP-") {
		t.Fatalf("next number: %#v", next)
	}
	t.Setenv("SMTP_HOST", "127.0.0.1")
	t.Setenv("SMTP_PORT", "1")
	t.Setenv("SMTP_FROM", "admin@digaji.id")
	t.Setenv("APP_ORIGIN", server.URL)
	accountEmail := strings.ToLower("qa-" + qaID + "@example.invalid")
	var linked struct {
		Data struct {
			UserID           string `json:"user_id"`
			InvitationStatus string `json:"invitation_status"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", root+"/"+id+"/account", map[string]string{"email": accountEmail}, 201, &linked)
	accountID = linked.Data.UserID
	if accountID == "" || linked.Data.InvitationStatus != "failed" {
		t.Fatalf("account link: %#v", linked)
	}
	var roleCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM user_roles ur JOIN roles ro ON ro.tenant_id=ur.tenant_id AND ro.id=ur.role_id WHERE ur.tenant_id=? AND ur.user_id=? AND ro.code='KARYAWAN'`, tenantID, accountID).Scan(&roleCount); err != nil || roleCount != 1 {
		t.Fatalf("ESS role: %d %v", roleCount, err)
	}
	doJSON(t, client, "POST", root+"/"+id+"/account", map[string]string{"email": accountEmail}, 200, nil)
	doJSON(t, client, "DELETE", root+"/"+id, nil, 409, nil)
	doJSON(t, client, "POST", root+"/bulk-archive", map[string]any{"ids": []string{id}}, 409, nil)
	// Fixture transition only: production termination is handled in phase 6.
	if _, err = db.Exec(`UPDATE employees SET status='resigned' WHERE tenant_id=? AND id=?`, tenantID, id); err != nil {
		t.Fatal(err)
	}
	doJSON(t, client, "GET", root+"/"+id, nil, 200, &detail)
	doJSON(t, client, "POST", root+"/bulk-archive", map[string]any{"ids": []string{id}}, 200, nil)
	if err = db.QueryRow(`SELECT COUNT(*) FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=1`, tenantID, id).Scan(&assignmentCount); err != nil || assignmentCount != 0 {
		t.Fatalf("open assignment after archive: %d %v", assignmentCount, err)
	}
	doJSON(t, client, "GET", root+"/"+id, nil, 200, &detail)
	var archivedPage pageResult[employee]
	doJSON(t, client, "GET", root+"?q="+code+"&archive=archived", nil, 200, &archivedPage)
	if archivedPage.Total != 1 || archivedPage.Items[0].ID != id {
		t.Fatalf("archived list: %#v", archivedPage)
	}
	doJSON(t, client, "DELETE", root+"/"+id, nil, 204, nil)
	doJSON(t, client, "POST", root+"/"+id+"/restore", map[string]any{}, 200, nil)
	var archived sql.NullTime
	if err = db.QueryRow(`SELECT deleted_at FROM employees WHERE tenant_id=? AND id=?`, tenantID, id).Scan(&archived); err != nil || archived.Valid {
		t.Fatalf("restore failed: %v", err)
	}
}
