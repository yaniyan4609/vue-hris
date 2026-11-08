package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
)

type leaveInput struct {
	LeaveTypeID      string  `json:"leave_type_id"`
	StartDate        string  `json:"start_date"`
	EndDate          string  `json:"end_date"`
	Reason           string  `json:"reason"`
	AttachmentFileID *string `json:"attachment_file_id"`
}

type leaveAllocation struct {
	Year          int    `json:"year"`
	FirstWorkDate string `json:"first_work_date"`
	Days          int    `json:"days"`
	BalanceBefore string `json:"balance_before"`
	Reserved      string `json:"reserved_days"`
	BalanceAfter  string `json:"balance_after"`
}

type leavePrepared struct {
	Type        essLeaveType
	Days        int
	Allocations []leaveAllocation
	Flow        approvalFlow
	Approvers   []string
}

type leaveQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type leaveIssue struct {
	Status        int
	Code, Message string
}

func (e leaveIssue) Error() string             { return e.Message }
func leaveConflict(code, message string) error { return leaveIssue{409, code, message} }

func validateLeaveInput(in *leaveInput) (time.Time, time.Time, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	start, e1 := time.Parse("2006-01-02", in.StartDate)
	end, e2 := time.Parse("2006-01-02", in.EndDate)
	if len(in.LeaveTypeID) != 26 || e1 != nil || e2 != nil || start.Format("2006-01-02") != in.StartDate || end.Format("2006-01-02") != in.EndDate || end.Before(start) || end.Sub(start) >= 366*24*time.Hour || len([]rune(in.Reason)) > 2000 {
		return time.Time{}, time.Time{}, leaveIssue{400, "VALIDATION_FAILED", "Jenis, rentang tanggal, atau alasan cuti tidak valid."}
	}
	if in.AttachmentFileID != nil && len(*in.AttachmentFileID) != 26 {
		return time.Time{}, time.Time{}, leaveIssue{400, "VALIDATION_FAILED", "ID lampiran tidak valid."}
	}
	return start, end, nil
}

func writeLeaveIssue(w http.ResponseWriter, r *http.Request, err error) {
	var issue leaveIssue
	if errors.As(err, &issue) {
		writeAPIError(w, r, issue.Status, issue.Code, issue.Message)
		return
	}
	writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa pengajuan cuti.")
}

