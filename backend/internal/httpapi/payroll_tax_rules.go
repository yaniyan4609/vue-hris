package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"digaji.id/hris/internal/ids"
)

const pph21RuleSet = "PPH21_TER_V1"

var taxPTKPStatusPattern = regexp.MustCompile(`^[A-Z0-9/+-]{1,16}$`)

type payrollTaxPTKP struct {
	Status       string      `json:"status"`
	TERCategory  string      `json:"ter_category"`
	AnnualAmount json.Number `json:"annual_amount"`
}

type payrollTaxBracket struct {
	Category            string       `json:"category,omitempty"`
	LowerBound          json.Number  `json:"lower_bound"`
	UpperBoundExclusive *json.Number `json:"upper_bound_exclusive"`
	Rate                json.Number  `json:"rate"`
}

type payrollTaxRuleInput struct {
	EffectiveFrom   string              `json:"effective_from"`
	EffectiveTo     *string             `json:"effective_to"`
	SourceReference string              `json:"source_reference"`
	PTKP            []payrollTaxPTKP    `json:"ptkp"`
	TERBrackets     []payrollTaxBracket `json:"ter_brackets"`
	AnnualBrackets  []payrollTaxBracket `json:"annual_brackets"`
}

func validPayrollMoney(raw string) (int64, bool) {
	value, ok := leaveHundredths(raw)
	return value, ok && value >= 0 && len(strings.SplitN(raw, ".", 2)[0]) <= 16
}

func validateTaxBrackets(brackets []payrollTaxBracket, categorized bool) bool {
	if len(brackets) == 0 || len(brackets) > 400 {
		return false
	}
	groups := map[string][]payrollTaxBracket{}
	for _, bracket := range brackets {
		category := "ANNUAL"
		if categorized {
			category = bracket.Category
			if category != "A" && category != "B" && category != "C" {
				return false
			}
		} else if bracket.Category != "" {
			return false
		}
		lower, ok := validPayrollMoney(bracket.LowerBound.String())
		if !ok || !validPayrollRate(bracket.Rate.String()) {
			return false
		}
		if bracket.UpperBoundExclusive != nil {
			upper, valid := validPayrollMoney(bracket.UpperBoundExclusive.String())
			if !valid || upper <= lower {
				return false
			}
		}
		groups[category] = append(groups[category], bracket)
	}
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			left, _ := validPayrollMoney(group[i].LowerBound.String())
			right, _ := validPayrollMoney(group[j].LowerBound.String())
			return left < right
		})
		var expected int64
		for i, bracket := range group {
			lower, _ := validPayrollMoney(bracket.LowerBound.String())
			if lower != expected || (i == len(group)-1) != (bracket.UpperBoundExclusive == nil) {
				return false
			}
			if bracket.UpperBoundExclusive != nil {
				expected, _ = validPayrollMoney(bracket.UpperBoundExclusive.String())
			}
		}
	}
	return true
}

func validatePayrollTaxRule(in *payrollTaxRuleInput) bool {
	in.SourceReference = strings.TrimSpace(in.SourceReference)
	if !validRuleDates(in.EffectiveFrom, in.EffectiveTo) || in.SourceReference == "" || len([]rune(in.SourceReference)) > 500 || len(in.PTKP) == 0 || len(in.PTKP) > 30 {
		return false
	}
	if !validateTaxBrackets(in.TERBrackets, true) || !validateTaxBrackets(in.AnnualBrackets, false) {
		return false
	}
	categories := map[string]bool{}
	for _, bracket := range in.TERBrackets {
		categories[bracket.Category] = true
	}
	if !categories["A"] || !categories["B"] || !categories["C"] {
		return false
	}
	statuses := map[string]bool{}
	for i := range in.PTKP {
		entry := &in.PTKP[i]
		entry.Status = strings.ToUpper(strings.TrimSpace(entry.Status))
		if !taxPTKPStatusPattern.MatchString(entry.Status) || statuses[entry.Status] || !categories[entry.TERCategory] {
			return false
		}
		amount, valid := validPayrollMoney(entry.AnnualAmount.String())
		if !valid || amount < 0 {
			return false
		}
		statuses[entry.Status] = true
	}
	return true
}

func (a adminAPI) getPayrollTaxRules(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	date, ok := payrollRuleDate(w, r, c.tenant.Timezone)
	if !ok {
		return
	}
	var id, source, from string
	var to sql.NullTime
	var data json.RawMessage
	err := a.db.QueryRowContext(r.Context(), `SELECT id,DATE_FORMAT(effective_from,'%Y-%m-%d'),effective_to,data_json FROM tax_rules WHERE tenant_id=? AND rule_set=? AND effective_from<=? AND (effective_to IS NULL OR effective_to>=?) ORDER BY effective_from DESC LIMIT 1`, c.tenant.ID, pph21RuleSet, date, date).Scan(&id, &from, &to, &data)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIData(w, 200, map[string]any{"configured": false, "rule_set": pph21RuleSet, "date": date}, nil)
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aturan PPh 21.")
		return
	}
	var parsed struct {
		SourceReference string `json:"source_reference"`
	}
	_ = json.Unmarshal(data, &parsed)
	source = parsed.SourceReference
	var until *string
	if to.Valid {
		value := to.Time.Format("2006-01-02")
		until = &value
	}
	writeAPIData(w, 200, map[string]any{"configured": true, "id": id, "rule_set": pph21RuleSet, "effective_from": from, "effective_to": until, "source_reference": source, "data": data}, nil)
}

func (a adminAPI) publishPayrollTaxRule(w http.ResponseWriter, r *http.Request) {
	c, ok := a.payrollRuleWriter(w, r)
	if !ok {
		return
	}
	var body payrollTaxRuleInput
	if !readJSON(w, r, &body) {
		return
	}
	if !validatePayrollTaxRule(&body) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Aturan PPh 21 tidak lengkap, rentang tarif bertumpang tindih, atau nilai tidak valid.")
		return
	}
	payload, err := json.Marshal(body)
	if err != nil || len(payload) > 256*1024 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Aturan PPh 21 terlalu besar.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menerbitkan aturan PPh 21.")
		return
	}
	defer tx.Rollback()
	if !lockPayrollRuleTenant(w, r, tx, c.tenant.ID, body.EffectiveFrom) {
		return
	}
	priorID, ok := nextPayrollRuleVersion(w, r, tx, "tax_rules", "rule_set", c.tenant.ID, pph21RuleSet, body.EffectiveFrom)
	if !ok {
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID aturan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO tax_rules (tenant_id,id,rule_set,effective_from,effective_to,data_json) VALUES (?,?,?,?,?,?)`, c.tenant.ID, id, pph21RuleSet, body.EffectiveFrom, body.EffectiveTo, string(payload)); err != nil {
		writeAPIError(w, r, 409, "RULE_VERSION_CONFLICT", "Aturan PPh 21 sudah berubah.")
		return
	}
	digest := sha256.Sum256(payload)
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "publish", "tax_rules", id, map[string]any{"previous_id": priorID}, map[string]any{"rule_set": pph21RuleSet, "effective_from": body.EffectiveFrom, "effective_to": body.EffectiveTo, "source_reference": body.SourceReference, "sha256": hex.EncodeToString(digest[:])}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat aturan PPh 21.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menerbitkan aturan PPh 21.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "rule_set": pph21RuleSet, "effective_from": body.EffectiveFrom, "effective_to": body.EffectiveTo, "source_reference": body.SourceReference, "sha256": hex.EncodeToString(digest[:])}, nil)
}
