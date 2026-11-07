package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type shiftTemplate struct {
	ID                           string `json:"id"`
	Code                         string `json:"code"`
	Name                         string `json:"name"`
	StartTime                    string `json:"start_time"`
	EndTime                      string `json:"end_time"`
	CrossesMidnight              bool   `json:"crosses_midnight"`
	ClockInWindowStartTime       string `json:"clock_in_window_start_time"`
	ClockInWindowStartDayOffset  int    `json:"clock_in_window_start_day_offset"`
	ClockInWindowEndTime         string `json:"clock_in_window_end_time"`
	ClockInWindowEndDayOffset    int    `json:"clock_in_window_end_day_offset"`
	ClockOutWindowStartTime      string `json:"clock_out_window_start_time"`
	ClockOutWindowStartDayOffset int    `json:"clock_out_window_start_day_offset"`
	ClockOutWindowEndTime        string `json:"clock_out_window_end_time"`
	ClockOutWindowEndDayOffset   int    `json:"clock_out_window_end_day_offset"`
	LateToleranceMin             int    `json:"late_tolerance_min"`
	BreakMinutes                 int    `json:"break_minutes"`
	TargetWorkMinutes            *int   `json:"target_work_minutes"`
	FlexTime                     bool   `json:"flex_time"`
	FlexLocation                 bool   `json:"flex_location"`
	RequireLocation              bool   `json:"require_location"`
	RequireSelfie                bool   `json:"require_selfie"`
	RequireFaceRecognition       bool   `json:"require_face_recognition"`
	IsPinned                     bool   `json:"is_pinned"`
	PinOrder                     *int   `json:"pin_order"`
	ColorHex                     string `json:"color_hex"`
	IsActive                     bool   `json:"is_active"`
}

const shiftColumns = "code,name,start_time,end_time,crosses_midnight,clock_in_window_start_time,clock_in_window_start_day_offset,clock_in_window_end_time,clock_in_window_end_day_offset,clock_out_window_start_time,clock_out_window_start_day_offset,clock_out_window_end_time,clock_out_window_end_day_offset,late_tolerance_min,break_minutes,target_work_minutes,flex_time,flex_location,require_location,require_selfie,require_face_recognition,is_pinned,pin_order,color_hex,is_active"

var shiftColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (a adminAPI) mountScheduleMasters(r chi.Router, config Options) {
	r.Get("/shift-templates", a.listShiftTemplates)
	r.With(checkOrigin(config)).Post("/shift-templates", a.createShiftTemplate)
	r.Get("/shift-templates/{id}", a.getShiftTemplate)
	r.With(checkOrigin(config)).Patch("/shift-templates/{id}", a.updateShiftTemplate)
	r.With(checkOrigin(config)).Delete("/shift-templates/{id}", a.deactivateShiftTemplate)
	r.With(checkOrigin(config)).Post("/shift-templates/{id}/duplicate", a.duplicateShiftTemplate)
	a.mountSchedulePatterns(r, config)
	a.mountWorkSchedules(r, config)
	r.Get("/holidays", a.listHolidays)
	r.With(checkOrigin(config)).Post("/holidays", a.createHoliday)
	r.With(checkOrigin(config)).Patch("/holidays/{id}", a.updateHoliday)
	r.With(checkOrigin(config)).Delete("/holidays/{id}", a.deleteHoliday)
}

