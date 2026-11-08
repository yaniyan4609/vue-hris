package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func validOvertimeInput(body *overtimeInput) bool {
	body.Reason = strings.TrimSpace(body.Reason)
	date, err := time.Parse("2006-01-02", body.WorkDate)
	return err == nil && date.Format("2006-01-02") == body.WorkDate && body.ProposedMinutes >= 1 && body.ProposedMinutes <= 1440 && body.Reason != "" && len([]rune(body.Reason)) <= 2000 && (body.ShiftInstanceID == nil || len(*body.ShiftInstanceID) == 26)
}

func (a adminAPI) createAdminOvertime(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "lembur.buat")
	if !ok {
		return
	}
	var body struct {
		EmployeeID      string  `json:"employee_id"`
		WorkDate        string  `json:"work_date"`
		ProposedMinutes int     `json:"proposed_minutes"`
		Reason          string  `json:"reason"`
		ShiftInstanceID *string `json:"shift_instance_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	in := overtimeInput{WorkDate: body.WorkDate, ProposedMinutes: body.ProposedMinutes, Reason: body.Reason, ShiftInstanceID: body.ShiftInstanceID}
	if !validOvertimeInput(&in) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal, durasi, alasan, atau shift lembur tidak valid.")
		return
	}
	if _, ok = a.adminScopedEmployee(w, r, c, body.EmployeeID); !ok {
		return
	}
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	a.createOvertimeSubmission(w, r, c.tenant.ID, body.EmployeeID, c.state.User.ID, zone, in)
}

func (a adminAPI) updateAdminOvertime(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "lembur.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan lembur tidak ditemukan.")
		return
	}
	var body struct {
		Version         uint64          `json:"version"`
		Note            string          `json:"note"`
		WorkDate        *string         `json:"work_date"`
		ProposedMinutes *int            `json:"proposed_minutes"`
		Reason          *string         `json:"reason"`
		ShiftInstanceID json.RawMessage `json:"shift_instance_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Note = strings.TrimSpace(body.Note)
	if len([]rune(body.Note)) > 2000 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Catatan revisi terlalu panjang.")
		return
	}
	var employeeID, status string
	var requestID sql.NullString
	err := a.db.QueryRowContext(r.Context(), `SELECT employee_id,request_id,status FROM overtime_requests WHERE tenant_id=? AND id=?`, c.tenant.ID, id).Scan(&employeeID, &requestID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan lembur tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan lembur.")
		return
	}
	if _, ok = a.adminScopedEmployee(w, r, c, employeeID); !ok {
		return
	}
	if status != "draft" && status != "returned" {
		writeAPIError(w, r, 409, "REQUEST_NOT_EDITABLE", "Hanya draft atau lembur yang dikembalikan dapat diubah.")
		return
	}
	if (status == "returned" || requestID.Valid) && (body.Version == 0) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi pengajuan wajib diisi.")
		return
	}
	if status == "returned" && body.Note == "" {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Catatan revisi wajib untuk lembur returned.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah pengajuan lembur.")
		return
	}
	defer tx.Rollback()
	var requester string
	var attemptNo, requestVersion uint64
	if requestID.Valid {
		var kind, refTable, refID, requestStatus string
		err = tx.QueryRowContext(r.Context(), `SELECT request_type,requested_by,ref_table,ref_id,status,attempt_no,version FROM requests WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, requestID.String).Scan(&kind, &requester, &refTable, &refID, &requestStatus, &attemptNo, &requestVersion)
		if err != nil || kind != "overtime" || refTable != "overtime_requests" || refID != id || requestStatus != status {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Pengajuan tidak sesuai data lembur.")
			return
		}
		if requestVersion != body.Version {
			writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pengajuan sudah berubah.")
			return
		}
		if status == "returned" && attemptNo >= 100 {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Batas kirim ulang tercapai.")
			return
		}
	}
	if status == "returned" && !requestID.Valid {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Referensi pengajuan tidak tersedia.")
		return
	}
	var instanceID, flowID string
	var instanceVersion, instanceAttempt uint64
	if status == "returned" {
		var instanceStatus string
		err = tx.QueryRowContext(r.Context(), `SELECT id,flow_id,status,attempt_no,version FROM approval_instances WHERE tenant_id=? AND request_id=? FOR UPDATE`, c.tenant.ID, requestID.String).Scan(&instanceID, &flowID, &instanceStatus, &instanceAttempt, &instanceVersion)
		if err != nil || instanceStatus != "returned" || instanceAttempt != attemptNo {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Proses approval tidak sesuai lembur.")
			return
		}
	}
	var oldDate time.Time
	var oldReason, oldShift sql.NullString
	var oldMinutes int
	var domainStatus string
	err = tx.QueryRowContext(r.Context(), `SELECT work_date,proposed_minutes,reason,shift_instance_id,status FROM overtime_requests WHERE tenant_id=? AND id=? AND employee_id=? FOR UPDATE`, c.tenant.ID, id, employeeID).Scan(&oldDate, &oldMinutes, &oldReason, &oldShift, &domainStatus)
	if err != nil || domainStatus != status {
		writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Data lembur sudah berubah.")
		return
	}
	var locked string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, employeeID).Scan(&locked); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return
	}
	in := overtimeInput{WorkDate: oldDate.Format("2006-01-02"), ProposedMinutes: oldMinutes, Reason: oldReason.String}
	if oldShift.Valid {
		value := oldShift.String
		in.ShiftInstanceID = &value
	}
	if body.WorkDate != nil {
		in.WorkDate = *body.WorkDate
	}
	if body.ProposedMinutes != nil {
		in.ProposedMinutes = *body.ProposedMinutes
	}
	if body.Reason != nil {
		in.Reason = *body.Reason
	}
	if len(body.ShiftInstanceID) > 0 {
		if string(body.ShiftInstanceID) == "null" {
			in.ShiftInstanceID = nil
		} else {
			var value string
			if err = json.Unmarshal(body.ShiftInstanceID, &value); err != nil {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID shift tidak valid.")
				return
			}
			in.ShiftInstanceID = &value
		}
	}
	if !validOvertimeInput(&in) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data revisi lembur tidak valid.")
		return
	}
	if in.ShiftInstanceID != nil {
		var count int
		if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM shift_instances WHERE tenant_id=? AND id=? AND employee_id=? AND work_date=?`, c.tenant.ID, *in.ShiftInstanceID, employeeID, in.WorkDate).Scan(&count); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa shift.")
			return
		}
		if count != 1 {
			writeAPIError(w, r, 404, "SHIFT_NOT_FOUND", "Shift tidak sesuai tanggal lembur.")
			return
		}
	}
	var duplicate int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM overtime_requests WHERE tenant_id=? AND employee_id=? AND work_date=? AND id<>? AND status IN ('submitted','in_review','approved')`, c.tenant.ID, employeeID, in.WorkDate, id).Scan(&duplicate); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa bentrok lembur.")
		return
	}
	if duplicate > 0 {
		writeAPIError(w, r, 409, "OVERTIME_EXISTS", "Sudah ada pengajuan lembur aktif pada tanggal itu.")
		return
	}
	var approvers []string
	if status == "returned" {
		step, e := scanFlowStep(tx.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=1`, c.tenant.ID, flowID))
		if e != nil || (step.RequiredPermission != nil && *step.RequiredPermission != "lembur.approve") {
			writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap awal approval lembur tidak valid.")
			return
		}
		employee, e := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.tenant.ID, employeeID))
		if e != nil {
			writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Penugasan karyawan tidak tersedia.")
			return
		}
		target, e := a.targetFor(r.Context(), c.tenant.ID, employee)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca scope karyawan.")
			return
		}
		zone, e := time.LoadLocation(c.tenant.Timezone)
		if e != nil {
			zone = time.UTC
		}
		today := time.Now().In(zone).Format("2006-01-02")
		candidates, e := a.stepApproverCandidates(r, c.tenant.ID, today, employee, step)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan approver.")
			return
		}
		approvers, e = a.eligibleApprovers(r, c.tenant.ID, c.tenant.Timezone, employee, target, "lembur.approve", candidates)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa approver.")
			return
		}
		approvers = removeApprover(approvers, requester)
		if len(approvers) == 0 {
			var fallback sql.NullString
			if e = tx.QueryRowContext(r.Context(), `SELECT fallback_role_id FROM approval_flows WHERE tenant_id=? AND id=?`, c.tenant.ID, flowID).Scan(&fallback); e != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa fallback.")
				return
			}
			if fallback.Valid {
				candidates, e = a.roleApproverCandidates(r, c.tenant.ID, fallback.String, today)
				if e != nil {
					writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca fallback.")
					return
				}
				approvers, e = a.eligibleApprovers(r, c.tenant.ID, c.tenant.Timezone, employee, target, "lembur.approve", candidates)
				if e != nil {
					writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa fallback.")
					return
				}
				approvers = removeApprover(approvers, requester)
			}
		}
		if len(approvers) == 0 {
			writeAPIError(w, r, 409, "NO_APPROVER", "Belum ada approver lain yang berhak.")
			return
		}
	}
	nextStatus := status
	if status == "returned" {
		nextStatus = "in_review"
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE overtime_requests SET work_date=?,proposed_minutes=?,reason=?,shift_instance_id=?,status=? WHERE tenant_id=? AND id=? AND employee_id=? AND status=?`, in.WorkDate, in.ProposedMinutes, in.Reason, in.ShiftInstanceID, nextStatus, c.tenant.ID, id, employeeID, status)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan revisi lembur.")
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 && status != "draft" {
		writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Data lembur sudah berubah.")
		return
	}
	if status == "returned" {
		nextAttempt := attemptNo + 1
		revisionID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat revisi.")
			return
		}
		correction, _ := json.Marshal(map[string]any{"note": body.Note, "work_date": in.WorkDate, "proposed_minutes": in.ProposedMinutes, "reason": in.Reason, "shift_instance_id": in.ShiftInstanceID})
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO request_revisions (tenant_id,id,request_id,attempt_no,correction_json,submitted_by) VALUES (?,?,?,?,?,?)`, c.tenant.ID, revisionID, requestID.String, nextAttempt, string(correction), c.state.User.ID); e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan revisi.")
			return
		}
		for _, userID := range approvers {
			taskID, e := ids.New()
			if e != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas.")
				return
			}
			if _, e = tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,?,1,?,'pending')`, c.tenant.ID, taskID, instanceID, nextAttempt, userID); e != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas.")
				return
			}
		}
		if _, e = tx.ExecContext(r.Context(), `UPDATE approval_instances SET status='in_review',attempt_no=?,current_level=1,version=version+1,started_at=UTC_TIMESTAMP(6),finished_at=NULL WHERE tenant_id=? AND id=?`, nextAttempt, c.tenant.ID, instanceID); e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka approval.")
			return
		}
		if _, e = tx.ExecContext(r.Context(), `UPDATE requests SET status='in_review',attempt_no=?,version=version+1,submitted_at=UTC_TIMESTAMP(6),final_at=NULL WHERE tenant_id=? AND id=?`, nextAttempt, c.tenant.ID, requestID.String); e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka pengajuan.")
			return
		}
		outboxID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
			return
		}
		payload, _ := json.Marshal(map[string]any{"request_id": requestID.String, "attempt_no": nextAttempt, "approver_user_ids": approvers, "revision_id": revisionID})
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.resubmitted','request',?,?,?)`, c.tenant.ID, outboxID, requestID.String, string(payload), "approval.resubmitted:"+requestID.String+":"+revisionID); e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
			return
		}
	}
	if status == "draft" && requestID.Valid {
		if _, err = tx.ExecContext(r.Context(), `UPDATE requests SET version=version+1 WHERE tenant_id=? AND id=? AND status='draft'`, c.tenant.ID, requestID.String); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui versi pengajuan.")
			return
		}
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "overtime_requests", id, map[string]any{"status": status, "work_date": oldDate.Format("2006-01-02"), "proposed_minutes": oldMinutes}, map[string]any{"status": nextStatus, "work_date": in.WorkDate, "proposed_minutes": in.ProposedMinutes}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan revisi lembur.")
		return
	}
	var responseRequestID, version, approvalVersion any
	if requestID.Valid {
		responseRequestID = requestID.String
		version = requestVersion + 1
	}
	if status == "returned" {
		approvalVersion = instanceVersion + 1
	}
	writeAPIData(w, 200, map[string]any{"id": id, "request_id": responseRequestID, "status": nextStatus, "work_date": in.WorkDate, "proposed_minutes": in.ProposedMinutes, "reason": in.Reason, "version": version, "approval_version": approvalVersion, "approver_user_ids": approvers}, nil)
}
