package httpapi

import (
	"database/sql"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
)

type attendanceNotYetRow struct {
	EmployeeID      string     `json:"employee_id"`
	EmployeeName    string     `json:"employee_name"`
	WorkDate        string     `json:"work_date"`
	ShiftInstanceID *string    `json:"shift_instance_id"`
	ExpectedStart   *time.Time `json:"expected_start"`
	Category        string     `json:"category"`
}

const attendanceNotYetJoin = ` FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE LEFT JOIN shift_instances i ON i.tenant_id=e.tenant_id AND i.employee_id=e.id AND i.work_date=? AND i.status<>'cancelled' LEFT JOIN attendance_daily d ON d.tenant_id=i.tenant_id AND d.shift_instance_id=i.id`
const attendanceNotYetCategory = `CASE WHEN EXISTS (SELECT 1 FROM leave_requests l WHERE l.tenant_id=e.tenant_id AND l.employee_id=e.id AND l.status='approved' AND ? BETWEEN l.start_date AND l.end_date) THEN 'izin_cuti' WHEN EXISTS (SELECT 1 FROM holidays h WHERE h.tenant_id=e.tenant_id AND h.holiday_date=? AND (h.region_id IS NULL OR h.region_id=a.region_id)) THEN 'libur' WHEN i.id IS NULL THEN 'tidak_dijadwalkan' WHEN EXISTS (SELECT 1 FROM attendance_corrections c WHERE c.tenant_id=i.tenant_id AND c.shift_instance_id=i.id AND c.status IN ('in_review','returned')) THEN 'menunggu_koreksi' WHEN i.clock_in_window_start_at>UTC_TIMESTAMP(6) THEN 'belum_waktunya' ELSE 'belum_absen' END`

func (a adminAPI) attendanceNotYet(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "absensi.monitor_bawahan")
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
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	where := ` WHERE e.tenant_id=? AND e.status IN ('active','probation','leave') AND d.first_in IS NULL AND ` + filter
	args := append([]any{day, c.tenant.ID}, scopeArgs...)
	for _, entry := range []struct{ key, column string }{{"employee_id", "e.id"}, {"office_id", "a.office_id"}, {"department_id", "a.department_id"}} {
		if value := r.URL.Query().Get(entry.key); value != "" {
			if !fileIDPattern.MatchString(value) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter belum absen tidak valid.")
				return
			}
			where += ` AND ` + entry.column + `=?`
			args = append(args, value)
		}
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+attendanceNotYetJoin+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca status belum absen.")
		return
	}
	queryArgs := append([]any{day, day}, args...)
	queryArgs = append(queryArgs, page.PageSize, page.Offset)
	rows, err := a.db.QueryContext(r.Context(), `SELECT e.id,e.display_name,i.id,i.expected_start,`+attendanceNotYetCategory+attendanceNotYetJoin+where+` ORDER BY e.display_name,e.id LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca status belum absen.")
		return
	}
	defer rows.Close()
	items := []attendanceNotYetRow{}
	for rows.Next() {
		var item attendanceNotYetRow
		var instance sql.NullString
		var expected sql.NullTime
		if err = rows.Scan(&item.EmployeeID, &item.EmployeeName, &instance, &expected, &item.Category); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca status belum absen.")
			return
		}
		item.WorkDate = day
		if instance.Valid {
			item.ShiftInstanceID = &instance.String
		}
		if expected.Valid {
			item.ExpectedStart = &expected.Time
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca status belum absen.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}
