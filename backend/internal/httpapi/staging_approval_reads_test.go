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

func TestStagingApprovalReadScope(t *testing.T) {
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
	defer db.Close()
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
	var employeeID string
	if err := db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? AND user_id=?`, adminState.SelectedTenantID, essState.User.ID).Scan(&employeeID); err != nil {
		t.Fatal(err)
	}
	flowID, _ := ids.New()
	requestID, _ := ids.New()
	instanceID, _ := ids.New()
	taskID, _ := ids.New()
	stepID, _ := ids.New()
	if _, err := db.Exec(`INSERT INTO approval_flows (tenant_id,id,request_type,name,valid_from) VALUES (?,?,'leave','QA Read Flow','2000-01-01')`, adminState.SelectedTenantID, flowID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(`DELETE FROM approval_flows WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, flowID); err != nil {
			t.Errorf("cleanup flow: %v", err)
		}
	}()
	if _, err := db.Exec(`INSERT INTO approval_flow_steps (tenant_id,id,flow_id,level_no,approver_kind,approver_ref_id,required_permission) VALUES (?,?,?,1,'user',?,'cuti.approve')`, adminState.SelectedTenantID, stepID, flowID, adminState.User.ID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(`DELETE FROM approval_flow_steps WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, stepID); err != nil {
			t.Errorf("cleanup step: %v", err)
		}
	}()
	if _, err := db.Exec(`INSERT INTO requests (tenant_id,id,request_type,subject_employee_id,requested_by,status,submitted_at) VALUES (?,?,'leave',?,?,'in_review',UTC_TIMESTAMP(6))`, adminState.SelectedTenantID, requestID, employeeID, essState.User.ID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(`DELETE FROM requests WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, requestID); err != nil {
			t.Errorf("cleanup request: %v", err)
		}
	}()
	if _, err := db.Exec(`INSERT INTO approval_instances (tenant_id,id,request_id,flow_id,current_level,status,version) VALUES (?,?,?,?,1,'in_review',1)`, adminState.SelectedTenantID, instanceID, requestID, flowID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(`DELETE FROM approval_instances WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, instanceID); err != nil {
			t.Errorf("cleanup instance: %v", err)
		}
	}()
	if _, err := db.Exec(`INSERT INTO approval_tasks (tenant_id,id,instance_id,level_no,assignee_user_id,status) VALUES (?,?,?,1,?,'pending')`, adminState.SelectedTenantID, taskID, instanceID, adminState.User.ID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(`DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id=?`, adminState.SelectedTenantID, instanceID); err != nil {
			t.Errorf("cleanup task: %v", err)
		}
	}()
	var flows struct {
		Data []approvalFlow `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/approval-flows?request_type=leave", nil, 200, &flows)
	flowFound := false
	for _, flow := range flows.Data {
		if flow.ID == flowID {
			flowFound = true
		}
	}
	if !flowFound {
		t.Fatal("approval flow missing")
	}
	var flowDetail struct {
		Data approvalFlow `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/approval-flows/"+flowID, nil, 200, &flowDetail)
	if flowDetail.Data.ID != flowID || flowDetail.Data.Steps == nil {
		t.Fatalf("approval flow detail: %#v", flowDetail.Data)
	}
	var page struct {
		Data []requestView `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/requests?request_type=leave", nil, 200, &page)
	found := false
	for _, item := range page.Data {
		if item.ID == requestID {
			found = true
		}
	}
	if !found {
		t.Fatal("ESS request missing")
	}
	var detail struct {
		Data requestView `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/requests/"+requestID, nil, 200, &detail)
	if detail.Data.ID != requestID || detail.Data.AttemptNo != 1 || detail.Data.Version != 1 || detail.Data.Approval == nil || detail.Data.Approval.AttemptNo != 1 {
		t.Fatalf("ESS detail: %#v", detail.Data)
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/approvals/inbox?request_type=leave", nil, 200, &page)
	found = false
	for _, item := range page.Data {
		if item.ID == requestID {
			found = true
		}
	}
	if !found {
		t.Fatal("admin inbox missing")
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/approvals?request_type=leave", nil, 200, &page)
	found = false
	for _, item := range page.Data {
		if item.ID == requestID {
			found = true
		}
	}
	if !found {
		t.Fatal("admin approval list missing")
	}
	var summary struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/approvals/summary", nil, 200, &summary)
	if summary.Data.Total < 1 {
		t.Fatal("approval summary missing task")
	}
	var adminDetail struct {
		Data struct {
			Request requestView      `json:"request"`
			Tasks   []map[string]any `json:"tasks"`
		} `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/approvals/"+requestID, nil, 200, &adminDetail)
	if adminDetail.Data.Request.ID != requestID || len(adminDetail.Data.Tasks) != 1 {
		t.Fatalf("admin detail: %#v", adminDetail.Data)
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/admin/approvals/"+requestID, nil, 403, nil)
	doJSON(t, ess, "POST", server.URL+"/api/v1/admin/approvals/"+requestID+"/actions", map[string]any{"action": "return", "reason": "Perbaiki pengajuan", "level_no": 1, "version": 1}, 403, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+requestID+"/actions", map[string]any{"action": "return", "reason": "Perbaiki pengajuan", "level_no": 1, "version": 2}, 409, nil)
	unsupportedRefID, _ := ids.New()
	if _, err := db.Exec(`UPDATE requests SET ref_table='leave_requests',ref_id=? WHERE tenant_id=? AND id=?`, unsupportedRefID, adminState.SelectedTenantID, requestID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+requestID+"/actions", map[string]any{"action": "return", "reason": "Perbaiki pengajuan", "level_no": 1, "version": 1}, 409, nil)
	if _, err := db.Exec(`UPDATE requests SET ref_table=NULL,ref_id=NULL WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, requestID); err != nil {
		t.Fatal(err)
	}
	var returned struct {
		Data struct {
			Status          string `json:"status"`
			AttemptNo       int    `json:"attempt_no"`
			ApprovalVersion int    `json:"approval_version"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+requestID+"/actions", map[string]any{"action": "return", "reason": "Perbaiki pengajuan", "level_no": 1, "version": 1}, 200, &returned)
	defer func() {
		for _, entry := range []struct{ table, column string }{{"audit_logs", "entity_id"}, {"notification_outbox", "aggregate_id"}, {"approval_actions", "instance_id"}} {
			id := requestID
			if entry.table == "approval_actions" {
				id = instanceID
			}
			if _, err := db.Exec(`DELETE FROM `+entry.table+` WHERE tenant_id=? AND `+entry.column+`=?`, adminState.SelectedTenantID, id); err != nil {
				t.Errorf("cleanup %s: %v", entry.table, err)
			}
		}
	}()
	if returned.Data.Status != "returned" || returned.Data.AttemptNo != 1 || returned.Data.ApprovalVersion != 2 {
		t.Fatalf("unexpected return response: %#v", returned.Data)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+requestID+"/actions", map[string]any{"action": "return", "reason": "Ulang", "level_no": 1, "version": 1}, 409, nil)
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/requests/"+requestID, nil, 200, &detail)
	if detail.Data.Status != "returned" || detail.Data.AttemptNo != 1 || detail.Data.Approval == nil || detail.Data.Approval.Status != "returned" {
		t.Fatalf("ESS return detail: %#v", detail.Data)
	}
	defer func() {
		if _, err := db.Exec(`DELETE FROM request_revisions WHERE tenant_id=? AND request_id=?`, adminState.SelectedTenantID, requestID); err != nil {
			t.Errorf("cleanup revisions: %v", err)
		}
	}()
	var resubmitted struct {
		Data struct {
			RequestID       string `json:"request_id"`
			Status          string `json:"status"`
			AttemptNo       int    `json:"attempt_no"`
			ApprovalVersion int    `json:"approval_version"`
		} `json:"data"`
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+requestID+"/resubmit", map[string]any{"version": 1, "correction": map[string]any{"note": "Sudah diperbaiki"}}, 409, nil)
	if _, err := db.Exec(`UPDATE requests SET ref_table='leave_requests',ref_id=? WHERE tenant_id=? AND id=?`, unsupportedRefID, adminState.SelectedTenantID, requestID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+requestID+"/resubmit", map[string]any{"version": 2, "correction": map[string]any{"note": "Sudah diperbaiki"}}, 409, nil)
	if _, err := db.Exec(`UPDATE requests SET ref_table=NULL,ref_id=NULL WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, requestID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+requestID+"/resubmit", map[string]any{"version": 2, "correction": map[string]any{"note": "Sudah diperbaiki"}}, 200, &resubmitted)
	if resubmitted.Data.RequestID != requestID || resubmitted.Data.Status != "in_review" || resubmitted.Data.AttemptNo != 2 || resubmitted.Data.ApprovalVersion != 3 {
		t.Fatalf("unexpected resubmit response: %#v", resubmitted.Data)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+requestID+"/resubmit", map[string]any{"version": 2, "correction": map[string]any{"note": "Ulang"}}, 409, nil)
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/requests/"+requestID, nil, 200, &detail)
	if detail.Data.ID != requestID || detail.Data.AttemptNo != 2 || detail.Data.Status != "in_review" || string(detail.Data.LatestCorrection) != `{"note":"Sudah diperbaiki"}` {
		t.Fatalf("ESS resubmit detail: %#v", detail.Data)
	}
	var retryTasks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM approval_tasks WHERE tenant_id=? AND instance_id=? AND attempt_no=2 AND status='pending'`, adminState.SelectedTenantID, instanceID).Scan(&retryTasks); err != nil || retryTasks != 1 {
		t.Fatalf("retry tasks: count=%d err=%v", retryTasks, err)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+requestID+"/actions", map[string]any{"action": "return", "reason": "Perlu perbaikan lagi", "level_no": 1, "version": 3}, 200, &returned)
	if returned.Data.Status != "returned" || returned.Data.AttemptNo != 2 || returned.Data.ApprovalVersion != 4 {
		t.Fatalf("second return response: %#v", returned.Data)
	}
	var actionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM approval_actions WHERE tenant_id=? AND instance_id=? AND action='return'`, adminState.SelectedTenantID, instanceID).Scan(&actionCount); err != nil || actionCount != 2 {
		t.Fatalf("return actions: count=%d err=%v", actionCount, err)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+requestID+"/resubmit", map[string]any{"version": 4, "correction": map[string]any{"note": "Perbaikan kedua"}}, 200, &resubmitted)
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/requests/"+requestID, nil, 200, &detail)
	if detail.Data.AttemptNo != 3 || len(detail.Data.Revisions) != 2 || detail.Data.Revisions[0].AttemptNo != 2 || detail.Data.Revisions[1].AttemptNo != 3 || string(detail.Data.LatestCorrection) != `{"note":"Perbaikan kedua"}` {
		t.Fatalf("revision history: %#v", detail.Data)
	}
}
