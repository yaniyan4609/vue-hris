package httpapi

import (
	"database/sql"
	"errors"
	"net/http"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) accessContractBulkOverrides(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.contractActor(w, r, "pengguna.kelola_akses")
	if !ok {
		return
	}
	var body struct {
		Overrides []struct {
			PermissionCode string `json:"permission_code"`
			Decision       string `json:"decision"`
		} `json:"overrides"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if len(body.Overrides) == 0 || len(body.Overrides) > 100 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Pilih 1 sampai 100 perubahan izin.")
		return
	}
	seen := map[string]bool{}
	for _, item := range body.Overrides {
		if item.PermissionCode == "" || seen[item.PermissionCode] || item.Decision != "allow" && item.Decision != "deny" && item.Decision != "inherit" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Daftar izin tidak valid atau berulang.")
			return
		}
		seen[item.PermissionCode] = true
	}
	targetID := chi.URLParam(r, "id")
	tx, ok := a.beginAccessChange(w, r, actor, targetID, "")
	if !ok {
		return
	}
	defer tx.Rollback()
	store := access.Store{DB: tx}
	for _, item := range body.Overrides {
		var scoped bool
		err := tx.QueryRowContext(r.Context(), `SELECT requires_scope FROM permission_registry WHERE code=?`, item.PermissionCode).Scan(&scoped)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Izin tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin.")
			return
		}
		_, decision, err := store.Decide(r.Context(), actor.tenant.ID, actor.state.User.ID, actor.tenant.Timezone, item.PermissionCode)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa delegasi.")
			return
		}
		if !decision.Allowed {
			writeAPIError(w, r, 403, "DELEGATION_EXCEEDED", "Izin melebihi hak delegasi Admin.")
			return
		}
		if scoped {
			all := false
			for _, rule := range decision.Scopes {
				all = all || rule.AllTenant
			}
			if !all {
				writeAPIError(w, r, 403, "DELEGATION_EXCEEDED", "Scope melebihi hak delegasi Admin.")
				return
			}
			if item.Decision == "allow" {
				var count int
				if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM user_scope_rules WHERE tenant_id=? AND user_id=? AND permission_code=?`, actor.tenant.ID, targetID, item.PermissionCode).Scan(&count); err != nil {
					writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca scope pengguna.")
					return
				}
				if count == 0 {
					writeAPIError(w, r, 400, "SCOPE_REQUIRED", "Atur scope pengguna sebelum memberi izin khusus.")
					return
				}
			}
		}
		if item.Decision == "inherit" {
			var count int
			if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM user_scope_overrides WHERE tenant_id=? AND user_id=? AND permission_code=?`, actor.tenant.ID, targetID, item.PermissionCode).Scan(&count); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca scope pengguna.")
				return
			}
			if count > 0 {
				writeAPIError(w, r, 409, "SCOPE_OVERRIDE_EXISTS", "Hapus scope khusus sebelum memulihkan izin.")
				return
			}
		}
	}
	changed := false
	for _, item := range body.Overrides {
		var previous sql.NullString
		err := tx.QueryRowContext(r.Context(), `SELECT decision FROM user_permission_overrides WHERE tenant_id=? AND user_id=? AND permission_code=? FOR UPDATE`, actor.tenant.ID, targetID, item.PermissionCode).Scan(&previous)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca izin pengguna.")
			return
		}
		before := "inherit"
		if previous.Valid {
			before = previous.String
		}
		if before == item.Decision {
			continue
		}
		if item.Decision == "inherit" {
			_, err = tx.ExecContext(r.Context(), `DELETE FROM user_permission_overrides WHERE tenant_id=? AND user_id=? AND permission_code=?`, actor.tenant.ID, targetID, item.PermissionCode)
		} else {
			_, err = tx.ExecContext(r.Context(), `INSERT INTO user_permission_overrides (tenant_id,user_id,permission_code,decision) VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE decision=VALUES(decision)`, actor.tenant.ID, targetID, item.PermissionCode, item.Decision)
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan izin pengguna.")
			return
		}
		if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.izin_ubah", "user_permission_overrides", targetID, map[string]string{"permission": item.PermissionCode, "decision": before}, map[string]string{"permission": item.PermissionCode, "decision": item.Decision}); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
			return
		}
		changed = true
	}
	if changed {
		if _, err := tx.ExecContext(r.Context(), `UPDATE tenant_memberships SET access_version=access_version+1 WHERE tenant_id=? AND user_id=?`, actor.tenant.ID, targetID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui versi akses.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan perubahan izin.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
