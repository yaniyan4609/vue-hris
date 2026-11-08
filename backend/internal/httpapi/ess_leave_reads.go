package httpapi

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type essLeaveType struct {
	ID                 string `json:"id"`
	Code               string `json:"code"`
	Name               string `json:"name"`
	IsPaid             bool   `json:"is_paid"`
	RequiresAttachment bool   `json:"requires_attachment"`
	MaxConsecutiveDays *int   `json:"max_consecutive_days"`
}

func scanEssLeaveType(row interface{ Scan(...any) error }) (essLeaveType, error) {
	var item essLeaveType
	var maximum sql.NullInt64
	err := row.Scan(&item.ID, &item.Code, &item.Name, &item.IsPaid, &item.RequiresAttachment, &maximum)
	if maximum.Valid {
		value := int(maximum.Int64)
		item.MaxConsecutiveDays = &value
	}
	return item, err
}

func (a adminAPI) listEssLeaveTypes(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,code,name,is_paid,requires_attachment,max_consecutive_days FROM leave_types WHERE tenant_id=? AND is_active=TRUE ORDER BY name,id`, c.state.SelectedTenantID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jenis cuti.")
		return
	}
	defer rows.Close()
	items := []essLeaveType{}
	for rows.Next() {
		item, err := scanEssLeaveType(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jenis cuti.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jenis cuti.")
		return
	}
	writeAPIData(w, 200, items, nil)
}

func leaveYear(w http.ResponseWriter, r *http.Request, zone *time.Location) (int, bool) {
	year := time.Now().In(zone).Year()
	if raw := r.URL.Query().Get("year"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 2000 || value > 2100 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tahun tidak valid.")
			return 0, false
		}
		year = value
	}
	return year, true
}

func (a adminAPI) listEssLeaveBalances(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	year, ok := leaveYear(w, r, c.zone)
	if !ok {
		return
	}
	from, to := strconv.Itoa(year)+"-01-01", strconv.Itoa(year+1)+"-01-01"
	rows, err := a.db.QueryContext(r.Context(), `SELECT t.id,t.code,t.name,COALESCE(CAST(SUM(l.days) AS CHAR),'0.00') FROM leave_types t LEFT JOIN leave_balance_ledger l ON l.tenant_id=t.tenant_id AND l.leave_type_id=t.id AND l.employee_id=? AND l.effective_date>=? AND l.effective_date<? WHERE t.tenant_id=? AND t.is_active=TRUE GROUP BY t.id,t.code,t.name ORDER BY t.name,t.id`, c.employeeID, from, to, c.state.SelectedTenantID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca saldo cuti.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, code, name, balance string
		if err := rows.Scan(&id, &code, &name, &balance); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca saldo cuti.")
			return
		}
		items = append(items, map[string]any{"leave_type_id": id, "code": code, "name": name, "year": year, "balance_days": balance})
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca saldo cuti.")
		return
	}
	writeAPIData(w, 200, items, nil)
}

func (a adminAPI) listEssLeaveLedger(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	typeID := chi.URLParam(r, "leaveTypeId")
	if len(typeID) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Jenis cuti tidak ditemukan.")
		return
	}
	var typeExists int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_types WHERE tenant_id=? AND id=?`, c.state.SelectedTenantID, typeID).Scan(&typeExists); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jenis cuti.")
		return
	}
	if typeExists != 1 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Jenis cuti tidak ditemukan.")
		return
	}
	year, ok := leaveYear(w, r, c.zone)
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	from, to := strconv.Itoa(year)+"-01-01", strconv.Itoa(year+1)+"-01-01"
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_balance_ledger WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND effective_date>=? AND effective_date<?`, c.state.SelectedTenantID, c.employeeID, typeID, from, to).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ledger cuti.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,entry_type,CAST(days AS CHAR),effective_date,ref_type,ref_id,note,created_at FROM leave_balance_ledger WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND effective_date>=? AND effective_date<? ORDER BY effective_date DESC,created_at DESC,id DESC LIMIT ? OFFSET ?`, c.state.SelectedTenantID, c.employeeID, typeID, from, to, page.PageSize, page.Offset)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ledger cuti.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, kind, days, refType, refID string
		var date, created time.Time
		var note sql.NullString
		if err := rows.Scan(&id, &kind, &days, &date, &refType, &refID, &note, &created); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ledger cuti.")
			return
		}
		var noteValue any
		if note.Valid {
			noteValue = note.String
		}
		items = append(items, map[string]any{"id": id, "entry_type": kind, "days": days, "effective_date": date.Format("2006-01-02"), "ref_type": refType, "ref_id": refID, "note": noteValue, "created_at": created})
	}
	if err := rows.Err(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ledger cuti.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func scanLeaveRequest(row interface{ Scan(...any) error }) (map[string]any, error) {
	var id, requestID, employeeID, typeID, typeName, status string
	var start, end time.Time
	var days string
	var reason, attachment sql.NullString
	err := row.Scan(&id, &requestID, &employeeID, &typeID, &typeName, &start, &end, &days, &reason, &attachment, &status)
	if err != nil {
		return nil, err
	}
	var reasonValue, attachmentValue any
	if reason.Valid {
		reasonValue = reason.String
	}
	if attachment.Valid {
		attachmentValue = attachment.String
	}
	return map[string]any{"id": id, "request_id": requestID, "employee_id": employeeID, "leave_type_id": typeID, "leave_type_name": typeName, "start_date": start.Format("2006-01-02"), "end_date": end.Format("2006-01-02"), "days": days, "reason": reasonValue, "attachment_file_id": attachmentValue, "status": status}, nil
}

const leaveRequestColumns = `SELECT lr.id,lr.request_id,lr.employee_id,lr.leave_type_id,lt.name,lr.start_date,lr.end_date,CAST(lr.days AS CHAR),lr.reason,lr.attachment_file_id,lr.status FROM leave_requests lr JOIN leave_types lt ON lt.tenant_id=lr.tenant_id AND lt.id=lr.leave_type_id`

func (a adminAPI) listEssLeave(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_requests WHERE tenant_id=? AND employee_id=?`, c.state.SelectedTenantID, c.employeeID).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan cuti.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), leaveRequestColumns+` WHERE lr.tenant_id=? AND lr.employee_id=? ORDER BY lr.start_date DESC,lr.id DESC LIMIT ? OFFSET ?`, c.state.SelectedTenantID, c.employeeID, page.PageSize, page.Offset)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan cuti.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		item, e := scanLeaveRequest(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan cuti.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan cuti.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) getEssLeave(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan cuti tidak ditemukan.")
		return
	}
	item, err := scanLeaveRequest(a.db.QueryRowContext(r.Context(), leaveRequestColumns+` WHERE lr.tenant_id=? AND lr.employee_id=? AND lr.id=?`, c.state.SelectedTenantID, c.employeeID, id))
	if err == sql.ErrNoRows {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan cuti tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan cuti.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT period_year,first_work_date,CAST(days AS CHAR) FROM leave_request_allocations WHERE tenant_id=? AND leave_request_id=? ORDER BY period_year`, c.state.SelectedTenantID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alokasi cuti.")
		return
	}
	defer rows.Close()
	allocations := []map[string]any{}
	for rows.Next() {
		var year int
		var first time.Time
		var days string
		if err = rows.Scan(&year, &first, &days); err != nil {
			break
		}
		allocations = append(allocations, map[string]any{"year": year, "first_work_date": first.Format("2006-01-02"), "days": days})
	}
	if err != nil || rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alokasi cuti.")
		return
	}
	item["allocations"] = allocations
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) listAdminLeave(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	where, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	filter, args, ok := adminLeaveFilters(w, r)
	if !ok {
		return
	}
	base := ` FROM leave_requests lr JOIN leave_types lt ON lt.tenant_id=lr.tenant_id AND lt.id=lr.leave_type_id JOIN employees e ON e.tenant_id=lr.tenant_id AND e.id=lr.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE lr.tenant_id=? AND ` + where + filter
	params := append([]any{c.tenant.ID}, scopeArgs...)
	params = append(params, args...)
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+base, params...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan cuti.")
		return
	}
	query := `SELECT lr.id,lr.request_id,lr.employee_id,lr.leave_type_id,lt.name,lr.start_date,lr.end_date,CAST(lr.days AS CHAR),lr.reason,lr.attachment_file_id,lr.status` + base + ` ORDER BY lr.start_date DESC,lr.id DESC LIMIT ? OFFSET ?`
	params = append(params, page.PageSize, page.Offset)
	rows, err := a.db.QueryContext(r.Context(), query, params...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan cuti.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		item, e := scanLeaveRequest(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan cuti.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan cuti.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func adminLeaveFilters(w http.ResponseWriter, r *http.Request) (string, []any, bool) {
	filter := ""
	args := []any{}
	for _, field := range []struct{ key, column string }{{"status", "lr.status"}, {"leave_type_id", "lr.leave_type_id"}, {"employee_id", "lr.employee_id"}} {
		value := r.URL.Query().Get(field.key)
		if value == "" {
			continue
		}
		if field.key == "status" {
			switch value {
			case "draft", "submitted", "in_review", "returned", "approved", "rejected", "cancelled":
			default:
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter status tidak valid.")
				return "", nil, false
			}
		} else if len(value) != 26 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID filter tidak valid.")
			return "", nil, false
		}
		filter += " AND " + field.column + "=?"
		args = append(args, value)
	}
	for _, field := range []struct{ key, column, op string }{{"from", "lr.end_date", ">="}, {"to", "lr.start_date", "<="}} {
		value := r.URL.Query().Get(field.key)
		if value == "" {
			continue
		}
		date, err := time.Parse("2006-01-02", value)
		if err != nil || date.Format("2006-01-02") != value {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter tanggal tidak valid.")
			return "", nil, false
		}
		filter += " AND " + field.column + field.op + "?"
		args = append(args, value)
	}
	return filter, args, true
}

func (a adminAPI) getAdminLeave(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan cuti tidak ditemukan.")
		return
	}
	where, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	query := leaveRequestColumns + ` JOIN employees e ON e.tenant_id=lr.tenant_id AND e.id=lr.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE lr.tenant_id=? AND lr.id=? AND ` + where
	args := append([]any{c.tenant.ID, id}, scopeArgs...)
	item, err := scanLeaveRequest(a.db.QueryRowContext(r.Context(), query, args...))
	if err == sql.ErrNoRows {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan cuti tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan cuti.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
