package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) roleSnapshot(w http.ResponseWriter, r *http.Request, permission string) (adminContext, roleRecord, []string, map[string][]access.ScopeRule, bool) {
	actor, ok := a.contractActor(w, r, permission)
	if !ok {
		return adminContext{}, roleRecord{}, nil, nil, false
	}
	var role roleRecord
	err := a.db.QueryRowContext(r.Context(), `SELECT id,code,name,COALESCE(description,''),is_system,version FROM roles WHERE tenant_id=? AND id=?`, actor.tenant.ID, chi.URLParam(r, "id")).Scan(&role.ID, &role.Code, &role.Name, &role.Description, &role.System, &role.Version)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Role tidak ditemukan.")
		return actor, role, nil, nil, false
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca role.")
		return actor, role, nil, nil, false
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT permission_code FROM role_permissions WHERE tenant_id=? AND role_id=? ORDER BY permission_code`, actor.tenant.ID, role.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin role.")
		return actor, role, nil, nil, false
	}
	permissions := []string{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			rows.Close()
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin role.")
			return actor, role, nil, nil, false
		}
		permissions = append(permissions, code)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin role.")
		return actor, role, nil, nil, false
	}
	rows.Close()
	scopes, err := a.rawRoleScopes(r, actor.tenant.ID, role.ID, permissions)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca scope role.")
		return actor, role, nil, nil, false
	}
	return actor, role, permissions, scopes, true
}

func (a adminAPI) rawRoleScopes(r *http.Request, tenantID, roleID string, permissions []string) (map[string][]access.ScopeRule, error) {
	result := map[string][]access.ScopeRule{}
	for _, code := range permissions {
		result[code] = []access.ScopeRule{}
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT rule.permission_code,rule.id,rule.kind,v.dimension,v.node_id FROM role_scope_rules rule LEFT JOIN role_scope_values v ON v.tenant_id=rule.tenant_id AND v.rule_id=rule.id WHERE rule.tenant_id=? AND rule.role_id=? ORDER BY rule.permission_code,rule.id,v.dimension,v.node_id`, tenantID, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var code, id, kind string
		var dimension, node sql.NullString
		if err := rows.Scan(&code, &id, &kind, &dimension, &node); err != nil {
			return nil, err
		}
		if !seen[id] {
			rule := access.ScopeRule{}
			switch kind {
			case "all_tenant":
				rule.AllTenant = true
			case "self":
				rule.Self = true
			case "direct_reports":
				rule.DirectReports = true
			}
			result[code] = append(result[code], rule)
			seen[id] = true
		}
		index := len(result[code]) - 1
		if dimension.Valid && node.Valid {
			switch dimension.String {
			case "region":
				result[code][index].Regions = append(result[code][index].Regions, node.String)
			case "office":
				result[code][index].Offices = append(result[code][index].Offices, node.String)
			case "division":
				result[code][index].Divisions = append(result[code][index].Divisions, node.String)
			case "department":
				result[code][index].Departments = append(result[code][index].Departments, node.String)
			case "position":
				result[code][index].Positions = append(result[code][index].Positions, node.String)
			}
		}
	}
	return result, rows.Err()
}

func (a adminAPI) accessContractRoleDetail(w http.ResponseWriter, r *http.Request) {
	_, role, permissions, scopes, ok := a.roleSnapshot(w, r, "pengguna.lihat")
	if !ok {
		return
	}
	writeAPIData(w, 200, map[string]any{"id": role.ID, "code": role.Code, "name": role.Name, "description": role.Description, "is_system": role.System, "version": role.Version, "permissions": permissions, "scopes": snakeValueJSON(scopes)}, nil)
}

func (a adminAPI) accessContractRoleScopes(w http.ResponseWriter, r *http.Request) {
	_, role, _, scopes, ok := a.roleSnapshot(w, r, "pengguna.kelola_akses")
	if !ok {
		return
	}
	writeAPIData(w, 200, map[string]any{"version": role.Version, "scopes": snakeValueJSON(scopes)}, nil)
}

