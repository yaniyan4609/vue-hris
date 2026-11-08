package httpapi

import (
	"database/sql"
	"errors"
	"math/big"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// This rounds to the nearest cent, with halves rounded up. A big integer
// prevents overflow when a large DECIMAL(18,2) amount is multiplied by days.
func prorateFixedCents(amount int64, workedDays, periodWorkDays int) (int64, bool) {
	if amount < 0 || workedDays < 0 || periodWorkDays <= 0 || workedDays > periodWorkDays {
		return 0, false
	}
	numerator := new(big.Int).Mul(big.NewInt(amount), big.NewInt(int64(workedDays)))
	numerator.Add(numerator, big.NewInt(int64(periodWorkDays/2)))
	numerator.Div(numerator, big.NewInt(int64(periodWorkDays)))
	if !numerator.IsInt64() {
		return 0, false
	}
	return numerator.Int64(), true
}

func (a adminAPI) previewPayrollProration(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	periodID := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(periodID) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
		return
	}
	employeeID := r.URL.Query().Get("employee_id")
	scope, scopeArgs, ok := a.payEmployeeScope(w, r, c, employeeID, false)
	if !ok {
		return
	}
	var start, end time.Time
	err := a.db.QueryRowContext(r.Context(), `SELECT start_date,end_date FROM payroll_periods WHERE tenant_id=? AND id=?`, c.tenant.ID, periodID).Scan(&start, &end)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca periode payroll.")
		return
	}
	var joined, departed sql.NullTime
	err = a.db.QueryRowContext(r.Context(), `SELECT e.join_date,e.end_date FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=? AND `+scope, append([]any{c.tenant.ID, employeeID}, scopeArgs...)...).Scan(&joined, &departed)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope payroll.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tanggal kerja karyawan.")
		return
	}
	from, until := start, end
	if joined.Valid && joined.Time.After(from) {
		from = joined.Time
	}
	if departed.Valid && departed.Time.Before(until) {
		until = departed.Time
	}
	startUTC := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endUTC := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	calendarDays := int(endUTC.Sub(startUTC).Hours()/24) + 1
	if calendarDays < 1 || calendarDays > 366 {
		writeAPIError(w, r, 409, "PERIOD_INVALID", "Rentang periode payroll tidak valid.")
		return
	}
	var publishedDays, periodWorkDays, employedWorkDays int
	err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(day_type='shift'),0),COALESCE(SUM(day_type='shift' AND work_date>=? AND work_date<=?),0) FROM work_schedules WHERE tenant_id=? AND employee_id=? AND status='published' AND work_date>=? AND work_date<=?`, from.Format("2006-01-02"), until.Format("2006-01-02"), c.tenant.ID, employeeID, start.Format("2006-01-02"), end.Format("2006-01-02")).Scan(&publishedDays, &periodWorkDays, &employedWorkDays)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca roster terbit.")
		return
	}
	ready := publishedDays == calendarDays && periodWorkDays > 0
	issues := []string{}
	if !joined.Valid {
		issues = append(issues, "JOIN_DATE_MISSING")
		ready = false
	}
	if joined.Valid && departed.Valid && departed.Time.Before(joined.Time) {
		issues = append(issues, "EMPLOYMENT_DATES_INVALID")
		ready = false
	}
	if from.After(until) {
		issues = append(issues, "NOT_EMPLOYED_IN_PERIOD")
		ready = false
	}
	if publishedDays < calendarDays {
		issues = append(issues, "ROSTER_INCOMPLETE")
	}
	if periodWorkDays == 0 {
		issues = append(issues, "NO_WORK_DAYS")
	}
	writeAPIData(w, 200, map[string]any{"period_id": periodID, "employee_id": employeeID, "period_start": start.Format("2006-01-02"), "period_end": end.Format("2006-01-02"), "calendar_days": calendarDays, "published_roster_days": publishedDays, "missing_roster_days": calendarDays - publishedDays, "period_work_days": periodWorkDays, "employed_work_days": employedWorkDays, "ready": ready, "issues": issues, "preview_only": true}, nil)
}
