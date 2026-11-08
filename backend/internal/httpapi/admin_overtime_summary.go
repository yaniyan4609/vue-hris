package httpapi

import (
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
)

func (a adminAPI) adminOvertimeSummary(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "lembur.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	now := time.Now().In(zone)
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if from == "" {
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, zone).Format("2006-01-02")
	}
	if to == "" {
		to = time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, zone).Format("2006-01-02")
	}
	start, e1 := time.Parse("2006-01-02", from)
	end, e2 := time.Parse("2006-01-02", to)
	if e1 != nil || e2 != nil || start.Format("2006-01-02") != from || end.Format("2006-01-02") != to || end.Before(start) || end.Sub(start) > 366*24*time.Hour {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Periode lembur tidak valid atau melebihi satu tahun.")
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	base := overtimeAdminFrom + ` WHERE o.tenant_id=? AND ` + scope + ` AND o.work_date>=? AND o.work_date<=?`
	args := append([]any{c.tenant.ID}, scopeArgs...)
	args = append(args, from, to)
	if employeeID := r.URL.Query().Get("employee_id"); employeeID != "" {
		if len(employeeID) != 26 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID karyawan tidak valid.")
			return
		}
		base += ` AND o.employee_id=?`
		args = append(args, employeeID)
	}
	var requests, proposed, approved, paid int64
	if err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(o.proposed_minutes),0),COALESCE(SUM(o.approved_minutes),0),COALESCE(SUM(o.paid_minutes),0)`+base, args...).Scan(&requests, &proposed, &approved, &paid); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca rekap lembur.")
		return
	}
	statusCounts := map[string]int64{}
	rows, err := a.db.QueryContext(r.Context(), `SELECT o.status,COUNT(*)`+base+` GROUP BY o.status`, args...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca status lembur.")
		return
	}
	for rows.Next() {
		var status string
		var count int64
		if err = rows.Scan(&status, &count); err != nil {
			break
		}
		statusCounts[status] = count
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca status lembur.")
		return
	}
	var employees int64
	if err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(DISTINCT o.employee_id)`+base, args...).Scan(&employees); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jumlah karyawan.")
		return
	}
	rows, err = a.db.QueryContext(r.Context(), `SELECT o.employee_id,e.display_name,COUNT(*),COALESCE(SUM(o.proposed_minutes),0),COALESCE(SUM(o.approved_minutes),0),COALESCE(SUM(o.paid_minutes),0)`+base+` GROUP BY o.employee_id,e.display_name ORDER BY e.display_name,o.employee_id LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca rincian lembur.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var employeeID, name string
		var count, proposed, approved, paid int64
		if err = rows.Scan(&employeeID, &name, &count, &proposed, &approved, &paid); err != nil {
			break
		}
		items = append(items, map[string]any{"employee_id": employeeID, "display_name": name, "requests": count, "proposed_minutes": proposed, "approved_minutes": approved, "paid_minutes": paid})
	}
	if err != nil || rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca rincian lembur.")
		return
	}
	writeAPIData(w, 200, map[string]any{"from": from, "to": to, "requests": requests, "proposed_minutes": proposed, "approved_minutes": approved, "paid_minutes": paid, "status_counts": statusCounts, "employees": items}, map[string]any{"page": page.Page, "per_page": page.PageSize, "total": employees})
}
