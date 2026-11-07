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

var approvalRequestTypes = map[string]bool{"leave": true, "overtime": true, "attendance_correction": true, "loan": true, "reimbursement": true, "mutation": true, "promotion": true, "payroll": true, "termination": true, "shift_swap": true, "profile_change": true}

func validApprovalFlow(item *approvalFlow) bool {
	item.Name = strings.TrimSpace(item.Name)
	if !approvalRequestTypes[item.RequestType] || item.Name == "" || len([]rune(item.Name)) > 160 {
		return false
	}
	from, err := time.Parse("2006-01-02", item.ValidFrom)
	if err != nil || from.Format("2006-01-02") != item.ValidFrom {
		return false
	}
	if item.ValidTo != nil {
		to, err := time.Parse("2006-01-02", *item.ValidTo)
		if err != nil || to.Format("2006-01-02") != *item.ValidTo || to.Before(from) {
			return false
		}
	}
	return item.FallbackRoleID == nil || len(*item.FallbackRoleID) == 26
}

func (a adminAPI) flowFallbackExists(r *http.Request, tenantID string, id *string) (bool, error) {
	if id == nil {
		return true, nil
	}
	var count int
	err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM roles WHERE tenant_id=? AND id=?`, tenantID, *id).Scan(&count)
	return count == 1, err
}

func (a adminAPI) createApprovalFlow(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengaturan.kelola")
	if !ok {
		return
	}
	var body struct {
		RequestType    string  `json:"request_type"`
		Name           string  `json:"name"`
		ValidFrom      string  `json:"valid_from"`
		ValidTo        *string `json:"valid_to"`
		FallbackRoleID *string `json:"fallback_role_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	item := approvalFlow{RequestType: body.RequestType, Name: body.Name, ValidFrom: body.ValidFrom, ValidTo: body.ValidTo, FallbackRoleID: body.FallbackRoleID, IsActive: true, Version: 1, Steps: []approvalFlowStep{}}
	if !validApprovalFlow(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Alur approval tidak valid.")
		return
	}
	exists, err := a.flowFallbackExists(r, c.tenant.ID, item.FallbackRoleID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa role fallback.")
		return
	}
	if !exists {
		writeAPIError(w, r, 404, "NOT_FOUND", "Role fallback tidak ditemukan dalam tenant.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat alur approval.")
		return
	}
	item.ID = id
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan alur approval.")
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO approval_flows (tenant_id,id,request_type,name,is_default,is_active,version,valid_from,valid_to,fallback_role_id) VALUES (?,?,?,?,FALSE,TRUE,1,?,?,?)`, c.tenant.ID, id, item.RequestType, item.Name, item.ValidFrom, item.ValidTo, item.FallbackRoleID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan alur approval.")
		return
	}
	if err := tx.QueryRowContext(r.Context(), `SELECT created_at FROM approval_flows WHERE tenant_id=? AND id=?`, c.tenant.ID, id).Scan(&item.CreatedAt); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "approval_flows", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan alur approval.")
		return
	}
	writeAPIData(w, 201, item, nil)
}

func (a adminAPI) updateApprovalFlow(w http.ResponseWriter, r *http.Request) {
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
		Version        uint64          `json:"version"`
		Name           *string         `json:"name"`
		ValidFrom      *string         `json:"valid_from"`
		ValidTo        json.RawMessage `json:"valid_to"`
		FallbackRoleID json.RawMessage `json:"fallback_role_id"`
		IsActive       *bool           `json:"is_active"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Version == 0 || (body.Name == nil && body.ValidFrom == nil && body.ValidTo == nil && body.FallbackRoleID == nil && body.IsActive == nil) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi atau perubahan tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan alur approval.")
		return
	}
	defer tx.Rollback()
	old, err := scanFlow(tx.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Alur approval tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	if old.Version != body.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Alur approval sudah berubah.")
		return
	}
	item := old
	if body.Name != nil {
		item.Name = *body.Name
	}
	if body.ValidFrom != nil {
		item.ValidFrom = *body.ValidFrom
	}
	if body.ValidTo != nil {
		item.ValidTo, err = contractPatchText(body.ValidTo)
		if err != nil {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal akhir tidak valid.")
			return
		}
	}
	if body.FallbackRoleID != nil {
		item.FallbackRoleID, err = contractPatchText(body.FallbackRoleID)
		if err != nil {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Role fallback tidak valid.")
			return
		}
	}
	if body.IsActive != nil {
		item.IsActive = *body.IsActive
		if !item.IsActive {
			item.IsDefault = false
		}
	}
	if !validApprovalFlow(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Alur approval tidak valid.")
		return
	}
	exists, err := a.flowFallbackExists(r, c.tenant.ID, item.FallbackRoleID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa role fallback.")
		return
	}
	if !exists {
		writeAPIError(w, r, 404, "NOT_FOUND", "Role fallback tidak ditemukan dalam tenant.")
		return
	}
	item.Version++
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_flows SET name=?,valid_from=?,valid_to=?,fallback_role_id=?,is_active=?,is_default=?,version=? WHERE tenant_id=? AND id=? AND version=?`, item.Name, item.ValidFrom, item.ValidTo, item.FallbackRoleID, item.IsActive, item.IsDefault, item.Version, c.tenant.ID, id, old.Version); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan alur approval.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "approval_flows", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan alur approval.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) deactivateApprovalFlow(w http.ResponseWriter, r *http.Request) {
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
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan alur.")
		return
	}
	defer tx.Rollback()
	old, err := scanFlow(tx.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Alur approval tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur.")
		return
	}
	if !old.IsActive {
		w.WriteHeader(204)
		return
	}
	after := old
	after.IsActive = false
	after.IsDefault = false
	after.Version++
	if _, err := tx.ExecContext(r.Context(), `UPDATE approval_flows SET is_active=FALSE,is_default=FALSE,version=? WHERE tenant_id=? AND id=?`, after.Version, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan alur.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "deactivate", "approval_flows", id, old, after); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan alur.")
		return
	}
	w.WriteHeader(204)
}

func externalApproverKind(kind string) string {
	switch kind {
	case "manager":
		return "reports_to"
	case "user":
		return "membership"
	default:
		return kind
	}
}
func storedApproverKind(kind string) string {
	switch kind {
	case "reports_to":
		return "manager"
	case "membership":
		return "user"
	default:
		return kind
	}
}
