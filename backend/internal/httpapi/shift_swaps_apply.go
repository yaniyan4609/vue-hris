package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"digaji.id/hris/internal/schedule"
)

func swappedTimes(workDate, timezone string, snapshot json.RawMessage) (schedule.InstanceTimes, shiftTemplate, error) {
	var source struct {
		Shift shiftTemplate `json:"shift"`
	}
	if err := json.Unmarshal(snapshot, &source); err != nil {
		return schedule.InstanceTimes{}, shiftTemplate{}, err
	}
	s := source.Shift
	if len(s.StartTime) < 5 || len(s.EndTime) < 5 || len(s.ClockInWindowStartTime) < 5 || len(s.ClockInWindowEndTime) < 5 || len(s.ClockOutWindowStartTime) < 5 || len(s.ClockOutWindowEndTime) < 5 {
		return schedule.InstanceTimes{}, s, errors.New("snapshot shift tidak lengkap")
	}
	date, err := time.Parse("2006-01-02", workDate)
	if err != nil {
		return schedule.InstanceTimes{}, s, err
	}
	times, err := schedule.MaterializeTimes(date, timezone, schedule.ShiftTimes{Start: s.StartTime[:5], End: s.EndTime[:5], CrossesMidnight: s.CrossesMidnight, ClockInStart: schedule.LocalBound{Clock: s.ClockInWindowStartTime[:5], DayOffset: s.ClockInWindowStartDayOffset}, ClockInEnd: schedule.LocalBound{Clock: s.ClockInWindowEndTime[:5], DayOffset: s.ClockInWindowEndDayOffset}, ClockOutStart: schedule.LocalBound{Clock: s.ClockOutWindowStartTime[:5], DayOffset: s.ClockOutWindowStartDayOffset}, ClockOutEnd: schedule.LocalBound{Clock: s.ClockOutWindowEndTime[:5], DayOffset: s.ClockOutWindowEndDayOffset}})
	return times, s, err
}

