package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type holiday struct {
	ID       string  `json:"id"`
	Date     string  `json:"holiday_date"`
	Name     string  `json:"name"`
	Type     string  `json:"holiday_type"`
	RegionID *string `json:"region_id"`
}

func scanHoliday(row interface{ Scan(...any) error }) (holiday, error) {
	var item holiday
	var region sql.NullString
	var date time.Time
	err := row.Scan(&item.ID, &date, &item.Name, &item.Type, &region)
	if err == nil {
		item.Date = date.Format("2006-01-02")
	}
	if region.Valid {
		item.RegionID = &region.String
	}
	return item, err
}
func (a adminAPI) readHoliday(r *http.Request, tenantID, id string) (holiday, error) {
	return scanHoliday(a.db.QueryRowContext(r.Context(), `SELECT id,holiday_date,name,holiday_type,region_id FROM holidays WHERE tenant_id=? AND id=?`, tenantID, id))
}
func validHoliday(item *holiday) bool {
	item.Name = strings.TrimSpace(item.Name)
	item.Type = strings.TrimSpace(item.Type)
	date, err := time.Parse("2006-01-02", item.Date)
	if err != nil || date.Format("2006-01-02") != item.Date || item.Name == "" || len([]rune(item.Name)) > 160 || item.Type == "" || len(item.Type) > 24 {
		return false
	}
	if item.RegionID != nil && len(*item.RegionID) != 26 {
		return false
	}
	return true
}
func holidayYear(w http.ResponseWriter, r *http.Request, zone string) (int, bool) {
	location, err := time.LoadLocation(zone)
	if err != nil {
		location = time.UTC
	}
	year := time.Now().In(location).Year()
	if text := r.URL.Query().Get("year"); text != "" {
		year, err = strconv.Atoi(text)
		if err != nil || year < 2000 || year > 2100 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tahun tidak valid.")
			return 0, false
		}
	}
	return year, true
}
func (a adminAPI) listHolidays(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.lihat")
	if !ok {
		return
	}
	year, ok := holidayYear(w, r, c.tenant.Timezone)
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	where := "tenant_id=? AND holiday_date>=? AND holiday_date<?"
	args := []any{c.tenant.ID, strconv.Itoa(year) + "-01-01", strconv.Itoa(year+1) + "-01-01"}
	if region := r.URL.Query().Get("region_id"); region != "" {
		if len(region) != 26 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Wilayah tidak valid.")
			return
		}
		where += " AND (region_id IS NULL OR region_id=?)"
		args = append(args, region)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM holidays WHERE "+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hari libur.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,holiday_date,name,holiday_type,region_id FROM holidays WHERE "+where+" ORDER BY holiday_date,name,id LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hari libur.")
		return
	}
	defer rows.Close()
	items := []holiday{}
	for rows.Next() {
		item, err := scanHoliday(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hari libur.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hari libur.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}
func (a adminAPI) calendarHolidays(w http.ResponseWriter, r *http.Request) {
	state, err := a.auth.Current(r.Context(), cookieToken(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	if state.SelectedTenantID == "" {
		writeAPIError(w, r, 403, "TENANT_REQUIRED", "Pilih perusahaan terlebih dahulu.")
		return
	}
	var zone string
	for _, tenant := range state.Tenants {
		if tenant.ID == state.SelectedTenantID {
			zone = tenant.Timezone
			break
		}
	}
	if zone == "" {
		writeAPIError(w, r, 403, "TENANT_REQUIRED", "Perusahaan tidak tersedia.")
		return
	}
	year, ok := holidayYear(w, r, zone)
	if !ok {
		return
	}
	var region sql.NullString
	_ = a.db.QueryRowContext(r.Context(), `SELECT a.region_id FROM tenant_memberships m JOIN employee_assignments a ON a.tenant_id=m.tenant_id AND a.employee_id=m.employee_id AND a.current_flag=TRUE WHERE m.tenant_id=? AND m.user_id=?`, state.SelectedTenantID, state.User.ID).Scan(&region)
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,holiday_date,name,holiday_type,region_id FROM holidays WHERE tenant_id=? AND holiday_date>=? AND holiday_date<? AND (region_id IS NULL OR region_id=?) ORDER BY holiday_date,name,id`, state.SelectedTenantID, strconv.Itoa(year)+"-01-01", strconv.Itoa(year+1)+"-01-01", region)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hari libur.")
		return
	}
	defer rows.Close()
	items := []holiday{}
	for rows.Next() {
		item, err := scanHoliday(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hari libur.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hari libur.")
		return
	}
	writeAPIData(w, 200, items, nil)
}
func (a adminAPI) createHoliday(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.ubah")
	if !ok {
		return
	}
	var item holiday
	if !readJSON(w, r, &item) {
		return
	}
	item.ID = ""
	if !validHoliday(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data hari libur tidak valid.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat hari libur.")
		return
	}
	item.ID = id
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan hari libur.")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO holidays (tenant_id,id,holiday_date,name,holiday_type,region_id) VALUES (?,?,?,?,?,?)`, c.tenant.ID, id, item.Date, item.Name, item.Type, item.RegionID); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Wilayah tidak tersedia atau data hari libur tidak valid.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "holidays", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan hari libur.")
		return
	}
	writeAPIData(w, 201, item, nil)
}
func (a adminAPI) updateHoliday(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	old, err := a.readHoliday(r, c.tenant.ID, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Hari libur tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hari libur.")
		return
	}
	item := old
	if !readJSON(w, r, &item) {
		return
	}
	item.ID = id
	if !validHoliday(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data hari libur tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan hari libur.")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `UPDATE holidays SET holiday_date=?,name=?,holiday_type=?,region_id=? WHERE tenant_id=? AND id=?`, item.Date, item.Name, item.Type, item.RegionID, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Wilayah tidak tersedia atau data hari libur tidak valid.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "holidays", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan hari libur.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
func (a adminAPI) deleteHoliday(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	old, err := a.readHoliday(r, c.tenant.ID, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Hari libur tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hari libur.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus hari libur.")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM holidays WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus hari libur.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "delete", "holidays", id, old, nil); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus hari libur.")
		return
	}
	w.WriteHeader(204)
}
