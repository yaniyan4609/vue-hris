package httpapi

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/auth"
	"github.com/go-chi/chi/v5"
)

type essAttendanceContext struct {
	state      auth.State
	employeeID string
	zone       *time.Location
}

func (a adminAPI) essAttendanceContext(w http.ResponseWriter, r *http.Request) (essAttendanceContext, bool) {
	state, err := a.auth.Current(r.Context(), cookieToken(r))
	if err != nil {
		writeAuthError(w, err)
		return essAttendanceContext{}, false
	}
	var selected auth.Tenant
	for _, tenant := range state.Tenants {
		if tenant.ID == state.SelectedTenantID {
			selected = tenant
			break
		}
	}
	if !selected.ESS {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Panel ESS tidak tersedia.")
		return essAttendanceContext{}, false
	}
	var employeeID string
	err = a.db.QueryRowContext(r.Context(), `SELECT m.employee_id FROM tenant_memberships m JOIN employees e ON e.tenant_id=m.tenant_id AND e.id=m.employee_id WHERE m.tenant_id=? AND m.user_id=? AND m.status='active' AND e.status IN ('active','probation','leave')`, state.SelectedTenantID, state.User.ID).Scan(&employeeID)
	if err == sql.ErrNoRows {
		writeAPIError(w, r, 404, "EMPLOYEE_NOT_FOUND", "Profil karyawan tidak ditemukan.")
		return essAttendanceContext{}, false
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca profil karyawan.")
		return essAttendanceContext{}, false
	}
	zone, err := time.LoadLocation(selected.Timezone)
	if err != nil {
		zone = time.UTC
	}
	return essAttendanceContext{state: state, employeeID: employeeID, zone: zone}, true
}

func (a adminAPI) mountEssAttendance(r chi.Router) {
	r.Get("/api/v1/ess/attendance/today", a.essAttendanceToday)
	r.Get("/api/v1/ess/attendance/history", a.essAttendanceHistory)
	r.Get("/api/v1/ess/attendance/calendar", a.essAttendanceCalendar)
	r.Get("/api/v1/ess/attendance/summary", a.essAttendanceSummary)
}

type essAttendanceEvent struct {
	ID                string     `json:"id"`
	ShiftInstanceID   string     `json:"shift_instance_id"`
	EventType         string     `json:"event_type"`
	OccurredAt        time.Time  `json:"occurred_at"`
	Latitude          *float64   `json:"latitude"`
	Longitude         *float64   `json:"longitude"`
	AccuracyM         *float64   `json:"accuracy_m"`
	OfficeID          *string    `json:"office_id"`
	GeofenceID        *string    `json:"geofence_id"`
	LocationStatus    *string    `json:"location_status"`
	TimingStatus      *string    `json:"timing_status"`
	SelfieFileID      *string    `json:"selfie_file_id"`
	SelfieDownloadURL *string    `json:"selfie_download_url,omitempty"`
	DeviceTime        *time.Time `json:"device_time"`
}

const essEventColumns = `id,shift_instance_id,event_type,occurred_at,device_time,latitude,longitude,location_status,timing_status,selfie_file_id,accuracy_m,office_id,geofence_id`

func scanEssEvent(row interface{ Scan(...any) error }) (essAttendanceEvent, error) {
	var item essAttendanceEvent
	var device sql.NullTime
	var lat, lng, accuracy sql.NullFloat64
	var location, timing, selfie, office, geofence sql.NullString
	err := row.Scan(&item.ID, &item.ShiftInstanceID, &item.EventType, &item.OccurredAt, &device, &lat, &lng, &location, &timing, &selfie, &accuracy, &office, &geofence)
	if device.Valid {
		item.DeviceTime = &device.Time
	}
	if lat.Valid {
		item.Latitude = &lat.Float64
	}
	if lng.Valid {
		item.Longitude = &lng.Float64
	}
	if accuracy.Valid {
		item.AccuracyM = &accuracy.Float64
	}
	if office.Valid {
		item.OfficeID = &office.String
	}
	if geofence.Valid {
		item.GeofenceID = &geofence.String
	}
	if location.Valid {
		item.LocationStatus = &location.String
	}
	if timing.Valid {
		item.TimingStatus = &timing.String
	}
	if selfie.Valid {
		item.SelfieFileID = &selfie.String
	}
	return item, err
}

