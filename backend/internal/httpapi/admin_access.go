package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type accessUser struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Status        string `json:"status"`
	AccessVersion uint64 `json:"accessVersion"`
	RoleCount     int    `json:"roleCount"`
}

type userRole struct {
	AssignmentID string  `json:"assignmentId"`
	RoleID       string  `json:"roleId"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	ValidFrom    string  `json:"validFrom"`
	ValidUntil   *string `json:"validUntil,omitempty"`
	RevokedAt    *string `json:"revokedAt,omitempty"`
}

type roleOption struct {
	ID          string   `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type userPermission struct {
	Code          string             `json:"code"`
	RequiresScope bool               `json:"requiresScope"`
	Allowed       bool               `json:"allowed"`
	Reason        string             `json:"reason"`
	Override      string             `json:"override"`
	Sources       []string           `json:"sources,omitempty"`
	Scopes        []access.ScopeRule `json:"scopes,omitempty"`
	ScopeOverride bool               `json:"scopeOverride"`
}

type accessUserDetail struct {
	User        accessUser       `json:"user"`
	Roles       []userRole       `json:"roles"`
	Permissions []userPermission `json:"permissions"`
	History     []accessHistory  `json:"history"`
}

type accessHistory struct {
	Action string          `json:"action"`
	Actor  string          `json:"actor"`
	At     string          `json:"at"`
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}

func (a adminAPI) listAccessUsers(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengguna.lihat")
	if !ok {
		return
	}
	page, ok := readPage(w, r)
	if !ok {
		return
	}
	q, ok := searchTerm(w, r)
	if !ok {
		return
	}
	where := "tm.tenant_id=?"
	args := []any{c.tenant.ID}
	location, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		location = time.UTC
	}
	roleDate := time.Now().In(location).Format("2006-01-02")
	if q != "" {
		where += " AND (LOCATE(LOWER(?),LOWER(u.display_name))>0 OR LOCATE(LOWER(?),LOWER(u.email))>0)"
		args = append(args, q, q)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		if status != "active" && status != "disabled" {
			writeJSON(w, 400, map[string]string{"error": "Filter status tidak valid"})
			return
		}
		if status == "active" {
			where += " AND tm.status='active' AND u.status='active'"
		} else {
			where += " AND (tm.status='inactive' OR u.status='disabled')"
		}
	}
	if roleID := r.URL.Query().Get("role_id"); roleID != "" {
		if len(roleID) != 26 {
			writeJSON(w, 400, map[string]string{"error": "Role tidak valid"})
			return
		}
		where += ` AND EXISTS (SELECT 1 FROM user_roles ur WHERE ur.tenant_id=tm.tenant_id AND ur.user_id=tm.user_id AND ur.role_id=? AND ur.revoked_at IS NULL AND ur.valid_from<=? AND (ur.valid_until IS NULL OR ur.valid_until>=?))`
		args = append(args, roleID, roleDate, roleDate)
	}
	if officeID := r.URL.Query().Get("office_id"); officeID != "" {
		if len(officeID) != 26 {
			writeJSON(w, 400, map[string]string{"error": "Kantor tidak valid"})
			return
		}
		where += ` AND EXISTS (SELECT 1 FROM employees e WHERE e.tenant_id=tm.tenant_id AND e.user_id=tm.user_id AND e.base_office_id=?)`
		args = append(args, officeID)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenant_memberships tm JOIN users u ON u.id=tm.user_id WHERE `+where, args...).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca pengguna"})
		return
	}
	query := `SELECT u.id,u.display_name,u.email,CASE WHEN tm.status='active' AND u.status='active' THEN 'active' ELSE 'disabled' END,tm.access_version,
		(SELECT COUNT(*) FROM user_roles ur WHERE ur.tenant_id=tm.tenant_id AND ur.user_id=tm.user_id AND ur.revoked_at IS NULL AND ur.valid_from<=? AND (ur.valid_until IS NULL OR ur.valid_until>=?))
		FROM tenant_memberships tm JOIN users u ON u.id=tm.user_id WHERE ` + where + ` ORDER BY u.display_name,u.id LIMIT ? OFFSET ?`
	listArgs := append([]any{roleDate, roleDate}, args...)
	rows, err := a.db.QueryContext(r.Context(), query, append(listArgs, page.PageSize, page.Offset)...)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca pengguna"})
		return
	}
	defer rows.Close()
	items := []accessUser{}
	for rows.Next() {
		var item accessUser
		if err := rows.Scan(&item.ID, &item.Name, &item.Email, &item.Status, &item.AccessVersion, &item.RoleCount); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca pengguna"})
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca pengguna"})
		return
	}
	writeJSON(w, 200, pageResult[accessUser]{Items: items, Total: total, Page: page.Page, PageSize: page.PageSize})
}

func (a adminAPI) listAccessRoles(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengguna.lihat")
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT r.id,r.code,r.name,rp.permission_code FROM roles r LEFT JOIN role_permissions rp ON rp.tenant_id=r.tenant_id AND rp.role_id=r.id WHERE r.tenant_id=? ORDER BY r.name,rp.permission_code`, c.tenant.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca role"})
		return
	}
	defer rows.Close()
	items := []roleOption{}
	for rows.Next() {
		var id, code, name string
		var permission sql.NullString
		if err := rows.Scan(&id, &code, &name, &permission); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca role"})
			return
		}
		if len(items) == 0 || items[len(items)-1].ID != id {
			items = append(items, roleOption{ID: id, Code: code, Name: name, Permissions: []string{}})
		}
		if permission.Valid {
			items[len(items)-1].Permissions = append(items[len(items)-1].Permissions, permission.String)
		}
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca role"})
		return
	}
	writeJSON(w, 200, items)
}

