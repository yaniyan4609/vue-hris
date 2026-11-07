package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type employeeFamily struct {
	ID          string  `json:"id"`
	EmployeeID  string  `json:"employee_id"`
	Relation    string  `json:"relation"`
	Name        string  `json:"name"`
	BirthDate   *string `json:"birth_date"`
	IsDependent bool    `json:"is_dependent"`
}

func scanFamily(row interface{ Scan(...any) error }) (employeeFamily, error) {
	var item employeeFamily
	var birth sql.NullTime
	err := row.Scan(&item.ID, &item.EmployeeID, &item.Relation, &item.Name, &birth, &item.IsDependent)
	if err == nil && birth.Valid {
		date := birth.Time.Format("2006-01-02")
		item.BirthDate = &date
	}
	return item, err
}

const familyColumns = `id,employee_id,relation,name,birth_date,is_dependent`

func validFamily(item *employeeFamily) bool {
	item.Name = strings.TrimSpace(item.Name)
	item.Relation = strings.TrimSpace(item.Relation)
	if item.Name == "" || len([]rune(item.Name)) > 200 || item.Relation == "" || len([]rune(item.Relation)) > 32 {
		return false
	}
	if item.BirthDate != nil {
		date, err := time.Parse("2006-01-02", *item.BirthDate)
		if err != nil || date.Format("2006-01-02") != *item.BirthDate || date.After(time.Now().UTC()) {
			return false
		}
	}
	return true
}

func (a adminAPI) listEmployeeFamilies(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return
	}
	employeeID, ok := a.scopedEmployee(w, r, c)
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+familyColumns+` FROM employee_families WHERE tenant_id=? AND employee_id=? ORDER BY name,id`, c.tenant.ID, employeeID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca keluarga.")
		return
	}
	defer rows.Close()
	items := []employeeFamily{}
	for rows.Next() {
		item, err := scanFamily(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca keluarga.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca keluarga.")
		return
	}
	writeAPIData(w, 200, items, nil)
}

func (a adminAPI) createEmployeeFamily(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	employeeID, ok := a.scopedEmployee(w, r, c)
	if !ok {
		return
	}
	var body struct {
		Relation    string  `json:"relation"`
		Name        string  `json:"name"`
		BirthDate   *string `json:"birth_date"`
		IsDependent bool    `json:"is_dependent"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat data keluarga.")
		return
	}
	item := employeeFamily{ID: id, EmployeeID: employeeID, Relation: body.Relation, Name: body.Name, BirthDate: body.BirthDate, IsDependent: body.IsDependent}
	if !validFamily(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data keluarga tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan keluarga.")
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO employee_families (tenant_id,id,employee_id,relation,name,birth_date,is_dependent) VALUES (?,?,?,?,?,?,?)`, c.tenant.ID, item.ID, item.EmployeeID, item.Relation, item.Name, item.BirthDate, item.IsDependent); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan keluarga.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "employee_families", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan keluarga.")
		return
	}
	writeAPIData(w, 201, item, nil)
}

func (a adminAPI) updateEmployeeFamily(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	employeeID, ok := a.scopedEmployee(w, r, c)
	if !ok {
		return
	}
	id := chi.URLParam(r, "familyId")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Keluarga tidak ditemukan.")
		return
	}
	var body struct {
		Relation    *string         `json:"relation"`
		Name        *string         `json:"name"`
		BirthDate   json.RawMessage `json:"birth_date"`
		IsDependent *bool           `json:"is_dependent"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Relation == nil && body.Name == nil && body.BirthDate == nil && body.IsDependent == nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tidak ada perubahan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan keluarga.")
		return
	}
	defer tx.Rollback()
	old, err := scanFamily(tx.QueryRowContext(r.Context(), `SELECT `+familyColumns+` FROM employee_families WHERE tenant_id=? AND employee_id=? AND id=? FOR UPDATE`, c.tenant.ID, employeeID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Keluarga tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca keluarga.")
		return
	}
	item := old
	if body.Relation != nil {
		item.Relation = *body.Relation
	}
	if body.Name != nil {
		item.Name = *body.Name
	}
	if body.IsDependent != nil {
		item.IsDependent = *body.IsDependent
	}
	if body.BirthDate != nil {
		if string(body.BirthDate) == "null" {
			item.BirthDate = nil
		} else {
			var date string
			if json.Unmarshal(body.BirthDate, &date) != nil {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal lahir tidak valid.")
				return
			}
			item.BirthDate = &date
		}
	}
	if !validFamily(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data keluarga tidak valid.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE employee_families SET relation=?,name=?,birth_date=?,is_dependent=? WHERE tenant_id=? AND employee_id=? AND id=?`, item.Relation, item.Name, item.BirthDate, item.IsDependent, c.tenant.ID, employeeID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan keluarga.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "employee_families", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan keluarga.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) deleteEmployeeFamily(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	employeeID, ok := a.scopedEmployee(w, r, c)
	if !ok {
		return
	}
	id := chi.URLParam(r, "familyId")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Keluarga tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus keluarga.")
		return
	}
	defer tx.Rollback()
	old, err := scanFamily(tx.QueryRowContext(r.Context(), `SELECT `+familyColumns+` FROM employee_families WHERE tenant_id=? AND employee_id=? AND id=? FOR UPDATE`, c.tenant.ID, employeeID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Keluarga tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca keluarga.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM employee_families WHERE tenant_id=? AND employee_id=? AND id=?`, c.tenant.ID, employeeID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus keluarga.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "delete", "employee_families", id, old, nil); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus keluarga.")
		return
	}
	w.WriteHeader(204)
}
