package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) loanManager(w http.ResponseWriter, r *http.Request) (adminContext, bool) {
	c, ok := a.authorize(w, r, "kasbon.kelola")
	if !ok {
		return c, false
	}
	subject, decision, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "kasbon.lihat")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope kasbon.")
		return c, false
	}
	if !decision.Allowed {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak melihat kasbon diperlukan.")
		return c, false
	}
	c.subject, c.effective = subject, decision
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum mengubah cicilan kasbon.")
		return c, false
	}
	return c, true
}

func (a adminAPI) waiveLoanInstallment(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loanManager(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	seq, err := strconv.Atoi(chi.URLParam(r, "seq"))
	if !fileIDPattern.MatchString(id) || err != nil || seq < 1 || seq > 120 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Cicilan tidak ditemukan.")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if body.Reason == "" || len([]rune(body.Reason)) > 2000 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Alasan pembebasan cicilan wajib diisi.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membebaskan cicilan.")
		return
	}
	defer tx.Rollback()
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	var status, outstandingRaw string
	err = tx.QueryRowContext(r.Context(), `SELECT l.status,CAST(l.outstanding AS CHAR)`+loanAdminFrom+` WHERE l.tenant_id=? AND l.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, id}, scopeArgs...)...).Scan(&status, &outstandingRaw)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kasbon tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kasbon.")
		return
	}
	if status != "disbursed" && status != "repaying" {
		writeAPIError(w, r, 409, "LOAN_NOT_ACTIVE", "Kasbon tidak memiliki kewajiban aktif.")
		return
	}
	var lineID, lineStatus, amountRaw string
	var payrollID sql.NullString
	err = tx.QueryRowContext(r.Context(), `SELECT id,status,CAST(amount AS CHAR),payroll_item_line_id FROM loan_installments WHERE tenant_id=? AND loan_id=? AND seq=? FOR UPDATE`, c.tenant.ID, id, seq).Scan(&lineID, &lineStatus, &amountRaw, &payrollID)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Cicilan tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca cicilan.")
		return
	}
	if lineStatus != "pending" {
		writeAPIError(w, r, 409, "INSTALLMENT_NOT_PENDING", "Cicilan sudah diproses.")
		return
	}
	if payrollID.Valid {
		writeAPIError(w, r, 409, "PAYROLL_LOCKED", "Cicilan sudah terhubung ke payroll.")
		return
	}
	amount, validA := leaveHundredths(amountRaw)
	outstanding, validO := leaveHundredths(outstandingRaw)
	if !validA || !validO || amount < 1 || outstanding < amount {
		writeAPIError(w, r, 409, "OUTSTANDING_CONFLICT", "Sisa kasbon tidak sesuai cicilan.")
		return
	}
	next := outstanding - amount
	if _, err = tx.ExecContext(r.Context(), `UPDATE loan_installments SET status='waived',waived_at=UTC_TIMESTAMP(6),waive_reason=?,waived_by=? WHERE tenant_id=? AND id=? AND status='pending'`, body.Reason, c.state.User.ID, c.tenant.ID, lineID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membebaskan cicilan.")
		return
	}
	nextStatus := status
	if next == 0 {
		nextStatus = "settled"
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE employee_loans SET outstanding=?,status=?,closed_at=IF(?=0,UTC_TIMESTAMP(6),NULL) WHERE tenant_id=? AND id=?`, leaveDecimal(next), nextStatus, next, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui sisa kasbon.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "waive", "loan_installments", lineID, map[string]any{"status": "pending", "outstanding": outstandingRaw}, map[string]any{"status": "waived", "amount": amountRaw, "outstanding": leaveDecimal(next), "reason": body.Reason}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat pembebasan cicilan.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pembebasan cicilan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"loan_id": id, "seq": seq, "status": "waived", "amount": amountRaw, "outstanding": leaveDecimal(next), "loan_status": nextStatus}, nil)
}

