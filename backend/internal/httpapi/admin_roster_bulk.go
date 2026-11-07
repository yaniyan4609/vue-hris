package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"digaji.id/hris/internal/schedule"
)

type rosterBulkInput struct {
	EmployeeIDs []string `json:"employee_ids"`
	PatternID   string   `json:"pattern_id"`
	OfficeID    *string  `json:"office_id"`
	From        string   `json:"from"`
	To          string   `json:"to"`
	DryRun      bool     `json:"dry_run"`
}
type rosterBulkConflict struct {
	EmployeeID     string `json:"employee_id"`
	WorkDate       string `json:"work_date"`
	ExistingStatus string `json:"existing_status"`
}

func (a adminAPI) bulkWorkSchedules(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.buat")
	if !ok {
		return
	}
	var body rosterBulkInput
	if !readJSON(w, r, &body) {
		return
	}
	start, err := time.Parse("2006-01-02", body.From)
	if err != nil || start.Format("2006-01-02") != body.From {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal mulai tidak valid.")
		return
	}
	end, err := time.Parse("2006-01-02", body.To)
	if err != nil || end.Format("2006-01-02") != body.To || end.Before(start) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal akhir tidak valid.")
		return
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if len(body.EmployeeIDs) == 0 || len(body.EmployeeIDs) > 50 || days > 31 || len(body.EmployeeIDs)*days > 200 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Maksimal 50 karyawan, 31 hari, dan 200 sel per permintaan.")
		return
	}
	seen := map[string]bool{}
	for _, id := range body.EmployeeIDs {
		if len(id) != 26 || seen[id] {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Daftar karyawan tidak valid atau duplikat.")
			return
		}
		seen[id] = true
	}
	if body.OfficeID != nil && len(*body.OfficeID) != 26 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kantor tidak valid.")
		return
	}
	pattern, err := a.readPattern(r, c.tenant.ID, body.PatternID, true)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pola tidak ditemukan dalam tenant.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pola.")
		return
	}
	if !pattern.IsActive || !validatePatternSlots(pattern.PatternType, pattern.Slots) || !a.validatePatternShifts(r.Context(), c.tenant.ID, pattern.Slots) {
		writeAPIError(w, r, 409, "PATTERN_UNAVAILABLE", "Pola tidak aktif, tidak lengkap, atau memuat shift tidak aktif.")
		return
	}
	bySlot := map[schedule.Slot]patternSlot{}
	for _, slot := range pattern.Slots {
		bySlot[schedule.Slot{WeekNo: slot.WeekNo, Weekday: slot.Weekday}] = slot
	}
	var chosenOfficeRegion string
	if body.OfficeID != nil {
		err = a.db.QueryRowContext(r.Context(), `SELECT region_id FROM offices WHERE tenant_id=? AND id=?`, c.tenant.ID, *body.OfficeID).Scan(&chosenOfficeRegion)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Kantor tidak ditemukan dalam tenant.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kantor.")
			return
		}
	}
	proposed := make([]workSchedule, 0, len(body.EmployeeIDs)*days)
	for _, employeeID := range body.EmployeeIDs {
		e, err := scanEmployee(a.db.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=? AND e.status IN ('active','probation','leave')`, c.tenant.ID, employeeID))
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam tenant.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
			return
		}
		target, err := a.targetFor(r.Context(), c.tenant.ID, e)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca penugasan.")
			return
		}
		if !c.effective.Includes(c.subject, target) {
			writeAPIError(w, r, 403, "SCOPE_DENIED", "Karyawan di luar scope roster.")
			return
		}
		if body.OfficeID != nil {
			target.BaseOfficeID = *body.OfficeID
			target.RegionID = chosenOfficeRegion
			if !c.effective.Includes(c.subject, target) {
				writeAPIError(w, r, 403, "SCOPE_DENIED", "Kantor tujuan di luar scope roster.")
				return
			}
		}
		for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
			key, _ := schedule.SlotForDate(pattern.PatternType, date)
			slot := bySlot[key]
			item := workSchedule{EmployeeID: employeeID, WorkDate: date.Format("2006-01-02"), DayType: slot.DayType, ShiftTemplateID: slot.ShiftTemplateID, Status: "draft", Version: 1, SourcePatternID: &pattern.ID, SourcePatternVersion: &pattern.Version}
			if item.DayType == "shift" {
				if body.OfficeID != nil {
					item.OfficeID = body.OfficeID
				} else {
					office := e.BaseOfficeID
					item.OfficeID = &office
				}
			}
			proposed = append(proposed, item)
		}
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(body.EmployeeIDs)), ",")
	args := []any{c.tenant.ID, body.From, body.To}
	for _, id := range body.EmployeeIDs {
		args = append(args, id)
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT employee_id,work_date,status FROM work_schedules WHERE tenant_id=? AND work_date>=? AND work_date<=? AND employee_id IN (`+marks+`)`, args...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa benturan roster.")
		return
	}
	conflicts := []rosterBulkConflict{}
	for rows.Next() {
		var item rosterBulkConflict
		var date time.Time
		if err := rows.Scan(&item.EmployeeID, &date, &item.ExistingStatus); err != nil {
			rows.Close()
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa benturan roster.")
			return
		}
		item.WorkDate = date.Format("2006-01-02")
		conflicts = append(conflicts, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa benturan roster.")
		return
	}
	rows.Close()
	if body.DryRun {
		writeAPIData(w, 200, map[string]any{"pattern_id": pattern.ID, "pattern_version": pattern.Version, "proposed": proposed, "conflicts": conflicts, "can_apply": len(conflicts) == 0}, nil)
		return
	}
	if len(conflicts) > 0 {
		writeAPIError(w, r, 409, "ROSTER_CONFLICT", "Ada roster pada tanggal yang dipilih; jalankan dry_run untuk rinciannya.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan roster.")
		return
	}
	defer tx.Rollback()
	var version int
	var active bool
	if err = tx.QueryRowContext(r.Context(), `SELECT version,is_active FROM schedule_patterns WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, pattern.ID).Scan(&version, &active); err != nil || version != pattern.Version || !active {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pola sudah berubah.")
		return
	}
	for i := range proposed {
		id, err := ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID roster.")
			return
		}
		proposed[i].ID = id
		item := proposed[i]
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO work_schedules (tenant_id,id,employee_id,work_date,shift_template_id,day_type,office_id,source_pattern_id,source_pattern_version,status,created_by,version) VALUES (?,?,?,?,?,?,?,?,?,'draft',?,1)`, c.tenant.ID, id, item.EmployeeID, item.WorkDate, item.ShiftTemplateID, item.DayType, item.OfficeID, pattern.ID, pattern.Version, c.state.User.ID); err != nil {
			writeAPIError(w, r, 409, "ROSTER_CONFLICT", "Roster bertabrakan atau shift berubah; tidak ada baris yang disimpan.")
			return
		}
		if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "work_schedules", id, nil, item); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan roster.")
		return
	}
	writeAPIData(w, 201, map[string]any{"created": proposed, "count": len(proposed)}, nil)
}