func (a adminAPI) prepareLeave(r *http.Request, q leaveQuery, tenantID, employeeID, zone string, in leaveInput) (leavePrepared, error) {
	var p leavePrepared
	start, end, err := validateLeaveInput(&in)
	if err != nil {
		return p, err
	}
	p.Type, err = scanEssLeaveType(q.QueryRowContext(r.Context(), `SELECT id,code,name,is_paid,requires_attachment,max_consecutive_days FROM leave_types WHERE tenant_id=? AND id=? AND is_active=TRUE`, tenantID, in.LeaveTypeID))
	if errors.Is(err, sql.ErrNoRows) {
		return p, leaveConflict("LEAVE_TYPE_NOT_FOUND", "Jenis cuti tidak tersedia.")
	}
	if err != nil {
		return p, err
	}
	if p.Type.RequiresAttachment && in.AttachmentFileID == nil {
		return p, leaveIssue{400, "ATTACHMENT_REQUIRED", "Jenis cuti ini memerlukan lampiran."}
	}
	if in.AttachmentFileID != nil {
		var count int
		if err = q.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM files WHERE tenant_id=? AND id=? AND purpose IN ('leave','cuti') AND scan_status='clean' AND deleted_at IS NULL`, tenantID, *in.AttachmentFileID).Scan(&count); err != nil {
			return p, err
		}
		if count != 1 {
			return p, leaveConflict("ATTACHMENT_NOT_READY", "Lampiran cuti belum tersedia atau belum lolos pemeriksaan.")
		}
	}
	var overlap int
	if err = q.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_requests WHERE tenant_id=? AND employee_id=? AND start_date<=? AND end_date>=? AND status IN ('submitted','in_review','approved')`, tenantID, employeeID, in.EndDate, in.StartDate).Scan(&overlap); err != nil {
		return p, err
	}
	if overlap > 0 {
		return p, leaveConflict("LEAVE_OVERLAP", "Sudah ada cuti aktif pada rentang tanggal ini.")
	}
	rows, err := q.QueryContext(r.Context(), `SELECT work_date,day_type,status FROM work_schedules WHERE tenant_id=? AND employee_id=? AND work_date BETWEEN ? AND ? ORDER BY work_date`, tenantID, employeeID, in.StartDate, in.EndDate)
	if err != nil {
		return p, err
	}
	byDate := map[string]string{}
	for rows.Next() {
		var day time.Time
		var kind, status string
		if err = rows.Scan(&day, &kind, &status); err != nil {
			break
		}
		if status == "published" {
			byDate[day.Format("2006-01-02")] = kind
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return p, err
	}
	allocations := map[int]*leaveAllocation{}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		kind, exists := byDate[key]
		if !exists {
			return p, leaveConflict("ROSTER_NOT_PUBLISHED", "Roster harus terbit untuk seluruh tanggal pengajuan, termasuk hari libur.")
		}
		if kind != "shift" {
			continue
		}
		year := day.Year()
		if allocations[year] == nil {
			allocations[year] = &leaveAllocation{Year: year, FirstWorkDate: key}
		}
		allocations[year].Days++
		p.Days++
	}
	if p.Days == 0 {
		return p, leaveConflict("NO_WORK_DAYS", "Rentang cuti tidak memiliki hari kerja pada roster.")
	}
	if p.Type.MaxConsecutiveDays != nil && p.Days > *p.Type.MaxConsecutiveDays {
		return p, leaveConflict("LEAVE_LIMIT", "Jumlah hari kerja melebihi batas jenis cuti.")
	}
	for year := start.Year(); year <= end.Year(); year++ {
		segmentStart := in.StartDate
		segmentEnd := in.EndDate
		if year > start.Year() {
			segmentStart = fmt.Sprintf("%04d-01-01", year)
		}
		if year < end.Year() {
			segmentEnd = fmt.Sprintf("%04d-12-31", year)
		}
		var policyID string
		var eligibility sql.NullString
		err = q.QueryRowContext(r.Context(), `SELECT id,CAST(eligibility_json AS CHAR) FROM leave_policies WHERE tenant_id=? AND leave_type_id=? AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) ORDER BY valid_from DESC,id LIMIT 1`, tenantID, in.LeaveTypeID, segmentStart, segmentEnd).Scan(&policyID, &eligibility)
		if errors.Is(err, sql.ErrNoRows) {
			return p, leaveConflict("LEAVE_POLICY_NOT_FOUND", "Kebijakan cuti aktif belum mencakup seluruh tanggal pengajuan.")
		}
		if err != nil {
			return p, err
		}
		if eligibility.Valid && eligibility.String != "" && eligibility.String != "{}" {
			return p, leaveConflict("POLICY_NOT_READY", "Aturan kelayakan khusus pada kebijakan ini belum dapat dievaluasi.")
		}
	}
	for year := start.Year(); year <= end.Year(); year++ {
		allocation := allocations[year]
		if allocation == nil {
			continue
		}
		from, to := fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-01-01", year+1)
		var balance, reserved int64
		if err = q.QueryRowContext(r.Context(), `SELECT COALESCE(CAST(ROUND(SUM(days)*100) AS SIGNED),0) FROM leave_balance_ledger WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND effective_date>=? AND effective_date<?`, tenantID, employeeID, in.LeaveTypeID, from, to).Scan(&balance); err != nil {
			return p, err
		}
		if err = q.QueryRowContext(r.Context(), `SELECT COALESCE(CAST(ROUND(SUM(a.days)*100) AS SIGNED),0) FROM leave_request_allocations a JOIN leave_requests lr ON lr.tenant_id=a.tenant_id AND lr.id=a.leave_request_id WHERE a.tenant_id=? AND lr.employee_id=? AND lr.leave_type_id=? AND a.period_year=? AND lr.status IN ('submitted','in_review')`, tenantID, employeeID, in.LeaveTypeID, year).Scan(&reserved); err != nil {
			return p, err
		}
		if balance-reserved < int64(allocation.Days)*100 {
			return p, leaveConflict("INSUFFICIENT_LEAVE_BALANCE", fmt.Sprintf("Saldo cuti tahun %d tidak cukup.", year))
		}
		allocation.BalanceBefore = fmt.Sprintf("%.2f", float64(balance)/100)
		allocation.Reserved = fmt.Sprintf("%.2f", float64(reserved)/100)
		allocation.BalanceAfter = fmt.Sprintf("%.2f", float64(balance-reserved-int64(allocation.Days)*100)/100)
		p.Allocations = append(p.Allocations, *allocation)
	}
	today := time.Now().Format("2006-01-02")
	p.Flow, err = scanFlow(q.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND request_type='leave' AND is_active=TRUE AND is_default=TRUE AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) ORDER BY valid_from DESC,id LIMIT 1`, tenantID, today, today))
	if errors.Is(err, sql.ErrNoRows) {
		return p, leaveConflict("FLOW_NOT_FOUND", "Alur approval cuti belum tersedia.")
	}
	if err != nil {
		return p, err
	}
	step, err := scanFlowStep(q.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=1`, tenantID, p.Flow.ID))
	if err != nil || (step.RequiredPermission != nil && *step.RequiredPermission != "cuti.approve") {
		return p, leaveConflict("FLOW_INVALID", "Tahap awal approval cuti tidak valid.")
	}
	employee, err := scanEmployee(q.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, tenantID, employeeID))
	if err != nil {
		return p, leaveConflict("EMPLOYEE_INVALID", "Penugasan karyawan belum tersedia.")
	}
	target, err := a.targetFor(r.Context(), tenantID, employee)
	if err != nil {
		return p, err
	}
	candidates, err := a.stepApproverCandidates(r, tenantID, today, employee, step)
	if err != nil {
		return p, err
	}
	p.Approvers, err = a.eligibleApprovers(r, tenantID, zone, employee, target, "cuti.approve", candidates)
	if err != nil {
		return p, err
	}
	if len(p.Approvers) == 0 && p.Flow.FallbackRoleID != nil {
		candidates, err = a.roleApproverCandidates(r, tenantID, *p.Flow.FallbackRoleID, today)
		if err != nil {
			return p, err
		}
		p.Approvers, err = a.eligibleApprovers(r, tenantID, zone, employee, target, "cuti.approve", candidates)
		if err != nil {
			return p, err
		}
	}
	if len(p.Approvers) == 0 {
		return p, leaveConflict("NO_APPROVER", "Belum ada approver cuti yang berhak.")
	}
	return p, nil
}

