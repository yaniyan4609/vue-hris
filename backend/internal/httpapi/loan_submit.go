package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/ids"
)

func (a adminAPI) submitEssLoan(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	_, decision, err := a.access.Decide(r.Context(), c.state.SelectedTenantID, c.state.User.ID, c.zone.String(), "kasbon.buat")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak kasbon.")
		return
	}
	if !decision.Allowed {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak mengajukan kasbon tidak tersedia.")
		return
	}
	var body loanInput
	if !readJSON(w, r, &body) {
		return
	}
	amount, valid := validateLoanInput(body)
	if !valid {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Nominal atau tenor kasbon tidak valid.")
		return
	}
	a.createLoanSubmission(w, r, c.state.SelectedTenantID, c.employeeID, c.state.User.ID, c.zone, amount, body.TenorMonths)
}

func (a adminAPI) createAdminLoan(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kasbon.buat")
	if !ok {
		return
	}
	var body struct {
		EmployeeID  string      `json:"employee_id"`
		Amount      json.Number `json:"amount"`
		TenorMonths int         `json:"tenor_months"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	amount, valid := validateLoanInput(loanInput{Amount: body.Amount, TenorMonths: body.TenorMonths})
	if !valid {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Nominal atau tenor kasbon tidak valid.")
		return
	}
	if _, ok = a.adminScopedEmployee(w, r, c, body.EmployeeID); !ok {
		return
	}
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	a.createLoanSubmission(w, r, c.tenant.ID, body.EmployeeID, c.state.User.ID, zone, amount, body.TenorMonths)
}

func (a adminAPI) createLoanSubmission(w http.ResponseWriter, r *http.Request, tenantID, employeeID, actorID string, zone *time.Location, amount int64, tenor int) {
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat pengajuan kasbon.")
		return
	}
	defer tx.Rollback()
	var locked string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? AND status IN ('active','probation','leave') FOR UPDATE`, tenantID, employeeID).Scan(&locked); err != nil {
		writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Karyawan tidak aktif.")
		return
	}
	calc, err := calculateLoan(r.Context(), tx, tenantID, employeeID, amount, tenor)
	if err != nil {
		writeLoanIssue(w, r, err)
		return
	}
	today := time.Now().In(zone).Format("2006-01-02")
	flow, err := scanFlow(tx.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND request_type='loan' AND is_active=TRUE AND is_default=TRUE AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) ORDER BY valid_from DESC,id LIMIT 1 FOR UPDATE`, tenantID, today, today))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 409, "FLOW_NOT_FOUND", "Alur approval kasbon belum tersedia.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval kasbon.")
		return
	}
	step, err := scanFlowStep(tx.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=1`, tenantID, flow.ID))
	if err != nil || (step.RequiredPermission != nil && *step.RequiredPermission != "kasbon.approve") {
		writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap awal approval kasbon tidak valid.")
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
	eligible, err := a.eligibleApprovers(r, tenantID, zone.String(), employee, target, "kasbon.approve", candidates, "loan")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak approver.")
		return
	}
	eligible = removeApprover(eligible, actorID)
	if len(eligible) == 0 && flow.FallbackRoleID != nil {
		candidates, err = a.roleApproverCandidates(r, tenantID, *flow.FallbackRoleID, today)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan fallback approver.")
			return
		}
		eligible, err = a.eligibleApprovers(r, tenantID, zone.String(), employee, target, "kasbon.approve", candidates, "loan")
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak fallback.")
			return
		}
		eligible = removeApprover(eligible, actorID)
	}
	if len(eligible) == 0 {
		writeAPIError(w, r, 409, "NO_APPROVER", "Belum ada approver kasbon yang berhak.")
		return
	}
	requestID, e1 := ids.New()
	loanID, e2 := ids.New()
	instanceID, e3 := ids.New()
	if e1 != nil || e2 != nil || e3 != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID kasbon.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO requests (tenant_id,id,request_type,subject_employee_id,requested_by,ref_table,ref_id,status,submitted_at) VALUES (?,?,'loan',?,?,'employee_loans',?,'in_review',UTC_TIMESTAMP(6))`, tenantID, requestID, employeeID, actorID, loanID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pengajuan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO employee_loans (tenant_id,id,employee_id,amount,tenor_months,installment_amount,outstanding,status,request_id) VALUES (?,?,?,?,?,?,0,'in_review',?)`, tenantID, loanID, employeeID, leaveDecimal(amount), tenor, leaveDecimal(calc.Regular), requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kasbon.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_instances (tenant_id,id,request_id,flow_id,current_level,status,version) VALUES (?,?,?,?,1,'in_review',1)`, tenantID, instanceID, requestID, flow.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka approval.")
		return
	}
	for _, userID := range eligible {
		taskID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas approval.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,1,1,?,'pending')`, tenantID, taskID, instanceID, userID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas approval.")
			return
		}
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID, "request_type": "loan", "approver_user_ids": eligible})
	outboxID, e := ids.New()
	if e != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.submitted','request',?,?,?)`, tenantID, outboxID, requestID, string(payload), "approval.submitted:"+requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	if err = audit(r.Context(), tx, tenantID, actorID, "submit", "requests", requestID, nil, map[string]any{"request_type": "loan", "loan_id": loanID, "amount": leaveDecimal(amount), "tenor_months": tenor, "policy_version": calc.PolicyVersion}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat pengajuan.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pengajuan kasbon.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": loanID, "request_id": requestID, "employee_id": employeeID, "amount": leaveDecimal(amount), "tenor_months": tenor, "installment_amount": leaveDecimal(calc.Regular), "last_installment_amount": leaveDecimal(calc.Last), "outstanding": "0.00", "status": "in_review", "approval_version": 1, "approver_user_ids": eligible}, nil)
}
