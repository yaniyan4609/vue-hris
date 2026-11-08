package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type loanInput struct {
	Amount      json.Number `json:"amount"`
	TenorMonths int         `json:"tenor_months"`
}

type loanCalculation struct {
	Amount         int64
	Regular        int64
	Last           int64
	Existing       int64
	MaxOutstanding int64
	PolicyVersion  uint64
	MaxTenorMonths int
}

func validateLoanInput(body loanInput) (int64, bool) {
	amount, ok := leaveHundredths(body.Amount.String())
	return amount, ok && amount > 0 && len(strings.SplitN(body.Amount.String(), ".", 2)[0]) <= 16 && body.TenorMonths >= 1 && body.TenorMonths <= 120 && amount >= int64(body.TenorMonths)
}

type loanQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func calculateLoan(ctx context.Context, query loanQuerier, tenantID, employeeID string, amount int64, tenor int) (loanCalculation, error) {
	var calculation loanCalculation
	var maxRaw string
	err := query.QueryRowContext(ctx, `SELECT CAST(max_outstanding AS CHAR),max_tenor_months,version FROM loan_policies WHERE tenant_id=?`, tenantID).Scan(&maxRaw, &calculation.MaxTenorMonths, &calculation.PolicyVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return calculation, leaveIssue{409, "POLICY_NOT_CONFIGURED", "Kebijakan kasbon belum diatur."}
	}
	if err != nil {
		return calculation, err
	}
	max, valid := leaveHundredths(maxRaw)
	if !valid {
		return calculation, errors.New("invalid loan policy amount")
	}
	calculation.MaxOutstanding = max
	var usedRaw string
	err = query.QueryRowContext(ctx, `SELECT COALESCE(CAST(SUM(CASE WHEN outstanding>0 THEN outstanding WHEN status IN ('submitted','in_review','approved') THEN amount ELSE 0 END) AS CHAR),'0.00') FROM employee_loans WHERE tenant_id=? AND employee_id=?`, tenantID, employeeID).Scan(&usedRaw)
	if err != nil {
		return calculation, err
	}
	used, valid := leaveHundredths(usedRaw)
	if !valid {
		return calculation, errors.New("invalid loan exposure")
	}
	if tenor > calculation.MaxTenorMonths || amount > max || used > max-amount {
		return calculation, leaveIssue{409, "LOAN_POLICY_EXCEEDED", "Plafon total atau tenor kasbon terlampaui."}
	}
	calculation.Amount = amount
	calculation.Existing = used
	calculation.Regular = amount / int64(tenor)
	calculation.Last = amount - calculation.Regular*int64(tenor-1)
	return calculation, nil
}

func writeLoanIssue(w http.ResponseWriter, r *http.Request, err error) {
	var issue leaveIssue
	if errors.As(err, &issue) {
		writeAPIError(w, r, issue.Status, issue.Code, issue.Message)
		return
	}
	writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung kasbon.")
}

func (a adminAPI) simulateEssLoan(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	var body loanInput
	if !readJSON(w, r, &body) {
		return
	}
	amount, valid := validateLoanInput(body)
	if !valid {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Nominal atau tenor kasbon tidak valid.")
		return
	}
	calc, err := calculateLoan(r.Context(), a.db, c.state.SelectedTenantID, c.employeeID, amount, body.TenorMonths)
	if err != nil {
		writeLoanIssue(w, r, err)
		return
	}
	writeAPIData(w, 200, map[string]any{"amount": leaveDecimal(amount), "tenor_months": body.TenorMonths, "installment_amount": leaveDecimal(calc.Regular), "last_installment_amount": leaveDecimal(calc.Last), "existing_exposure": leaveDecimal(calc.Existing), "remaining_after": leaveDecimal(calc.MaxOutstanding - calc.Existing - amount), "max_outstanding": leaveDecimal(calc.MaxOutstanding), "max_tenor_months": calc.MaxTenorMonths, "policy_version": calc.PolicyVersion, "interest_amount": "0.00"}, nil)
}
