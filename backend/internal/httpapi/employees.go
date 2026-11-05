package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type employee struct {
	ID             string `json:"id"`
	EmployeeNumber string `json:"employeeNumber"`
	DisplayName    string `json:"displayName"`
	Status         string `json:"status"`
	BaseOfficeID   string `json:"baseOfficeId"`
	OfficeName     string `json:"officeName,omitempty"`
	PositionID     string `json:"positionId"`
	PositionName   string `json:"positionName,omitempty"`
	ReportsToID    string `json:"reportsToId,omitempty"`
	UserID         string `json:"userId,omitempty"`
}

const employeeFrom = ` FROM employees e JOIN offices o ON o.tenant_id=e.tenant_id AND o.id=e.base_office_id JOIN positions p ON p.tenant_id=e.tenant_id AND p.id=e.position_id JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id`
const employeeSelect = `SELECT e.id,e.employee_number,e.display_name,e.status,e.base_office_id,o.name,e.position_id,p.name,COALESCE(e.reports_to_id,''),COALESCE(e.user_id,'')` + employeeFrom

func scanEmployee(row interface{ Scan(...any) error }) (employee, error) {
	var e employee
	err := row.Scan(&e.ID, &e.EmployeeNumber, &e.DisplayName, &e.Status, &e.BaseOfficeID, &e.OfficeName, &e.PositionID, &e.PositionName, &e.ReportsToID, &e.UserID)
	return e, err
}

func (a adminAPI) listEmployees(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.lihat")
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
	sort := map[string]string{"name": "e.display_name", "number": "e.employee_number", "status": "e.status"}[r.URL.Query().Get("sort")]
	if sort == "" {
		if r.URL.Query().Get("sort") != "" {
			writeJSON(w, 400, map[string]string{"error": "Urutan tidak valid"})
			return
		}
		sort = "e.display_name"
	}
	direction := strings.ToLower(r.URL.Query().Get("order"))
	if direction == "" {
		direction = "asc"
	}
	if direction != "asc" && direction != "desc" {
		writeJSON(w, 400, map[string]string{"error": "Arah urutan tidak valid"})
		return
	}
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	where := "e.tenant_id=? AND " + filter
	params := append([]any{c.tenant.ID}, args...)
	if q != "" {
		where += " AND (LOCATE(LOWER(?), LOWER(e.employee_number))>0 OR LOCATE(LOWER(?), LOWER(e.display_name))>0)"
		params = append(params, q, q)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*)"+employeeFrom+" WHERE "+where, params...).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca jumlah karyawan"})
		return
	}
	rows, err := a.db.QueryContext(r.Context(), employeeSelect+" WHERE "+where+" ORDER BY "+sort+" "+direction+", e.id ASC LIMIT ? OFFSET ?", append(params, page.PageSize, page.Offset)...)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca karyawan"})
		return
	}
	defer rows.Close()
	items := []employee{}
	for rows.Next() {
		e, err := scanEmployee(rows)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca karyawan"})
			return
		}
		items = append(items, e)
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca karyawan"})
		return
	}
	writeJSON(w, 200, pageResult[employee]{Items: items, Total: total, Page: page.Page, PageSize: page.PageSize})
}

func (a adminAPI) getEmployee(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeJSON(w, 404, map[string]string{"error": "Karyawan tidak ditemukan"})
		return
	}
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, id}, args...)
	e, err := scanEmployee(a.db.QueryRowContext(r.Context(), employeeSelect+" WHERE e.tenant_id=? AND e.id=? AND "+filter, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Karyawan tidak ditemukan"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca karyawan"})
		return
	}
	writeJSON(w, 200, e)
}

func validateEmployee(e employee) bool {
	if strings.TrimSpace(e.EmployeeNumber) == "" || len(e.EmployeeNumber) > 64 || strings.TrimSpace(e.DisplayName) == "" || len([]rune(e.DisplayName)) > 200 || len(e.BaseOfficeID) != 26 || len(e.PositionID) != 26 {
		return false
	}
	if e.ReportsToID != "" && len(e.ReportsToID) != 26 {
		return false
	}
	if e.UserID != "" && len(e.UserID) != 26 {
		return false
	}
	switch e.Status {
	// Resign and termination require their own dated workflow so roles and
	// sessions are revoked atomically on the effective date.
	case "active", "probation", "leave":
		return true
	}
	return false
}

func (a adminAPI) targetFor(ctx context.Context, tenantID string, e employee) (access.Target, error) {
	t := access.Target{TenantID: tenantID, EmployeeID: e.ID, UserID: e.UserID, ReportsToID: e.ReportsToID, BaseOfficeID: e.BaseOfficeID, PositionID: e.PositionID}
	err := a.db.QueryRowContext(ctx, `SELECT o.region_id,p.department_id,d.division_id FROM offices o JOIN positions p ON p.tenant_id=o.tenant_id AND p.id=? JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id WHERE o.tenant_id=? AND o.id=?`, e.PositionID, tenantID, e.BaseOfficeID).Scan(&t.RegionID, &t.DepartmentID, &t.DivisionID)
	return t, err
}

