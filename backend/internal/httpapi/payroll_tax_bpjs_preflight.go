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

type payrollDatedStatus struct {
	From, To string
	PTKP     string
	NPWP     bool
	BPJSKes  bool
	BPJSTK   bool
}

type payrollTaxRuleStatus struct {
	From, To string
	PTKP     map[string]bool
}

func coversDay(from, to, day string) bool {
	return from <= day && (to == "" || to >= day)
}

func (a adminAPI) previewPayrollTaxBPJSPreflight(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	periodID, employeeID := chi.URLParam(r, "id"), r.URL.Query().Get("employee_id")
	if !fileIDPattern.MatchString(periodID) || !fileIDPattern.MatchString(employeeID) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID periode dan karyawan wajib valid.")
		return
	}
	period, err := scanPayrollPeriod(a.db.QueryRowContext(r.Context(), `SELECT `+payrollPeriodColumns+` FROM payroll_periods WHERE tenant_id=? AND id=?`, c.tenant.ID, periodID))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca periode payroll.")
		return
	}
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	var matched int
	if err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=? AND `+scope, append([]any{c.tenant.ID, employeeID}, args...)...).Scan(&matched); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return
	}
	if matched == 0 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope payroll.")
		return
	}
	profiles := []payrollDatedStatus{}
	rows, err := a.db.QueryContext(r.Context(), `SELECT DATE_FORMAT(valid_from,'%Y-%m-%d'),IFNULL(DATE_FORMAT(valid_to,'%Y-%m-%d'),''),IFNULL(ptkp_status,''),npwp_enc IS NOT NULL,bpjs_kes_no_enc IS NOT NULL,bpjs_tk_no_enc IS NOT NULL FROM employee_tax_bpjs WHERE tenant_id=? AND employee_id=? AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) ORDER BY valid_from`, c.tenant.ID, employeeID, period.End, period.Start)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kesiapan profil pajak.")
		return
	}
	for rows.Next() {
		var item payrollDatedStatus
		if err = rows.Scan(&item.From, &item.To, &item.PTKP, &item.NPWP, &item.BPJSKes, &item.BPJSTK); err != nil {
			break
		}
		profiles = append(profiles, item)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kesiapan profil pajak.")
		return
	}
	rules := []payrollTaxRuleStatus{}
	rows, err = a.db.QueryContext(r.Context(), `SELECT DATE_FORMAT(effective_from,'%Y-%m-%d'),IFNULL(DATE_FORMAT(effective_to,'%Y-%m-%d'),''),data_json FROM tax_rules WHERE tenant_id=? AND rule_set=? AND effective_from<=? AND (effective_to IS NULL OR effective_to>=?) ORDER BY effective_from`, c.tenant.ID, pph21RuleSet, period.End, period.Start)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kesiapan aturan PPh 21.")
		return
	}
	for rows.Next() {
		var item payrollTaxRuleStatus
		var data []byte
		if err = rows.Scan(&item.From, &item.To, &data); err != nil {
			break
		}
		var payload struct {
			PTKP []struct {
				Status string `json:"status"`
			} `json:"ptkp"`
		}
		if err = json.Unmarshal(data, &payload); err != nil {
			break
		}
		item.PTKP = map[string]bool{}
		for _, status := range payload.PTKP {
			item.PTKP[status.Status] = true
		}
		rules = append(rules, item)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Aturan PPh 21 tidak dapat dibaca.")
		return
	}
	bpjs := []payrollDatedStatus{}
	rows, err = a.db.QueryContext(r.Context(), `SELECT DATE_FORMAT(effective_from,'%Y-%m-%d'),IFNULL(DATE_FORMAT(effective_to,'%Y-%m-%d'),'') FROM bpjs_rules WHERE tenant_id=? AND effective_from<=? AND (effective_to IS NULL OR effective_to>=?)`, c.tenant.ID, period.End, period.Start)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kesiapan aturan BPJS.")
		return
	}
	for rows.Next() {
		var item payrollDatedStatus
		if err = rows.Scan(&item.From, &item.To); err != nil {
			break
		}
		bpjs = append(bpjs, item)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kesiapan aturan BPJS.")
		return
	}
	start, _ := time.Parse("2006-01-02", period.Start)
	end, _ := time.Parse("2006-01-02", period.End)
	checks := map[string]bool{"profile_covered": true, "ptkp_in_tax_rule": true, "tax_rule_covered": true, "bpjs_rule_present": true, "npwp_recorded": true, "bpjs_kes_recorded": true, "bpjs_tk_recorded": true}
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		var profile *payrollDatedStatus
		for i := range profiles {
			if coversDay(profiles[i].From, profiles[i].To, date) {
				profile = &profiles[i]
			}
		}
		if profile == nil || profile.PTKP == "" {
			checks["profile_covered"] = false
		}
		if profile == nil || !profile.NPWP {
			checks["npwp_recorded"] = false
		}
		if profile == nil || !profile.BPJSKes {
			checks["bpjs_kes_recorded"] = false
		}
		if profile == nil || !profile.BPJSTK {
			checks["bpjs_tk_recorded"] = false
		}
		var rule *payrollTaxRuleStatus
		for i := range rules {
			if coversDay(rules[i].From, rules[i].To, date) {
				rule = &rules[i]
			}
		}
		if rule == nil {
			checks["tax_rule_covered"] = false
		}
		if rule == nil || profile == nil || !rule.PTKP[profile.PTKP] {
			checks["ptkp_in_tax_rule"] = false
		}
		foundBPJS := false
		for _, item := range bpjs {
			if coversDay(item.From, item.To, date) {
				foundBPJS = true
				break
			}
		}
		if !foundBPJS {
			checks["bpjs_rule_present"] = false
		}
	}
	writeAPIData(w, 200, map[string]any{"employee_id": employeeID, "period_id": periodID, "start_date": period.Start, "end_date": period.End, "checks": checks, "note": "Pemeriksaan kelengkapan data dan keberlakuan aturan saja; belum menghitung atau memfinalisasi payroll. Program BPJS yang wajib masih perlu ditentukan per tenant."}, nil)
}
