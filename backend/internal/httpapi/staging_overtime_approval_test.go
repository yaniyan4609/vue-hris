package httpapi

import (
	"context"
	"database/sql"
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

func TestStagingOvertimeApprovalDecision(t *testing.T) {
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
	newClient := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, Timeout: 10 * time.Second}
	}
	admin, ess := newClient(), newClient()
	var adminState, essState auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &adminState)
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &essState)
	tenantID := adminState.SelectedTenantID
	var employeeID string
	if err := db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? AND user_id=?`, tenantID, essState.User.ID).Scan(&employeeID); err != nil {
		t.Fatal(err)
	}
	flowID, _ := ids.New()
	stepOne, _ := ids.New()
	stepTwo, _ := ids.New()
	if _, err := db.Exec(`INSERT INTO approval_flows (tenant_id,id,request_type,name,valid_from) VALUES (?,?,'overtime','QA Overtime Flow','2000-01-01')`, tenantID, flowID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM approval_flow_steps WHERE tenant_id=? AND flow_id=?`, tenantID, flowID); err != nil {
			t.Errorf("cleanup flow steps: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM approval_flows WHERE tenant_id=? AND id=?`, tenantID, flowID); err != nil {
			t.Errorf("cleanup flow: %v", err)
		}
	})
	for level, stepID := range []string{stepOne, stepTwo} {
		if _, err := db.Exec(`INSERT INTO approval_flow_steps (tenant_id,id,flow_id,level_no,approver_kind,approver_ref_id,required_permission) VALUES (?,?,?,?,'user',?,'lembur.approve')`, tenantID, stepID, flowID, level+1, adminState.User.ID); err != nil {
			t.Fatal(err)
		}
	}
	makeRequest := func() (string, string, string) {
		requestID, _ := ids.New()
		overtimeID, _ := ids.New()
		instanceID, _ := ids.New()
		taskID, _ := ids.New()
		if _, err := db.Exec(`INSERT INTO requests (tenant_id,id,request_type,subject_employee_id,requested_by,ref_table,ref_id,status,submitted_at) VALUES (?,?,'overtime',?,?,'overtime_requests',?,'in_review',UTC_TIMESTAMP(6))`, tenantID, requestID, employeeID, essState.User.ID, overtimeID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO overtime_requests (tenant_id,id,employee_id,work_date,proposed_minutes,status,request_id) VALUES (?,?,?,CURRENT_DATE(),180,'in_review',?)`, tenantID, overtimeID, employeeID, requestID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO approval_instances (tenant_id,id,request_id,flow_id,current_level,status,version) VALUES (?,?,?,?,1,'in_review',1)`, tenantID, instanceID, requestID, flowID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO approval_tasks (tenant_id,id,instance_id,level_no,assignee_user_id,status) VALUES (?,?,?,1,?,'pending')`, tenantID, taskID, instanceID, adminState.User.ID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			for _, entry := range []struct{ query, id string }{
				{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='requests' AND entity_id=?`, requestID},
				{`DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='request' AND aggregate_id=?`, requestID},
				{`DELETE FROM request_revisions WHERE tenant_id=? AND request_id=?`, requestID},
				{`DELETE FROM approval_actions WHERE tenant_id=? AND instance_id=?`, instanceID},
				{`DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id=?`, instanceID},
				{`DELETE FROM approval_instances WHERE tenant_id=? AND id=?`, instanceID},
				{`DELETE FROM overtime_requests WHERE tenant_id=? AND id=?`, overtimeID},
				{`DELETE FROM requests WHERE tenant_id=? AND id=?`, requestID},
			} {
				if _, err := db.Exec(entry.query, tenantID, entry.id); err != nil {
					t.Errorf("cleanup overtime fixture: %v", err)
				}
			}
		})
		return requestID, overtimeID, instanceID
	}
	requestID, overtimeID, instanceID := makeRequest()
	actionURL := server.URL + "/api/v1/admin/approvals/" + requestID + "/actions"
	var decision struct {
		Data struct {
			Status          string `json:"status"`
			CurrentLevel    int    `json:"current_level"`
			ApprovalVersion int    `json:"approval_version"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", actionURL, map[string]any{"action": "approve", "level_no": 1, "version": 1}, 200, &decision)
	if decision.Data.Status != "in_review" || decision.Data.CurrentLevel != 2 || decision.Data.ApprovalVersion != 2 {
		t.Fatalf("first level: %#v", decision.Data)
	}
	doJSON(t, admin, "POST", actionURL, map[string]any{"action": "approve", "level_no": 1, "version": 1}, 409, nil)
	doJSON(t, admin, "POST", actionURL, map[string]any{"action": "approve", "level_no": 2, "version": 2, "data": map[string]any{"approved_minutes": 181}}, 400, nil)
	doJSON(t, admin, "POST", actionURL, map[string]any{"action": "approve", "level_no": 2, "version": 2, "data": map[string]any{"approved_minutes": 120, "unexpected": "private"}}, 400, nil)
	doJSON(t, admin, "POST", actionURL, map[string]any{"action": "approve", "level_no": 2, "version": 2, "data": map[string]any{"approved_minutes": 120}}, 200, &decision)
	if decision.Data.Status != "approved" || decision.Data.ApprovalVersion != 3 {
		t.Fatalf("final decision: %#v", decision.Data)
	}
	doJSON(t, admin, "POST", actionURL, map[string]any{"action": "approve", "level_no": 2, "version": 2, "data": map[string]any{"approved_minutes": 120}}, 409, nil)
	var proposed, approved int
	var paid sql.NullInt64
	var status string
	if err := db.QueryRow(`SELECT proposed_minutes,approved_minutes,paid_minutes,status FROM overtime_requests WHERE tenant_id=? AND id=?`, tenantID, overtimeID).Scan(&proposed, &approved, &paid, &status); err != nil || proposed != 180 || approved != 120 || paid.Valid || status != "approved" {
		t.Fatalf("overtime domain: proposed=%d approved=%d paid=%v status=%s err=%v", proposed, approved, paid, status, err)
	}
	var actionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM approval_actions WHERE tenant_id=? AND instance_id=?`, tenantID, instanceID).Scan(&actionCount); err != nil || actionCount != 2 {
		t.Fatalf("approval actions: count=%d err=%v", actionCount, err)
	}
	var approvalDetail struct {
		Data struct {
			Actions []map[string]any `json:"actions"`
		} `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/approvals/"+requestID, nil, 200, &approvalDetail)
	if len(approvalDetail.Data.Actions) != 2 {
		t.Fatalf("approval detail actions: %#v", approvalDetail.Data.Actions)
	}
	lastData, ok := approvalDetail.Data.Actions[1]["data"].(map[string]any)
	if !ok || lastData["approved_minutes"] != float64(120) {
		t.Fatalf("approved minutes action data: %#v", approvalDetail.Data.Actions[1])
	}
	rejectedID, rejectedOvertimeID, _ := makeRequest()
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+rejectedID+"/actions", map[string]any{"action": "reject", "reason": "Jadwal tidak sesuai", "level_no": 1, "version": 1}, 200, &decision)
	if decision.Data.Status != "rejected" {
		t.Fatalf("reject response: %#v", decision.Data)
	}
	if err := db.QueryRow(`SELECT status FROM overtime_requests WHERE tenant_id=? AND id=?`, tenantID, rejectedOvertimeID).Scan(&status); err != nil || status != "rejected" {
		t.Fatalf("rejected domain: status=%s err=%v", status, err)
	}
	returnedID, returnedOvertimeID, _ := makeRequest()
	returnedActionURL := server.URL + "/api/v1/admin/approvals/" + returnedID + "/actions"
	doJSON(t, admin, "POST", returnedActionURL, map[string]any{"action": "return", "reason": "Durasi perlu diperbaiki", "level_no": 1, "version": 1}, 200, &decision)
	var returnedStatus string
	if err := db.QueryRow(`SELECT status FROM overtime_requests WHERE tenant_id=? AND id=?`, tenantID, returnedOvertimeID).Scan(&returnedStatus); err != nil || returnedStatus != "returned" {
		t.Fatalf("returned overtime: status=%s err=%v", returnedStatus, err)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+returnedID+"/resubmit", map[string]any{"version": 2, "correction": map[string]any{"note": "Menit dan alasan diperbaiki", "proposed_minutes": 90, "reason": "Rapat mendadak"}}, 200, nil)
	var revisedMinutes int
	var revisedReason string
	if err := db.QueryRow(`SELECT proposed_minutes,reason,status FROM overtime_requests WHERE tenant_id=? AND id=?`, tenantID, returnedOvertimeID).Scan(&revisedMinutes, &revisedReason, &returnedStatus); err != nil || revisedMinutes != 90 || revisedReason != "Rapat mendadak" || returnedStatus != "in_review" {
		t.Fatalf("revised overtime: minutes=%d reason=%s status=%s err=%v", revisedMinutes, revisedReason, returnedStatus, err)
	}
	doJSON(t, admin, "POST", returnedActionURL, map[string]any{"action": "approve", "level_no": 1, "version": 3}, 200, &decision)
	doJSON(t, admin, "POST", returnedActionURL, map[string]any{"action": "approve", "level_no": 2, "version": 4, "data": map[string]any{"approved_minutes": 80}}, 200, &decision)
	if decision.Data.Status != "approved" {
		t.Fatalf("revised overtime approval: %#v", decision.Data)
	}
	cancelID, cancelOvertimeID, _ := makeRequest()
	var cancelled struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+cancelID+"/cancel", nil, 200, &cancelled)
	if cancelled.Data.Status != "cancelled" {
		t.Fatalf("cancelled request: %#v", cancelled.Data)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+cancelID+"/cancel", nil, 409, nil)
	if err := db.QueryRow(`SELECT status FROM overtime_requests WHERE tenant_id=? AND id=?`, tenantID, cancelOvertimeID).Scan(&status); err != nil || status != "cancelled" {
		t.Fatalf("cancelled overtime: status=%s err=%v", status, err)
	}
	returnedCancelID, returnedCancelOvertimeID, _ := makeRequest()
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+returnedCancelID+"/actions", map[string]any{"action": "return", "reason": "Perbaiki", "level_no": 1, "version": 1}, 200, &decision)
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+returnedCancelID+"/cancel", nil, 200, &cancelled)
	if err := db.QueryRow(`SELECT status FROM overtime_requests WHERE tenant_id=? AND id=?`, tenantID, returnedCancelOvertimeID).Scan(&status); err != nil || status != "cancelled" {
		t.Fatalf("cancelled returned overtime: status=%s err=%v", status, err)
	}
	var existingDefault int
	if err := db.QueryRow(`SELECT COUNT(*) FROM approval_flows WHERE tenant_id=? AND request_type='overtime' AND is_default=TRUE`, tenantID).Scan(&existingDefault); err != nil {
		t.Fatal(err)
	}
	if existingDefault == 0 {
		t.Log("testing ESS submission with temporary default flow")
		if _, err := db.Exec(`UPDATE approval_flows SET is_default=TRUE WHERE tenant_id=? AND id=?`, tenantID, flowID); err != nil {
			t.Fatal(err)
		}
		workDate := time.Now().AddDate(0, 0, 20).Format("2006-01-02")
		var submitted struct {
			Data struct {
				ID        string `json:"id"`
				RequestID string `json:"request_id"`
				Status    string `json:"status"`
			} `json:"data"`
		}
		doJSON(t, ess, "POST", server.URL+"/api/v1/ess/overtime-requests", map[string]any{"work_date": workDate, "proposed_minutes": 150, "reason": "Persiapan tutup buku"}, 201, &submitted)
		t.Cleanup(func() {
			var submittedInstanceID string
			if err := db.QueryRow(`SELECT id FROM approval_instances WHERE tenant_id=? AND request_id=?`, tenantID, submitted.Data.RequestID).Scan(&submittedInstanceID); err != nil {
				t.Errorf("cleanup submitted instance lookup: %v", err)
				return
			}
			for _, entry := range []struct{ query, id string }{
				{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='requests' AND entity_id=?`, submitted.Data.RequestID},
				{`DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='request' AND aggregate_id=?`, submitted.Data.RequestID},
				{`DELETE FROM approval_actions WHERE tenant_id=? AND instance_id=?`, submittedInstanceID},
				{`DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id=?`, submittedInstanceID},
				{`DELETE FROM approval_instances WHERE tenant_id=? AND id=?`, submittedInstanceID},
				{`DELETE FROM overtime_requests WHERE tenant_id=? AND id=?`, submitted.Data.ID},
				{`DELETE FROM requests WHERE tenant_id=? AND id=?`, submitted.Data.RequestID},
			} {
				if _, err := db.Exec(entry.query, tenantID, entry.id); err != nil {
					t.Errorf("cleanup submitted overtime: %v", err)
				}
			}
		})
		if len(submitted.Data.ID) != 26 || len(submitted.Data.RequestID) != 26 || submitted.Data.Status != "in_review" {
			t.Fatalf("submitted overtime: %#v", submitted.Data)
		}
		doJSON(t, ess, "POST", server.URL+"/api/v1/ess/overtime-requests", map[string]any{"work_date": workDate, "proposed_minutes": 150, "reason": "Duplikat"}, 409, nil)
		var overtimeDetail struct {
			Data overtimeView `json:"data"`
		}
		doJSON(t, ess, "GET", server.URL+"/api/v1/ess/overtime-requests/"+submitted.Data.ID, nil, 200, &overtimeDetail)
		if overtimeDetail.Data.ID != submitted.Data.ID || overtimeDetail.Data.ProposedMinutes != 150 {
			t.Fatalf("ESS overtime detail: %#v", overtimeDetail.Data)
		}
		doJSON(t, admin, "GET", server.URL+"/api/v1/admin/overtime-requests/"+submitted.Data.ID, nil, 200, &overtimeDetail)
		var overtimePage struct {
			Data []overtimeView `json:"data"`
		}
		doJSON(t, ess, "GET", server.URL+"/api/v1/ess/overtime-requests?from="+workDate+"&to="+workDate, nil, 200, &overtimePage)
		if len(overtimePage.Data) == 0 {
			t.Fatal("ESS overtime list missing submission")
		}
		doJSON(t, admin, "GET", server.URL+"/api/v1/admin/overtime-requests?employee_id="+employeeID+"&from="+workDate+"&to="+workDate, nil, 200, &overtimePage)
		if len(overtimePage.Data) == 0 {
			t.Fatal("Admin overtime list missing submission")
		}
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+submitted.Data.RequestID+"/actions", map[string]any{"action": "approve", "level_no": 1, "version": 1}, 200, &decision)
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+submitted.Data.RequestID+"/actions", map[string]any{"action": "approve", "level_no": 2, "version": 2, "data": map[string]any{"approved_minutes": 90}}, 200, &decision)
		if decision.Data.Status != "approved" {
			t.Fatalf("submitted overtime approval: %#v", decision.Data)
		}
	} else {
		t.Log("ESS submission test omitted because the tenant already has a default overtime flow")
	}
}