func (a adminAPI) createEmployee(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.buat")
	if !ok {
		return
	}
	var e employee
	if !readJSON(w, r, &e) {
		return
	}
	e.EmployeeNumber = strings.TrimSpace(e.EmployeeNumber)
	e.DisplayName = strings.TrimSpace(e.DisplayName)
	if e.Status == "" {
		e.Status = "active"
	}
	if !validateEmployee(e) {
		writeJSON(w, 400, map[string]string{"error": "Data karyawan tidak valid"})
		return
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, e)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 400, map[string]string{"error": "Kantor atau jabatan tidak tersedia"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memvalidasi penempatan"})
		return
	}
	if !c.effective.Includes(c.subject, target) {
		writeJSON(w, 403, map[string]string{"error": "Penempatan di luar cakupan akses"})
		return
	}
	id, err := ids.New()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membuat ID"})
		return
	}
	e.ID = id
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan karyawan"})
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), `INSERT INTO employees (tenant_id,id,user_id,employee_number,display_name,status,base_office_id,position_id,reports_to_id) VALUES (?,?,?,?,?,?,?,?,?)`, c.tenant.ID, id, nullID(e.UserID), e.EmployeeNumber, e.DisplayName, e.Status, e.BaseOfficeID, e.PositionID, nullID(e.ReportsToID))
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Nomor karyawan duplikat atau referensi tidak tersedia"})
		return
	}
	if err = insertAssignment(r.Context(), tx, c.tenant.ID, e); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat penempatan"})
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "employees", id, nil, e); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan karyawan"})
		return
	}
	writeJSON(w, 201, e)
}

func (a adminAPI) updateEmployee(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	view, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeJSON(w, 404, map[string]string{"error": "Karyawan tidak ditemukan"})
		return
	}
	filter, args := access.EmployeeWhere(view.subject, view.effective)
	params := append([]any{c.tenant.ID, id}, args...)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan karyawan"})
		return
	}
	defer tx.Rollback()
	old, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+" WHERE e.tenant_id=? AND e.id=? AND "+filter+" FOR UPDATE", params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"error": "Karyawan tidak ditemukan"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal membaca karyawan"})
		return
	}
	var next employee
	if !readJSON(w, r, &next) {
		return
	}
	next.ID = id
	next.UserID = old.UserID
	next.EmployeeNumber = strings.TrimSpace(next.EmployeeNumber)
	next.DisplayName = strings.TrimSpace(next.DisplayName)
	if !validateEmployee(next) {
		writeJSON(w, 400, map[string]string{"error": "Data karyawan tidak valid"})
		return
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, next)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 400, map[string]string{"error": "Kantor atau jabatan tidak tersedia"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal memvalidasi penempatan"})
		return
	}
	if !c.effective.Includes(c.subject, target) || !view.effective.Includes(view.subject, target) {
		writeJSON(w, 403, map[string]string{"error": "Penempatan di luar cakupan akses"})
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE employees SET employee_number=?,display_name=?,status=?,base_office_id=?,position_id=?,reports_to_id=? WHERE tenant_id=? AND id=?`, next.EmployeeNumber, next.DisplayName, next.Status, next.BaseOfficeID, next.PositionID, nullID(next.ReportsToID), c.tenant.ID, id)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Nomor karyawan duplikat atau referensi tidak tersedia"})
		return
	}
	if old.BaseOfficeID != next.BaseOfficeID || old.PositionID != next.PositionID || old.ReportsToID != next.ReportsToID {
		if _, err = tx.ExecContext(r.Context(), `UPDATE employee_assignment_history SET valid_until=UTC_TIMESTAMP(6) WHERE tenant_id=? AND employee_id=? AND valid_until IS NULL`, c.tenant.ID, id); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal menutup penempatan lama"})
			return
		}
		if err = insertAssignment(r.Context(), tx, c.tenant.ID, next); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal mencatat penempatan"})
			return
		}
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "employees", id, old, next); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal mencatat audit"})
		return
	}
	if err = tx.Commit(); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Gagal menyimpan karyawan"})
		return
	}
	writeJSON(w, 200, next)
}

func nullID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func insertAssignment(ctx context.Context, tx *sql.Tx, tenantID string, e employee) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO employee_assignment_history (tenant_id,id,employee_id,base_office_id,position_id,reports_to_id,valid_from) VALUES (?,?,?,?,?,?,UTC_TIMESTAMP(6))`, tenantID, id, e.ID, e.BaseOfficeID, e.PositionID, nullID(e.ReportsToID))
	return err
}

func mountESS(r chi.Router, db *sql.DB) {
	service := auth.Service{DB: db}
	r.Get("/api/v1/ess/me/employee", func(w http.ResponseWriter, req *http.Request) {
		state, err := service.Current(req.Context(), cookieToken(req))
		if err != nil {
			writeAuthError(w, err)
			return
		}
		if state.SelectedTenantID == "" {
			writeJSON(w, 403, map[string]string{"error": "Pilih perusahaan terlebih dahulu"})
			return
		}
		e, err := scanEmployee(db.QueryRowContext(req.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.user_id=? AND e.status IN ('active','probation','leave')`, state.SelectedTenantID, state.User.ID))
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, 403, map[string]string{"error": "Profil karyawan tidak tersedia"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca profil"})
			return
		}
		writeJSON(w, 200, e)
	})
}
