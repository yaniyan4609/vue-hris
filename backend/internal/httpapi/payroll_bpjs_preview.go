package httpapi

import (
	"database/sql"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"digaji.id/hris/internal/payrollcalc"
	"github.com/go-chi/chi/v5"
)

type bpjsPreviewRule struct {
	ID, Program, EmployeeRate, EmployerRate string
	WageCap                                 sql.NullString
}

type bpjsPreviewComponent struct {
	ID, Amount, From string
	To               sql.NullTime
	Active           bool
	Type, CalcType   string
}

type bpjsResolvedInputs struct {
	Inputs     []payrollcalc.BPJSInput
	InputWages map[string]int64
	Selected   map[string][]string
	RuleIDs    map[string]string
	Issues     []string
}

func resolveBPJSPreviewInputs(policy bpjsWageBasisView, components map[string][]bpjsPreviewComponent, rules map[string]bpjsPreviewRule, minimumCents int64, from, to string) bpjsResolvedInputs {
	resolved := bpjsResolvedInputs{Inputs: []payrollcalc.BPJSInput{}, InputWages: map[string]int64{}, Selected: map[string][]string{}, RuleIDs: map[string]string{}, Issues: []string{}}
	if policy.ID == "" {
		return resolved
	}
	selectedByProgram := map[string][]string{}
	for _, item := range policy.Programs {
		selectedByProgram[item.Program] = item.ComponentIDs
	}
	for _, program := range requiredPayrollBPJSPrograms {
		componentIDs := selectedByProgram[program]
		resolved.Selected[program] = componentIDs
		if len(componentIDs) == 0 {
			resolved.Issues = append(resolved.Issues, "BPJS_COMPONENT_INCOMPLETE")
			continue
		}
		var wage int64
		for _, id := range componentIDs {
			versions := components[id]
			if len(versions) != 1 || versions[0].From > from || (versions[0].To.Valid && versions[0].To.Time.Format("2006-01-02") < to) || !versions[0].Active || versions[0].Type != "earning" || versions[0].CalcType != "fixed" {
				resolved.Issues = append(resolved.Issues, "BPJS_COMPONENT_INCOMPLETE")
				continue
			}
			amount, valid := validPayrollMoney(versions[0].Amount)
			if !valid || amount > math.MaxInt64-wage {
				resolved.Issues = append(resolved.Issues, "BPJS_COMPONENT_INVALID")
				continue
			}
			wage += amount
		}
		resolved.InputWages[program] = wage
		rule, exists := rules[program]
		if !exists {
			continue
		}
		resolved.RuleIDs[program] = rule.ID
		employeeRate, validEmployeeRate := payrollRatePPM(rule.EmployeeRate)
		employerRate, validEmployerRate := payrollRatePPM(rule.EmployerRate)
		capCents := int64(0)
		validCap := true
		if rule.WageCap.Valid {
			capCents, validCap = validPayrollMoney(rule.WageCap.String)
			validCap = validCap && capCents > 0
		}
		if !validEmployeeRate || !validEmployerRate || !validCap {
			resolved.Issues = append(resolved.Issues, "BPJS_RULE_INVALID")
			continue
		}
		if program == "KESEHATAN" && wage < minimumCents {
			wage = minimumCents
		}
		resolved.Inputs = append(resolved.Inputs, payrollcalc.BPJSInput{Program: program, WageCents: wage, WageCapCents: capCents, EmployeeRatePPM: employeeRate, EmployerRatePPM: employerRate})
	}
	return resolved
}

func payrollRatePPM(raw string) (int64, bool) {
	if !validPayrollRate(raw) {
		return 0, false
	}
	parts := strings.SplitN(raw, ".", 2)
	whole, _ := strconv.ParseInt(parts[0], 10, 64)
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	fraction += strings.Repeat("0", 6-len(fraction))
	frac, _ := strconv.ParseInt(fraction, 10, 64)
	return whole*1_000_000 + frac, true
}

