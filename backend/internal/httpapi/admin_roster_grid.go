package httpapi

import (
	"database/sql"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
)

// rosterGrid presents a bounded employee-by-day view without weakening the
// row scope applied by the ordinary roster list.
func (a adminAPI) rosterGrid(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "jadwal.lihat")
	if !ok {
		return
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	start, e1 := time.Parse("2006-01-02", from)
	end, e2 := time.Parse("2006-01-02", to)
	if e1 != nil || e2 != nil || start.Format("2006-01-02") != from || end.Format("2006-01-02") != to || end.Before(start) || end.Sub(start) > 30*24*time.Hour {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Rentang grid wajib dan maksimal 31 hari.")
		return
	}
	officeID, departmentID := r.URL.Query().Get("office_id"), r.URL.Query().Get("department_id")
	if (officeID == "" && departmentID == "") || (officeID != "" && departmentID != "") || (officeID != "" && len(officeID) != 26) || (departmentID != "" && len(departmentID) != 26) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Pilih satu kantor atau departemen.")
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	where := "e.tenant_id=? AND e.status IN ('active','probation','leave') AND " + scope
	args := append([]any{c.tenant.ID}, scopeArgs...)
	if officeID != "" {
		where += " AND a.office_id=?"
		args = append(args, officeID)
	} else {
		where += " AND a.department_id=?"
		args = append(args, departmentID)
	}
	join := ` FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`
	var total int
	if err := a.db.QueryRowContext(r.Context(), "SELECT COUNT(*)"+join+" WHERE "+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca grid roster.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT e.id,e.display_name,a.office_id,a.department_id"+join+" WHERE "+where+" ORDER BY e.display_name,e.id LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca grid roster.")
		return
	}
	type line struct {
		EmployeeID   string         `json:"employee_id"`
		EmployeeName string         `json:"employee_name"`
		OfficeID     string         `json:"office_id"`
		DepartmentID *string        `json:"department_id"`
		Days         []workSchedule `json:"days"`
	}
	items := []line{}
	for rows.Next() {
		var item line
		var department sql.NullString
		if err = rows.Scan(&item.EmployeeID, &item.EmployeeName, &item.OfficeID, &department); err != nil {
			break
		}
		if department.Valid {
			item.DepartmentID = &department.String
		}
		item.Days = []workSchedule{}
		items = append(items, item)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca grid roster.")
		return
	}
	if len(items) > 0 {
		indexed := map[string]int{}
		marks := ""
		cellArgs := []any{c.tenant.ID, from, to}
		for i, item := range items {
			indexed[item.EmployeeID] = i
			if i > 0 {
				marks += ","
			}
			marks += "?"
			cellArgs = append(cellArgs, item.EmployeeID)
		}
		cells, e := a.db.QueryContext(r.Context(), "SELECT "+rosterColumns+" FROM work_schedules w WHERE w.tenant_id=? AND w.work_date>=? AND w.work_date<=? AND w.employee_id IN ("+marks+") AND w.status<>'cancelled' ORDER BY w.work_date,w.employee_id", cellArgs...)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca sel roster.")
			return
		}
		for cells.Next() {
			item, e := scanWorkSchedule(cells)
			if e != nil {
				err = e
				break
			}
			i := indexed[item.EmployeeID]
			items[i].Days = append(items[i].Days, item)
		}
		if err == nil {
			err = cells.Err()
		}
		cells.Close()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca sel roster.")
			return
		}
	}
	writeAPIData(w, 200, map[string]any{"from": from, "to": to, "employees": items}, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}
