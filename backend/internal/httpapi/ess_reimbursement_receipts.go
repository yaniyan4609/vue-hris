package httpapi

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) addEssReimbursementReceipts(w http.ResponseWriter, r *http.Request) {
	c, ok := a.receiptUploader(w, r)
	if !ok {
		return
	}
	claimID := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(claimID) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Klaim tidak ditemukan.")
		return
	}
	var body struct {
		FileIDs []string `json:"file_ids"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if len(body.FileIDs) < 1 || len(body.FileIDs) > 5 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Kirim satu hingga lima kuitansi.")
		return
	}
	fileIDs := append([]string(nil), body.FileIDs...)
	sort.Strings(fileIDs)
	for i, id := range fileIDs {
		if !fileIDPattern.MatchString(id) || (i > 0 && id == fileIDs[i-1]) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID kuitansi tidak valid atau ganda.")
			return
		}
	}
	tenantID, userID := c.state.SelectedTenantID, c.state.User.ID
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menambah kuitansi.")
		return
	}
	defer tx.Rollback()
	var requestID string
	err = tx.QueryRowContext(r.Context(), `SELECT request_id FROM reimbursements WHERE tenant_id=? AND id=? AND employee_id=?`, tenantID, claimID, c.employeeID).Scan(&requestID)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Klaim tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca klaim.")
		return
	}
	var requestStatus string
	if err = tx.QueryRowContext(r.Context(), `SELECT status FROM requests WHERE tenant_id=? AND id=? AND request_type='reimbursement' AND ref_table='reimbursements' AND ref_id=? FOR UPDATE`, tenantID, requestID, claimID).Scan(&requestStatus); err != nil || requestStatus != "returned" {
		writeAPIError(w, r, 409, "REQUEST_INVALID", "Status pengajuan tidak sesuai klaim.")
		return
	}
	var employeeID, status, amount string
	var date time.Time
	err = tx.QueryRowContext(r.Context(), `SELECT employee_id,status,trx_date,CAST(amount AS CHAR) FROM reimbursements WHERE tenant_id=? AND id=? AND employee_id=? AND request_id=? FOR UPDATE`, tenantID, claimID, c.employeeID, requestID).Scan(&employeeID, &status, &date, &amount)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Klaim tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca klaim.")
		return
	}
	if status != "returned" {
		writeAPIError(w, r, 409, "CLAIM_NOT_RETURNED", "Kuitansi hanya dapat ditambah pada klaim yang dikembalikan.")
		return
	}
	var existing int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM reimbursement_receipts WHERE tenant_id=? AND reimbursement_id=?`, tenantID, claimID).Scan(&existing); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung kuitansi.")
		return
	}
	if existing+len(fileIDs) > 5 {
		writeAPIError(w, r, 409, "RECEIPT_LIMIT", "Maksimal lima kuitansi per klaim.")
		return
	}
	for _, fileID := range fileIDs {
		var purpose, scan string
		var hash []byte
		err = tx.QueryRowContext(r.Context(), `SELECT purpose,scan_status,sha256 FROM files WHERE tenant_id=? AND id=? AND uploaded_by=? AND deleted_at IS NULL FOR UPDATE`, tenantID, fileID, userID).Scan(&purpose, &scan, &hash)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "FILE_NOT_FOUND", "Kuitansi tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kuitansi.")
			return
		}
		if purpose != "receipt" || scan != "clean" {
			writeAPIError(w, r, 409, "FILE_NOT_READY", "Kuitansi belum siap dipakai.")
			return
		}
		var used int
		if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM reimbursement_receipts WHERE tenant_id=? AND file_id=?`, tenantID, fileID).Scan(&used); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa tautan kuitansi.")
			return
		}
		if used > 0 {
			writeAPIError(w, r, 409, "FILE_ALREADY_ATTACHED", "Kuitansi sudah dipakai pada klaim lain.")
			return
		}
		receiptID, e := ids.New()
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat tautan kuitansi.")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO reimbursement_receipts (tenant_id,id,reimbursement_id,file_id) VALUES (?,?,?,?)`, tenantID, receiptID, claimID, fileID); err != nil {
			writeAPIError(w, r, 409, "FILE_ALREADY_ATTACHED", "Kuitansi sudah dipakai pada klaim lain.")
			return
		}
	}
	rows, err := tx.QueryContext(r.Context(), `SELECT f.sha256 FROM reimbursement_receipts rr JOIN files f ON f.tenant_id=rr.tenant_id AND f.id=rr.file_id WHERE rr.tenant_id=? AND rr.reimbursement_id=? ORDER BY f.id`, tenantID, claimID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hash kuitansi.")
		return
	}
	hashes := []string{}
	for rows.Next() {
		var hash []byte
		if err = rows.Scan(&hash); err != nil {
			break
		}
		hashes = append(hashes, hex.EncodeToString(hash))
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hash kuitansi.")
		return
	}
	digest := reimbursementDuplicateHash(tenantID, employeeID, date.Format("2006-01-02"), amount, hashes)
	if _, err = tx.ExecContext(r.Context(), `UPDATE reimbursements SET duplicate_hash=? WHERE tenant_id=? AND id=?`, digest[:], tenantID, claimID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui klaim.")
		return
	}
	if err = audit(r.Context(), tx, tenantID, userID, "add_receipts", "reimbursements", claimID, map[string]any{"receipt_count": existing}, map[string]any{"receipt_count": existing + len(fileIDs)}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat kuitansi.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kuitansi.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": claimID, "request_id": requestID, "receipt_count": existing + len(fileIDs), "status": "returned"}, nil)
}