func (a adminAPI) getAccessUser(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengguna.lihat")
	if !ok {
		return
	}
	userID := chi.URLParam(r, "id")
	var detail accessUserDetail
	err := a.db.QueryRowContext(r.Context(), `SELECT u.id,u.display_name,u.email,CASE WHEN tm.status='active' AND u.status='active' THEN 'active' ELSE 'disabled' END,tm.access_version FROM tenant_memberships tm JOIN users u ON u.id=tm.user_id WHERE tm.tenant_id=? AND tm.user_id=?`, c.tenant.ID, userID).Scan(&detail.User.ID, &detail.User.Name, &detail.User.Email, &detail.User.Status, &detail.User.AccessVersion)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Pengguna tidak ditemukan"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca pengguna"})
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT ur.id,r.id,r.code,r.name,DATE_FORMAT(ur.valid_from,'%Y-%m-%d'),DATE_FORMAT(ur.valid_until,'%Y-%m-%d'),ur.revoked_at FROM user_roles ur JOIN roles r ON r.tenant_id=ur.tenant_id AND r.id=ur.role_id WHERE ur.tenant_id=? AND ur.user_id=? ORDER BY ur.assigned_at DESC`, c.tenant.ID, userID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca role pengguna"})
		return
	}
	detail.Roles = []userRole{}
	for rows.Next() {
		var role userRole
		var until sql.NullString
		var revoked sql.NullTime
		if err := rows.Scan(&role.AssignmentID, &role.RoleID, &role.Code, &role.Name, &role.ValidFrom, &until, &revoked); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca role pengguna"})
			return
		}
		if until.Valid {
			role.ValidUntil = &until.String
		}
		if revoked.Valid {
			value := revoked.Time.Format(time.RFC3339)
			role.RevokedAt = &value
		}
		detail.Roles = append(detail.Roles, role)
	}
	if rows.Err() != nil {
		rows.Close()
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca role pengguna"})
		return
	}
	rows.Close()
	rows, err = a.db.QueryContext(r.Context(), `SELECT p.code,p.requires_scope,COALESCE(o.decision,'inherit'),s.user_id IS NOT NULL FROM permission_registry p LEFT JOIN user_permission_overrides o ON o.tenant_id=? AND o.user_id=? AND o.permission_code=p.code LEFT JOIN user_scope_overrides s ON s.tenant_id=? AND s.user_id=? AND s.permission_code=p.code ORDER BY p.code`, c.tenant.ID, userID, c.tenant.ID, userID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca izin pengguna"})
		return
	}
	type overrideItem struct {
		code, decision string
		requiresScope  bool
		scopeOverride  bool
	}
	var overrides []overrideItem
	for rows.Next() {
		var item overrideItem
		if err := rows.Scan(&item.code, &item.requiresScope, &item.decision, &item.scopeOverride); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca izin pengguna"})
			return
		}
		overrides = append(overrides, item)
	}
	if rows.Err() != nil {
		rows.Close()
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca izin pengguna"})
		return
	}
	rows.Close()
	detail.Permissions = []userPermission{}
	decisions, _, err := a.access.DecideAll(r.Context(), c.tenant.ID, userID, c.tenant.Timezone)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca izin pengguna"})
		return
	}
	for _, item := range overrides {
		decision := decisions[item.code]
		detail.Permissions = append(detail.Permissions, userPermission{Code: item.code, RequiresScope: item.requiresScope, Allowed: decision.Allowed, Reason: decision.Reason, Override: item.decision, Sources: decision.Sources, Scopes: decision.Scopes, ScopeOverride: item.scopeOverride})
	}
	detail.History = []accessHistory{}
	rows, err = a.db.QueryContext(r.Context(), `SELECT a.action,u.display_name,DATE_FORMAT(a.created_at,'%Y-%m-%d %H:%i:%s'),a.before_json,a.after_json FROM audit_logs a JOIN users u ON u.id=a.actor_user_id WHERE a.tenant_id=? AND a.entity_id=? AND a.action LIKE 'pengguna.%' ORDER BY a.created_at DESC LIMIT 30`, c.tenant.ID, userID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca riwayat akses"})
		return
	}
	for rows.Next() {
		var item accessHistory
		var before, after sql.NullString
		if err := rows.Scan(&item.Action, &item.Actor, &item.At, &before, &after); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca riwayat akses"})
			return
		}
		if before.Valid {
			item.Before = json.RawMessage(before.String)
		}
		if after.Valid {
			item.After = json.RawMessage(after.String)
		}
		detail.History = append(detail.History, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca riwayat akses"})
		return
	}
	rows.Close()
	writeJSON(w, 200, detail)
}

func reauthenticate(ctx context.Context, tx *sql.Tx, actorID, password string) error {
	if password == "" || len(password) > 1024 {
		return auth.ErrInvalidCredentials
	}
	var encoded sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=? AND status='active' FOR UPDATE`, actorID).Scan(&encoded); err != nil {
		return err
	}
	if !encoded.Valid || !auth.VerifyPassword(encoded.String, password) {
		return auth.ErrInvalidCredentials
	}
	return nil
}

