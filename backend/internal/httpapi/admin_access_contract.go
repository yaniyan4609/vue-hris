package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"unicode"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

// Canonical routes share the existing access transaction handlers until the
// Vue editor has moved to the snake_case contract. Their permission, scope,
// delegation, reauthentication, and audit decisions are identical.
func (a adminAPI) mountAccessContract(r chi.Router, config Options) {
	r.Get("/users", a.accessContractList)
	r.With(checkOrigin(config)).Post("/users", a.accessContractCreateUser)
	r.Get("/users/{id}", a.accessContractDetail)
	r.With(checkOrigin(config)).Patch("/users/{id}", a.accessContractUpdateUser)
	r.With(checkOrigin(config)).Post("/users/{id}/reset-password", a.accessContractSendReset)
	r.Get("/users/{id}/roles", a.accessContractSection("roles", "pengguna.lihat"))
	r.Get("/users/{id}/sessions", a.accessContractUserSessions)
	r.With(checkOrigin(config)).Delete("/users/{id}/sessions", a.accessContractRevokeUserSessions)
	r.With(checkOrigin(config)).Delete("/users/{id}/sessions/{sessionId}", a.accessContractRevokeUserSessions)
	r.With(checkOrigin(config)).Post("/users/{id}/disable", a.accessContractSetUserActive(false))
	r.With(checkOrigin(config)).Post("/users/{id}/enable", a.accessContractSetUserActive(true))
	r.Get("/users/{id}/permission-overrides", a.accessContractSection("overrides", "pengguna.lihat"))
	r.Get("/users/{id}/scopes", a.accessContractSection("scopes", "pengguna.lihat"))
	r.Get("/users/{id}/effective-access", a.accessContractSection("effective", "pengguna.kelola_akses"))
	r.Get("/users/{id}/access-history", a.accessContractSection("history", "audit.lihat"))
	r.With(checkOrigin(config)).Post("/users/{id}/roles", a.accessContractAssignRole)
	r.With(checkOrigin(config)).Patch("/users/{id}/roles/{assignmentId}", a.accessContractUpdateRoleAssignment)
	r.With(checkOrigin(config)).Delete("/users/{id}/roles/{assignmentId}", a.accessContractRevokeRole)
	r.With(checkOrigin(config)).Post("/users/{id}/effective-access/preview", a.accessContractPreview)
	r.With(checkOrigin(config)).Put("/users/{id}/permission-overrides", a.accessContractBulkOverrides)
	r.With(checkOrigin(config)).Put("/users/{id}/scopes/{code}", a.accessContractUpdateScope)
	r.With(checkOrigin(config)).Delete("/users/{id}/scopes/{code}", a.accessContractClearScope)
	r.Get("/roles", a.accessContractRoles)
	r.Get("/roles/{id}", a.accessContractRoleDetail)
	r.With(checkOrigin(config)).Post("/roles", a.accessContractCreateRole)
	r.With(checkOrigin(config)).Patch("/roles/{id}", a.accessContractUpdateRole)
	r.With(checkOrigin(config)).Delete("/roles/{id}", a.accessContractDeleteRole)
	r.With(checkOrigin(config)).Put("/roles/{id}/permissions", a.accessContractRolePermissions)
	r.Get("/roles/{id}/scopes", a.accessContractRoleScopes)
	r.With(checkOrigin(config)).Put("/roles/{id}/scopes", a.accessContractUpdateRoleScopes)
	r.Get("/permissions", a.accessContractPermissions)
}

func (a adminAPI) captureLegacy(r *http.Request, handler http.HandlerFunc) (int, []byte) {
	recorder := httptest.NewRecorder()
	handler(recorder, r)
	return recorder.Code, recorder.Body.Bytes()
}

func legacyContractError(w http.ResponseWriter, r *http.Request, status int, body []byte) {
	var envelope map[string]json.RawMessage
	_ = json.Unmarshal(body, &envelope)
	message := ""
	code := "VALIDATION_FAILED"
	switch status {
	case 401:
		code = "AUTH_SESSION_INVALID"
	case 403:
		code = "PERMISSION_DENIED"
	case 404:
		code = "NOT_FOUND"
	case 409:
		code = "CONFLICT"
	case 500:
		code = "INTERNAL_ERROR"
	}
	if raw := envelope["error"]; len(raw) > 0 {
		if raw[0] == '"' {
			_ = json.Unmarshal(raw, &message)
		} else {
			var structured struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if json.Unmarshal(raw, &structured) == nil {
				if structured.Code != "" {
					code = structured.Code
				}
				message = structured.Message
			}
		}
	}
	var topCode string
	if json.Unmarshal(envelope["code"], &topCode) == nil && topCode != "" {
		code = topCode
	}
	if message == "" {
		message = "Permintaan tidak dapat diproses."
	}
	writeAPIError(w, r, status, code, message)
}