func (a adminAPI) accessContractUpdateRoleScopes(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.roleActor(w, r)
	if !ok {
		return
	}
	var body struct {
		Version uint32          `json:"version"`
		Scopes  json.RawMessage `json:"scopes"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	var raw map[string]any
	if err := json.Unmarshal(body.Scopes, &raw); err != nil || raw == nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Peta scope tidak valid.")
		return
	}
	encoded, _ := json.Marshal(camelValue(raw))
	var proposed map[string][]access.ScopeRule
	if err := json.Unmarshal(encoded, &proposed); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Peta scope tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah scope role.")
		return
	}
	defer tx.Rollback()
	role, ok := a.lockedRole(w, r, tx, actor)
	if !ok || !checkRoleVersion(w, r, role.Version, body.Version) {
		return
	}
	permissions := map[string]bool{}
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
		permissions[code] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin role.")
		return
	}
	rows.Close()
	store := access.Store{DB: tx}
	for code := range permissions {
		var scoped bool
		if err := tx.QueryRowContext(r.Context(), `SELECT requires_scope FROM permission_registry WHERE code=?`, code).Scan(&scoped); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin role.")
			return
		}
		_, decision, err := store.Decide(r.Context(), actor.tenant.ID, actor.state.User.ID, actor.tenant.Timezone, code)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa delegasi.")
			return
		}
		if !decision.Allowed {
			writeAPIError(w, r, 403, "DELEGATION_EXCEEDED", "Izin role melebihi hak delegasi Admin.")
			return
		}
		if scoped {
			all := false
			for _, rule := range decision.Scopes {
				all = all || rule.AllTenant
			}
			if !all {
				writeAPIError(w, r, 403, "DELEGATION_EXCEEDED", "Scope role melebihi hak delegasi Admin.")
				return
			}
		}
	}
	for code, rules := range proposed {
		if !permissions[code] || len(rules) > 12 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Scope role tidak sesuai izin.")
			return
		}
		var scoped bool
		if err := tx.QueryRowContext(r.Context(), `SELECT requires_scope FROM permission_registry WHERE code=?`, code).Scan(&scoped); err != nil || !scoped {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Izin tidak memerlukan scope.")
			return
		}
		_, decision, err := store.Decide(r.Context(), actor.tenant.ID, actor.state.User.ID, actor.tenant.Timezone, code)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa delegasi.")
			return
		}
		all := false
		for _, rule := range decision.Scopes {
			all = all || rule.AllTenant
		}
		if !decision.Allowed || !all {
			writeAPIError(w, r, 403, "DELEGATION_EXCEEDED", "Scope melebihi hak delegasi Admin.")
			return
		}
		allowed := map[string]bool{}
		dimRows, err := tx.QueryContext(r.Context(), `SELECT dimension FROM permission_scope_dimensions WHERE permission_code=?`, code)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca dimensi scope.")
			return
		}
		for dimRows.Next() {
			var dim string
			if err := dimRows.Scan(&dim); err != nil {
				dimRows.Close()
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca dimensi scope.")
				return
			}
			allowed[dim] = true
		}
		if err := dimRows.Err(); err != nil {
			dimRows.Close()
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca dimensi scope.")
			return
		}
		dimRows.Close()
		for _, rule := range rules {
			kind, values, valid := scopeKind(rule)
			if !valid || kind != "filter" && !allowed[kind] {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Aturan scope tidak valid.")
				return
			}
			count := 0
			for dimension, nodes := range values {
				if len(nodes) == 0 {
					continue
				}
				if !allowed[dimension] {
					writeAPIError(w, r, 400, "VALIDATION_FAILED", "Dimensi scope tidak diizinkan.")
					return
				}
				count += len(nodes)
				for _, node := range nodes {
					if len(node) != 26 {
						writeAPIError(w, r, 400, "VALIDATION_FAILED", "Node scope tidak valid.")
						return
					}
					table := map[string]string{"region": "regions", "office": "offices", "division": "divisions", "department": "departments", "position": "positions"}[dimension]
					var exists int
					if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM `+table+` WHERE tenant_id=? AND id=?`, actor.tenant.ID, node).Scan(&exists); err != nil || exists != 1 {
						writeAPIError(w, r, 400, "VALIDATION_FAILED", "Node scope tidak ada pada tenant.")
						return
					}
				}
			}
			if count > 40 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Terlalu banyak node scope.")
				return
			}
		}
	}
	previous, err := a.rawRoleScopes(r, actor.tenant.ID, role.ID, nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca scope sebelumnya.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE v FROM role_scope_values v JOIN role_scope_rules rule ON rule.tenant_id=v.tenant_id AND rule.id=v.rule_id WHERE rule.tenant_id=? AND rule.role_id=?`, actor.tenant.ID, role.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus scope role.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM role_scope_rules WHERE tenant_id=? AND role_id=?`, actor.tenant.ID, role.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus scope role.")
		return
	}
	for code, rules := range proposed {
		for _, rule := range rules {
			kind, values, _ := scopeKind(rule)
			ruleID, err := ids.New()
			if err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat aturan scope.")
				return
			}
			if _, err := tx.ExecContext(r.Context(), `INSERT INTO role_scope_rules (tenant_id,id,role_id,permission_code,kind) VALUES (?,?,?,?,?)`, actor.tenant.ID, ruleID, role.ID, code, kind); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan scope role.")
				return
			}
			for dimension, nodes := range values {
				for _, node := range nodes {
					if _, err := tx.ExecContext(r.Context(), `INSERT INTO role_scope_values (tenant_id,rule_id,dimension,node_id) VALUES (?,?,?,?)`, actor.tenant.ID, ruleID, dimension, node); err != nil {
						writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan node scope.")
						return
					}
				}
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
	if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.role_scope", "roles", role.ID, previous, proposed); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah scope role.")
		return
	}
	writeAPIData(w, 200, map[string]any{"version": role.Version + 1, "scopes": snakeValueJSON(proposed)}, nil)
}
