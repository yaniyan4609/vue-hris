package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type payrollValidationIssue struct {
	Code          string `json:"code"`
	Severity      string `json:"severity"`
	AffectedCount int    `json:"affected_count,omitempty"`
}

type payrollValidationView struct {
	RunID             string                   `json:"run_id"`
	PeriodID          string                   `json:"period_id"`
	CalculationState  string                   `json:"calculation_state"`
	Ready             bool                     `json:"ready"`
	EligibleEmployees int                      `json:"eligible_employees"`
	Issues            []payrollValidationIssue `json:"issues"`
}

type payrollRuleRange struct{ From, To string }

func ruleRangeCovers(ranges []payrollRuleRange, from, to string) bool {
	start, _ := time.Parse("2006-01-02", from)
	end, _ := time.Parse("2006-01-02", to)
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		day := date.Format("2006-01-02")
		covered := false
		for _, item := range ranges {
			if coversDay(item.From, item.To, day) {
				covered = true
				break
			}
		}
		if !covered {
			return false
		}
	}
	return true
}

func (a adminAPI) payrollValidation(ctx adminContext, r *http.Request, runID string) (payrollValidationView, error) {
	result := payrollValidationView{RunID: runID, Issues: []payrollValidationIssue{}}
	var periodStart, periodEnd time.Time
	err := a.db.QueryRowContext(r.Context(), `SELECT pr.period_id,pr.calculation_state,pp.start_date,pp.end_date FROM payroll_runs pr JOIN payroll_periods pp ON pp.tenant_id=pr.tenant_id AND pp.id=pr.period_id WHERE pr.tenant_id=? AND pr.id=?`, ctx.tenant.ID, runID).Scan(&result.PeriodID, &result.CalculationState, &periodStart, &periodEnd)
	if err != nil {
		return result, err
	}
	from, to := periodStart.Format("2006-01-02"), periodEnd.Format("2006-01-02")
	add := func(code string, count int) {
		result.Issues = append(result.Issues, payrollValidationIssue{Code: code, Severity: "blocker", AffectedCount: count})
	}
	add("TAX_ENGINE_UNVERIFIED", 0)
	var settingRaw []byte
	err = a.db.QueryRowContext(r.Context(), `SELECT value_json FROM tenant_settings WHERE tenant_id=? AND setting_key=?`, ctx.tenant.ID, payrollSettingsKey).Scan(&settingRaw)
	if errors.Is(err, sql.ErrNoRows) {
		add("PAYROLL_SETTINGS_MISSING", 0)
	} else if err != nil {
		return result, err
	} else {
		var settings payrollSettings
		if err = json.Unmarshal(settingRaw, &settings); err != nil {
			return result, err
		}
		if settings.PayDay == nil {
			add("PAY_DAY_MISSING", 0)
		}
	}
	for _, rule := range []struct{ table, key, value, missing string }{{"tax_rules", "rule_set", pph21RuleSet, "TAX_RULE_INCOMPLETE"}, {"bpjs_rules", "program", "KESEHATAN", "BPJS_KESEHATAN_INCOMPLETE"}, {"bpjs_rules", "program", "JHT", "BPJS_JHT_INCOMPLETE"}, {"bpjs_rules", "program", "JKK", "BPJS_JKK_INCOMPLETE"}, {"bpjs_rules", "program", "JKM", "BPJS_JKM_INCOMPLETE"}, {"bpjs_rules", "program", "JP", "BPJS_JP_INCOMPLETE"}} {
		rows, e := a.db.QueryContext(r.Context(), `SELECT DATE_FORMAT(effective_from,'%Y-%m-%d'),IFNULL(DATE_FORMAT(effective_to,'%Y-%m-%d'),'') FROM `+rule.table+` WHERE tenant_id=? AND `+rule.key+`=? AND effective_from<=? AND (effective_to IS NULL OR effective_to>=?)`, ctx.tenant.ID, rule.value, to, from)
		if e != nil {
			return result, e
		}
		ranges := []payrollRuleRange{}
		for rows.Next() {
			var item payrollRuleRange
			if e = rows.Scan(&item.From, &item.To); e != nil {
				break
			}
			ranges = append(ranges, item)
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return result, e
		}
		if !ruleRangeCovers(ranges, from, to) {
			add(rule.missing, 0)
		}
	}
	scope, scopeArgs := access.EmployeeWhere(ctx.subject, ctx.effective)
	fromQuery := ` FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND (e.join_date IS NULL OR e.join_date<=?) AND (e.end_date IS NULL OR e.end_date>=?) AND ` + scope
	args := append([]any{ctx.tenant.ID, to, from}, scopeArgs...)
	var missingJoin, missingBank, missingPay, missingTax int
	query := `SELECT COUNT(*),COALESCE(SUM(e.join_date IS NULL),0),COALESCE(SUM(NOT EXISTS(SELECT 1 FROM employee_bank_accounts b WHERE b.tenant_id=e.tenant_id AND b.employee_id=e.id AND b.is_primary=TRUE AND b.valid_from<=? AND (b.valid_to IS NULL OR b.valid_to>=?))),0),COALESCE(SUM(NOT EXISTS(SELECT 1 FROM employee_pay_components ep JOIN pay_components pc ON pc.tenant_id=ep.tenant_id AND pc.id=ep.component_id WHERE ep.tenant_id=e.tenant_id AND ep.employee_id=e.id AND pc.component_type='earning' AND pc.is_active=TRUE AND ep.valid_from<=? AND (ep.valid_to IS NULL OR ep.valid_to>=?))),0),COALESCE(SUM(NOT EXISTS(SELECT 1 FROM employee_tax_bpjs p WHERE p.tenant_id=e.tenant_id AND p.employee_id=e.id AND p.valid_from<=? AND (p.valid_to IS NULL OR p.valid_to>=?) AND p.ptkp_status IS NOT NULL AND p.bpjs_kes_no_enc IS NOT NULL AND p.bpjs_tk_no_enc IS NOT NULL)),0)` + fromQuery
	queryArgs := append([]any{to, from, to, from, to, from}, args...)
	if err = a.db.QueryRowContext(r.Context(), query, queryArgs...).Scan(&result.EligibleEmployees, &missingJoin, &missingBank, &missingPay, &missingTax); err != nil {
		return result, err
	}
	if result.EligibleEmployees == 0 {
		add("NO_ELIGIBLE_EMPLOYEES", 0)
	}
	if missingJoin > 0 {
		add("JOIN_DATE_MISSING", missingJoin)
	}
	if missingBank > 0 {
		add("PRIMARY_BANK_ACCOUNT_MISSING", missingBank)
	}
	if missingPay > 0 {
		add("EARNING_COMPONENT_MISSING", missingPay)
	}
	if missingTax > 0 {
		add("TAX_BPJS_PROFILE_MISSING", missingTax)
	}
	calendarDays := int(periodEnd.Sub(periodStart).Hours()/24) + 1
	if calendarDays < 1 || calendarDays > 31 {
		add("PERIOD_DATES_INVALID", 0)
	} else {
		rosterQuery := `SELECT COUNT(*) FROM (SELECT e.id,COUNT(DISTINCT ws.work_date) AS published_days FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE LEFT JOIN work_schedules ws ON ws.tenant_id=e.tenant_id AND ws.employee_id=e.id AND ws.status='published' AND ws.work_date BETWEEN ? AND ? WHERE e.tenant_id=? AND (e.join_date IS NULL OR e.join_date<=?) AND (e.end_date IS NULL OR e.end_date>=?) AND ` + scope + ` GROUP BY e.id HAVING published_days<?) missing`
		rosterArgs := append([]any{from, to, ctx.tenant.ID, to, from}, scopeArgs...)
		rosterArgs = append(rosterArgs, calendarDays)
		var missingRoster int
		if err = a.db.QueryRowContext(r.Context(), rosterQuery, rosterArgs...).Scan(&missingRoster); err != nil {
			return result, err
		}
		if missingRoster > 0 {
			add("PUBLISHED_ROSTER_INCOMPLETE", missingRoster)
		}
	}
	result.Ready = len(result.Issues) == 0
	return result, nil
}

func (a adminAPI) getPayrollRunValidation(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Run payroll tidak ditemukan.")
		return
	}
	result, err := a.payrollValidation(c, r, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Run payroll tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memvalidasi run payroll.")
		return
	}
	writeAPIData(w, 200, result, nil)
}