func snakeKey(value string) string {
	var result strings.Builder
	for _, char := range value {
		if unicode.IsUpper(char) {
			result.WriteByte('_')
			result.WriteRune(unicode.ToLower(char))
		} else {
			result.WriteRune(char)
		}
	}
	return result.String()
}

func snakeValue(value any) any {
	switch item := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(item))
		for key, element := range item {
			result[snakeKey(key)] = snakeValue(element)
		}
		return result
	case []any:
		for index := range item {
			item[index] = snakeValue(item[index])
		}
		return item
	default:
		return value
	}
}

func camelKey(value string) string {
	var result strings.Builder
	upper := false
	for _, char := range value {
		if char == '_' {
			upper = true
			continue
		}
		if upper {
			result.WriteRune(unicode.ToUpper(char))
			upper = false
		} else {
			result.WriteRune(char)
		}
	}
	return result.String()
}

func camelValue(value any) any {
	switch item := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(item))
		for key, element := range item {
			result[camelKey(key)] = camelValue(element)
		}
		return result
	case []any:
		for index := range item {
			item[index] = camelValue(item[index])
		}
		return item
	default:
		return value
	}
}

func writeLegacyData(w http.ResponseWriter, r *http.Request, status int, raw []byte, list bool) {
	if status >= 400 {
		legacyContractError(w, r, status, raw)
		return
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca respons akses.")
		return
	}
	if list {
		page, ok := decoded.(map[string]any)
		if !ok {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Format daftar akses tidak sah.")
			return
		}
		writeAPIData(w, status, snakeValue(page["items"]), map[string]any{"page": page["page"], "per_page": page["pageSize"], "total": page["total"]})
		return
	}
	writeAPIData(w, status, snakeValue(decoded), nil)
}

func (a adminAPI) accessContractList(w http.ResponseWriter, r *http.Request) {
	request := r.Clone(r.Context())
	urlCopy := *r.URL
	query := urlCopy.Query()
	if pageSize := query.Get("per_page"); pageSize != "" {
		query.Set("pageSize", pageSize)
	}
	urlCopy.RawQuery = query.Encode()
	request.URL = &urlCopy
	status, raw := a.captureLegacy(request, a.listAccessUsers)
	writeLegacyData(w, r, status, raw, true)
}

func (a adminAPI) accessContractDetail(w http.ResponseWriter, r *http.Request) {
	status, raw := a.captureLegacy(r, a.getAccessUser)
	writeLegacyData(w, r, status, raw, false)
}

func (a adminAPI) accessContractSection(section, permission string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if permission != "pengguna.lihat" {
			recorder := httptest.NewRecorder()
			if _, ok := a.authorize(recorder, r, permission); !ok {
				legacyContractError(w, r, recorder.Code, recorder.Body.Bytes())
				return
			}
		}
		status, raw := a.captureLegacy(r, a.getAccessUser)
		if status >= 400 {
			legacyContractError(w, r, status, raw)
			return
		}
		var detail accessUserDetail
		if err := json.Unmarshal(raw, &detail); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca akses pengguna.")
			return
		}
		switch section {
		case "roles":
			writeAPIData(w, 200, snakeValueJSON(detail.Roles), nil)
		case "history":
			writeAPIData(w, 200, snakeValueJSON(detail.History), nil)
		case "overrides":
			items := []map[string]string{}
			for _, item := range detail.Permissions {
				if item.Override != "inherit" {
					items = append(items, map[string]string{"permission_code": item.Code, "decision": item.Override})
				}
			}
			writeAPIData(w, 200, items, nil)
		case "scopes":
			items, err := a.rawUserScopes(r, detail.User.ID)
			if err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca scope pengguna.")
				return
			}
			writeAPIData(w, 200, items, nil)
		case "effective":
			items := map[string]any{}
			for _, item := range detail.Permissions {
				items[item.Code] = map[string]any{"allowed": item.Allowed, "reason": item.Reason, "sources": item.Sources, "scopes": snakeValueJSON(item.Scopes), "override": item.Override}
			}
			writeAPIData(w, 200, items, nil)
		}
	}
}