func (a adminAPI) essAttendanceToday(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	now := time.Now().UTC()
	today := now.In(c.zone).Format("2006-01-02")
	yesterday := now.In(c.zone).AddDate(0, 0, -1).Format("2006-01-02")
	var instance *shiftInstance
	item, err := scanInstance(a.db.QueryRowContext(r.Context(), `SELECT `+instanceColumns+` FROM shift_instances i WHERE i.tenant_id=? AND i.employee_id=? AND i.status<>'cancelled' AND (i.work_date=? OR (i.work_date=? AND ? BETWEEN i.clock_in_window_start_at AND i.clock_out_window_end_at)) ORDER BY CASE WHEN ? BETWEEN i.clock_in_window_start_at AND i.clock_out_window_end_at THEN 0 ELSE 1 END,i.work_date DESC LIMIT 1`, c.state.SelectedTenantID, c.employeeID, today, yesterday, now, now))
	if err != nil && err != sql.ErrNoRows {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift hari ini.")
		return
	}
	if err == nil {
		instance = &item
	}
	events := []essAttendanceEvent{}
	if instance != nil {
		rows, err := a.db.QueryContext(r.Context(), `SELECT `+essEventColumns+` FROM attendance_events WHERE tenant_id=? AND employee_id=? AND shift_instance_id=? ORDER BY occurred_at,id`, c.state.SelectedTenantID, c.employeeID, instance.ID)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca presensi.")
			return
		}
		defer rows.Close()
		for rows.Next() {
			event, err := scanEssEvent(rows)
			if err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca presensi.")
				return
			}
			events = append(events, event)
		}
		if rows.Err() != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca presensi.")
			return
		}
	}
	type allowedOffice struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		GeofenceCount int    `json:"geofence_count"`
	}
	offices := []allowedOffice{}
	rows, err := a.db.QueryContext(r.Context(), `SELECT o.id,o.name,(SELECT COUNT(*) FROM office_geofences g WHERE g.tenant_id=o.tenant_id AND g.office_id=o.id AND g.is_active=TRUE) FROM offices o WHERE o.tenant_id=? AND (o.id=(SELECT a.office_id FROM employee_assignments a WHERE a.tenant_id=? AND a.employee_id=? AND a.current_flag=TRUE) OR EXISTS (SELECT 1 FROM employee_allowed_offices ao WHERE ao.tenant_id=o.tenant_id AND ao.office_id=o.id AND ao.employee_id=? AND ao.revoked_at IS NULL AND ao.valid_from<=? AND (ao.valid_to IS NULL OR ao.valid_to>=?))) ORDER BY o.name,o.id`, c.state.SelectedTenantID, c.state.SelectedTenantID, c.employeeID, c.employeeID, today, today)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kantor presensi.")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var office allowedOffice
		if err := rows.Scan(&office.ID, &office.Name, &office.GeofenceCount); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kantor presensi.")
			return
		}
		offices = append(offices, office)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kantor presensi.")
		return
	}
	writeAPIData(w, 200, map[string]any{"work_date": today, "shift_instance": instance, "events": events, "allowed_offices": offices}, nil)
}

