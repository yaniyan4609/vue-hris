package httpapi

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
)

func TestStagingSchedulePhase8(t *testing.T) {
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
	client := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, Timeout: 10 * time.Second}
	}
	admin, ess, partner := client(), client(), client()
	var adminState, essState auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &adminState)
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &essState)
	tenantID := adminState.SelectedTenantID
	var employeeID, officeID, positionID, roleID string
	if err = db.QueryRow(`SELECT e.id,a.office_id,a.position_id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE e.tenant_id=? AND e.user_id=?`, tenantID, essState.User.ID).Scan(&employeeID, &officeID, &positionID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT id FROM roles WHERE tenant_id=? AND code='KARYAWAN'`, tenantID).Scan(&roleID); err != nil {
		t.Fatal(err)
	}
	newID := func() string {
		id, e := ids.New()
		if e != nil {
			t.Fatal(e)
		}
		return id
	}
	partnerUser, partnerEmployee, partnerAssignment, partnerRole := newID(), newID(), newID(), newID()
	var swapIDs []string
	var flowID, stepID string
	var cleanupRoster, cleanupPartner func()
	password := "QA-Schedule-2099"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	email := "qa-schedule-" + partnerUser + "@example.invalid"
	if _, err = db.Exec(`INSERT INTO users (id,email,email_normalized,display_name,password_hash) VALUES (?,?,?,'QA Schedule Partner',?)`, partnerUser, email, email, hash); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, id := range swapIDs {
			if _, e := db.Exec(`DELETE FROM notifications WHERE tenant_id=? AND dedupe_key LIKE ?`, tenantID, "shift-swap:"+id+":%"); e != nil {
				t.Error(e)
			}
			var requestID string
			_ = db.QueryRow(`SELECT request_id FROM shift_swaps WHERE tenant_id=? AND id=?`, tenantID, id).Scan(&requestID)
			if requestID != "" {
				if _, e := db.Exec(`UPDATE shift_swaps SET request_id=NULL WHERE tenant_id=? AND id=?`, tenantID, id); e != nil {
					t.Error(e)
				}
			}
			if requestID != "" {
				for _, q := range []string{`DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_id=?`, `DELETE FROM approval_actions WHERE tenant_id=? AND instance_id IN (SELECT id FROM approval_instances WHERE tenant_id=? AND request_id=?)`, `DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id IN (SELECT id FROM approval_instances WHERE tenant_id=? AND request_id=?)`, `DELETE FROM approval_instances WHERE tenant_id=? AND request_id=?`, `DELETE FROM request_revisions WHERE tenant_id=? AND request_id=?`, `DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='requests' AND entity_id=?`, `DELETE FROM requests WHERE tenant_id=? AND id=?`} {
					args := []any{tenantID, requestID}
					if strings.Contains(q, "SELECT id FROM approval_instances") {
						args = []any{tenantID, tenantID, requestID}
					}
					if _, e := db.Exec(q, args...); e != nil {
						t.Errorf("cleanup approval: %v", e)
					}
				}
			}
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='shift_swaps' AND entity_id=?`, tenantID, id); e != nil {
				t.Error(e)
			}
			if _, e := db.Exec(`DELETE FROM shift_swaps WHERE tenant_id=? AND id=?`, tenantID, id); e != nil {
				t.Error(e)
			}
		}
		if flowID != "" {
			db.Exec(`DELETE FROM approval_flow_steps WHERE tenant_id=? AND id=?`, tenantID, stepID)
			db.Exec(`DELETE FROM approval_flows WHERE tenant_id=? AND id=?`, tenantID, flowID)
		}
		if cleanupRoster != nil {
			cleanupRoster()
		}
		if cleanupPartner != nil {
			cleanupPartner()
		}
		var remaining int
		if e := db.QueryRow(`SELECT (SELECT COUNT(*) FROM users WHERE id=?)+(SELECT COUNT(*) FROM shift_templates WHERE tenant_id=? AND LEFT(code,4)='QA8_')`, partnerUser, tenantID).Scan(&remaining); e != nil || remaining != 0 {
			t.Errorf("QA schedule fixture remains: count=%d err=%v", remaining, e)
		}
	})
	if _, err = db.Exec(`INSERT INTO tenant_memberships (tenant_id,user_id,status) VALUES (?,?,'active')`, tenantID, partnerUser); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO user_roles (tenant_id,id,user_id,role_id,valid_from) VALUES (?,?,?,?,CURRENT_DATE())`, tenantID, partnerRole, partnerUser, roleID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO employees (tenant_id,id,user_id,employee_number,display_name,status,base_office_id,position_id) VALUES (?,?,?,?,'QA Schedule Partner','active',?,?)`, tenantID, partnerEmployee, partnerUser, "QA-"+partnerEmployee, officeID, positionID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,valid_from,current_flag,created_by) SELECT ?,?,?,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,UTC_TIMESTAMP(6)-INTERVAL 7 DAY,1,? FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=1`, tenantID, partnerAssignment, partnerEmployee, adminState.User.ID, tenantID, employeeID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE tenant_memberships SET employee_id=? WHERE tenant_id=? AND user_id=?`, partnerEmployee, tenantID, partnerUser); err != nil {
		t.Fatal(err)
	}
	cleanupPartner = func() {
		for _, q := range []struct {
			SQL  string
			Args []any
		}{{`DELETE FROM sessions WHERE user_id=?`, []any{partnerUser}}, {`DELETE FROM login_events WHERE user_id=?`, []any{partnerUser}}, {`DELETE FROM user_roles WHERE tenant_id=? AND id=?`, []any{tenantID, partnerRole}}, {`UPDATE tenant_memberships SET employee_id=NULL WHERE tenant_id=? AND user_id=?`, []any{tenantID, partnerUser}}, {`DELETE FROM employee_assignments WHERE tenant_id=? AND id=?`, []any{tenantID, partnerAssignment}}, {`DELETE FROM employees WHERE tenant_id=? AND id=?`, []any{tenantID, partnerEmployee}}, {`DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, []any{tenantID, partnerUser}}, {`DELETE FROM users WHERE id=?`, []any{partnerUser}}} {
			if _, e := db.Exec(q.SQL, q.Args...); e != nil {
				t.Errorf("cleanup partner: %v", e)
			}
		}
	}
	doJSON(t, partner, "POST", server.URL+"/api/v1/auth/login", map[string]string{"email": email, "password": password}, 200, nil)
	shifts := []shiftTemplate{{Code: "QA8_" + newID(), Name: "QA Day", StartTime: "09:00", EndTime: "17:00", ClockInWindowStartTime: "08:00", ClockInWindowEndTime: "10:00", ClockOutWindowStartTime: "16:00", ClockOutWindowEndTime: "18:00", IsActive: true}, {Code: "QA8_" + newID(), Name: "QA Night", StartTime: "22:00", EndTime: "06:00", CrossesMidnight: true, ClockInWindowStartTime: "21:00", ClockInWindowEndTime: "23:00", ClockOutWindowStartTime: "05:00", ClockOutWindowStartDayOffset: 1, ClockOutWindowEndTime: "07:00", ClockOutWindowEndDayOffset: 1, IsActive: true}}
	for i := range shifts {
		var out struct {
			Data shiftTemplate `json:"data"`
		}
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/shift-templates", shifts[i], 201, &out)
		shifts[i] = out.Data
	}
	var rosters []workSchedule
	var instances []string
	for i, employee := range []string{employeeID, partnerEmployee} {
		date := []string{"2099-11-11", "2099-11-12"}[i]
		var out struct {
			Data workSchedule `json:"data"`
		}
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/work-schedules", map[string]any{"employee_id": employee, "work_date": date, "day_type": "shift", "shift_template_id": shifts[i].ID, "office_id": officeID}, 201, &out)
		rosters = append(rosters, out.Data)
		var published struct {
			Data struct {
				Published []publishedInstance `json:"published"`
			} `json:"data"`
		}
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/work-schedules/publish", map[string]any{"from": date, "to": date, "employee_ids": []string{employee}}, 200, &published)
		if len(published.Data.Published) != 1 || published.Data.Published[0].InstanceID == nil {
			t.Fatalf("publish: %#v", published)
		}
		instances = append(instances, *published.Data.Published[0].InstanceID)
	}
	cleanupRoster = func() {
		for _, id := range instances {
			for _, q := range []string{`DELETE FROM notification_schedules WHERE tenant_id=? AND subject_type='shift_instance' AND subject_id=?`, `DELETE FROM attendance_daily WHERE tenant_id=? AND shift_instance_id=?`, `DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='shift_instances' AND entity_id=?`, `DELETE FROM shift_instances WHERE tenant_id=? AND id=?`} {
				if _, e := db.Exec(q, tenantID, id); e != nil {
					t.Errorf("cleanup instance: %v", e)
				}
			}
		}
		for _, roster := range rosters {
			for _, q := range []string{`DELETE FROM notifications WHERE tenant_id=? AND dedupe_key=?`, `DELETE FROM work_schedule_revisions WHERE tenant_id=? AND work_schedule_id=?`, `DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='work_schedules' AND entity_id=?`, `DELETE FROM work_schedules WHERE tenant_id=? AND id=?`} {
				value := roster.ID
				if q == `DELETE FROM notifications WHERE tenant_id=? AND dedupe_key=?` {
					value = "roster:" + value
				}
				if _, e := db.Exec(q, tenantID, value); e != nil {
					t.Errorf("cleanup roster: %v", e)
				}
			}
		}
		for _, shift := range shifts {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='shift_templates' AND entity_id=?`, tenantID, shift.ID); e != nil {
				t.Error(e)
			}
			if _, e := db.Exec(`DELETE FROM shift_templates WHERE tenant_id=? AND id=?`, tenantID, shift.ID); e != nil {
				t.Error(e)
			}
		}
	}
	var grid struct {
		Data struct {
			Employees []struct {
				EmployeeID string         `json:"employee_id"`
				Days       []workSchedule `json:"days"`
			} `json:"employees"`
		} `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/roster?office_id="+officeID+"&from=2099-11-11&to=2099-11-12", nil, 200, &grid)
	if len(grid.Data.Employees) < 2 {
		t.Fatalf("grid missing employees: %#v", grid.Data)
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/admin/roster?office_id="+officeID+"&from=2099-11-11&to=2099-11-12", nil, 403, nil)
	if _, err = db.Exec(`DELETE FROM shift_instances WHERE tenant_id=? AND id=?`, tenantID, instances[0]); err != nil {
		t.Fatal(err)
	}
	var generated struct {
		Data struct {
			Generated []shiftInstance `json:"generated"`
			Count     int             `json:"count"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/shift-instances/generate", map[string]any{"from": "2099-11-11", "to": "2099-11-12", "employee_ids": []string{employeeID}}, 200, &generated)
	if generated.Data.Count != 1 {
		t.Fatalf("generate: %#v", generated.Data)
	}
	instances[0] = generated.Data.Generated[0].ID
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/shift-instances/generate", map[string]any{"from": "2099-11-11", "to": "2099-11-12", "employee_ids": []string{employeeID}}, 200, &generated)
	if generated.Data.Count != 0 {
		t.Fatalf("generate retry: %#v", generated.Data)
	}
	var proposed struct {
		Data struct {
			ID      string `json:"id"`
			Version uint64 `json:"version"`
		} `json:"data"`
	}
	swapURL := server.URL + "/api/v1/ess/shift-swaps"
	doJSON(t, ess, "POST", swapURL, map[string]any{"instance_a_id": instances[0], "instance_b_id": instances[1], "reason": "QA bertukar jadwal"}, 201, &proposed)
	swapIDs = append(swapIDs, proposed.Data.ID)
	doJSON(t, ess, "POST", swapURL, map[string]any{"instance_a_id": instances[0], "instance_b_id": instances[1], "reason": "Duplikat"}, 409, nil)
	doJSON(t, partner, "POST", swapURL+"/"+proposed.Data.ID+"/respond", map[string]any{"accept": false, "version": proposed.Data.Version}, 200, nil)
	var accepted struct {
		Data struct {
			RequestID       string `json:"request_id"`
			ApprovalVersion uint64 `json:"approval_version"`
		} `json:"data"`
	}
	var defaultFlow int
	if err = db.QueryRow(`SELECT COUNT(*) FROM approval_flows WHERE tenant_id=? AND request_type='shift_swap' AND is_default=TRUE`, tenantID).Scan(&defaultFlow); err != nil {
		t.Fatal(err)
	}
	if defaultFlow == 0 {
		flowID, stepID = newID(), newID()
		if _, err = db.Exec(`INSERT INTO approval_flows (tenant_id,id,request_type,name,is_default,valid_from) VALUES (?,?,'shift_swap','QA Shift Swap',TRUE,'2000-01-01')`, tenantID, flowID); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO approval_flow_steps (tenant_id,id,flow_id,level_no,approver_kind,approver_ref_id,required_permission) VALUES (?,?,?,1,'user',?,'jadwal.approve')`, tenantID, stepID, flowID, adminState.User.ID); err != nil {
			t.Fatal(err)
		}
		doJSON(t, ess, "POST", swapURL, map[string]any{"instance_a_id": instances[0], "instance_b_id": instances[1], "reason": "QA tukar lintas malam"}, 201, &proposed)
		swapIDs = append(swapIDs, proposed.Data.ID)
		doJSON(t, partner, "POST", swapURL+"/"+proposed.Data.ID+"/respond", map[string]any{"accept": true, "version": proposed.Data.Version}, 200, &accepted)
		if accepted.Data.RequestID == "" {
			t.Fatal("request not created")
		}
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+accepted.Data.RequestID+"/actions", map[string]any{"action": "approve", "level_no": 1, "version": accepted.Data.ApprovalVersion}, 200, nil)
		var firstAfter, secondAfter shiftInstance
		if err = db.QueryRow(`SELECT shift_template_id,expected_start,expected_end FROM shift_instances WHERE tenant_id=? AND id=?`, tenantID, instances[0]).Scan(&firstAfter.ShiftTemplateID, &firstAfter.ExpectedStart, &firstAfter.ExpectedEnd); err != nil {
			t.Fatal(err)
		}
		if err = db.QueryRow(`SELECT shift_template_id FROM shift_instances WHERE tenant_id=? AND id=?`, tenantID, instances[1]).Scan(&secondAfter.ShiftTemplateID); err != nil {
			t.Fatal(err)
		}
		if firstAfter.ShiftTemplateID != shifts[1].ID || secondAfter.ShiftTemplateID != shifts[0].ID || firstAfter.ExpectedEnd.Sub(firstAfter.ExpectedStart) != 8*time.Hour {
			t.Fatalf("swap not applied: first=%#v second=%#v", firstAfter, secondAfter)
		}
		doJSON(t, ess, "POST", swapURL, map[string]any{"instance_a_id": instances[0], "instance_b_id": instances[1], "reason": "QA batalkan sesudah diterima"}, 201, &proposed)
		swapIDs = append(swapIDs, proposed.Data.ID)
		doJSON(t, partner, "POST", swapURL+"/"+proposed.Data.ID+"/respond", map[string]any{"accept": true, "version": proposed.Data.Version}, 200, &accepted)
		doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+accepted.Data.RequestID+"/cancel", map[string]any{}, 200, nil)
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+accepted.Data.RequestID+"/actions", map[string]any{"action": "approve", "level_no": 1, "version": accepted.Data.ApprovalVersion}, 409, nil)
		doJSON(t, ess, "POST", swapURL, map[string]any{"instance_a_id": instances[0], "instance_b_id": instances[1], "reason": "QA return dan kirim ulang"}, 201, &proposed)
		swapIDs = append(swapIDs, proposed.Data.ID)
		doJSON(t, partner, "POST", swapURL+"/"+proposed.Data.ID+"/respond", map[string]any{"accept": true, "version": proposed.Data.Version}, 200, &accepted)
		var returned struct {
			Data struct {
				Version uint64 `json:"version"`
			} `json:"data"`
		}
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+accepted.Data.RequestID+"/actions", map[string]any{"action": "return", "reason": "Jelaskan pertukaran QA", "level_no": 1, "version": accepted.Data.ApprovalVersion}, 200, &returned)
		doJSON(t, ess, "POST", swapURL, map[string]any{"instance_a_id": instances[0], "instance_b_id": instances[1], "reason": "Bentrok dengan returned"}, 409, nil)
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/shift-instances/"+instances[0]+"/cancel", map[string]any{"reason": "Masih ditinjau"}, 409, nil)
		var resubmitted struct {
			Data struct {
				ApprovalVersion uint64 `json:"approval_version"`
				RequestID       string `json:"request_id"`
			} `json:"data"`
		}
		doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+accepted.Data.RequestID+"/resubmit", map[string]any{"version": returned.Data.Version, "correction": map[string]any{"note": "Jadwal sudah disepakati bersama."}}, 200, &resubmitted)
		if resubmitted.Data.RequestID != accepted.Data.RequestID {
			t.Fatal("resubmit created different request")
		}
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/approvals/"+accepted.Data.RequestID+"/actions", map[string]any{"action": "approve", "level_no": 1, "version": resubmitted.Data.ApprovalVersion}, 200, nil)
	}
	var third struct {
		Data workSchedule `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/work-schedules", map[string]any{"employee_id": employeeID, "work_date": "2099-11-13", "day_type": "shift", "shift_template_id": shifts[0].ID, "office_id": officeID}, 201, &third)
	rosters = append(rosters, third.Data)
	var thirdPublished struct {
		Data struct {
			Published []publishedInstance `json:"published"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/work-schedules/publish", map[string]any{"from": "2099-11-13", "to": "2099-11-13", "employee_ids": []string{employeeID}}, 200, &thirdPublished)
	if len(thirdPublished.Data.Published) != 1 || thirdPublished.Data.Published[0].InstanceID == nil {
		t.Fatal("third shift not published")
	}
	thirdID := *thirdPublished.Data.Published[0].InstanceID
	instances = append(instances, thirdID)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/shift-instances/"+thirdID+"/cancel", map[string]any{"reason": "QA batalkan jadwal"}, 200, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/shift-instances/"+thirdID+"/cancel", map[string]any{"reason": "Duplikat"}, 409, nil)
	workDate, _ := time.Parse("2006-01-02", tenantToday("Asia/Jakarta"))
	workerDate := workDate.AddDate(0, 0, 7).Format("2006-01-02")
	var workerRoster struct {
		Data workSchedule `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/work-schedules", map[string]any{"employee_id": partnerEmployee, "work_date": workerDate, "day_type": "shift", "shift_template_id": shifts[0].ID, "office_id": officeID}, 201, &workerRoster)
	rosters = append(rosters, workerRoster.Data)
	var workerPublished struct {
		Data struct {
			Published []publishedInstance `json:"published"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/work-schedules/publish", map[string]any{"from": workerDate, "to": workerDate, "employee_ids": []string{partnerEmployee}}, 200, &workerPublished)
	if len(workerPublished.Data.Published) != 1 || workerPublished.Data.Published[0].InstanceID == nil {
		t.Fatal("worker fixture not published")
	}
	if _, err = db.Exec(`DELETE FROM shift_instances WHERE tenant_id=? AND id=?`, tenantID, *workerPublished.Data.Published[0].InstanceID); err != nil {
		t.Fatal(err)
	}
	created, err := materializeDueRoster(ctx, db, tenantID, workerRoster.Data.ID)
	if err != nil || !created {
		t.Fatalf("worker materialization: created=%t err=%v", created, err)
	}
	created, err = materializeDueRoster(ctx, db, tenantID, workerRoster.Data.ID)
	if err != nil || created {
		t.Fatalf("worker retry: created=%t err=%v", created, err)
	}
	var workerInstanceID string
	if err = db.QueryRow(`SELECT id FROM shift_instances WHERE tenant_id=? AND work_schedule_id=?`, tenantID, workerRoster.Data.ID).Scan(&workerInstanceID); err != nil {
		t.Fatal(err)
	}
	instances = append(instances, workerInstanceID)
	var list struct {
		Data []shiftSwap `json:"data"`
	}
	doJSON(t, ess, "GET", swapURL, nil, 200, &list)
	if len(list.Data) < 1 {
		t.Fatal("ESS swap list empty")
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/shift-swaps", nil, 200, &list)
}
