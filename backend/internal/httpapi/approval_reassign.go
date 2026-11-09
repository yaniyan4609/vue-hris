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

func (a adminAPI) reassignApproval(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "persetujuan.delegasikan")
	if !ok {
		return
	}
	requestID := chi.URLParam(r, "requestId")
	if !fileIDPattern.MatchString(requestID) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan dalam scope.")
		return
	}
	var body struct {
		TaskID   string `json:"task_id"`
		ToUserID string `json:"to_user_id"`
		Version  uint64 `json:"version"`
		Reason   string `json:"reason"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if !fileIDPattern.MatchString(body.TaskID) || !fileIDPattern.MatchString(body.ToUserID) || body.Version == 0 || len([]rune(body.Reason)) < 10 || len([]rune(body.Reason)) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tugas, penerima, versi, atau alasan tidak valid.")
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum mengalihkan tugas.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengalihkan tugas.")
		return
	}
	defer tx.Rollback()
	var kind, requester, requestStatus string
	var employeeID sql.NullString
	var requestVersion uint64
	err = tx.QueryRowContext(r.Context(), `SELECT request_type,requested_by,subject_employee_id,status,version FROM requests WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, requestID).Scan(&kind, &requester, &employeeID, &requestStatus, &requestVersion)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan.")
		return
	}
	if requester == c.state.User.ID {
		writeAPIError(w, r, 403, "SELF_APPROVAL_FORBIDDEN", "Pengaju tidak dapat mengalihkan tugas persetujuannya sendiri.")
		return
	}
	if requestStatus != "in_review" || !employeeID.Valid || approvalPermissions[kind] == "" {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Pengajuan tidak sedang ditinjau.")
		return
	}
	var instanceID, instanceStatus string
	var level, attempt int
	var version uint64
	err = tx.QueryRowContext(r.Context(), `SELECT id,status,current_level,attempt_no,version FROM approval_instances WHERE tenant_id=? AND request_id=? FOR UPDATE`, c.tenant.ID, requestID).Scan(&instanceID, &instanceStatus, &level, &attempt, &version)
	if err != nil {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Proses persetujuan tidak tersedia.")
		return
	}
	if instanceStatus != "in_review" || version != body.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Tahap atau versi persetujuan sudah berubah.")
		return
	}
	var fromUser, taskStatus string
	var taskAttempt, taskLevel int
	err = tx.QueryRowContext(r.Context(), `SELECT assignee_user_id,status,attempt_no,level_no FROM approval_tasks WHERE tenant_id=? AND id=? AND instance_id=? FOR UPDATE`, c.tenant.ID, body.TaskID, instanceID).Scan(&fromUser, &taskStatus, &taskAttempt, &taskLevel)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Tugas tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tugas.")
		return
	}
	if taskStatus != "pending" || taskAttempt != attempt || taskLevel != level {
		writeAPIError(w, r, 409, "ALREADY_DECIDED", "Tugas tidak lagi menunggu keputusan.")
		return
	}
	if fromUser == body.ToUserID || requester == body.ToUserID {
		writeAPIError(w, r, 409, "TARGET_INVALID", "Penerima sudah bertugas atau merupakan pengaju.")
		return
	}
	employee, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.tenant.ID, employeeID.String))
	if err != nil {
		writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Profil karyawan tidak tersedia.")
		return
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope karyawan.")
		return
	}
	if !c.effective.Includes(c.subject, target) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan di luar scope delegasi.")
		return
	}
	permission := approvalPermissions[kind]
	actorSubject, actorRight, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, permission)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kewenangan delegasi.")
		return
	}
	if !actorRight.Allowed || !actorRight.Includes(actorSubject, target) {
		writeAPIError(w, r, 403, "DELEGATION_EXCEEDS_RIGHTS", "Pengalih tidak memiliki hak approve dalam scope ini.")
		return
	}
	eligible, err := a.eligibleApproversDirect(r, c.tenant.ID, c.tenant.Timezone, employee, target, permission, []string{body.ToUserID})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa penerima tugas.")
		return
	}
	if len(eligible) != 1 {
		writeAPIError(w, r, 403, "TARGET_NOT_ELIGIBLE", "Penerima tidak memiliki hak approve dalam scope ini.")
		return
	}
	var existing int
	err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM approval_tasks WHERE tenant_id=? AND instance_id=? AND attempt_no=? AND level_no=? AND assignee_user_id=?`, c.tenant.ID, instanceID, attempt, level, body.ToUserID).Scan(&existing)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa tugas penerima.")
		return
	}
	if existing != 0 {
		writeAPIError(w, r, 409, "TARGET_ALREADY_ASSIGNED", "Penerima pernah ditugaskan pada tahap ini.")
		return
	}
	newTaskID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID tugas.")
		return
	}
	reassignmentID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID pengalihan.")
		return
	}
	outboxID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE approval_tasks SET status='cancelled',decided_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=? AND status='pending'`, c.tenant.ID, body.TaskID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup tugas lama.")
		return
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		writeAPIError(w, r, 409, "ALREADY_DECIDED", "Tugas sudah berubah.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,?,?,?,'pending')`, c.tenant.ID, newTaskID, instanceID, attempt, level, body.ToUserID); err != nil {
		writeAPIError(w, r, 409, "TARGET_ALREADY_ASSIGNED", "Penerima tidak dapat ditugaskan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_reassignments (tenant_id,id,instance_id,from_task_id,to_task_id,actor_user_id,reason) VALUES (?,?,?,?,?,?,?)`, c.tenant.ID, reassignmentID, instanceID, body.TaskID, newTaskID, c.state.User.ID, body.Reason); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat pengalihan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE approval_instances SET version=version+1 WHERE tenant_id=? AND id=?`, c.tenant.ID, instanceID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui versi persetujuan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE requests SET version=version+1 WHERE tenant_id=? AND id=?`, c.tenant.ID, requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui pengajuan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE notifications SET obsolete_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND recipient_user_id=? AND event_type LIKE 'approval.%' AND obsolete_at IS NULL AND (JSON_UNQUOTE(JSON_EXTRACT(payload_json,'$.request_id'))=? OR LOCATE(?,dedupe_key)>0)`, c.tenant.ID, fromUser, requestID, requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan notifikasi tugas lama.")
		return
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID, "reassignment_id": reassignmentID, "from_user_id": fromUser, "to_user_id": body.ToUserID, "requester_user_id": requester, "attempt_no": attempt})
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.reassigned','request',?,?,?)`, c.tenant.ID, outboxID, requestID, string(payload), "approval.reassigned:"+requestID+":"+reassignmentID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "reassign", "requests", requestID, map[string]any{"task_id": body.TaskID, "assignee_user_id": fromUser, "approval_version": version}, map[string]any{"task_id": newTaskID, "assignee_user_id": body.ToUserID, "approval_version": version + 1, "reason": body.Reason}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit pengalihan.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengalihkan tugas.")
		return
	}
	writeAPIData(w, 200, map[string]any{"request_id": requestID, "reassignment_id": reassignmentID, "from_task_id": body.TaskID, "to_task_id": newTaskID, "approval_version": version + 1, "version": requestVersion + 1}, nil)
}
