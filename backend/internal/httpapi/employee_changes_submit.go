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

func (a adminAPI) submitEmployeeChange(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "mutasi.buat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Perubahan tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengajukan perubahan.")
		return
	}
	defer tx.Rollback()
	scope, args := employeeChangeScope(c)
	params := append([]any{c.tenant.ID, id}, args...)
	var employeeID, kind, status, sourceID, officeID, positionID string
	var managerID, requestID sql.NullString
	var effectiveDate time.Time
	var version uint64
	err = tx.QueryRowContext(r.Context(), `SELECT x.employee_id,x.change_type,x.status,x.effective_date,x.source_assignment_id,x.new_office_id,x.new_position_id,x.new_reports_to_id,x.request_id,x.version FROM employee_changes x JOIN employees e ON e.tenant_id=x.tenant_id AND e.id=x.employee_id LEFT JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE x.tenant_id=? AND x.id=? AND `+scope+` FOR UPDATE`, params...).Scan(&employeeID, &kind, &status, &effectiveDate, &sourceID, &officeID, &positionID, &managerID, &requestID, &version)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Perubahan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca perubahan.")
		return
	}
	if status != "draft" && status != "returned" {
		writeAPIError(w, r, 409, "CHANGE_NOT_SUBMITTABLE", "Perubahan sudah diajukan atau selesai.")
		return
	}
	if effectiveDate.Format("2006-01-02") < tenantToday(c.tenant.Timezone) {
		writeAPIError(w, r, 409, "DATE_CONFLICT", "Tanggal efektif sudah berlalu.")
		return
	}
	var employeeStatus, currentAssignment string
	var deleted sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT e.status,e.deleted_at,a.id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE e.tenant_id=? AND e.id=? FOR UPDATE`, c.tenant.ID, employeeID).Scan(&employeeStatus, &deleted, &currentAssignment)
	if err != nil || deleted.Valid || employeeStatus == "resigned" || employeeStatus == "terminated" || currentAssignment != sourceID {
		writeAPIError(w, r, 409, "ASSIGNMENT_CHANGED", "Penugasan karyawan sudah berubah atau tidak aktif.")
		return
	}
	if err = lockEmployeeOrganization(r.Context(), tx, c.tenant.ID, officeID, positionID); err != nil {
		writeAPIError(w, r, 409, "ORGANIZATION_UNAVAILABLE", "Tujuan organisasi tidak aktif.")
		return
	}
	var conflicts int
	if err = tx.QueryRowContext(r.Context(), `SELECT (SELECT COUNT(*) FROM work_schedules WHERE tenant_id=? AND employee_id=? AND status='published' AND work_date>=?)+(SELECT COUNT(*) FROM payroll_runs pr JOIN payroll_periods pp ON pp.tenant_id=pr.tenant_id AND pp.id=pr.period_id WHERE pr.tenant_id=? AND pr.status='finalized' AND pp.end_date>=?)`, c.tenant.ID, employeeID, effectiveDate, c.tenant.ID, effectiveDate).Scan(&conflicts); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa roster dan payroll.")
		return
	}
	if conflicts != 0 {
		writeAPIError(w, r, 409, "HISTORY_LOCKED", "Roster terbit atau payroll final memakai tanggal efektif ini.")
		return
	}
	currentEmployee, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.tenant.ID, employeeID))
	if err != nil {
		writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Profil karyawan tidak tersedia.")
		return
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, currentEmployee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope karyawan.")
		return
	}
	if !c.effective.Includes(c.subject, target) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan di luar scope.")
		return
	}
	dest := employee{ID: employeeID, BaseOfficeID: officeID, PositionID: positionID, ReportsToID: managerID.String, UserID: currentEmployee.UserID}
	destTarget, err := a.targetFor(r.Context(), c.tenant.ID, dest)
	if err != nil || !c.effective.Includes(c.subject, destTarget) {
		writeAPIError(w, r, 403, "SCOPE_DENIED", "Penempatan tujuan di luar scope.")
		return
	}
	flowType := changeRequestType(kind)
	today := tenantToday(c.tenant.Timezone)
	flowQuery := `SELECT ` + flowColumns + ` FROM approval_flows WHERE tenant_id=? AND request_type=? AND is_active=TRUE AND is_default=TRUE AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) ORDER BY valid_from DESC,id LIMIT 1`
	flowArgs := []any{c.tenant.ID, flowType, today, today}
	if requestID.Valid {
		flowQuery = `SELECT ` + flowColumns + ` FROM approval_flows WHERE tenant_id=? AND id=(SELECT flow_id FROM approval_instances WHERE tenant_id=? AND request_id=?) AND request_type=? AND is_active=TRUE AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?)`
		flowArgs = []any{c.tenant.ID, c.tenant.ID, requestID.String, flowType, today, today}
	}
	flow, err := scanFlow(tx.QueryRowContext(r.Context(), flowQuery, flowArgs...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 409, "FLOW_NOT_FOUND", "Alur approval perubahan belum tersedia.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	if requestID.Valid {
		var oldFlow string
		var instanceStatus string
		if err = tx.QueryRowContext(r.Context(), `SELECT flow_id,status FROM approval_instances WHERE tenant_id=? AND request_id=? FOR UPDATE`, c.tenant.ID, requestID.String).Scan(&oldFlow, &instanceStatus); err != nil || instanceStatus != "returned" {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Pengajuan tidak dalam status dikembalikan.")
			return
		}
		if flow.ID != oldFlow {
			writeAPIError(w, r, 409, "FLOW_INVALID", "Alur approval pengajuan sudah berubah.")
			return
		}
	}
	step, err := scanFlowStep(tx.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=1`, c.tenant.ID, flow.ID))
	if err != nil || step.RequiredPermission != nil && *step.RequiredPermission != "mutasi.approve" {
		writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap awal approval perubahan tidak valid.")
		return
	}
	candidates, err := a.stepApproverCandidates(r, c.tenant.ID, today, currentEmployee, step)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan approver.")
		return
	}
	approvers, err := a.eligibleApprovers(r, c.tenant.ID, c.tenant.Timezone, currentEmployee, target, "mutasi.approve", candidates, flowType)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa approver.")
		return
	}
	approvers = removeApprover(approvers, c.state.User.ID)
	if len(approvers) == 0 && flow.FallbackRoleID != nil {
		candidates, err = a.roleApproverCandidates(r, c.tenant.ID, *flow.FallbackRoleID, today)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencari fallback approver.")
			return
		}
		approvers, err = a.eligibleApprovers(r, c.tenant.ID, c.tenant.Timezone, currentEmployee, target, "mutasi.approve", candidates, flowType)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa fallback approver.")
			return
		}
		approvers = removeApprover(approvers, c.state.User.ID)
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
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat proses approval.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO requests (tenant_id,id,request_type,subject_employee_id,requested_by,ref_table,ref_id,status,submitted_at) VALUES (?,?,?,?,?,'employee_changes',?,'in_review',UTC_TIMESTAMP(6))`, c.tenant.ID, requestID.String, flowType, employeeID, c.state.User.ID, id); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat pengajuan.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_instances (tenant_id,id,request_id,flow_id,current_level,status,version) VALUES (?,?,?,?,1,'in_review',1)`, c.tenant.ID, instanceID, requestID.String, flow.ID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memulai approval.")
			return
		}
	} else {
		var requester string
		var requestAttempt uint64
		var requestStatus string
		if err = tx.QueryRowContext(r.Context(), `SELECT requested_by,status,attempt_no FROM requests WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, requestID.String).Scan(&requester, &requestStatus, &requestAttempt); err != nil || requester != c.state.User.ID || requestStatus != "returned" {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Hanya pembuat pengajuan yang dapat mengirim ulang.")
			return
		}
		attempt = requestAttempt + 1
		var oldVersion uint64
		if err = tx.QueryRowContext(r.Context(), `SELECT id,version FROM approval_instances WHERE tenant_id=? AND request_id=? FOR UPDATE`, c.tenant.ID, requestID.String).Scan(&instanceID, &oldVersion); err != nil {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Proses approval tidak tersedia.")
			return
		}
		approvalVersion = oldVersion + 1
		revisionID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat revisi.")
			return
		}
		correction, _ := json.Marshal(map[string]any{"effective_date": effectiveDate.Format("2006-01-02"), "office_id": officeID, "position_id": positionID, "reports_to_employee_id": managerID.String})
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO request_revisions (tenant_id,id,request_id,attempt_no,correction_json,submitted_by) VALUES (?,?,?,?,?,?)`, c.tenant.ID, revisionID, requestID.String, attempt, string(correction), c.state.User.ID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat revisi.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE approval_instances SET status='in_review',attempt_no=?,current_level=1,version=version+1,started_at=UTC_TIMESTAMP(6),finished_at=NULL WHERE tenant_id=? AND id=?`, attempt, c.tenant.ID, instanceID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka approval.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE requests SET status='in_review',attempt_no=?,version=version+1,submitted_at=UTC_TIMESTAMP(6),final_at=NULL WHERE tenant_id=? AND id=?`, attempt, c.tenant.ID, requestID.String); err != nil {
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
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,?,1,?,'pending')`, c.tenant.ID, taskID, instanceID, attempt, userID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas approval.")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE employee_changes SET status='in_review',request_id=?,version=version+1,apply_error=NULL WHERE tenant_id=? AND id=? AND status=?`, requestID.String, c.tenant.ID, id, status); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui perubahan.")
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
	payload, _ := json.Marshal(map[string]any{"request_id": requestID.String, "request_type": flowType, "approver_user_ids": approvers, "attempt_no": attempt})
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,?,'request',?,?,?)`, c.tenant.ID, outboxID, event, requestID.String, string(payload), event+":"+requestID.String+":"+outboxID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "submit", "employee_changes", id, map[string]any{"status": status, "version": version}, map[string]any{"status": "in_review", "version": version + 1, "request_id": requestID.String, "attempt_no": attempt}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengajukan perubahan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "request_id": requestID.String, "status": "in_review", "version": version + 1, "approval_version": approvalVersion, "approver_user_ids": approvers}, nil)
}