func (a adminAPI) settleAdminLoan(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loanManager(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kasbon tidak ditemukan.")
		return
	}
	var body struct {
		SettlementRef string `json:"settlement_ref"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if !validPaymentRef(body.SettlementRef) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Referensi pelunasan tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal melunasi kasbon.")
		return
	}
	defer tx.Rollback()
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	var status, outstandingRaw string
	err = tx.QueryRowContext(r.Context(), `SELECT l.status,CAST(l.outstanding AS CHAR)`+loanAdminFrom+` WHERE l.tenant_id=? AND l.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, id}, scopeArgs...)...).Scan(&status, &outstandingRaw)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kasbon tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kasbon.")
		return
	}
	if status == "settled" {
		var oldRef, oldAmount string
		err = tx.QueryRowContext(r.Context(), `SELECT settlement_ref,CAST(amount AS CHAR) FROM loan_settlements WHERE tenant_id=? AND loan_id=?`, c.tenant.ID, id).Scan(&oldRef, &oldAmount)
		if err == nil && oldRef == body.SettlementRef {
			writeAPIData(w, 200, map[string]any{"id": id, "status": "settled", "settlement_ref": oldRef, "amount": oldAmount, "idempotent": true}, nil)
			return
		}
		writeAPIError(w, r, 409, "ALREADY_SETTLED", "Kasbon sudah ditutup dengan cara lain.")
		return
	}
	if status != "disbursed" && status != "repaying" {
		writeAPIError(w, r, 409, "LOAN_NOT_ACTIVE", "Kasbon belum dicairkan.")
		return
	}
	outstanding, valid := leaveHundredths(outstandingRaw)
	if !valid || outstanding <= 0 {
		writeAPIError(w, r, 409, "OUTSTANDING_CONFLICT", "Sisa kasbon tidak valid.")
		return
	}
	var pending, locked int
	var pendingRaw string
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(payroll_item_line_id IS NOT NULL),0),CAST(COALESCE(SUM(amount),0) AS CHAR) FROM loan_installments WHERE tenant_id=? AND loan_id=? AND status='pending'`, c.tenant.ID, id).Scan(&pending, &locked, &pendingRaw); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa cicilan.")
		return
	}
	if pending == 0 {
		writeAPIError(w, r, 409, "INSTALLMENTS_CONFLICT", "Tidak ada cicilan pending.")
		return
	}
	if locked != 0 {
		writeAPIError(w, r, 409, "PAYROLL_LOCKED", "Ada cicilan yang sudah terhubung ke payroll.")
		return
	}
	pendingAmount, validPending := leaveHundredths(pendingRaw)
	if !validPending || pendingAmount != outstanding {
		writeAPIError(w, r, 409, "OUTSTANDING_CONFLICT", "Jumlah cicilan pending tidak sama dengan sisa kasbon.")
		return
	}
	var used int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM loan_settlements WHERE tenant_id=? AND settlement_ref=?`, c.tenant.ID, body.SettlementRef).Scan(&used); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa referensi pelunasan.")
		return
	}
	if used != 0 {
		writeAPIError(w, r, 409, "SETTLEMENT_REF_EXISTS", "Referensi pelunasan sudah dipakai.")
		return
	}
	settlementID, e := ids.New()
	if e != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID pelunasan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO loan_settlements (tenant_id,id,loan_id,amount,settlement_ref,recorded_by) VALUES (?,?,?,?,?,?)`, c.tenant.ID, settlementID, id, outstandingRaw, body.SettlementRef, c.state.User.ID); err != nil {
		writeAPIError(w, r, 409, "SETTLEMENT_REF_EXISTS", "Referensi pelunasan sudah dipakai.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE loan_installments SET status='settled' WHERE tenant_id=? AND loan_id=? AND status='pending'`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup cicilan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE employee_loans SET outstanding=0,status='settled',closed_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup kasbon.")
		return
	}
	payload, _ := json.Marshal(map[string]any{"loan_id": id, "settlement_id": settlementID, "amount": outstandingRaw})
	outboxID, e := ids.New()
	if e != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'loan.settled','loan',?,?,?)`, c.tenant.ID, outboxID, id, string(payload), "loan.settled:"+id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "settle", "employee_loans", id, map[string]any{"status": status, "outstanding": outstandingRaw}, map[string]any{"status": "settled", "outstanding": "0.00", "settlement_ref": body.SettlementRef, "amount": outstandingRaw}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat pelunasan.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pelunasan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "status": "settled", "settlement_id": settlementID, "settlement_ref": body.SettlementRef, "amount": outstandingRaw, "outstanding": "0.00", "installments_closed": pending, "idempotent": false}, nil)
}
