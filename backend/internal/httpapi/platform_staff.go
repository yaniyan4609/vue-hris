package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type platformActor struct {
	state      auth.State
	superadmin bool
}

func authorizePlatform(w http.ResponseWriter, r *http.Request, db *sql.DB, permission string) (platformActor, bool) {
	state, err := (auth.Service{DB: db}).Current(r.Context(), cookieToken(r))
	if err != nil {
		writeAuthError(w, err)
		return platformActor{}, false
	}
	if !state.Platform {
		writeJSON(w, 403, map[string]string{"error": "Panel platform tidak tersedia"})
		return platformActor{}, false
	}
	grants, superadmin, err := platformAccess(r.Context(), db, state.User.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memeriksa akses"})
		return platformActor{}, false
	}
	if !grants[permission].Allowed {
		writeJSON(w, 403, map[string]string{"error": "Akses platform tidak diizinkan"})
		return platformActor{}, false
	}
	if permission == "platform.pengguna.kelola" && !superadmin {
		writeJSON(w, 403, map[string]string{"error": "Hanya Superadmin dapat mengelola staf platform"})
		return platformActor{}, false
	}
	return platformActor{state: state, superadmin: superadmin}, true
}

func listPlatformStaff(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if _, ok := authorizePlatform(w, r, db, "platform.pengguna.lihat"); !ok {
		return
	}
	q, ok := searchTerm(w, r)
	if !ok {
		return
	}
	page, ok := readPage(w, r)
	if !ok {
		return
	}
	where := `EXISTS (SELECT 1 FROM platform_user_roles pur WHERE pur.user_id=u.id)`
	args := []any{}
	if q != "" {
		where += ` AND (LOCATE(LOWER(?),LOWER(u.display_name))>0 OR LOCATE(LOWER(?),LOWER(u.email))>0)`
		args = append(args, q, q)
	}
	status := r.URL.Query().Get("status")
	if status != "" {
		if status != "active" && status != "disabled" {
			writeJSON(w, 400, map[string]string{"error": "Filter status tidak valid"})
			return
		}
		where += ` AND u.status=?`
		args = append(args, status)
	}
	var total int
	if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users u WHERE `+where, args...).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca staf"})
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT u.id,u.display_name,u.email,u.status,(SELECT COUNT(*) FROM platform_user_roles p WHERE p.user_id=u.id AND p.valid_from<=CURRENT_DATE() AND (p.valid_until IS NULL OR p.valid_until>=CURRENT_DATE())) FROM users u WHERE `+where+` ORDER BY u.display_name,u.id LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca staf"})
		return
	}
	defer rows.Close()
	type item struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		Status    string `json:"status"`
		RoleCount int    `json:"roleCount"`
	}
	items := []item{}
	for rows.Next() {
		var x item
		if err := rows.Scan(&x.ID, &x.Name, &x.Email, &x.Status, &x.RoleCount); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca staf"})
			return
		}
		items = append(items, x)
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca staf"})
		return
	}
	writeJSON(w, 200, pageResult[item]{Items: items, Total: total, Page: page.Page, PageSize: page.PageSize})
}

func listPlatformRoles(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if _, ok := authorizePlatform(w, r, db, "platform.pengguna.lihat"); !ok {
		return
	}
	rows, err := db.QueryContext(r.Context(), `SELECT code,name FROM platform_roles ORDER BY name`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca role"})
		return
	}
	defer rows.Close()
	items := []map[string]string{}
	for rows.Next() {
		var code, name string
		if err := rows.Scan(&code, &name); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca role"})
			return
		}
		items = append(items, map[string]string{"code": code, "name": name})
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca role"})
		return
	}
	writeJSON(w, 200, items)
}

func getPlatformStaff(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if _, ok := authorizePlatform(w, r, db, "platform.pengguna.lihat"); !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var user struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := db.QueryRowContext(r.Context(), `SELECT id,display_name,email FROM users WHERE id=? AND EXISTS (SELECT 1 FROM platform_user_roles WHERE user_id=users.id)`, id).Scan(&user.ID, &user.Name, &user.Email); err != nil {
		writeJSON(w, 404, map[string]string{"error": "Staf tidak ditemukan"})
		return
	}
	roles := []map[string]string{}
	rows, err := db.QueryContext(r.Context(), `SELECT pr.code,pr.name,DATE_FORMAT(pur.valid_from,'%Y-%m-%d'),COALESCE(DATE_FORMAT(pur.valid_until,'%Y-%m-%d'),'') FROM platform_user_roles pur JOIN platform_roles pr ON pr.id=pur.role_id WHERE pur.user_id=? ORDER BY pur.valid_from DESC`, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca role staf"})
		return
	}
	for rows.Next() {
		var code, name, from, until string
		if err := rows.Scan(&code, &name, &from, &until); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca role staf"})
			return
		}
		roles = append(roles, map[string]string{"code": code, "name": name, "validFrom": from, "validUntil": until})
	}
	if rows.Err() != nil {
		rows.Close()
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca role staf"})
		return
	}
	rows.Close()
	tenantIDs := []string{}
	rows, err = db.QueryContext(r.Context(), `SELECT tenant_id FROM platform_user_tenants WHERE user_id=? ORDER BY tenant_id`, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca penugasan tenant"})
		return
	}
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca penugasan tenant"})
			return
		}
		tenantIDs = append(tenantIDs, tenantID)
	}
	if rows.Err() != nil {
		rows.Close()
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca penugasan tenant"})
		return
	}
	rows.Close()
	grants, superadmin, err := platformAccess(r.Context(), db, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca izin staf"})
		return
	}
	writeJSON(w, 200, map[string]any{"user": user, "roles": roles, "tenantIds": tenantIDs, "permissions": grants, "superadmin": superadmin})
}

func beginPlatformChange(w http.ResponseWriter, r *http.Request, db *sql.DB, actor platformActor, targetID, password string) (*sql.Tx, bool) {
	if targetID == actor.state.User.ID {
		writeJSON(w, 403, map[string]string{"error": "Hak akses sendiri tidak dapat diubah"})
		return nil, false
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memulai perubahan"})
		return nil, false
	}
	var status string
	if err := tx.QueryRowContext(r.Context(), `SELECT status FROM users WHERE id=? FOR UPDATE`, targetID).Scan(&status); err != nil || status != "active" {
		tx.Rollback()
		writeJSON(w, 404, map[string]string{"error": "Pengguna aktif tidak ditemukan"})
		return nil, false
	}
	if err := reauthenticate(r.Context(), tx, actor.state.User.ID, password); err != nil {
		tx.Rollback()
		writeJSON(w, 403, map[string]string{"error": "Kata sandi Superadmin tidak benar"})
		return nil, false
	}
	return tx, true
}

func platformAudit(r *http.Request, tx *sql.Tx, actorID, targetID, action string, detail any) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO platform_audit_logs (id,actor_user_id,target_user_id,action,detail_json) VALUES (?,?,?,?,?)`, id, actorID, targetID, action, encoded)
	return err
}