func (a adminAPI) rawUserScopes(r *http.Request, userID string) (map[string]any, error) {
	state, err := a.auth.Current(r.Context(), cookieToken(r))
	if err != nil {
		return nil, err
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT o.permission_code,rule.id,rule.kind,v.dimension,v.node_id
		FROM user_scope_overrides o
		LEFT JOIN user_scope_rules rule ON rule.tenant_id=o.tenant_id AND rule.user_id=o.user_id AND rule.permission_code=o.permission_code
		LEFT JOIN user_scope_values v ON v.tenant_id=rule.tenant_id AND v.rule_id=rule.id
		WHERE o.tenant_id=? AND o.user_id=? ORDER BY o.permission_code,rule.id,v.dimension,v.node_id`, state.SelectedTenantID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]any{}
	all := map[string][]access.ScopeRule{}
	seen := map[string]bool{}
	for rows.Next() {
		var code string
		var id, kind, dimension, node sql.NullString
		if err := rows.Scan(&code, &id, &kind, &dimension, &node); err != nil {
			return nil, err
		}
		if _, ok := all[code]; !ok {
			all[code] = []access.ScopeRule{}
		}
		if !id.Valid {
			continue
		}
		if !seen[id.String] {
			rule := access.ScopeRule{}
			switch kind.String {
			case "all_tenant":
				rule.AllTenant = true
			case "self":
				rule.Self = true
			case "direct_reports":
				rule.DirectReports = true
			}
			all[code] = append(all[code], rule)
			seen[id.String] = true
		}
		index := len(all[code]) - 1
		if dimension.Valid && node.Valid {
			switch dimension.String {
			case "region":
				all[code][index].Regions = append(all[code][index].Regions, node.String)
			case "office":
				all[code][index].Offices = append(all[code][index].Offices, node.String)
			case "division":
				all[code][index].Divisions = append(all[code][index].Divisions, node.String)
			case "department":
				all[code][index].Departments = append(all[code][index].Departments, node.String)
			case "position":
				all[code][index].Positions = append(all[code][index].Positions, node.String)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for code, rules := range all {
		result[code] = snakeValueJSON(rules)
	}
	return result, nil
}

func snakeValueJSON(value any) any {
	raw, _ := json.Marshal(value)
	var decoded any
	_ = json.Unmarshal(raw, &decoded)
	return snakeValue(decoded)
}

func (a adminAPI) accessContractRoles(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.contractActor(w, r, "pengguna.lihat")
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT role.id,role.code,role.name,COALESCE(role.description,''),role.is_system,role.version,rp.permission_code FROM roles role LEFT JOIN role_permissions rp ON rp.tenant_id=role.tenant_id AND rp.role_id=role.id WHERE role.tenant_id=? ORDER BY role.name,role.id,rp.permission_code`, actor.tenant.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca role.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, code, name, description string
		var system bool
		var version uint32
		var permission sql.NullString
		if err := rows.Scan(&id, &code, &name, &description, &system, &version, &permission); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca role.")
			return
		}
		if len(items) == 0 || items[len(items)-1]["id"] != id {
			items = append(items, map[string]any{"id": id, "code": code, "name": name, "description": description, "is_system": system, "version": version, "permissions": []string{}})
		}
		if permission.Valid {
			items[len(items)-1]["permissions"] = append(items[len(items)-1]["permissions"].([]string), permission.String)
		}
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca role.")
		return
	}
	writeAPIData(w, 200, items, nil)
}

