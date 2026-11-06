package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/ids"
)

func createPlatformStaff(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	actor, ok := authorizePlatform(w, r, db, "platform.pengguna.kelola")
	if !ok {
		return
	}
	valid, err := (auth.Service{DB: db}).RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !valid {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang kata sandi sebelum menambah staf platform.")
		return
	}
	var body struct {
		Email     string   `json:"email"`
		Name      string   `json:"name"`
		RoleCode  string   `json:"role_code"`
		TenantIDs []string `json:"tenant_ids"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	body.Name = strings.TrimSpace(body.Name)
	address, err := mail.ParseAddress(body.Email)
	if err != nil || address.Address != body.Email || len(body.Email) > 320 || body.Name == "" || len(body.Name) > 200 || len(body.TenantIDs) > 200 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data staf platform tidak valid.")
		return
	}
	if body.RoleCode != "SUPERADMIN" && body.RoleCode != "ADMIN_PLATFORM" && body.RoleCode != "SUPPORT" && body.RoleCode != "FINANCE_PLATFORM" {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Role platform tidak valid.")
		return
	}
	if body.RoleCode == "SUPERADMIN" && len(body.TenantIDs) > 0 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Superadmin memiliki akses seluruh tenant.")
		return
	}
	if !smtpConfigured() {
		writeAPIError(w, r, 503, "DELIVERY_UNAVAILABLE", "Pengiriman undangan belum dikonfigurasi.")
		return
	}
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menambah staf platform.")
		return
	}
	defer tx.Rollback()
	var roleID string
	if err := tx.QueryRowContext(r.Context(), `SELECT id FROM platform_roles WHERE code=?`, body.RoleCode).Scan(&roleID); err != nil {
		writeAPIError(w, r, 404, "NOT_FOUND", "Role platform tidak ditemukan.")
		return
	}
	var userID, status, accountName string
	newUser := false
	err = tx.QueryRowContext(r.Context(), `SELECT id,status,display_name FROM users WHERE email_normalized=? FOR UPDATE`, body.Email).Scan(&userID, &status, &accountName)
	if errors.Is(err, sql.ErrNoRows) {
		userID, err = ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat identitas staf.")
			return
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO users (id,email,email_normalized,display_name) VALUES (?,?,?,?)`, userID, body.Email, body.Email, body.Name); err != nil {
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
	var existing int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM platform_user_roles WHERE user_id=?`, userID).Scan(&existing); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa staf.")
		return
	}
	if existing > 0 {
		writeAPIError(w, r, 409, "CONFLICT", "Akun sudah menjadi staf platform.")
		return
	}
	seen := map[string]bool{}
	for _, tenantID := range body.TenantIDs {
		if len(tenantID) != 26 || seen[tenantID] {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Penugasan tenant tidak valid atau berulang.")
			return
		}
		seen[tenantID] = true
		var exists int
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenants WHERE id=?`, tenantID).Scan(&exists); err != nil || exists != 1 {
			writeAPIError(w, r, 404, "NOT_FOUND", "Tenant penugasan tidak ditemukan.")
			return
		}
	}
	date := time.Now().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02")
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO platform_user_roles (user_id,role_id,valid_from) VALUES (?,?,?)`, userID, roleID, date); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menugaskan role platform.")
		return
	}
	for _, tenantID := range body.TenantIDs {
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO platform_user_tenants (user_id,tenant_id) VALUES (?,?)`, userID, tenantID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menugaskan tenant.")
			return
		}
	}
	if err := platformAudit(r, tx, actor.state.User.ID, userID, "platform.staf_buat", map[string]any{"role_code": body.RoleCode, "tenant_ids": body.TenantIDs, "new_identity": newUser}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menambah staf platform.")
		return
	}
	delivery := "sent"
	if newUser {
		service := auth.Service{DB: db}
		if err := service.IssuePasswordReset(r.Context(), body.Email, func(to, token string) error { return sendSetupMail(r.Context(), to, token) }); err != nil {
			delivery = "failed"
		}
	} else if err := sendTenantInviteMail(r.Context(), body.Email, "platform digaji.id"); err != nil {
		delivery = "failed"
	}
	writeAPIData(w, 201, map[string]any{"id": userID, "email": body.Email, "name": accountName, "role_code": body.RoleCode, "tenant_ids": body.TenantIDs, "invitation_status": delivery, "new_identity": newUser}, nil)
}
