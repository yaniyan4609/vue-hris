package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

var assetCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._-]{1,63}$`)

type assetRecord struct {
	ID         string     `json:"id"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	Category   *string    `json:"category"`
	Status     string     `json:"status"`
	Version    uint64     `json:"version"`
	ArchivedAt *time.Time `json:"archived_at"`
	CreatedBy  *string    `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

const assetColumns = `s.id,s.code,s.name,s.category,s.status,s.version,s.archived_at,s.created_by,s.created_at,s.updated_at`

func scanAsset(row interface{ Scan(...any) error }) (assetRecord, error) {
	var item assetRecord
	var category, actor sql.NullString
	var archived sql.NullTime
	err := row.Scan(&item.ID, &item.Code, &item.Name, &category, &item.Status, &item.Version, &archived, &actor, &item.CreatedAt, &item.UpdatedAt)
	if category.Valid {
		item.Category = &category.String
	}
	if actor.Valid {
		item.CreatedBy = &actor.String
	}
	if archived.Valid {
		item.ArchivedAt = &archived.Time
	}
	return item, err
}
func hasAllAssetScope(c adminContext) bool {
	for _, s := range c.effective.Scopes {
		if s.AllTenant {
			return true
		}
	}
	return false
}
func normalizeAsset(code, name, category *string) bool {
	*code = strings.ToUpper(strings.TrimSpace(*code))
	*name = strings.TrimSpace(*name)
	*category = strings.TrimSpace(*category)
	return assetCodePattern.MatchString(*code) && len([]rune(*name)) >= 3 && len([]rune(*name)) <= 160 && len([]rune(*category)) <= 64
}

func (a adminAPI) listAssets(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "aset.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	where := `s.tenant_id=?`
	args := []any{c.tenant.ID}
	if !hasAllAssetScope(c) {
		scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
		where += ` AND EXISTS (SELECT 1 FROM employee_assets x JOIN employees e ON e.tenant_id=x.tenant_id AND e.id=x.employee_id` + employeeScopeJoin + ` WHERE x.tenant_id=s.tenant_id AND x.asset_id=s.id AND ` + scope + `)`
		args = append(args, scopeArgs...)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		if status != "available" && status != "assigned" && status != "maintenance" && status != "archived" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status tidak valid.")
			return
		}
		where += ` AND s.status=?`
		args = append(args, status)
	}
	if category := strings.TrimSpace(r.URL.Query().Get("category")); category != "" {
		if len([]rune(category)) > 64 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kategori terlalu panjang.")
			return
		}
		where += ` AND s.category=?`
		args = append(args, category)
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		if len([]rune(q)) > 100 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Pencarian terlalu panjang.")
			return
		}
		where += ` AND (s.code LIKE ? OR s.name LIKE ?)`
		args = append(args, "%"+q+"%", "%"+q+"%")
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM assets s WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung aset.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+assetColumns+` FROM assets s WHERE `+where+` ORDER BY s.created_at DESC,s.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aset.")
		return
	}
	defer rows.Close()
	items := []assetRecord{}
	for rows.Next() {
		item, e := scanAsset(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aset.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aset.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) createAsset(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "aset.buat")
	if !ok {
		return
	}
	if !hasAllAssetScope(c) {
		writeAPIError(w, r, 403, "SCOPE_DENIED", "Kelola master aset memerlukan scope seluruh tenant.")
		return
	}
	var in struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		Category string `json:"category"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if !normalizeAsset(&in.Code, &in.Name, &in.Category) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kode, nama, atau kategori tidak valid.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID aset.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat aset.")
		return
	}
	defer tx.Rollback()
	var existing int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM assets WHERE tenant_id=? AND code=?`, c.tenant.ID, in.Code).Scan(&existing); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kode aset.")
		return
	}
	if existing > 0 {
		writeAPIError(w, r, 409, "CODE_EXISTS", "Kode aset sudah dipakai.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO assets (tenant_id,id,code,name,category,status,created_by) VALUES (?,?,?,?,?,'available',?)`, c.tenant.ID, id, in.Code, in.Name, nullableString(in.Category), c.state.User.ID); err != nil {
		writeAPIError(w, r, 409, "CODE_EXISTS", "Kode aset sudah dipakai.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "assets", id, nil, map[string]any{"code": in.Code, "status": "available", "version": 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat aset.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "status": "available", "version": 1}, nil)
}

func (a adminAPI) patchAsset(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "aset.ubah")
	if !ok {
		return
	}
	if !hasAllAssetScope(c) {
		writeAPIError(w, r, 403, "SCOPE_DENIED", "Kelola master aset memerlukan scope seluruh tenant.")
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Aset tidak ditemukan.")
		return
	}
	var in struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		Category string `json:"category"`
		Status   string `json:"status"`
		Version  uint64 `json:"version"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if in.Version == 0 || !normalizeAsset(&in.Code, &in.Name, &in.Category) || (in.Status != "" && in.Status != "available" && in.Status != "maintenance") {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kode, nama, status, atau versi tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah aset.")
		return
	}
	defer tx.Rollback()
	old, err := scanAsset(tx.QueryRowContext(r.Context(), `SELECT `+assetColumns+` FROM assets s WHERE s.tenant_id=? AND s.id=? FOR UPDATE`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Aset tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aset.")
		return
	}
	if old.Status == "archived" {
		writeAPIError(w, r, 409, "ASSET_ARCHIVED", "Aset sudah diarsipkan.")
		return
	}
	if old.Version != in.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Aset sudah berubah.")
		return
	}
	status := old.Status
	if in.Status != "" {
		status = in.Status
	}
	if old.Status == "assigned" && status != "assigned" {
		writeAPIError(w, r, 409, "ASSET_ASSIGNED", "Aset masih dipinjam.")
		return
	}
	if old.Status != "assigned" && status == "assigned" {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status dipinjam hanya melalui penyerahan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE assets SET code=?,name=?,category=?,status=?,version=version+1 WHERE tenant_id=? AND id=? AND version=?`, in.Code, in.Name, nullableString(in.Category), status, c.tenant.ID, id, in.Version); err != nil {
		writeAPIError(w, r, 409, "CODE_EXISTS", "Kode aset sudah dipakai.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "assets", id, map[string]any{"code": old.Code, "status": old.Status, "version": old.Version}, map[string]any{"code": in.Code, "status": status, "version": old.Version + 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah aset.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "status": status, "version": old.Version + 1}, nil)
}

func (a adminAPI) archiveAsset(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "aset.hapus")
	if !ok {
		return
	}
	if !hasAllAssetScope(c) {
		writeAPIError(w, r, 403, "SCOPE_DENIED", "Arsip master aset memerlukan scope seluruh tenant.")
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Aset tidak ditemukan.")
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum mengarsipkan aset.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengarsipkan aset.")
		return
	}
	defer tx.Rollback()
	old, err := scanAsset(tx.QueryRowContext(r.Context(), `SELECT `+assetColumns+` FROM assets s WHERE s.tenant_id=? AND s.id=? FOR UPDATE`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Aset tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aset.")
		return
	}
	if old.Status == "archived" {
		writeAPIError(w, r, 409, "ASSET_ARCHIVED", "Aset sudah diarsipkan.")
		return
	}
	var count int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employee_assets WHERE tenant_id=? AND asset_id=? AND active_flag=1`, c.tenant.ID, id).Scan(&count); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa peminjaman.")
		return
	}
	if count > 0 || old.Status == "assigned" {
		writeAPIError(w, r, 409, "ASSET_ASSIGNED", "Aset masih dipinjam.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE assets SET status='archived',archived_at=UTC_TIMESTAMP(6),version=version+1 WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengarsipkan aset.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "archive", "assets", id, map[string]any{"status": old.Status, "version": old.Version}, map[string]any{"status": "archived", "version": old.Version + 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengarsipkan aset.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
