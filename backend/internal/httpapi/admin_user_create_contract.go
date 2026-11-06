package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/ids"
)

func (a adminAPI) accessContractCreateUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.contractActor(w, r, "pengguna.buat")
	if !ok {
		return
	}
	valid, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !valid {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang kata sandi sebelum menambah pengguna.")
		return
	}
	var body struct {
		Email      string `json:"email"`
		Name       string `json:"name"`
		Phone      string `json:"phone"`
		EmployeeID string `json:"employee_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	body.Name = strings.TrimSpace(body.Name)
	body.Phone = strings.TrimSpace(body.Phone)
	address, err := mail.ParseAddress(body.Email)
	if err != nil || address.Address != body.Email || len(body.Email) > 320 || body.Name == "" || len(body.Name) > 200 || len(body.Phone) > 32 || body.EmployeeID != "" && len(body.EmployeeID) != 26 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Email, nama, telepon, atau karyawan tidak valid.")
		return
	}
	if !smtpConfigured() {
		writeAPIError(w, r, 503, "DELIVERY_UNAVAILABLE", "Pengiriman undangan belum dikonfigurasi.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menambah pengguna.")
		return
	}
	defer tx.Rollback()
	var tenantName string
	if err := tx.QueryRowContext(r.Context(), `SELECT name FROM tenants WHERE id=? FOR UPDATE`, actor.tenant.ID).Scan(&tenantName); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca perusahaan.")
		return
	}
	var quota sql.NullInt64
	if err := tx.QueryRowContext(r.Context(), `SELECT p.max_users FROM tenant_subscriptions ts JOIN plans p ON p.id=ts.plan_id WHERE ts.tenant_id=? AND ts.status='active'`, actor.tenant.ID).Scan(&quota); err != nil {
		writeAPIError(w, r, 409, "ENTITLEMENT_REQUIRED", "Paket aktif tidak tersedia.")
		return
	}
	if quota.Valid {
		var count int64
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id=? AND status='active'`, actor.tenant.ID).Scan(&count); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kuota pengguna.")
			return
		}
		if count >= quota.Int64 {
			writeAPIError(w, r, 409, "QUOTA_EXCEEDED", "Kuota pengguna paket telah tercapai.")
			return
		}
	}
	var userID, status, accountName string
	newUser := false
	err = tx.QueryRowContext(r.Context(), `SELECT id,status,display_name FROM users WHERE email_normalized=? FOR UPDATE`, body.Email).Scan(&userID, &status, &accountName)
	if errors.Is(err, sql.ErrNoRows) {
		userID, err = ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat identitas pengguna.")
			return
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO users (id,email,email_normalized,display_name,phone) VALUES (?,?,?,?,?)`, userID, body.Email, body.Email, body.Name, nullableString(body.Phone)); err != nil {
			writeAPIError(w, r, 409, "CONFLICT", "Email sudah digunakan.")
			return
		}
		newUser = true
		accountName = body.Name
	} else if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca akun.")
		return
	} else if status != "active" {
		writeAPIError(w, r, 409, "ACCOUNT_DISABLED", "Akun dinonaktifkan secara global.")
		return
	}
	var member int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, actor.tenant.ID, userID).Scan(&member); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa keanggotaan.")
		return
	}
	if member > 0 {
		writeAPIError(w, r, 409, "CONFLICT", "Pengguna sudah pernah tergabung di perusahaan ini.")
		return
	}
	if body.EmployeeID != "" {
		_, decision, err := (access.Store{DB: tx}).Decide(r.Context(), actor.tenant.ID, actor.state.User.ID, actor.tenant.Timezone, "karyawan.ubah")
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa izin karyawan.")
			return
		}
		all := false
		for _, rule := range decision.Scopes {
			all = all || rule.AllTenant
		}
		if !decision.Allowed || !all {
			writeAPIError(w, r, 403, "PERMISSION_DENIED", "Menghubungkan akun ke karyawan memerlukan akses seluruh tenant.")
			return
		}
		var linked sql.NullString
		err = tx.QueryRowContext(r.Context(), `SELECT user_id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, actor.tenant.ID, body.EmployeeID).Scan(&linked)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
			return
		}
		if linked.Valid {
			writeAPIError(w, r, 409, "CONFLICT", "Karyawan sudah memiliki akun.")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO tenant_memberships (tenant_id,user_id,status) VALUES (?,?,'active')`, actor.tenant.ID, userID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan keanggotaan.")
		return
	}
	if body.EmployeeID != "" {
		if _, err := tx.ExecContext(r.Context(), `UPDATE employees SET user_id=? WHERE tenant_id=? AND id=?`, userID, actor.tenant.ID, body.EmployeeID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghubungkan karyawan.")
			return
		}
	}
	if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.akun_buat", "users", userID, nil, map[string]any{"email": body.Email, "employee_id": body.EmployeeID, "new_identity": newUser}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menambah pengguna.")
		return
	}
	delivery := "sent"
	if newUser {
		service := auth.Service{DB: a.db}
		if err := service.IssuePasswordReset(r.Context(), body.Email, func(to, token string) error { return sendSetupMail(r.Context(), to, token) }); err != nil {
			delivery = "failed"
		}
	} else if err := sendTenantInviteMail(r.Context(), body.Email, tenantName); err != nil {
		delivery = "failed"
	}
	writeAPIData(w, 201, map[string]any{"id": userID, "email": body.Email, "name": accountName, "invitation_status": delivery, "new_identity": newUser}, nil)
}
