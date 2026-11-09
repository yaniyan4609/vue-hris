package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) cancelTermination(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "terminasi.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Terminasi tidak ditemukan.")
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum membatalkan terminasi.")
		return
	}
	var body struct {
		Version uint64 `json:"version"`
		Reason  string `json:"reason"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if body.Version == 0 || len([]rune(body.Reason)) < 10 || len([]rune(body.Reason)) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi dan alasan pembatalan 10–500 karakter wajib diisi.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan terminasi.")
		return
	}
	defer tx.Rollback()
	old, err := lockTerminationEmployee(r, tx, c, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Terminasi tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca terminasi.")
		return
	}
	if old.Version != body.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Terminasi sudah berubah.")
		return
	}
	if old.Status != "draft" && old.Status != "returned" && old.Status != "in_review" && old.Status != "approved" {
		writeAPIError(w, r, 409, "TERMINATION_LOCKED", "Terminasi tidak dapat dibatalkan.")
		return
	}
	if old.EffectiveDate <= tenantToday(c.tenant.Timezone) && old.Status == "approved" {
		writeAPIError(w, r, 409, "EFFECTIVE_DATE_REACHED", "Tanggal efektif sudah tiba.")
		return
	}
	if old.RequestID != nil {
		var requestStatus, instanceID, instanceStatus string
		var attempt uint64
		if err = tx.QueryRowContext(r.Context(), `SELECT status,attempt_no FROM requests WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, *old.RequestID).Scan(&requestStatus, &attempt); err != nil {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Pengajuan tidak tersedia.")
			return
		}
		if err = tx.QueryRowContext(r.Context(), `SELECT id,status FROM approval_instances WHERE tenant_id=? AND request_id=? FOR UPDATE`, c.tenant.ID, *old.RequestID).Scan(&instanceID, &instanceStatus); err != nil {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Approval tidak tersedia.")
			return
		}
		if requestStatus != old.Status || instanceStatus != old.Status {
			writeAPIError(w, r, 409, "REQUEST_INVALID", "Status approval tidak sesuai terminasi.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE approval_tasks SET status='cancelled',decided_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND instance_id=? AND attempt_no=? AND status='pending'`, c.tenant.ID, instanceID, attempt); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup tugas.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE approval_instances SET status='cancelled',version=version+1,finished_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, c.tenant.ID, instanceID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup approval.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE requests SET status='cancelled',version=version+1,final_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, c.tenant.ID, *old.RequestID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup pengajuan.")
			return
		}
		outboxID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'approval.cancelled','request',?,?,?)`, c.tenant.ID, outboxID, *old.RequestID, `{"request_type":"termination"}`, "approval.cancelled:"+*old.RequestID+":"+outboxID); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE terminations SET status='cancelled',cancelled_at=UTC_TIMESTAMP(6),version=version+1 WHERE tenant_id=? AND id=? AND version=?`, c.tenant.ID, id, old.Version); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan terminasi.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "cancel", "terminations", id, map[string]any{"status": old.Status, "version": old.Version}, map[string]any{"status": "cancelled", "version": old.Version + 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan terminasi.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "status": "cancelled", "version": old.Version + 1}, nil)
}
