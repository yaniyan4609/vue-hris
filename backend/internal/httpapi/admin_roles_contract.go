package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

var roleCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,63}$`)

type roleRecord struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	System      bool   `json:"is_system"`
	Version     uint32 `json:"version"`
}

func (a adminAPI) roleActor(w http.ResponseWriter, r *http.Request) (adminContext, bool) {
	actor, ok := a.contractActor(w, r, "pengguna.kelola_akses")
	if !ok {
		return adminContext{}, false
	}
	valid, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !valid {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang kata sandi sebelum mengelola role.")
		return adminContext{}, false
	}
	return actor, true
}

func (a adminAPI) lockedRole(w http.ResponseWriter, r *http.Request, tx *sql.Tx, actor adminContext) (roleRecord, bool) {
	var role roleRecord
	if len(chi.URLParam(r, "id")) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Role tidak ditemukan.")
		return role, false
	}
	err := tx.QueryRowContext(r.Context(), `SELECT id,code,name,COALESCE(description,''),is_system,version FROM roles WHERE tenant_id=? AND id=? FOR UPDATE`, actor.tenant.ID, chi.URLParam(r, "id")).Scan(&role.ID, &role.Code, &role.Name, &role.Description, &role.System, &role.Version)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Role tidak ditemukan.")
		return role, false
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca role.")
		return role, false
	}
	if role.System {
		writeAPIError(w, r, 403, "SYSTEM_ROLE_IMMUTABLE", "Role bawaan tidak dapat diubah.")
		return role, false
	}
	var self int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM user_roles WHERE tenant_id=? AND role_id=? AND user_id=? AND revoked_at IS NULL`, actor.tenant.ID, role.ID, actor.state.User.ID).Scan(&self); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa role Admin.")
		return role, false
	}
	if self > 0 {
		writeAPIError(w, r, 403, "SELF_ESCALATION", "Role yang digunakan sendiri tidak dapat diubah.")
		return role, false
	}
	return role, true
}

func checkRoleVersion(w http.ResponseWriter, r *http.Request, actual, supplied uint32) bool {
	if supplied == 0 {
		writeAPIError(w, r, 400, "VERSION_REQUIRED", "Versi role wajib diisi.")
		return false
	}
	if actual != supplied {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Role telah berubah. Muat ulang data sebelum menyimpan.")
		return false
	}
	return true
}

func (a adminAPI) accessContractCreateRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.roleActor(w, r)
	if !ok {
		return
	}
	var body struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Code = strings.TrimSpace(body.Code)
	body.Name = strings.TrimSpace(body.Name)
	body.Description = strings.TrimSpace(body.Description)
	if !roleCodePattern.MatchString(body.Code) || body.Name == "" || len(body.Name) > 120 || len(body.Description) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kode, nama, atau deskripsi role tidak valid.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat role.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat role.")
		return
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM roles WHERE tenant_id=? AND code=?`, actor.tenant.ID, body.Code).Scan(&exists); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa role.")
		return
	}
	if exists > 0 {
		writeAPIError(w, r, 409, "CONFLICT", "Kode role sudah digunakan.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO roles (tenant_id,id,code,name,description) VALUES (?,?,?,?,?)`, actor.tenant.ID, id, body.Code, body.Name, body.Description); err != nil {
		writeAPIError(w, r, 409, "CONFLICT", "Role tidak dapat dibuat.")
		return
	}
	role := roleRecord{ID: id, Code: body.Code, Name: body.Name, Description: body.Description, Version: 1}
	if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.role_buat", "roles", id, nil, role); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat role.")
		return
	}
	writeAPIData(w, 201, role, nil)
}

