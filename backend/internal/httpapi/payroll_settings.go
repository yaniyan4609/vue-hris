package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-sql-driver/mysql"
)

const payrollSettingsKey = "payroll_settings"

type payrollSettings struct {
	Version        uint64 `json:"version"`
	PayDay         *int   `json:"pay_day"`
	RoundingMode   string `json:"rounding_mode"`
	Currency       string `json:"currency"`
	ProrationBasis string `json:"proration_basis"`
}

func defaultPayrollSettings() payrollSettings {
	return payrollSettings{Version: 0, RoundingMode: "none", Currency: "IDR", ProrationBasis: "published_roster_workdays"}
}

func (a adminAPI) getPayrollSettings(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	item := defaultPayrollSettings()
	var raw []byte
	err := a.db.QueryRowContext(r.Context(), `SELECT value_json FROM tenant_settings WHERE tenant_id=? AND setting_key=?`, c.tenant.ID, payrollSettingsKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIData(w, 200, item, nil)
		return
	}
	if err != nil || json.Unmarshal(raw, &item) != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Pengaturan payroll tidak dapat dibaca.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) putPayrollSettings(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.kelola")
	if !ok {
		return
	}
	var input struct {
		Version        *uint64 `json:"version"`
		PayDay         *int    `json:"pay_day"`
		RoundingMode   string  `json:"rounding_mode"`
		Currency       string  `json:"currency"`
		ProrationBasis string  `json:"proration_basis"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if input.Version == nil || input.PayDay == nil || *input.PayDay < 1 || *input.PayDay > 31 || (input.RoundingMode != "none" && input.RoundingMode != "half_up_rupiah") || input.Currency != "IDR" || input.ProrationBasis != "published_roster_workdays" {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal gajian, pembulatan, mata uang, atau dasar prorata tidak valid.")
		return
	}
	body := payrollSettings{Version: *input.Version, PayDay: input.PayDay, RoundingMode: input.RoundingMode, Currency: input.Currency, ProrationBasis: input.ProrationBasis}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pengaturan payroll.")
		return
	}
	defer tx.Rollback()
	var tenant string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM tenants WHERE id=? FOR UPDATE`, c.tenant.ID).Scan(&tenant); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci pengaturan payroll.")
		return
	}
	previous := defaultPayrollSettings()
	var raw []byte
	err = tx.QueryRowContext(r.Context(), `SELECT value_json FROM tenant_settings WHERE tenant_id=? AND setting_key=? FOR UPDATE`, c.tenant.ID, payrollSettingsKey).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengaturan payroll.")
		return
	}
	if err == nil && json.Unmarshal(raw, &previous) != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Pengaturan payroll tidak valid.")
		return
	}
	if body.Version != previous.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pengaturan payroll telah berubah; muat ulang sebelum menyimpan.")
		return
	}
	body.Version++
	encoded, err := json.Marshal(body)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pengaturan payroll.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO tenant_settings (tenant_id,setting_key,value_json,updated_by) VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE value_json=VALUES(value_json),updated_by=VALUES(updated_by)`, c.tenant.ID, payrollSettingsKey, encoded, c.state.User.ID)
	if err != nil {
		var dbErr *mysql.MySQLError
		if errors.As(err, &dbErr) && dbErr.Number == 1062 {
			writeAPIError(w, r, 409, "VERSION_CONFLICT", "Pengaturan payroll telah berubah.")
		} else {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pengaturan payroll.")
		}
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "payroll_settings", c.tenant.ID, previous, body); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat pengaturan payroll.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pengaturan payroll.")
		return
	}
	writeAPIData(w, 200, body, nil)
}