func essAttendanceRange(w http.ResponseWriter, r *http.Request, c essAttendanceContext) (string, string, bool) {
	today := time.Now().In(c.zone)
	from := today.AddDate(0, 0, -30).Format("2006-01-02")
	to := today.Format("2006-01-02")
	if value := r.URL.Query().Get("from"); value != "" {
		from = value
	}
	if value := r.URL.Query().Get("to"); value != "" {
		to = value
	}
	start, err := time.Parse("2006-01-02", from)
	if err != nil || start.Format("2006-01-02") != from {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal mulai tidak valid.")
		return "", "", false
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil || end.Format("2006-01-02") != to || end.Before(start) || end.Sub(start) > 92*24*time.Hour {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Rentang riwayat maksimal 93 hari.")
		return "", "", false
	}
	return from, to, true
}

func (a adminAPI) essAttendanceHistory(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	from, to, ok := essAttendanceRange(w, r, c)
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	args := []any{c.state.SelectedTenantID, c.employeeID, from, to}
	where := ` FROM attendance_events ev JOIN shift_instances i ON i.tenant_id=ev.tenant_id AND i.id=ev.shift_instance_id WHERE ev.tenant_id=? AND ev.employee_id=? AND i.work_date>=? AND i.work_date<=?`
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT ev.`+strings.ReplaceAll(essEventColumns, ",", ",ev.")+where+` ORDER BY ev.occurred_at DESC,ev.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat.")
		return
	}
	defer rows.Close()
	items := []essAttendanceEvent{}
	for rows.Next() {
		item, err := scanEssEvent(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

type essAttendanceDay struct {
	WorkDate      string     `json:"work_date"`
	Status        string     `json:"status"`
	FirstIn       *time.Time `json:"first_in"`
	LastOut       *time.Time `json:"last_out"`
	WorkedMinutes int        `json:"worked_minutes"`
	LateMinutes   int        `json:"late_minutes"`
}

func essAttendanceMonth(w http.ResponseWriter, r *http.Request, zone *time.Location) (string, string, bool) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().In(zone).Format("2006-01")
	}
	start, err := time.Parse("2006-01", month)
	if err != nil || start.Format("2006-01") != month {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Bulan tidak valid.")
		return "", "", false
	}
	return start.Format("2006-01-02"), start.AddDate(0, 1, 0).Format("2006-01-02"), true
}

func (a adminAPI) essAttendanceCalendar(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	from, to, ok := essAttendanceMonth(w, r, c.zone)
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT work_date,status,first_in,last_out,worked_minutes,late_minutes FROM attendance_daily WHERE tenant_id=? AND employee_id=? AND work_date>=? AND work_date<? ORDER BY work_date`, c.state.SelectedTenantID, c.employeeID, from, to)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kalender presensi.")
		return
	}
	defer rows.Close()
	items := []essAttendanceDay{}
	for rows.Next() {
		var item essAttendanceDay
		var day time.Time
		var first, last sql.NullTime
		if err := rows.Scan(&day, &item.Status, &first, &last, &item.WorkedMinutes, &item.LateMinutes); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kalender presensi.")
			return
		}
		item.WorkDate = day.Format("2006-01-02")
		if first.Valid {
			item.FirstIn = &first.Time
		}
		if last.Valid {
			item.LastOut = &last.Time
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kalender presensi.")
		return
	}
	writeAPIData(w, 200, items, nil)
}

func (a adminAPI) essAttendanceSummary(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	from, to, ok := essAttendanceMonth(w, r, c.zone)
	if !ok {
		return
	}
	var scheduled, present, late, worked int
	err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(first_in IS NOT NULL),0),COALESCE(SUM(late_minutes>0),0),COALESCE(SUM(worked_minutes),0) FROM attendance_daily WHERE tenant_id=? AND employee_id=? AND work_date>=? AND work_date<?`, c.state.SelectedTenantID, c.employeeID, from, to).Scan(&scheduled, &present, &late, &worked)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ringkasan presensi.")
		return
	}
	writeAPIData(w, 200, map[string]any{"month": from[:7], "days_recorded": scheduled, "present_days": present, "late_days": late, "worked_minutes": worked}, nil)
}