func (a adminAPI) accessContractAssignRole(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RoleID     string `json:"role_id"`
		ValidFrom  string `json:"valid_from"`
		ValidUntil string `json:"valid_until"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	legacy, _ := json.Marshal(map[string]string{"roleId": body.RoleID, "validFrom": body.ValidFrom, "validUntil": body.ValidUntil})
	request := r.Clone(r.Context())
	request.Body = http.NoBody
	request.Body = ioBody(legacy)
	status, raw := a.captureLegacy(request, a.assignUserRole)
	writeLegacyData(w, r, status, raw, false)
}

func (a adminAPI) accessContractRevokeRole(w http.ResponseWriter, r *http.Request) {
	request := r.Clone(r.Context())
	request.Body = ioBody([]byte(`{}`))
	request.Header = request.Header.Clone()
	request.Header.Set("Content-Type", "application/json")
	status, raw := a.captureLegacy(request, a.revokeUserRole)
	if status >= 400 {
		legacyContractError(w, r, status, raw)
		return
	}
	w.WriteHeader(status)
}

func (a adminAPI) accessContractPreview(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.contractActor(w, r, "pengguna.kelola_akses"); !ok {
		return
	}
	var body struct {
		PermissionCode string          `json:"permission_code"`
		Decision       string          `json:"decision"`
		Rules          json.RawMessage `json:"rules"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	var rules any
	if len(body.Rules) > 0 {
		if err := json.Unmarshal(body.Rules, &rules); err != nil {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Scope tidak valid.")
			return
		}
	}
	legacy, _ := json.Marshal(map[string]any{"userId": chi.URLParam(r, "id"), "permissionCode": body.PermissionCode, "decision": body.Decision, "rules": camelValue(rules)})
	request := r.Clone(r.Context())
	request.Body = ioBody(legacy)
	status, raw := a.captureLegacy(request, a.previewUserAccess)
	writeLegacyData(w, r, status, raw, false)
}

func (a adminAPI) accessContractUpdateScope(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Rules json.RawMessage `json:"rules"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	var rules any
	if err := json.Unmarshal(body.Rules, &rules); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Scope tidak valid.")
		return
	}
	legacy, _ := json.Marshal(map[string]any{"rules": camelValue(rules)})
	request := r.Clone(r.Context())
	request.Body = ioBody(legacy)
	status, raw := a.captureLegacy(request, a.updateUserScope)
	if status >= 400 {
		legacyContractError(w, r, status, raw)
		return
	}
	w.WriteHeader(status)
}

func (a adminAPI) accessContractClearScope(w http.ResponseWriter, r *http.Request) {
	request := r.Clone(r.Context())
	request.Body = ioBody([]byte(`{}`))
	request.Header = request.Header.Clone()
	request.Header.Set("Content-Type", "application/json")
	status, raw := a.captureLegacy(request, a.clearUserScope)
	if status >= 400 {
		legacyContractError(w, r, status, raw)
		return
	}
	w.WriteHeader(status)
}

func ioBody(body []byte) *readCloser { return &readCloser{Reader: bytes.NewReader(body)} }

type readCloser struct{ *bytes.Reader }

func (*readCloser) Close() error { return nil }

func (a adminAPI) contractActor(w http.ResponseWriter, r *http.Request, permission string) (adminContext, bool) {
	recorder := httptest.NewRecorder()
	actor, ok := a.authorize(recorder, r, permission)
	if !ok {
		legacyContractError(w, r, recorder.Code, recorder.Body.Bytes())
	}
	return actor, ok
}

func (a adminAPI) accessContractPermissions(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.contractActor(w, r, "pengguna.kelola_akses")
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT p.code,p.module,p.action,p.is_sensitive,p.requires_scope,COALESCE(p.entitlement_code,''),GROUP_CONCAT(d.dimension ORDER BY d.dimension)
		FROM permission_registry p LEFT JOIN permission_scope_dimensions d ON d.permission_code=p.code
		WHERE p.entitlement_code IS NULL OR EXISTS (SELECT 1 FROM tenant_subscriptions ts JOIN plan_entitlements pe ON pe.plan_id=ts.plan_id WHERE ts.tenant_id=? AND ts.status='active' AND pe.entitlement_code=p.entitlement_code)
		GROUP BY p.code,p.module,p.action,p.is_sensitive,p.requires_scope,p.entitlement_code ORDER BY p.module,p.action`, actor.tenant.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca registry izin.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var code, module, action, entitlement string
		var sensitive, scoped bool
		var dimensions sql.NullString
		if err := rows.Scan(&code, &module, &action, &sensitive, &scoped, &entitlement, &dimensions); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca registry izin.")
			return
		}
		dims := []string{}
		if dimensions.Valid && dimensions.String != "" {
			dims = strings.Split(dimensions.String, ",")
		}
		items = append(items, map[string]any{"code": code, "module": module, "action": action, "is_sensitive": sensitive, "requires_scope": scoped, "entitlement_code": entitlement, "scope_dimensions": dims})
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca registry izin.")
		return
	}
	writeAPIData(w, 200, items, nil)
}
