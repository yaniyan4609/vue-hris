package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
)

type payrollBPJSRuleView struct {
	ID              string  `json:"id"`
	Program         string  `json:"program"`
	EffectiveFrom   string  `json:"effective_from"`
	EffectiveTo     *string `json:"effective_to"`
	RateEmployee    string  `json:"rate_employee"`
	RateEmployer    string  `json:"rate_employer"`
	WageCap         *string `json:"wage_cap"`
	SourceReference string  `json:"source_reference"`
}

func scanPayrollBPJSRule(row interface{ Scan(...any) error }) (payrollBPJSRuleView, error) {
	var item payrollBPJSRuleView
	var from time.Time
	var to sql.NullTime
	var cap, source sql.NullString
	err := row.Scan(&item.ID, &item.Program, &from, &to, &item.RateEmployee, &item.RateEmployer, &cap, &source)
	if err != nil {
		return item, err
	}
	item.EffectiveFrom = from.Format("2006-01-02")
	if to.Valid {
		value := to.Time.Format("2006-01-02")
		item.EffectiveTo = &value
	}
	if cap.Valid {
		item.WageCap = &cap.String
	}
	item.SourceReference = source.String
	return item, nil
}

func (a adminAPI) getPayrollBPJSRules(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	date, ok := payrollRuleDate(w, r, c.tenant.Timezone)
	if !ok {
		return
	}
	where := `tenant_id=? AND effective_from<=? AND (effective_to IS NULL OR effective_to>=?)`
	args := []any{c.tenant.ID, date, date}
	if program := r.URL.Query().Get("program"); program != "" {
		program = strings.ToUpper(strings.TrimSpace(program))
		if !payrollRuleCodePattern.MatchString(program) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kode program BPJS tidak valid.")
			return
		}
		where += ` AND program=?`
		args = append(args, program)
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,program,effective_from,effective_to,CAST(rate_employee AS CHAR),CAST(rate_employer AS CHAR),CAST(wage_cap AS CHAR),source_reference FROM bpjs_rules WHERE `+where+` ORDER BY program`, args...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aturan BPJS.")
		return
	}
	defer rows.Close()
	items := []payrollBPJSRuleView{}
	for rows.Next() {
		item, e := scanPayrollBPJSRule(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aturan BPJS.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aturan BPJS.")
		return
	}
	writeAPIData(w, 200, map[string]any{"date": date, "rules": items}, nil)
}

func (a adminAPI) publishPayrollBPJSRule(w http.ResponseWriter, r *http.Request) {
	c, ok := a.payrollRuleWriter(w, r)
	if !ok {
		return
	}
	var body struct {
		Program         string          `json:"program"`
		EffectiveFrom   string          `json:"effective_from"`
		EffectiveTo     *string         `json:"effective_to"`
		RateEmployee    json.Number     `json:"rate_employee"`
		RateEmployer    json.Number     `json:"rate_employer"`
		WageCap         json.RawMessage `json:"wage_cap"`
		SourceReference string          `json:"source_reference"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Program = strings.ToUpper(strings.TrimSpace(body.Program))
	body.SourceReference = strings.TrimSpace(body.SourceReference)
	if !payrollRuleCodePattern.MatchString(body.Program) || !validRuleDates(body.EffectiveFrom, body.EffectiveTo) || !validPayrollRate(body.RateEmployee.String()) || !validPayrollRate(body.RateEmployer.String()) || body.SourceReference == "" || len([]rune(body.SourceReference)) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Program, tanggal, tarif, atau sumber aturan BPJS tidak valid.")
		return
	}
	var wageCap *string
	if len(body.WageCap) > 0 && string(body.WageCap) != "null" {
		var n json.Number
		if err := json.Unmarshal(body.WageCap, &n); err != nil {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Plafon upah BPJS tidak valid.")
			return
		}
		cents, valid := validPayrollMoney(n.String())
		if !valid || cents <= 0 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Plafon upah BPJS tidak valid.")
			return
		}
		formatted := leaveDecimal(cents)
		wageCap = &formatted
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menerbitkan aturan BPJS.")
		return
	}
	defer tx.Rollback()
	if !lockPayrollRuleTenant(w, r, tx, c.tenant.ID, body.EffectiveFrom) {
		return
	}
	priorID, ok := nextPayrollRuleVersion(w, r, tx, "bpjs_rules", "program", c.tenant.ID, body.Program, body.EffectiveFrom)
	if !ok {
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID aturan BPJS.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO bpjs_rules (tenant_id,id,program,effective_from,effective_to,rate_employee,rate_employer,wage_cap,source_reference) VALUES (?,?,?,?,?,?,?,?,?)`, c.tenant.ID, id, body.Program, body.EffectiveFrom, body.EffectiveTo, body.RateEmployee.String(), body.RateEmployer.String(), wageCap, body.SourceReference); err != nil {
		writeAPIError(w, r, 409, "RULE_VERSION_CONFLICT", "Aturan BPJS sudah berubah.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "publish", "bpjs_rules", id, map[string]any{"previous_id": priorID}, map[string]any{"program": body.Program, "effective_from": body.EffectiveFrom, "effective_to": body.EffectiveTo, "rate_employee": body.RateEmployee.String(), "rate_employer": body.RateEmployer.String(), "wage_cap": wageCap, "source_reference": body.SourceReference}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat aturan BPJS.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menerbitkan aturan BPJS.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "program": body.Program, "effective_from": body.EffectiveFrom, "effective_to": body.EffectiveTo, "rate_employee": body.RateEmployee.String(), "rate_employer": body.RateEmployer.String(), "wage_cap": wageCap, "source_reference": body.SourceReference}, nil)
}
