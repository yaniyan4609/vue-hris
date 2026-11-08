package httpapi

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

const adminLeaveBalanceColumns = `SELECT e.id,t.id,t.code,t.name,COALESCE(CAST(l.balance AS CHAR),'0.00'),COALESCE(CAST(res.reserved AS CHAR),'0.00'),COALESCE(CAST(COALESCE(l.balance,0)-COALESCE(res.reserved,0) AS CHAR),'0.00')`

func adminLeaveBalanceFrom(scope string) string {
	return ` FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE JOIN leave_types t ON t.tenant_id=e.tenant_id AND t.is_active=TRUE` +
		` LEFT JOIN (SELECT employee_id,leave_type_id,SUM(days) AS balance FROM leave_balance_ledger WHERE tenant_id=? AND effective_date>=? AND effective_date<? GROUP BY employee_id,leave_type_id) l ON l.employee_id=e.id AND l.leave_type_id=t.id` +
		` LEFT JOIN (SELECT lr.employee_id,lr.leave_type_id,SUM(al.days) AS reserved FROM leave_request_allocations al JOIN leave_requests lr ON lr.tenant_id=al.tenant_id AND lr.id=al.leave_request_id WHERE al.tenant_id=? AND al.period_year=? AND lr.status IN ('submitted','in_review') GROUP BY lr.employee_id,lr.leave_type_id) res ON res.employee_id=e.id AND res.leave_type_id=t.id` +
		` WHERE e.tenant_id=? AND ` + scope
}

func adminBalanceYear(w http.ResponseWriter, r *http.Request, c adminContext) (int, bool) {
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	return leaveYear(w, r, zone)
}

func adminBalanceArgs(c adminContext, year int, scopeArgs []any) []any {
	from, to := fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-01-01", year+1)
	args := []any{c.tenant.ID, from, to, c.tenant.ID, year, c.tenant.ID}
	return append(args, scopeArgs...)
}

func scanAdminBalance(row interface{ Scan(...any) error }, year int) (map[string]any, error) {
	var employeeID, typeID, code, name, balance, reserved, available string
	if err := row.Scan(&employeeID, &typeID, &code, &name, &balance, &reserved, &available); err != nil {
		return nil, err
	}
	return map[string]any{"employee_id": employeeID, "leave_type_id": typeID, "code": code, "name": name, "year": year, "balance_days": balance, "reserved_days": reserved, "available_days": available}, nil
}

func (a adminAPI) listAdminLeaveBalances(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.lihat")
	if !ok {
		return
	}
	year, ok := adminBalanceYear(w, r, c)
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	base := adminLeaveBalanceFrom(scope)
	args := adminBalanceArgs(c, year, scopeArgs)
	if employeeID := r.URL.Query().Get("employee_id"); employeeID != "" {
		if len(employeeID) != 26 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID karyawan tidak valid.")
			return
		}
		base += " AND e.id=?"
		args = append(args, employeeID)
	}
	if typeID := r.URL.Query().Get("leave_type_id"); typeID != "" {
		if len(typeID) != 26 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID jenis cuti tidak valid.")
			return
		}
		base += " AND t.id=?"
		args = append(args, typeID)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+base, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca saldo cuti.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), adminLeaveBalanceColumns+base+` ORDER BY e.id,t.name,t.id LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca saldo cuti.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		item, e := scanAdminBalance(rows, year)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca saldo cuti.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca saldo cuti.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) getAdminEmployeeLeaveBalances(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.lihat")
	if !ok {
		return
	}
	year, ok := adminBalanceYear(w, r, c)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan.")
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	base := adminLeaveBalanceFrom(scope) + ` AND e.id=?`
	args := append(adminBalanceArgs(c, year, scopeArgs), id)
	rows, err := a.db.QueryContext(r.Context(), adminLeaveBalanceColumns+base+` ORDER BY t.name,t.id`, args...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca saldo cuti.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		item, e := scanAdminBalance(rows, year)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca saldo cuti.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca saldo cuti.")
		return
	}
	if len(items) == 0 {
		var exists int
		checkArgs := append([]any{c.tenant.ID, id}, scopeArgs...)
		if err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=? AND `+scope, checkArgs...).Scan(&exists); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope karyawan.")
			return
		}
		if exists == 0 {
			writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
			return
		}
	}
	writeAPIData(w, 200, map[string]any{"employee_id": id, "year": year, "balances": items}, nil)
}

