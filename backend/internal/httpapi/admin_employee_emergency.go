package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type emergencyContact struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employee_id"`
	Name       string `json:"name"`
	Relation   string `json:"relation"`
	Phone      string `json:"phone"`
}

const contactColumns = `id,employee_id,name,relation,phone`

var contactPhone = regexp.MustCompile(`^[+0-9][0-9()+ .-]{5,31}$`)

func scanContact(row interface{ Scan(...any) error }) (emergencyContact, error) {
	var item emergencyContact
	err := row.Scan(&item.ID, &item.EmployeeID, &item.Name, &item.Relation, &item.Phone)
	return item, err
}

func validContact(item *emergencyContact) bool {
	item.Name = strings.TrimSpace(item.Name)
	item.Relation = strings.TrimSpace(item.Relation)
	item.Phone = strings.TrimSpace(item.Phone)
	if item.Name == "" || len([]rune(item.Name)) > 200 || item.Relation == "" || len([]rune(item.Relation)) > 32 || !contactPhone.MatchString(item.Phone) {
		return false
	}
	digits := 0
	for _, c := range item.Phone {
		if c >= '0' && c <= '9' {
			digits++
		}
	}
	return digits >= 6
}

func (a adminAPI) listEmployeeEmergencyContacts(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return
	}
	employeeID, ok := a.scopedEmployee(w, r, c)
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+contactColumns+` FROM employee_emergency_contacts WHERE tenant_id=? AND employee_id=? ORDER BY name,id`, c.tenant.ID, employeeID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontak darurat.")
		return
	}
	defer rows.Close()
	items := []emergencyContact{}
	for rows.Next() {
		item, err := scanContact(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontak darurat.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontak darurat.")
		return
	}
	writeAPIData(w, 200, items, nil)
}

func (a adminAPI) createEmployeeEmergencyContact(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	employeeID, ok := a.scopedEmployee(w, r, c)
	if !ok {
		return
	}
	var body struct {
		Name     string `json:"name"`
		Relation string `json:"relation"`
		Phone    string `json:"phone"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat kontak darurat.")
		return
	}
	item := emergencyContact{ID: id, EmployeeID: employeeID, Name: body.Name, Relation: body.Relation, Phone: body.Phone}
	if !validContact(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kontak darurat tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontak darurat.")
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO employee_emergency_contacts (tenant_id,id,employee_id,name,relation,phone) VALUES (?,?,?,?,?,?)`, c.tenant.ID, id, employeeID, item.Name, item.Relation, item.Phone); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontak darurat.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "employee_emergency_contacts", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontak darurat.")
		return
	}
	writeAPIData(w, 201, item, nil)
}

func (a adminAPI) updateEmployeeEmergencyContact(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	employeeID, ok := a.scopedEmployee(w, r, c)
	if !ok {
		return
	}
	id := chi.URLParam(r, "contactId")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontak darurat tidak ditemukan.")
		return
	}
	var body struct {
		Name     *string `json:"name"`
		Relation *string `json:"relation"`
		Phone    *string `json:"phone"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Name == nil && body.Relation == nil && body.Phone == nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tidak ada perubahan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontak darurat.")
		return
	}
	defer tx.Rollback()
	old, err := scanContact(tx.QueryRowContext(r.Context(), `SELECT `+contactColumns+` FROM employee_emergency_contacts WHERE tenant_id=? AND employee_id=? AND id=? FOR UPDATE`, c.tenant.ID, employeeID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontak darurat tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontak darurat.")
		return
	}
	item := old
	if body.Name != nil {
		item.Name = *body.Name
	}
	if body.Relation != nil {
		item.Relation = *body.Relation
	}
	if body.Phone != nil {
		item.Phone = *body.Phone
	}
	if !validContact(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kontak darurat tidak valid.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE employee_emergency_contacts SET name=?,relation=?,phone=? WHERE tenant_id=? AND employee_id=? AND id=?`, item.Name, item.Relation, item.Phone, c.tenant.ID, employeeID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontak darurat.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "employee_emergency_contacts", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontak darurat.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) deleteEmployeeEmergencyContact(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	employeeID, ok := a.scopedEmployee(w, r, c)
	if !ok {
		return
	}
	id := chi.URLParam(r, "contactId")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontak darurat tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus kontak darurat.")
		return
	}
	defer tx.Rollback()
	old, err := scanContact(tx.QueryRowContext(r.Context(), `SELECT `+contactColumns+` FROM employee_emergency_contacts WHERE tenant_id=? AND employee_id=? AND id=? FOR UPDATE`, c.tenant.ID, employeeID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontak darurat tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontak darurat.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM employee_emergency_contacts WHERE tenant_id=? AND employee_id=? AND id=?`, c.tenant.ID, employeeID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus kontak darurat.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "delete", "employee_emergency_contacts", id, old, nil); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus kontak darurat.")
		return
	}
	w.WriteHeader(204)
}
