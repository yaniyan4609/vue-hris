package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) disburseAdminLoan(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kasbon.cairkan")
	if !ok {
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum mencairkan kasbon.")
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kasbon tidak ditemukan dalam scope.")
		return
	}
	var body struct {
		DisbursementRef string `json:"disbursement_ref"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if !validPaymentRef(body.DisbursementRef) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Referensi pencairan tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencairkan kasbon.")
		return
	}
	defer tx.Rollback()
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	args := append([]any{c.tenant.ID, id}, scopeArgs...)
	var requestID, ref sql.NullString
	var status, amountRaw, regularRaw string
	var tenor int
	err = tx.QueryRowContext(r.Context(), `SELECT l.request_id,l.disbursement_ref,l.status,CAST(l.amount AS CHAR),CAST(l.installment_amount AS CHAR),l.tenor_months`+loanAdminFrom+` WHERE l.tenant_id=? AND l.id=? AND `+scope+` FOR UPDATE`, args...).Scan(&requestID, &ref, &status, &amountRaw, &regularRaw, &tenor)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kasbon tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kasbon.")
		return
	}
	if status == "disbursed" || status == "repaying" || status == "settled" {
		if ref.Valid && ref.String == body.DisbursementRef {
			writeAPIData(w, 200, map[string]any{"id": id, "status": status, "disbursement_ref": ref.String, "idempotent": true}, nil)
			return
		}
		writeAPIError(w, r, 409, "ALREADY_DISBURSED", "Kasbon sudah dicairkan dengan referensi lain.")
		return
	}
	if status != "approved" || !requestID.Valid {
		writeAPIError(w, r, 409, "LOAN_NOT_APPROVED", "Hanya kasbon yang disetujui dapat dicairkan.")
		return
	}
	var requestStatus, kind, table, refID, instanceStatus string
	err = tx.QueryRowContext(r.Context(), `SELECT q.status,q.request_type,q.ref_table,q.ref_id,i.status FROM requests q JOIN approval_instances i ON i.tenant_id=q.tenant_id AND i.request_id=q.id WHERE q.tenant_id=? AND q.id=? FOR UPDATE`, c.tenant.ID, requestID.String).Scan(&requestStatus, &kind, &table, &refID, &instanceStatus)
	if err != nil || requestStatus != "approved" || instanceStatus != "approved" || kind != "loan" || table != "employee_loans" || refID != id {
		writeAPIError(w, r, 409, "APPROVAL_REQUIRED", "Proses approval kasbon belum lengkap.")
		return
	}
	amount, validAmount := leaveHundredths(amountRaw)
	regular, validRegular := leaveHundredths(regularRaw)
	if !validAmount || !validRegular || tenor < 1 || regular < 1 || amount < regular*int64(tenor) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Rencana cicilan tidak valid.")
		return
	}
	var used int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employee_loans WHERE tenant_id=? AND disbursement_ref=? AND id<>?`, c.tenant.ID, body.DisbursementRef, id).Scan(&used); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa referensi pencairan.")
		return
	}
	if used != 0 {
		writeAPIError(w, r, 409, "DISBURSEMENT_REF_EXISTS", "Referensi pencairan sudah dipakai.")
		return
	}
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	now := time.Now().In(zone)
	first := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, zone)
	result, err := tx.ExecContext(r.Context(), `UPDATE employee_loans SET status='disbursed',outstanding=amount,disbursed_at=UTC_TIMESTAMP(6),disbursement_ref=? WHERE tenant_id=? AND id=? AND status='approved'`, body.DisbursementRef, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 409, "DISBURSEMENT_REF_EXISTS", "Referensi pencairan sudah dipakai.")
		return
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		writeAPIError(w, r, 409, "LOAN_CHANGED", "Status kasbon sudah berubah.")
		return
	}
	for seq := 1; seq <= tenor; seq++ {
		lineAmount := regular
		if seq == tenor {
			lineAmount = amount - regular*int64(tenor-1)
		}
		lineID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID cicilan.")
			return
		}
		period := first.AddDate(0, seq-1, 0).Format("2006-01-02")
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO loan_installments (tenant_id,id,loan_id,seq,due_period,amount,status) VALUES (?,?,?,?,?,?,'pending')`, c.tenant.ID, lineID, id, seq, period, leaveDecimal(lineAmount)); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat cicilan.")
			return
		}
	}
	outboxID, e := ids.New()
	if e != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat notifikasi.")
		return
	}
	payload, _ := json.Marshal(map[string]any{"loan_id": id, "request_id": requestID.String, "disbursement_ref": body.DisbursementRef})
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'loan.disbursed','loan',?,?,?)`, c.tenant.ID, outboxID, id, string(payload), "loan.disbursed:"+id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan notifikasi.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "disburse", "employee_loans", id, map[string]any{"status": "approved", "outstanding": "0.00"}, map[string]any{"status": "disbursed", "outstanding": amountRaw, "disbursement_ref": body.DisbursementRef, "first_due_period": first.Format("2006-01-02")}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat pencairan.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan pencairan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "status": "disbursed", "outstanding": amountRaw, "disbursement_ref": body.DisbursementRef, "installment_count": tenor, "first_due_period": first.Format("2006-01-02"), "idempotent": false}, nil)
}