func (a adminAPI) listAdminLeaveLedger(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	year, ok := adminBalanceYear(w, r, c)
	if !ok {
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	from, to := fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-01-01", year+1)
	base := ` FROM leave_balance_ledger l JOIN employees e ON e.tenant_id=l.tenant_id AND e.id=l.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE l.tenant_id=? AND ` + scope + ` AND l.effective_date>=? AND l.effective_date<?`
	args := append([]any{c.tenant.ID}, scopeArgs...)
	args = append(args, from, to)
	for _, filter := range []struct{ key, column string }{{"employee_id", "l.employee_id"}, {"leave_type_id", "l.leave_type_id"}} {
		if value := r.URL.Query().Get(filter.key); value != "" {
			if len(value) != 26 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID filter tidak valid.")
				return
			}
			base += " AND " + filter.column + "=?"
			args = append(args, value)
		}
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+base, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ledger cuti.")
		return
	}
	query := `SELECT l.id,l.employee_id,l.leave_type_id,l.entry_type,CAST(l.days AS CHAR),l.effective_date,l.ref_type,l.ref_id,l.note,l.created_at` + base + ` ORDER BY l.effective_date DESC,l.created_at DESC,l.id DESC LIMIT ? OFFSET ?`
	rows, err := a.db.QueryContext(r.Context(), query, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ledger cuti.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, employeeID, typeID, kind, days, refType, refID string
		var date, created time.Time
		var note sql.NullString
		if err = rows.Scan(&id, &employeeID, &typeID, &kind, &days, &date, &refType, &refID, &note, &created); err != nil {
			break
		}
		var noteValue any
		if note.Valid {
			noteValue = note.String
		}
		items = append(items, map[string]any{"id": id, "employee_id": employeeID, "leave_type_id": typeID, "entry_type": kind, "days": days, "effective_date": date.Format("2006-01-02"), "ref_type": refType, "ref_id": refID, "note": noteValue, "created_at": created})
	}
	if err != nil || rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ledger cuti.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) adminLeaveCalendar(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	zone, e := time.LoadLocation(c.tenant.Timezone)
	if e != nil {
		zone = time.UTC
	}
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().In(zone).Format("2006-01")
	}
	first, e := time.Parse("2006-01", month)
	if e != nil || first.Format("2006-01") != month {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Bulan harus YYYY-MM.")
		return
	}
	next := first.AddDate(0, 1, 0).Format("2006-01-02")
	start := first.Format("2006-01-02")
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	base := ` FROM leave_requests lr JOIN leave_types lt ON lt.tenant_id=lr.tenant_id AND lt.id=lr.leave_type_id JOIN employees e ON e.tenant_id=lr.tenant_id AND e.id=lr.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE lr.tenant_id=? AND ` + scope + ` AND lr.status='approved' AND lr.start_date<? AND lr.end_date>=?`
	args := append([]any{c.tenant.ID}, scopeArgs...)
	args = append(args, next, start)
	if officeID := r.URL.Query().Get("office_id"); officeID != "" {
		if len(officeID) != 26 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID kantor tidak valid.")
			return
		}
		base += ` AND a.office_id=?`
		args = append(args, officeID)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+base, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kalender cuti.")
		return
	}
	query := `SELECT lr.id,lr.employee_id,lt.name,lr.start_date,lr.end_date,CAST(lr.days AS CHAR),a.office_id` + base + ` ORDER BY lr.start_date,lr.id LIMIT ? OFFSET ?`
	rows, err := a.db.QueryContext(r.Context(), query, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kalender cuti.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, employeeID, typeName, days, officeID string
		var from, to time.Time
		if err = rows.Scan(&id, &employeeID, &typeName, &from, &to, &days, &officeID); err != nil {
			break
		}
		visibleStart, visibleEnd := from.Format("2006-01-02"), to.Format("2006-01-02")
		if visibleStart < start {
			visibleStart = start
		}
		last := first.AddDate(0, 1, -1).Format("2006-01-02")
		if visibleEnd > last {
			visibleEnd = last
		}
		items = append(items, map[string]any{"id": id, "employee_id": employeeID, "leave_type_name": typeName, "office_id": officeID, "start_date": from.Format("2006-01-02"), "end_date": to.Format("2006-01-02"), "visible_start_date": visibleStart, "visible_end_date": visibleEnd, "days": days, "status": "approved"})
	}
	if err != nil || rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kalender cuti.")
		return
	}
	writeAPIData(w, 200, items, map[string]any{"month": month, "page": page.Page, "per_page": page.PageSize, "total": total})
}
