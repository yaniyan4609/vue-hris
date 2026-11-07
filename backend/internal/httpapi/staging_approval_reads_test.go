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
	if _, err := db.Exec(`INSERT INTO approval_flows (tenant_id,id,request_type,name,valid_from) VALUES (?,?,'leave','QA Read Flow','2000-01-01')`, adminState.SelectedTenantID, flowID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(`DELETE FROM approval_flows WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, flowID); err != nil {
			t.Errorf("cleanup flow: %v", err)
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
		if _, err := db.Exec(`DELETE FROM approval_tasks WHERE tenant_id=? AND id=?`, adminState.SelectedTenantID, taskID); err != nil {
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
	if detail.Data.ID != requestID || detail.Data.Approval == nil {
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
}
