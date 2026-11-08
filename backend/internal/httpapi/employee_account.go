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
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) linkEmployeeAccount(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	_, canCreate, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "pengguna.buat")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak akun.")
		return
	}
	if !canCreate.Allowed {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak membuat pengguna diperlukan.")
		return
	}
	employeeID := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(employeeID) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum menghubungkan akun ESS.")
		return
	}
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	body.Name = strings.TrimSpace(body.Name)
	body.Phone = strings.TrimSpace(body.Phone)
	address, err := mail.ParseAddress(body.Email)
	if err != nil || address.Address != body.Email || len(body.Email) > 320 || len([]rune(body.Name)) > 200 || len(body.Phone) > 32 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Email, nama, atau telepon tidak valid.")
		return
	}
	if !smtpConfigured() {
		writeAPIError(w, r, 503, "DELIVERY_UNAVAILABLE", "Pengiriman undangan belum dikonfigurasi.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat akun karyawan.")
		return
	}
	defer tx.Rollback()
	var tenantName string
	if err = tx.QueryRowContext(r.Context(), `SELECT name FROM tenants WHERE id=? FOR UPDATE`, c.tenant.ID).Scan(&tenantName); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tenant.")
		return
	}
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	var linked sql.NullString
	var employeeName string
	var deleted sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT e.user_id,e.display_name,e.deleted_at FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE e.tenant_id=? AND e.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, employeeID}, args...)...).Scan(&linked, &employeeName, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return
	}
	if deleted.Valid {
		writeAPIError(w, r, 409, "EMPLOYEE_ARCHIVED", "Karyawan sudah diarsipkan.")
		return
	}
	if body.Name == "" {
		body.Name = employeeName
	}
	var userID, status, accountName string
	var passwordHash sql.NullString
	newIdentity := false
	err = tx.QueryRowContext(r.Context(), `SELECT id,status,display_name,password_hash FROM users WHERE email_normalized=? FOR UPDATE`, body.Email).Scan(&userID, &status, &accountName, &passwordHash)
	if errors.Is(err, sql.ErrNoRows) {
		userID, err = ids.New()
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID akun.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO users (id,email,email_normalized,display_name,phone) VALUES (?,?,?,?,?)`, userID, body.Email, body.Email, body.Name, nullableString(body.Phone)); err != nil {
			writeAPIError(w, r, 409, "CONFLICT", "Email sudah digunakan.")
			return
		}
		newIdentity = true
		accountName = body.Name
	} else if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca akun.")
		return
	} else if status != "active" {
		writeAPIError(w, r, 409, "ACCOUNT_DISABLED", "Akun dinonaktifkan.")
		return
	}
	if linked.Valid {
		if linked.String == userID {
			_ = tx.Rollback()
			invitation := a.sendEmployeeAccountInvitation(r, body.Email, tenantName, !passwordHash.Valid)
			writeAPIData(w, 200, map[string]any{"user_id": userID, "employee_id": employeeID, "already_linked": true, "invitation_status": invitation}, nil)
			return
		}
		writeAPIError(w, r, 409, "ACCOUNT_LINKED", "Karyawan sudah memiliki akun lain.")
		return
	}
	var memberStatus string
	var memberEmployee sql.NullString
	err = tx.QueryRowContext(r.Context(), `SELECT status,employee_id FROM tenant_memberships WHERE tenant_id=? AND user_id=? FOR UPDATE`, c.tenant.ID, userID).Scan(&memberStatus, &memberEmployee)
	newMembership := errors.Is(err, sql.ErrNoRows)
	if err != nil && !newMembership {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca keanggotaan.")
		return
	}
	if !newMembership && (memberStatus != "active" || memberEmployee.Valid) {
		writeAPIError(w, r, 409, "MEMBERSHIP_UNAVAILABLE", "Akun sudah terhubung atau keanggotaannya tidak aktif.")
		return
	}
	if newMembership {
		var quota sql.NullInt64
		if err = tx.QueryRowContext(r.Context(), `SELECT p.max_users FROM tenant_subscriptions s JOIN plans p ON p.id=s.plan_id WHERE s.tenant_id=? AND s.status='active'`, c.tenant.ID).Scan(&quota); err != nil {
			writeAPIError(w, r, 409, "ENTITLEMENT_REQUIRED", "Paket aktif tidak tersedia.")
			return
		}
		if quota.Valid {
			var count int64
			if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id=? AND status='active'`, c.tenant.ID).Scan(&count); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung kuota pengguna.")
				return
			}
			if count >= quota.Int64 {
				writeAPIError(w, r, 422, "QUOTA_EXCEEDED", "Kuota pengguna penuh.")
				return
			}
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO tenant_memberships (tenant_id,user_id,status,employee_id) VALUES (?,?,'active',?)`, c.tenant.ID, userID, employeeID); err != nil {
			writeAPIError(w, r, 409, "CONFLICT", "Akun tidak dapat dihubungkan.")
			return
		}
	} else {
		if _, err = tx.ExecContext(r.Context(), `UPDATE tenant_memberships SET employee_id=?,access_version=access_version+1 WHERE tenant_id=? AND user_id=?`, employeeID, c.tenant.ID, userID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghubungkan keanggotaan.")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE employees SET user_id=?,version=version+1 WHERE tenant_id=? AND id=?`, userID, c.tenant.ID, employeeID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghubungkan karyawan.")
		return
	}
	var roleID string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM roles WHERE tenant_id=? AND code='KARYAWAN'`, c.tenant.ID).Scan(&roleID); err != nil {
		writeAPIError(w, r, 409, "ROLE_UNAVAILABLE", "Role ESS belum tersedia.")
		return
	}
	var existingRole int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM user_roles WHERE tenant_id=? AND user_id=? AND role_id=? AND revoked_at IS NULL AND valid_from<=CURRENT_DATE() AND (valid_until IS NULL OR valid_until>=CURRENT_DATE())`, c.tenant.ID, userID, roleID).Scan(&existingRole); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa role ESS.")
		return
	}
	if existingRole == 0 {
		assignmentID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID role.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO user_roles (tenant_id,id,user_id,role_id,valid_from) VALUES (?,?,?,?,CURRENT_DATE())`, c.tenant.ID, assignmentID, userID, roleID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memberikan role ESS.")
			return
		}
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "link_account", "employees", employeeID, nil, map[string]any{"user_id": userID, "new_identity": newIdentity}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit akun.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat akun karyawan.")
		return
	}
	delivery := a.sendEmployeeAccountInvitation(r, body.Email, tenantName, newIdentity || !passwordHash.Valid)
	writeAPIData(w, 201, map[string]any{"user_id": userID, "employee_id": employeeID, "name": accountName, "email": body.Email, "invitation_status": delivery, "new_identity": newIdentity}, nil)
}

func (a adminAPI) sendEmployeeAccountInvitation(r *http.Request, email, tenantName string, needsSetup bool) string {
	if needsSetup {
		service := auth.Service{DB: a.db}
		if err := service.IssuePasswordReset(r.Context(), email, func(to, token string) error { return sendSetupMail(r.Context(), to, token) }); err != nil {
			return "failed"
		}
		return "sent"
	}
	if err := sendTenantInviteMail(r.Context(), email, tenantName); err != nil {
		return "failed"
	}
	return "sent"
}