func (a adminAPI) recordRejectedAccess(ctx context.Context, c adminContext, targetID, operation, reason string) {
	if len(targetID) != 26 {
		return
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("Gagal membuka audit akses ditolak", "error", err)
		return
	}
	defer tx.Rollback()
	if err := audit(ctx, tx, c.tenant.ID, c.state.User.ID, "pengguna.akses_ditolak", "users", targetID, nil, map[string]string{"operation": operation, "reason": reason}); err != nil {
		slog.Error("Gagal mencatat akses ditolak", "error", err)
		return
	}
	if err := tx.Commit(); err != nil {
		slog.Error("Gagal menyimpan audit akses ditolak", "error", err)
	}
}

func (a adminAPI) beginAccessChange(w http.ResponseWriter, r *http.Request, c adminContext, targetID, password string) (*sql.Tx, bool) {
	if targetID == c.state.User.ID {
		a.recordRejectedAccess(r.Context(), c, targetID, "ubah_hak_akses", "self_change")
		writeJSON(w, 403, map[string]string{"error": "Hak akses diri sendiri tidak dapat diubah"})
		return nil, false
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memulai perubahan akses"})
		return nil, false
	}
	var actorStatus, targetStatus string
	if err := tx.QueryRowContext(r.Context(), `SELECT status FROM tenant_memberships WHERE tenant_id=? AND user_id=? FOR UPDATE`, c.tenant.ID, c.state.User.ID).Scan(&actorStatus); err != nil || actorStatus != "active" {
		tx.Rollback()
		writeJSON(w, 403, map[string]string{"error": "Akses Admin tidak tersedia"})
		return nil, false
	}
	if err := tx.QueryRowContext(r.Context(), `SELECT status FROM tenant_memberships WHERE tenant_id=? AND user_id=? FOR UPDATE`, c.tenant.ID, targetID).Scan(&targetStatus); err != nil || targetStatus != "active" {
		tx.Rollback()
		writeJSON(w, 404, map[string]string{"error": "Pengguna tidak ditemukan"})
		return nil, false
	}
	for _, code := range []string{"panel.admin", "pengguna.kelola_akses"} {
		_, decision, err := (access.Store{DB: tx}).Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, code)
		if err != nil || !decision.Allowed {
			tx.Rollback()
			writeJSON(w, 403, map[string]string{"error": "Akses mengelola pengguna tidak tersedia"})
			return nil, false
		}
	}
	if password == "" {
		valid, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
		if err != nil || !valid {
			tx.Rollback()
			writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang kata sandi sebelum mengubah hak akses.")
			return nil, false
		}
	} else if err := reauthenticate(r.Context(), tx, c.state.User.ID, password); err != nil {
		tx.Rollback()
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeJSON(w, 403, map[string]string{"error": "Kata sandi Admin tidak benar"})
		} else {
			writeJSON(w, 500, map[string]string{"error": "Gagal memverifikasi Admin"})
		}
		return nil, false
	}
	return tx, true
}

