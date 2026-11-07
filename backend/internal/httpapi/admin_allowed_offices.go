package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type allowedOffice struct {
	ID         string     `json:"id"`
	EmployeeID string     `json:"employee_id"`
	OfficeID   string     `json:"office_id"`
	OfficeName string     `json:"office_name"`
	ValidFrom  string     `json:"valid_from"`
	ValidTo    *string    `json:"valid_to"`
	Reason     *string    `json:"reason"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

func scanAllowedOffice(row interface{ Scan(...any) error }) (allowedOffice, error) {
	var item allowedOffice
	var from time.Time
	var to sql.NullTime
	var reason sql.NullString
	var revoked sql.NullTime
	err := row.Scan(&item.ID, &item.EmployeeID, &item.OfficeID, &item.OfficeName, &from, &to, &reason, &revoked)
	if err != nil {
		return item, err
	}
	item.ValidFrom = from.Format("2006-01-02")
	if to.Valid {
		value := to.Time.Format("2006-01-02")
		item.ValidTo = &value
	}
	if reason.Valid {
		item.Reason = &reason.String
	}
	if revoked.Valid {
		item.RevokedAt = &revoked.Time
	}
	return item, nil
}

const allowedOfficeColumns = `ao.id,ao.employee_id,ao.office_id,o.name,ao.valid_from,ao.valid_to,ao.reason,ao.revoked_at`
const allowedOfficeJoin = ` FROM employee_allowed_offices ao JOIN offices o ON o.tenant_id=ao.tenant_id AND o.id=ao.office_id`

func (a adminAPI) allowedOfficeEmployee(w http.ResponseWriter, r *http.Request, c adminContext) (string, bool) {
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return "", false
	}
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, id}, args...)
	var found string
	err := a.db.QueryRowContext(r.Context(), `SELECT e.id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=? AND `+filter, params...).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return "", false
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
		return "", false
	}
	return found, true
}

func (a adminAPI) listAllowedOffices(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return
	}
	id, ok := a.allowedOfficeEmployee(w, r, c)
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+allowedOfficeColumns+allowedOfficeJoin+` WHERE ao.tenant_id=? AND ao.employee_id=? ORDER BY ao.valid_from DESC,ao.id DESC`, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin kantor.")
		return
	}
	defer rows.Close()
	items := []allowedOffice{}
	for rows.Next() {
		item, err := scanAllowedOffice(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin kantor.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin kantor.")
		return
	}
	writeAPIData(w, 200, items, nil)
}

func (a adminAPI) createAllowedOffice(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	employeeID, ok := a.allowedOfficeEmployee(w, r, c)
	if !ok {
		return
	}
	var body struct {
		OfficeID  string  `json:"office_id"`
		ValidFrom string  `json:"valid_from"`
		ValidTo   *string `json:"valid_to"`
		Reason    string  `json:"reason"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	from, err := time.Parse("2006-01-02", body.ValidFrom)
	if err != nil || from.Format("2006-01-02") != body.ValidFrom || len(body.OfficeID) != 26 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kantor atau tanggal mulai tidak valid.")
		return
	}
	if body.ValidTo != nil {
		to, err := time.Parse("2006-01-02", *body.ValidTo)
		if err != nil || to.Format("2006-01-02") != *body.ValidTo || to.Before(from) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal akhir tidak valid.")
			return
		}
	}
	reason := strings.TrimSpace(body.Reason)
	if len([]rune(reason)) > 255 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Alasan terlalu panjang.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menambah izin kantor.")
		return
	}
	defer tx.Rollback()
	var locked string
	if err := tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, employeeID).Scan(&locked); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menambah izin kantor.")
		return
	}
	var officeName string
	if err := tx.QueryRowContext(r.Context(), `SELECT name FROM offices WHERE tenant_id=? AND id=?`, c.tenant.ID, body.OfficeID).Scan(&officeName); errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kantor tidak ditemukan dalam tenant.")
		return
	} else if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kantor.")
		return
	}
	var overlap int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employee_allowed_offices WHERE tenant_id=? AND employee_id=? AND office_id=? AND revoked_at IS NULL AND (valid_to IS NULL OR valid_to>=?) AND (? IS NULL OR valid_from<=?)`, c.tenant.ID, employeeID, body.OfficeID, body.ValidFrom, body.ValidTo, body.ValidTo).Scan(&overlap); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa masa berlaku.")
		return
	}
	if overlap != 0 {
		writeAPIError(w, r, 409, "DATE_OVERLAP", "Masa berlaku kantor sudah ada.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menambah izin kantor.")
		return
	}
	var reasonValue any
	if reason != "" {
		reasonValue = reason
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO employee_allowed_offices (tenant_id,id,employee_id,office_id,valid_from,valid_to,reason) VALUES (?,?,?,?,?,?,?)`, c.tenant.ID, id, employeeID, body.OfficeID, body.ValidFrom, body.ValidTo, reasonValue); err != nil {
		writeAPIError(w, r, 409, "DATE_OVERLAP", "Izin kantor sudah ada.")
		return
	}
	item := allowedOffice{ID: id, EmployeeID: employeeID, OfficeID: body.OfficeID, OfficeName: officeName, ValidFrom: body.ValidFrom, ValidTo: body.ValidTo}
	if reason != "" {
		item.Reason = &reason
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "employee_allowed_offices", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menambah izin kantor.")
		return
	}
	writeAPIData(w, 201, item, nil)
}

func (a adminAPI) revokeAllowedOffice(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	employeeID, ok := a.allowedOfficeEmployee(w, r, c)
	if !ok {
		return
	}
	id := chi.URLParam(r, "allowedId")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Izin kantor tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencabut izin kantor.")
		return
	}
	defer tx.Rollback()
	var locked string
	if err := tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, employeeID).Scan(&locked); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencabut izin kantor.")
		return
	}
	old, err := scanAllowedOffice(tx.QueryRowContext(r.Context(), `SELECT `+allowedOfficeColumns+allowedOfficeJoin+` WHERE ao.tenant_id=? AND ao.employee_id=? AND ao.id=? FOR UPDATE`, c.tenant.ID, employeeID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Izin kantor tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin kantor.")
		return
	}
	if old.RevokedAt != nil {
		writeAPIError(w, r, 409, "ALREADY_REVOKED", "Izin kantor sudah dicabut.")
		return
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(r.Context(), `UPDATE employee_allowed_offices SET revoked_at=?,revoked_by=? WHERE tenant_id=? AND id=? AND revoked_at IS NULL`, now, c.state.User.ID, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencabut izin kantor.")
		return
	}
	after := old
	after.RevokedAt = &now
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "revoke", "employee_allowed_offices", id, old, after); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencabut izin kantor.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
