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

func TestStagingLeaveCrossYearWorkflow(t *testing.T) {
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
	client := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, Timeout: 10 * time.Second}
	}
	admin, ess := client(), client()
	var adminState, essState auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &adminState)
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &essState)
	tenantID := essState.SelectedTenantID
	var employeeID, officeID string
	if err := db.QueryRow(`SELECT e.id,e.base_office_id FROM employees e WHERE e.tenant_id=? AND e.user_id=?`, tenantID, essState.User.ID).Scan(&employeeID, &officeID); err != nil {
		t.Fatal(err)
	}
	newID := func() string {
		id, e := ids.New()
		if e != nil {
			t.Fatal(e)
		}
		return id
	}
	typeID, policyID, flowID, stepID, shiftID := newID(), newID(), newID(), newID(), newID()
	ledgerIDs := []string{newID(), newID()}
	year := time.Now().Year() + 3
	start, end := fmt.Sprintf("%04d-12-30", year), fmt.Sprintf("%04d-01-02", year+1)
	dates := []string{start, fmt.Sprintf("%04d-12-31", year), fmt.Sprintf("%04d-01-01", year+1), end}
	scheduleIDs := []string{newID(), newID(), newID(), newID()}
	var leaveID, requestID string
	t.Cleanup(func() {
		if requestID != "" {
			var instanceID string
			_ = db.QueryRow(`SELECT id FROM approval_instances WHERE tenant_id=? AND request_id=?`, tenantID, requestID).Scan(&instanceID)
			for _, query := range []string{
				`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='requests' AND entity_id=?`,
				`DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='request' AND aggregate_id=?`,
				`DELETE FROM request_revisions WHERE tenant_id=? AND request_id=?`,
			} {
				if _, e := db.Exec(query, tenantID, requestID); e != nil {
					t.Errorf("cleanup request: %v", e)
				}
			}
			if instanceID != "" {
				for _, query := range []string{`DELETE FROM approval_actions WHERE tenant_id=? AND instance_id=?`, `DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id=?`, `DELETE FROM approval_instances WHERE tenant_id=? AND id=?`} {
					if _, e := db.Exec(query, tenantID, instanceID); e != nil {
						t.Errorf("cleanup approval: %v", e)
					}
				}
			}
			if leaveID != "" {
				for _, query := range []string{`DELETE FROM leave_balance_ledger WHERE tenant_id=? AND ref_type='leave_request_allocation' AND ref_id IN (SELECT id FROM leave_request_allocations WHERE tenant_id=? AND leave_request_id=?)`, `DELETE FROM leave_request_allocations WHERE tenant_id=? AND leave_request_id=?`, `DELETE FROM leave_requests WHERE tenant_id=? AND id=?`} {
					if query == `DELETE FROM leave_balance_ledger WHERE tenant_id=? AND ref_type='leave_request_allocation' AND ref_id IN (SELECT id FROM leave_request_allocations WHERE tenant_id=? AND leave_request_id=?)` {
						_, e := db.Exec(query, tenantID, tenantID, leaveID)
						if e != nil {
							t.Errorf("cleanup usage ledger: %v", e)
						}
					} else {
						_, e := db.Exec(query, tenantID, leaveID)
						if e != nil {
							t.Errorf("cleanup leave: %v", e)
						}
					}
				}
			}
			if _, e := db.Exec(`DELETE FROM requests WHERE tenant_id=? AND id=?`, tenantID, requestID); e != nil {
				t.Errorf("cleanup request row: %v", e)
			}
		}
		for _, id := range scheduleIDs {
			if _, e := db.Exec(`DELETE FROM work_schedules WHERE tenant_id=? AND id=?`, tenantID, id); e != nil {
				t.Errorf("cleanup schedule: %v", e)
			}
		}
		for _, id := range ledgerIDs {
			if _, e := db.Exec(`DELETE FROM leave_balance_ledger WHERE tenant_id=? AND id=?`, tenantID, id); e != nil {
				t.Errorf("cleanup accrual: %v", e)
			}
		}
		if _, e := db.Exec(`DELETE FROM leave_balances WHERE tenant_id=? AND employee_id=? AND leave_type_id=?`, tenantID, employeeID, typeID); e != nil {
			t.Errorf("cleanup leave balances: %v", e)
		}
		for _, query := range []struct{ sql, id string }{{`DELETE FROM approval_flow_steps WHERE tenant_id=? AND flow_id=?`, flowID}, {`DELETE FROM approval_flows WHERE tenant_id=? AND id=?`, flowID}, {`DELETE FROM leave_policies WHERE tenant_id=? AND id=?`, policyID}, {`DELETE FROM leave_types WHERE tenant_id=? AND id=?`, typeID}, {`DELETE FROM shift_templates WHERE tenant_id=? AND id=?`, shiftID}} {
			if _, e := db.Exec(query.sql, tenantID, query.id); e != nil {
				t.Errorf("cleanup fixture: %v", e)
			}
		}
	})
	if _, err = db.Exec(`INSERT INTO leave_types (tenant_id,id,code,name) VALUES (?,?,?,'QA cross-year leave')`, tenantID, typeID, "QA-"+typeID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO leave_policies (tenant_id,id,leave_type_id,accrual_type,valid_from) VALUES (?,?,?,'manual','2000-01-01')`, tenantID, policyID, typeID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO approval_flows (tenant_id,id,request_type,name,is_default,valid_from) VALUES (?,?,'leave','QA Leave Flow',TRUE,'2000-01-01')`, tenantID, flowID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO approval_flow_steps (tenant_id,id,flow_id,level_no,approver_kind,approver_ref_id,required_permission) VALUES (?,?,?,1,'user',?,'cuti.approve')`, tenantID, stepID, flowID, adminState.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO shift_templates (tenant_id,id,code,name,start_time,end_time,clock_in_window_start_time,clock_in_window_end_time,clock_out_window_start_time,clock_out_window_end_time) VALUES (?,?,?,'QA Leave Shift','09:00:00','17:00:00','08:00:00','10:00:00','16:00:00','18:00:00')`, tenantID, shiftID, "QA-"+shiftID); err != nil {
		t.Fatal(err)
	}
	for i, date := range dates {
		if _, err = db.Exec(`INSERT INTO work_schedules (tenant_id,id,employee_id,work_date,shift_template_id,day_type,office_id,status,published_at) VALUES (?,?,?,?,?,'shift',?,'published',UTC_TIMESTAMP(6))`, tenantID, scheduleIDs[i], employeeID, date, shiftID, officeID); err != nil {
			t.Fatal(err)
		}
	}
	for i, y := range []int{year, year + 1} {
		if _, err = db.Exec(`INSERT INTO leave_balance_ledger (tenant_id,id,employee_id,leave_type_id,entry_type,days,effective_date,ref_type,ref_id) VALUES (?,?,?,?,'accrual',3,?,'qa',?)`, tenantID, ledgerIDs[i], employeeID, typeID, fmt.Sprintf("%04d-01-01", y), ledgerIDs[i]); err != nil {
			t.Fatal(err)
		}
	}
	input := map[string]any{"leave_type_id": typeID, "start_date": start, "end_date": end, "reason": "QA lintas tahun"}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/leave-requests/preview", map[string]any{"leave_type_id": typeID, "start_date": start, "end_date": fmt.Sprintf("%04d-01-03", year+1)}, 409, nil)
	if _, err = db.Exec(`UPDATE leave_balance_ledger SET days=1 WHERE tenant_id=? AND id=?`, tenantID, ledgerIDs[1]); err != nil {
		t.Fatal(err)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/leave-requests/preview", input, 409, nil)
	if _, err = db.Exec(`UPDATE leave_balance_ledger SET days=3 WHERE tenant_id=? AND id=?`, tenantID, ledgerIDs[1]); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Data struct {
			Days        int               `json:"days"`
			Allocations []leaveAllocation `json:"allocations"`
		} `json:"data"`
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/leave-requests/preview", input, 200, &preview)
	if preview.Data.Days != 4 || len(preview.Data.Allocations) != 2 || preview.Data.Allocations[0].Days != 2 || preview.Data.Allocations[1].Days != 2 {
		t.Fatalf("cross-year preview: %#v", preview.Data)
	}
	var submitted struct {
		Data struct {
			ID        string `json:"id"`
			RequestID string `json:"request_id"`
		} `json:"data"`
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/leave-requests", input, 201, &submitted)
	leaveID, requestID = submitted.Data.ID, submitted.Data.RequestID
	var adminPage struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/leave-requests?employee_id="+employeeID+"&from="+start+"&to="+end, nil, 200, &adminPage)
	if len(adminPage.Data) != 1 || adminPage.Data[0]["id"] != leaveID {
		t.Fatalf("admin leave page: %#v", adminPage.Data)
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/leave-requests/"+leaveID, nil, 200, nil)
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/leave-requests", input, 409, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+requestID+"/actions", map[string]any{"action": "return", "reason": "Perjelas alasan", "level_no": 1, "version": 1}, 200, nil)
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+requestID+"/resubmit", map[string]any{"version": 2, "correction": map[string]any{"note": "Alasan sudah diperjelas", "reason": "Cuti keluarga lintas tahun", "start_date": start, "end_date": end}}, 200, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+requestID+"/actions", map[string]any{"action": "approve", "level_no": 1, "version": 3}, 200, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+requestID+"/actions", map[string]any{"action": "approve", "level_no": 1, "version": 1}, 409, nil)
	var usage int
	var total string
	if err = db.QueryRow(`SELECT COUNT(*),CAST(SUM(days) AS CHAR) FROM leave_balance_ledger WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND entry_type='usage' AND ref_type='leave_request_allocation'`, tenantID, employeeID, typeID).Scan(&usage, &total); err != nil || usage != 2 || total != "-4.00" {
		t.Fatalf("usage=%d total=%q err=%v", usage, total, err)
	}
	var detail struct {
		Data map[string]any `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/leave-requests/"+leaveID, nil, 200, &detail)
	if detail.Data["status"] != "approved" {
		t.Fatalf("detail=%#v", detail.Data)
	}
}
