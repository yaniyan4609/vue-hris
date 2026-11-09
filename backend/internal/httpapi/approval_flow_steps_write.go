package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) approvalStepRefExists(ctx context.Context, tenantID string, step approvalFlowStep) (bool, error) {
	if step.ApproverKind == "reports_to" || step.ApproverKind == "manager" {
		return step.ApproverRefID == nil, nil
	}
	if step.ApproverRefID == nil || len(*step.ApproverRefID) != 26 {
		return false, nil
	}
	var query string
	switch step.ApproverKind {
	case "role", "fallback_role":
		query = `SELECT COUNT(*) FROM roles WHERE tenant_id=? AND id=?`
	case "position":
		query = `SELECT COUNT(*) FROM positions WHERE tenant_id=? AND id=? AND deleted_at IS NULL`
	case "membership", "user":
		query = `SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id=? AND user_id=? AND status='active'`
	default:
		return false, nil
	}
	var count int
	if err := a.db.QueryRowContext(ctx, query, tenantID, *step.ApproverRefID).Scan(&count); err != nil {
		return false, err
	}
	return count == 1, nil
}

func (a adminAPI) validateApprovalSteps(r *http.Request, tenantID string, steps []approvalFlowStep) (bool, error) {
	if len(steps) == 0 || len(steps) > 10 {
		return false, nil
	}
	seen := map[int]bool{}
	for _, step := range steps {
		if step.LevelNo < 1 || step.LevelNo > len(steps) || seen[step.LevelNo] {
			return false, nil
		}
		seen[step.LevelNo] = true
		if step.SLAHours != nil && (*step.SLAHours < 1 || *step.SLAHours > 720) {
			return false, nil
		}
		allowed, err := a.approvalStepRefExists(r.Context(), tenantID, step)
		if err != nil || !allowed {
			return false, err
		}
		if step.RequiredPermission != nil {
			if len(*step.RequiredPermission) > 96 || !strings.Contains(*step.RequiredPermission, ".") {
				return false, nil
			}
			var count int
			if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM permission_registry WHERE code=?`, *step.RequiredPermission).Scan(&count); err != nil {
				return false, err
			}
			if count != 1 {
				return false, nil
			}
		}
	}
	return true, nil
}

func scanFlowStep(row interface{ Scan(...any) error }) (approvalFlowStep, error) {
	var step approvalFlowStep
	var ref, permission sql.NullString
	var sla sql.NullInt64
	err := row.Scan(&step.ID, &step.LevelNo, &step.ApproverKind, &ref, &permission, &sla)
	if err != nil {
		return step, err
	}
	step.ApproverKind = externalApproverKind(step.ApproverKind)
	if ref.Valid {
		step.ApproverRefID = &ref.String
	}
	if permission.Valid {
		step.RequiredPermission = &permission.String
	}
	if sla.Valid {
		value := int(sla.Int64)
		step.SLAHours = &value
	}
	return step, nil
}

func loadFlowSteps(ctx context.Context, tx *sql.Tx, tenantID, flowID string) ([]approvalFlowStep, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? ORDER BY level_no,id`, tenantID, flowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []approvalFlowStep{}
	for rows.Next() {
		item, err := scanFlowStep(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (a adminAPI) replaceApprovalFlowSteps(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengaturan.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Alur approval tidak ditemukan.")
		return
	}
	var body struct {
		Version uint64             `json:"version"`
		Steps   []approvalFlowStep `json:"steps"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	valid, err := a.validateApprovalSteps(r, c.tenant.ID, body.Steps)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memvalidasi tahap approval.")
		return
	}
	if body.Version == 0 || !valid {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tahap approval harus lengkap, berurutan, dan mengacu data tenant yang sah.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan tahap approval.")
		return
	}
	defer tx.Rollback()
	flow, err := scanFlow(tx.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Alur approval tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	if !flow.IsActive {
		writeAPIError(w, r, 409, "FLOW_INACTIVE", "Alur approval tidak aktif.")
		return
	}
	if flow.Version != body.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Alur approval sudah berubah.")
		return
	}
	for _, step := range body.Steps {
		if step.RequiredPermission != nil && *step.RequiredPermission != approvalPermissions[flow.RequestType] {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Permission tahap harus sesuai jenis pengajuan.")
			return
		}
	}
	var used int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM approval_instances WHERE tenant_id=? AND flow_id=?`, c.tenant.ID, id).Scan(&used); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa pemakaian alur.")
		return
	}
	if used > 0 {
		writeAPIError(w, r, 409, "FLOW_IN_USE", "Alur yang sudah dipakai tidak dapat mengubah tahap; buat alur baru.")
		return
	}
	old, err := loadFlowSteps(r.Context(), tx, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tahap lama.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM approval_flow_steps WHERE tenant_id=? AND flow_id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengganti tahap approval.")
		return
	}
	for i := range body.Steps {
		newID, err := ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tahap approval.")
			return
		}
		body.Steps[i].ID = newID
		step := body.Steps[i]
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO approval_flow_steps (tenant_id,id,flow_id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours) VALUES (?,?,?,?,?,?,?,?)`, c.tenant.ID, newID, id, step.LevelNo, storedApproverKind(step.ApproverKind), step.ApproverRefID, step.RequiredPermission, step.SLAHours); err != nil {
			writeAPIError(w, r, 409, "STEP_CONFLICT", "Tahap approval duplikat atau referensi berubah.")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_flows SET version=version+1 WHERE tenant_id=? AND id=? AND version=?`, c.tenant.ID, id, flow.Version); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menaikkan versi alur.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "replace_steps", "approval_flows", id, map[string]any{"version": flow.Version, "steps": old}, map[string]any{"version": flow.Version + 1, "steps": body.Steps}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan tahap approval.")
		return
	}
	writeAPIData(w, 200, map[string]any{"version": flow.Version + 1, "steps": body.Steps}, nil)
}

func (a adminAPI) setDefaultApprovalFlow(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengaturan.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Alur approval tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengatur alur default.")
		return
	}
	defer tx.Rollback()
	var locked string
	if err := tx.QueryRowContext(r.Context(), `SELECT id FROM tenants WHERE id=? FOR UPDATE`, c.tenant.ID).Scan(&locked); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci konfigurasi tenant.")
		return
	}
	item, err := scanFlow(tx.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Alur approval tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	if !item.IsActive {
		writeAPIError(w, r, 409, "FLOW_INACTIVE", "Alur approval tidak aktif.")
		return
	}
	var stepCount int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM approval_flow_steps WHERE tenant_id=? AND flow_id=?`, c.tenant.ID, id).Scan(&stepCount); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa tahap approval.")
		return
	}
	if stepCount == 0 {
		writeAPIError(w, r, 409, "FLOW_INCOMPLETE", "Alur belum memiliki tahap approval.")
		return
	}
	if item.IsDefault {
		writeAPIData(w, 200, item, nil)
		return
	}
	rows, err := tx.QueryContext(r.Context(), `SELECT id,version FROM approval_flows WHERE tenant_id=? AND request_type=? AND is_default=TRUE`, c.tenant.ID, item.RequestType)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur default lama.")
		return
	}
	type oldDefault struct {
		id      string
		version uint64
	}
	previous := []oldDefault{}
	for rows.Next() {
		var old oldDefault
		if err := rows.Scan(&old.id, &old.version); err != nil {
			rows.Close()
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur default lama.")
			return
		}
		previous = append(previous, old)
	}
	if rows.Err() != nil {
		rows.Close()
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur default lama.")
		return
	}
	rows.Close()
	for _, old := range previous {
		if _, err := tx.ExecContext(r.Context(), `UPDATE approval_flows SET is_default=FALSE,version=version+1 WHERE tenant_id=? AND id=?`, c.tenant.ID, old.id); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengganti alur default.")
			return
		}
		if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "unset_default", "approval_flows", old.id, map[string]any{"is_default": true, "version": old.version}, map[string]any{"is_default": false, "version": old.version + 1}); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_flows SET is_default=TRUE,version=version+1 WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengatur alur default.")
		return
	}
	after := item
	after.IsDefault = true
	after.Version++
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "set_default", "approval_flows", id, item, after); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengatur alur default.")
		return
	}
	writeAPIData(w, 200, after, nil)
}