func (a adminAPI) applyApprovedShiftSwap(r *http.Request, tx *sql.Tx, c adminContext, d overtimeDecision) error {
	var swap shiftSwap
	var err error
	swap, err = scanSwap(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(swapColumns, "s.", "")+` FROM shift_swaps WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, d.RefID))
	if err != nil || swap.Status != "in_review" || swap.RequestID == nil || *swap.RequestID != d.RequestID {
		return errors.New("usulan pertukaran sudah berubah")
	}
	first, second := swap.InstanceAID, swap.InstanceBID
	if first > second {
		first, second = second, first
	}
	locked := map[string]shiftInstance{}
	for _, id := range []string{first, second} {
		item, e := scanInstance(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(instanceColumns, "i.", "")+` FROM shift_instances WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id))
		if e != nil {
			return errors.New("shift tidak tersedia")
		}
		locked[id] = item
	}
	aShift, bShift := locked[swap.InstanceAID], locked[swap.InstanceBID]
	if aShift.EmployeeID != swap.RequesterID || bShift.EmployeeID != swap.PartnerID || aShift.Status != "scheduled" || bShift.Status != "scheduled" || aShift.WorkDate < tenantToday(c.tenant.Timezone) || bShift.WorkDate < tenantToday(c.tenant.Timezone) {
		return errors.New("shift sudah berubah atau berlalu")
	}
	partner, err := scanEmployee(tx.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.tenant.ID, swap.PartnerID))
	if err != nil {
		return errors.New("rekan tidak tersedia")
	}
	partnerTarget, err := a.targetFor(r.Context(), c.tenant.ID, partner)
	if err != nil {
		return errors.New("scope rekan tidak tersedia")
	}
	approverSubject, approverRight, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "jadwal.approve")
	if err != nil || !approverRight.Allowed || !approverRight.Includes(approverSubject, partnerTarget) {
		return errors.New("approver tidak memiliki scope rekan")
	}
	var used int
	if err = tx.QueryRowContext(r.Context(), `SELECT (SELECT COUNT(*) FROM attendance_events WHERE tenant_id=? AND shift_instance_id IN (?,?))+(SELECT COUNT(*) FROM attendance_daily WHERE tenant_id=? AND shift_instance_id IN (?,?))+(SELECT COUNT(*) FROM leave_requests WHERE tenant_id=? AND status='approved' AND ((employee_id=? AND start_date<=? AND end_date>=?) OR (employee_id=? AND start_date<=? AND end_date>=?)))`, c.tenant.ID, first, second, c.tenant.ID, first, second, c.tenant.ID, aShift.EmployeeID, aShift.WorkDate, aShift.WorkDate, bShift.EmployeeID, bShift.WorkDate, bShift.WorkDate).Scan(&used); err != nil || used > 0 {
		return errors.New("shift sudah dipakai presensi atau cuti")
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payroll_runs run JOIN payroll_periods p ON p.tenant_id=run.tenant_id AND p.id=run.period_id WHERE run.tenant_id=? AND run.status='finalized' AND ((p.start_date<=? AND p.end_date>=?) OR (p.start_date<=? AND p.end_date>=?))`, c.tenant.ID, aShift.WorkDate, aShift.WorkDate, bShift.WorkDate, bShift.WorkDate).Scan(&used); err != nil || used > 0 {
		return errors.New("periode payroll sudah final")
	}
	var tzA, tzB string
	if err = tx.QueryRowContext(r.Context(), `SELECT timezone FROM offices WHERE tenant_id=? AND id=?`, c.tenant.ID, bShift.OfficeID).Scan(&tzA); err != nil {
		return errors.New("kantor shift rekan tidak tersedia")
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT timezone FROM offices WHERE tenant_id=? AND id=?`, c.tenant.ID, aShift.OfficeID).Scan(&tzB); err != nil {
		return errors.New("kantor shift pengaju tidak tersedia")
	}
	timesA, shiftA, err := swappedTimes(aShift.WorkDate, tzA, bShift.RuleSnapshot)
	if err != nil {
		return errors.New("snapshot shift rekan tidak valid")
	}
	timesB, shiftB, err := swappedTimes(bShift.WorkDate, tzB, aShift.RuleSnapshot)
	if err != nil {
		return errors.New("snapshot shift pengaju tidak valid")
	}
	if shiftA.ID != bShift.ShiftTemplateID || shiftB.ID != aShift.ShiftTemplateID {
		return errors.New("snapshot shift tidak cocok")
	}
	for _, change := range []struct {
		Current  shiftInstance
		Other    shiftInstance
		Times    schedule.InstanceTimes
		Shift    shiftTemplate
		Timezone string
	}{{aShift, bShift, timesA, shiftA, tzA}, {bShift, aShift, timesB, shiftB, tzB}} {
		roster, e := scanWorkSchedule(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(rosterColumns, "w.", "")+` FROM work_schedules WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, change.Current.WorkScheduleID))
		if e != nil || roster.Status != "published" || roster.ShiftTemplateID == nil || *roster.ShiftTemplateID != change.Current.ShiftTemplateID || roster.OfficeID == nil || *roster.OfficeID != change.Current.OfficeID {
			return errors.New("roster sudah berubah")
		}
		snapshot, _ := json.Marshal(map[string]any{"shift": change.Shift, "swap_id": swap.ID, "source_instance_id": change.Other.ID, "roster_version": roster.Version + 1})
		result, e := tx.ExecContext(r.Context(), `UPDATE shift_instances SET shift_template_id=?,office_id=?,timezone=?,expected_start=?,expected_end=?,clock_in_window_start_at=?,clock_in_window_end_at=?,clock_out_window_start_at=?,clock_out_window_end_at=?,rule_snapshot=? WHERE tenant_id=? AND id=? AND status='scheduled'`, change.Other.ShiftTemplateID, change.Other.OfficeID, change.Timezone, change.Times.ExpectedStart, change.Times.ExpectedEnd, change.Times.ClockInStart, change.Times.ClockInEnd, change.Times.ClockOutStart, change.Times.ClockOutEnd, string(snapshot), c.tenant.ID, change.Current.ID)
		if e != nil {
			return errors.New("gagal menyimpan shift pertukaran")
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			return errors.New("shift sudah berubah")
		}
		result, e = tx.ExecContext(r.Context(), `UPDATE work_schedules SET shift_template_id=?,office_id=?,version=version+1 WHERE tenant_id=? AND id=? AND status='published'`, change.Other.ShiftTemplateID, change.Other.OfficeID, c.tenant.ID, change.Current.WorkScheduleID)
		if e != nil {
			return errors.New("gagal menyimpan roster pertukaran")
		}
		count, _ = result.RowsAffected()
		if count != 1 {
			return errors.New("roster sudah berubah")
		}
		after := roster
		after.ShiftTemplateID = &change.Other.ShiftTemplateID
		after.OfficeID = &change.Other.OfficeID
		after.Version++
		beforeJSON, _ := json.Marshal(roster)
		afterJSON, _ := json.Marshal(after)
		revisionID, _ := ids.New()
		if _, e = tx.ExecContext(r.Context(), `INSERT INTO work_schedule_revisions (tenant_id,id,work_schedule_id,revision_no,before_json,after_json,actor_user_id,reason) VALUES (?,?,?,?,?,?,?,?)`, c.tenant.ID, revisionID, roster.ID, after.Version, string(beforeJSON), string(afterJSON), c.state.User.ID, "Pertukaran shift "+swap.ID); e != nil {
			return errors.New("gagal mencatat revisi roster")
		}
		if e = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "swap", "work_schedules", roster.ID, roster, after); e != nil {
			return errors.New("gagal mencatat audit roster")
		}
		if _, e = tx.ExecContext(r.Context(), `UPDATE notification_schedules SET status='cancelled' WHERE tenant_id=? AND subject_type='shift_instance' AND subject_id=? AND status='pending'`, c.tenant.ID, change.Current.ID); e != nil {
			return errors.New("gagal membatalkan pengingat lama")
		}
		if e = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "swap", "shift_instances", change.Current.ID, map[string]any{"shift_template_id": change.Current.ShiftTemplateID, "office_id": change.Current.OfficeID, "rule_snapshot": change.Current.RuleSnapshot}, map[string]any{"shift_template_id": change.Other.ShiftTemplateID, "office_id": change.Other.OfficeID, "rule_snapshot": json.RawMessage(snapshot), "swap_id": swap.ID}); e != nil {
			return errors.New("gagal mencatat audit pertukaran")
		}
	}
	for _, employeeID := range []string{swap.RequesterID, swap.PartnerID} {
		var userID string
		if err = tx.QueryRowContext(r.Context(), `SELECT user_id FROM employees WHERE tenant_id=? AND id=?`, c.tenant.ID, employeeID).Scan(&userID); err != nil {
			return errors.New("akun penerima notifikasi tidak tersedia")
		}
		if err = notifyShiftSwap(r.Context(), tx, c.tenant.ID, userID, swap.ID, "schedule.swap_approved", "Tukar shift disetujui", "Jadwal pertukaran shift sudah diperbarui."); err != nil {
			return errors.New("gagal membuat notifikasi pertukaran")
		}
	}
	return nil
}
