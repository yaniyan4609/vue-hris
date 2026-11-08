package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var payrollRatePattern = regexp.MustCompile(`^(0(?:\.[0-9]{1,6})?|1(?:\.0{1,6})?)$`)
var payrollRuleCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,47}$`)

func validPayrollRate(raw string) bool {
	return payrollRatePattern.MatchString(raw)
}

func payrollRuleDate(w http.ResponseWriter, r *http.Request, zoneName string) (string, bool) {
	date := r.URL.Query().Get("date")
	if date == "" {
		zone, err := time.LoadLocation(zoneName)
		if err != nil {
			zone = time.UTC
		}
		date = time.Now().In(zone).Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal aturan tidak valid.")
		return "", false
	}
	return date, true
}

func validRuleDates(from string, to *string) bool {
	start, err := time.Parse("2006-01-02", from)
	if err != nil || start.Year() < 2000 || start.Year() > 2100 {
		return false
	}
	if to == nil {
		return true
	}
	end, err := time.Parse("2006-01-02", *to)
	return err == nil && !end.Before(start) && end.Year() <= 2100
}

func dateBefore(date string) string {
	parsed, _ := time.Parse("2006-01-02", date)
	return parsed.AddDate(0, 0, -1).Format("2006-01-02")
}

func (a adminAPI) payrollRuleWriter(w http.ResponseWriter, r *http.Request) (adminContext, bool) {
	c, ok := a.authorize(w, r, "penggajian.kelola")
	if !ok {
		return c, false
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum menerbitkan aturan payroll.")
		return c, false
	}
	return c, true
}

// Locking the tenant row serializes publication of different effective dates.
func lockPayrollRuleTenant(w http.ResponseWriter, r *http.Request, tx *sql.Tx, tenantID string, from string) bool {
	var locked string
	if err := tx.QueryRowContext(r.Context(), `SELECT id FROM tenants WHERE id=? FOR UPDATE`, tenantID).Scan(&locked); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci konfigurasi tenant.")
		return false
	}
	var finalized int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payroll_runs pr JOIN payroll_periods pp ON pp.tenant_id=pr.tenant_id AND pp.id=pr.period_id WHERE pr.tenant_id=? AND pr.status='finalized' AND pp.end_date>=?`, tenantID, from).Scan(&finalized); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa payroll final.")
		return false
	}
	if finalized != 0 {
		writeAPIError(w, r, 409, "PAYROLL_LOCKED", "Tanggal aturan menyentuh payroll final.")
		return false
	}
	return true
}

// The next version must start later than the latest one. An active predecessor
// is closed at the previous day in the same transaction as the new version.
func nextPayrollRuleVersion(w http.ResponseWriter, r *http.Request, tx *sql.Tx, table, keyColumn, tenantID, key, from string) (string, bool) {
	if table != "tax_rules" && table != "bpjs_rules" {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Tabel aturan tidak valid.")
		return "", false
	}
	if keyColumn != "rule_set" && keyColumn != "program" {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Jenis aturan tidak valid.")
		return "", false
	}
	var priorID, priorFrom string
	var priorTo sql.NullTime
	err := tx.QueryRowContext(r.Context(), `SELECT id,DATE_FORMAT(effective_from,'%Y-%m-%d'),effective_to FROM `+table+` WHERE tenant_id=? AND `+keyColumn+`=? ORDER BY effective_from DESC LIMIT 1 FOR UPDATE`, tenantID, key).Scan(&priorID, &priorFrom, &priorTo)
	if errors.Is(err, sql.ErrNoRows) {
		return "", true
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca versi aturan terdahulu.")
		return "", false
	}
	if strings.Compare(from, priorFrom) <= 0 {
		writeAPIError(w, r, 409, "RULE_VERSION_CONFLICT", "Tanggal berlaku harus setelah versi terbaru.")
		return "", false
	}
	if !priorTo.Valid || priorTo.Time.Format("2006-01-02") >= from {
		if _, err = tx.ExecContext(r.Context(), `UPDATE `+table+` SET effective_to=? WHERE tenant_id=? AND id=?`, dateBefore(from), tenantID, priorID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup versi aturan terdahulu.")
			return "", false
		}
	}
	return priorID, true
}
