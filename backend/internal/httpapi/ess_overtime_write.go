package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
)

type overtimeInput struct {
	WorkDate        string  `json:"work_date"`
	ProposedMinutes int     `json:"proposed_minutes"`
	Reason          string  `json:"reason"`
	ShiftInstanceID *string `json:"shift_instance_id"`
}

func (a adminAPI) submitEssOvertime(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	_, permission, err := a.access.Decide(r.Context(), c.state.SelectedTenantID, c.state.User.ID, c.zone.String(), "lembur.buat")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak pengajuan lembur.")
		return
	}
	if !permission.Allowed {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak mengajukan lembur tidak tersedia.")
		return
	}
	var body overtimeInput
	if !readJSON(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	date, err := time.Parse("2006-01-02", body.WorkDate)
	if err != nil || date.Format("2006-01-02") != body.WorkDate || body.ProposedMinutes < 1 || body.ProposedMinutes > 1440 || body.Reason == "" || len([]rune(body.Reason)) > 2000 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal, durasi, atau alasan lembur tidak valid.")
		return
	}
	if body.ShiftInstanceID != nil && len(*body.ShiftInstanceID) != 26 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Shift instance tidak valid.")
		return
	}
	a.createOvertimeSubmission(w, r, c.state.SelectedTenantID, c.employeeID, c.state.User.ID, c.zone, body)
}

func (a adminAPI) createOvertimeSubmission(w http.ResponseWriter, r *http.Request, tenantID, employeeID, actorID string, zone *time.Location, body overtimeInput) {
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat pengajuan lembur.")
		return
	}
	defer tx.Rollback()
	var locked string
	if err := tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, employeeID).Scan(&locked); err != nil {
		writeAPIError(w, r, 404, "EMPLOYEE_NOT_FOUND", "Karyawan tidak ditemukan.")
		return
	}
	if body.ShiftInstanceID != nil {
		var count int
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM shift_instances WHERE tenant_id=? AND id=? AND employee_id=? AND work_date=?`, tenantID, *body.ShiftInstanceID, employeeID, body.WorkDate).Scan(&count); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa shift.")
			return
		}
		if count != 1 {
			writeAPIError(w, r, 404, "SHIFT_NOT_FOUND", "Shift tidak ditemukan pada tanggal tersebut.")
			return
		}
	}
	var duplicate int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM overtime_requests WHERE tenant_id=? AND employee_id=? AND work_date=? AND status IN ('submitted','in_review','approved')`, tenantID, employeeID, body.WorkDate).Scan(&duplicate); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa pengajuan lembur.")
		return
	}
	if duplicate > 0 {
		writeAPIError(w, r, 409, "OVERTIME_EXISTS", "Sudah ada pengajuan lembur aktif untuk tanggal tersebut.")
		return
	}
	today := time.Now().In(zone).Format("2006-01-02")
	flow, err := scanFlow(tx.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND request_type='overtime' AND is_active=TRUE AND is_default=TRUE AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) ORDER BY valid_from DESC,id LIMIT 1 FOR UPDATE`, tenantID, today, today))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 409, "FLOW_NOT_FOUND", "Alur approval lembur belum tersedia.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	step, err := scanFlowStep(tx.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=1`, tenantID, flow.ID))
	if err != nil || (step.RequiredPermission != nil && *step.RequiredPermission != "lembur.approve") {
		writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap awal approval lembur tidak valid.")
		return
	}
	employee, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, tenantID, employeeID))
	if err != nil {
		writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Penugasan karyawan belum tersedia.")
		return
	}
	target, err := a.targetFor(r.Context(), tenantID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope karyawan.")
		return
	}
	candidates, err := a.stepApproverCandidates(r, tenantID, today, employee, step)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan approver.")
		return
	}
	eligible, err := a.eligibleApprovers(r, tenantID, zone.String(), employee, target, "lembur.approve", candidates)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak approver.")
		return
	}
	if len(eligible) == 0 && flow.FallbackRoleID != nil {
		candidates, err = a.roleApproverCandidates(r, tenantID, *flow.FallbackRoleID, today)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan fallback approver.")
			return
		}
		eligible, err = a.eligibleApprovers(r, tenantID, zone.String(), employee, target, "lembur.approve", candidates)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak fallback.")
			return
		}
	}
	eligible = removeApprover(eligible, actorID)
	if len(eligible) == 0 && flow.FallbackRoleID != nil {
		candidates, err = a.roleApproverCandidates(r, tenantID, *flow.FallbackRoleID, today)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan fallback approver.")
			return
		}
		eligible, err = a.eligibleApprovers(r, tenantID, zone.String(), employee, target, "lembur.approve", candidates)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak fallback.")
			return
		}
		eligible = removeApprover(eligible, actorID)
	}
	if len(eligible) == 0 {
		writeAPIError(w, r, 409, "NO_APPROVER", "Belum ada approver lembur yang berhak.")
		return
	}
	requestID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID pengajuan.")
		return
	}
	overtimeID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID lembur.")
		return
	}
	instanceID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat proses approval.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO requests (tenant_id,id,request_type,subject_employee_id,requested_by,ref_table,ref_id,status,submitted_at) VALUES (?,?,'overtime',?,?,'overtime_requests',?,'in_review',UTC_TIMESTAMP(6))`, tenantID, requestID, employeeID, actorID, overtimeID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat pengajuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO overtime_requests (tenant_id,id,employee_id,work_date,shift_instance_id,proposed_minutes,reason,status,request_id) VALUES (?,?,?,?,?,?,?,'in_review',?)`, tenantID, overtimeID, employeeID, body.WorkDate, body.ShiftInstanceID, body.ProposedMinutes, body.Reason, requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan lembur.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO approval_instances (tenant_id,id,request_id,flow_id,current_level,status,version) VALUES (?,?,?,?,1,'in_review',1)`, tenantID, instanceID, requestID, flow.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka approval.")
		return
	}
	for _, userID := range eligible {
		taskID, err := ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas approval.")
			return
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,1,1,?,'pending')`, tenantID, taskID, instanceID, userID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas approval.")
			return
		}
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID, "request_type": "overtime", "approver_user_ids": eligible})
	outboxID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.submitted','request',?,?,?)`, tenantID, outboxID, requestID, string(payload), "approval.submitted:"+requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	if err := audit(r.Context(), tx, tenantID, actorID, "submit", "requests", requestID, nil, map[string]any{"request_type": "overtime", "overtime_id": overtimeID, "work_date": body.WorkDate, "proposed_minutes": body.ProposedMinutes}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat pengajuan lembur.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": overtimeID, "request_id": requestID, "employee_id": employeeID, "work_date": body.WorkDate, "proposed_minutes": body.ProposedMinutes, "approved_minutes": nil, "paid_minutes": nil, "reason": body.Reason, "status": "in_review", "approval_version": 1, "approver_user_ids": eligible}, nil)
}
