package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type shiftInstance struct {
	ID                  string          `json:"id"`
	EmployeeID          string          `json:"employee_id"`
	WorkScheduleID      string          `json:"work_schedule_id"`
	ShiftTemplateID     string          `json:"shift_template_id"`
	OfficeID            string          `json:"office_id"`
	WorkDate            string          `json:"work_date"`
	Timezone            string          `json:"timezone"`
	ExpectedStart       time.Time       `json:"expected_start"`
	ExpectedEnd         time.Time       `json:"expected_end"`
	ClockInWindowStart  time.Time       `json:"clock_in_window_start_at"`
	ClockInWindowEnd    time.Time       `json:"clock_in_window_end_at"`
	ClockOutWindowStart time.Time       `json:"clock_out_window_start_at"`
	ClockOutWindowEnd   time.Time       `json:"clock_out_window_end_at"`
	RuleSnapshot        json.RawMessage `json:"rule_snapshot"`
	Status              string          `json:"status"`
}

const instanceColumns = `i.id,i.employee_id,i.work_schedule_id,i.shift_template_id,i.office_id,i.work_date,i.timezone,i.expected_start,i.expected_end,i.clock_in_window_start_at,i.clock_in_window_end_at,i.clock_out_window_start_at,i.clock_out_window_end_at,i.rule_snapshot,i.status`
const instanceJoin = ` FROM shift_instances i JOIN employees e ON e.tenant_id=i.tenant_id AND e.id=i.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`

func scanInstance(row interface{ Scan(...any) error }) (shiftInstance, error) {
	var item shiftInstance
	var date time.Time
	var raw []byte
	err := row.Scan(&item.ID, &item.EmployeeID, &item.WorkScheduleID, &item.ShiftTemplateID, &item.OfficeID, &date, &item.Timezone, &item.ExpectedStart, &item.ExpectedEnd, &item.ClockInWindowStart, &item.ClockInWindowEnd, &item.ClockOutWindowStart, &item.ClockOutWindowEnd, &raw, &item.Status)
	if err != nil {
		return item, err
	}
	item.WorkDate = date.Format("2006-01-02")
	item.RuleSnapshot = json.RawMessage(raw)
	return item, nil
}
func (a adminAPI) listShiftInstances(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	filter, filterArgs := access.EmployeeWhere(c.subject, c.effective)
	where := "i.tenant_id=? AND " + filter
	args := append([]any{c.tenant.ID}, filterArgs...)
	for _, entry := range []struct{ query, column string }{{"employee_id", "i.employee_id"}, {"office_id", "i.office_id"}} {
		if value := r.URL.Query().Get(entry.query); value != "" {
			if len(value) != 26 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter instance tidak valid.")
				return
			}
			where += " AND " + entry.column + "=?"
			args = append(args, value)
		}
	}
	for _, entry := range []struct{ query, op string }{{"from", ">="}, {"to", "<="}} {
		if value := r.URL.Query().Get(entry.query); value != "" {
			date, err := time.Parse("2006-01-02", value)
			if err != nil || date.Format("2006-01-02") != value {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal filter tidak valid.")
				return
			}
			where += " AND i.work_date" + entry.op + "?"
			args = append(args, value)
		}
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*)"+instanceJoin+" WHERE "+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift instance.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT "+instanceColumns+instanceJoin+" WHERE "+where+" ORDER BY i.work_date,i.employee_id LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift instance.")
		return
	}
	defer rows.Close()
	items := []shiftInstance{}
	for rows.Next() {
		item, err := scanInstance(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift instance.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift instance.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}
func (a adminAPI) getShiftInstance(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.lihat")
	if !ok {
		return
	}
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, chi.URLParam(r, "id")}, args...)
	item, err := scanInstance(a.db.QueryRowContext(r.Context(), "SELECT "+instanceColumns+instanceJoin+" WHERE i.tenant_id=? AND i.id=? AND "+filter, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Shift instance tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca shift instance.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

type essScheduleDay struct {
	WorkDate      string          `json:"work_date"`
	DayType       string          `json:"day_type"`
	RosterID      string          `json:"roster_id"`
	InstanceID    *string         `json:"shift_instance_id"`
	Status        *string         `json:"status"`
	Timezone      *string         `json:"timezone"`
	ExpectedStart *time.Time      `json:"expected_start"`
	ExpectedEnd   *time.Time      `json:"expected_end"`
	RuleSnapshot  json.RawMessage `json:"rule_snapshot"`
}

func (a adminAPI) essSchedule(w http.ResponseWriter, r *http.Request) {
	state, err := a.auth.Current(r.Context(), cookieToken(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	var ess bool
	for _, tenant := range state.Tenants {
		if tenant.ID == state.SelectedTenantID && tenant.ESS {
			ess = true
			break
		}
	}
	if !ess {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Panel ESS tidak tersedia.")
		return
	}
	location := time.UTC
	for _, tenant := range state.Tenants {
		if tenant.ID == state.SelectedTenantID {
			if loc, err := time.LoadLocation(tenant.Timezone); err == nil {
				location = loc
			}
			break
		}
	}
	today := time.Now().In(location)
	from := today.Format("2006-01-02")
	to := today.AddDate(0, 0, 30).Format("2006-01-02")
	if value := r.URL.Query().Get("from"); value != "" {
		from = value
	}
	if value := r.URL.Query().Get("to"); value != "" {
		to = value
	}
	start, err := time.Parse("2006-01-02", from)
	if err != nil || start.Format("2006-01-02") != from {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal mulai tidak valid.")
		return
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil || end.Format("2006-01-02") != to || end.Before(start) || end.Sub(start) > 90*24*time.Hour {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Rentang jadwal maksimal 91 hari.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT w.id,w.work_date,w.day_type,i.id,i.status,i.timezone,i.expected_start,i.expected_end,i.rule_snapshot FROM work_schedules w JOIN tenant_memberships m ON m.tenant_id=w.tenant_id AND m.employee_id=w.employee_id AND m.status='active' LEFT JOIN shift_instances i ON i.tenant_id=w.tenant_id AND i.work_schedule_id=w.id WHERE w.tenant_id=? AND m.user_id=? AND w.status='published' AND w.work_date>=? AND w.work_date<=? ORDER BY w.work_date`, state.SelectedTenantID, state.User.ID, from, to)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jadwal.")
		return
	}
	defer rows.Close()
	items := []essScheduleDay{}
	for rows.Next() {
		var item essScheduleDay
		var date time.Time
		var instanceID, status, timezone sql.NullString
		var expectedStart, expectedEnd sql.NullTime
		var snapshot []byte
		if err := rows.Scan(&item.RosterID, &date, &item.DayType, &instanceID, &status, &timezone, &expectedStart, &expectedEnd, &snapshot); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jadwal.")
			return
		}
		item.WorkDate = date.Format("2006-01-02")
		if instanceID.Valid {
			item.InstanceID = &instanceID.String
		}
		if status.Valid {
			item.Status = &status.String
		}
		if timezone.Valid {
			item.Timezone = &timezone.String
		}
		if expectedStart.Valid {
			item.ExpectedStart = &expectedStart.Time
		}
		if expectedEnd.Valid {
			item.ExpectedEnd = &expectedEnd.Time
		}
		if len(snapshot) > 0 {
			item.RuleSnapshot = json.RawMessage(snapshot)
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jadwal.")
		return
	}
	writeAPIData(w, 200, items, nil)
}
