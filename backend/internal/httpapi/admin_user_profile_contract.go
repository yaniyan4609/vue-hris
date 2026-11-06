package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"digaji.id/hris/internal/auth"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) accessContractUpdateUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.contractActor(w, r, "pengguna.ubah")
	if !ok {
		return
	}
	valid, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !valid {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang kata sandi sebelum mengubah pengguna.")
		return
	}
	var body struct {
		Name  *string `json:"name"`
		Phone *string `json:"phone"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Name == nil && body.Phone == nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Pilih data pengguna yang akan diubah.")
		return
	}
	targetID := chi.URLParam(r, "id")
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah pengguna.")
		return
	}
	defer tx.Rollback()
	var previousName string
	var previousPhone sql.NullString
	err = tx.QueryRowContext(r.Context(), `SELECT u.display_name,u.phone FROM tenant_memberships tm JOIN users u ON u.id=tm.user_id WHERE tm.tenant_id=? AND tm.user_id=? AND tm.status='active' FOR UPDATE`, actor.tenant.ID, targetID).Scan(&previousName, &previousPhone)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengguna tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengguna.")
		return
	}
	var membershipCount int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenant_memberships WHERE user_id=?`, targetID).Scan(&membershipCount); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa keanggotaan.")
		return
	}
	if membershipCount > 1 {
		writeAPIError(w, r, 409, "SHARED_IDENTITY", "Profil akun yang tergabung di beberapa tenant dikelola oleh pemilik akun.")
		return
	}
	name, phone := previousName, previousPhone.String
	if body.Name != nil {
		name = strings.TrimSpace(*body.Name)
	}
	if body.Phone != nil {
		phone = strings.TrimSpace(*body.Phone)
	}
	if name == "" || len(name) > 200 || len(phone) > 32 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Nama atau telepon tidak valid.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE users SET display_name=?,phone=? WHERE id=?`, name, nullableString(phone), targetID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah pengguna.")
		return
	}
	if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.profil_ubah", "users", targetID, map[string]string{"name": previousName, "phone": previousPhone.String}, map[string]string{"name": name, "phone": phone}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah pengguna.")
		return
	}
	writeAPIData(w, 200, map[string]string{"id": targetID, "name": name, "phone": phone}, nil)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (a adminAPI) accessContractSendReset(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.contractActor(w, r, "pengguna.ubah")
	if !ok {
		return
	}
	valid, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !valid {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang kata sandi sebelum mengirim reset.")
		return
	}
	if !smtpConfigured() {
		writeAPIError(w, r, 503, "DELIVERY_UNAVAILABLE", "Pengiriman email belum dikonfigurasi.")
		return
	}
	targetID := chi.URLParam(r, "id")
	var email string
	err = a.db.QueryRowContext(r.Context(), `SELECT u.email FROM tenant_memberships tm JOIN users u ON u.id=tm.user_id WHERE tm.tenant_id=? AND tm.user_id=? AND tm.status='active' AND u.status='active'`, actor.tenant.ID, targetID).Scan(&email)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengguna tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengguna.")
		return
	}
	service := auth.Service{DB: a.db}
	if err := service.IssuePasswordReset(r.Context(), email, func(to, token string) error { return sendResetMail(r.Context(), to, token) }); err != nil {
		writeAPIError(w, r, 503, "DELIVERY_UNAVAILABLE", "Email reset belum dapat dikirim.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit reset.")
		return
	}
	defer tx.Rollback()
	if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.reset_dikirim", "users", targetID, nil, map[string]string{"delivery": "email"}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit reset.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit reset.")
		return
	}
	writeAPIData(w, http.StatusAccepted, map[string]string{"message": "Jika akun tersedia, tautan reset dikirim ke email."}, nil)
}
