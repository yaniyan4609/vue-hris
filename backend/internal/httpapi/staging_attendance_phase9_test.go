package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
)

func TestStagingAttendancePhase9Reads(t *testing.T) {
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
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 15 * time.Second}
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, nil)
	for _, path := range []string{
		"/api/v1/admin/attendance/today?date=2099-11-11",
		"/api/v1/admin/attendance/not-yet?date=2099-11-11",
		"/api/v1/admin/attendance/daily?from=2099-11-11&to=2099-11-11",
		"/api/v1/admin/attendance/summary?from=2099-11-11&to=2099-11-11",
		"/api/v1/admin/attendance/summary?from=2099-11-11&to=2099-11-11&group_by=department",
		"/api/v1/admin/attendance/events?from=2099-11-11&to=2099-11-11",
		"/api/v1/admin/attendance/corrections",
	} {
		doJSON(t, client, "GET", server.URL+path, nil, 200, nil)
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/attendance/events/invalid", nil, 404, nil)
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/ess/attendance/corrections", nil, 200, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/attendance/events", nil, 403, nil)
	doJSON(t, client, "POST", server.URL+"/api/v1/ess/attendance/clock-in", map[string]any{"client_event_id": "bad", "lat": -6.2, "lng": 106.8, "accuracy_m": 12}, 400, nil)
}