func shiftArgs(s shiftTemplate) []any {
	return []any{s.Code, s.Name, s.StartTime, s.EndTime, s.CrossesMidnight, s.ClockInWindowStartTime, s.ClockInWindowStartDayOffset, s.ClockInWindowEndTime, s.ClockInWindowEndDayOffset, s.ClockOutWindowStartTime, s.ClockOutWindowStartDayOffset, s.ClockOutWindowEndTime, s.ClockOutWindowEndDayOffset, s.LateToleranceMin, s.BreakMinutes, s.TargetWorkMinutes, s.FlexTime, s.FlexLocation, s.RequireLocation, s.RequireSelfie, s.RequireFaceRecognition, s.IsPinned, s.PinOrder, s.ColorHex, s.IsActive}
}
func scanShift(row interface{ Scan(...any) error }) (shiftTemplate, error) {
	var s shiftTemplate
	var target, pin sql.NullInt64
	err := row.Scan(&s.ID, &s.Code, &s.Name, &s.StartTime, &s.EndTime, &s.CrossesMidnight, &s.ClockInWindowStartTime, &s.ClockInWindowStartDayOffset, &s.ClockInWindowEndTime, &s.ClockInWindowEndDayOffset, &s.ClockOutWindowStartTime, &s.ClockOutWindowStartDayOffset, &s.ClockOutWindowEndTime, &s.ClockOutWindowEndDayOffset, &s.LateToleranceMin, &s.BreakMinutes, &target, &s.FlexTime, &s.FlexLocation, &s.RequireLocation, &s.RequireSelfie, &s.RequireFaceRecognition, &s.IsPinned, &pin, &s.ColorHex, &s.IsActive)
	if target.Valid {
		value := int(target.Int64)
		s.TargetWorkMinutes = &value
	}
	if pin.Valid {
		value := int(pin.Int64)
		s.PinOrder = &value
	}
	return s, err
}
func (a adminAPI) readShift(r *http.Request, tenantID, id string) (shiftTemplate, error) {
	return scanShift(a.db.QueryRowContext(r.Context(), "SELECT id,"+shiftColumns+" FROM shift_templates WHERE tenant_id=? AND id=?", tenantID, id))
}
func normalizeClock(value string) (string, int, bool) {
	if len(value) == 5 {
		value += ":00"
	}
	parsed, err := time.Parse("15:04:05", value)
	if err != nil || parsed.Second() != 0 {
		return "", 0, false
	}
	return parsed.Format("15:04:05"), parsed.Hour()*3600 + parsed.Minute()*60 + parsed.Second(), true
}
func validShift(s *shiftTemplate) bool {
	s.Code = strings.ToUpper(strings.TrimSpace(s.Code))
	s.Name = strings.TrimSpace(s.Name)
	if !nodeCode.MatchString(s.Code) || s.Name == "" || len([]rune(s.Name)) > 160 {
		return false
	}
	if s.ColorHex == "" {
		s.ColorHex = "#3B82F6"
	}
	if !shiftColor.MatchString(s.ColorHex) {
		return false
	}
	times := []*string{&s.StartTime, &s.EndTime, &s.ClockInWindowStartTime, &s.ClockInWindowEndTime, &s.ClockOutWindowStartTime, &s.ClockOutWindowEndTime}
	seconds := make([]int, len(times))
	for i, value := range times {
		normalized, sec, ok := normalizeClock(*value)
		if !ok {
			return false
		}
		*value = normalized
		seconds[i] = sec
	}
	if s.CrossesMidnight && seconds[1] > seconds[0] || !s.CrossesMidnight && seconds[1] <= seconds[0] {
		return false
	}
	offsets := []int{s.ClockInWindowStartDayOffset, s.ClockInWindowEndDayOffset, s.ClockOutWindowStartDayOffset, s.ClockOutWindowEndDayOffset}
	for _, offset := range offsets {
		if offset < -1 || offset > 2 {
			return false
		}
	}
	if offsets[1]*86400+seconds[3] < offsets[0]*86400+seconds[2] || offsets[3]*86400+seconds[5] < offsets[2]*86400+seconds[4] {
		return false
	}
	if s.LateToleranceMin < 0 || s.LateToleranceMin > 1440 || s.BreakMinutes < 0 || s.BreakMinutes > 1440 {
		return false
	}
	if s.TargetWorkMinutes != nil && (*s.TargetWorkMinutes <= 0 || *s.TargetWorkMinutes > 2880) {
		return false
	}
	if s.IsPinned {
		if s.PinOrder == nil || *s.PinOrder < 1 || *s.PinOrder > 32767 {
			return false
		}
	} else if s.PinOrder != nil {
		return false
	}
	return true
}

