package httpapi

import (
	"database/sql"
	"errors"
	"net/http"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type scopeChange struct {
	Rules           []access.ScopeRule `json:"rules"`
	CurrentPassword string             `json:"currentPassword"`
}

func (a adminAPI) listScopeOptions(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengguna.lihat")
	if !ok {
		return
	}
	result := map[string][]map[string]string{}
	for dimension, table := range map[string]string{"regions": "regions", "offices": "offices", "divisions": "divisions", "departments": "departments", "positions": "positions"} {
		rows, err := a.db.QueryContext(r.Context(), `SELECT id,name FROM `+table+` WHERE tenant_id=? ORDER BY name`, c.tenant.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca struktur organisasi"})
			return
		}
		items := []map[string]string{}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				writeJSON(w, 500, map[string]string{"error": "Gagal membaca struktur organisasi"})
				return
			}
			items = append(items, map[string]string{"value": id, "label": name})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca struktur organisasi"})
			return
		}
		rows.Close()
		result[dimension] = items
	}
	writeJSON(w, 200, result)
}

func scopeKind(rule access.ScopeRule) (string, map[string][]string, bool) {
	values := map[string][]string{"region": rule.Regions, "office": rule.Offices, "division": rule.Divisions, "department": rule.Departments, "position": rule.Positions}
	count := 0
	if rule.AllTenant {
		count++
	}
	if rule.Self {
		count++
	}
	if rule.DirectReports {
		count++
	}
	filtered := 0
	for _, v := range values {
		if len(v) > 0 {
			filtered++
		}
	}
	if count+filtered == 0 || count > 1 || (count > 0 && filtered > 0) {
		return "", nil, false
	}
	if rule.AllTenant {
		return "all_tenant", values, true
	}
	if rule.Self {
		return "self", values, true
	}
	if rule.DirectReports {
		return "direct_reports", values, true
	}
	return "filter", values, true
}

func (a adminAPI) updateUserScope(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengguna.kelola_akses")
	if !ok {
		return
	}
	var body scopeChange
	if !readJSON(w, r, &body) {
		return
	}
	if len(body.Rules) == 0 || len(body.Rules) > 12 {
		writeJSON(w, 400, map[string]string{"error": "Pilih 1 sampai 12 aturan scope"})
		return
	}
	targetID, code := chi.URLParam(r, "id"), chi.URLParam(r, "code")
	tx, ok := a.beginAccessChange(w, r, c, targetID, body.CurrentPassword)
	if !ok {
		return
	}
	defer tx.Rollback()
	var scoped bool
	if err := tx.QueryRowContext(r.Context(), `SELECT requires_scope FROM permission_registry WHERE code=?`, code).Scan(&scoped); errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Izin tidak ditemukan"})
		return
	} else if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca izin"})
		return
	}
	if !scoped {
		writeJSON(w, 400, map[string]string{"error": "Izin ini tidak memerlukan scope"})
		return
	}
	_, actor, err := (access.Store{DB: tx}).Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, code)
	if err != nil || !actor.Allowed {
		writeJSON(w, 403, map[string]string{"error": "Izin melebihi hak delegasi"})
		return
	}
	actorAll := false
	for _, rule := range actor.Scopes {
		actorAll = actorAll || rule.AllTenant
	}
	if !actorAll {
		writeJSON(w, 403, map[string]string{"error": "Pengaturan scope memerlukan kewenangan seluruh tenant"})
		return
	}
	allowedDimensions := map[string]bool{}
	rows, err := tx.QueryContext(r.Context(), `SELECT dimension FROM permission_scope_dimensions WHERE permission_code=?`, code)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca dimensi scope"})
		return
	}
	for rows.Next() {
		var dimension string
		if err := rows.Scan(&dimension); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca dimensi scope"})
			return
		}
		allowedDimensions[dimension] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca dimensi scope"})
		return
	}
	rows.Close()
	for _, rule := range body.Rules {
		kind, values, valid := scopeKind(rule)
		if !valid || !allowedDimensions[kind] && kind != "filter" {
			writeJSON(w, 400, map[string]string{"error": "Aturan scope tidak valid"})
			return
		}
		count := 0
		for dimension, nodes := range values {
			if len(nodes) == 0 {
				continue
			}
			if !allowedDimensions[dimension] {
				writeJSON(w, 400, map[string]string{"error": "Dimensi scope tidak diizinkan"})
				return
			}
			count += len(nodes)
			for _, node := range nodes {
				if len(node) != 26 {
					writeJSON(w, 400, map[string]string{"error": "Node organisasi tidak valid"})
					return
				}
				var exists int
				table := map[string]string{"region": "regions", "office": "offices", "division": "divisions", "department": "departments", "position": "positions"}[dimension]
				if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM `+table+` WHERE tenant_id=? AND id=?`, c.tenant.ID, node).Scan(&exists); err != nil || exists != 1 {
					writeJSON(w, 400, map[string]string{"error": "Node organisasi tidak ada pada tenant ini"})
					return
				}
			}
		}
		if count > 40 {
			writeJSON(w, 400, map[string]string{"error": "Terlalu banyak node scope"})
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE v FROM user_scope_values v JOIN user_scope_rules rule ON rule.tenant_id=v.tenant_id AND rule.id=v.rule_id WHERE rule.tenant_id=? AND rule.user_id=? AND rule.permission_code=?`, c.tenant.ID, targetID, code); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan scope"})
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM user_scope_rules WHERE tenant_id=? AND user_id=? AND permission_code=?`, c.tenant.ID, targetID, code); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan scope"})
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO user_scope_overrides (tenant_id,user_id,permission_code) VALUES (?,?,?) ON DUPLICATE KEY UPDATE updated_at=UTC_TIMESTAMP(6)`, c.tenant.ID, targetID, code); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan scope"})
		return
	}
	for _, rule := range body.Rules {
		kind, values, _ := scopeKind(rule)
		ruleID, err := ids.New()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan scope"})
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO user_scope_rules (tenant_id,id,user_id,permission_code,kind) VALUES (?,?,?,?,?)`, c.tenant.ID, ruleID, targetID, code, kind); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan scope"})
			return
		}
		for dimension, nodes := range values {
			for _, node := range nodes {
				if _, err = tx.ExecContext(r.Context(), `INSERT INTO user_scope_values (tenant_id,rule_id,dimension,node_id) VALUES (?,?,?,?)`, c.tenant.ID, ruleID, dimension, node); err != nil {
					writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan scope"})
					return
				}
			}
		}
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "pengguna.scope_ubah", "user_scope_overrides", targetID, nil, map[string]any{"permission": code, "rules": body.Rules}); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE tenant_memberships SET access_version=access_version+1 WHERE tenant_id=? AND user_id=?`, c.tenant.ID, targetID); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memperbarui akses"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan scope"})
		return
	}
	w.WriteHeader(204)
}

