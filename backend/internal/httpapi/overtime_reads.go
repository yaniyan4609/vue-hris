package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type overtimeView struct {
	ID              string  `json:"id"`
	RequestID       *string `json:"request_id"`
	EmployeeID      string  `json:"employee_id"`
	WorkDate        string  `json:"work_date"`
	ShiftInstanceID *string `json:"shift_instance_id"`
	ProposedMinutes int     `json:"proposed_minutes"`
	ApprovedMinutes *int    `json:"approved_minutes"`
	PaidMinutes     *int    `json:"paid_minutes"`
	Reason          *string `json:"reason"`
	Status          string  `json:"status"`
}

const overtimeColumns = `o.id,o.request_id,o.employee_id,o.work_date,o.shift_instance_id,o.proposed_minutes,o.approved_minutes,o.paid_minutes,o.reason,o.status`
const overtimeAdminFrom = ` FROM overtime_requests o JOIN employees e ON e.tenant_id=o.tenant_id AND e.id=o.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`
const overtimeEssFrom = ` FROM overtime_requests o`

func scanOvertime(row interface{ Scan(...any) error }) (overtimeView, error) {
	var item overtimeView
	var requestID, shiftID, reason sql.NullString
	var approved, paid sql.NullInt64
	var workDate time.Time
	err := row.Scan(&item.ID, &requestID, &item.EmployeeID, &workDate, &shiftID, &item.ProposedMinutes, &approved, &paid, &reason, &item.Status)
	if err != nil {
		return item, err
	}
	item.WorkDate = workDate.Format("2006-01-02")
	if requestID.Valid {
		item.RequestID = &requestID.String
	}
	if shiftID.Valid {
		item.ShiftInstanceID = &shiftID.String
	}
	if approved.Valid {
		value := int(approved.Int64)
		item.ApprovedMinutes = &value
	}
	if paid.Valid {
		value := int(paid.Int64)
		item.PaidMinutes = &value
	}
	if reason.Valid {
		item.Reason = &reason.String
	}
	return item, nil
}

func overtimeFilters(w http.ResponseWriter, r *http.Request, includeEmployee bool) (string, []any, bool) {
	where := ""
	args := []any{}
	if status := r.URL.Query().Get("status"); status != "" {
		if len(status) > 24 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status tidak valid.")
			return "", nil, false
		}
		where += " AND o.status=?"
		args = append(args, status)
	}
	if includeEmployee {
		if employeeID := r.URL.Query().Get("employee_id"); employeeID != "" {
			if len(employeeID) != 26 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Karyawan tidak valid.")
				return "", nil, false
			}
			where += " AND o.employee_id=?"
			args = append(args, employeeID)
		}
	}
	for _, entry := range []struct{ key, op string }{{"from", ">="}, {"to", "<="}} {
		if value := r.URL.Query().Get(entry.key); value != "" {
			date, err := time.Parse("2006-01-02", value)
			if err != nil || date.Format("2006-01-02") != value {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal filter tidak valid.")
				return "", nil, false
			}
			where += " AND o.work_date" + entry.op + "?"
			args = append(args, value)
		}
	}
	return where, args, true
}

func (a adminAPI) overtimePage(w http.ResponseWriter, r *http.Request, from, where string, args []any) {
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+from+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan lembur.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+overtimeColumns+from+` WHERE `+where+` ORDER BY o.work_date DESC,o.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan lembur.")
		return
	}
	defer rows.Close()
	items := []overtimeView{}
	for rows.Next() {
		item, err := scanOvertime(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan lembur.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan lembur.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) listEssOvertime(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	filter, filterArgs, ok := overtimeFilters(w, r, false)
	if !ok {
		return
	}
	args := append([]any{c.state.SelectedTenantID, c.employeeID}, filterArgs...)
	a.overtimePage(w, r, overtimeEssFrom, "o.tenant_id=? AND o.employee_id=?"+filter, args)
}

func (a adminAPI) getEssOvertime(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan lembur tidak ditemukan.")
		return
	}
	item, err := scanOvertime(a.db.QueryRowContext(r.Context(), `SELECT `+overtimeColumns+overtimeEssFrom+` WHERE o.tenant_id=? AND o.employee_id=? AND o.id=?`, c.state.SelectedTenantID, c.employeeID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan lembur tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan lembur.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) listAdminOvertime(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "lembur.lihat")
	if !ok {
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	extra, extraArgs, ok := overtimeFilters(w, r, true)
	if !ok {
		return
	}
	args := append([]any{c.tenant.ID}, scopeArgs...)
	args = append(args, extraArgs...)
	a.overtimePage(w, r, overtimeAdminFrom, "o.tenant_id=? AND "+filter+extra, args)
}

func (a adminAPI) getAdminOvertime(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "lembur.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan lembur tidak ditemukan.")
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	args := append([]any{c.tenant.ID, id}, scopeArgs...)
	item, err := scanOvertime(a.db.QueryRowContext(r.Context(), `SELECT `+overtimeColumns+overtimeAdminFrom+` WHERE o.tenant_id=? AND o.id=? AND `+filter, args...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan lembur tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan lembur.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
