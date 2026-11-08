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

func (a adminAPI) resubmitEssRequest(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	requestID := chi.URLParam(r, "requestId")
	if len(requestID) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan.")
		return
	}
	var body struct {
		Version    uint64          `json:"version"`
		Correction json.RawMessage `json:"correction"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	var correction map[string]any
	if body.Version == 0 || len(body.Correction) > 8192 || json.Unmarshal(body.Correction, &correction) != nil || len(correction) < 1 || len(correction) > 3 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi dan catatan perbaikan pengajuan wajib diisi.")
		return
	}
	note, noteOK := correction["note"].(string)
	note = strings.TrimSpace(note)
	if !noteOK || note == "" || len([]rune(note)) > 2000 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Catatan perbaikan tidak valid.")
		return
	}
	normalized := map[string]any{"note": note}
	for key, value := range correction {
		switch key {
		case "note":
		case "proposed_minutes":
			minutes, ok := value.(float64)
			if !ok || minutes < 1 || minutes > 1440 || minutes != float64(int(minutes)) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Menit lembur perbaikan tidak valid.")
				return
			}
			normalized[key] = int(minutes)
		case "reason":
			reason, ok := value.(string)
			reason = strings.TrimSpace(reason)
			if !ok || reason == "" || len([]rune(reason)) > 2000 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Alasan lembur perbaikan tidak valid.")
				return
			}
			normalized[key] = reason
		default:
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Field perbaikan tidak dikenal.")
			return
		}
	}
	correctionJSON, _ := json.Marshal(normalized)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengirim ulang pengajuan.")
		return
	}
	defer tx.Rollback()
	var kind, requester, status string
	var subjectID, refTable, refID sql.NullString
	var attemptNo, version uint64
	err = tx.QueryRowContext(r.Context(), `SELECT request_type,subject_employee_id,requested_by,ref_table,ref_id,status,attempt_no,version FROM requests WHERE tenant_id=? AND id=? FOR UPDATE`, c.state.SelectedTenantID, requestID).Scan(&kind, &subjectID, &requester, &refTable, &refID, &status, &attemptNo, &version)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (requester != c.state.User.ID || !subjectID.Valid || subjectID.String != c.employeeID)) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan.")
		return
	}
	if status != "returned" {
		writeAPIError(w, r, 409, "REQUEST_NOT_RETURNED", "Hanya pengajuan yang dikembalikan dapat dikirim ulang.")
		return
	}
	if version != body.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pengajuan sudah berubah.")
		return
	}
	if approvalPermissions[kind] == "" || attemptNo >= 100 {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Jenis atau jumlah kirim ulang pengajuan tidak valid.")
		return
	}
	if kind != "overtime" && len(normalized) > 1 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Perbaikan field domain belum tersedia untuk jenis pengajuan ini.")
		return
	}
	if kind != "overtime" && refTable.Valid {
		writeAPIError(w, r, 409, "ACTION_NOT_READY", "Kirim ulang jenis pengajuan ini menunggu handler domain.")
		return
	}
	if kind == "overtime" && (!refTable.Valid || refTable.String != "overtime_requests" || !refID.Valid) {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Referensi data lembur tidak valid.")
		return
	}
	var instanceID, flowID, instanceStatus string
	var instanceAttempt, instanceVersion uint64
	err = tx.QueryRowContext(r.Context(), `SELECT id,flow_id,status,attempt_no,version FROM approval_instances WHERE tenant_id=? AND request_id=? FOR UPDATE`, c.state.SelectedTenantID, requestID).Scan(&instanceID, &flowID, &instanceStatus, &instanceAttempt, &instanceVersion)
	if err != nil || instanceStatus != "returned" || instanceAttempt != attemptNo {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Riwayat proses persetujuan tidak sesuai.")
		return
	}
	var fallback sql.NullString
	if err := tx.QueryRowContext(r.Context(), `SELECT fallback_role_id FROM approval_flows WHERE tenant_id=? AND id=?`, c.state.SelectedTenantID, flowID).Scan(&fallback); err != nil {
		writeAPIError(w, r, 409, "FLOW_NOT_FOUND", "Alur persetujuan awal tidak tersedia.")
		return
	}
	step, err := scanFlowStep(tx.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=1`, c.state.SelectedTenantID, flowID))
	if err != nil || (step.RequiredPermission != nil && *step.RequiredPermission != approvalPermissions[kind]) {
		writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap awal alur persetujuan tidak valid.")
		return
	}
	employee, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.state.SelectedTenantID, subjectID.String))
	if err != nil {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Profil karyawan tidak tersedia.")
		return
	}
	target, err := a.targetFor(r.Context(), c.state.SelectedTenantID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca scope karyawan.")
		return
	}
	today := time.Now().In(c.zone).Format("2006-01-02")
	candidates, err := a.stepApproverCandidates(r, c.state.SelectedTenantID, today, employee, step)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan approver.")
		return
	}
	eligible, err := a.eligibleApprovers(r, c.state.SelectedTenantID, c.zone.String(), employee, target, approvalPermissions[kind], candidates)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak approver.")
		return
	}
	if len(eligible) == 0 && fallback.Valid {
		candidates, err = a.roleApproverCandidates(r, c.state.SelectedTenantID, fallback.String, today)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan fallback approver.")
			return
		}
		eligible, err = a.eligibleApprovers(r, c.state.SelectedTenantID, c.zone.String(), employee, target, approvalPermissions[kind], candidates)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak fallback.")
			return
		}
	}
	if len(eligible) == 0 {
		writeAPIError(w, r, 409, "NO_APPROVER", "Belum ada approver yang berhak untuk tahap awal.")
		return
	}
	if kind == "overtime" {
		var proposed int
		var overtimeReason sql.NullString
		var overtimeStatus string
		if err := tx.QueryRowContext(r.Context(), `SELECT proposed_minutes,reason,status FROM overtime_requests WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? FOR UPDATE`, c.state.SelectedTenantID, refID.String, requestID, subjectID.String).Scan(&proposed, &overtimeReason, &overtimeStatus); err != nil || overtimeStatus != "returned" {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status data lembur tidak sesuai pengajuan.")
			return
		}
		if value, ok := normalized["proposed_minutes"]; ok {
			proposed = value.(int)
		}
		reason := overtimeReason.String
		if value, ok := normalized["reason"]; ok {
			reason = value.(string)
		}
		result, err := tx.ExecContext(r.Context(), `UPDATE overtime_requests SET proposed_minutes=?,reason=?,status='in_review' WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status='returned'`, proposed, reason, c.state.SelectedTenantID, refID.String, requestID, subjectID.String)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka ulang data lembur.")
			return
		}
		rows, _ := result.RowsAffected()
		if rows != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status data lembur tidak sesuai pengajuan.")
			return
		}
	}
	nextAttempt := attemptNo + 1
	revisionID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat revisi pengajuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO request_revisions (tenant_id,id,request_id,attempt_no,correction_json,submitted_by) VALUES (?,?,?,?,?,?)`, c.state.SelectedTenantID, revisionID, requestID, nextAttempt, string(correctionJSON), c.state.User.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan revisi pengajuan.")
		return
	}
	for _, userID := range eligible {
		taskID, err := ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas persetujuan.")
			return
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,?,1,?,'pending')`, c.state.SelectedTenantID, taskID, instanceID, nextAttempt, userID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas persetujuan.")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_instances SET status='in_review',attempt_no=?,current_level=1,version=version+1,started_at=UTC_TIMESTAMP(6),finished_at=NULL WHERE tenant_id=? AND id=?`, nextAttempt, c.state.SelectedTenantID, instanceID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka ulang proses persetujuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE requests SET status='in_review',attempt_no=?,version=version+1,submitted_at=UTC_TIMESTAMP(6),final_at=NULL WHERE tenant_id=? AND id=?`, nextAttempt, c.state.SelectedTenantID, requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka ulang pengajuan.")
		return
	}
	outboxID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID, "attempt_no": nextAttempt, "approver_user_ids": eligible, "revision_id": revisionID})
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.resubmitted','request',?,?,?)`, c.state.SelectedTenantID, outboxID, requestID, string(payload), "approval.resubmitted:"+requestID+":"+revisionID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	before := map[string]any{"status": "returned", "attempt_no": attemptNo, "version": version}
	after := map[string]any{"status": "in_review", "attempt_no": nextAttempt, "version": version + 1, "revision_id": revisionID}
	if err := audit(r.Context(), tx, c.state.SelectedTenantID, c.state.User.ID, "resubmit", "requests", requestID, before, after); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengirim ulang pengajuan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"request_id": requestID, "status": "in_review", "attempt_no": nextAttempt, "version": version + 1, "approval_version": instanceVersion + 1, "revision_id": revisionID, "approver_user_ids": eligible}, nil)
}
