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

func (a adminAPI) adminLeaveEmployee(w http.ResponseWriter, r *http.Request, c adminContext, id string) (employee, bool) {
	if len(id) != 26 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID karyawan tidak valid.")
		return employee{}, false
	}
	item, err := scanEmployee(a.db.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return employee{}, false
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
		return employee{}, false
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, item)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca scope karyawan.")
		return employee{}, false
	}
	if !c.effective.Includes(c.subject, target) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return employee{}, false
	}
	if item.Status != "active" && item.Status != "probation" && item.Status != "leave" {
		writeAPIError(w, r, 409, "EMPLOYEE_INACTIVE", "Karyawan tidak aktif.")
		return employee{}, false
	}
	return item, true
}

func (a adminAPI) createAdminLeave(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.buat")
	if !ok {
		return
	}
	var body struct {
		EmployeeID       string  `json:"employee_id"`
		LeaveTypeID      string  `json:"leave_type_id"`
		StartDate        string  `json:"start_date"`
		EndDate          string  `json:"end_date"`
		Reason           string  `json:"reason"`
		AttachmentFileID *string `json:"attachment_file_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	in := leaveInput{LeaveTypeID: body.LeaveTypeID, StartDate: body.StartDate, EndDate: body.EndDate, Reason: body.Reason, AttachmentFileID: body.AttachmentFileID}
	if _, _, err := validateLeaveInput(&in); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if _, ok = a.adminLeaveEmployee(w, r, c, body.EmployeeID); !ok {
		return
	}
	a.createLeaveSubmission(w, r, c.tenant.ID, body.EmployeeID, c.state.User.ID, c.tenant.Timezone, in)
}

func (a adminAPI) updateAdminLeave(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan cuti tidak ditemukan.")
		return
	}
	var body struct {
		Version          uint64          `json:"version"`
		Note             string          `json:"note"`
		LeaveTypeID      *string         `json:"leave_type_id"`
		StartDate        *string         `json:"start_date"`
		EndDate          *string         `json:"end_date"`
		Reason           *string         `json:"reason"`
		AttachmentFileID json.RawMessage `json:"attachment_file_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Note = strings.TrimSpace(body.Note)
	if len([]rune(body.Note)) > 2000 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Catatan revisi terlalu panjang.")
		return
	}
	var employeeID, requestStatus string
	var requestID sql.NullString
	err := a.db.QueryRowContext(r.Context(), `SELECT employee_id,request_id,status FROM leave_requests WHERE tenant_id=? AND id=?`, c.tenant.ID, id).Scan(&employeeID, &requestID, &requestStatus)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan cuti tidak ditemukan.")
		return
	}
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if _, ok = a.adminLeaveEmployee(w, r, c, employeeID); !ok {
		return
	}
	if requestStatus != "draft" && requestStatus != "returned" {
		writeAPIError(w, r, 409, "REQUEST_NOT_EDITABLE", "Hanya draft atau pengajuan yang dikembalikan dapat diubah.")
		return
	}
	if requestStatus == "returned" && (body.Version == 0 || body.Note == "") {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi dan catatan revisi wajib untuk pengajuan returned.")
		return
	}
	if requestStatus == "draft" && requestID.Valid && body.Version == 0 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi pengajuan draft wajib diisi.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	defer tx.Rollback()
	var requester string
	var attemptNo, requestVersion uint64
	if requestID.Valid {
		var kind, refTable, refID, status string
		err = tx.QueryRowContext(r.Context(), `SELECT request_type,requested_by,ref_table,ref_id,status,attempt_no,version FROM requests WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, requestID.String).Scan(&kind, &requester, &refTable, &refID, &status, &attemptNo, &requestVersion)
		if err != nil || kind != "leave" || refTable != "leave_requests" || refID != id || status != requestStatus {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Pengajuan yang dikembalikan tidak sesuai data cuti.")
			return
		}
		if requestVersion != body.Version {
			writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pengajuan sudah berubah.")
			return
		}
		if requestStatus == "returned" && attemptNo >= 100 {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Batas kirim ulang tercapai.")
			return
		}
	}
	if requestStatus == "returned" && !requestID.Valid {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Referensi pengajuan tidak tersedia.")
		return
	}
	var instanceID, flowID string
	var instanceVersion, instanceAttempt uint64
	if requestStatus == "returned" {
		var instanceStatus string
		err = tx.QueryRowContext(r.Context(), `SELECT id,flow_id,status,attempt_no,version FROM approval_instances WHERE tenant_id=? AND request_id=? FOR UPDATE`, c.tenant.ID, requestID.String).Scan(&instanceID, &flowID, &instanceStatus, &instanceAttempt, &instanceVersion)
		if err != nil || instanceStatus != "returned" || instanceAttempt != attemptNo {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Proses approval tidak sesuai pengajuan.")
			return
		}
	}
	var oldType, domainStatus string
	var oldStart, oldEnd time.Time
	var oldReason, oldAttachment sql.NullString
	err = tx.QueryRowContext(r.Context(), `SELECT leave_type_id,start_date,end_date,reason,attachment_file_id,status FROM leave_requests WHERE tenant_id=? AND id=? AND employee_id=? FOR UPDATE`, c.tenant.ID, id, employeeID).Scan(&oldType, &oldStart, &oldEnd, &oldReason, &oldAttachment, &domainStatus)
	if err != nil || domainStatus != requestStatus {
		writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Data cuti sudah berubah.")
		return
	}
	var locked string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, employeeID).Scan(&locked); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	in := leaveInput{LeaveTypeID: oldType, StartDate: oldStart.Format("2006-01-02"), EndDate: oldEnd.Format("2006-01-02"), Reason: oldReason.String}
	if oldAttachment.Valid {
		value := oldAttachment.String
		in.AttachmentFileID = &value
	}
	if body.LeaveTypeID != nil {
		in.LeaveTypeID = *body.LeaveTypeID
	}
	if body.StartDate != nil {
		in.StartDate = *body.StartDate
	}
	if body.EndDate != nil {
		in.EndDate = *body.EndDate
	}
	if body.Reason != nil {
		in.Reason = *body.Reason
	}
	if len(body.AttachmentFileID) > 0 {
		if string(body.AttachmentFileID) == "null" {
			in.AttachmentFileID = nil
		} else {
			var value string
			if err = json.Unmarshal(body.AttachmentFileID, &value); err != nil {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Lampiran tidak valid.")
				return
			}
			in.AttachmentFileID = &value
		}
	}
	prepared, err := a.prepareLeave(r, tx, c.tenant.ID, employeeID, c.tenant.Timezone, in)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if requestStatus == "returned" && prepared.Flow.ID != flowID {
		writeAPIError(w, r, 409, "FLOW_CHANGED", "Alur approval cuti telah berubah.")
		return
	}
	if requestStatus == "returned" {
		prepared.Approvers = removeApprover(prepared.Approvers, requester)
		if len(prepared.Approvers) == 0 {
			prepared.Approvers, err = a.fallbackLeaveApprovers(r, c.tenant.ID, employeeID, c.tenant.Timezone, prepared.Flow, requester)
			if err != nil {
				writeLeaveIssue(w, r, err)
				return
			}
		}
		if len(prepared.Approvers) == 0 {
			writeAPIError(w, r, 409, "NO_APPROVER", "Tidak ada approver lain yang berhak.")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM leave_request_allocations WHERE tenant_id=? AND leave_request_id=?`, c.tenant.ID, id); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	for _, allocation := range prepared.Allocations {
		allocationID, e := ids.New()
		if e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO leave_request_allocations (tenant_id,id,leave_request_id,period_year,first_work_date,days) VALUES (?,?,?,?,?,?)`, c.tenant.ID, allocationID, id, allocation.Year, allocation.FirstWorkDate, allocation.Days); e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
	}
	nextStatus := requestStatus
	if requestStatus == "returned" {
		nextStatus = "in_review"
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE leave_requests SET leave_type_id=?,start_date=?,end_date=?,days=?,reason=?,attachment_file_id=?,status=? WHERE tenant_id=? AND id=? AND employee_id=? AND status=?`, in.LeaveTypeID, in.StartDate, in.EndDate, prepared.Days, in.Reason, in.AttachmentFileID, nextStatus, c.tenant.ID, id, employeeID, requestStatus)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 && requestStatus != "draft" {
		writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Data cuti sudah berubah.")
		return
	}
	if requestStatus == "returned" {
		nextAttempt := attemptNo + 1
		revisionID, e := ids.New()
		if e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
		correction, _ := json.Marshal(map[string]any{"note": body.Note, "leave_type_id": in.LeaveTypeID, "start_date": in.StartDate, "end_date": in.EndDate, "reason": in.Reason, "attachment_file_id": in.AttachmentFileID})
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO request_revisions (tenant_id,id,request_id,attempt_no,correction_json,submitted_by) VALUES (?,?,?,?,?,?)`, c.tenant.ID, revisionID, requestID.String, nextAttempt, string(correction), c.state.User.ID); e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
		for _, userID := range prepared.Approvers {
			taskID, e := ids.New()
			if e != nil {
				writeLeaveIssue(w, r, e)
				return
			}
			if _, e = tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,?,1,?,'pending')`, c.tenant.ID, taskID, instanceID, nextAttempt, userID); e != nil {
				writeLeaveIssue(w, r, e)
				return
			}
		}
		if _, e = tx.ExecContext(r.Context(), `UPDATE approval_instances SET status='in_review',attempt_no=?,current_level=1,version=version+1,started_at=UTC_TIMESTAMP(6),finished_at=NULL WHERE tenant_id=? AND id=?`, nextAttempt, c.tenant.ID, instanceID); e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
		if _, e = tx.ExecContext(r.Context(), `UPDATE requests SET status='in_review',attempt_no=?,version=version+1,submitted_at=UTC_TIMESTAMP(6),final_at=NULL WHERE tenant_id=? AND id=?`, nextAttempt, c.tenant.ID, requestID.String); e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
		outboxID, e := ids.New()
		if e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
		payload, _ := json.Marshal(map[string]any{"request_id": requestID.String, "attempt_no": nextAttempt, "approver_user_ids": prepared.Approvers, "revision_id": revisionID})
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.resubmitted','request',?,?,?)`, c.tenant.ID, outboxID, requestID.String, string(payload), "approval.resubmitted:"+requestID.String+":"+revisionID); e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
	}
	if requestStatus == "draft" && requestID.Valid {
		if _, err = tx.ExecContext(r.Context(), `UPDATE requests SET version=version+1 WHERE tenant_id=? AND id=? AND status='draft'`, c.tenant.ID, requestID.String); err != nil {
			writeLeaveIssue(w, r, err)
			return
		}
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "leave_requests", id, map[string]any{"status": requestStatus, "leave_type_id": oldType, "start_date": oldStart.Format("2006-01-02"), "end_date": oldEnd.Format("2006-01-02")}, map[string]any{"status": nextStatus, "leave_type_id": in.LeaveTypeID, "start_date": in.StartDate, "end_date": in.EndDate, "days": prepared.Days}); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = tx.Commit(); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	var responseRequestID, version, approvalVersion any
	if requestID.Valid {
		responseRequestID = requestID.String
		version = requestVersion + 1
	}
	if requestStatus == "returned" {
		approvalVersion = instanceVersion + 1
	}
	writeAPIData(w, 200, map[string]any{"id": id, "request_id": responseRequestID, "status": nextStatus, "days": prepared.Days, "allocations": prepared.Allocations, "version": version, "approval_version": approvalVersion}, nil)
}