func (a adminAPI) clearUserScope(w http.ResponseWriter, r *http.Request) {
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
	targetID, code := chi.URLParam(r, "id"), chi.URLParam(r, "code")
	tx, ok := a.beginAccessChange(w, r, c, targetID, body.CurrentPassword)
	if !ok {
		return
	}
	defer tx.Rollback()
	_, actor, err := (access.Store{DB: tx}).Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, code)
	if err != nil || !actor.Allowed {
		writeJSON(w, 403, map[string]string{"error": "Izin melebihi hak delegasi"})
		return
	}
	actorAll := false
	for _, rule := range actor.Scopes {
		actorAll = actorAll || rule.AllTenant
	}
	if !actorAll {
		writeJSON(w, 403, map[string]string{"error": "Pemulihan scope memerlukan kewenangan seluruh tenant"})
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE v FROM user_scope_values v JOIN user_scope_rules rule ON rule.tenant_id=v.tenant_id AND rule.id=v.rule_id WHERE rule.tenant_id=? AND rule.user_id=? AND rule.permission_code=?`, c.tenant.ID, targetID, code); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menghapus scope"})
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM user_scope_rules WHERE tenant_id=? AND user_id=? AND permission_code=?`, c.tenant.ID, targetID, code); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menghapus scope"})
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM user_scope_overrides WHERE tenant_id=? AND user_id=? AND permission_code=?`, c.tenant.ID, targetID, code); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menghapus scope"})
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "pengguna.scope_pulihkan", "user_scope_overrides", targetID, nil, map[string]string{"permission": code}); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE tenant_memberships SET access_version=access_version+1 WHERE tenant_id=? AND user_id=?`, c.tenant.ID, targetID); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memperbarui akses"})
		return
	}
	if err := tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan scope"})
		return
	}
	w.WriteHeader(204)
}

func (a adminAPI) previewUserAccess(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengguna.lihat")
	if !ok {
		return
	}
	var body struct {
		UserID         string              `json:"userId"`
		PermissionCode string              `json:"permissionCode"`
		Decision       string              `json:"decision"`
		Rules          *[]access.ScopeRule `json:"rules"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Decision != "inherit" && body.Decision != "allow" && body.Decision != "deny" {
		writeJSON(w, 400, map[string]string{"error": "Pilihan izin tidak valid"})
		return
	}
	var active int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id=? AND user_id=? AND status='active'`, c.tenant.ID, body.UserID).Scan(&active); err != nil || active != 1 {
		writeJSON(w, 404, map[string]string{"error": "Pengguna tidak ditemukan"})
		return
	}
	subject, _, err := a.access.Decide(r.Context(), c.tenant.ID, body.UserID, c.tenant.Timezone, body.PermissionCode)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menghitung akses"})
		return
	}
	var p access.Permission
	if err := a.db.QueryRowContext(r.Context(), `SELECT code,COALESCE(entitlement_code,''),requires_scope FROM permission_registry WHERE code=?`, body.PermissionCode).Scan(&p.Code, &p.Entitlement, &p.RequiresScope); err != nil {
		writeJSON(w, 404, map[string]string{"error": "Izin tidak ditemukan"})
		return
	}
	override := subject.Overrides[body.PermissionCode]
	override.Decision = access.DecisionKind(body.Decision)
	if body.Rules != nil {
		override.Scopes = body.Rules
	}
	subject.Overrides[body.PermissionCode] = override
	decision := access.EvaluateNow(c.tenant.Timezone, c.tenant.ID, subject, p)
	writeJSON(w, http.StatusOK, accessEntry{Allowed: decision.Allowed, Reason: decision.Reason, Sources: decision.Sources, Scopes: decision.Scopes})
}
