package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type attendanceCorrectionInput struct {
	EmployeeID       string  `json:"employee_id"`
	ShiftInstanceID  string  `json:"shift_instance_id"`
	OriginalEventID  *string `json:"original_event_id"`
	RequestedType    string  `json:"requested_type"`
	RequestedTime    string  `json:"requested_time"`
	Reason           string  `json:"reason"`
	AttachmentFileID *string `json:"attachment_file_id"`
}

type attendanceCorrection struct {
	ID               string    `json:"id"`
	EmployeeID       string    `json:"employee_id"`
	ShiftInstanceID  string    `json:"shift_instance_id"`
	OriginalEventID  *string   `json:"original_event_id"`
	RequestedType    string    `json:"requested_type"`
	RequestedTime    time.Time `json:"requested_time"`
	Reason           string    `json:"reason"`
	AttachmentFileID *string   `json:"attachment_file_id"`
	Status           string    `json:"status"`
	RequestID        string    `json:"request_id"`
	ResultEventID    *string   `json:"result_event_id"`
	Version          uint64    `json:"version"`
	CreatedAt        time.Time `json:"created_at"`
}

const correctionColumns = `c.id,c.employee_id,c.shift_instance_id,c.original_event_id,c.requested_type,c.requested_time,c.reason,c.attachment_file_id,c.status,c.request_id,c.result_event_id,c.version,c.created_at`
const correctionAdminJoin = ` FROM attendance_corrections c JOIN employees e ON e.tenant_id=c.tenant_id AND e.id=c.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`

func scanCorrection(row interface{ Scan(...any) error }) (attendanceCorrection, error) {
	var item attendanceCorrection
	var original, attachment, result, request sql.NullString
	err := row.Scan(&item.ID, &item.EmployeeID, &item.ShiftInstanceID, &original, &item.RequestedType, &item.RequestedTime, &item.Reason, &attachment, &item.Status, &request, &result, &item.Version, &item.CreatedAt)
	if original.Valid {
		item.OriginalEventID = &original.String
	}
	if attachment.Valid {
		item.AttachmentFileID = &attachment.String
	}
	if result.Valid {
		item.ResultEventID = &result.String
	}
	if request.Valid {
		item.RequestID = request.String
	}
	return item, err
}

func validCorrectionInput(in attendanceCorrectionInput, zone *time.Location) (time.Time, bool) {
	if !fileIDPattern.MatchString(in.ShiftInstanceID) || (in.OriginalEventID != nil && !fileIDPattern.MatchString(*in.OriginalEventID)) || (in.AttachmentFileID != nil && !fileIDPattern.MatchString(*in.AttachmentFileID)) || (in.RequestedType != "clock_in" && in.RequestedType != "clock_out") || strings.TrimSpace(in.Reason) == "" || len([]rune(in.Reason)) > 2000 {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339Nano, in.RequestedTime)
	if err != nil || at.After(time.Now().Add(5*time.Minute)) || at.Before(time.Now().AddDate(0, 0, -32)) {
		return time.Time{}, false
	}
	return at.UTC(), true
}

func (a adminAPI) createEssAttendanceCorrection(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	var body attendanceCorrectionInput
	if !readJSON(w, r, &body) {
		return
	}
	body.EmployeeID = c.employeeID
	a.createAttendanceCorrection(w, r, c.state.SelectedTenantID, c.state.User.ID, c.zone, body)
}
func (a adminAPI) createAdminAttendanceCorrection(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "absensi.koreksi")
	if !ok {
		return
	}
	var body attendanceCorrectionInput
	if !readJSON(w, r, &body) {
		return
	}
	if _, ok = a.adminScopedEmployee(w, r, c, body.EmployeeID); !ok {
		return
	}
	zone, _ := time.LoadLocation(c.tenant.Timezone)
	if zone == nil {
		zone = time.UTC
	}
	a.createAttendanceCorrection(w, r, c.tenant.ID, c.state.User.ID, zone, body)
}