func (a adminAPI) previewEssLeave(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	var in leaveInput
	if !readJSON(w, r, &in) {
		return
	}
	p, err := a.prepareLeave(r, a.db, c.state.SelectedTenantID, c.employeeID, c.zone.String(), in)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	writeAPIData(w, 200, map[string]any{"leave_type": p.Type, "days": p.Days, "allocations": p.Allocations, "approval_flow_id": p.Flow.ID, "approver_user_ids": p.Approvers}, nil)
}

func (a adminAPI) submitEssLeave(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	_, permission, err := a.access.Decide(r.Context(), c.state.SelectedTenantID, c.state.User.ID, c.zone.String(), "cuti.buat")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak pengajuan cuti.")
		return
	}
	if !permission.Allowed {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak mengajukan cuti tidak tersedia.")
		return
	}
	var in leaveInput
	if !readJSON(w, r, &in) {
		return
	}
	if _, _, err = validateLeaveInput(&in); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	tenantID := c.state.SelectedTenantID
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	defer tx.Rollback()
	var locked string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, c.employeeID).Scan(&locked); err != nil {
		writeAPIError(w, r, 404, "EMPLOYEE_NOT_FOUND", "Karyawan tidak ditemukan.")
		return
	}
	p, err := a.prepareLeave(r, tx, tenantID, c.employeeID, c.zone.String(), in)
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	requestID, err := ids.New()
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	leaveID, err := ids.New()
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	instanceID, err := ids.New()
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO requests (tenant_id,id,request_type,subject_employee_id,requested_by,ref_table,ref_id,status,submitted_at) VALUES (?,?,'leave',?,?,'leave_requests',?,'in_review',UTC_TIMESTAMP(6))`, tenantID, requestID, c.employeeID, c.state.User.ID, leaveID); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO leave_requests (tenant_id,id,employee_id,leave_type_id,start_date,end_date,days,reason,attachment_file_id,status,request_id) VALUES (?,?,?,?,?,?,?,?,?,'in_review',?)`, tenantID, leaveID, c.employeeID, in.LeaveTypeID, in.StartDate, in.EndDate, p.Days, in.Reason, in.AttachmentFileID, requestID); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	for _, allocation := range p.Allocations {
		allocationID, e := ids.New()
		if e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO leave_request_allocations (tenant_id,id,leave_request_id,period_year,first_work_date,days) VALUES (?,?,?,?,?,?)`, tenantID, allocationID, leaveID, allocation.Year, allocation.FirstWorkDate, allocation.Days); e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_instances (tenant_id,id,request_id,flow_id,current_level,status,version) VALUES (?,?,?,?,1,'in_review',1)`, tenantID, instanceID, requestID, p.Flow.ID); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	for _, userID := range p.Approvers {
		taskID, e := ids.New()
		if e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,1,1,?,'pending')`, tenantID, taskID, instanceID, userID); e != nil {
			writeLeaveIssue(w, r, e)
			return
		}
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID, "request_type": "leave", "approver_user_ids": p.Approvers})
	outboxID, err := ids.New()
	if err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.submitted','request',?,?,?)`, tenantID, outboxID, requestID, string(payload), "approval.submitted:"+requestID); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = audit(r.Context(), tx, tenantID, c.state.User.ID, "submit", "requests", requestID, nil, map[string]any{"request_type": "leave", "leave_id": leaveID, "days": p.Days, "allocations": p.Allocations}); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	if err = tx.Commit(); err != nil {
		writeLeaveIssue(w, r, err)
		return
	}
	writeAPIData(w, 201, map[string]any{"id": leaveID, "request_id": requestID, "leave_type_id": in.LeaveTypeID, "start_date": in.StartDate, "end_date": in.EndDate, "days": p.Days, "allocations": p.Allocations, "status": "in_review", "approval_version": 1, "approver_user_ids": p.Approvers}, nil)
}
