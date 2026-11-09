package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type shiftSwap struct {
	ID          string     `json:"id"`
	RequesterID string     `json:"requester_id"`
	PartnerID   string     `json:"partner_id"`
	InstanceAID string     `json:"instance_a_id"`
	InstanceBID string     `json:"instance_b_id"`
	Status      string     `json:"status"`
	RequestID   *string    `json:"request_id"`
	Reason      string     `json:"reason"`
	Version     uint64     `json:"version"`
	RespondedAt *time.Time `json:"responded_at"`
	AppliedAt   *time.Time `json:"applied_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func notifyShiftSwap(ctx context.Context, tx *sql.Tx, tenantID, userID, swapID, event, title, body string) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notifications (tenant_id,id,recipient_user_id,category,event_type,title,body,deep_link,dedupe_key) VALUES (?,?,?,'system',?,?,?,'/ess/shift-swaps',?)`, tenantID, id, userID, event, title, body, "shift-swap:"+swapID+":"+event+":"+userID)
	return err
}

const swapColumns = `s.id,s.requester_id,s.partner_id,s.instance_a_id,s.instance_b_id,s.status,s.request_id,s.reason,s.version,s.responded_at,s.applied_at,s.created_at`

func scanSwap(row interface{ Scan(...any) error }) (shiftSwap, error) {
	var x shiftSwap
	var request sql.NullString
	var responded, applied sql.NullTime
	err := row.Scan(&x.ID, &x.RequesterID, &x.PartnerID, &x.InstanceAID, &x.InstanceBID, &x.Status, &request, &x.Reason, &x.Version, &responded, &applied, &x.CreatedAt)
	if request.Valid {
		x.RequestID = &request.String
	}
	if responded.Valid {
		x.RespondedAt = &responded.Time
	}
	if applied.Valid {
		x.AppliedAt = &applied.Time
	}
	return x, err
}
func (a adminAPI) listEssShiftSwaps(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	where := `s.tenant_id=? AND (s.requester_id=? OR s.partner_id=?)`
	args := []any{c.state.SelectedTenantID, c.employeeID, c.employeeID}
	if status := r.URL.Query().Get("status"); status != "" {
		if !validSwapStatus(status) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status tidak valid.")
			return
		}
		where += ` AND s.status=?`
		args = append(args, status)
	}
	a.listSwaps(w, r, where, args, page)
}
func validSwapStatus(status string) bool {
	switch status {
	case "pending_partner", "in_review", "returned", "approved", "rejected", "cancelled":
		return true
	}
	return false
}
func (a adminAPI) listAdminShiftSwaps(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	partnerScope := strings.ReplaceAll(strings.ReplaceAll(scope, "e.", "p."), "a.", "pa.")
	where := `s.tenant_id=? AND ` + scope + ` AND ` + partnerScope
	args := append([]any{c.tenant.ID}, scopeArgs...)
	args = append(args, scopeArgs...)
	if status := r.URL.Query().Get("status"); status != "" {
		if !validSwapStatus(status) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status tidak valid.")
			return
		}
		where += ` AND s.status=?`
		args = append(args, status)
	}
	if id := r.URL.Query().Get("employee_id"); id != "" {
		if len(id) != 26 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Karyawan tidak valid.")
			return
		}
		where += ` AND (s.requester_id=? OR s.partner_id=?)`
		args = append(args, id, id)
	}
	join := ` JOIN employees e ON e.tenant_id=s.tenant_id AND e.id=s.requester_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE JOIN employees p ON p.tenant_id=s.tenant_id AND p.id=s.partner_id JOIN employee_assignments pa ON pa.tenant_id=p.tenant_id AND pa.employee_id=p.id AND pa.current_flag=TRUE`
	a.listSwapsJoined(w, r, where, args, page, join)
}
func (a adminAPI) listSwaps(w http.ResponseWriter, r *http.Request, where string, args []any, page pageOptions) {
	a.listSwapsJoined(w, r, where, args, page, "")
}
func (a adminAPI) listSwapsJoined(w http.ResponseWriter, r *http.Request, where string, args []any, page pageOptions, join string) {
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM shift_swaps s`+join+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pertukaran shift.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+swapColumns+` FROM shift_swaps s`+join+` WHERE `+where+` ORDER BY s.created_at DESC,s.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pertukaran shift.")
		return
	}
	defer rows.Close()
	items := []shiftSwap{}
	for rows.Next() {
		item, e := scanSwap(rows)
		if e != nil {
			err = e
			break
		}
		items = append(items, item)
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pertukaran shift.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) createEssShiftSwap(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	tenantID := c.state.SelectedTenantID
	var body struct {
		InstanceAID string `json:"instance_a_id"`
		InstanceBID string `json:"instance_b_id"`
		Reason      string `json:"reason"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if len(body.InstanceAID) != 26 || len(body.InstanceBID) != 26 || body.InstanceAID == body.InstanceBID || body.Reason == "" || len([]rune(body.Reason)) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Instance atau alasan pertukaran tidak valid.")
		return
	}
	employee, err := scanEmployee(a.db.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, tenantID, c.employeeID))
	if err != nil {
		writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Profil karyawan tidak tersedia.")
		return
	}
	target, err := a.targetFor(r.Context(), tenantID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope.")
		return
	}
	subject, right, err := a.access.Decide(r.Context(), tenantID, c.state.User.ID, c.zone.String(), "jadwal.ubah")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa izin.")
		return
	}
	if !right.Allowed || !right.Includes(subject, target) {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak mengajukan tukar shift tidak tersedia.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat pertukaran.")
		return
	}
	defer tx.Rollback()
	first, second := body.InstanceAID, body.InstanceBID
	if first > second {
		first, second = second, first
	}
	locked := map[string]shiftInstance{}
	for _, id := range []string{first, second} {
		item, e := scanInstance(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(instanceColumns, "i.", "")+` FROM shift_instances WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, id))
		if e != nil {
			writeAPIError(w, r, 404, "NOT_FOUND", "Shift tidak ditemukan dalam tenant.")
			return
		}
		locked[id] = item
	}
	aShift, bShift := locked[body.InstanceAID], locked[body.InstanceBID]
	if aShift.EmployeeID != c.employeeID || bShift.EmployeeID == c.employeeID || aShift.Status != "scheduled" || bShift.Status != "scheduled" || aShift.WorkDate < time.Now().In(c.zone).Format("2006-01-02") || bShift.WorkDate < time.Now().In(c.zone).Format("2006-01-02") {
		writeAPIError(w, r, 409, "SHIFT_UNAVAILABLE", "Shift harus mendatang, belum dipakai, dan milik dua karyawan berbeda.")
		return
	}
	var partnerUser string
	if err = tx.QueryRowContext(r.Context(), `SELECT e.user_id FROM employees e JOIN tenant_memberships m ON m.tenant_id=e.tenant_id AND m.employee_id=e.id AND m.user_id=e.user_id AND m.status='active' WHERE e.tenant_id=? AND e.id=? AND e.status IN ('active','probation','leave')`, tenantID, bShift.EmployeeID).Scan(&partnerUser); err != nil {
		writeAPIError(w, r, 409, "PARTNER_UNAVAILABLE", "Rekan tidak memiliki akun aktif.")
		return
	}
	var conflicts int
	if err = tx.QueryRowContext(r.Context(), `SELECT (SELECT COUNT(*) FROM attendance_events WHERE tenant_id=? AND shift_instance_id IN (?,?))+(SELECT COUNT(*) FROM shift_swaps WHERE tenant_id=? AND status IN ('pending_partner','in_review','returned') AND (instance_a_id IN (?,?) OR instance_b_id IN (?,?)))`, tenantID, first, second, tenantID, first, second, first, second).Scan(&conflicts); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa benturan.")
		return
	}
	if conflicts > 0 {
		writeAPIError(w, r, 409, "SHIFT_IN_USE", "Shift sudah dipakai atau sedang ditukar.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO shift_swaps (tenant_id,id,requester_id,partner_id,instance_a_id,instance_b_id,status,reason) VALUES (?,?,?,?,?,?,'pending_partner',?)`, tenantID, id, c.employeeID, bShift.EmployeeID, body.InstanceAID, body.InstanceBID, body.Reason); err != nil {
		writeAPIError(w, r, 409, "SWAP_CONFLICT", "Gagal menyimpan usulan.")
		return
	}
	if err = notifyShiftSwap(r.Context(), tx, tenantID, partnerUser, id, "schedule.swap_proposed", "Usulan tukar shift", "Rekan mengajukan pertukaran shift."); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi rekan.")
		return
	}
	if err = audit(r.Context(), tx, tenantID, c.state.User.ID, "create", "shift_swaps", id, nil, map[string]any{"requester_id": c.employeeID, "partner_id": bShift.EmployeeID, "instance_a_id": body.InstanceAID, "instance_b_id": body.InstanceBID, "status": "pending_partner"}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan usulan.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "requester_id": c.employeeID, "partner_id": bShift.EmployeeID, "instance_a_id": body.InstanceAID, "instance_b_id": body.InstanceBID, "status": "pending_partner", "reason": body.Reason, "version": 1, "partner_user_id": partnerUser}, nil)
}