func (a adminAPI) listShiftTemplates(w http.ResponseWriter, r *http.Request) {
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
	if active := r.URL.Query().Get("active"); active != "" {
		if active != "true" && active != "false" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter active tidak valid.")
			return
		}
		where += " AND is_active=?"
		args = append(args, active == "true")
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM shift_templates WHERE "+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,"+shiftColumns+" FROM shift_templates WHERE "+where+" ORDER BY is_pinned DESC,pin_order,name,id LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
		return
	}
	defer rows.Close()
	items := []shiftTemplate{}
	for rows.Next() {
		item, err := scanShift(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}
func (a adminAPI) getShiftTemplate(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.lihat")
	if !ok {
		return
	}
	item, err := a.readShift(r, c.tenant.ID, chi.URLParam(r, "id"))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Shift tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
func (a adminAPI) saveNewShift(w http.ResponseWriter, r *http.Request, c adminContext, item shiftTemplate) {
	if !validShift(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data shift atau jendela waktu tidak valid.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat shift.")
		return
	}
	item.ID = id
	columns := strings.Split(shiftColumns, ",")
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(columns)+2), ",")
	args := append([]any{c.tenant.ID, id}, shiftArgs(item)...)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan shift.")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO shift_templates (tenant_id,id,"+shiftColumns+") VALUES ("+placeholders+")", args...); err != nil {
		writeAPIError(w, r, 409, "CONFLICT", "Kode atau urutan pin sudah dipakai, atau data shift tidak valid.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "shift_templates", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan shift.")
		return
	}
	writeAPIData(w, 201, item, nil)
}
func (a adminAPI) createShiftTemplate(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.buat")
	if !ok {
		return
	}
	item := shiftTemplate{IsActive: true}
	if !readJSON(w, r, &item) {
		return
	}
	item.ID = ""
	a.saveNewShift(w, r, c, item)
}
func (a adminAPI) updateShiftTemplate(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	old, err := a.readShift(r, c.tenant.ID, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Shift tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
		return
	}
	item := old
	if !readJSON(w, r, &item) {
		return
	}
	item.ID = id
	if !item.IsPinned {
		item.PinOrder = nil
	}
	if !validShift(&item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data shift atau jendela waktu tidak valid.")
		return
	}
	columns := strings.Split(shiftColumns, ",")
	sets := make([]string, len(columns))
	for i, column := range columns {
		sets[i] = column + "=?"
	}
	args := append(shiftArgs(item), c.tenant.ID, id)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan shift.")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "UPDATE shift_templates SET "+strings.Join(sets, ",")+" WHERE tenant_id=? AND id=?", args...); err != nil {
		writeAPIError(w, r, 409, "CONFLICT", "Kode atau urutan pin sudah dipakai, atau data shift tidak valid.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "shift_templates", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan shift.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
func (a adminAPI) deactivateShiftTemplate(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.hapus")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	old, err := a.readShift(r, c.tenant.ID, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Shift tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan shift.")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `UPDATE shift_templates SET is_active=FALSE,is_pinned=FALSE,pin_order=NULL WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan shift.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "delete", "shift_templates", id, old, map[string]any{"is_active": false}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan shift.")
		return
	}
	w.WriteHeader(204)
}
func (a adminAPI) duplicateShiftTemplate(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.buat")
	if !ok {
		return
	}
	old, err := a.readShift(r, c.tenant.ID, chi.URLParam(r, "id"))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Shift tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
		return
	}
	var body struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	old.ID = ""
	old.Code = body.Code
	old.Name = body.Name
	old.IsPinned = false
	old.PinOrder = nil
	old.IsActive = true
	a.saveNewShift(w, r, c, old)
}