func TestStagingAttendanceClockIdempotency(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	t.Setenv("FILE_STORAGE_DIR", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 15 * time.Second}
	var state auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &state)
	var employeeID, officeID string
	err = db.QueryRow(`SELECT m.employee_id,a.office_id FROM tenant_memberships m JOIN employee_assignments a ON a.tenant_id=m.tenant_id AND a.employee_id=m.employee_id AND a.current_flag=TRUE WHERE m.tenant_id=? AND m.user_id=? LIMIT 1`, state.SelectedTenantID, state.User.ID).Scan(&employeeID, &officeID)
	if err != nil {
		t.Fatal(err)
	}
	shiftID, _ := ids.New()
	_, err = db.Exec(`INSERT INTO shift_templates (tenant_id,id,code,name,start_time,end_time,clock_in_window_start_time,clock_in_window_end_time,clock_out_window_start_time,clock_out_window_end_time) VALUES (?,?,?,'QA Attendance','09:00','17:00','08:00','10:00','16:00','19:00')`, state.SelectedTenantID, shiftID, "QA_"+shiftID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, e := db.Exec(`DELETE FROM shift_templates WHERE tenant_id=? AND id=?`, state.SelectedTenantID, shiftID); e != nil {
			t.Errorf("cleanup shift: %v", e)
		}
	}()
	rosterID, _ := ids.New()
	instanceID, _ := ids.New()
	inClientID, _ := ids.New()
	outClientID, _ := ids.New()
	var inEventID, outEventID string
	var selfieInID, selfieOutID string
	workDate := "2099-12-17"
	now := time.Now().UTC()
	_, err = db.Exec(`INSERT INTO work_schedules (tenant_id,id,employee_id,work_date,shift_template_id,day_type,office_id,status,published_at) VALUES (?,?,?,?,?,'shift',?,'published',UTC_TIMESTAMP(6))`, state.SelectedTenantID, rosterID, employeeID, workDate, shiftID, officeID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, entry := range []struct {
			query string
			args  []any
		}{
			{`DELETE FROM notifications WHERE tenant_id=? AND dedupe_key IN (?,?)`, []any{state.SelectedTenantID, "attendance:" + inEventID, "attendance:" + outEventID}},
			{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='attendance_events' AND entity_id IN (SELECT id FROM attendance_events WHERE tenant_id=? AND shift_instance_id=?)`, []any{state.SelectedTenantID, state.SelectedTenantID, instanceID}},
			{`DELETE FROM attendance_daily WHERE tenant_id=? AND shift_instance_id=?`, []any{state.SelectedTenantID, instanceID}},
			{`DELETE FROM attendance_events WHERE tenant_id=? AND shift_instance_id=?`, []any{state.SelectedTenantID, instanceID}},
			{`DELETE FROM files WHERE tenant_id=? AND id IN (?,?)`, []any{state.SelectedTenantID, selfieInID, selfieOutID}},
			{`DELETE FROM shift_instances WHERE tenant_id=? AND id=?`, []any{state.SelectedTenantID, instanceID}},
			{`DELETE FROM work_schedules WHERE tenant_id=? AND id=?`, []any{state.SelectedTenantID, rosterID}},
		} {
			if _, e := db.Exec(entry.query, entry.args...); e != nil {
				t.Errorf("cleanup: %v", e)
			}
		}
	}()
	snapshot := `{"shift":{"late_tolerance_min":5,"break_minutes":0,"require_location":false,"require_selfie":true,"require_face_recognition":false}}`
	_, err = db.Exec(`INSERT INTO shift_instances (tenant_id,id,employee_id,work_schedule_id,shift_template_id,office_id,work_date,timezone,expected_start,expected_end,clock_in_window_start_at,clock_in_window_end_at,clock_out_window_start_at,clock_out_window_end_at,rule_snapshot,status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'scheduled')`, state.SelectedTenantID, instanceID, employeeID, rosterID, shiftID, officeID, workDate, "Asia/Jakarta", now.Add(-5*time.Minute), now.Add(30*time.Minute), now.Add(-time.Hour), now.Add(time.Hour), now.Add(-time.Hour), now.Add(time.Hour), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	picture := image.NewRGBA(image.Rect(0, 0, 1, 1))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	var raw bytes.Buffer
	if err = png.Encode(&raw, picture); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw.Bytes())
	uploadSelfie := func() string {
		var upload struct {
			Data struct {
				FileID string `json:"file_id"`
			} `json:"data"`
		}
		doJSON(t, client, "POST", server.URL+"/api/v1/files/uploads", map[string]any{"purpose": "selfie", "original_name": "qa.png", "mime_type": "image/png", "size_bytes": raw.Len(), "sha256": hex.EncodeToString(hash[:])}, 201, &upload)
		id := upload.Data.FileID
		req, _ := http.NewRequest("PUT", server.URL+"/api/v1/files/"+id+"/content", bytes.NewReader(raw.Bytes()))
		req.Header.Set("Content-Type", "image/png")
		req.Header.Set("X-CSRF-Token", csrfToken(cookieFromJar(t, client, server.URL)))
		response, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		if response.StatusCode != 204 {
			t.Fatalf("selfie upload: %d", response.StatusCode)
		}
		doJSON(t, client, "POST", server.URL+"/api/v1/files/"+id+"/complete", map[string]any{}, 200, nil)
		return id
	}
	selfieInID = uploadSelfie()
	selfieOutID = uploadSelfie()
	doJSON(t, client, "POST", server.URL+"/api/v1/ess/attendance/clock-in", map[string]any{"client_event_id": "gps-" + inClientID, "selfie_file_id": selfieInID, "lat": -6.2, "lng": 106.8, "accuracy_m": 101}, 422, nil)
	faceSnapshot := `{"shift":{"late_tolerance_min":5,"break_minutes":0,"require_location":false,"require_selfie":true,"require_face_recognition":true}}`
	if _, err = db.Exec(`UPDATE shift_instances SET rule_snapshot=? WHERE tenant_id=? AND id=?`, faceSnapshot, state.SelectedTenantID, instanceID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/ess/attendance/clock-in", map[string]any{"client_event_id": "face-" + inClientID, "selfie_file_id": selfieInID}, 503, nil)
	if _, err = db.Exec(`UPDATE shift_instances SET rule_snapshot=? WHERE tenant_id=? AND id=?`, snapshot, state.SelectedTenantID, instanceID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/ess/attendance/clock-in", map[string]any{"client_event_id": "missing-" + inClientID}, 422, nil)
	var in struct {
		Data essAttendanceEvent `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/ess/attendance/clock-in", map[string]any{"client_event_id": inClientID, "selfie_file_id": selfieInID}, 201, &in)
	if in.Data.ID == "" || in.Data.ShiftInstanceID != instanceID {
		t.Fatalf("clock in: %#v", in)
	}
	inEventID = in.Data.ID
	var retry struct {
		Data essAttendanceEvent `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/ess/attendance/clock-in", map[string]any{"client_event_id": inClientID, "selfie_file_id": selfieInID}, 200, &retry)
	if retry.Data.ID != in.Data.ID {
		t.Fatal("retry created a new event")
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/ess/attendance/clock-in", map[string]any{"client_event_id": "other-" + inClientID, "selfie_file_id": selfieOutID}, 409, nil)
	type clockResponse struct {
		Status int
		Event  essAttendanceEvent
		Error  error
	}
	responses := make([]clockResponse, 2)
	clockCSRF := csrfToken(cookieFromJar(t, client, server.URL))
	var wg sync.WaitGroup
	for index := range responses {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			payload, _ := json.Marshal(map[string]any{"client_event_id": outClientID, "selfie_file_id": selfieOutID})
			req, e := http.NewRequest("POST", server.URL+"/api/v1/ess/attendance/clock-out", bytes.NewReader(payload))
			if e != nil {
				responses[index].Error = e
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-CSRF-Token", clockCSRF)
			reply, e := client.Do(req)
			if e != nil {
				responses[index].Error = e
				return
			}
			defer reply.Body.Close()
			responses[index].Status = reply.StatusCode
			var body struct {
				Data essAttendanceEvent `json:"data"`
			}
			responses[index].Error = json.NewDecoder(reply.Body).Decode(&body)
			responses[index].Event = body.Data
		}(index)
	}
	wg.Wait()
	if responses[0].Error != nil || responses[1].Error != nil || !((responses[0].Status == 201 && responses[1].Status == 200) || (responses[0].Status == 200 && responses[1].Status == 201)) || responses[0].Event.ID == "" || responses[0].Event.ID != responses[1].Event.ID {
		t.Fatalf("parallel clock-out: %#v", responses)
	}
	out := struct{ Data essAttendanceEvent }{Data: responses[0].Event}
	if out.Data.ID == "" || out.Data.ID == in.Data.ID {
		t.Fatal("clock out event missing")
	}
	outEventID = out.Data.ID
	var events, daily int
	if err = db.QueryRow(`SELECT COUNT(*) FROM attendance_events WHERE tenant_id=? AND shift_instance_id=?`, state.SelectedTenantID, instanceID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM attendance_daily WHERE tenant_id=? AND shift_instance_id=? AND first_in IS NOT NULL AND last_out IS NOT NULL`, state.SelectedTenantID, instanceID).Scan(&daily); err != nil {
		t.Fatal(err)
	}
	if events != 2 || daily != 1 {
		t.Fatalf("events=%d daily=%d", events, daily)
	}
	adminJar, _ := cookiejar.New(nil)
	adminClient := &http.Client{Jar: adminJar, Timeout: 15 * time.Second}
	var admin auth.State
	doJSON(t, adminClient, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &admin)
	doJSON(t, client, "DELETE", server.URL+"/api/v1/files/"+selfieInID, nil, 409, nil)
	doJSON(t, adminClient, "GET", server.URL+"/api/v1/files/"+selfieInID, nil, 200, nil)
	flowID, _ := ids.New()
	stepID, _ := ids.New()
	var correctionID, correctionRequestID string
	_, err = db.Exec(`INSERT INTO approval_flows (tenant_id,id,request_type,name,is_default,valid_from,is_active) VALUES (?,?,'attendance_correction','QA Correction',TRUE,'2020-01-01',TRUE)`, state.SelectedTenantID, flowID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, entry := range []struct {
			query string
			args  []any
		}{
			{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='requests' AND entity_id=?`, []any{state.SelectedTenantID, correctionRequestID}},
			{`DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_id=?`, []any{state.SelectedTenantID, correctionRequestID}},
			{`DELETE FROM request_revisions WHERE tenant_id=? AND request_id=?`, []any{state.SelectedTenantID, correctionRequestID}},
			{`DELETE FROM approval_actions WHERE tenant_id=? AND instance_id IN (SELECT id FROM approval_instances WHERE tenant_id=? AND flow_id=?)`, []any{state.SelectedTenantID, state.SelectedTenantID, flowID}},
			{`DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id IN (SELECT id FROM approval_instances WHERE tenant_id=? AND flow_id=?)`, []any{state.SelectedTenantID, state.SelectedTenantID, flowID}},
			{`DELETE FROM approval_instances WHERE tenant_id=? AND flow_id=?`, []any{state.SelectedTenantID, flowID}},
			{`DELETE FROM attendance_corrections WHERE tenant_id=? AND id=?`, []any{state.SelectedTenantID, correctionID}},
			{`DELETE FROM requests WHERE tenant_id=? AND id=?`, []any{state.SelectedTenantID, correctionRequestID}},
			{`DELETE FROM approval_flow_steps WHERE tenant_id=? AND flow_id=?`, []any{state.SelectedTenantID, flowID}},
			{`DELETE FROM approval_flows WHERE tenant_id=? AND id=?`, []any{state.SelectedTenantID, flowID}},
		} {
			if _, e := db.Exec(entry.query, entry.args...); e != nil {
				t.Errorf("cleanup correction: %v", e)
			}
		}
	}()
	_, err = db.Exec(`INSERT INTO approval_flow_steps (tenant_id,id,flow_id,level_no,approver_kind,approver_ref_id,required_permission) VALUES (?,?,?,1,'user',?,'absensi.approve')`, state.SelectedTenantID, stepID, flowID, admin.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	var correction struct {
		Data struct {
			ID        string `json:"id"`
			RequestID string `json:"request_id"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/ess/attendance/corrections", map[string]any{"shift_instance_id": instanceID, "original_event_id": in.Data.ID, "requested_type": "clock_in", "requested_time": now.Add(-4 * time.Minute).Format(time.RFC3339Nano), "reason": "QA corrected time"}, 201, &correction)
	if correction.Data.ID == "" || correction.Data.RequestID == "" {
		t.Fatal("correction not created")
	}
	correctionID, correctionRequestID = correction.Data.ID, correction.Data.RequestID
	doJSON(t, adminClient, "POST", server.URL+"/api/v1/admin/approvals/"+correction.Data.RequestID+"/actions", map[string]any{"action": "return", "reason": "Perbaiki menit masuk", "level_no": 1, "version": 1}, 200, nil)
	var returned string
	if err = db.QueryRow(`SELECT status FROM attendance_corrections WHERE tenant_id=? AND id=?`, state.SelectedTenantID, correction.Data.ID).Scan(&returned); err != nil || returned != "returned" {
		t.Fatalf("return status=%s err=%v", returned, err)
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/ess/requests/"+correction.Data.RequestID+"/resubmit", map[string]any{"version": 2, "correction": map[string]any{"note": "Waktu masuk diperbaiki", "requested_time": now.Add(-3 * time.Minute).Format(time.RFC3339Nano), "reason": "QA corrected time again"}}, 200, nil)
	doJSON(t, adminClient, "POST", server.URL+"/api/v1/admin/approvals/"+correction.Data.RequestID+"/actions", map[string]any{"action": "approve", "level_no": 1, "version": 3}, 200, nil)
	var approved, resultID string
	if err = db.QueryRow(`SELECT status,result_event_id FROM attendance_corrections WHERE tenant_id=? AND id=?`, state.SelectedTenantID, correction.Data.ID).Scan(&approved, &resultID); err != nil {
		t.Fatal(err)
	}
	if approved != "approved" || resultID == "" {
		t.Fatalf("correction=%s event=%s", approved, resultID)
	}
	doJSON(t, adminClient, "GET", server.URL+"/api/v1/admin/attendance/corrections/"+correctionID, nil, 200, nil)
	doJSON(t, adminClient, "GET", server.URL+"/api/v1/admin/attendance/events/"+resultID, nil, 200, nil)
	doJSON(t, adminClient, "GET", server.URL+"/api/v1/admin/attendance/daily?from="+workDate+"&to="+workDate+"&employee_id="+employeeID, nil, 200, nil)
	doJSON(t, adminClient, "GET", server.URL+"/api/v1/admin/attendance/summary?from="+workDate+"&to="+workDate+"&group_by=employee&employee_id="+employeeID, nil, 200, nil)
	doJSON(t, adminClient, "POST", server.URL+"/api/v1/admin/attendance/recalculate", map[string]any{"from": workDate, "to": workDate, "employee_id": employeeID}, 200, nil)
}