func (a adminAPI) roleDelegable(ctx context.Context, tx *sql.Tx, c adminContext, roleID string) (bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT rp.permission_code,p.requires_scope FROM role_permissions rp JOIN permission_registry p ON p.code=rp.permission_code WHERE rp.tenant_id=? AND rp.role_id=?`, c.tenant.ID, roleID)
	if err != nil {
		return false, err
	}
	type item struct {
		code   string
		scoped bool
	}
	var permissions []item
	for rows.Next() {
		var p item
		if err := rows.Scan(&p.code, &p.scoped); err != nil {
			rows.Close()
			return false, err
		}
		permissions = append(permissions, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	store := access.Store{DB: tx}
	for _, p := range permissions {
		_, decision, err := store.Decide(ctx, c.tenant.ID, c.state.User.ID, c.tenant.Timezone, p.code)
		if err != nil {
			return false, err
		}
		if !decision.Allowed {
			return false, nil
		}
		if p.scoped {
			all := false
			for _, scope := range decision.Scopes {
				all = all || scope.AllTenant
			}
			if !all {
				return false, nil
			}
		}
	}
	return true, nil
}

func (a adminAPI) assignUserRole(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengguna.kelola_akses")
	if !ok {
		return
	}
	var body struct {
		RoleID          string `json:"roleId"`
		ValidFrom       string `json:"validFrom"`
		ValidUntil      string `json:"validUntil"`
		CurrentPassword string `json:"currentPassword"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if len(body.RoleID) != 26 || len(body.CurrentPassword) > 1024 {
		writeJSON(w, 400, map[string]string{"error": "Role atau kata sandi tidak valid"})
		return
	}
	if body.ValidFrom == "" {
		location, _ := time.LoadLocation(c.tenant.Timezone)
		if location == nil {
			location = time.UTC
		}
		body.ValidFrom = time.Now().In(location).Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", body.ValidFrom); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Tanggal mulai tidak valid"})
		return
	}
	if body.ValidUntil != "" {
		if _, err := time.Parse("2006-01-02", body.ValidUntil); err != nil || body.ValidUntil < body.ValidFrom {
			writeJSON(w, 400, map[string]string{"error": "Tanggal akhir tidak valid"})
			return
		}
	}
	targetID := chi.URLParam(r, "id")
	tx, ok := a.beginAccessChange(w, r, c, targetID, body.CurrentPassword)
	if !ok {
		return
	}
	defer tx.Rollback()
	var roleCode string
	if err := tx.QueryRowContext(r.Context(), `SELECT code FROM roles WHERE tenant_id=? AND id=?`, c.tenant.ID, body.RoleID).Scan(&roleCode); errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Role tidak ditemukan"})
		return
	} else if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca role"})
		return
	}
	allowed, err := a.roleDelegable(r.Context(), tx, c, body.RoleID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memeriksa delegasi"})
		return
	}
	if !allowed {
		tx.Rollback()
		a.recordRejectedAccess(r.Context(), c, targetID, "beri_role", "delegation_exceeded")
		writeJSON(w, 403, map[string]string{"error": "Role melebihi hak delegasi Admin"})
		return
	}
	until := body.ValidUntil
	if until == "" {
		until = "9999-12-31"
	}
	var overlaps int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM user_roles WHERE tenant_id=? AND user_id=? AND role_id=? AND revoked_at IS NULL AND valid_from<=? AND (valid_until IS NULL OR valid_until>=?)`, c.tenant.ID, targetID, body.RoleID, until, body.ValidFrom).Scan(&overlaps); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memeriksa role"})
		return
	}
	if overlaps > 0 {
		writeJSON(w, 409, map[string]string{"error": "Role sudah aktif pada periode tersebut"})
		return
	}
	assignmentID, err := ids.New()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membuat penugasan"})
		return
	}
	var validUntil any
	if body.ValidUntil != "" {
		validUntil = body.ValidUntil
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO user_roles (tenant_id,id,user_id,role_id,valid_from,valid_until,assigned_by) VALUES (?,?,?,?,?,?,?)`, c.tenant.ID, assignmentID, targetID, body.RoleID, body.ValidFrom, validUntil, c.state.User.ID); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan role"})
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "pengguna.role_beri", "user_roles", assignmentID, nil, map[string]any{"userId": targetID, "roleCode": roleCode, "validFrom": body.ValidFrom, "validUntil": body.ValidUntil}); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE tenant_memberships SET access_version=access_version+1 WHERE tenant_id=? AND user_id=?`, c.tenant.ID, targetID); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memperbarui versi akses"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan perubahan akses"})
		return
	}
	writeJSON(w, 201, map[string]string{"assignmentId": assignmentID})
}

func (a adminAPI) revokeUserRole(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengguna.kelola_akses")
	if !ok {
		return
	}
	var body struct {
		CurrentPassword string `json:"currentPassword"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	targetID := chi.URLParam(r, "id")
	tx, ok := a.beginAccessChange(w, r, c, targetID, body.CurrentPassword)
	if !ok {
		return
	}
	defer tx.Rollback()
	assignmentID := chi.URLParam(r, "assignmentId")
	var roleCode string
	err := tx.QueryRowContext(r.Context(), `SELECT ro.code FROM user_roles ur JOIN roles ro ON ro.tenant_id=ur.tenant_id AND ro.id=ur.role_id WHERE ur.tenant_id=? AND ur.user_id=? AND ur.id=? AND ur.revoked_at IS NULL FOR UPDATE`, c.tenant.ID, targetID, assignmentID).Scan(&roleCode)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Penugasan role tidak ditemukan"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca penugasan role"})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE user_roles SET revoked_at=UTC_TIMESTAMP(6),revoked_by=? WHERE tenant_id=? AND user_id=? AND id=? AND revoked_at IS NULL`, c.state.User.ID, c.tenant.ID, targetID, assignmentID); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencabut role"})
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "pengguna.role_cabut", "user_roles", assignmentID, map[string]any{"userId": targetID, "roleCode": roleCode}, nil); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE tenant_memberships SET access_version=access_version+1 WHERE tenant_id=? AND user_id=?`, c.tenant.ID, targetID); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memperbarui versi akses"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan perubahan akses"})
		return
	}
	w.WriteHeader(204)
}

