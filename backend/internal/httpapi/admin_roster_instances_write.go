package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"digaji.id/hris/internal/schedule"
	"github.com/go-chi/chi/v5"
)

func materializeRosterShift(ctx context.Context, tx *sql.Tx, tenantID string, item workSchedule) (shiftInstance, error) {
	if item.ShiftTemplateID == nil || item.OfficeID == nil {
		return shiftInstance{}, errors.New("incomplete roster")
	}
	shift, err := scanShift(tx.QueryRowContext(ctx, `SELECT id,`+shiftColumns+` FROM shift_templates WHERE tenant_id=? AND id=?`, tenantID, *item.ShiftTemplateID))
	if err != nil || !shift.IsActive {
		return shiftInstance{}, errors.New("shift unavailable")
	}
	var timezone string
	if err = tx.QueryRowContext(ctx, `SELECT timezone FROM offices WHERE tenant_id=? AND id=?`, tenantID, *item.OfficeID).Scan(&timezone); err != nil {
		return shiftInstance{}, err
	}
	date, _ := time.Parse("2006-01-02", item.WorkDate)
	times, err := schedule.MaterializeTimes(date, timezone, schedule.ShiftTimes{Start: shift.StartTime[:5], End: shift.EndTime[:5], CrossesMidnight: shift.CrossesMidnight, ClockInStart: schedule.LocalBound{Clock: shift.ClockInWindowStartTime[:5], DayOffset: shift.ClockInWindowStartDayOffset}, ClockInEnd: schedule.LocalBound{Clock: shift.ClockInWindowEndTime[:5], DayOffset: shift.ClockInWindowEndDayOffset}, ClockOutStart: schedule.LocalBound{Clock: shift.ClockOutWindowStartTime[:5], DayOffset: shift.ClockOutWindowStartDayOffset}, ClockOutEnd: schedule.LocalBound{Clock: shift.ClockOutWindowEndTime[:5], DayOffset: shift.ClockOutWindowEndDayOffset}})
	if err != nil {
		return shiftInstance{}, err
	}
	snapshot, _ := json.Marshal(map[string]any{"shift": shift, "source_pattern_id": item.SourcePatternID, "source_pattern_version": item.SourcePatternVersion, "roster_version": item.Version})
	id, err := ids.New()
	if err != nil {
		return shiftInstance{}, err
	}
	result := shiftInstance{ID: id, EmployeeID: item.EmployeeID, WorkScheduleID: item.ID, ShiftTemplateID: *item.ShiftTemplateID, OfficeID: *item.OfficeID, WorkDate: item.WorkDate, Timezone: timezone, ExpectedStart: times.ExpectedStart, ExpectedEnd: times.ExpectedEnd, ClockInWindowStart: times.ClockInStart, ClockInWindowEnd: times.ClockInEnd, ClockOutWindowStart: times.ClockOutStart, ClockOutWindowEnd: times.ClockOutEnd, RuleSnapshot: snapshot, Status: "scheduled"}
	_, err = tx.ExecContext(ctx, `INSERT INTO shift_instances (tenant_id,id,employee_id,work_schedule_id,shift_template_id,office_id,work_date,timezone,expected_start,expected_end,clock_in_window_start_at,clock_in_window_end_at,clock_out_window_start_at,clock_out_window_end_at,rule_snapshot,status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'scheduled')`, tenantID, id, item.EmployeeID, item.ID, *item.ShiftTemplateID, *item.OfficeID, item.WorkDate, timezone, times.ExpectedStart, times.ExpectedEnd, times.ClockInStart, times.ClockInEnd, times.ClockOutStart, times.ClockOutEnd, string(snapshot))
	return result, err
}