func (a adminAPI) createAttendanceCorrection(w http.ResponseWriter, r *http.Request, tenantID, actorID string, zone *time.Location, in attendanceCorrectionInput) {
	at, valid := validCorrectionInput(in, zone)
	if !valid {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data koreksi tidak valid atau melewati batas 32 hari.")
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat koreksi.")
		return
	}
	defer tx.Rollback()
	instance, err := scanInstance(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(instanceColumns, "i.", "")+` FROM shift_instances WHERE tenant_id=? AND id=? AND employee_id=? AND status<>'cancelled' FOR UPDATE`, tenantID, in.ShiftInstanceID, in.EmployeeID))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Shift tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
		return
	}
	if at.Before(instance.ExpectedStart.Add(-24*time.Hour)) || at.After(instance.ExpectedEnd.Add(24*time.Hour)) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Waktu koreksi terlalu jauh dari shift.")
		return
	}
	if in.OriginalEventID != nil {
		var eventType string
		err = tx.QueryRowContext(r.Context(), `SELECT event_type FROM attendance_events WHERE tenant_id=? AND id=? AND employee_id=? AND shift_instance_id=?`, tenantID, *in.OriginalEventID, in.EmployeeID, in.ShiftInstanceID).Scan(&eventType)
		if err != nil || eventType != in.RequestedType {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Event asli tidak sesuai jenis koreksi.")
			return
		}
	}
	if in.AttachmentFileID != nil {
		var count int
		err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM files WHERE tenant_id=? AND id=? AND uploaded_by=? AND purpose='selfie' AND scan_status='clean' AND deleted_at IS NULL`, tenantID, *in.AttachmentFileID, actorID).Scan(&count)
		if err != nil || count != 1 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Lampiran tidak tersedia.")
			return
		}
	}
	var active int
	err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM attendance_corrections WHERE tenant_id=? AND shift_instance_id=? AND requested_type=? AND status IN ('in_review','returned')`, tenantID, in.ShiftInstanceID, in.RequestedType).Scan(&active)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa koreksi.")
		return
	}
	if active > 0 {
		writeAPIError(w, r, 409, "CORRECTION_EXISTS", "Koreksi aktif untuk shift dan jenis ini sudah ada.")
		return
	}
	today := time.Now().In(zone).Format("2006-01-02")
	flow, err := scanFlow(tx.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND request_type='attendance_correction' AND is_active=TRUE AND is_default=TRUE AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) ORDER BY valid_from DESC,id LIMIT 1`, tenantID, today, today))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 409, "FLOW_NOT_FOUND", "Alur approval koreksi presensi belum tersedia.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	step, err := scanFlowStep(tx.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=1`, tenantID, flow.ID))
	if err != nil || (step.RequiredPermission != nil && *step.RequiredPermission != "absensi.approve") {
		writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap awal approval koreksi tidak valid.")
		return
	}
	employee, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, tenantID, in.EmployeeID))
	if err != nil {
		writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Karyawan tidak tersedia.")
		return
	}
	target, err := a.targetFor(r.Context(), tenantID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope.")
		return
	}
	candidates, err := a.stepApproverCandidates(r, tenantID, today, employee, step)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan approver.")
		return
	}
	eligible, err := a.eligibleApprovers(r, tenantID, zone.String(), employee, target, "absensi.approve", candidates, "attendance_correction")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa approver.")
		return
	}
	eligible = removeApprover(eligible, actorID)
	if len(eligible) == 0 && flow.FallbackRoleID != nil {
		candidates, err = a.roleApproverCandidates(r, tenantID, *flow.FallbackRoleID, today)
		if err == nil {
			eligible, err = a.eligibleApprovers(r, tenantID, zone.String(), employee, target, "absensi.approve", candidates, "attendance_correction")
			eligible = removeApprover(eligible, actorID)
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan fallback approver.")
			return
		}
	}
	if len(eligible) == 0 {
		writeAPIError(w, r, 409, "NO_APPROVER", "Belum ada approver koreksi yang berhak.")
		return
	}
	requestID, _ := ids.New()
	correctionID, _ := ids.New()
	approvalID, _ := ids.New()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO requests (tenant_id,id,request_type,subject_employee_id,requested_by,ref_table,ref_id,status,submitted_at) VALUES (?,?,'attendance_correction',?,?,'attendance_corrections',?,'in_review',UTC_TIMESTAMP(6))`, tenantID, requestID, in.EmployeeID, actorID, correctionID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat pengajuan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO attendance_corrections (tenant_id,id,employee_id,shift_instance_id,original_event_id,requested_type,requested_time,reason,status,request_id,attachment_file_id,created_by) VALUES (?,?,?,?,?,?,?,?,'in_review',?,?,?)`, tenantID, correctionID, in.EmployeeID, in.ShiftInstanceID, in.OriginalEventID, in.RequestedType, at, in.Reason, requestID, in.AttachmentFileID, actorID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan koreksi.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_instances (tenant_id,id,request_id,flow_id,current_level,status,version) VALUES (?,?,?,?,1,'in_review',1)`, tenantID, approvalID, requestID, flow.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka approval.")
		return
	}
	for _, userID := range eligible {
		taskID, _ := ids.New()
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,1,1,?,'pending')`, tenantID, taskID, approvalID, userID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas approval.")
			return
		}
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID, "request_type": "attendance_correction", "approver_user_ids": eligible})
	outboxID, _ := ids.New()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.submitted','request',?,?,?)`, tenantID, outboxID, requestID, string(payload), "approval.submitted:"+requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	if err = audit(r.Context(), tx, tenantID, actorID, "submit", "requests", requestID, nil, map[string]any{"request_type": "attendance_correction", "correction_id": correctionID}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan koreksi.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": correctionID, "request_id": requestID, "status": "in_review", "approval_version": 1, "approver_user_ids": eligible}, nil)
}

