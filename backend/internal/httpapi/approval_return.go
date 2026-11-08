package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) decideApprovalRequest(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "persetujuan.lihat")
	if !ok {
		return
	}
	requestID := chi.URLParam(r, "requestId")
	if len(requestID) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan.")
		return
	}
	var body struct {
		Action  string          `json:"action"`
		Reason  string          `json:"reason"`
		LevelNo int             `json:"level_no"`
		Version uint64          `json:"version"`
		Data    json.RawMessage `json:"data"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if body.Action != "return" && body.Action != "approve" && body.Action != "reject" {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Keputusan tidak valid.")
		return
	}
	if ((body.Action == "return" || body.Action == "reject") && body.Reason == "") || len([]rune(body.Reason)) > 2000 || body.LevelNo < 1 || body.Version == 0 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Alasan, tahap, atau versi tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memproses pengajuan.")
		return
	}
	defer tx.Rollback()
	var kind, requester, requestStatus string
	var subjectID, refTable, refID sql.NullString
	var attemptNo, requestVersion uint64
	err = tx.QueryRowContext(r.Context(), `SELECT request_type,subject_employee_id,requested_by,ref_table,ref_id,status,attempt_no,version FROM requests WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, requestID).Scan(&kind, &subjectID, &requester, &refTable, &refID, &requestStatus, &attemptNo, &requestVersion)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan.")
		return
	}
	if requester == c.state.User.ID {
		writeAPIError(w, r, 403, "SELF_APPROVAL_FORBIDDEN", "Pengaju tidak dapat memutuskan pengajuannya sendiri.")
		return
	}
	if requestStatus != "in_review" {
		writeAPIError(w, r, 409, "ALREADY_DECIDED", "Pengajuan tidak sedang ditinjau.")
		return
	}
	permission := approvalPermissions[kind]
	if permission == "" || !subjectID.Valid {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Pengajuan tidak memiliki jenis atau karyawan yang dapat dievaluasi.")
		return
	}
	var instanceID, flowID, instanceStatus string
	var level int
	var instanceVersion, instanceAttempt uint64
	err = tx.QueryRowContext(r.Context(), `SELECT id,flow_id,current_level,status,version,attempt_no FROM approval_instances WHERE tenant_id=? AND request_id=? FOR UPDATE`, c.tenant.ID, requestID).Scan(&instanceID, &flowID, &level, &instanceStatus, &instanceVersion, &instanceAttempt)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Pengajuan belum memiliki proses persetujuan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca proses persetujuan.")
		return
	}
	if instanceVersion != body.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Persetujuan sudah berubah.")
		return
	}
	if instanceStatus != "in_review" || level != body.LevelNo || instanceAttempt != attemptNo {
		writeAPIError(w, r, 409, "ALREADY_DECIDED", "Tahap persetujuan sudah berubah.")
		return
	}
	var taskID, taskStatus string
	err = tx.QueryRowContext(r.Context(), `SELECT id,status FROM approval_tasks WHERE tenant_id=? AND instance_id=? AND attempt_no=? AND level_no=? AND assignee_user_id=? FOR UPDATE`, c.tenant.ID, instanceID, attemptNo, level, c.state.User.ID).Scan(&taskID, &taskStatus)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 403, "NOT_ASSIGNED", "Tugas persetujuan tidak ditugaskan kepada Anda.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tugas persetujuan.")
		return
	}
	if taskStatus != "pending" {
		writeAPIError(w, r, 409, "ALREADY_DECIDED", "Tugas persetujuan sudah diputuskan.")
		return
	}
	employee, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.tenant.ID, subjectID.String))
	if err != nil {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Karyawan pengajuan tidak tersedia.")
		return
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope karyawan.")
		return
	}
	subject, decision, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, permission)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak persetujuan.")
		return
	}
	if !decision.Allowed {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak persetujuan tidak tersedia.")
		return
	}
	if !decision.Includes(subject, target) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan dalam scope.")
		return
	}
	domainReady := refTable.Valid && refID.Valid && ((kind == "overtime" && refTable.String == "overtime_requests") || (kind == "leave" && refTable.String == "leave_requests"))
	if body.Action == "return" && refTable.Valid && !domainReady {
		writeAPIError(w, r, 409, "ACTION_NOT_READY", "Pengembalian jenis pengajuan ini menunggu handler domain.")
		return
	}
	if body.Action != "return" {
		if !domainReady {
			writeAPIError(w, r, 409, "ACTION_NOT_READY", "Keputusan akhir jenis pengajuan ini menunggu handler domain.")
			return
		}
		a.decideOvertimeApproval(w, r, tx, c, overtimeDecision{
			Action: body.Action, Reason: body.Reason, Data: body.Data, RequestID: requestID,
			Kind:  kind,
			RefID: refID.String, InstanceID: instanceID, FlowID: flowID, TaskID: taskID,
			Requester: requester, Employee: employee, Target: target, Level: level,
			AttemptNo: attemptNo, RequestVersion: requestVersion, InstanceVersion: instanceVersion,
		})
		return
	}
	if kind == "overtime" && refTable.Valid && refTable.String == "overtime_requests" && refID.Valid {
		result, err := tx.ExecContext(r.Context(), `UPDATE overtime_requests SET status='returned' WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status='in_review'`, c.tenant.ID, refID.String, requestID, subjectID.String)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengembalikan data lembur.")
			return
		}
		rows, _ := result.RowsAffected()
		if rows != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status data lembur tidak sesuai pengajuan.")
			return
		}
	}
	if kind == "leave" && domainReady {
		result, err := tx.ExecContext(r.Context(), `UPDATE leave_requests SET status='returned' WHERE tenant_id=? AND id=? AND request_id=? AND employee_id=? AND status='in_review'`, c.tenant.ID, refID.String, requestID, subjectID.String)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengembalikan data cuti.")
			return
		}
		rows, _ := result.RowsAffected()
		if rows != 1 {
			writeAPIError(w, r, 409, "DOMAIN_CONFLICT", "Status data cuti tidak sesuai pengajuan.")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_tasks SET status='returned',decided_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=? AND status='pending'`, c.tenant.ID, taskID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup tugas persetujuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_tasks SET status='cancelled',decided_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND instance_id=? AND attempt_no=? AND status='pending'`, c.tenant.ID, instanceID, attemptNo); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup tugas lain.")
		return
	}
	actionID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat riwayat persetujuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO approval_actions (tenant_id,id,instance_id,task_id,level_no,actor_user_id,action,reason) VALUES (?,?,?,?,?,?,'return',?)`, c.tenant.ID, actionID, instanceID, taskID, level, c.state.User.ID, body.Reason); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan riwayat persetujuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_instances SET status='returned',version=version+1,finished_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, c.tenant.ID, instanceID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui proses persetujuan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE requests SET status='returned',version=version+1,final_at=NULL WHERE tenant_id=? AND id=?`, c.tenant.ID, requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui pengajuan.")
		return
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID, "requester_user_id": requester, "attempt_no": attemptNo, "action_id": actionID})
	outboxID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.returned','request',?,?,?)`, c.tenant.ID, outboxID, requestID, string(payload), "approval.returned:"+requestID+":"+actionID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	before := map[string]any{"status": requestStatus, "attempt_no": attemptNo, "version": requestVersion}
	after := map[string]any{"status": "returned", "attempt_no": attemptNo, "version": requestVersion + 1, "reason": body.Reason}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "return", "requests", requestID, before, after); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengembalikan pengajuan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"request_id": requestID, "status": "returned", "attempt_no": attemptNo, "version": requestVersion + 1, "approval_version": instanceVersion + 1, "action_id": actionID}, nil)
}