// generateShiftInstances is safe to retry: only published rosters without an
// instance are selected. The roster row is locked before materialization.
func (a adminAPI) generateShiftInstances(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.publikasi")
	if !ok {
		return
	}
	var body struct {
		From        string   `json:"from"`
		To          string   `json:"to"`
		EmployeeIDs []string `json:"employee_ids"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	start, e1 := time.Parse("2006-01-02", body.From)
	end, e2 := time.Parse("2006-01-02", body.To)
	if e1 != nil || e2 != nil || start.Format("2006-01-02") != body.From || end.Format("2006-01-02") != body.To || end.Before(start) || end.Sub(start) > 30*24*time.Hour {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Rentang generate maksimal 31 hari.")
		return
	}
	if len(body.EmployeeIDs) > 100 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Maksimal 100 karyawan.")
		return
	}
	filter, filterArgs := access.EmployeeWhere(c.subject, c.effective)
	where := "w.tenant_id=? AND w.status='published' AND w.day_type='shift' AND w.work_date>=? AND w.work_date<=? AND i.id IS NULL AND " + filter
	args := append([]any{c.tenant.ID, body.From, body.To}, filterArgs...)
	if len(body.EmployeeIDs) > 0 {
		seen := map[string]bool{}
		for _, id := range body.EmployeeIDs {
			if len(id) != 26 || seen[id] {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Daftar karyawan tidak valid.")
				return
			}
			seen[id] = true
			args = append(args, id)
		}
		where += " AND w.employee_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(body.EmployeeIDs)), ",") + ")"
	}
	join := rosterJoin + ` LEFT JOIN shift_instances i ON i.tenant_id=w.tenant_id AND i.work_schedule_id=w.id`
	rows, err := a.db.QueryContext(r.Context(), "SELECT w.id"+join+" WHERE "+where+" ORDER BY w.work_date,w.employee_id LIMIT 201", args...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencari roster terbit.")
		return
	}
	idsToGenerate := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		idsToGenerate = append(idsToGenerate, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencari roster terbit.")
		return
	}
	if len(idsToGenerate) > 200 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Maksimal 200 instance per generate.")
		return
	}
	if len(idsToGenerate) == 0 {
		writeAPIData(w, 200, map[string]any{"generated": []shiftInstance{}, "count": 0}, nil)
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memulai generate.")
		return
	}
	defer tx.Rollback()
	generated := []shiftInstance{}
	for _, id := range idsToGenerate {
		item, e := scanWorkSchedule(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(rosterColumns, "w.", "")+` FROM work_schedules WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id))
		if e != nil || item.Status != "published" || item.DayType != "shift" {
			writeAPIError(w, r, 409, "ROSTER_CONFLICT", "Roster sudah berubah.")
			return
		}
		var count int
		if e = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM shift_instances WHERE tenant_id=? AND work_schedule_id=?`, c.tenant.ID, id).Scan(&count); e != nil || count != 0 {
			writeAPIError(w, r, 409, "INSTANCE_CONFLICT", "Instance sudah dibuat oleh proses lain.")
			return
		}
		instance, e := materializeRosterShift(r.Context(), tx, c.tenant.ID, item)
		if e != nil {
			writeAPIError(w, r, 409, "SHIFT_INVALID", "Shift tidak dapat dimaterialisasi atau instance bertabrakan.")
			return
		}
		if e = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "generate", "shift_instances", instance.ID, nil, map[string]any{"roster_id": id, "work_date": item.WorkDate}); e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
			return
		}
		generated = append(generated, instance)
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan instance.")
		return
	}
	writeAPIData(w, 200, map[string]any{"generated": generated, "count": len(generated)}, nil)
}

func (a adminAPI) cancelShiftInstance(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Instance tidak ditemukan.")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if body.Reason == "" || len([]rune(body.Reason)) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Alasan pembatalan wajib, maksimal 500 karakter.")
		return
	}
	filter, filterArgs := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, id}, filterArgs...)
	item, err := scanInstance(a.db.QueryRowContext(r.Context(), "SELECT "+instanceColumns+instanceJoin+" WHERE i.tenant_id=? AND i.id=? AND "+filter, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Instance tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca instance.")
		return
	}
	if item.Status != "scheduled" || item.WorkDate < tenantToday(c.tenant.Timezone) {
		writeAPIError(w, r, 409, "INSTANCE_LOCKED", "Hanya instance mendatang yang belum dipakai dapat dibatalkan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memulai pembatalan.")
		return
	}
	defer tx.Rollback()
	var status string
	if err = tx.QueryRowContext(r.Context(), `SELECT status FROM shift_instances WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id).Scan(&status); err != nil || status != "scheduled" {
		writeAPIError(w, r, 409, "INSTANCE_LOCKED", "Status instance berubah.")
		return
	}
	var dependencies int
	if err = tx.QueryRowContext(r.Context(), `SELECT (SELECT COUNT(*) FROM attendance_events WHERE tenant_id=? AND shift_instance_id=?)+(SELECT COUNT(*) FROM attendance_daily WHERE tenant_id=? AND shift_instance_id=?)+(SELECT COUNT(*) FROM shift_swaps WHERE tenant_id=? AND (instance_a_id=? OR instance_b_id=?) AND status IN ('pending_partner','in_review','returned'))+(SELECT COUNT(*) FROM leave_requests WHERE tenant_id=? AND employee_id=? AND status='approved' AND start_date<=? AND end_date>=?)`, c.tenant.ID, id, c.tenant.ID, id, c.tenant.ID, id, id, c.tenant.ID, item.EmployeeID, item.WorkDate, item.WorkDate).Scan(&dependencies); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa penggunaan instance.")
		return
	}
	if dependencies > 0 {
		writeAPIError(w, r, 409, "INSTANCE_IN_USE", "Instance dipakai presensi, cuti, atau pertukaran shift.")
		return
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payroll_runs run JOIN payroll_periods p ON p.tenant_id=run.tenant_id AND p.id=run.period_id WHERE run.tenant_id=? AND run.status='finalized' AND p.start_date<=? AND p.end_date>=?`, c.tenant.ID, item.WorkDate, item.WorkDate).Scan(&dependencies); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa periode payroll.")
		return
	}
	if dependencies > 0 {
		writeAPIError(w, r, 409, "PAYROLL_LOCKED", "Periode payroll sudah final.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE shift_instances SET status='cancelled' WHERE tenant_id=? AND id=? AND status='scheduled'`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan instance.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE notification_schedules SET status='cancelled' WHERE tenant_id=? AND subject_type='shift_instance' AND subject_id=? AND status='pending'`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan pengingat.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "cancel", "shift_instances", id, map[string]any{"status": status}, map[string]any{"status": "cancelled", "reason": body.Reason}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pembatalan.")
		return
	}
	item.Status = "cancelled"
	writeAPIData(w, 200, map[string]any{"instance": item, "reason": body.Reason}, nil)
}