func assignPlatformRole(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	actor, ok := authorizePlatform(w, r, db, "platform.pengguna.kelola")
	if !ok {
		return
	}
	var body struct {
		Email           string `json:"email"`
		RoleCode        string `json:"roleCode"`
		CurrentPassword string `json:"currentPassword"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	var targetID, roleID string
	if err := db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE email_normalized=LOWER(TRIM(?)) AND status='active'`, body.Email).Scan(&targetID); err != nil {
		writeJSON(w, 404, map[string]string{"error": "Akun tidak ditemukan"})
		return
	}
	if err := db.QueryRowContext(r.Context(), `SELECT id FROM platform_roles WHERE code=?`, body.RoleCode).Scan(&roleID); err != nil {
		writeJSON(w, 404, map[string]string{"error": "Role tidak ditemukan"})
		return
	}
	tx, ok := beginPlatformChange(w, r, db, actor, targetID, body.CurrentPassword)
	if !ok {
		return
	}
	defer tx.Rollback()
	date := time.Now().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02")
	if _, err := tx.ExecContext(r.Context(), `INSERT IGNORE INTO platform_user_roles (user_id,role_id,valid_from) VALUES (?,?,?)`, targetID, roleID, date); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memberi role"})
		return
	}
	if err := platformAudit(r, tx, actor.state.User.ID, targetID, "platform.role_beri", map[string]string{"role": body.RoleCode}); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan role"})
		return
	}
	w.WriteHeader(204)
}