func (a adminAPI) respondEssShiftSwap(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	tenantID := c.state.SelectedTenantID
	id := chi.URLParam(r, "id")
	var body struct {
		Accept  *bool  `json:"accept"`
		Version uint64 `json:"version"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if len(id) != 26 || body.Accept == nil || body.Version == 0 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Keputusan atau versi tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memproses respons.")
		return
	}
	defer tx.Rollback()
	swap, err := scanSwap(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(swapColumns, "s.", "")+` FROM shift_swaps WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, id))
	if errors.Is(err, sql.ErrNoRows) || err == nil && swap.PartnerID != c.employeeID {
		writeAPIError(w, r, 404, "NOT_FOUND", "Usulan tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca usulan.")
		return
	}
	if swap.Status != "pending_partner" || swap.Version != body.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Usulan sudah berubah.")
		return
	}
	if !*body.Accept {
		if _, err = tx.ExecContext(r.Context(), `UPDATE shift_swaps SET status='rejected',version=version+1,responded_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, tenantID, id); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menolak usulan.")
			return
		}
		if err = audit(r.Context(), tx, tenantID, c.state.User.ID, "reject", "shift_swaps", id, map[string]any{"status": swap.Status}, map[string]any{"status": "rejected"}); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
			return
		}
		var requesterUser string
		if err = tx.QueryRowContext(r.Context(), `SELECT user_id FROM employees WHERE tenant_id=? AND id=?`, tenantID, swap.RequesterID).Scan(&requesterUser); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca akun pengaju.")
			return
		}
		if err = notifyShiftSwap(r.Context(), tx, tenantID, requesterUser, id, "schedule.swap_rejected", "Tukar shift ditolak", "Rekan menolak usulan pertukaran shift."); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi pengaju.")
			return
		}
		if err = tx.Commit(); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan respons.")
			return
		}
		writeAPIData(w, 200, map[string]any{"id": id, "status": "rejected", "version": swap.Version + 1}, nil)
		return
	}
	first, second := swap.InstanceAID, swap.InstanceBID
	if first > second {
		first, second = second, first
	}
	locked := map[string]shiftInstance{}
	for _, instanceID := range []string{first, second} {
		item, e := scanInstance(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(instanceColumns, "i.", "")+` FROM shift_instances WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, instanceID))
		if e != nil {
			writeAPIError(w, r, 409, "SHIFT_UNAVAILABLE", "Shift tidak tersedia.")
			return
		}
		locked[instanceID] = item
	}
	aShift, bShift := locked[swap.InstanceAID], locked[swap.InstanceBID]
	if aShift.EmployeeID != swap.RequesterID || bShift.EmployeeID != swap.PartnerID || aShift.Status != "scheduled" || bShift.Status != "scheduled" || aShift.WorkDate < time.Now().In(c.zone).Format("2006-01-02") || bShift.WorkDate < time.Now().In(c.zone).Format("2006-01-02") {
		writeAPIError(w, r, 409, "SHIFT_UNAVAILABLE", "Shift sudah berubah atau berlalu.")
		return
	}
	var used int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM attendance_events WHERE tenant_id=? AND shift_instance_id IN (?,?)`, tenantID, first, second).Scan(&used); err != nil || used > 0 {
		writeAPIError(w, r, 409, "SHIFT_IN_USE", "Shift sudah dipakai presensi.")
		return
	}
	today := time.Now().In(c.zone).Format("2006-01-02")
	flow, err := scanFlow(tx.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND request_type='shift_swap' AND is_default=TRUE AND is_active=TRUE AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) ORDER BY valid_from DESC,id LIMIT 1 FOR UPDATE`, tenantID, today, today))
	if err != nil {
		writeAPIError(w, r, 409, "FLOW_NOT_FOUND", "Alur approval tukar shift belum tersedia.")
		return
	}
	step, err := scanFlowStep(tx.QueryRowContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? AND level_no=1`, tenantID, flow.ID))
	if err != nil || (step.RequiredPermission != nil && *step.RequiredPermission != "jadwal.approve") {
		writeAPIError(w, r, 409, "FLOW_INVALID", "Tahap awal approval tidak valid.")
		return
	}
	employee, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, tenantID, swap.RequesterID))
	if err != nil {
		writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Pengaju tidak tersedia.")
		return
	}
	target, err := a.targetFor(r.Context(), tenantID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope.")
		return
	}
	candidates, err := a.stepApproverCandidates(r, tenantID, today, employee, step)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan approver.")
		return
	}
	eligible, err := a.eligibleApprovers(r, tenantID, c.zone.String(), employee, target, "jadwal.approve", candidates, "shift_swap")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa approver.")
		return
	}
	var requesterUser string
	if err = tx.QueryRowContext(r.Context(), `SELECT user_id FROM employees WHERE tenant_id=? AND id=?`, tenantID, swap.RequesterID).Scan(&requesterUser); err != nil {
		writeAPIError(w, r, 409, "EMPLOYEE_INVALID", "Akun pengaju tidak tersedia.")
		return
	}
	eligible = removeApprover(eligible, requesterUser)
	eligible = removeApprover(eligible, c.state.User.ID)
	if len(eligible) == 0 && flow.FallbackRoleID != nil {
		candidates, err = a.roleApproverCandidates(r, tenantID, *flow.FallbackRoleID, today)
		if err == nil {
			eligible, err = a.eligibleApprovers(r, tenantID, c.zone.String(), employee, target, "jadwal.approve", candidates, "shift_swap")
		}
		eligible = removeApprover(removeApprover(eligible, requesterUser), c.state.User.ID)
	}
	if err != nil || len(eligible) == 0 {
		writeAPIError(w, r, 409, "NO_APPROVER", "Belum ada approver tukar shift yang berhak.")
		return
	}
	requestID, _ := ids.New()
	approvalID, _ := ids.New()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO requests (tenant_id,id,request_type,subject_employee_id,requested_by,ref_table,ref_id,status,submitted_at) VALUES (?,?,'shift_swap',?,?,'shift_swaps',?,'in_review',UTC_TIMESTAMP(6))`, tenantID, requestID, swap.RequesterID, requesterUser, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat pengajuan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE shift_swaps SET status='in_review',request_id=?,version=version+1,responded_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=? AND status='pending_partner'`, requestID, tenantID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui usulan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_instances (tenant_id,id,request_id,flow_id,current_level,status,version) VALUES (?,?,?,?,1,'in_review',1)`, tenantID, approvalID, requestID, flow.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka approval.")
		return
	}
	for _, userID := range eligible {
		taskID, _ := ids.New()
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_tasks (tenant_id,id,instance_id,attempt_no,level_no,assignee_user_id,status) VALUES (?,?,?,1,1,?,'pending')`, tenantID, taskID, approvalID, userID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tugas approval.")
			return
		}
	}
	payload, _ := json.Marshal(map[string]any{"request_id": requestID, "request_type": "shift_swap", "approver_user_ids": eligible})
	outboxID, _ := ids.New()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?, 'approval.submitted','request',?,?,?)`, tenantID, outboxID, requestID, string(payload), "approval.submitted:"+requestID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	if err = audit(r.Context(), tx, tenantID, c.state.User.ID, "accept", "shift_swaps", id, map[string]any{"status": "pending_partner"}, map[string]any{"status": "in_review", "request_id": requestID}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan persetujuan rekan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "status": "in_review", "request_id": requestID, "version": swap.Version + 1, "approval_version": 1}, nil)
}
