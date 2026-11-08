package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type loanPolicyView struct {
	MaxOutstanding string    `json:"max_outstanding"`
	MaxTenorMonths int       `json:"max_tenor_months"`
	Version        uint64    `json:"version"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func scanLoanPolicy(row interface{ Scan(...any) error }) (loanPolicyView, error) {
	var item loanPolicyView
	err := row.Scan(&item.MaxOutstanding, &item.MaxTenorMonths, &item.Version, &item.UpdatedAt)
	return item, err
}

func (a adminAPI) getLoanPolicy(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kasbon.kelola")
	if !ok {
		return
	}
	item, err := scanLoanPolicy(a.db.QueryRowContext(r.Context(), `SELECT CAST(max_outstanding AS CHAR),max_tenor_months,version,updated_at FROM loan_policies WHERE tenant_id=?`, c.tenant.ID))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIData(w, 200, map[string]any{"configured": false}, nil)
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kebijakan kasbon.")
		return
	}
	writeAPIData(w, 200, map[string]any{"configured": true, "max_outstanding": item.MaxOutstanding, "max_tenor_months": item.MaxTenorMonths, "version": item.Version, "updated_at": item.UpdatedAt}, nil)
}

func (a adminAPI) putLoanPolicy(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kasbon.kelola")
	if !ok {
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum mengubah kebijakan kasbon.")
		return
	}
	var body struct {
		MaxOutstanding json.Number `json:"max_outstanding"`
		MaxTenorMonths int         `json:"max_tenor_months"`
		Version        uint64      `json:"version"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	max, valid := leaveHundredths(body.MaxOutstanding.String())
	if !valid || max <= 0 || len(strings.SplitN(body.MaxOutstanding.String(), ".", 2)[0]) > 16 || body.MaxTenorMonths < 1 || body.MaxTenorMonths > 120 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Plafon atau tenor maksimum tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kebijakan kasbon.")
		return
	}
	defer tx.Rollback()
	old, err := scanLoanPolicy(tx.QueryRowContext(r.Context(), `SELECT CAST(max_outstanding AS CHAR),max_tenor_months,version,updated_at FROM loan_policies WHERE tenant_id=? FOR UPDATE`, c.tenant.ID))
	if errors.Is(err, sql.ErrNoRows) {
		if body.Version != 0 {
			writeAPIError(w, r, 409, "VERSION_CONFLICT", "Kebijakan belum ada; gunakan versi 0.")
			return
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO loan_policies (tenant_id,max_outstanding,max_tenor_months,version,updated_by) VALUES (?,?,?,1,?)`, c.tenant.ID, leaveDecimal(max), body.MaxTenorMonths, c.state.User.ID)
		if err != nil {
			writeAPIError(w, r, 409, "VERSION_CONFLICT", "Kebijakan berubah saat disimpan.")
			return
		}
	} else {
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kebijakan kasbon.")
			return
		}
		if body.Version != old.Version {
			writeAPIError(w, r, 409, "VERSION_CONFLICT", "Kebijakan sudah berubah.")
			return
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE loan_policies SET max_outstanding=?,max_tenor_months=?,version=version+1,updated_by=? WHERE tenant_id=? AND version=?`, leaveDecimal(max), body.MaxTenorMonths, c.state.User.ID, c.tenant.ID, body.Version)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui kebijakan kasbon.")
			return
		}
	}
	var before any
	if old.Version != 0 {
		before = old
	}
	var after loanPolicyView
	if err = tx.QueryRowContext(r.Context(), `SELECT CAST(max_outstanding AS CHAR),max_tenor_months,version,updated_at FROM loan_policies WHERE tenant_id=?`, c.tenant.ID).Scan(&after.MaxOutstanding, &after.MaxTenorMonths, &after.Version, &after.UpdatedAt); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kebijakan baru.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "upsert", "loan_policies", c.tenant.ID, before, after); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat kebijakan.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kebijakan kasbon.")
		return
	}
	writeAPIData(w, 200, after, nil)
}
