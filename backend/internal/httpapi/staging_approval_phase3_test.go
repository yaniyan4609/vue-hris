package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
)

func phase3BulkCall(t *testing.T, client *http.Client, base, key string, body any) (int, string, map[string]any) {
	t.Helper()
	csrfResponse, err := client.Get(base + "/api/v1/auth/csrf")
	if err != nil {
		t.Fatal(err)
	}
	var csrf struct {
		Data struct {
			Token string `json:"csrf_token"`
		} `json:"data"`
	}
	if err = json.NewDecoder(csrfResponse.Body).Decode(&csrf); err != nil {
		t.Fatal(err)
	}
	csrfResponse.Body.Close()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", base+"/api/v1/admin/approvals/bulk-actions", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf.Data.Token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, response.Header.Get("Idempotent-Replay"), decoded
}

func TestStagingApprovalPhase3BulkDelegationReassign(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
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
	newClient := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, Timeout: 10 * time.Second}
	}
	admin, ess := newClient(), newClient()
	var adminState, essState auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &adminState)
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &essState)
	tenantID := adminState.SelectedTenantID
	var employeeID, roleID string
	if err = db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? AND user_id=? AND deleted_at IS NULL`, tenantID, essState.User.ID).Scan(&employeeID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT id FROM roles WHERE tenant_id=? AND code='ADMIN_TENANT'`, tenantID).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	qaUser, _ := ids.New()
	qaRole, _ := ids.New()
	flowID, _ := ids.New()
	stepID, _ := ids.New()
	delegationID := ""
	notificationID := ""
	requestIDs := []string{}
	instanceIDs := []string{}
	taskIDs := []string{}
	key := "23e90f3b-b58b-482d-a6b3-2d8be8952710"
	t.Cleanup(func() {
		if notificationID != "" {
			if _, e := db.Exec(`DELETE FROM notifications WHERE tenant_id=? AND id=?`, tenantID, notificationID); e != nil {
				t.Errorf("cleanup notification: %v", e)
			}
		}
		for _, query := range []string{
			`DELETE FROM approval_bulk_items WHERE tenant_id=? AND batch_id IN (SELECT id FROM approval_bulk_batches WHERE tenant_id=? AND actor_user_id=? AND idempotency_key=?)`,
			`DELETE FROM approval_bulk_batches WHERE tenant_id=? AND actor_user_id=? AND idempotency_key=?`,
		} {
			var e error
			if strings.Contains(query, "approval_bulk_items") {
				_, e = db.Exec(query, tenantID, tenantID, adminState.User.ID, key)
			} else {
				_, e = db.Exec(query, tenantID, adminState.User.ID, key)
			}
			if e != nil {
				t.Errorf("cleanup bulk: %v", e)
			}
		}
		for i, id := range requestIDs {
			for _, query := range []string{
				`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='requests' AND entity_id=?`,
				`DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='request' AND aggregate_id=?`,
			} {
				if _, e := db.Exec(query, tenantID, id); e != nil {
					t.Errorf("cleanup request log: %v", e)
				}
			}
			instance := instanceIDs[i]
			for _, query := range []string{
				`DELETE FROM approval_reassignments WHERE tenant_id=? AND instance_id=?`,
				`DELETE FROM approval_actions WHERE tenant_id=? AND instance_id=?`,
				`DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id=?`,
				`DELETE FROM approval_instances WHERE tenant_id=? AND id=?`,
			} {
				if _, e := db.Exec(query, tenantID, instance); e != nil {
					t.Errorf("cleanup approval: %v", e)
				}
			}
			if _, e := db.Exec(`DELETE FROM requests WHERE tenant_id=? AND id=?`, tenantID, id); e != nil {
				t.Errorf("cleanup request: %v", e)
			}
		}
		if delegationID != "" {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='approval_delegations' AND entity_id=?`, tenantID, delegationID); e != nil {
				t.Error(e)
			}
		}
		if _, e := db.Exec(`DELETE FROM approval_delegations WHERE tenant_id=? AND id=?`, tenantID, delegationID); e != nil {
			t.Error(e)
		}
		if _, e := db.Exec(`DELETE FROM approval_flow_steps WHERE tenant_id=? AND id=?`, tenantID, stepID); e != nil {
			t.Error(e)
		}
		if _, e := db.Exec(`DELETE FROM approval_flows WHERE tenant_id=? AND id=?`, tenantID, flowID); e != nil {
			t.Error(e)
		}
		if _, e := db.Exec(`DELETE FROM user_roles WHERE tenant_id=? AND id=?`, tenantID, qaRole); e != nil {
			t.Error(e)
		}
		if _, e := db.Exec(`DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, tenantID, qaUser); e != nil {
			t.Error(e)
		}
		if _, e := db.Exec(`DELETE FROM users WHERE id=?`, qaUser); e != nil {
			t.Error(e)
		}
	})
	if _, err = db.Exec(`INSERT INTO users (id,email,email_normalized,display_name) VALUES (?,?,?,?)`, qaUser, "qa-"+qaUser+"@example.invalid", "qa-"+qaUser+"@example.invalid", "QA Approver"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO tenant_memberships (tenant_id,user_id,status) VALUES (?,?,'active')`, tenantID, qaUser); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO user_roles (tenant_id,id,user_id,role_id,valid_from) VALUES (?,?,?,?,?)`, tenantID, qaRole, qaUser, roleID, "2000-01-01"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO approval_flows (tenant_id,id,request_type,name,valid_from) VALUES (?,?,'profile_change','QA Phase3 Flow','2000-01-01')`, tenantID, flowID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO approval_flow_steps (tenant_id,id,flow_id,level_no,approver_kind,approver_ref_id,required_permission) VALUES (?,?,?,1,'user',?,'karyawan.approve')`, tenantID, stepID, flowID, adminState.User.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		requestID, _ := ids.New()
		instanceID, _ := ids.New()
		taskID, _ := ids.New()
		if _, err = db.Exec(`INSERT INTO requests (tenant_id,id,request_type,subject_employee_id,requested_by,status,submitted_at) VALUES (?,?,'profile_change',?,?,'in_review',UTC_TIMESTAMP(6))`, tenantID, requestID, employeeID, essState.User.ID); err != nil {
			t.Fatal(err)
		}
		requestIDs = append(requestIDs, requestID)
		instanceIDs = append(instanceIDs, instanceID)
		taskIDs = append(taskIDs, taskID)
		if _, err = db.Exec(`INSERT INTO approval_instances (tenant_id,id,request_id,flow_id,current_level,status,version) VALUES (?,?,?,?,1,'in_review',1)`, tenantID, instanceID, requestID, flowID); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO approval_tasks (tenant_id,id,instance_id,level_no,assignee_user_id,status) VALUES (?,?,?,1,?,'pending')`, tenantID, taskID, instanceID, adminState.User.ID); err != nil {
			t.Fatal(err)
		}
	}
	items := []map[string]any{{"request_id": requestIDs[0], "action": "return", "reason": "Perbaiki data profil QA", "level_no": 1, "version": 1}, {"request_id": requestIDs[1], "action": "approve", "level_no": 1, "version": 1}}
	status, replay, first := phase3BulkCall(t, admin, server.URL, key, map[string]any{"items": items})
	if status != 200 || replay != "" {
		t.Fatalf("bulk status/replay: %d %q %#v", status, replay, first)
	}
	data := first["data"].(map[string]any)
	results := data["items"].([]any)
	if data["succeeded"] != float64(1) || data["failed"] != float64(1) || results[0].(map[string]any)["status_code"] != float64(200) || results[1].(map[string]any)["status_code"] != float64(409) {
		t.Fatalf("bulk outcomes: %#v", first)
	}
	status, replay, second := phase3BulkCall(t, admin, server.URL, key, map[string]any{"items": items})
	if status != 200 || replay != "true" || !reflect.DeepEqual(first, second) {
		t.Fatalf("bulk replay: %d %q %#v", status, replay, second)
	}
	changed := []map[string]any{{"request_id": requestIDs[0], "action": "reject", "reason": "Perbaiki data profil QA", "level_no": 1, "version": 1}}
	status, _, _ = phase3BulkCall(t, admin, server.URL, key, map[string]any{"items": changed})
	if status != 409 {
		t.Fatalf("idempotency conflict: %d", status)
	}
	today := tenantToday("Asia/Jakarta")
	tomorrowDate, _ := time.Parse("2006-01-02", today)
	tomorrow := tomorrowDate.AddDate(0, 0, 1).Format("2006-01-02")
	delegationInput := map[string]any{"from_user_id": adminState.User.ID, "to_user_id": qaUser, "request_type": "profile_change", "valid_from": today, "valid_to": tomorrow, "reason": "Alih tugas QA selama cuti"}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approval-delegations", delegationInput, 403, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "password"}, 200, nil)
	delegationInput["to_user_id"] = essState.User.ID
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approval-delegations", delegationInput, 403, nil)
	nonmemberUser, _ := ids.New()
	delegationInput["to_user_id"] = nonmemberUser
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approval-delegations", delegationInput, 403, nil)
	delegationInput["to_user_id"] = qaUser
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approval-delegations", delegationInput, 201, &created)
	delegationID = created.Data.ID
	if delegationID == "" {
		t.Fatal("missing delegation ID")
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approval-delegations", delegationInput, 409, nil)
	var listed struct {
		Data []approvalDelegation `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/approval-delegations?active=true", nil, 200, &listed)
	found := false
	for _, item := range listed.Data {
		if item.ID == delegationID {
			found = true
		}
	}
	if !found {
		t.Fatal("delegation absent from list")
	}
	a := adminAPI{db: db, auth: auth.Service{DB: db}, access: access.Store{DB: db}}
	employee, err := scanEmployee(db.QueryRow(employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, tenantID, employeeID))
	if err != nil {
		t.Fatal(err)
	}
	target, err := a.targetFor(ctx, tenantID, employee)
	if err != nil {
		t.Fatal(err)
	}
	lookup := httptest.NewRequest("GET", server.URL+"/", nil)
	selected, err := a.eligibleApprovers(lookup, tenantID, "Asia/Jakarta", employee, target, "karyawan.approve", []string{adminState.User.ID}, "profile_change")
	if err != nil || len(selected) != 1 || selected[0] != qaUser {
		t.Fatalf("delegation not applied: %v %#v", err, selected)
	}
	doJSON(t, admin, "DELETE", server.URL+"/api/v1/admin/approval-delegations/"+delegationID, nil, 204, nil)
	selected, err = a.eligibleApprovers(lookup, tenantID, "Asia/Jakarta", employee, target, "karyawan.approve", []string{adminState.User.ID}, "profile_change")
	if err != nil || len(selected) != 1 || selected[0] != adminState.User.ID {
		t.Fatalf("revocation not applied: %v %#v", err, selected)
	}
	notificationID, _ = ids.New()
	notificationPayload, _ := json.Marshal(map[string]any{"request_id": requestIDs[1]})
	if _, err = db.Exec(`INSERT INTO notifications (tenant_id,id,recipient_user_id,category,event_type,title,body,payload_json,dedupe_key) VALUES (?,?,?,'approval','approval.submitted','QA Approval','QA Approval',?,?)`, tenantID, notificationID, adminState.User.ID, string(notificationPayload), "qa-approval:"+requestIDs[1]); err != nil {
		t.Fatal(err)
	}
	var reassigned struct {
		Data struct {
			ToTaskID        string `json:"to_task_id"`
			ApprovalVersion uint64 `json:"approval_version"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+requestIDs[1]+"/reassign", map[string]any{"task_id": taskIDs[1], "to_user_id": nonmemberUser, "version": 1, "reason": "Alih tugas ke pengguna di luar tenant"}, 403, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+requestIDs[1]+"/reassign", map[string]any{"task_id": taskIDs[1], "to_user_id": qaUser, "version": 1, "reason": "Alih tugas ke approver QA"}, 200, &reassigned)
	if reassigned.Data.ToTaskID == "" || reassigned.Data.ApprovalVersion != 2 {
		t.Fatalf("reassign response: %#v", reassigned)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+requestIDs[1]+"/reassign", map[string]any{"task_id": taskIDs[1], "to_user_id": qaUser, "version": 1, "reason": "Alih tugas ke approver QA"}, 409, nil)
	var detail struct {
		Data struct {
			Reassignments []map[string]any `json:"reassignments"`
		} `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/approvals/"+requestIDs[1], nil, 200, &detail)
	if len(detail.Data.Reassignments) != 1 {
		t.Fatalf("reassign history: %#v", detail)
	}
	var obsolete int
	if err = db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE tenant_id=? AND id=? AND obsolete_at IS NOT NULL`, tenantID, notificationID).Scan(&obsolete); err != nil || obsolete != 1 {
		t.Fatalf("old notification still actionable: %d %v", obsolete, err)
	}
	var outboxCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM notification_outbox WHERE tenant_id=? AND aggregate_type='request' AND aggregate_id=? AND event_type='approval.reassigned'`, tenantID, requestIDs[1]).Scan(&outboxCount); err != nil || outboxCount != 1 {
		t.Fatalf("reassign outbox: %d %v", outboxCount, err)
	}
}
