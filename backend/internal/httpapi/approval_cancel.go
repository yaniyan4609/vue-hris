package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) cancelEssRequest(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	requestID := chi.URLParam(r, "requestId")
	if len(requestID) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan.")
		return
	}
	tenantID := c.state.SelectedTenantID
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan pengajuan.")
		return
	}
	defer tx.Rollback()
	var kind, requester, status string
	var subjectID, refTable, refID sql.NullString
	var attemptNo, version uint64
	err = tx.QueryRowContext(r.Context(), `SELECT request_type,subject_employee_id,requested_by,ref_table,ref_id,status,attempt_no,version FROM requests WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, requestID).Scan(&kind, &subjectID, &requester, &refTable, &refID, &status, &attemptNo, &version)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (requester != c.state.User.ID || !subjectID.Valid || subjectID.String != c.employeeID)) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan.")
		return
	}
	if status != "in_review" && status != "submitted" && status != "returned" {
		writeAPIError(w, r, 409, "REQUEST_NOT_CANCELLABLE", "Pengajuan tidak dapat dibatalkan pada status ini.")
		return
	}
	domainReady := refTable.Valid && refID.Valid && ((kind == "overtime" && refTable.String == "overtime_requests") || (kind == "leave" && refTable.String == "leave_requests") || (kind == "reimbursement" && refTable.String == "reimbursements") || (kind == "loan" && refTable.String == "employee_loans"))
	if refTable.Valid && !domainReady {
		writeAPIError(w, r, 409, "ACTION_NOT_READY", "Pembatalan jenis pengajuan ini menunggu handler domain.")
		return
	}
	var instanceID, instanceStatus string
	var instanceAttempt, instanceVersion uint64
	err = tx.QueryRowContext(r.Context(), `SELECT id,status,attempt_no,version FROM approval_instances WHERE tenant_id=? AND request_id=? FOR UPDATE`, tenantID, requestID).Scan(&instanceID, &instanceStatus, &instanceAttempt, &instanceVersion)
	if err != nil || instanceAttempt != attemptNo || (instanceStatus != "in_review" && instanceStatus != "returned") {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Proses persetujuan tidak sesuai status pengajuan.")
		return
	}
	if kind == "overtime" {
		if !refID.Valid {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Referensi lembur tidak tersedia.")
			return
		}
		result, err := tx.ExecContext(r.Context(), `UPDATE overtime_requests SET status='cancelled' WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status=?`, tenantID, refID.String, requestID, c.employeeID, status)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan lembur.")
			return
		}
		rows, _ := result.RowsAffected()
		if rows != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status lembur tidak sesuai pengajuan.")
			return
		}
	}
	if kind == "leave" && domainReady {
		result, err := tx.ExecContext(r.Context(), `UPDATE leave_requests SET status='cancelled' WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status=?`, tenantID, refID.String, requestID, c.employeeID, status)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan cuti.")
			return
		}
		rows, _ := result.RowsAffected()
		if rows != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status cuti tidak sesuai pengajuan.")
			return
		}
	}
	if kind == "reimbursement" && domainReady {
		result, err := tx.ExecContext(r.Context(), `UPDATE reimbursements SET status='cancelled' WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status=?`, tenantID, refID.String, requestID, c.employeeID, status)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan klaim.")
			return
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status klaim tidak sesuai pengajuan.")
			return
		}
	}
	if kind == "loan" && domainReady {
		result, err := tx.ExecContext(r.Context(), `UPDATE employee_loans SET status='cancelled' WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status=?`, tenantID, refID.String, requestID, c.employeeID, status)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan kasbon.")
			return
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status kasbon tidak sesuai pengajuan.")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_tasks SET status='cancelled',decided_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND instance_id=? AND attempt_no=? AND status='pending'`, tenantID, instanceID, attemptNo); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup tugas persetujuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_instances SET status='cancelled',version=version+1,finished_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, tenantID, instanceID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup proses persetujuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE requests SET status='cancelled',version=version+1,final_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, tenantID, requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan pengajuan.")
		return
	}
	outboxID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID, "request_type": kind, "attempt_no": attemptNo, "requester_user_id": requester})
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.cancelled','request',?,?,?)`, tenantID, outboxID, requestID, string(payload), "approval.cancelled:"+requestID+":"+outboxID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	if err := audit(r.Context(), tx, tenantID, c.state.User.ID, "cancel", "requests", requestID, map[string]any{"status": status, "version": version}, map[string]any{"status": "cancelled", "version": version + 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan pengajuan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"request_id": requestID, "status": "cancelled", "attempt_no": attemptNo, "version": version + 1, "approval_version": instanceVersion + 1}, nil)
}
