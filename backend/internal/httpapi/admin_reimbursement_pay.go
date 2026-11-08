package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

func validPaymentRef(value string) bool {
	if value == "" || len(value) > 160 || strings.TrimSpace(value) != value {
		return false
	}
	for _, ch := range value {
		if ch < 33 || ch > 126 {
			return false
		}
	}
	return true
}

func (a adminAPI) payAdminReimbursement(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "reimbursement.bayar")
	if !ok {
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum menandai klaim dibayar.")
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Klaim tidak ditemukan dalam scope.")
		return
	}
	var body struct {
		PaymentRef string `json:"payment_ref"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if !validPaymentRef(body.PaymentRef) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Referensi pembayaran tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memproses pembayaran klaim.")
		return
	}
	defer tx.Rollback()
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	args := append([]any{c.tenant.ID, id}, scopeArgs...)
	var status string
	var requestID, oldRef sql.NullString
	err = tx.QueryRowContext(r.Context(), `SELECT r.status,r.request_id,r.payment_ref`+reimbursementAdminFrom+` WHERE r.tenant_id=? AND r.id=? AND `+scope+` FOR UPDATE`, args...).Scan(&status, &requestID, &oldRef)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Klaim tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca klaim.")
		return
	}
	if status == "paid" {
		if oldRef.Valid && oldRef.String == body.PaymentRef {
			writeAPIData(w, 200, map[string]any{"id": id, "status": "paid", "payment_ref": body.PaymentRef, "idempotent": true}, nil)
			return
		}
		writeAPIError(w, r, 409, "ALREADY_PAID", "Klaim sudah dibayar dengan referensi lain.")
		return
	}
	if status != "approved" {
		writeAPIError(w, r, 409, "CLAIM_NOT_APPROVED", "Hanya klaim yang disetujui dapat dibayar.")
		return
	}
	if !requestID.Valid {
		writeAPIError(w, r, 409, "APPROVAL_REQUIRED", "Klaim belum memiliki pengajuan persetujuan.")
		return
	}
	var requestStatus, requestType, refTable, refID, instanceStatus string
	err = tx.QueryRowContext(r.Context(), `SELECT q.status,q.request_type,q.ref_table,q.ref_id,i.status FROM requests q JOIN approval_instances i ON i.tenant_id=q.tenant_id AND i.request_id=q.id WHERE q.tenant_id=? AND q.id=? FOR UPDATE`, c.tenant.ID, requestID.String).Scan(&requestStatus, &requestType, &refTable, &refID, &instanceStatus)
	if err != nil || requestStatus != "approved" || instanceStatus != "approved" || requestType != "reimbursement" || refTable != "reimbursements" || refID != id {
		writeAPIError(w, r, 409, "APPROVAL_REQUIRED", "Proses persetujuan klaim belum lengkap.")
		return
	}
	var used int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM reimbursements WHERE tenant_id=? AND payment_ref=? AND id<>?`, c.tenant.ID, body.PaymentRef, id).Scan(&used); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa referensi pembayaran.")
		return
	}
	if used > 0 {
		writeAPIError(w, r, 409, "PAYMENT_REF_EXISTS", "Referensi pembayaran sudah dipakai.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE reimbursements SET status='paid',payment_ref=?,paid_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=? AND status='approved'`, body.PaymentRef, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 409, "PAYMENT_REF_EXISTS", "Referensi pembayaran sudah dipakai.")
		return
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		writeAPIError(w, r, 409, "CLAIM_CHANGED", "Status klaim sudah berubah.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "pay", "reimbursements", id, map[string]any{"status": "approved"}, map[string]any{"status": "paid", "payment_ref": body.PaymentRef}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat pembayaran.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pembayaran.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "status": "paid", "payment_ref": body.PaymentRef, "idempotent": false}, nil)
}
