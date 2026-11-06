package httpapi

import (
	"database/sql"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func (a adminAPI) memberInTenant(w http.ResponseWriter, r *http.Request, actor adminContext) bool {
	var exists int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, actor.tenant.ID, chi.URLParam(r, "id")).Scan(&exists); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengguna.")
		return false
	}
	if exists == 0 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengguna tidak ditemukan.")
		return false
	}
	return true
}

func (a adminAPI) accessContractUserSessions(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.contractActor(w, r, "pengguna.lihat")
	if !ok || !a.memberInTenant(w, r, actor) {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,COALESCE(device_label,'Perangkat'),ip,remember,created_at,last_seen_at FROM sessions WHERE user_id=? AND selected_tenant_id=? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>UTC_TIMESTAMP(6)) AND (remember=1 OR TIMESTAMPDIFF(SECOND,last_seen_at,UTC_TIMESTAMP(6))<=idle_timeout_minutes*60) ORDER BY last_seen_at DESC LIMIT 100`, chi.URLParam(r, "id"), actor.tenant.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca sesi pengguna.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, label string
		var ip []byte
		var remember bool
		var created, last time.Time
		if err := rows.Scan(&id, &label, &ip, &remember, &created, &last); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca sesi pengguna.")
			return
		}
		address := ""
		if len(ip) == net.IPv6len {
			address = net.IP(ip).String()
		}
		items = append(items, map[string]any{"id": id, "device_label": label, "ip": address, "remember": remember, "created_at": created, "last_seen_at": last})
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca sesi pengguna.")
		return
	}
	writeAPIData(w, 200, items, nil)
}

func (a adminAPI) accessContractRevokeUserSessions(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.contractActor(w, r, "pengguna.cabut_sesi")
	if !ok {
		return
	}
	valid, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !valid {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang kata sandi sebelum mencabut sesi.")
		return
	}
	if !a.memberInTenant(w, r, actor) {
		return
	}
	targetID, sessionID := chi.URLParam(r, "id"), chi.URLParam(r, "sessionId")
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencabut sesi.")
		return
	}
	defer tx.Rollback()
	query := `UPDATE sessions SET revoked_at=UTC_TIMESTAMP(6),revoked_reason='admin_revoked' WHERE user_id=? AND selected_tenant_id=? AND revoked_at IS NULL`
	args := []any{targetID, actor.tenant.ID}
	if sessionID != "" {
		query += ` AND id=?`
		args = append(args, sessionID)
	}
	result, err := tx.ExecContext(r.Context(), query, args...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencabut sesi.")
		return
	}
	count, err := result.RowsAffected()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencabut sesi.")
		return
	}
	if sessionID != "" && count == 0 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Sesi tidak ditemukan dalam tenant ini.")
		return
	}
	if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.sesi_cabut", "users", targetID, nil, map[string]any{"session_id": sessionID, "count": count}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencabut sesi.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a adminAPI) accessContractSetUserActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := a.contractActor(w, r, "pengguna.nonaktifkan")
		if !ok {
			return
		}
		valid, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
		if err != nil || !valid {
			writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang kata sandi sebelum mengubah status pengguna.")
			return
		}
		targetID := chi.URLParam(r, "id")
		if targetID == actor.state.User.ID {
			writeAPIError(w, r, 403, "SELF_CHANGE_DENIED", "Akun sendiri tidak dapat dinonaktifkan atau diaktifkan.")
			return
		}
		tx, err := a.db.BeginTx(r.Context(), nil)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah status pengguna.")
			return
		}
		defer tx.Rollback()
		var membership, global string
		err = tx.QueryRowContext(r.Context(), `SELECT tm.status,u.status FROM tenant_memberships tm JOIN users u ON u.id=tm.user_id WHERE tm.tenant_id=? AND tm.user_id=? FOR UPDATE`, actor.tenant.ID, targetID).Scan(&membership, &global)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Pengguna tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengguna.")
			return
		}
		if active && global != "active" {
			writeAPIError(w, r, 409, "ACCOUNT_DISABLED", "Akun dinonaktifkan secara global.")
			return
		}
		before := membership
		after := "inactive"
		if active {
			after = "active"
		}
		if before == after {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if active {
			_, err = tx.ExecContext(r.Context(), `UPDATE tenant_memberships SET status='active',ended_at=NULL,access_version=access_version+1 WHERE tenant_id=? AND user_id=?`, actor.tenant.ID, targetID)
		} else {
			_, err = tx.ExecContext(r.Context(), `UPDATE tenant_memberships SET status='inactive',ended_at=UTC_TIMESTAMP(6),access_version=access_version+1 WHERE tenant_id=? AND user_id=?`, actor.tenant.ID, targetID)
			if err == nil {
				_, err = tx.ExecContext(r.Context(), `UPDATE sessions SET revoked_at=UTC_TIMESTAMP(6),revoked_reason='membership_disabled' WHERE user_id=? AND selected_tenant_id=? AND revoked_at IS NULL`, targetID, actor.tenant.ID)
			}
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah status pengguna.")
			return
		}
		if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.status_ubah", "users", targetID, map[string]string{"status": before}, map[string]string{"status": after}); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah status pengguna.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
