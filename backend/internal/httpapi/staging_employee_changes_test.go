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

func TestStagingEmployeeChangeWorkflow(t *testing.T) {
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
	admin, approver, ess := newClient(), newClient(), newClient()
	var adminState, essState auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &adminState)
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &essState)
	tenantID := adminState.SelectedTenantID
	var roleID string
	if err = db.QueryRow(`SELECT id FROM roles WHERE tenant_id=? AND code='ADMIN_TENANT'`, tenantID).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	qaUser, _ := ids.New()
	qaRole, _ := ids.New()
	qaEmployee, _ := ids.New()
	qaAssignment, _ := ids.New()
	flowID, _ := ids.New()
	stepID, _ := ids.New()
	email := "qa-change-" + qaUser + "@example.invalid"
	password := "Qa-Change-Password-29"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	var changeID, requestID, instanceID, appliedAssignmentID, newHistoryID string
	var rosterID string
	extraIDs := []string{}
	t.Cleanup(func() {
		for _, extraID := range extraIDs {
			var extraRequest sql.NullString
			_ = db.QueryRow(`SELECT request_id FROM employee_changes WHERE tenant_id=? AND id=?`, tenantID, extraID).Scan(&extraRequest)
			if extraRequest.Valid {
				var extraInstance string
				_ = db.QueryRow(`SELECT id FROM approval_instances WHERE tenant_id=? AND request_id=?`, tenantID, extraRequest.String).Scan(&extraInstance)
				for _, q := range []string{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='requests' AND entity_id=?`, `DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='request' AND aggregate_id=?`, `DELETE FROM request_revisions WHERE tenant_id=? AND request_id=?`} {
					if _, e := db.Exec(q, tenantID, extraRequest.String); e != nil {
						t.Error(e)
					}
				}
				if extraInstance != "" {
					for _, q := range []string{`DELETE FROM approval_actions WHERE tenant_id=? AND instance_id=?`, `DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id=?`, `DELETE FROM approval_instances WHERE tenant_id=? AND id=?`} {
						if _, e := db.Exec(q, tenantID, extraInstance); e != nil {
							t.Error(e)
						}
					}
				}
			}
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employee_changes' AND entity_id=?`, tenantID, extraID); e != nil {
				t.Error(e)
			}
			if _, e := db.Exec(`DELETE FROM employee_changes WHERE tenant_id=? AND id=?`, tenantID, extraID); e != nil {
				t.Error(e)
			}
			if extraRequest.Valid {
				if _, e := db.Exec(`DELETE FROM requests WHERE tenant_id=? AND id=?`, tenantID, extraRequest.String); e != nil {
					t.Error(e)
				}
			}
		}
		if changeID != "" {
			for _, q := range []string{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employee_changes' AND entity_id=?`, `DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='employee_change' AND aggregate_id=?`} {
				if _, e := db.Exec(q, tenantID, changeID); e != nil {
					t.Error(e)
				}
			}
		}
		if requestID != "" {
			for _, q := range []string{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='requests' AND entity_id=?`, `DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='request' AND aggregate_id=?`} {
				if _, e := db.Exec(q, tenantID, requestID); e != nil {
					t.Error(e)
				}
			}
			if _, e := db.Exec(`DELETE FROM request_revisions WHERE tenant_id=? AND request_id=?`, tenantID, requestID); e != nil {
				t.Error(e)
			}
		}
		if instanceID != "" {
			for _, q := range []string{`DELETE FROM approval_actions WHERE tenant_id=? AND instance_id=?`, `DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id=?`, `DELETE FROM approval_instances WHERE tenant_id=? AND id=?`} {
				if _, e := db.Exec(q, tenantID, instanceID); e != nil {
					t.Error(e)
				}
			}
		}
		if changeID != "" {
			if _, e := db.Exec(`DELETE FROM employee_changes WHERE tenant_id=? AND id=?`, tenantID, changeID); e != nil {
				t.Error(e)
			}
		}
		if requestID != "" {
			if _, e := db.Exec(`DELETE FROM requests WHERE tenant_id=? AND id=?`, tenantID, requestID); e != nil {
				t.Error(e)
			}
		}
		if appliedAssignmentID != "" {
			if _, e := db.Exec(`DELETE FROM employee_assignments WHERE tenant_id=? AND id=?`, tenantID, appliedAssignmentID); e != nil {
				t.Error(e)
			}
		}
		if rosterID != "" {
			if _, e := db.Exec(`DELETE FROM work_schedules WHERE tenant_id=? AND id=?`, tenantID, rosterID); e != nil {
				t.Error(e)
			}
		}
		if newHistoryID != "" {
			if _, e := db.Exec(`DELETE FROM employee_assignment_history WHERE tenant_id=? AND id=?`, tenantID, newHistoryID); e != nil {
				t.Error(e)
			}
		}
		if _, e := db.Exec(`DELETE FROM employee_assignments WHERE tenant_id=? AND id=?`, tenantID, qaAssignment); e != nil {
			t.Error(e)
		}
		if _, e := db.Exec(`DELETE FROM employees WHERE tenant_id=? AND id=?`, tenantID, qaEmployee); e != nil {
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
		if _, e := db.Exec(`DELETE FROM sessions WHERE user_id=?`, qaUser); e != nil {
			t.Error(e)
		}
		if _, e := db.Exec(`DELETE FROM login_events WHERE user_id=?`, qaUser); e != nil {
			t.Error(e)
		}
		if _, e := db.Exec(`DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, tenantID, qaUser); e != nil {
			t.Error(e)
		}
		if _, e := db.Exec(`DELETE FROM users WHERE id=?`, qaUser); e != nil {
			t.Error(e)
		}
	})
	if _, err = db.Exec(`INSERT INTO users (id,email,email_normalized,display_name,password_hash) VALUES (?,?,?,'QA Change Approver',?)`, qaUser, email, email, hash); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO tenant_memberships (tenant_id,user_id,status) VALUES (?,?,'active')`, tenantID, qaUser); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO user_roles (tenant_id,id,user_id,role_id,valid_from) VALUES (?,?,?,?,?)`, tenantID, qaRole, qaUser, roleID, "2000-01-01"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO employees (tenant_id,id,employee_number,display_name,status,base_office_id,position_id) SELECT ?,?,?, 'QA Change Employee','active',base_office_id,position_id FROM employees WHERE tenant_id=? AND user_id=? LIMIT 1`, tenantID, qaEmployee, "QA-"+qaEmployee, tenantID, essState.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,valid_from,current_flag,created_by) SELECT ?,?,?,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,UTC_TIMESTAMP(6)-INTERVAL 7 DAY,1,? FROM employee_assignments WHERE tenant_id=? AND employee_id=(SELECT id FROM employees WHERE tenant_id=? AND user_id=? LIMIT 1) AND current_flag=1`, tenantID, qaAssignment, qaEmployee, qaUser, tenantID, tenantID, essState.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO approval_flows (tenant_id,id,request_type,name,is_default,valid_from) VALUES (?,?,'mutation','QA Change Flow',TRUE,'2000-01-01')`, tenantID, flowID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO approval_flow_steps (tenant_id,id,flow_id,level_no,approver_kind,approver_ref_id,required_permission) VALUES (?,?,?,1,'user',?,'mutasi.approve')`, tenantID, stepID, flowID, qaUser); err != nil {
		t.Fatal(err)
	}
	doJSON(t, approver, "POST", server.URL+"/api/v1/auth/login", map[string]string{"email": email, "password": password}, 200, nil)
	var officeID, positionID, essEmployeeID, oldManager string
	if err = db.QueryRow(`SELECT office_id,position_id,COALESCE(reports_to_employee_id,'') FROM employee_assignments WHERE tenant_id=? AND id=?`, tenantID, qaAssignment).Scan(&officeID, &positionID, &oldManager); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? AND user_id=?`, tenantID, essState.User.ID).Scan(&essEmployeeID); err != nil {
		t.Fatal(err)
	}
	newManager := essEmployeeID
	if oldManager == newManager {
		newManager = ""
	}
	start := tenantToday("Asia/Jakarta")
	parsed, _ := time.Parse("2006-01-02", start)
	tomorrow := parsed.AddDate(0, 0, 1).Format("2006-01-02")
	input := map[string]any{"employee_id": qaEmployee, "change_type": "mutasi", "effective_date": tomorrow, "office_id": officeID, "position_id": positionID, "reports_to_employee_id": newManager, "reason": "Perubahan penugasan QA terjadwal"}
	var created struct {
		Data struct {
			ID      string `json:"id"`
			Version uint64 `json:"version"`
		} `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/admin/employee-changes", nil, 403, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/employee-changes", input, 201, &created)
	changeID = created.Data.ID
	if changeID == "" || created.Data.Version != 1 {
		t.Fatalf("create: %#v", created)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/employee-changes", input, 409, nil)
	var detail struct {
		Data employeeChange `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/employee-changes/"+changeID, nil, 200, &detail)
	if detail.Data.Status != "draft" {
		t.Fatalf("detail: %#v", detail.Data)
	}
	input["version"] = uint64(1)
	doJSON(t, admin, "PATCH", server.URL+"/api/v1/admin/employee-changes/"+changeID, input, 200, nil)
	doJSON(t, admin, "PATCH", server.URL+"/api/v1/admin/employee-changes/"+changeID, input, 409, nil)
	var submitted struct {
		Data struct {
			RequestID       string `json:"request_id"`
			ApprovalVersion uint64 `json:"approval_version"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/employee-changes/"+changeID+"/submit", nil, 200, &submitted)
	requestID = submitted.Data.RequestID
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/employee-changes/"+changeID, nil, 200, &detail)
	chain, ok := detail.Data.Approval.(map[string]any)
	if !ok || len(chain["tasks"].([]any)) != 1 {
		t.Fatalf("approval chain: %#v", detail.Data.Approval)
	}
	if err = db.QueryRow(`SELECT id FROM approval_instances WHERE tenant_id=? AND request_id=?`, tenantID, requestID).Scan(&instanceID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, approver, "POST", server.URL+"/api/v1/admin/approvals/"+requestID+"/actions", map[string]any{"action": "return", "reason": "Perbaiki rincian mutasi QA", "level_no": 1, "version": submitted.Data.ApprovalVersion}, 200, nil)
	input["version"] = uint64(4)
	doJSON(t, admin, "PATCH", server.URL+"/api/v1/admin/employee-changes/"+changeID, input, 200, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/employee-changes/"+changeID+"/submit", nil, 200, &submitted)
	doJSON(t, approver, "POST", server.URL+"/api/v1/admin/approvals/"+requestID+"/actions", map[string]any{"action": "approve", "level_no": 1, "version": submitted.Data.ApprovalVersion}, 200, nil)
	if applied, err := RunDueEmployeeChanges(ctx, db); err != nil || applied != 0 {
		t.Fatalf("future change applied early: %d %v", applied, err)
	}
	if _, err = db.Exec(`UPDATE employee_changes SET effective_date=? WHERE tenant_id=? AND id=?`, start, tenantID, changeID); err != nil {
		t.Fatal(err)
	}
	rosterID, _ = ids.New()
	if _, err = db.Exec(`INSERT INTO work_schedules (tenant_id,id,employee_id,work_date,day_type,status,created_by,published_at) VALUES (?,?,?,?,'off','published',?,UTC_TIMESTAMP(6))`, tenantID, rosterID, qaEmployee, start, adminState.User.ID); err != nil {
		t.Fatal(err)
	}
	if applied, err := RunDueEmployeeChanges(ctx, db); err != nil || applied != 0 {
		t.Fatalf("locked roster applied: %d %v", applied, err)
	}
	var applyError string
	if err = db.QueryRow(`SELECT apply_error FROM employee_changes WHERE tenant_id=? AND id=?`, tenantID, changeID).Scan(&applyError); err != nil || applyError != "HISTORY_LOCKED" {
		t.Fatalf("missing apply conflict: %q %v", applyError, err)
	}
	if _, err = db.Exec(`DELETE FROM work_schedules WHERE tenant_id=? AND id=?`, tenantID, rosterID); err != nil {
		t.Fatal(err)
	}
	rosterID = ""
	if applied, err := RunDueEmployeeChanges(ctx, db); err != nil || applied != 1 {
		t.Fatalf("due change not applied: %d %v", applied, err)
	}
	if applied, err := RunDueEmployeeChanges(ctx, db); err != nil || applied != 0 {
		t.Fatalf("duplicate apply: %d %v", applied, err)
	}
	var status string
	if err = db.QueryRow(`SELECT status,applied_assignment_id FROM employee_changes WHERE tenant_id=? AND id=?`, tenantID, changeID).Scan(&status, &appliedAssignmentID); err != nil || status != "applied" || appliedAssignmentID == "" {
		t.Fatalf("applied state: %s %s %v", status, appliedAssignmentID, err)
	}
	if err = db.QueryRow(`SELECT source_history_id FROM employee_assignments WHERE tenant_id=? AND id=?`, tenantID, appliedAssignmentID).Scan(&newHistoryID); err != nil {
		t.Fatal(err)
	}
	var current string
	if err = db.QueryRow(`SELECT id FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=1`, tenantID, qaEmployee).Scan(&current); err != nil || current != appliedAssignmentID {
		t.Fatalf("current assignment: %s %v", current, err)
	}
	var timeline struct {
		Data struct {
			Events []employeeTimelineEvent `json:"events"`
		} `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/employees/"+qaEmployee+"/timeline", nil, 200, &timeline)
	seenChange := false
	for _, event := range timeline.Data.Events {
		if event.Type == "employee_change_request" && event.ReferenceID == changeID {
			seenChange = true
		}
	}
	if !seenChange {
		t.Fatal("mutation missing from employee timeline")
	}
	for _, kind := range []string{"promosi", "demosi"} {
		input["change_type"] = kind
		input["reports_to_employee_id"] = oldManager
		delete(input, "version")
		var draft struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/employee-changes", input, 201, &draft)
		extraIDs = append(extraIDs, draft.Data.ID)
		if kind == "promosi" {
			doJSON(t, admin, "POST", server.URL+"/api/v1/admin/employee-changes/"+draft.Data.ID+"/submit", nil, 409, nil)
		}
		doJSON(t, admin, "DELETE", server.URL+"/api/v1/admin/employee-changes/"+draft.Data.ID, nil, 204, nil)
	}
}
