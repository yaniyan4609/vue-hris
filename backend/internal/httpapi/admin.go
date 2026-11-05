package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type adminAPI struct {
	db     *sql.DB
	auth   auth.Service
	access access.Store
}
type adminContext struct {
	state     auth.State
	tenant    auth.Tenant
	subject   access.Subject
	effective access.Effective
}

func mountAdmin(r chi.Router, db *sql.DB, config Options) {
	a := adminAPI{db: db, auth: auth.Service{DB: db}, access: access.Store{DB: db}}
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Get("/capabilities", a.capabilities)
		r.Get("/organization/{kind}", a.listOrganization)
		r.With(checkOrigin(config)).Post("/organization/{kind}", a.createOrganization)
		r.With(checkOrigin(config)).Patch("/organization/{kind}/{id}", a.updateOrganization)
		r.Get("/employees", a.listEmployees)
		r.Get("/employees/{id}", a.getEmployee)
		r.With(checkOrigin(config)).Post("/employees", a.createEmployee)
		r.With(checkOrigin(config)).Patch("/employees/{id}", a.updateEmployee)
		r.Get("/access/users", a.listAccessUsers)
		r.Get("/access/users/{id}", a.getAccessUser)
		r.Get("/access/roles", a.listAccessRoles)
		r.Get("/access/scope-options", a.listScopeOptions)
		r.With(checkOrigin(config)).Post("/access/users/{id}/roles", a.assignUserRole)
		r.With(checkOrigin(config)).Delete("/access/users/{id}/roles/{assignmentId}", a.revokeUserRole)
		r.With(checkOrigin(config)).Put("/access/users/{id}/permissions/{code}", a.updateUserPermission)
		r.With(checkOrigin(config)).Put("/access/users/{id}/scopes/{code}", a.updateUserScope)
		r.With(checkOrigin(config)).Delete("/access/users/{id}/scopes/{code}", a.clearUserScope)
		r.With(checkOrigin(config)).Post("/access/preview", a.previewUserAccess)
	})
}

func (a adminAPI) authorize(w http.ResponseWriter, r *http.Request, code string) (adminContext, bool) {
	state, err := a.auth.Current(r.Context(), cookieToken(r))
	if err != nil {
		writeAuthError(w, err)
		return adminContext{}, false
	}
	var tenant auth.Tenant
	for _, item := range state.Tenants {
		if item.ID == state.SelectedTenantID {
			tenant = item
			break
		}
	}
	if tenant.ID == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Pilih perusahaan terlebih dahulu"})
		return adminContext{}, false
	}
	if !tenant.Admin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Panel Admin tidak tersedia"})
		return adminContext{}, false
	}
	subject, decision, err := a.access.Decide(r.Context(), tenant.ID, state.User.ID, tenant.Timezone, code)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Terjadi kesalahan server"})
		return adminContext{}, false
	}
	if !decision.Allowed {
		writeJSON(w, 403, map[string]string{"error": "Akses tidak diizinkan"})
		return adminContext{}, false
	}
	return adminContext{state: state, tenant: tenant, subject: subject, effective: decision}, true
}

func (a adminAPI) capabilities(w http.ResponseWriter, r *http.Request) {
	state, err := a.auth.Current(r.Context(), cookieToken(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var tenant auth.Tenant
	for _, item := range state.Tenants {
		if item.ID == state.SelectedTenantID {
			tenant = item
			break
		}
	}
	if tenant.ID == "" || !tenant.Admin {
		writeJSON(w, 403, map[string]string{"error": "Panel Admin tidak tersedia"})
		return
	}
	result := map[string]bool{}
	for _, code := range []string{"organisasi.lihat", "organisasi.buat", "organisasi.ubah", "karyawan.lihat", "karyawan.buat", "karyawan.ubah"} {
		_, decision, err := a.access.Decide(r.Context(), tenant.ID, state.User.ID, tenant.Timezone, code)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca hak akses"})
			return
		}
		result[code] = decision.Allowed
	}
	writeJSON(w, 200, result)
}

type orgKind struct {
	table, parentColumn, parentJSON string
	hasTimezone                     bool
}

var orgKinds = map[string]orgKind{
	"regions":     {table: "regions"},
	"offices":     {table: "offices", parentColumn: "region_id", parentJSON: "parentId", hasTimezone: true},
	"divisions":   {table: "divisions"},
	"departments": {table: "departments", parentColumn: "division_id", parentJSON: "parentId"},
	"positions":   {table: "positions", parentColumn: "department_id", parentJSON: "parentId"},
}
var nodeCode = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,47}$`)

type orgNode struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	ParentID string `json:"parentId,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