func (a adminAPI) previewPayrollBPJS(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	periodID, employeeID := chi.URLParam(r, "id"), r.URL.Query().Get("employee_id")
	if !fileIDPattern.MatchString(periodID) || !fileIDPattern.MatchString(employeeID) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID periode dan karyawan wajib valid.")
		return
	}
	scope, scopeArgs, ok := a.payEmployeeScope(w, r, c, employeeID, true)
	if !ok {
		return
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pratinjau BPJS.")
		return
	}
	defer tx.Rollback()
	var start, end time.Time
	err = tx.QueryRowContext(r.Context(), `SELECT start_date,end_date FROM payroll_periods WHERE tenant_id=? AND id=?`, c.tenant.ID, periodID).Scan(&start, &end)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca periode payroll.")
		return
	}
	from, to := start.Format("2006-01-02"), end.Format("2006-01-02")
	var joined, departed sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT e.join_date,e.end_date FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=? AND `+scope, append([]any{c.tenant.ID, employeeID}, scopeArgs...)...).Scan(&joined, &departed)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope payroll.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
		return
	}
	issues := []string{}
	if !joined.Valid || joined.Time.Format("2006-01-02") > from || (departed.Valid && departed.Time.Format("2006-01-02") < to) {
		issues = append(issues, "PARTIAL_EMPLOYMENT_PERIOD")
	}
	assignmentRows, err := tx.QueryContext(r.Context(), `SELECT office_id,valid_from,valid_to FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND DATE(valid_from)<=? AND (valid_to IS NULL OR DATE(valid_to)>=?) ORDER BY valid_from`, c.tenant.ID, employeeID, to, from)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca penugasan payroll.")
		return
	}
	var officeID string
	var assignmentCount int
	var assignmentFrom time.Time
	var assignmentTo sql.NullTime
	for assignmentRows.Next() {
		var candidateOffice string
		var candidateFrom time.Time
		var candidateTo sql.NullTime
		if err = assignmentRows.Scan(&candidateOffice, &candidateFrom, &candidateTo); err != nil {
			break
		}
		assignmentCount++
		officeID, assignmentFrom, assignmentTo = candidateOffice, candidateFrom, candidateTo
	}
	if err == nil {
		err = assignmentRows.Err()
	}
	assignmentRows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca penugasan payroll.")
		return
	}
	if assignmentCount != 1 || assignmentFrom.Format("2006-01-02") > from || (assignmentTo.Valid && assignmentTo.Time.Format("2006-01-02") < to) {
		issues = append(issues, "ASSIGNMENT_CHANGE_IN_PERIOD")
	}
	policy, err := scanBPJSWageBasis(tx.QueryRowContext(r.Context(), `SELECT id,effective_from,effective_to,data_json,source_reference FROM bpjs_wage_basis_policies WHERE tenant_id=? AND effective_from<=? AND (effective_to IS NULL OR effective_to>=?) ORDER BY effective_from DESC LIMIT 1`, c.tenant.ID, from, to))
	if errors.Is(err, sql.ErrNoRows) {
		issues = append(issues, "BPJS_WAGE_BASIS_INCOMPLETE")
	} else if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca dasar upah BPJS.")
		return
	}
	minimumCents := int64(0)
	if err == nil {
		for _, minimum := range policy.OfficeMinimums {
			if minimum.OfficeID == officeID {
				minimumCents, ok = validPayrollMoney(minimum.Amount.String())
				if !ok || minimumCents <= 0 {
					issues = append(issues, "BPJS_HEALTH_MINIMUM_INVALID")
				}
				break
			}
		}
		if minimumCents == 0 {
			issues = append(issues, "BPJS_HEALTH_MINIMUM_MISSING")
		}
	}
	ruleRows, err := tx.QueryContext(r.Context(), `SELECT id,program,CAST(rate_employee AS CHAR),CAST(rate_employer AS CHAR),CAST(wage_cap AS CHAR) FROM bpjs_rules WHERE tenant_id=? AND effective_from<=? AND (effective_to IS NULL OR effective_to>=?)`, c.tenant.ID, from, to)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tarif BPJS.")
		return
	}
	rules := map[string]bpjsPreviewRule{}
	duplicateRule := false
	for ruleRows.Next() {
		var item bpjsPreviewRule
		if err = ruleRows.Scan(&item.ID, &item.Program, &item.EmployeeRate, &item.EmployerRate, &item.WageCap); err != nil {
			break
		}
		if _, exists := rules[item.Program]; exists {
			duplicateRule = true
		}
		rules[item.Program] = item
	}
	if err == nil {
		err = ruleRows.Err()
	}
	ruleRows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tarif BPJS.")
		return
	}
	if duplicateRule || len(rules) < len(requiredPayrollBPJSPrograms) {
		issues = append(issues, "BPJS_RULE_INCOMPLETE")
	}
	componentRows, err := tx.QueryContext(r.Context(), `SELECT ep.component_id,CAST(ep.amount AS CHAR),DATE_FORMAT(ep.valid_from,'%Y-%m-%d'),ep.valid_to,pc.is_active,pc.component_type,pc.calc_type FROM employee_pay_components ep JOIN pay_components pc ON pc.tenant_id=ep.tenant_id AND pc.id=ep.component_id WHERE ep.tenant_id=? AND ep.employee_id=? AND ep.valid_from<=? AND (ep.valid_to IS NULL OR ep.valid_to>=?)`, c.tenant.ID, employeeID, to, from)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen gaji.")
		return
	}
	components := map[string][]bpjsPreviewComponent{}
	for componentRows.Next() {
		var item bpjsPreviewComponent
		if err = componentRows.Scan(&item.ID, &item.Amount, &item.From, &item.To, &item.Active, &item.Type, &item.CalcType); err != nil {
			break
		}
		components[item.ID] = append(components[item.ID], item)
	}
	if err == nil {
		err = componentRows.Err()
	}
	componentRows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca komponen gaji.")
		return
	}
	resolved := resolveBPJSPreviewInputs(policy, components, rules, minimumCents, from, to)
	issues = append(issues, resolved.Issues...)
	if len(issues) > 0 {
		writeAPIData(w, 200, map[string]any{"period_id": periodID, "employee_id": employeeID, "ready": false, "issues": uniquePayrollCodes(issues), "preview_only": true, "contributions": nil}, nil)
		return
	}
	result, err := payrollcalc.MonthlyBPJS(resolved.Inputs)
	if err != nil {
		writeAPIData(w, 200, map[string]any{"period_id": periodID, "employee_id": employeeID, "ready": false, "issues": []string{"BPJS_CALCULATION_ERROR"}, "preview_only": true, "contributions": nil}, nil)
		return
	}
	lines := []map[string]any{}
	for _, line := range result.Lines {
		lines = append(lines, map[string]any{"program": line.Program, "rule_id": resolved.RuleIDs[line.Program], "component_ids": resolved.Selected[line.Program], "component_wage": leaveDecimal(resolved.InputWages[line.Program]), "contribution_base": leaveDecimal(line.BaseCents), "employee_contribution": leaveDecimal(line.EmployeeCents), "employer_contribution": leaveDecimal(line.EmployerCents)})
	}
	writeAPIData(w, 200, map[string]any{"period_id": periodID, "employee_id": employeeID, "office_id": officeID, "policy_id": policy.ID, "ready": true, "issues": []string{}, "preview_only": true, "contributions": lines, "total_employee": leaveDecimal(result.TotalEmployeeCents), "total_employer": leaveDecimal(result.TotalEmployerCents)}, nil)
}

func uniquePayrollCodes(codes []string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(codes))
	for _, code := range codes {
		if !seen[code] {
			unique = append(unique, code)
			seen[code] = true
		}
	}
	return unique
}
