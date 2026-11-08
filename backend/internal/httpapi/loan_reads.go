package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type loanView struct {
	ID              string     `json:"id"`
	RequestID       *string    `json:"request_id"`
	EmployeeID      string     `json:"employee_id"`
	Amount          string     `json:"amount"`
	TenorMonths     int        `json:"tenor_months"`
	Installment     string     `json:"installment_amount"`
	Outstanding     string     `json:"outstanding"`
	Status          string     `json:"status"`
	DisbursedAt     *time.Time `json:"disbursed_at"`
	DisbursementRef *string    `json:"disbursement_ref"`
	ClosedAt        *time.Time `json:"closed_at"`
}

type loanInstallmentView struct {
	ID                string     `json:"id"`
	Seq               int        `json:"seq"`
	DuePeriod         string     `json:"due_period"`
	Amount            string     `json:"amount"`
	Status            string     `json:"status"`
	PayrollItemLineID *string    `json:"payroll_item_line_id"`
	WaivedAt          *time.Time `json:"waived_at"`
	WaiveReason       *string    `json:"waive_reason"`
	WaivedBy          *string    `json:"waived_by"`
}

type loanSettlementView struct {
	ID            string    `json:"id"`
	Amount        string    `json:"amount"`
	SettlementRef string    `json:"settlement_ref"`
	RecordedBy    string    `json:"recorded_by"`
	SettledAt     time.Time `json:"settled_at"`
}

const loanColumns = `l.id,l.request_id,l.employee_id,CAST(l.amount AS CHAR),l.tenor_months,CAST(l.installment_amount AS CHAR),CAST(l.outstanding AS CHAR),l.status,l.disbursed_at,l.disbursement_ref,l.closed_at`
const loanEssFrom = ` FROM employee_loans l`
const loanAdminFrom = loanEssFrom + ` JOIN employees e ON e.tenant_id=l.tenant_id AND e.id=l.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`

func scanLoan(row interface{ Scan(...any) error }) (loanView, error) {
	var item loanView
	var requestID, disbursementRef sql.NullString
	var disbursed, closed sql.NullTime
	err := row.Scan(&item.ID, &requestID, &item.EmployeeID, &item.Amount, &item.TenorMonths, &item.Installment, &item.Outstanding, &item.Status, &disbursed, &disbursementRef, &closed)
	if err != nil {
		return item, err
	}
	if requestID.Valid {
		item.RequestID = &requestID.String
	}
	if disbursed.Valid {
		item.DisbursedAt = &disbursed.Time
	}
	if disbursementRef.Valid {
		item.DisbursementRef = &disbursementRef.String
	}
	if closed.Valid {
		item.ClosedAt = &closed.Time
	}
	return item, nil
}

func (a adminAPI) loanPage(w http.ResponseWriter, r *http.Request, from, where string, args []any) {
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+from+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kasbon.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+loanColumns+from+` WHERE `+where+` ORDER BY l.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kasbon.")
		return
	}
	defer rows.Close()
	items := []loanView{}
	for rows.Next() {
		item, e := scanLoan(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kasbon.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kasbon.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func loanFilters(w http.ResponseWriter, r *http.Request, admin bool) (string, []any, bool) {
	where := ""
	args := []any{}
	if status := r.URL.Query().Get("status"); status != "" {
		if len(status) > 24 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status tidak valid.")
			return "", nil, false
		}
		where += " AND l.status=?"
		args = append(args, status)
	}
	if admin {
		if id := r.URL.Query().Get("employee_id"); id != "" {
			if !fileIDPattern.MatchString(id) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID karyawan tidak valid.")
				return "", nil, false
			}
			where += " AND l.employee_id=?"
			args = append(args, id)
		}
	}
	return where, args, true
}

func (a adminAPI) listEssLoans(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	filter, extra, ok := loanFilters(w, r, false)
	if !ok {
		return
	}
	a.loanPage(w, r, loanEssFrom, "l.tenant_id=? AND l.employee_id=?"+filter, append([]any{c.state.SelectedTenantID, c.employeeID}, extra...))
}

func (a adminAPI) listAdminLoans(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kasbon.lihat")
	if !ok {
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	filter, extra, ok := loanFilters(w, r, true)
	if !ok {
		return
	}
	args := append([]any{c.tenant.ID}, scopeArgs...)
	a.loanPage(w, r, loanAdminFrom, "l.tenant_id=? AND "+scope+filter, append(args, extra...))
}

func (a adminAPI) loanDetail(w http.ResponseWriter, r *http.Request, from, where string, args []any) {
	item, err := scanLoan(a.db.QueryRowContext(r.Context(), `SELECT `+loanColumns+from+` WHERE `+where, args...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kasbon tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kasbon.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,seq,due_period,CAST(amount AS CHAR),status,payroll_item_line_id,waived_at,waive_reason,waived_by FROM loan_installments WHERE tenant_id=? AND loan_id=? ORDER BY seq`, args[0], item.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca cicilan kasbon.")
		return
	}
	defer rows.Close()
	installments := []loanInstallmentView{}
	for rows.Next() {
		var line loanInstallmentView
		var due time.Time
		var payroll, reason, by sql.NullString
		var waived sql.NullTime
		err = rows.Scan(&line.ID, &line.Seq, &due, &line.Amount, &line.Status, &payroll, &waived, &reason, &by)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca cicilan kasbon.")
			return
		}
		line.DuePeriod = due.Format("2006-01-02")
		if payroll.Valid {
			line.PayrollItemLineID = &payroll.String
		}
		if waived.Valid {
			line.WaivedAt = &waived.Time
		}
		if reason.Valid {
			line.WaiveReason = &reason.String
		}
		if by.Valid {
			line.WaivedBy = &by.String
		}
		installments = append(installments, line)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca cicilan kasbon.")
		return
	}
	var settlement *loanSettlementView
	var record loanSettlementView
	err = a.db.QueryRowContext(r.Context(), `SELECT id,CAST(amount AS CHAR),settlement_ref,recorded_by,settled_at FROM loan_settlements WHERE tenant_id=? AND loan_id=?`, args[0], item.ID).Scan(&record.ID, &record.Amount, &record.SettlementRef, &record.RecordedBy, &record.SettledAt)
	if err == nil {
		settlement = &record
	} else if !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pelunasan kasbon.")
		return
	}
	writeAPIData(w, 200, map[string]any{"loan": item, "installments": installments, "settlement": settlement}, nil)
}

func (a adminAPI) getEssLoan(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kasbon tidak ditemukan.")
		return
	}
	a.loanDetail(w, r, loanEssFrom, "l.tenant_id=? AND l.employee_id=? AND l.id=?", []any{c.state.SelectedTenantID, c.employeeID, id})
}

func (a adminAPI) getAdminLoan(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kasbon.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kasbon tidak ditemukan.")
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	a.loanDetail(w, r, loanAdminFrom, "l.tenant_id=? AND l.id=? AND "+scope, append([]any{c.tenant.ID, id}, scopeArgs...))
}