func orgConfig(w http.ResponseWriter, r *http.Request) (orgKind, bool) {
	kind, ok := orgKinds[chi.URLParam(r, "kind")]
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "Jenis organisasi tidak ditemukan"})
	}
	return kind, ok
}

func (a adminAPI) listOrganization(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "organisasi.lihat")
	if !ok {
		return
	}
	kind, ok := orgConfig(w, r)
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
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "name"
	}
	if sort != "name" && sort != "code" {
		writeJSON(w, 400, map[string]string{"error": "Urutan tidak valid"})
		return
	}
	direction := strings.ToLower(r.URL.Query().Get("order"))
	if direction == "" {
		direction = "asc"
	}
	if direction != "asc" && direction != "desc" {
		writeJSON(w, 400, map[string]string{"error": "Arah urutan tidak valid"})
		return
	}
	where := "tenant_id=?"
	args := []any{c.tenant.ID}
	if q != "" {
		where += " AND (LOCATE(LOWER(?), LOWER(code))>0 OR LOCATE(LOWER(?), LOWER(name))>0)"
		args = append(args, q, q)
	}
	if parentID := r.URL.Query().Get("parentId"); parentID != "" {
		if kind.parentColumn == "" || len(parentID) != 26 {
			writeJSON(w, 400, map[string]string{"error": "Filter induk tidak valid"})
			return
		}
		where += " AND " + kind.parentColumn + "=?"
		args = append(args, parentID)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM "+kind.table+" WHERE "+where, args...).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca jumlah data"})
		return
	}
	columns := "id, code, name"
	if kind.parentColumn != "" {
		columns += ", " + kind.parentColumn
	}
	if kind.hasTimezone {
		columns += ", timezone"
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT "+columns+" FROM "+kind.table+" WHERE "+where+" ORDER BY "+sort+" "+direction+", id ASC LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca data"})
		return
	}
	defer rows.Close()
	nodes := []orgNode{}
	for rows.Next() {
		var n orgNode
		dest := []any{&n.ID, &n.Code, &n.Name}
		if kind.parentColumn != "" {
			dest = append(dest, &n.ParentID)
		}
		if kind.hasTimezone {
			dest = append(dest, &n.Timezone)
		}
		if err := rows.Scan(dest...); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca data"})
			return
		}
		nodes = append(nodes, n)
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca data"})
		return
	}
	writeJSON(w, 200, pageResult[orgNode]{Items: nodes, Total: total, Page: page.Page, PageSize: page.PageSize})
}

func validateNode(n orgNode, kind orgKind) bool {
	if !nodeCode.MatchString(n.Code) || strings.TrimSpace(n.Name) == "" || len([]rune(n.Name)) > 160 {
		return false
	}
	if kind.parentColumn != "" && len(n.ParentID) != 26 {
		return false
	}
	if kind.hasTimezone {
		if n.Timezone == "" {
			n.Timezone = "Asia/Jakarta"
		}
		_, err := time.LoadLocation(n.Timezone)
		if err != nil {
			return false
		}
	}
	return true
}

