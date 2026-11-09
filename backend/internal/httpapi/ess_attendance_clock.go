package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-sql-driver/mysql"
)

var clientAttendanceID = regexp.MustCompile(`^[A-Za-z0-9_.:-]{8,96}$`)

type attendanceClockInput struct {
	ClientEventID string   `json:"client_event_id"`
	Lat           *float64 `json:"lat"`
	Lng           *float64 `json:"lng"`
	AccuracyM     *float64 `json:"accuracy_m"`
	DeviceTime    *string  `json:"device_time"`
	SelfieFileID  *string  `json:"selfie_file_id"`
}

func validAttendanceCoordinates(in attendanceClockInput) bool {
	if in.Lat == nil && in.Lng == nil && in.AccuracyM == nil {
		return true
	}
	return in.Lat != nil && in.Lng != nil && in.AccuracyM != nil && !math.IsNaN(*in.Lat) && !math.IsNaN(*in.Lng) && !math.IsNaN(*in.AccuracyM) && !math.IsInf(*in.Lat, 0) && !math.IsInf(*in.Lng, 0) && !math.IsInf(*in.AccuracyM, 0) && *in.Lat >= -90 && *in.Lat <= 90 && *in.Lng >= -180 && *in.Lng <= 180 && *in.AccuracyM > 0 && *in.AccuracyM <= 999999.99
}

func (a adminAPI) clockIn(w http.ResponseWriter, r *http.Request) {
	a.clockAttendance(w, r, "clock_in")
}
func (a adminAPI) clockOut(w http.ResponseWriter, r *http.Request) {
	a.clockAttendance(w, r, "clock_out")
}

