package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func (a adminAPI) accessContractUpdateRoleAssignment(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.contractActor(w, r, "pengguna.kelola_akses")
	if !ok {
		return
	}
	var body struct {
		ValidFrom  string `json:"valid_from"`
		ValidUntil string `json:"valid_until"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.ValidFrom == "" {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal mulai wajib diisi.")
		return
	}
	if _, err := time.Parse("2006-01-02", body.ValidFrom); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal mulai tidak valid.")
		return
	}
	if body.ValidUntil != "" {
		if _, err := time.Parse("2006-01-02", body.ValidUntil); err != nil || body.ValidUntil < body.ValidFrom {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal akhir tidak valid.")
			return
		}
	}
	targetID, assignmentID := chi.URLParam(r, "id"), chi.URLParam(r, "assignmentId")
	tx, ok := a.beginAccessChange(w, r, actor, targetID, "")
	if !ok {
		return
	}
	defer tx.Rollback()
	var roleID, oldFrom string
	var oldUntil sql.NullString
	err := tx.QueryRowContext(r.Context(), `SELECT role_id,DATE_FORMAT(valid_from,'%Y-%m-%d'),DATE_FORMAT(valid_until,'%Y-%m-%d') FROM user_roles WHERE tenant_id=? AND user_id=? AND id=? AND revoked_at IS NULL FOR UPDATE`, actor.tenant.ID, targetID, assignmentID).Scan(&roleID, &oldFrom, &oldUntil)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Penugasan role tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca penugasan role.")
		return
	}
	delegable, err := a.roleDelegable(r.Context(), tx, actor, roleID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa delegasi role.")
		return
	}
	if !delegable {
		writeAPIError(w, r, 403, "DELEGATION_EXCEEDED", "Role melebihi hak delegasi Admin.")
		return
	}
	until := body.ValidUntil
	if until == "" {
		until = "9999-12-31"
	}
	var overlaps int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM user_roles WHERE tenant_id=? AND user_id=? AND role_id=? AND id<>? AND revoked_at IS NULL AND valid_from<=? AND (valid_until IS NULL OR valid_until>=?)`, actor.tenant.ID, targetID, roleID, assignmentID, until, body.ValidFrom).Scan(&overlaps); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa periode role.")
		return
	}
	if overlaps > 0 {
		writeAPIError(w, r, 409, "CONFLICT", "Periode role bertumpang tindih.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE user_roles SET valid_from=?,valid_until=? WHERE tenant_id=? AND user_id=? AND id=?`, body.ValidFrom, nullableString(body.ValidUntil), actor.tenant.ID, targetID, assignmentID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah periode role.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE tenant_memberships SET access_version=access_version+1 WHERE tenant_id=? AND user_id=?`, actor.tenant.ID, targetID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui akses pengguna.")
		return
	}
	if err := audit(r.Context(), tx, actor.tenant.ID, actor.state.User.ID, "pengguna.role_periode", "user_roles", assignmentID, map[string]any{"valid_from": oldFrom, "valid_until": oldUntil.String}, map[string]string{"valid_from": body.ValidFrom, "valid_until": body.ValidUntil}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah periode role.")
		return
	}
	writeAPIData(w, http.StatusOK, map[string]string{"assignment_id": assignmentID, "valid_from": body.ValidFrom, "valid_until": body.ValidUntil}, nil)
}