func (a adminAPI) createOrganization(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "organisasi.buat")
	if !ok {
		return
	}
	kind, ok := orgConfig(w, r)
	if !ok {
		return
	}
	var n orgNode
	if !readJSON(w, r, &n) {
		return
	}
	n.Code = strings.ToUpper(strings.TrimSpace(n.Code))
	n.Name = strings.TrimSpace(n.Name)
	if kind.hasTimezone && n.Timezone == "" {
		n.Timezone = "Asia/Jakarta"
	}
	if !validateNode(n, kind) {
		writeJSON(w, 400, map[string]string{"error": "Kode, nama, atau induk tidak valid"})
		return
	}
	id, err := ids.New()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membuat ID"})
		return
	}
	n.ID = id
	columns := "tenant_id,id,code,name"
	args := []any{c.tenant.ID, n.ID, n.Code, n.Name}
	marks := "?,?,?,?"
	if kind.parentColumn != "" {
		columns += "," + kind.parentColumn
		marks += ",?"
		args = append(args, n.ParentID)
	}
	if kind.hasTimezone {
		columns += ",timezone"
		marks += ",?"
		args = append(args, n.Timezone)
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan data"})
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), "INSERT INTO "+kind.table+" ("+columns+") VALUES ("+marks+")", args...)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Kode duplikat atau induk tidak tersedia"})
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", kind.table, n.ID, nil, n); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan data"})
		return
	}
	writeJSON(w, 201, n)
}

func (a adminAPI) updateOrganization(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "organisasi.ubah")
	if !ok {
		return
	}
	if _, ok = a.authorize(w, r, "organisasi.lihat"); !ok {
		return
	}
	kind, ok := orgConfig(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeJSON(w, 400, map[string]string{"error": "ID tidak valid"})
		return
	}
	var n orgNode
	if !readJSON(w, r, &n) {
		return
	}
	n.ID = id
	n.Code = strings.ToUpper(strings.TrimSpace(n.Code))
	n.Name = strings.TrimSpace(n.Name)
	if kind.hasTimezone && n.Timezone == "" {
		n.Timezone = "Asia/Jakarta"
	}
	if !validateNode(n, kind) {
		writeJSON(w, 400, map[string]string{"error": "Kode, nama, atau induk tidak valid"})
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan data"})
		return
	}
	defer tx.Rollback()
	columns := "id,code,name"
	if kind.parentColumn != "" {
		columns += "," + kind.parentColumn
	}
	if kind.hasTimezone {
		columns += ",timezone"
	}
	var old orgNode
	dest := []any{&old.ID, &old.Code, &old.Name}
	if kind.parentColumn != "" {
		dest = append(dest, &old.ParentID)
	}
	if kind.hasTimezone {
		dest = append(dest, &old.Timezone)
	}
	err = tx.QueryRowContext(r.Context(), "SELECT "+columns+" FROM "+kind.table+" WHERE tenant_id=? AND id=? FOR UPDATE", c.tenant.ID, id).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Data tidak ditemukan"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca data"})
		return
	}
	sets := "code=?,name=?"
	args := []any{n.Code, n.Name}
	if kind.parentColumn != "" {
		sets += "," + kind.parentColumn + "=?"
		args = append(args, n.ParentID)
	}
	if kind.hasTimezone {
		sets += ",timezone=?"
		args = append(args, n.Timezone)
	}
	args = append(args, c.tenant.ID, id)
	_, err = tx.ExecContext(r.Context(), "UPDATE "+kind.table+" SET "+sets+" WHERE tenant_id=? AND id=?", args...)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Kode duplikat atau induk tidak tersedia"})
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", kind.table, id, old, n); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan data"})
		return
	}
	writeJSON(w, 200, n)
}

func audit(ctx context.Context, tx *sql.Tx, tenantID, actorID, action, entityType, entityID string, before, after any) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	var beforeJSON, afterJSON any
	if before != nil {
		raw, err := json.Marshal(before)
		if err != nil {
			return err
		}
		beforeJSON = string(raw)
	}
	if after != nil {
		raw, err := json.Marshal(after)
		if err != nil {
			return err
		}
		afterJSON = string(raw)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs (tenant_id,id,actor_user_id,action,entity_type,entity_id,before_json,after_json) VALUES (?,?,?,?,?,?,?,?)`, tenantID, id, actorID, action, entityType, entityID, beforeJSON, afterJSON)
	return err
}
