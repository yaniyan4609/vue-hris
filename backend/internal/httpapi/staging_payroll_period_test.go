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
	periodID, loanID, componentID, payEmployeeID, payOrgAssignmentID := "", "", "", "", ""
	t.Cleanup(func() {
		if payEmployeeID != "" {
			for _, query := range []string{
				`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employee_pay_components' AND entity_id=?`,
				`DELETE FROM employee_pay_components WHERE tenant_id=? AND employee_id=?`,
				`DELETE FROM employee_pay_set_versions WHERE tenant_id=? AND employee_id=?`,
			} {
				if _, e := db.Exec(query, tenantID, payEmployeeID); e != nil {
					t.Errorf("cleanup employee pay: %v", e)
				}
			}
		}
		if componentID != "" {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='pay_components' AND entity_id=?`, tenantID, componentID); e != nil {
				t.Errorf("cleanup component audit: %v", e)
			}
			if _, e := db.Exec(`DELETE FROM pay_components WHERE tenant_id=? AND id=?`, tenantID, componentID); e != nil {
				t.Errorf("cleanup component: %v", e)
			}
		}
		if loanID != "" {
			if _, e := db.Exec(`DELETE FROM loan_installments WHERE tenant_id=? AND loan_id=?`, tenantID, loanID); e != nil {
				t.Errorf("cleanup loan installments: %v", e)
			}
			if _, e := db.Exec(`DELETE FROM employee_loans WHERE tenant_id=? AND id=?`, tenantID, loanID); e != nil {
				t.Errorf("cleanup loan: %v", e)
			}
		}
		if periodID != "" {
			_, _ = db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='payroll_periods' AND entity_id=?`, tenantID, periodID)
			if _, e := db.Exec(`DELETE FROM payroll_periods WHERE tenant_id=? AND id=?`, tenantID, periodID); e != nil {
				t.Errorf("cleanup payroll period: %v", e)
			}
		}
		if payOrgAssignmentID != "" {
			if _, e := db.Exec(`DELETE FROM employee_assignments WHERE tenant_id=? AND id=?`, tenantID, payOrgAssignmentID); e != nil {
				t.Errorf("cleanup organization assignment: %v", e)
			}
		}
		if payEmployeeID != "" {
			if _, e := db.Exec(`DELETE FROM employees WHERE tenant_id=? AND id=?`, tenantID, payEmployeeID); e != nil {
				t.Errorf("cleanup QA employee: %v", e)
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
	server := httptest.NewServer(Router(db, Options{CookieSecure: false, DemoLogin: true}))
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
	var employeeID string
	if err = db.QueryRow(`SELECT e.id FROM employees e JOIN users u ON u.id=e.user_id WHERE e.tenant_id=? AND u.email_normalized='ess@digaji.id'`, tenantID).Scan(&employeeID); err != nil {
		t.Fatal(err)
	}
	loanID, _ = ids.New()
	lineID, _ := ids.New()
	nextLineID, _ := ids.New()
	dueDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	if _, err = db.Exec(`INSERT INTO employee_loans (tenant_id,id,employee_id,amount,tenor_months,installment_amount,outstanding,status,disbursed_at,disbursement_ref) VALUES (?,?,?,'1000.00',2,'500.00','1000.00','disbursed',UTC_TIMESTAMP(6),?)`, tenantID, loanID, employeeID, "QA-PAYROLL-"+loanID); err != nil {
		t.Fatal(err)
	}
	for _, line := range []struct {
		id, due string
		seq     int
	}{
		{lineID, dueDate.Format("2006-01-02"), 1},
		{nextLineID, dueDate.AddDate(0, 1, 0).Format("2006-01-02"), 2},
	} {
		if _, err = db.Exec(`INSERT INTO loan_installments (tenant_id,id,loan_id,seq,due_period,amount,status) VALUES (?,?,?,?,?,'500.00','pending')`, tenantID, line.id, loanID, line.seq, line.due); err != nil {
			t.Fatal(err)
		}
	}
	simulateURL := server.URL + "/api/v1/admin/payroll/periods/" + periodID + "/loan-deductions/simulate"
	var simulation struct {
		Data struct {
			Planned   []payrollLoanCandidate `json:"planned"`
			Deferred  []payrollLoanDeferred  `json:"deferred"`
			Remaining string                 `json:"remaining_net"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", simulateURL, map[string]any{"employee_id": employeeID, "available_net": 499.99}, 200, &simulation)
	if len(simulation.Data.Planned) != 0 || len(simulation.Data.Deferred) != 1 || simulation.Data.Deferred[0].Reason != "INSUFFICIENT_NET" || simulation.Data.Remaining != "499.99" {
		t.Fatalf("insufficient payroll net: %#v", simulation.Data)
	}
	doJSON(t, client, "POST", simulateURL, map[string]any{"employee_id": employeeID, "available_net": 500}, 200, &simulation)
	if len(simulation.Data.Planned) != 1 || len(simulation.Data.Deferred) != 0 || simulation.Data.Remaining != "0.00" {
		t.Fatalf("exact payroll net: %#v", simulation.Data)
	}
	doJSON(t, client, "POST", simulateURL, map[string]any{"employee_id": userID, "available_net": 500}, 404, nil)
	var outstanding, lineStatus string
	if err = db.QueryRow(`SELECT CAST(outstanding AS CHAR) FROM employee_loans WHERE tenant_id=? AND id=?`, tenantID, loanID).Scan(&outstanding); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT status FROM loan_installments WHERE tenant_id=? AND id=?`, tenantID, lineID).Scan(&lineStatus); err != nil {
		t.Fatal(err)
	}
	if outstanding != "1000.00" || lineStatus != "pending" {
		t.Fatalf("simulation changed loan: outstanding=%s installment=%s", outstanding, lineStatus)
	}
	componentURL := server.URL + "/api/v1/admin/payroll/components"
	componentCode := "QA_" + userID
	var component struct {
		Data payrollComponentView `json:"data"`
	}
	componentInput := map[string]any{"code": componentCode, "name": "Tunjangan QA", "component_type": "earning", "taxable": true, "calc_type": "fixed"}
	doJSON(t, client, "POST", componentURL, componentInput, 201, &component)
	componentID = component.Data.ID
	if component.Data.Version != 1 || !component.Data.IsActive {
		t.Fatalf("component create: %#v", component.Data)
	}
	doJSON(t, client, "POST", componentURL, componentInput, 409, nil)
	doJSON(t, client, "GET", componentURL+"/"+componentID, nil, 200, nil)
	doJSON(t, client, "PATCH", componentURL+"/"+componentID, map[string]any{"version": 1, "name": "Tunjangan QA Baru"}, 200, &component)
	doJSON(t, client, "PATCH", componentURL+"/"+componentID, map[string]any{"version": 1, "name": "Versi Lama"}, 409, nil)
	payEmployeeID, _ = ids.New()
	payOrgAssignmentID, _ = ids.New()
	if _, err = db.Exec(`INSERT INTO employees (tenant_id,id,employee_number,display_name,status,base_office_id,position_id) SELECT ?,? ,?,'QA Payroll Employee','active',base_office_id,position_id FROM employees WHERE tenant_id=? AND id=?`, tenantID, payEmployeeID, "QA-"+payEmployeeID, tenantID, employeeID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,valid_from,current_flag) SELECT ?,?,?,region_id,office_id,division_id,department_id,position_id,UTC_TIMESTAMP(6),1 FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=TRUE`, tenantID, payOrgAssignmentID, payEmployeeID, tenantID, employeeID); err != nil {
		t.Fatal(err)
	}
	payURL := server.URL + "/api/v1/admin/employees/" + payEmployeeID + "/pay-components"
	assignmentInput := map[string]any{"valid_from": "2100-01-01", "version": 0, "components": []map[string]any{{"component_id": componentID, "amount": 100}}}
	doJSON(t, client, "PUT", payURL, assignmentInput, 403, nil)
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": password}, 200, nil)
	doJSON(t, client, "PUT", payURL, assignmentInput, 200, nil)
	doJSON(t, client, "PUT", payURL, assignmentInput, 409, nil)
	var payDetail struct {
		Data struct {
			Version    uint64                     `json:"version"`
			Components []employeePayComponentView `json:"components"`
		} `json:"data"`
	}
	doJSON(t, client, "GET", payURL, nil, 200, &payDetail)
	if payDetail.Data.Version != 1 || len(payDetail.Data.Components) != 1 || payDetail.Data.Components[0].Amount == nil || *payDetail.Data.Components[0].Amount != "100.00" {
		t.Fatalf("employee pay detail: %#v", payDetail.Data)
	}
	nextAssignment := map[string]any{"valid_from": "2100-02-01", "version": 1, "components": []map[string]any{{"component_id": componentID, "amount": 125}}}
	doJSON(t, client, "PUT", payURL, nextAssignment, 200, nil)
	doJSON(t, client, "PUT", payURL, nextAssignment, 409, nil)
	doJSON(t, client, "GET", payURL, nil, 200, &payDetail)
	if payDetail.Data.Version != 2 || len(payDetail.Data.Components) != 2 || payDetail.Data.Components[0].Amount == nil || *payDetail.Data.Components[0].Amount != "125.00" || payDetail.Data.Components[1].ValidTo == nil || *payDetail.Data.Components[1].ValidTo != "2100-01-31" {
		t.Fatalf("employee pay history: %#v", payDetail.Data)
	}
	adminJar, _ := cookiejar.New(nil)
	adminClient := &http.Client{Jar: adminJar, Timeout: 10 * time.Second}
	doJSON(t, adminClient, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, nil)
	doJSON(t, adminClient, "GET", payURL, nil, 200, &payDetail)
	if len(payDetail.Data.Components) != 2 || payDetail.Data.Components[0].Amount != nil || payDetail.Data.Components[1].Amount != nil {
		t.Fatalf("salary visible without sensitive permission: %#v", payDetail.Data)
	}
	doJSON(t, adminClient, "PUT", payURL, assignmentInput, 403, nil)
	doJSON(t, client, "DELETE", componentURL+"/"+componentID, nil, 409, nil)
	if _, err = db.Exec(`DELETE FROM employee_pay_components WHERE tenant_id=? AND employee_id=?`, tenantID, payEmployeeID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, client, "DELETE", componentURL+"/"+componentID, nil, 204, nil)
	doJSON(t, client, "DELETE", componentURL+"/"+componentID, nil, 204, nil)
	doJSON(t, client, "GET", componentURL+"/"+componentID, nil, 200, &component)
	if component.Data.IsActive {
		t.Fatalf("component still active: %#v", component.Data)
	}
}
