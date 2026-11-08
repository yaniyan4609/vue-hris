package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type reimbursementCategory struct {
	ID           string  `json:"id"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	MonthlyLimit *string `json:"monthly_limit"`
	IsActive     bool    `json:"is_active"`
}

func categoryLimit(raw json.RawMessage) (*string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, true
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, false
	}
	value := n.String()
	amount, ok := leaveHundredths(value)
	if !ok || amount <= 0 || len(strings.SplitN(value, ".", 2)[0]) > 16 {
		return nil, false
	}
	formatted := fmt.Sprintf("%d.%02d", amount/100, amount%100)
	return &formatted, true
}

func validCategory(item *reimbursementCategory) bool {
	item.Code = strings.ToUpper(strings.TrimSpace(item.Code))
	item.Name = strings.TrimSpace(item.Name)
	return leaveCodePattern.MatchString(item.Code) && item.Name != "" && len([]rune(item.Name)) <= 160
}

func scanCategory(row interface{ Scan(...any) error }) (reimbursementCategory, error) {
	var item reimbursementCategory
	var limit sql.NullString
	err := row.Scan(&item.ID, &item.Code, &item.Name, &limit, &item.IsActive)
	if limit.Valid {
		item.MonthlyLimit = &limit.String
	}
	return item, err
}

func (a adminAPI) listReimbursementCategories(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengaturan.kelola")
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
	if q != "" {
		where += " AND (LOCATE(LOWER(?),LOWER(code))>0 OR LOCATE(LOWER(?),LOWER(name))>0)"
		args = append(args, q, q)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM reimbursement_categories WHERE "+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kategori reimbursement.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,code,name,CAST(monthly_limit AS CHAR),is_active FROM reimbursement_categories WHERE "+where+" ORDER BY name,id LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kategori reimbursement.")
		return
	}
	defer rows.Close()
	items := []reimbursementCategory{}
	for rows.Next() {
		item, err := scanCategory(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kategori reimbursement.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kategori reimbursement.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) createReimbursementCategory(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengaturan.kelola")
	if !ok {
		return
	}
	var body struct {
		Code         string          `json:"code"`
		Name         string          `json:"name"`
		MonthlyLimit json.RawMessage `json:"monthly_limit"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	limit, valid := categoryLimit(body.MonthlyLimit)
	item := reimbursementCategory{Code: body.Code, Name: body.Name, MonthlyLimit: limit, IsActive: true}
	if !valid || !validCategory(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kode, nama, atau batas bulanan tidak valid.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat kategori reimbursement.")
		return
	}
	item.ID = id
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kategori reimbursement.")
		return
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM reimbursement_categories WHERE tenant_id=? AND code=?`, c.tenant.ID, item.Code).Scan(&count); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kategori reimbursement.")
		return
	}
	if count > 0 {
		writeAPIError(w, r, 409, "CATEGORY_EXISTS", "Kode kategori sudah digunakan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO reimbursement_categories (tenant_id,id,code,name,monthly_limit,is_active) VALUES (?,?,?,?,?,TRUE)`, c.tenant.ID, item.ID, item.Code, item.Name, item.MonthlyLimit); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kategori reimbursement.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "reimbursement_categories", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat perubahan kategori.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kategori reimbursement.")
		return
	}
	writeAPIData(w, 201, item, nil)
}

func (a adminAPI) updateReimbursementCategory(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengaturan.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kategori tidak ditemukan.")
		return
	}
	var body struct {
		Code         *string         `json:"code"`
		Name         *string         `json:"name"`
		MonthlyLimit json.RawMessage `json:"monthly_limit"`
		IsActive     *bool           `json:"is_active"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah kategori.")
		return
	}
	defer tx.Rollback()
	old, err := scanCategory(tx.QueryRowContext(r.Context(), `SELECT id,code,name,CAST(monthly_limit AS CHAR),is_active FROM reimbursement_categories WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kategori tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kategori.")
		return
	}
	item := old
	if body.Code != nil {
		item.Code = *body.Code
	}
	if body.Name != nil {
		item.Name = *body.Name
	}
	if body.MonthlyLimit != nil {
		var valid bool
		item.MonthlyLimit, valid = categoryLimit(body.MonthlyLimit)
		if !valid {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Batas bulanan tidak valid.")
			return
		}
	}
	if body.IsActive != nil {
		item.IsActive = *body.IsActive
	}
	if !validCategory(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kode atau nama kategori tidak valid.")
		return
	}
	var count int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM reimbursement_categories WHERE tenant_id=? AND code=? AND id<>?`, c.tenant.ID, item.Code, id).Scan(&count); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kategori.")
		return
	}
	if count > 0 {
		writeAPIError(w, r, 409, "CATEGORY_EXISTS", "Kode kategori sudah digunakan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE reimbursement_categories SET code=?,name=?,monthly_limit=?,is_active=? WHERE tenant_id=? AND id=?`, item.Code, item.Name, item.MonthlyLimit, item.IsActive, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah kategori.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "reimbursement_categories", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat perubahan kategori.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah kategori.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) deactivateReimbursementCategory(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengaturan.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kategori tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan kategori.")
		return
	}
	defer tx.Rollback()
	old, err := scanCategory(tx.QueryRowContext(r.Context(), `SELECT id,code,name,CAST(monthly_limit AS CHAR),is_active FROM reimbursement_categories WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kategori tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kategori.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE reimbursement_categories SET is_active=FALSE WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan kategori.")
		return
	}
	item := old
	item.IsActive = false
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "deactivate", "reimbursement_categories", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat perubahan kategori.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan kategori.")
		return
	}
	w.WriteHeader(204)
}
