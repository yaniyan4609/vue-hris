package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
	"github.com/go-sql-driver/mysql"
)

type payrollComponentView struct {
	ID            string `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	ComponentType string `json:"component_type"`
	Taxable       bool   `json:"taxable"`
	CalcType      string `json:"calc_type"`
	IsSystem      bool   `json:"is_system"`
	IsActive      bool   `json:"is_active"`
	Version       uint64 `json:"version"`
}

const payrollComponentColumns = `id,code,name,component_type,taxable,calc_type,is_system,is_active,version`

func scanPayrollComponent(row interface{ Scan(...any) error }) (payrollComponentView, error) {
	var item payrollComponentView
	err := row.Scan(&item.ID, &item.Code, &item.Name, &item.ComponentType, &item.Taxable, &item.CalcType, &item.IsSystem, &item.IsActive, &item.Version)
	return item, err
}

func validPayrollComponent(item *payrollComponentView) bool {
	item.Code = strings.ToUpper(strings.TrimSpace(item.Code))
	item.Name = strings.TrimSpace(item.Name)
	return leaveCodePattern.MatchString(item.Code) && item.Name != "" && len([]rune(item.Name)) <= 160 && (item.ComponentType == "earning" || item.ComponentType == "deduction" || item.ComponentType == "employer") && item.CalcType == "fixed"
}

func (a adminAPI) listPayrollComponents(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	q, ok := searchTerm(w, r)
	if !ok {
		return
	}
	where := "tenant_id=?"
	args := []any{c.tenant.ID}
	if active := r.URL.Query().Get("is_active"); active != "" {
		if active != "true" && active != "false" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter aktif tidak valid.")
			return
		}
		where += " AND is_active=?"
		args = append(args, active == "true")
	}
	if q != "" {
		where += " AND (LOCATE(LOWER(?),LOWER(code))>0 OR LOCATE(LOWER(?),LOWER(name))>0)"
		args = append(args, q, q)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM pay_components WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen gaji.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+payrollComponentColumns+` FROM pay_components WHERE `+where+` ORDER BY name,id LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen gaji.")
		return
	}
	defer rows.Close()
	items := []payrollComponentView{}
	for rows.Next() {
		item, e := scanPayrollComponent(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen gaji.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen gaji.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) getPayrollComponent(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Komponen gaji tidak ditemukan.")
		return
	}
	item, err := scanPayrollComponent(a.db.QueryRowContext(r.Context(), `SELECT `+payrollComponentColumns+` FROM pay_components WHERE tenant_id=? AND id=?`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Komponen gaji tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen gaji.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) createPayrollComponent(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.kelola")
	if !ok {
		return
	}
	var body struct {
		Code          string `json:"code"`
		Name          string `json:"name"`
		ComponentType string `json:"component_type"`
		Taxable       bool   `json:"taxable"`
		CalcType      string `json:"calc_type"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	item := payrollComponentView{Code: body.Code, Name: body.Name, ComponentType: body.ComponentType, Taxable: body.Taxable, CalcType: body.CalcType, IsActive: true, Version: 1}
	if !validPayrollComponent(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kode, nama, jenis, atau cara hitung komponen tidak valid. Saat ini hanya fixed tersedia.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID komponen.")
		return
	}
	item.ID = id
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan komponen gaji.")
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), `INSERT INTO pay_components (tenant_id,id,code,name,component_type,taxable,calc_type,is_system,is_active,version) VALUES (?,?,?,?,?,?,?,FALSE,TRUE,1)`, c.tenant.ID, id, item.Code, item.Name, item.ComponentType, item.Taxable, item.CalcType)
	if err != nil {
		var dbError *mysql.MySQLError
		if errors.As(err, &dbError) && dbError.Number == 1062 {
			writeAPIError(w, r, 409, "COMPONENT_EXISTS", "Kode komponen sudah dipakai.")
		} else {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan komponen gaji.")
		}
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "pay_components", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat komponen gaji.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan komponen gaji.")
		return
	}
	writeAPIData(w, 201, item, nil)
}

func (a adminAPI) updatePayrollComponent(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Komponen gaji tidak ditemukan.")
		return
	}
	var body struct {
		Version  uint64  `json:"version"`
		Name     *string `json:"name"`
		Taxable  *bool   `json:"taxable"`
		IsActive *bool   `json:"is_active"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Version == 0 || (body.Name == nil && body.Taxable == nil && body.IsActive == nil) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi dan perubahan komponen wajib diisi.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah komponen gaji.")
		return
	}
	defer tx.Rollback()
	old, err := scanPayrollComponent(tx.QueryRowContext(r.Context(), `SELECT `+payrollComponentColumns+` FROM pay_components WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Komponen gaji tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen gaji.")
		return
	}
	if old.IsSystem {
		writeAPIError(w, r, 409, "SYSTEM_COMPONENT", "Komponen sistem tidak dapat diubah.")
		return
	}
	if body.Version != old.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Komponen sudah berubah.")
		return
	}
	item := old
	if body.Name != nil {
		item.Name = *body.Name
	}
	if body.Taxable != nil {
		item.Taxable = *body.Taxable
	}
	if body.IsActive != nil {
		item.IsActive = *body.IsActive
	}
	if !validPayrollComponent(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Nama komponen tidak valid.")
		return
	}
	if !item.IsActive {
		if !a.canDeactivatePayrollComponent(w, r, tx, c.tenant.ID, id) {
			return
		}
	}
	item.Version++
	if _, err = tx.ExecContext(r.Context(), `UPDATE pay_components SET name=?,taxable=?,is_active=?,version=? WHERE tenant_id=? AND id=?`, item.Name, item.Taxable, item.IsActive, item.Version, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah komponen gaji.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "pay_components", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat komponen gaji.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah komponen gaji.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) canDeactivatePayrollComponent(w http.ResponseWriter, r *http.Request, tx *sql.Tx, tenantID, componentID string) bool {
	rows, err := tx.QueryContext(r.Context(), `SELECT data_json FROM bpjs_wage_basis_policies WHERE tenant_id=? AND (effective_to IS NULL OR effective_to>=CURRENT_DATE())`, tenantID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa dasar upah BPJS.")
		return false
	}
	usedInBPJS := false
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			break
		}
		var data bpjsWageBasisData
		if err = json.Unmarshal(raw, &data); err != nil {
			break
		}
		for _, program := range data.Programs {
			for _, id := range program.ComponentIDs {
				usedInBPJS = usedInBPJS || id == componentID
			}
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa dasar upah BPJS.")
		return false
	}
	if usedInBPJS {
		writeAPIError(w, r, 409, "COMPONENT_IN_BPJS_BASIS", "Komponen masih dipakai pada dasar upah BPJS. Terbitkan aturan baru sebelum menonaktifkannya.")
		return false
	}
	var active int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employee_pay_components WHERE tenant_id=? AND component_id=? AND (valid_to IS NULL OR valid_to>=CURRENT_DATE())`, tenantID, componentID).Scan(&active); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa penetapan komponen.")
		return false
	}
	if active != 0 {
		writeAPIError(w, r, 409, "COMPONENT_ASSIGNED", "Komponen masih ditetapkan ke karyawan.")
		return false
	}
	return true
}

func (a adminAPI) deactivatePayrollComponent(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Komponen gaji tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan komponen gaji.")
		return
	}
	defer tx.Rollback()
	old, err := scanPayrollComponent(tx.QueryRowContext(r.Context(), `SELECT `+payrollComponentColumns+` FROM pay_components WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Komponen gaji tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen gaji.")
		return
	}
	if old.IsSystem {
		writeAPIError(w, r, 409, "SYSTEM_COMPONENT", "Komponen sistem tidak dapat dinonaktifkan.")
		return
	}
	if !old.IsActive {
		w.WriteHeader(204)
		return
	}
	if !a.canDeactivatePayrollComponent(w, r, tx, c.tenant.ID, id) {
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE pay_components SET is_active=FALSE,version=version+1 WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan komponen gaji.")
		return
	}
	item := old
	item.IsActive = false
	item.Version++
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "deactivate", "pay_components", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat komponen gaji.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan komponen gaji.")
		return
	}
	w.WriteHeader(204)
}