func (a adminAPI) updateUserPermission(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengguna.kelola_akses")
	if !ok {
		return
	}
	var body struct {
		Decision        string `json:"decision"`
		CurrentPassword string `json:"currentPassword"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Decision != "inherit" && body.Decision != "allow" && body.Decision != "deny" {
		writeJSON(w, 400, map[string]string{"error": "Pilihan izin tidak valid"})
		return
	}
	targetID, code := chi.URLParam(r, "id"), chi.URLParam(r, "code")
	tx, ok := a.beginAccessChange(w, r, c, targetID, body.CurrentPassword)
	if !ok {
		return
	}
	defer tx.Rollback()
	var requiresScope bool
	err := tx.QueryRowContext(r.Context(), `SELECT requires_scope FROM permission_registry WHERE code=?`, code).Scan(&requiresScope)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Izin tidak ditemukan"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca izin"})
		return
	}
	_, actorDecision, err := (access.Store{DB: tx}).Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, code)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memeriksa delegasi"})
		return
	}
	if !actorDecision.Allowed {
		tx.Rollback()
		a.recordRejectedAccess(r.Context(), c, targetID, "ubah_izin", "permission_not_owned")
		writeJSON(w, 403, map[string]string{"error": "Izin melebihi hak delegasi Admin"})
		return
	}
	if requiresScope {
		allTenant := false
		for _, scope := range actorDecision.Scopes {
			allTenant = allTenant || scope.AllTenant
		}
		if !allTenant {
			tx.Rollback()
			a.recordRejectedAccess(r.Context(), c, targetID, "ubah_izin", "scope_exceeded")
			writeJSON(w, 403, map[string]string{"error": "Scope melebihi hak delegasi Admin"})
			return
		}
		if body.Decision == "allow" {
			var scopeCount int
			if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM user_scope_rules WHERE tenant_id=? AND user_id=? AND permission_code=?`, c.tenant.ID, targetID, code).Scan(&scopeCount); err != nil || scopeCount == 0 {
				writeJSON(w, 400, map[string]string{"error": "Atur scope pengguna sebelum memberi izin khusus"})
				return
			}
		}
	}
	var previous sql.NullString
	err = tx.QueryRowContext(r.Context(), `SELECT decision FROM user_permission_overrides WHERE tenant_id=? AND user_id=? AND permission_code=? FOR UPDATE`, c.tenant.ID, targetID, code).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca izin pengguna"})
		return
	}
	before := "inherit"
	if previous.Valid {
		before = previous.String
	}
	if before == body.Decision {
		w.WriteHeader(204)
		return
	}
	if body.Decision == "inherit" {
		var scopeOverride int
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM user_scope_overrides WHERE tenant_id=? AND user_id=? AND permission_code=?`, c.tenant.ID, targetID, code).Scan(&scopeOverride); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal memeriksa scope"})
			return
		}
		if scopeOverride > 0 {
			writeJSON(w, 409, map[string]string{"error": "Scope khusus masih ada. Hapus melalui pengaturan scope terlebih dahulu"})
			return
		}
		_, err = tx.ExecContext(r.Context(), `DELETE FROM user_permission_overrides WHERE tenant_id=? AND user_id=? AND permission_code=?`, c.tenant.ID, targetID, code)
	} else {
		_, err = tx.ExecContext(r.Context(), `INSERT INTO user_permission_overrides (tenant_id,user_id,permission_code,decision) VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE decision=VALUES(decision)`, c.tenant.ID, targetID, code, body.Decision)
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan izin pengguna"})
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "pengguna.izin_ubah", "user_permission_overrides", targetID, map[string]string{"permission": code, "decision": before}, map[string]string{"permission": code, "decision": body.Decision}); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE tenant_memberships SET access_version=access_version+1 WHERE tenant_id=? AND user_id=?`, c.tenant.ID, targetID); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memperbarui versi akses"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan perubahan akses"})
		return
	}
	w.WriteHeader(204)
}
