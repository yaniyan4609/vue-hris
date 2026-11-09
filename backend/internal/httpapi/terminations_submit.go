package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) submitTermination(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "terminasi.buat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Terminasi tidak ditemukan.")
		return
	}
	scope, args := employeeChangeScope(c)
	var employeeID string
	err := a.db.QueryRowContext(r.Context(), `SELECT x.employee_id`+terminationJoin+` WHERE x.tenant_id=? AND x.id=? AND `+scope, append([]any{c.tenant.ID, id}, args...)...).Scan(&employeeID)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Terminasi tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca terminasi.")
		return
	}
	a.submitTerminationRecord(w, r, c.tenant.ID, c.tenant.Timezone, c.state.User.ID, employeeID, id)
}

// Both Admin and ESS use this transaction; the caller must establish employee scope.
func (a adminAPI) submitTerminationRecord(w http.ResponseWriter, r *http.Request, tenantID, zone, actorID, employeeID, id string) {
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengajukan terminasi.")
		return
	}
	defer tx.Rollback()
	var employeeStatus, assignmentID string
	var deleted sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT e.status,e.deleted_at,a.id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE e.tenant_id=? AND e.id=? FOR UPDATE`, tenantID, employeeID).Scan(&employeeStatus, &deleted, &assignmentID)
	if err != nil || deleted.Valid || employeeStatus == "resigned" || employeeStatus == "terminated" {
		writeAPIError(w, r, 409, "EMPLOYEE_INACTIVE", "Karyawan tidak aktif.")
		return
	}
	var status, sourceID, creator string
	var requestID sql.NullString
	var effectiveDate time.Time
	var version uint64
	err = tx.QueryRowContext(r.Context(), `SELECT status,source_assignment_id,created_by,request_id,effective_date,version FROM terminations WHERE tenant_id=? AND id=? AND employee_id=? FOR UPDATE`, tenantID, id, employeeID).Scan(&status, &sourceID, &creator, &requestID, &effectiveDate, &version)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Terminasi tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca terminasi.")
		return
	}
	if creator != actorID || status != "draft" && status != "returned" {
		writeAPIError(w, r, 409, "TERMINATION_NOT_SUBMITTABLE", "Hanya pembuat dapat mengajukan draft atau revisi.")
		return
	}
	if sourceID != assignmentID || effectiveDate.Format("2006-01-02") < tenantToday(zone) {
		writeAPIError(w, r, 409, "ASSIGNMENT_CHANGED", "Penugasan atau tanggal efektif sudah berubah.")
		return
	}
	employee, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, tenantID, employeeID))
	if err != nil {
		writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Profil karyawan tidak tersedia.")
		return
	}
	target, err := a.targetFor(r.Context(), tenantID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca scope.")
		return
	}
	today := tenantToday(zone)
	flowQuery := `SELECT ` + flowColumns + ` FROM approval_flows WHERE tenant_id=? AND request_type='termination' AND is_active=TRUE AND is_default=TRUE AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) ORDER BY valid_from DESC,id LIMIT 1`
	flowArgs := []any{tenantID, today, today}
	if requestID.Valid {
		flowQuery = `SELECT ` + flowColumns + ` FROM approval_flows WHERE tenant_id=? AND id=(SELECT flow_id FROM approval_instances WHERE tenant_id=? AND request_id=?) AND request_type='termination' AND is_active=TRUE AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?)`
		flowArgs = []any{tenantID, tenantID, requestID.String, today, today}
	}
	flow, err := scanFlow(tx.QueryRowContext(r.Context(), flowQuery, flowArgs...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 409, "FLOW_NOT_FOUND", "Alur approval terminasi belum tersedia.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	step, err := scanFlowStep(tx.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=1`, tenantID, flow.ID))
	if err != nil || step.RequiredPermission != nil && *step.RequiredPermission != "terminasi.approve" {
		writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap approval terminasi tidak valid.")
		return
	}
	candidates, err := a.stepApproverCandidates(r, tenantID, today, employee, step)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencari approver.")
		return
	}
	approvers, err := a.eligibleApprovers(r, tenantID, zone, employee, target, "terminasi.approve", candidates, "termination")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa approver.")
		return
	}
	approvers = removeApprover(approvers, actorID)
	if len(approvers) == 0 && flow.FallbackRoleID != nil {
		candidates, err = a.roleApproverCandidates(r, tenantID, *flow.FallbackRoleID, today)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencari fallback.")
			return
		}
		approvers, err = a.eligibleApprovers(r, tenantID, zone, employee, target, "terminasi.approve", candidates, "termination")
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa fallback.")
			return
		}
		approvers = removeApprover(approvers, actorID)
	}
	if len(approvers) == 0 {
		writeAPIError(w, r, 409, "NO_APPROVER", "Belum ada approver lain yang berhak.")
		return
	}
	var instanceID string
	var attempt uint64 = 1
	var approvalVersion uint64 = 1
	if !requestID.Valid {
		requestID.String, err = ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat pengajuan.")
			return
		}
		requestID.Valid = true
		instanceID, err = ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat approval.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO requests (tenant_id,id,request_type,subject_employee_id,requested_by,ref_table,ref_id,status,submitted_at) VALUES (?,?,'termination',?,?,'terminations',?,'in_review',UTC_TIMESTAMP(6))`, tenantID, requestID.String, employeeID, actorID, id); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat pengajuan.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_instances (tenant_id,id,request_id,flow_id,current_level,status,version) VALUES (?,?,?,?,1,'in_review',1)`, tenantID, instanceID, requestID.String, flow.ID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memulai approval.")
			return
		}
	} else {
		var requestStatus, requester, instanceStatus, oldFlow string
		var oldAttempt, oldVersion uint64
		err = tx.QueryRowContext(r.Context(), `SELECT requested_by,status,attempt_no FROM requests WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, requestID.String).Scan(&requester, &requestStatus, &oldAttempt)
		if err != nil || requester != actorID || requestStatus != "returned" {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Pengajuan tidak dapat dikirim ulang.")
			return
		}
		err = tx.QueryRowContext(r.Context(), `SELECT id,flow_id,status,version FROM approval_instances WHERE tenant_id=? AND request_id=? FOR UPDATE`, tenantID, requestID.String).Scan(&instanceID, &oldFlow, &instanceStatus, &oldVersion)
		if err != nil || oldFlow != flow.ID || instanceStatus != "returned" {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Approval tidak dapat dibuka ulang.")
			return
		}
		attempt = oldAttempt + 1
		approvalVersion = oldVersion + 1
		revisionID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat revisi.")
			return
		}
		correction, _ := json.Marshal(map[string]any{"effective_date": effectiveDate.Format("2006-01-02")})
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO request_revisions (tenant_id,id,request_id,attempt_no,correction_json,submitted_by) VALUES (?,?,?,?,?,?)`, tenantID, revisionID, requestID.String, attempt, string(correction), actorID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan revisi.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE approval_instances SET status='in_review',attempt_no=?,current_level=1,version=version+1,started_at=UTC_TIMESTAMP(6),finished_at=NULL WHERE tenant_id=? AND id=?`, attempt, tenantID, instanceID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka approval.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE requests SET status='in_review',attempt_no=?,version=version+1,submitted_at=UTC_TIMESTAMP(6),final_at=NULL WHERE tenant_id=? AND id=?`, attempt, tenantID, requestID.String); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka pengajuan.")
			return
		}
	}
	for _, userID := range approvers {
		taskID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,?,1,?,'pending')`, tenantID, taskID, instanceID, attempt, userID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan tugas.")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE terminations SET status='in_review',request_id=?,version=version+1,apply_error=NULL WHERE tenant_id=? AND id=? AND status=?`, requestID.String, tenantID, id, status); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui terminasi.")
		return
	}
	outboxID, e := ids.New()
	if e != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	event := "approval.submitted"
	if status == "returned" {
		event = "approval.resubmitted"
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID.String, "request_type": "termination", "approver_user_ids": approvers, "attempt_no": attempt})
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,?,'request',?,?,?)`, tenantID, outboxID, event, requestID.String, string(payload), event+":"+requestID.String+":"+outboxID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan notifikasi.")
		return
	}
	if err = audit(r.Context(), tx, tenantID, actorID, "submit", "terminations", id, map[string]any{"status": status, "version": version}, map[string]any{"status": "in_review", "version": version + 1, "request_id": requestID.String}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengajukan terminasi.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "request_id": requestID.String, "status": "in_review", "version": version + 1, "approval_version": approvalVersion, "approver_user_ids": approvers}, nil)
}
