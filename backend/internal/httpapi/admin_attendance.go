package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) mountAdminAttendance(r chi.Router, config Options) {
	r.Get("/attendance/today", a.adminAttendanceToday)
	r.Get("/attendance/not-yet", a.adminAttendanceNotYet)
	r.Get("/attendance/corrections", a.listAdminAttendanceCorrections)
	r.Get("/attendance/corrections/{id}", a.getAdminAttendanceCorrection)
	r.With(checkOrigin(config)).Post("/attendance/corrections", a.createAdminAttendanceCorrection)
	r.Get("/attendance/events", a.adminAttendanceEvents)
	r.Get("/attendance/events/{id}", a.adminAttendanceEvent)
	r.Get("/attendance/daily", a.adminAttendanceDaily)
	r.Get("/attendance/summary", a.adminAttendanceSummary)
	r.With(checkOrigin(config)).Post("/attendance/recalculate", a.adminAttendanceRecalculate)
}

const attendanceScopedJoin = ` FROM shift_instances i JOIN employees e ON e.tenant_id=i.tenant_id AND e.id=i.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`

func attendanceDate(w http.ResponseWriter, r *http.Request, key, fallback string) (string, bool) {
	value := r.URL.Query().Get(key)
	if value == "" {
		value = fallback
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil || date.Format("2006-01-02") != value {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal presensi tidak valid.")
		return "", false
	}
	return value, true
}

func attendancePeriod(w http.ResponseWriter, r *http.Request, zone *time.Location) (string, string, bool) {
	today := time.Now().In(zone)
	from, ok := attendanceDate(w, r, "from", today.AddDate(0, 0, -30).Format("2006-01-02"))
	if !ok {
		return "", "", false
	}
	to, ok := attendanceDate(w, r, "to", today.Format("2006-01-02"))
	if !ok {
		return "", "", false
	}
	start, _ := time.Parse("2006-01-02", from)
	end, _ := time.Parse("2006-01-02", to)
	if end.Before(start) || end.Sub(start) > 92*24*time.Hour {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Rentang presensi maksimal 93 hari.")
		return "", "", false
	}
	return from, to, true
}

type adminAttendanceRow struct {
	ShiftInstanceID string     `json:"shift_instance_id"`
	EmployeeID      string     `json:"employee_id"`
	EmployeeName    string     `json:"employee_name"`
	WorkDate        string     `json:"work_date"`
	ExpectedStart   time.Time  `json:"expected_start"`
	ExpectedEnd     time.Time  `json:"expected_end"`
	FirstIn         *time.Time `json:"first_in"`
	LastOut         *time.Time `json:"last_out"`
	WorkedMinutes   int        `json:"worked_minutes"`
	LateMinutes     int        `json:"late_minutes"`
	Status          string     `json:"status"`
	Category        string     `json:"category,omitempty"`
}

func scanAdminAttendance(row interface{ Scan(...any) error }) (adminAttendanceRow, error) {
	var item adminAttendanceRow
	var day time.Time
	var first, last sql.NullTime
	var status, category sql.NullString
	err := row.Scan(&item.ShiftInstanceID, &item.EmployeeID, &item.EmployeeName, &day, &item.ExpectedStart, &item.ExpectedEnd, &first, &last, &item.WorkedMinutes, &item.LateMinutes, &status, &category)
	if err != nil {
		return item, err
	}
	item.WorkDate = day.Format("2006-01-02")
	item.Status = "absent"
	if status.Valid {
		item.Status = status.String
	}
	if first.Valid {
		item.FirstIn = &first.Time
	}
	if last.Valid {
		item.LastOut = &last.Time
	}
	if category.Valid {
		item.Category = category.String
	}
	return item, nil
}

const adminAttendanceColumns = `i.id,i.employee_id,e.display_name,i.work_date,i.expected_start,i.expected_end,d.first_in,d.last_out,COALESCE(d.worked_minutes,0),COALESCE(d.late_minutes,0),d.status,CASE WHEN EXISTS (SELECT 1 FROM leave_requests l WHERE l.tenant_id=i.tenant_id AND l.employee_id=i.employee_id AND l.status='approved' AND i.work_date BETWEEN l.start_date AND l.end_date) THEN 'izin_cuti' WHEN EXISTS (SELECT 1 FROM holidays h WHERE h.tenant_id=i.tenant_id AND h.holiday_date=i.work_date AND (h.region_id IS NULL OR h.region_id=a.region_id)) THEN 'libur' WHEN EXISTS (SELECT 1 FROM attendance_corrections c WHERE c.tenant_id=i.tenant_id AND c.shift_instance_id=i.id AND c.status IN ('in_review','returned')) THEN 'menunggu_koreksi' WHEN i.clock_in_window_start_at>UTC_TIMESTAMP(6) THEN 'belum_waktunya' WHEN d.first_in IS NULL THEN 'belum_absen' ELSE NULL END`
const adminAttendanceDailyJoin = attendanceScopedJoin + ` LEFT JOIN attendance_daily d ON d.tenant_id=i.tenant_id AND d.shift_instance_id=i.id`

func (a adminAPI) attendanceRows(w http.ResponseWriter, r *http.Request, c adminContext, from, to string, notYet bool) {
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	where := `i.tenant_id=? AND i.status<>'cancelled' AND i.work_date>=? AND i.work_date<=? AND ` + filter
	args := append([]any{c.tenant.ID, from, to}, scopeArgs...)
	if id := r.URL.Query().Get("employee_id"); id != "" {
		if !fileIDPattern.MatchString(id) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID karyawan tidak valid.")
			return
		}
		where += ` AND i.employee_id=?`
		args = append(args, id)
	}
	for _, entry := range []struct{ key, column string }{{"office_id", "a.office_id"}, {"department_id", "a.department_id"}} {
		if value := r.URL.Query().Get(entry.key); value != "" {
			if !fileIDPattern.MatchString(value) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter organisasi tidak valid.")
				return
			}
			where += ` AND ` + entry.column + `=?`
			args = append(args, value)
		}
	}
	if status := r.URL.Query().Get("status"); status != "" {
		switch status {
		case "absent", "incomplete", "present", "late":
			where += ` AND COALESCE(d.status,'absent')=?`
			args = append(args, status)
		default:
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter status presensi tidak valid.")
			return
		}
	}
	if notYet {
		where += ` AND d.first_in IS NULL`
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+adminAttendanceDailyJoin+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca rekap presensi.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+adminAttendanceColumns+adminAttendanceDailyJoin+` WHERE `+where+` ORDER BY i.work_date DESC,e.display_name,i.id LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca rekap presensi.")
		return
	}
	defer rows.Close()
	items := []adminAttendanceRow{}
	for rows.Next() {
		item, e := scanAdminAttendance(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca rekap presensi.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca rekap presensi.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) adminAttendanceToday(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "absensi.lihat")
	if !ok {
		return
	}
	zone, _ := time.LoadLocation(c.tenant.Timezone)
	if zone == nil {
		zone = time.UTC
	}
	day, ok := attendanceDate(w, r, "date", time.Now().In(zone).Format("2006-01-02"))
	if !ok {
		return
	}
	a.attendanceRows(w, r, c, day, day, false)
}
func (a adminAPI) adminAttendanceNotYet(w http.ResponseWriter, r *http.Request) {
	a.attendanceNotYet(w, r)
}
func (a adminAPI) adminAttendanceDaily(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "absensi.lihat")
	if !ok {
		return
	}
	zone, _ := time.LoadLocation(c.tenant.Timezone)
	if zone == nil {
		zone = time.UTC
	}
	from, to, ok := attendancePeriod(w, r, zone)
	if !ok {
		return
	}
	a.attendanceRows(w, r, c, from, to, false)
}
func (a adminAPI) adminAttendanceSummary(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "absensi.lihat")
	if !ok {
		return
	}
	zone, _ := time.LoadLocation(c.tenant.Timezone)
	if zone == nil {
		zone = time.UTC
	}
	from, to, ok := attendancePeriod(w, r, zone)
	if !ok {
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	args := append([]any{c.tenant.ID, from, to}, scopeArgs...)
	where := ` WHERE i.tenant_id=? AND i.work_date>=? AND i.work_date<=? AND i.status<>'cancelled' AND ` + filter
	for _, entry := range []struct{ key, column string }{{"employee_id", "i.employee_id"}, {"office_id", "a.office_id"}, {"department_id", "a.department_id"}} {
		if value := r.URL.Query().Get(entry.key); value != "" {
			if !fileIDPattern.MatchString(value) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter ringkasan tidak valid.")
				return
			}
			where += ` AND ` + entry.column + `=?`
			args = append(args, value)
		}
	}
	metrics := `COUNT(*),COALESCE(SUM(d.first_in IS NOT NULL AND d.last_out IS NOT NULL),0),COALESCE(SUM(d.late_minutes>0),0),COALESCE(SUM(d.status='incomplete'),0),COALESCE(SUM(d.first_in IS NULL AND d.last_out IS NULL),0),COALESCE(SUM(d.worked_minutes),0)`
	if group := r.URL.Query().Get("group_by"); group != "" {
		column := "i.employee_id"
		name := "e.display_name"
		if group == "department" {
			column = "a.department_id"
			name = "dep.name"
		} else if group != "employee" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "group_by harus employee atau department.")
			return
		}
		join := adminAttendanceDailyJoin
		if group == "department" {
			join += ` JOIN departments dep ON dep.tenant_id=a.tenant_id AND dep.id=a.department_id`
		}
		rows, err := a.db.QueryContext(r.Context(), `SELECT `+column+`,`+name+`,`+metrics+join+where+` GROUP BY `+column+`,`+name+` ORDER BY `+name+` LIMIT 1001`, args...)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ringkasan presensi.")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, label string
			var scheduled, present, late, incomplete, absent, worked int
			if err = rows.Scan(&id, &label, &scheduled, &present, &late, &incomplete, &absent, &worked); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ringkasan presensi.")
				return
			}
			items = append(items, map[string]any{"id": id, "name": label, "scheduled": scheduled, "present": present, "late": late, "incomplete": incomplete, "absent": absent, "worked_minutes": worked})
		}
		if rows.Err() != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ringkasan presensi.")
			return
		}
		if len(items) > 1000 {
			writeAPIError(w, r, 422, "TOO_MANY_RECORDS", "Maksimal 1000 grup.")
			return
		}
		writeAPIData(w, 200, map[string]any{"from": from, "to": to, "group_by": group, "items": items}, nil)
		return
	}
	var scheduled, present, late, incomplete, absent, worked int
	err := a.db.QueryRowContext(r.Context(), `SELECT `+metrics+adminAttendanceDailyJoin+where, args...).Scan(&scheduled, &present, &late, &incomplete, &absent, &worked)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ringkasan presensi.")
		return
	}
	writeAPIData(w, 200, map[string]any{"from": from, "to": to, "scheduled": scheduled, "present": present, "late": late, "incomplete": incomplete, "absent": absent, "worked_minutes": worked}, nil)
}

const adminEventColumns = `ev.id,ev.shift_instance_id,ev.event_type,ev.occurred_at,ev.device_time,ev.latitude,ev.longitude,ev.location_status,ev.timing_status,ev.selfie_file_id,ev.accuracy_m,ev.office_id,ev.geofence_id`
const adminEventJoin = ` FROM attendance_events ev JOIN shift_instances i ON i.tenant_id=ev.tenant_id AND i.id=ev.shift_instance_id JOIN employees e ON e.tenant_id=i.tenant_id AND e.id=i.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`

func (a adminAPI) adminAttendanceEvents(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "absensi.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	zone, _ := time.LoadLocation(c.tenant.Timezone)
	if zone == nil {
		zone = time.UTC
	}
	from, to, ok := attendancePeriod(w, r, zone)
	if !ok {
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	where := `ev.tenant_id=? AND i.work_date>=? AND i.work_date<=? AND ` + filter
	args := append([]any{c.tenant.ID, from, to}, scopeArgs...)
	if id := r.URL.Query().Get("employee_id"); id != "" {
		if !fileIDPattern.MatchString(id) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID karyawan tidak valid.")
			return
		}
		where += ` AND ev.employee_id=?`
		args = append(args, id)
	}
	for _, entry := range []struct{ key, column string }{{"event_type", "ev.event_type"}, {"timing_status", "ev.timing_status"}, {"location_status", "ev.location_status"}} {
		if value := r.URL.Query().Get(entry.key); value != "" {
			valid := false
			switch entry.key {
			case "event_type":
				valid = value == "clock_in" || value == "clock_out" || value == "correction_in" || value == "correction_out"
			case "timing_status":
				valid = value == "on_time" || value == "late" || value == "early_leave" || value == "out_of_window"
			case "location_status":
				valid = value == "inside" || value == "outside" || value == "flexible" || value == "unknown"
			}
			if !valid {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter event tidak valid.")
				return
			}
			where += ` AND ` + entry.column + `=?`
			args = append(args, value)
		}
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+adminEventJoin+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca event presensi.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+adminEventColumns+adminEventJoin+` WHERE `+where+` ORDER BY ev.occurred_at DESC,ev.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca event presensi.")
		return
	}
	defer rows.Close()
	items := []essAttendanceEvent{}
	for rows.Next() {
		item, e := scanEssEvent(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca event presensi.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca event presensi.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}
func (a adminAPI) adminAttendanceEvent(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "absensi.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Event tidak ditemukan.")
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	args := append([]any{c.tenant.ID, id}, scopeArgs...)
	item, err := scanEssEvent(a.db.QueryRowContext(r.Context(), `SELECT `+adminEventColumns+adminEventJoin+` WHERE ev.tenant_id=? AND ev.id=? AND `+filter, args...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Event tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca event presensi.")
		return
	}
	if item.SelfieFileID != nil {
		url := "/api/v1/files/" + *item.SelfieFileID + "/download"
		item.SelfieDownloadURL = &url
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) adminAttendanceRecalculate(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "absensi.koreksi")
	if !ok {
		return
	}
	var body struct {
		From       string `json:"from"`
		To         string `json:"to"`
		EmployeeID string `json:"employee_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	start, e1 := time.Parse("2006-01-02", body.From)
	end, e2 := time.Parse("2006-01-02", body.To)
	if e1 != nil || e2 != nil || start.Format("2006-01-02") != body.From || end.Format("2006-01-02") != body.To || end.Before(start) || end.Sub(start) > 31*24*time.Hour || (body.EmployeeID != "" && !fileIDPattern.MatchString(body.EmployeeID)) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Rentang maksimal 32 hari dan filter karyawan harus valid.")
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	where := `i.tenant_id=? AND i.work_date>=? AND i.work_date<=? AND i.status<>'cancelled' AND ` + filter
	args := append([]any{c.tenant.ID, body.From, body.To}, scopeArgs...)
	if body.EmployeeID != "" {
		where += ` AND i.employee_id=?`
		args = append(args, body.EmployeeID)
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+instanceColumns+instanceJoin+` WHERE `+where+` ORDER BY i.work_date,i.id`, args...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
		return
	}
	instances := []shiftInstance{}
	for rows.Next() {
		item, e := scanInstance(rows)
		if e != nil {
			rows.Close()
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
			return
		}
		instances = append(instances, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift.")
		return
	}
	if len(instances) > 1000 {
		writeAPIError(w, r, 422, "TOO_MANY_RECORDS", "Maksimal 1000 shift per permintaan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung ulang.")
		return
	}
	defer tx.Rollback()
	for _, item := range instances {
		if err = recalculateAttendanceDay(r.Context(), tx, c.tenant.ID, item); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung ulang.")
			return
		}
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "recalculate", "attendance_daily", "", nil, map[string]any{"from": body.From, "to": body.To, "count": len(instances)}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan rekap.")
		return
	}
	writeAPIData(w, 200, map[string]any{"recalculated": len(instances)}, nil)
}
