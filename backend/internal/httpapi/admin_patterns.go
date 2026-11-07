package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"digaji.id/hris/internal/schedule"
	"github.com/go-chi/chi/v5"
)

type patternSlot struct {
	WeekNo          int     `json:"week_no"`
	Weekday         int     `json:"weekday"`
	DayType         string  `json:"day_type"`
	ShiftTemplateID *string `json:"shift_template_id"`
}
type schedulePattern struct {
	ID          string        `json:"id"`
	Code        string        `json:"code"`
	Name        string        `json:"name"`
	PatternType string        `json:"pattern_type"`
	Version     int           `json:"version"`
	IsPinned    bool          `json:"is_pinned"`
	PinOrder    *int          `json:"pin_order"`
	IsActive    bool          `json:"is_active"`
	Slots       []patternSlot `json:"slots,omitempty"`
}

func (a adminAPI) mountSchedulePatterns(r chi.Router, config Options) {
	r.Get("/schedule-patterns", a.listPatterns)
	r.With(checkOrigin(config)).Post("/schedule-patterns", a.createPattern)
	r.Get("/schedule-patterns/{id}", a.getPattern)
	r.With(checkOrigin(config)).Patch("/schedule-patterns/{id}", a.updatePattern)
	r.With(checkOrigin(config)).Delete("/schedule-patterns/{id}", a.deactivatePattern)
	r.Get("/schedule-patterns/{id}/slots", a.getPatternSlots)
	r.With(checkOrigin(config)).Put("/schedule-patterns/{id}/slots", a.replacePatternSlots)
	r.With(checkOrigin(config)).Post("/schedule-patterns/{id}/duplicate", a.duplicatePattern)
	r.With(checkOrigin(config)).Post("/schedule-patterns/{id}/preview", a.previewPattern)
}
func scanPattern(row interface{ Scan(...any) error }) (schedulePattern, error) {
	var item schedulePattern
	var pin sql.NullInt64
	err := row.Scan(&item.ID, &item.Code, &item.Name, &item.PatternType, &item.Version, &item.IsPinned, &pin, &item.IsActive)
	if pin.Valid {
		value := int(pin.Int64)
		item.PinOrder = &value
	}
	return item, err
}
func (a adminAPI) readPattern(r *http.Request, tenantID, id string, withSlots bool) (schedulePattern, error) {
	item, err := scanPattern(a.db.QueryRowContext(r.Context(), `SELECT id,code,name,pattern_type,version,is_pinned,pin_order,is_active FROM schedule_patterns WHERE tenant_id=? AND id=?`, tenantID, id))
	if err != nil || !withSlots {
		return item, err
	}
	item.Slots, err = a.readPatternSlots(r.Context(), tenantID, id)
	return item, err
}
func (a adminAPI) readPatternSlots(ctx context.Context, tenantID, id string) ([]patternSlot, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT week_no,weekday,day_type,shift_template_id FROM schedule_pattern_slots WHERE tenant_id=? AND pattern_id=? ORDER BY week_no,weekday`, tenantID, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []patternSlot{}
	for rows.Next() {
		var item patternSlot
		var shift sql.NullString
		if err := rows.Scan(&item.WeekNo, &item.Weekday, &item.DayType, &shift); err != nil {
			return nil, err
		}
		if shift.Valid {
			item.ShiftTemplateID = &shift.String
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func validPattern(item *schedulePattern) bool {
	item.Code = strings.ToUpper(strings.TrimSpace(item.Code))
	item.Name = strings.TrimSpace(item.Name)
	if !nodeCode.MatchString(item.Code) || item.Name == "" || len([]rune(item.Name)) > 160 || item.PatternType != "weekly" && item.PatternType != "monthly" {
		return false
	}
	if item.IsPinned {
		if item.PinOrder == nil || *item.PinOrder < 1 || *item.PinOrder > 32767 {
			return false
		}
	} else if item.PinOrder != nil {
		return false
	}
	return true
}
func validatePatternSlots(patternType string, items []patternSlot) bool {
	weeks := 1
	if patternType == "monthly" {
		weeks = 5
	} else if patternType != "weekly" {
		return false
	}
	if len(items) != weeks*7 {
		return false
	}
	seen := map[int]bool{}
	for _, item := range items {
		if item.WeekNo < 1 || item.WeekNo > weeks || item.Weekday < 1 || item.Weekday > 7 {
			return false
		}
		key := item.WeekNo*10 + item.Weekday
		if seen[key] {
			return false
		}
		seen[key] = true
		if item.DayType == "off" {
			if item.ShiftTemplateID != nil {
				return false
			}
		} else if item.DayType == "shift" {
			if item.ShiftTemplateID == nil || len(*item.ShiftTemplateID) != 26 {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
func (a adminAPI) validatePatternShifts(ctx context.Context, tenantID string, items []patternSlot) bool {
	unique := map[string]bool{}
	for _, item := range items {
		if item.ShiftTemplateID != nil {
			unique[*item.ShiftTemplateID] = true
		}
	}
	if len(unique) == 0 {
		return true
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(unique)), ",")
	args := []any{tenantID}
	for id := range unique {
		args = append(args, id)
	}
	var count int
	return a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM shift_templates WHERE tenant_id=? AND is_active=TRUE AND id IN (`+marks+`)`, args...).Scan(&count) == nil && count == len(unique)
}
func insertPatternSlots(ctx context.Context, tx *sql.Tx, tenantID, patternID string, items []patternSlot) error {
	for _, item := range items {
		id, err := ids.New()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schedule_pattern_slots (tenant_id,id,pattern_id,week_no,weekday,day_type,shift_template_id) VALUES (?,?,?,?,?,?,?)`, tenantID, id, patternID, item.WeekNo, item.Weekday, item.DayType, item.ShiftTemplateID); err != nil {
			return err
		}
	}
	return nil
}
func (a adminAPI) listPatterns(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.lihat")
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
	if patternType := r.URL.Query().Get("pattern_type"); patternType != "" {
		if patternType != "weekly" && patternType != "monthly" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jenis pola tidak valid.")
			return
		}
		where += " AND pattern_type=?"
		args = append(args, patternType)
	}
	if active := r.URL.Query().Get("active"); active != "" {
		if active != "true" && active != "false" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter active tidak valid.")
			return
		}
		where += " AND is_active=?"
		args = append(args, active == "true")
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM schedule_patterns WHERE "+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pola.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,code,name,pattern_type,version,is_pinned,pin_order,is_active FROM schedule_patterns WHERE "+where+" ORDER BY is_pinned DESC,pin_order,name,id LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pola.")
		return
	}
	defer rows.Close()
	items := []schedulePattern{}
	for rows.Next() {
		item, err := scanPattern(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pola.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pola.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}
func (a adminAPI) getPattern(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.lihat")
	if !ok {
		return
	}
	item, err := a.readPattern(r, c.tenant.ID, chi.URLParam(r, "id"), true)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pola tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pola.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
func (a adminAPI) createPatternData(w http.ResponseWriter, r *http.Request, c adminContext, item schedulePattern) {
	item.Version = 1
	item.ID = ""
	if !validPattern(&item) || !validatePatternSlots(item.PatternType, item.Slots) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Pola harus berisi 7 atau 35 slot yang lengkap.")
		return
	}
	if !a.validatePatternShifts(r.Context(), c.tenant.ID, item.Slots) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Shift pada pola tidak aktif atau di luar tenant.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat pola.")
		return
	}
	item.ID = id
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pola.")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO schedule_patterns (tenant_id,id,code,name,pattern_type,version,is_pinned,pin_order,is_active) VALUES (?,?,?,?,?,?,?,?,?)`, c.tenant.ID, id, item.Code, item.Name, item.PatternType, 1, item.IsPinned, item.PinOrder, item.IsActive); err != nil {
		writeAPIError(w, r, 409, "CONFLICT", "Kode atau urutan pin sudah dipakai.")
		return
	}
	if err = insertPatternSlots(r.Context(), tx, c.tenant.ID, id, item.Slots); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Gagal menyimpan slot pola.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "schedule_patterns", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pola.")
		return
	}
	writeAPIData(w, 201, item, nil)
}
func (a adminAPI) createPattern(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.buat")
	if !ok {
		return
	}
	item := schedulePattern{IsActive: true}
	if !readJSON(w, r, &item) {
		return
	}
	a.createPatternData(w, r, c, item)
}
func (a adminAPI) updatePattern(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	old, err := a.readPattern(r, c.tenant.ID, id, false)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pola tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pola.")
		return
	}
	var body struct {
		Version  int     `json:"version"`
		Code     *string `json:"code"`
		Name     *string `json:"name"`
		IsPinned *bool   `json:"is_pinned"`
		PinOrder *int    `json:"pin_order"`
		IsActive *bool   `json:"is_active"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Version != old.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pola sudah berubah; muat ulang sebelum menyimpan.")
		return
	}
	item := old
	if body.Code != nil {
		item.Code = *body.Code
	}
	if body.Name != nil {
		item.Name = *body.Name
	}
	if body.IsPinned != nil {
		item.IsPinned = *body.IsPinned
	}
	if body.PinOrder != nil {
		item.PinOrder = body.PinOrder
	}
	if body.IsActive != nil {
		item.IsActive = *body.IsActive
	}
	if !item.IsPinned {
		item.PinOrder = nil
	}
	if !validPattern(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Metadata pola tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pola.")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE schedule_patterns SET code=?,name=?,is_pinned=?,pin_order=?,is_active=?,version=version+1 WHERE tenant_id=? AND id=? AND version=?`, item.Code, item.Name, item.IsPinned, item.PinOrder, item.IsActive, c.tenant.ID, id, body.Version)
	if err != nil {
		writeAPIError(w, r, 409, "CONFLICT", "Kode atau urutan pin sudah dipakai.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pola sudah berubah.")
		return
	}
	item.Version++
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "schedule_patterns", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pola.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
func (a adminAPI) deactivatePattern(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.hapus")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	old, err := a.readPattern(r, c.tenant.ID, id, false)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pola tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pola.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan pola.")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE schedule_patterns SET is_active=FALSE,is_pinned=FALSE,pin_order=NULL,version=version+1 WHERE tenant_id=? AND id=? AND version=?`, c.tenant.ID, id, old.Version)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan pola.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pola sudah berubah.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "delete", "schedule_patterns", id, old, map[string]any{"is_active": false}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan pola.")
		return
	}
	w.WriteHeader(204)
}
func (a adminAPI) getPatternSlots(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.lihat")
	if !ok {
		return
	}
	item, err := a.readPattern(r, c.tenant.ID, chi.URLParam(r, "id"), true)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pola tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca slot pola.")
		return
	}
	writeAPIData(w, 200, map[string]any{"version": item.Version, "slots": item.Slots}, nil)
}
func (a adminAPI) replacePatternSlots(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	old, err := a.readPattern(r, c.tenant.ID, id, true)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pola tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pola.")
		return
	}
	var body struct {
		Version int           `json:"version"`
		Slots   []patternSlot `json:"slots"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Version != old.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pola sudah berubah.")
		return
	}
	if !validatePatternSlots(old.PatternType, body.Slots) || !a.validatePatternShifts(r.Context(), c.tenant.ID, body.Slots) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Slot tidak lengkap atau shift tidak aktif.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan slot.")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE schedule_patterns SET version=version+1 WHERE tenant_id=? AND id=? AND version=? AND is_active=TRUE`, c.tenant.ID, id, body.Version)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan slot.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pola sudah berubah atau tidak aktif.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM schedule_pattern_slots WHERE tenant_id=? AND pattern_id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengganti slot.")
		return
	}
	if err = insertPatternSlots(r.Context(), tx, c.tenant.ID, id, body.Slots); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Gagal menyimpan slot pola.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "replace_slots", "schedule_patterns", id, old.Slots, body.Slots); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan slot.")
		return
	}
	writeAPIData(w, 200, map[string]any{"version": body.Version + 1, "slots": body.Slots}, nil)
}
func (a adminAPI) duplicatePattern(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.buat")
	if !ok {
		return
	}
	old, err := a.readPattern(r, c.tenant.ID, chi.URLParam(r, "id"), true)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pola tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pola.")
		return
	}
	var body struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	old.Code = body.Code
	old.Name = body.Name
	old.IsPinned = false
	old.PinOrder = nil
	old.IsActive = true
	a.createPatternData(w, r, c, old)
}
func (a adminAPI) previewPattern(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.lihat")
	if !ok {
		return
	}
	item, err := a.readPattern(r, c.tenant.ID, chi.URLParam(r, "id"), true)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pola tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pola.")
		return
	}
	var body struct {
		Month string `json:"month"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	month, err := time.Parse("2006-01", body.Month)
	if err != nil || month.Format("2006-01") != body.Month {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Bulan harus YYYY-MM.")
		return
	}
	bySlot := map[schedule.Slot]patternSlot{}
	for _, slot := range item.Slots {
		bySlot[schedule.Slot{WeekNo: slot.WeekNo, Weekday: slot.Weekday}] = slot
	}
	type previewDay struct {
		WorkDate        string  `json:"work_date"`
		WeekNo          int     `json:"week_no"`
		Weekday         int     `json:"weekday"`
		DayType         string  `json:"day_type"`
		ShiftTemplateID *string `json:"shift_template_id"`
	}
	days := []previewDay{}
	for date := month; date.Month() == month.Month(); date = date.AddDate(0, 0, 1) {
		key, _ := schedule.SlotForDate(item.PatternType, date)
		slot, exists := bySlot[key]
		if !exists {
			writeAPIError(w, r, 409, "PATTERN_INCOMPLETE", "Pola belum memiliki seluruh slot.")
			return
		}
		days = append(days, previewDay{WorkDate: date.Format("2006-01-02"), WeekNo: key.WeekNo, Weekday: key.Weekday, DayType: slot.DayType, ShiftTemplateID: slot.ShiftTemplateID})
	}
	writeAPIData(w, 200, map[string]any{"pattern_id": item.ID, "pattern_version": item.Version, "month": body.Month, "days": days}, nil)
}
