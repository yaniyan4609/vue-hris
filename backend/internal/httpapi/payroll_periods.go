package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
	"github.com/go-sql-driver/mysql"
)

type payrollPeriodView struct {
	ID     string `json:"id"`
	Year   int    `json:"year"`
	Month  int    `json:"month"`
	Start  string `json:"start_date"`
	End    string `json:"end_date"`
	Status string `json:"status"`
}

func scanPayrollPeriod(row interface{ Scan(...any) error }) (payrollPeriodView, error) {
	var p payrollPeriodView
	var start, end time.Time
	err := row.Scan(&p.ID, &p.Year, &p.Month, &start, &end, &p.Status)
	if err == nil {
		p.Start, p.End = start.Format("2006-01-02"), end.Format("2006-01-02")
	}
	return p, err
}

const payrollPeriodColumns = `id,period_year,period_month,start_date,end_date,status`

func (a adminAPI) listPayrollPeriods(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payroll_periods WHERE tenant_id=?`, c.tenant.ID).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca periode payroll.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+payrollPeriodColumns+` FROM payroll_periods WHERE tenant_id=? ORDER BY period_year DESC,period_month DESC LIMIT ? OFFSET ?`, c.tenant.ID, page.PageSize, page.Offset)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca periode payroll.")
		return
	}
	defer rows.Close()
	items := []payrollPeriodView{}
	for rows.Next() {
		item, e := scanPayrollPeriod(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca periode payroll.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca periode payroll.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) getPayrollPeriod(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
		return
	}
	item, err := scanPayrollPeriod(a.db.QueryRowContext(r.Context(), `SELECT `+payrollPeriodColumns+` FROM payroll_periods WHERE tenant_id=? AND id=?`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca periode payroll.")
		return
	}
	var runs int
	if err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payroll_runs WHERE tenant_id=? AND period_id=?`, c.tenant.ID, id).Scan(&runs); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca jumlah run payroll.")
		return
	}
	writeAPIData(w, 200, map[string]any{"period": item, "run_count": runs}, nil)
}

func (a adminAPI) createPayrollPeriod(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.proses")
	if !ok {
		return
	}
	var body struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Year < 2000 || body.Year > 2100 || body.Month < 1 || body.Month > 12 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tahun atau bulan payroll tidak valid.")
		return
	}
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	start := time.Date(body.Year, time.Month(body.Month), 1, 0, 0, 0, 0, zone)
	end := start.AddDate(0, 1, -1)
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID periode.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka periode payroll.")
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), `INSERT INTO payroll_periods (tenant_id,id,period_year,period_month,start_date,end_date,status) VALUES (?,?,?,?,?,?,'open')`, c.tenant.ID, id, body.Year, body.Month, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		var dbError *mysql.MySQLError
		if errors.As(err, &dbError) && dbError.Number == 1062 {
			writeAPIError(w, r, 409, "PERIOD_EXISTS", "Periode payroll bulan ini sudah ada.")
		} else {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka periode payroll.")
		}
		return
	}
	item := payrollPeriodView{ID: id, Year: body.Year, Month: body.Month, Start: start.Format("2006-01-02"), End: end.Format("2006-01-02"), Status: "open"}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "payroll_periods", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat periode payroll.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuka periode payroll.")
		return
	}
	writeAPIData(w, 201, item, nil)
}

type payrollLoanDeductionView struct {
	InstallmentID string `json:"installment_id"`
	LoanID        string `json:"loan_id"`
	EmployeeID    string `json:"employee_id"`
	Seq           int    `json:"seq"`
	DuePeriod     string `json:"due_period"`
	Amount        string `json:"amount"`
	Outstanding   string `json:"loan_outstanding"`
}

func (a adminAPI) previewPayrollLoanDeductions(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
		return
	}
	var end time.Time
	err := a.db.QueryRowContext(r.Context(), `SELECT end_date FROM payroll_periods WHERE tenant_id=? AND id=?`, c.tenant.ID, id).Scan(&end)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca periode payroll.")
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	from := ` FROM loan_installments li JOIN employee_loans l ON l.tenant_id=li.tenant_id AND l.id=li.loan_id JOIN employees e ON e.tenant_id=l.tenant_id AND e.id=l.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`
	where := ` WHERE li.tenant_id=? AND li.due_period<=? AND li.status='pending' AND li.payroll_item_line_id IS NULL AND l.status IN ('disbursed','repaying') AND l.outstanding>0 AND ` + scope
	args := append([]any{c.tenant.ID, end.Format("2006-01-02")}, scopeArgs...)
	if loanID := r.URL.Query().Get("loan_id"); loanID != "" {
		if !fileIDPattern.MatchString(loanID) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID kasbon tidak valid.")
			return
		}
		where += ` AND l.id=?`
		args = append(args, loanID)
	}
	var total int
	var totalRaw string
	if err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*),CAST(COALESCE(SUM(li.amount),0) AS CHAR)`+from+where, args...).Scan(&total, &totalRaw); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung cicilan payroll.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT li.id,li.loan_id,l.employee_id,li.seq,li.due_period,CAST(li.amount AS CHAR),CAST(l.outstanding AS CHAR)`+from+where+` ORDER BY li.due_period,li.id LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca cicilan payroll.")
		return
	}
	defer rows.Close()
	items := []payrollLoanDeductionView{}
	for rows.Next() {
		var item payrollLoanDeductionView
		var due time.Time
		if err = rows.Scan(&item.InstallmentID, &item.LoanID, &item.EmployeeID, &item.Seq, &due, &item.Amount, &item.Outstanding); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca cicilan payroll.")
			return
		}
		item.DuePeriod = due.Format("2006-01-02")
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca cicilan payroll.")
		return
	}
	writeAPIData(w, 200, items, map[string]any{"page": page.Page, "per_page": page.PageSize, "total": total, "total_amount": totalRaw, "period_id": id, "as_of": end.Format("2006-01-02"), "preview_only": true})
}