func revokePlatformRole(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	actor, ok := authorizePlatform(w, r, db, "platform.pengguna.kelola")
	if !ok {
		return
	}
	var body struct {
		CurrentPassword string `json:"currentPassword"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	id, code := chi.URLParam(r, "id"), chi.URLParam(r, "code")
	if code == "SUPERADMIN" {
		writeJSON(w, 403, map[string]string{"error": "Role Superadmin tidak dapat dicabut melalui halaman ini"})
		return
	}
	tx, ok := beginPlatformChange(w, r, db, actor, id, body.CurrentPassword)
	if !ok {
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `DELETE pur FROM platform_user_roles pur JOIN platform_roles pr ON pr.id=pur.role_id WHERE pur.user_id=? AND pr.code=?`, id, code)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencabut role"})
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeJSON(w, 404, map[string]string{"error": "Role tidak ditemukan"})
		return
	}
	if err := platformAudit(r, tx, actor.state.User.ID, id, "platform.role_cabut", map[string]string{"role": code}); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan role"})
		return
	}
	w.WriteHeader(204)
}

func updatePlatformOverride(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	actor, ok := authorizePlatform(w, r, db, "platform.pengguna.kelola")
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
	id, code := chi.URLParam(r, "id"), chi.URLParam(r, "code")
	tx, ok := beginPlatformChange(w, r, db, actor, id, body.CurrentPassword)
	if !ok {
		return
	}
	defer tx.Rollback()
	var known int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM platform_permission_registry WHERE code=?`, code).Scan(&known); err != nil || known != 1 {
		writeJSON(w, 404, map[string]string{"error": "Izin tidak ditemukan"})
		return
	}
	if body.Decision == "inherit" {
		_, err := tx.ExecContext(r.Context(), `DELETE FROM platform_user_permission_overrides WHERE user_id=? AND permission_code=?`, id, code)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan izin"})
			return
		}
	} else {
		_, err := tx.ExecContext(r.Context(), `INSERT INTO platform_user_permission_overrides (user_id,permission_code,decision) VALUES (?,?,?) ON DUPLICATE KEY UPDATE decision=VALUES(decision)`, id, code, body.Decision)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan izin"})
			return
		}
	}
	if err := platformAudit(r, tx, actor.state.User.ID, id, "platform.izin_ubah", map[string]string{"permission": code, "decision": body.Decision}); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan izin"})
		return
	}
	w.WriteHeader(204)
}

func updatePlatformTenants(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	actor, ok := authorizePlatform(w, r, db, "platform.pengguna.kelola")
	if !ok {
		return
	}
	var body struct {
		TenantIDs       []string `json:"tenantIds"`
		CurrentPassword string   `json:"currentPassword"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if len(body.TenantIDs) > 200 {
		writeJSON(w, 400, map[string]string{"error": "Terlalu banyak tenant"})
		return
	}
	id := chi.URLParam(r, "id")
	tx, ok := beginPlatformChange(w, r, db, actor, id, body.CurrentPassword)
	if !ok {
		return
	}
	defer tx.Rollback()
	var superadmin int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM platform_user_roles pur JOIN platform_roles pr ON pr.id=pur.role_id WHERE pur.user_id=? AND pr.code='SUPERADMIN' AND pur.valid_from<=CURRENT_DATE() AND (pur.valid_until IS NULL OR pur.valid_until>=CURRENT_DATE())`, id).Scan(&superadmin); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memeriksa role"})
		return
	}
	if superadmin > 0 {
		writeJSON(w, 400, map[string]string{"error": "Superadmin memiliki akses seluruh tenant"})
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM platform_user_tenants WHERE user_id=?`, id); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan tenant"})
		return
	}
	seen := map[string]bool{}
	for _, tenantID := range body.TenantIDs {
		if seen[tenantID] {
			continue
		}
		seen[tenantID] = true
		if len(tenantID) != 26 {
			writeJSON(w, 400, map[string]string{"error": "ID tenant tidak valid"})
			return
		}
		var exists int
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenants WHERE id=?`, tenantID).Scan(&exists); err != nil || exists != 1 {
			writeJSON(w, 400, map[string]string{"error": "Tenant tidak ditemukan"})
			return
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO platform_user_tenants (user_id,tenant_id) VALUES (?,?)`, id, tenantID); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan tenant"})
			return
		}
	}
	if err := platformAudit(r, tx, actor.state.User.ID, id, "platform.tenant_ubah", map[string]any{"tenantIds": body.TenantIDs}); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan tenant"})
		return
	}
	w.WriteHeader(204)
}
