package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type payrollLoanCandidate struct {
	InstallmentID string `json:"installment_id"`
	LoanID        string `json:"loan_id"`
	Seq           int    `json:"seq"`
	DuePeriod     string `json:"due_period"`
	Amount        string `json:"amount"`
	amountCents   int64
}

type payrollLoanDeferred struct {
	Installment payrollLoanCandidate `json:"installment"`
	Reason      string               `json:"reason"`
}

// A later installment of the same loan stays deferred when an earlier one
// cannot be covered. Other loans may still use the remaining available net.
func allocatePayrollLoanDeductions(candidates []payrollLoanCandidate, available int64) ([]payrollLoanCandidate, []payrollLoanDeferred, int64) {
	planned := []payrollLoanCandidate{}
	deferred := []payrollLoanDeferred{}
	blockedLoans := map[string]bool{}
	for _, item := range candidates {
		if blockedLoans[item.LoanID] {
			deferred = append(deferred, payrollLoanDeferred{Installment: item, Reason: "PRIOR_INSTALLMENT_DEFERRED"})
			continue
		}
		if item.amountCents > available {
			blockedLoans[item.LoanID] = true
			deferred = append(deferred, payrollLoanDeferred{Installment: item, Reason: "INSUFFICIENT_NET"})
			continue
		}
		available -= item.amountCents
		planned = append(planned, item)
	}
	return planned, deferred, available
}

func (a adminAPI) simulatePayrollLoanDeductions(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	sensitiveSubject, sensitiveDecision, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "penggajian.lihat_sensitif")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak payroll sensitif.")
		return
	}
	if !sensitiveDecision.Allowed {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak melihat data payroll sensitif diperlukan.")
		return
	}
	periodID := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(periodID) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
		return
	}
	var body struct {
		EmployeeID   string      `json:"employee_id"`
		AvailableNet json.Number `json:"available_net"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	available, valid := leaveHundredths(body.AvailableNet.String())
	if !fileIDPattern.MatchString(body.EmployeeID) || !valid || available < 0 || len(strings.SplitN(body.AvailableNet.String(), ".", 2)[0]) > 16 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Karyawan atau sisa gaji bersih tidak valid.")
		return
	}
	var end time.Time
	err = a.db.QueryRowContext(r.Context(), `SELECT end_date FROM payroll_periods WHERE tenant_id=? AND id=?`, c.tenant.ID, periodID).Scan(&end)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca periode payroll.")
		return
	}
	viewScope, viewArgs := access.EmployeeWhere(c.subject, c.effective)
	sensitiveScope, sensitiveArgs := access.EmployeeWhere(sensitiveSubject, sensitiveDecision)
	scope := `(` + viewScope + `) AND (` + sensitiveScope + `)`
	args := append([]any{c.tenant.ID, body.EmployeeID}, viewArgs...)
	args = append(args, sensitiveArgs...)
	var matched int
	err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=? AND `+scope, args...).Scan(&matched)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope karyawan.")
		return
	}
	if matched == 0 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope payroll.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT li.id,li.loan_id,li.seq,li.due_period,CAST(li.amount AS CHAR) FROM loan_installments li JOIN employee_loans l ON l.tenant_id=li.tenant_id AND l.id=li.loan_id WHERE li.tenant_id=? AND l.employee_id=? AND li.due_period<=? AND li.status='pending' AND li.payroll_item_line_id IS NULL AND l.status IN ('disbursed','repaying') AND l.outstanding>0 ORDER BY li.due_period,li.loan_id,li.seq LIMIT 1001`, c.tenant.ID, body.EmployeeID, end.Format("2006-01-02"))
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca cicilan kasbon.")
		return
	}
	defer rows.Close()
	candidates := []payrollLoanCandidate{}
	for rows.Next() {
		var item payrollLoanCandidate
		var due time.Time
		if err = rows.Scan(&item.InstallmentID, &item.LoanID, &item.Seq, &due, &item.Amount); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca cicilan kasbon.")
			return
		}
		item.DuePeriod = due.Format("2006-01-02")
		item.amountCents, valid = leaveHundredths(item.Amount)
		if !valid || item.amountCents <= 0 {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Nominal cicilan tidak valid.")
			return
		}
		candidates = append(candidates, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca cicilan kasbon.")
		return
	}
	if len(candidates) > 1000 {
		writeAPIError(w, r, 409, "TOO_MANY_INSTALLMENTS", "Cicilan jatuh tempo perlu ditinjau manual.")
		return
	}
	planned, deferred, remaining := allocatePayrollLoanDeductions(candidates, available)
	var plannedTotal int64
	for _, item := range planned {
		plannedTotal += item.amountCents
	}
	writeAPIData(w, 200, map[string]any{"employee_id": body.EmployeeID, "period_id": periodID, "available_net": leaveDecimal(available), "planned_total": leaveDecimal(plannedTotal), "remaining_net": leaveDecimal(remaining), "planned": planned, "deferred": deferred, "preview_only": true}, nil)
}
