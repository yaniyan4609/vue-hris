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

func TestStagingLoanWorkflow(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	newClient := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, Timeout: 10 * time.Second}
	}
	admin, ess := newClient(), newClient()
	var adminState auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &adminState)
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, nil)
	tenantID := adminState.SelectedTenantID
	var policyCount, flowCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM loan_policies WHERE tenant_id=?`, tenantID).Scan(&policyCount); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM approval_flows WHERE tenant_id=? AND request_type='loan' AND is_default=TRUE`, tenantID).Scan(&flowCount); err != nil {
		t.Fatal(err)
	}
	if policyCount != 0 || flowCount != 0 {
		t.Skip("tenant already has loan configuration")
	}
	flowID, stepID, loanID, requestID := "", "", "", ""
	t.Cleanup(func() {
		if requestID == "" && loanID != "" {
			_ = db.QueryRow(`SELECT request_id FROM employee_loans WHERE tenant_id=? AND id=?`, tenantID, loanID).Scan(&requestID)
		}
		if requestID != "" {
			var instanceID string
			_ = db.QueryRow(`SELECT id FROM approval_instances WHERE tenant_id=? AND request_id=?`, tenantID, requestID).Scan(&instanceID)
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='loan_installments' AND entity_id IN (SELECT id FROM loan_installments WHERE tenant_id=? AND loan_id=?)`, tenantID, tenantID, loanID); e != nil {
				t.Errorf("cleanup waiver audit: %v", e)
			}
			for _, entry := range []struct{ q, id string }{
				{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='requests' AND entity_id=?`, requestID},
				{`DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='request' AND aggregate_id=?`, requestID},
				{`DELETE FROM request_revisions WHERE tenant_id=? AND request_id=?`, requestID},
				{`DELETE FROM approval_actions WHERE tenant_id=? AND instance_id=?`, instanceID},
				{`DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id=?`, instanceID},
				{`DELETE FROM approval_instances WHERE tenant_id=? AND id=?`, instanceID},
				{`DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='loan' AND aggregate_id=?`, loanID},
				{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employee_loans' AND entity_id=?`, loanID},
				{`DELETE FROM loan_settlements WHERE tenant_id=? AND loan_id=?`, loanID},
				{`DELETE FROM loan_installments WHERE tenant_id=? AND loan_id=?`, loanID},
				{`DELETE FROM employee_loans WHERE tenant_id=? AND id=?`, loanID},
				{`DELETE FROM requests WHERE tenant_id=? AND id=?`, requestID},
			} {
				if _, e := db.Exec(entry.q, tenantID, entry.id); e != nil {
					t.Errorf("cleanup loan: %v", e)
				}
			}
		}
		if flowID != "" {
			if _, e := db.Exec(`DELETE FROM approval_flow_steps WHERE tenant_id=? AND flow_id=?`, tenantID, flowID); e != nil {
				t.Errorf("cleanup steps: %v", e)
			}
			if _, e := db.Exec(`DELETE FROM approval_flows WHERE tenant_id=? AND id=?`, tenantID, flowID); e != nil {
				t.Errorf("cleanup flow: %v", e)
			}
		}
		if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='loan_policies' AND entity_id=?`, tenantID, tenantID); e != nil {
			t.Errorf("cleanup policy audit: %v", e)
		}
		if _, e := db.Exec(`DELETE FROM loan_policies WHERE tenant_id=?`, tenantID); e != nil {
			t.Errorf("cleanup policy: %v", e)
		}
	})
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/loans/simulate", map[string]any{"amount": 1000, "tenor_months": 3}, 409, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/loan-policies", nil, 200, nil)
	doJSON(t, admin, "PUT", server.URL+"/api/v1/admin/loan-policies", map[string]any{"max_outstanding": 10000, "max_tenor_months": 6, "version": 0}, 403, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "password"}, 200, nil)
	doJSON(t, admin, "PUT", server.URL+"/api/v1/admin/loan-policies", map[string]any{"max_outstanding": 10000, "max_tenor_months": 6, "version": 0}, 200, nil)
	doJSON(t, admin, "PUT", server.URL+"/api/v1/admin/loan-policies", map[string]any{"max_outstanding": 10000, "max_tenor_months": 6, "version": 0}, 409, nil)
	flowID, _ = ids.New()
	stepID, _ = ids.New()
	if _, err = db.Exec(`INSERT INTO approval_flows (tenant_id,id,request_type,name,is_default,is_active,version,valid_from) VALUES (?,?,'loan','QA Loan',TRUE,TRUE,1,'2000-01-01')`, tenantID, flowID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO approval_flow_steps (tenant_id,id,flow_id,level_no,approver_kind,approver_ref_id,required_permission) VALUES (?,?,?,1,'user',?,'kasbon.approve')`, tenantID, stepID, flowID, adminState.User.ID); err != nil {
		t.Fatal(err)
	}
	var simulation struct {
		Data struct {
			Installment string `json:"installment_amount"`
			Last        string `json:"last_installment_amount"`
		} `json:"data"`
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/loans/simulate", map[string]any{"amount": 1000, "tenor_months": 3}, 200, &simulation)
	if simulation.Data.Installment != "333.33" || simulation.Data.Last != "333.34" {
		t.Fatalf("simulation rounding: %#v", simulation.Data)
	}
	var submitted struct {
		Data struct {
			ID        string `json:"id"`
			RequestID string `json:"request_id"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/loans", map[string]any{"amount": 1000, "tenor_months": 3}, 201, &submitted)
	loanID, requestID = submitted.Data.ID, submitted.Data.RequestID
	if submitted.Data.Status != "in_review" {
		t.Fatalf("loan status: %s", submitted.Data.Status)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/loans", map[string]any{"amount": 9001, "tenor_months": 3}, 409, nil)
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/loans/"+loanID, nil, 200, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/loans/"+loanID, nil, 200, nil)
	actionURL := server.URL + "/api/v1/admin/approvals/" + requestID + "/actions"
	doJSON(t, admin, "POST", actionURL, map[string]any{"action": "return", "reason": "Perjelas kebutuhan", "level_no": 1, "version": 1}, 200, nil)
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+requestID+"/resubmit", map[string]any{"version": 2, "correction": map[string]any{"note": "Kebutuhan telah dijelaskan"}}, 200, nil)
	doJSON(t, admin, "POST", actionURL, map[string]any{"action": "approve", "level_no": 1, "version": 3}, 200, nil)
	uid, _ := ids.New()
	ref := "QA-LOAN-" + uid
	url := server.URL + "/api/v1/admin/loans/" + loanID + "/disburse"
	doJSON(t, admin, "POST", url, map[string]string{"disbursement_ref": ref}, 200, nil)
	doJSON(t, admin, "POST", url, map[string]string{"disbursement_ref": ref}, 200, nil)
	doJSON(t, admin, "POST", url, map[string]string{"disbursement_ref": "OTHER-" + uid}, 409, nil)
	var detail struct {
		Data struct {
			Loan         loanView              `json:"loan"`
			Installments []loanInstallmentView `json:"installments"`
			Settlement   *loanSettlementView   `json:"settlement"`
		} `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/loans/"+loanID, nil, 200, &detail)
	if detail.Data.Loan.Status != "disbursed" || len(detail.Data.Installments) != 3 {
		t.Fatalf("loan detail: %#v", detail.Data)
	}
	var sum int64
	for _, line := range detail.Data.Installments {
		value, ok := leaveHundredths(line.Amount)
		if !ok {
			t.Fatalf("invalid installment: %s", line.Amount)
		}
		sum += value
	}
	if sum != 100000 {
		t.Fatalf("installments sum: %d", sum)
	}
	zone, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(zone)
	want := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, zone).Format("2006-01-02")
	if detail.Data.Installments[0].DuePeriod != want {
		t.Fatalf("first due period: %s want %s", detail.Data.Installments[0].DuePeriod, want)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "password"}, 200, nil)
	waiveURL := server.URL + "/api/v1/admin/loans/" + loanID + "/installments/1/waive"
	doJSON(t, admin, "POST", waiveURL, map[string]string{"reason": "Pembebasan QA"}, 200, nil)
	doJSON(t, admin, "POST", waiveURL, map[string]string{"reason": "Pembebasan QA"}, 409, nil)
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/loans/"+loanID, nil, 200, &detail)
	if detail.Data.Loan.Outstanding != "666.67" || detail.Data.Installments[0].Status != "waived" || detail.Data.Installments[0].WaiveReason == nil {
		t.Fatalf("waiver detail: %#v", detail.Data)
	}
	settlementRef := "SETTLE-" + uid
	settleURL := server.URL + "/api/v1/admin/loans/" + loanID + "/settle"
	doJSON(t, admin, "POST", settleURL, map[string]string{"settlement_ref": settlementRef}, 200, nil)
	doJSON(t, admin, "POST", settleURL, map[string]string{"settlement_ref": settlementRef}, 200, nil)
	doJSON(t, admin, "POST", settleURL, map[string]string{"settlement_ref": "OTHER-" + uid}, 409, nil)
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/loans/"+loanID, nil, 200, &detail)
	if detail.Data.Loan.Status != "settled" || detail.Data.Loan.Outstanding != "0.00" || detail.Data.Loan.ClosedAt == nil || detail.Data.Settlement == nil || detail.Data.Settlement.Amount != "666.67" || detail.Data.Installments[1].Status != "settled" || detail.Data.Installments[2].Status != "settled" {
		t.Fatalf("settlement detail: %#v", detail.Data)
	}
}