func (a adminAPI) listEssAttendanceCorrections(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	a.attendanceCorrectionPage(w, r, ` FROM attendance_corrections c WHERE c.tenant_id=? AND c.employee_id=?`, []any{c.state.SelectedTenantID, c.employeeID})
}
func (a adminAPI) listAdminAttendanceCorrections(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "absensi.lihat")
	if !ok {
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	a.attendanceCorrectionPage(w, r, correctionAdminJoin+` WHERE c.tenant_id=? AND `+filter, append([]any{c.tenant.ID}, scopeArgs...))
}
func (a adminAPI) attendanceCorrectionPage(w http.ResponseWriter, r *http.Request, from string, args []any) {
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	where := from
	if status := r.URL.Query().Get("status"); status != "" {
		switch status {
		case "in_review", "returned", "approved", "rejected", "cancelled":
			where += ` AND c.status=?`
			args = append(args, status)
		default:
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter status tidak valid.")
			return
		}
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca koreksi.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+correctionColumns+where+` ORDER BY c.created_at DESC,c.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca koreksi.")
		return
	}
	defer rows.Close()
	items := []attendanceCorrection{}
	for rows.Next() {
		item, e := scanCorrection(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca koreksi.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca koreksi.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}
func (a adminAPI) getAdminAttendanceCorrection(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "absensi.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Koreksi tidak ditemukan.")
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	item, err := scanCorrection(a.db.QueryRowContext(r.Context(), `SELECT `+correctionColumns+correctionAdminJoin+` WHERE c.tenant_id=? AND c.id=? AND `+filter, append([]any{c.tenant.ID, id}, scopeArgs...)...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Koreksi tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca koreksi.")
		return
	}
	if item.RequestID == "" {
		writeAPIData(w, 200, map[string]any{"correction": item, "approval": nil, "actions": []any{}}, nil)
		return
	}
	var approval struct {
		Status       string `json:"status"`
		CurrentLevel int    `json:"current_level"`
		Version      uint64 `json:"version"`
		AttemptNo    uint64 `json:"attempt_no"`
	}
	err = a.db.QueryRowContext(r.Context(), `SELECT status,current_level,version,attempt_no FROM approval_instances WHERE tenant_id=? AND request_id=?`, c.tenant.ID, item.RequestID).Scan(&approval.Status, &approval.CurrentLevel, &approval.Version, &approval.AttemptNo)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca approval koreksi.")
		return
	}
	type action struct {
		LevelNo     int       `json:"level_no"`
		Action      string    `json:"action"`
		Reason      *string   `json:"reason"`
		ActorUserID string    `json:"actor_user_id"`
		CreatedAt   time.Time `json:"created_at"`
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT aa.level_no,aa.action,aa.reason,aa.actor_user_id,aa.created_at FROM approval_actions aa JOIN approval_instances ai ON ai.tenant_id=aa.tenant_id AND ai.id=aa.instance_id WHERE ai.tenant_id=? AND ai.request_id=? ORDER BY aa.created_at,aa.id`, c.tenant.ID, item.RequestID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat approval.")
		return
	}
	defer rows.Close()
	actions := []action{}
	for rows.Next() {
		var entry action
		var reason sql.NullString
		if err = rows.Scan(&entry.LevelNo, &entry.Action, &reason, &entry.ActorUserID, &entry.CreatedAt); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat approval.")
			return
		}
		if reason.Valid {
			entry.Reason = &reason.String
		}
		actions = append(actions, entry)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat approval.")
		return
	}
	writeAPIData(w, 200, map[string]any{"correction": item, "approval": approval, "actions": actions}, nil)
}