func (a adminAPI) accessContractUpdateRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.roleActor(w, r)
	if !ok {
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Version     uint32 `json:"version"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.Description = strings.TrimSpace(body.Description)
	if body.Name == "" || len(body.Name) > 120 || len(body.Description) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Nama atau deskripsi role tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah role.")
		return
	}
	defer tx.Rollback()
	role, ok := a.lockedRole(w, r, tx, actor)
	if !ok || !checkRoleVersion(w, r, role.Version, body.Version) {
		return
	}
	previous := role
	role.Name, role.Description, role.Version = body.Name, body.Description, role.Version+1
	if _, err := tx.ExecContext(r.Context(), `UPDATE roles SET name=?,description=?,version=? WHERE tenant_id=? AND id=?`, role.Name, role.Description, role.Version, actor.tenant.ID, role.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah role.")
		return
	}
	if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.role_ubah", "roles", role.ID, previous, role); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah role.")
		return
	}
	writeAPIData(w, 200, role, nil)
}

func (a adminAPI) accessContractDeleteRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.roleActor(w, r)
	if !ok {
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus role.")
		return
	}
	defer tx.Rollback()
	role, ok := a.lockedRole(w, r, tx, actor)
	if !ok {
		return
	}
	var assignments int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM user_roles WHERE tenant_id=? AND role_id=?`, actor.tenant.ID, role.ID).Scan(&assignments); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa role.")
		return
	}
	if assignments > 0 {
		writeAPIError(w, r, 409, "ROLE_IN_USE", "Role memiliki riwayat penugasan dan tidak dapat dihapus.")
		return
	}
	for _, query := range []string{
		`DELETE v FROM role_scope_values v JOIN role_scope_rules rule ON rule.tenant_id=v.tenant_id AND rule.id=v.rule_id WHERE rule.tenant_id=? AND rule.role_id=?`,
		`DELETE FROM role_scope_rules WHERE tenant_id=? AND role_id=?`,
		`DELETE FROM role_permissions WHERE tenant_id=? AND role_id=?`,
		`DELETE FROM roles WHERE tenant_id=? AND id=?`,
	} {
		if _, err := tx.ExecContext(r.Context(), query, actor.tenant.ID, role.ID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus role.")
			return
		}
	}
	if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.role_hapus", "roles", role.ID, role, nil); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus role.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a adminAPI) accessContractRolePermissions(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.roleActor(w, r)
	if !ok {
		return
	}
	var body struct {
		Version     uint32   `json:"version"`
		Permissions []string `json:"permissions"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if len(body.Permissions) > 200 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Terlalu banyak izin role.")
		return
	}
	desired := map[string]bool{}
	for _, code := range body.Permissions {
		if code == "" || desired[code] {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Izin role tidak valid atau berulang.")
			return
		}
		desired[code] = true
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah izin role.")
		return
	}
	defer tx.Rollback()
	role, ok := a.lockedRole(w, r, tx, actor)
	if !ok || !checkRoleVersion(w, r, role.Version, body.Version) {
		return
	}
	current := map[string]bool{}
	rows, err := tx.QueryContext(r.Context(), `SELECT permission_code FROM role_permissions WHERE tenant_id=? AND role_id=?`, actor.tenant.ID, role.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin role.")
		return
	}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			rows.Close()
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin role.")
			return
		}
		current[code] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin role.")
		return
	}
	rows.Close()
	union := map[string]bool{}
	for code := range current {
		union[code] = true
	}
	for code := range desired {
		union[code] = true
	}
	store := access.Store{DB: tx}
	for code := range union {
		var scoped bool
		err := tx.QueryRowContext(r.Context(), `SELECT requires_scope FROM permission_registry WHERE code=?`, code).Scan(&scoped)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Izin tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin.")
			return
		}
		_, decision, err := store.Decide(r.Context(), actor.tenant.ID, actor.state.User.ID, actor.tenant.Timezone, code)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa delegasi.")
			return
		}
		if !decision.Allowed {
			writeAPIError(w, r, 403, "DELEGATION_EXCEEDED", "Izin melebihi hak delegasi Admin.")
			return
		}
		if scoped {
			all := false
			for _, rule := range decision.Scopes {
				all = all || rule.AllTenant
			}
			if !all {
				writeAPIError(w, r, 403, "DELEGATION_EXCEEDED", "Scope melebihi hak delegasi Admin.")
				return
			}
		}
	}
	for code := range current {
		if !desired[code] {
			if _, err := tx.ExecContext(r.Context(), `DELETE v FROM role_scope_values v JOIN role_scope_rules rule ON rule.tenant_id=v.tenant_id AND rule.id=v.rule_id WHERE rule.tenant_id=? AND rule.role_id=? AND rule.permission_code=?`, actor.tenant.ID, role.ID, code); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus scope role.")
				return
			}
			if _, err := tx.ExecContext(r.Context(), `DELETE FROM role_scope_rules WHERE tenant_id=? AND role_id=? AND permission_code=?`, actor.tenant.ID, role.ID, code); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus scope role.")
				return
			}
			if _, err := tx.ExecContext(r.Context(), `DELETE FROM role_permissions WHERE tenant_id=? AND role_id=? AND permission_code=?`, actor.tenant.ID, role.ID, code); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus izin role.")
				return
			}
		}
	}
	for code := range desired {
		if !current[code] {
			if _, err := tx.ExecContext(r.Context(), `INSERT INTO role_permissions (tenant_id,role_id,permission_code) VALUES (?,?,?)`, actor.tenant.ID, role.ID, code); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menambah izin role.")
				return
			}
		}
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE roles SET version=version+1 WHERE tenant_id=? AND id=?`, actor.tenant.ID, role.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui role.")
		return
	}
	if err := a.bumpRoleMembers(r, tx, actor.tenant.ID, role.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui akses anggota.")
		return
	}
	if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.role_izin", "roles", role.ID, current, desired); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah izin role.")
		return
	}
	writeAPIData(w, 200, map[string]any{"version": role.Version + 1, "permissions": body.Permissions}, nil)
}

func (a adminAPI) bumpRoleMembers(r *http.Request, tx *sql.Tx, tenantID, roleID string) error {
	_, err := tx.ExecContext(r.Context(), `UPDATE tenant_memberships tm SET access_version=access_version+1 WHERE tm.tenant_id=? AND EXISTS (SELECT 1 FROM user_roles ur WHERE ur.tenant_id=tm.tenant_id AND ur.user_id=tm.user_id AND ur.role_id=? AND ur.revoked_at IS NULL)`, tenantID, roleID)
	return err
}
