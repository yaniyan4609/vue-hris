package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"digaji.id/hris/internal/schedule"
)

type publishedInstance struct {
	RosterID   string  `json:"roster_id"`
	InstanceID *string `json:"instance_id"`
	EmployeeID string  `json:"employee_id"`
	WorkDate   string  `json:"work_date"`
	DayType    string  `json:"day_type"`
}

func (a adminAPI) publishWorkSchedules(w http.ResponseWriter, r *http.Request) {
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
	start, err := time.Parse("2006-01-02", body.From)
	if err != nil || start.Format("2006-01-02") != body.From {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal mulai tidak valid.")
		return
	}
	end, err := time.Parse("2006-01-02", body.To)
	if err != nil || end.Format("2006-01-02") != body.To || end.Before(start) || end.Sub(start) > 30*24*time.Hour {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Rentang publikasi maksimal 31 hari.")
		return
	}
	filter, filterArgs := access.EmployeeWhere(c.subject, c.effective)
	where := "w.tenant_id=? AND w.status='draft' AND w.work_date>=? AND w.work_date<=? AND " + filter
	args := append([]any{c.tenant.ID, body.From, body.To}, filterArgs...)
	if len(body.EmployeeIDs) > 0 {
		if len(body.EmployeeIDs) > 100 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Maksimal 100 karyawan.")
			return
		}
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
	rows, err := a.db.QueryContext(r.Context(), `SELECT w.id`+rosterJoin+" WHERE "+where+` ORDER BY w.work_date,w.employee_id LIMIT 201`, args...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca roster draft.")
		return
	}
	idsToPublish := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca roster draft.")
			return
		}
		idsToPublish = append(idsToPublish, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca roster draft.")
		return
	}
	rows.Close()
	if len(idsToPublish) == 0 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Tidak ada roster draft dalam scope dan rentang itu.")
		return
	}
	if len(idsToPublish) > 200 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Maksimal 200 roster per publikasi.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menerbitkan roster.")
		return
	}
	defer tx.Rollback()
	published := make([]publishedInstance, 0, len(idsToPublish))
	for _, id := range idsToPublish {
		item, err := scanWorkSchedule(tx.QueryRowContext(r.Context(), `SELECT `+strings.ReplaceAll(rosterColumns, "w.", "")+` FROM work_schedules WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id))
		if err != nil || item.Status != "draft" {
			writeAPIError(w, r, 409, "VERSION_CONFLICT", "Roster sudah berubah; muat ulang.")
			return
		}
		result := publishedInstance{RosterID: id, EmployeeID: item.EmployeeID, WorkDate: item.WorkDate, DayType: item.DayType}
		if item.DayType == "shift" {
			if item.ShiftTemplateID == nil || item.OfficeID == nil {
				writeAPIError(w, r, 409, "ROSTER_INVALID", "Roster shift tidak lengkap.")
				return
			}
			shift, err := scanShift(tx.QueryRowContext(r.Context(), `SELECT id,`+shiftColumns+` FROM shift_templates WHERE tenant_id=? AND id=?`, c.tenant.ID, *item.ShiftTemplateID))
			if err != nil || !shift.IsActive {
				writeAPIError(w, r, 409, "SHIFT_UNAVAILABLE", "Shift tidak tersedia atau tidak aktif.")
				return
			}
			var timezone string
			if err = tx.QueryRowContext(r.Context(), `SELECT timezone FROM offices WHERE tenant_id=? AND id=?`, c.tenant.ID, *item.OfficeID).Scan(&timezone); err != nil {
				writeAPIError(w, r, 409, "OFFICE_UNAVAILABLE", "Kantor tidak tersedia.")
				return
			}
			date, _ := time.Parse("2006-01-02", item.WorkDate)
			times, err := schedule.MaterializeTimes(date, timezone, schedule.ShiftTimes{Start: shift.StartTime[:5], End: shift.EndTime[:5], CrossesMidnight: shift.CrossesMidnight, ClockInStart: schedule.LocalBound{Clock: shift.ClockInWindowStartTime[:5], DayOffset: shift.ClockInWindowStartDayOffset}, ClockInEnd: schedule.LocalBound{Clock: shift.ClockInWindowEndTime[:5], DayOffset: shift.ClockInWindowEndDayOffset}, ClockOutStart: schedule.LocalBound{Clock: shift.ClockOutWindowStartTime[:5], DayOffset: shift.ClockOutWindowStartDayOffset}, ClockOutEnd: schedule.LocalBound{Clock: shift.ClockOutWindowEndTime[:5], DayOffset: shift.ClockOutWindowEndDayOffset}})
			if err != nil {
				writeAPIError(w, r, 409, "SHIFT_INVALID", "Jendela shift tidak dapat dimaterialisasi.")
				return
			}
			snapshot, _ := json.Marshal(map[string]any{"shift": shift, "source_pattern_id": item.SourcePatternID, "source_pattern_version": item.SourcePatternVersion, "roster_version": item.Version})
			instanceID, err := ids.New()
			if err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat shift instance.")
				return
			}
			if _, err = tx.ExecContext(r.Context(), `INSERT INTO shift_instances (tenant_id,id,employee_id,work_schedule_id,shift_template_id,office_id,work_date,timezone,expected_start,expected_end,clock_in_window_start_at,clock_in_window_end_at,clock_out_window_start_at,clock_out_window_end_at,rule_snapshot,status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'scheduled')`, c.tenant.ID, instanceID, item.EmployeeID, id, *item.ShiftTemplateID, *item.OfficeID, item.WorkDate, timezone, times.ExpectedStart, times.ExpectedEnd, times.ClockInStart, times.ClockInEnd, times.ClockOutStart, times.ClockOutEnd, string(snapshot)); err != nil {
				writeAPIError(w, r, 409, "ROSTER_CONFLICT", "Shift instance untuk tanggal itu sudah ada.")
				return
			}
			result.InstanceID = &instanceID
			var userID sql.NullString
			if err = tx.QueryRowContext(r.Context(), `SELECT user_id FROM employees WHERE tenant_id=? AND id=?`, c.tenant.ID, item.EmployeeID).Scan(&userID); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca akun karyawan.")
				return
			}
			if userID.Valid {
				notificationID, err := ids.New()
				if err != nil {
					writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
					return
				}
				if _, err = tx.ExecContext(r.Context(), `INSERT INTO notifications (tenant_id,id,recipient_user_id,category,event_type,title,body,deep_link,dedupe_key) VALUES (?,?,?,'system','schedule.published','Jadwal kerja diterbitkan',?,'/ess/schedule',?)`, c.tenant.ID, notificationID, userID.String, "Jadwal kerja untuk "+item.WorkDate+" tersedia.", "roster:"+id); err != nil {
					writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
					return
				}
			}
		}
		publishedAt := time.Now().UTC()
		update, err := tx.ExecContext(r.Context(), `UPDATE work_schedules SET status='published',published_at=?,version=version+1 WHERE tenant_id=? AND id=? AND status='draft' AND version=?`, publishedAt, c.tenant.ID, id, item.Version)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menerbitkan roster.")
			return
		}
		affected, _ := update.RowsAffected()
		if affected != 1 {
			writeAPIError(w, r, 409, "VERSION_CONFLICT", "Roster sudah berubah.")
			return
		}
		after := item
		after.Status = "published"
		after.Version++
		after.PublishedAt = &publishedAt
		beforeJSON, _ := json.Marshal(item)
		afterJSON, _ := json.Marshal(after)
		revisionID, _ := ids.New()
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO work_schedule_revisions (tenant_id,id,work_schedule_id,revision_no,before_json,after_json,actor_user_id,reason) VALUES (?,?,?,?,?,?,?,'Roster diterbitkan')`, c.tenant.ID, revisionID, id, after.Version, string(beforeJSON), string(afterJSON), c.state.User.ID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat revisi.")
			return
		}
		if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "publish", "work_schedules", id, item, after); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
			return
		}
		published = append(published, result)
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menerbitkan roster.")
		return
	}
	writeAPIData(w, 200, map[string]any{"published": published, "count": len(published)}, nil)
}