func (a adminAPI) clockAttendance(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	tenantID := c.state.SelectedTenantID
	var body attendanceClockInput
	if !readJSON(w, r, &body) {
		return
	}
	if !clientAttendanceID.MatchString(body.ClientEventID) || !validAttendanceCoordinates(body) || (body.SelfieFileID != nil && !fileIDPattern.MatchString(*body.SelfieFileID)) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID percobaan, lokasi, atau berkas selfie tidak valid.")
		return
	}
	var deviceTime any
	if body.DeviceTime != nil {
		parsed, e := time.Parse(time.RFC3339Nano, *body.DeviceTime)
		if e != nil {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "device_time harus RFC3339.")
			return
		}
		deviceTime = parsed.UTC()
	}
	readExisting := func() (essAttendanceEvent, error) {
		return scanEssEvent(a.db.QueryRowContext(r.Context(), `SELECT `+essEventColumns+` FROM attendance_events WHERE tenant_id=? AND employee_id=? AND client_event_id=?`, tenantID, c.employeeID, body.ClientEventID))
	}
	if event, e := readExisting(); e == nil {
		if event.EventType != kind {
			writeAPIError(w, r, 409, "IDEMPOTENCY_CONFLICT", "ID percobaan sudah dipakai untuk jenis presensi lain.")
			return
		}
		writeAPIData(w, 200, event, nil)
		return
	} else if !errors.Is(e, sql.ErrNoRows) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa percobaan presensi.")
		return
	}
	now := time.Now().UTC()
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memulai presensi.")
		return
	}
	defer tx.Rollback()
	recheckExisting := func() bool {
		event, e := scanEssEvent(tx.QueryRowContext(r.Context(), `SELECT `+essEventColumns+` FROM attendance_events WHERE tenant_id=? AND employee_id=? AND client_event_id=?`, tenantID, c.employeeID, body.ClientEventID))
		if e == nil {
			if event.EventType != kind {
				writeAPIError(w, r, 409, "IDEMPOTENCY_CONFLICT", "ID percobaan sudah dipakai untuk jenis presensi lain.")
			} else {
				writeAPIData(w, 200, event, nil)
			}
			return true
		}
		if !errors.Is(e, sql.ErrNoRows) {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa percobaan presensi.")
			return true
		}
		return false
	}
	windowStart, windowEnd := "clock_in_window_start_at", "clock_in_window_end_at"
	if kind == "clock_out" {
		windowStart, windowEnd = "clock_out_window_start_at", "clock_out_window_end_at"
	}
	query := `SELECT ` + strings.ReplaceAll(instanceColumns, "i.", "") + ` FROM shift_instances WHERE tenant_id=? AND employee_id=? AND status IN ('scheduled','open') AND ? BETWEEN ` + windowStart + ` AND ` + windowEnd
	if kind == "clock_out" {
		query += ` AND EXISTS (SELECT 1 FROM attendance_events ev WHERE ev.tenant_id=shift_instances.tenant_id AND ev.shift_instance_id=shift_instances.id AND ev.event_type IN ('clock_in','correction_in'))`
	}
	query += ` ORDER BY work_date DESC,id LIMIT 1 FOR UPDATE`
	instance, err := scanInstance(tx.QueryRowContext(r.Context(), query, tenantID, c.employeeID, now))
	if errors.Is(err, sql.ErrNoRows) {
		if recheckExisting() {
			return
		}
		var count int
		_ = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM shift_instances WHERE tenant_id=? AND employee_id=? AND status<>'cancelled' AND work_date BETWEEN ? AND ?`, tenantID, c.employeeID, now.In(c.zone).AddDate(0, 0, -1).Format("2006-01-02"), now.In(c.zone).AddDate(0, 0, 1).Format("2006-01-02")).Scan(&count)
		if count > 0 {
			writeAPIError(w, r, 409, "OUT_OF_WINDOW", "Tidak ada shift dengan jendela presensi yang cocok.")
		} else {
			writeAPIError(w, r, 404, "NO_SHIFT_FOUND", "Tidak ada shift terbit untuk presensi.")
		}
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift presensi.")
		return
	}
	if recheckExisting() {
		return
	}
	var rule struct {
		Shift shiftTemplate `json:"shift"`
	}
	if json.Unmarshal(instance.RuleSnapshot, &rule) != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Snapshot shift tidak valid.")
		return
	}
	if rule.Shift.RequireFaceRecognition {
		writeAPIError(w, r, 503, "FACE_PROVIDER_UNAVAILABLE", "Provider pengenalan wajah belum dikonfigurasi.")
		return
	}
	if rule.Shift.RequireSelfie {
		if body.SelfieFileID == nil {
			writeAPIError(w, r, 422, "SELFIE_REQUIRED", "Selfie wajib untuk shift ini.")
			return
		}
		var scan string
		var uploadedAt time.Time
		if err = tx.QueryRowContext(r.Context(), `SELECT scan_status,created_at FROM files WHERE tenant_id=? AND id=? AND uploaded_by=? AND purpose='selfie' AND deleted_at IS NULL FOR UPDATE`, tenantID, *body.SelfieFileID, c.state.User.ID).Scan(&scan, &uploadedAt); err != nil || scan != "clean" || receiptStorageRoot() == "" || now.Sub(uploadedAt) > 15*time.Minute {
			writeAPIError(w, r, 422, "SELFIE_REQUIRED", "Selfie terverifikasi milik Anda wajib tersedia.")
			return
		}
		var used int
		if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM attendance_events WHERE tenant_id=? AND selfie_file_id=?`, tenantID, *body.SelfieFileID).Scan(&used); err != nil || used != 0 {
			writeAPIError(w, r, 422, "SELFIE_REQUIRED", "Selfie sudah dipakai atau tidak dapat diverifikasi.")
			return
		}
		if _, err = os.Stat(receiptFilePath(receiptStorageRoot(), tenantID, *body.SelfieFileID)); err != nil {
			writeAPIError(w, r, 422, "SELFIE_REQUIRED", "Berkas selfie tidak tersedia.")
			return
		}
	}
	locationStatus := "flexible"
	var officeID, geofenceID any
	if rule.Shift.RequireLocation && !rule.Shift.FlexLocation {
		if body.Lat == nil {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Lokasi wajib untuk shift ini.")
			return
		}
		if *body.AccuracyM > maxAttendanceAccuracyM {
			writeAPIError(w, r, 422, "LOCATION_ACCURACY_LOW", "Akurasi GPS lebih buruk dari 100 meter.")
			return
		}
		match, status, e := a.attendanceGeofence(r, tx, tenantID, c.employeeID, instance.WorkDate, instance.OfficeID, *body.Lat, *body.Lng)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa geofence.")
			return
		}
		if status == "unknown" {
			writeAPIError(w, r, 409, "LOCATION_NOT_CONFIGURED", "Geofence kantor yang diizinkan belum tersedia.")
			return
		}
		if status == "outside" {
			writeAPIError(w, r, 403, "LOCATION_OUT_OF_AREA", "Lokasi berada di luar area presensi.")
			return
		}
		locationStatus = "inside"
		officeID, geofenceID = match.OfficeID, match.GeofenceID
	} else if body.Lat != nil && *body.AccuracyM > maxAttendanceAccuracyM {
		writeAPIError(w, r, 422, "LOCATION_ACCURACY_LOW", "Akurasi GPS lebih buruk dari 100 meter.")
		return
	}
	var existing int
	selectTypes := "'clock_in','correction_in'"
	conflictCode := "ALREADY_CLOCKED_IN"
	if kind == "clock_out" {
		selectTypes = "'clock_out','correction_out'"
		conflictCode = "ALREADY_CLOCKED_OUT"
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM attendance_events WHERE tenant_id=? AND shift_instance_id=? AND event_type IN (`+selectTypes+`)`, tenantID, instance.ID).Scan(&existing); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa event presensi.")
		return
	}
	if existing > 0 {
		writeAPIError(w, r, 409, conflictCode, "Presensi untuk shift ini sudah tercatat.")
		return
	}
	timing := "on_time"
	if kind == "clock_in" && now.After(instance.ExpectedStart.Add(time.Duration(rule.Shift.LateToleranceMin)*time.Minute)) {
		timing = "late"
	}
	if kind == "clock_out" && now.Before(instance.ExpectedEnd) {
		timing = "early_leave"
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID presensi.")
		return
	}
	var lat, lng, accuracy any
	if body.Lat != nil {
		lat = *body.Lat
		lng = *body.Lng
		accuracy = *body.AccuracyM
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO attendance_events (tenant_id,id,employee_id,shift_instance_id,event_type,occurred_at,device_time,latitude,longitude,accuracy_m,office_id,geofence_id,location_status,timing_status,selfie_file_id,source,client_event_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'device',?)`, tenantID, id, c.employeeID, instance.ID, kind, now, deviceTime, lat, lng, accuracy, officeID, geofenceID, locationStatus, timing, body.SelfieFileID, body.ClientEventID)
	if err != nil {
		var dbErr *mysql.MySQLError
		if errors.As(err, &dbErr) && dbErr.Number == 1062 {
			tx.Rollback()
			event, e := readExisting()
			if e == nil && event.EventType == kind {
				writeAPIData(w, 200, event, nil)
				return
			}
			writeAPIError(w, r, 409, "IDEMPOTENCY_CONFLICT", "ID percobaan atau shift sudah dipakai.")
			return
		}
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat presensi.")
		return
	}
	newStatus := "open"
	if kind == "clock_out" {
		newStatus = "completed"
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE shift_instances SET status=? WHERE tenant_id=? AND id=? AND status IN ('scheduled','open')`, newStatus, tenantID, instance.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui shift.")
		return
	}
	instance.Status = newStatus
	if err = recalculateAttendanceDay(r.Context(), tx, tenantID, instance); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung rekap presensi.")
		return
	}
	reminderType := "attendance.clock_in"
	if kind == "clock_out" {
		reminderType = "attendance.clock_out"
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE notification_schedules ns JOIN notification_rules nr ON nr.tenant_id=ns.tenant_id AND nr.id=ns.rule_id SET ns.status='cancelled' WHERE ns.tenant_id=? AND ns.subject_type='shift_instance' AND ns.subject_id=? AND ns.status='pending' AND nr.event_type IN (?,CONCAT(?,'_reminder'))`, tenantID, instance.ID, reminderType, reminderType); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan pengingat.")
		return
	}
	var managerUser sql.NullString
	_ = tx.QueryRowContext(r.Context(), `SELECT manager.user_id FROM employee_assignments a JOIN employees manager ON manager.tenant_id=a.tenant_id AND manager.id=a.reports_to_employee_id WHERE a.tenant_id=? AND a.employee_id=? AND a.current_flag=TRUE`, tenantID, c.employeeID).Scan(&managerUser)
	if managerUser.Valid && managerUser.String != c.state.User.ID {
		notificationID, _ := ids.New()
		_, err = tx.ExecContext(r.Context(), `INSERT INTO notifications (tenant_id,id,recipient_user_id,category,event_type,title,body,deep_link,dedupe_key) VALUES (?,?,?,'system',?,'Presensi karyawan tercatat',?,'/admin/attendance/today',?)`, tenantID, notificationID, managerUser.String, "attendance."+kind, "Event presensi baru tersedia.", "attendance:"+id)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memberi notifikasi atasan.")
			return
		}
	}
	if err = audit(r.Context(), tx, tenantID, c.state.User.ID, "create", "attendance_events", id, nil, map[string]any{"event_type": kind, "shift_instance_id": instance.ID, "timing_status": timing}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit presensi.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan presensi.")
		return
	}
	event := essAttendanceEvent{ID: id, ShiftInstanceID: instance.ID, EventType: kind, OccurredAt: now, LocationStatus: &locationStatus, TimingStatus: &timing, SelfieFileID: body.SelfieFileID, DeviceTime: nil}
	if body.Lat != nil {
		event.Latitude = body.Lat
		event.Longitude = body.Lng
		event.AccuracyM = body.AccuracyM
	}
	if office, ok := officeID.(string); ok {
		event.OfficeID = &office
	}
	if geofence, ok := geofenceID.(string); ok {
		event.GeofenceID = &geofence
	}
	if parsed, ok := deviceTime.(time.Time); ok {
		event.DeviceTime = &parsed
	}
	writeAPIData(w, 201, event, nil)
}
