package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"time"

	"digaji.id/hris/internal/access"
)

var approvalPermissions = map[string]string{
	"leave": "cuti.approve", "overtime": "lembur.approve", "attendance_correction": "absensi.approve",
	"loan": "kasbon.approve", "reimbursement": "reimbursement.approve", "mutation": "mutasi.approve",
	"promotion": "mutasi.approve", "payroll": "penggajian.approve", "termination": "terminasi.approve",
	"shift_swap": "jadwal.approve", "profile_change": "karyawan.approve",
}

func (a adminAPI) roleApproverCandidates(r *http.Request, tenantID, roleID, date string) ([]string, error) {
	rows, err := a.db.QueryContext(r.Context(), `SELECT DISTINCT ur.user_id FROM user_roles ur JOIN tenant_memberships m ON m.tenant_id=ur.tenant_id AND m.user_id=ur.user_id AND m.status='active' WHERE ur.tenant_id=? AND ur.role_id=? AND ur.revoked_at IS NULL AND ur.valid_from<=? AND (ur.valid_until IS NULL OR ur.valid_until>=?) ORDER BY ur.user_id LIMIT 201`, tenantID, roleID, date, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func (a adminAPI) stepApproverCandidates(r *http.Request, tenantID, date string, employee employee, step approvalFlowStep) ([]string, error) {
	switch step.ApproverKind {
	case "reports_to", "manager":
		if employee.ReportsToID == "" {
			return []string{}, nil
		}
		var userID sql.NullString
		err := a.db.QueryRowContext(r.Context(), `SELECT e.user_id FROM employees e JOIN tenant_memberships m ON m.tenant_id=e.tenant_id AND m.user_id=e.user_id AND m.status='active' WHERE e.tenant_id=? AND e.id=? AND e.status IN ('active','probation','leave')`, tenantID, employee.ReportsToID).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			return []string{}, nil
		}
		if err != nil {
			return nil, err
		}
		if userID.Valid {
			return []string{userID.String}, nil
		}
		return []string{}, nil
	case "role", "fallback_role":
		if step.ApproverRefID == nil {
			return []string{}, nil
		}
		return a.roleApproverCandidates(r, tenantID, *step.ApproverRefID, date)
	case "position":
		if step.ApproverRefID == nil {
			return []string{}, nil
		}
		rows, err := a.db.QueryContext(r.Context(), `SELECT DISTINCT e.user_id FROM employees e JOIN employee_assignments x ON x.tenant_id=e.tenant_id AND x.employee_id=e.id AND x.current_flag=TRUE JOIN tenant_memberships m ON m.tenant_id=e.tenant_id AND m.user_id=e.user_id AND m.status='active' WHERE e.tenant_id=? AND x.position_id=? AND e.user_id IS NOT NULL AND e.status IN ('active','probation','leave') ORDER BY e.user_id LIMIT 201`, tenantID, *step.ApproverRefID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := []string{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			items = append(items, id)
		}
		return items, rows.Err()
	case "membership", "user":
		if step.ApproverRefID == nil {
			return []string{}, nil
		}
		var count int
		if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id=? AND user_id=? AND status='active'`, tenantID, *step.ApproverRefID).Scan(&count); err != nil {
			return nil, err
		}
		if count == 1 {
			return []string{*step.ApproverRefID}, nil
		}
		return []string{}, nil
	default:
		return []string{}, nil
	}
}

func (a adminAPI) eligibleApprovers(r *http.Request, tenantID, timezone string, employee employee, target access.Target, permission string, candidates []string, requestType ...string) ([]string, error) {
	selected, err := a.eligibleApproversDirect(r, tenantID, timezone, employee, target, permission, candidates)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return selected, nil
	}
	kind := ""
	if len(requestType) > 0 {
		kind = requestType[0]
	}
	if kind == "" {
		for name, code := range approvalPermissions {
			if code == permission {
				if kind != "" {
					kind = ""
					break
				}
				kind = name
			}
		}
	}
	zone, err := time.LoadLocation(timezone)
	if err != nil {
		zone = time.UTC
	}
	today := time.Now().In(zone).Format("2006-01-02")
	resolved := []string{}
	seen := map[string]bool{}
	for _, from := range selected {
		assignee := from
		var to, delegator string
		err = a.db.QueryRowContext(r.Context(), `SELECT to_user_id,COALESCE(created_by,'') FROM approval_delegations WHERE tenant_id=? AND from_user_id=? AND revoked_at IS NULL AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) AND (request_type=? OR request_type IS NULL) ORDER BY request_type IS NULL,created_at DESC,id DESC LIMIT 1`, tenantID, from, today, today, kind).Scan(&to, &delegator)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil && delegator != "" && to != employee.UserID {
			actorSubject, actorRight, e := a.access.Decide(r.Context(), tenantID, delegator, timezone, "persetujuan.delegasikan")
			if e != nil {
				return nil, e
			}
			if actorRight.Allowed && actorRight.Includes(actorSubject, target) {
				eligible, e := a.eligibleApproversDirect(r, tenantID, timezone, employee, target, permission, []string{to})
				if e != nil {
					return nil, e
				}
				if len(eligible) == 1 {
					assignee = to
				}
			}
		}
		if !seen[assignee] {
			seen[assignee] = true
			resolved = append(resolved, assignee)
		}
	}
	sort.Strings(resolved)
	return resolved, nil
}

func (a adminAPI) eligibleApproversDirect(r *http.Request, tenantID, timezone string, employee employee, target access.Target, permission string, candidates []string) ([]string, error) {
	if len(candidates) > 200 {
		return nil, errors.New("too many candidate approvers")
	}
	selected := []string{}
	seen := map[string]bool{}
	for _, userID := range candidates {
		if userID == "" || userID == employee.UserID || seen[userID] {
			continue
		}
		seen[userID] = true
		subject, decision, err := a.access.Decide(r.Context(), tenantID, userID, timezone, permission)
		if err != nil {
			return nil, err
		}
		if decision.Allowed && decision.Includes(subject, target) {
			selected = append(selected, userID)
		}
	}
	sort.Strings(selected)
	return selected, nil
}

func (a adminAPI) simulateApprovalFlow(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengaturan.kelola")
	if !ok {
		return
	}
	var body struct {
		RequestType string `json:"request_type"`
		EmployeeID  string `json:"employee_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if !approvalRequestTypes[body.RequestType] || len(body.EmployeeID) != 26 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jenis pengajuan atau karyawan tidak valid.")
		return
	}
	employee, err := scanEmployee(a.db.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.tenant.ID, body.EmployeeID))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
		return
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca penugasan.")
		return
	}
	employeeSubject, employeeAccess, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "karyawan.lihat")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope karyawan.")
		return
	}
	if !employeeAccess.Allowed {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Izin melihat karyawan diperlukan untuk simulasi.")
		return
	}
	if !employeeAccess.Includes(employeeSubject, target) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	today := time.Now().In(zone).Format("2006-01-02")
	flow, err := scanFlow(a.db.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND request_type=? AND is_active=TRUE AND is_default=TRUE AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) ORDER BY valid_from DESC,id LIMIT 1`, c.tenant.ID, body.RequestType, today, today))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "FLOW_NOT_FOUND", "Alur default yang berlaku belum tersedia.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? ORDER BY level_no,id`, c.tenant.ID, flow.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tahap approval.")
		return
	}
	steps := []approvalFlowStep{}
	for rows.Next() {
		step, err := scanFlowStep(rows)
		if err != nil {
			rows.Close()
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tahap approval.")
			return
		}
		steps = append(steps, step)
	}
	if rows.Err() != nil {
		rows.Close()
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tahap approval.")
		return
	}
	rows.Close()
	if len(steps) == 0 {
		writeAPIError(w, r, 409, "FLOW_INCOMPLETE", "Alur belum memiliki tahap.")
		return
	}
	levels := []map[string]any{}
	complete := true
	for _, step := range steps {
		permission := approvalPermissions[body.RequestType]
		if step.RequiredPermission != nil && *step.RequiredPermission != permission {
			writeAPIError(w, r, 409, "FLOW_INVALID", "Permission tahap tidak sesuai jenis pengajuan.")
			return
		}
		candidates, err := a.stepApproverCandidates(r, c.tenant.ID, today, employee, step)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan approver.")
			return
		}
		eligible, err := a.eligibleApprovers(r, c.tenant.ID, c.tenant.Timezone, employee, target, permission, candidates, body.RequestType)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak approver.")
			return
		}
		fallback := false
		if len(eligible) == 0 && flow.FallbackRoleID != nil {
			candidates, err = a.roleApproverCandidates(r, c.tenant.ID, *flow.FallbackRoleID, today)
			if err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan fallback.")
				return
			}
			eligible, err = a.eligibleApprovers(r, c.tenant.ID, c.tenant.Timezone, employee, target, permission, candidates, body.RequestType)
			if err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak fallback.")
				return
			}
			fallback = len(eligible) > 0
		}
		if len(eligible) == 0 {
			complete = false
		}
		levels = append(levels, map[string]any{"level_no": step.LevelNo, "approver_kind": step.ApproverKind, "required_permission": permission, "candidate_user_ids": eligible, "fallback_used": fallback, "resolved": len(eligible) > 0})
	}
	writeAPIData(w, 200, map[string]any{"flow_id": flow.ID, "flow_version": flow.Version, "request_type": body.RequestType, "employee_id": body.EmployeeID, "resolvable": complete, "levels": levels}, nil)
}
