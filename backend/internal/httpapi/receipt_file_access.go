package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"time"

	"digaji.id/hris/internal/auth"
	"github.com/go-chi/chi/v5"
)

type receiptFileView struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	Purpose      string    `json:"purpose"`
	ScanStatus   string    `json:"scan_status"`
	CreatedAt    time.Time `json:"created_at"`
	UploadedBy   string    `json:"-"`
	ClaimID      *string   `json:"reimbursement_id,omitempty"`
}

func (a adminAPI) authorizedReceiptFile(w http.ResponseWriter, r *http.Request) (receiptFileView, auth.State, bool) {
	var item receiptFileView
	state, err := a.auth.Current(r.Context(), cookieToken(r))
	if err != nil {
		writeAuthError(w, err)
		return item, state, false
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas tidak ditemukan.")
		return item, state, false
	}
	var uploader, claim sql.NullString
	err = a.db.QueryRowContext(r.Context(), `SELECT f.id,f.original_name,f.mime_type,f.size_bytes,f.purpose,f.scan_status,f.created_at,f.uploaded_by,rr.reimbursement_id FROM files f LEFT JOIN reimbursement_receipts rr ON rr.tenant_id=f.tenant_id AND rr.file_id=f.id WHERE f.tenant_id=? AND f.id=? AND f.purpose IN ('receipt','selfie') AND f.deleted_at IS NULL`, state.SelectedTenantID, id).Scan(&item.ID, &item.OriginalName, &item.MimeType, &item.SizeBytes, &item.Purpose, &item.ScanStatus, &item.CreatedAt, &uploader, &claim)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas tidak ditemukan.")
		return item, state, false
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca berkas.")
		return item, state, false
	}
	if uploader.Valid {
		item.UploadedBy = uploader.String
	}
	if claim.Valid {
		item.ClaimID = &claim.String
	}
	if uploader.Valid && uploader.String == state.User.ID {
		return item, state, true
	}
	if !claim.Valid && item.Purpose != "selfie" {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas tidak ditemukan.")
		return item, state, false
	}
	var tenant auth.Tenant
	for _, candidate := range state.Tenants {
		if candidate.ID == state.SelectedTenantID {
			tenant = candidate
			break
		}
	}
	if !tenant.Admin {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas tidak ditemukan.")
		return item, state, false
	}
	permission := "reimbursement.lihat"
	if item.Purpose == "selfie" {
		permission = "absensi.lihat"
	}
	subject, decision, err := a.access.Decide(r.Context(), tenant.ID, state.User.ID, tenant.Timezone, permission)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak berkas.")
		return item, state, false
	}
	if !decision.Allowed {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak melihat kuitansi tidak tersedia.")
		return item, state, false
	}
	var employeeID string
	if item.Purpose == "selfie" {
		err = a.db.QueryRowContext(r.Context(), `SELECT employee_id FROM attendance_events WHERE tenant_id=? AND selfie_file_id=? ORDER BY created_at DESC LIMIT 1`, tenant.ID, id).Scan(&employeeID)
	} else {
		err = a.db.QueryRowContext(r.Context(), `SELECT employee_id FROM reimbursements WHERE tenant_id=? AND id=?`, tenant.ID, claim.String).Scan(&employeeID)
	}
	if err != nil {
		writeAPIError(w, r, 404, "NOT_FOUND", "Klaim tidak ditemukan.")
		return item, state, false
	}
	employee, err := scanEmployee(a.db.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, tenant.ID, employeeID))
	if err != nil {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan.")
		return item, state, false
	}
	target, err := a.targetFor(r.Context(), tenant.ID, employee)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope kuitansi.")
		return item, state, false
	}
	if !decision.Includes(subject, target) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kuitansi tidak ditemukan dalam scope.")
		return item, state, false
	}
	return item, state, true
}

func (a adminAPI) getReceiptFile(w http.ResponseWriter, r *http.Request) {
	item, _, ok := a.authorizedReceiptFile(w, r)
	if !ok {
		return
	}
	writeAPIData(w, 200, item, nil)
}

func receiptDownloadSignature(session, id string, expires int64) string {
	mac := hmac.New(sha256.New, []byte(session))
	_, _ = fmt.Fprintf(mac, "digaji-file-v1\x00%s\x00%d", id, expires)
	return hex.EncodeToString(mac.Sum(nil))
}

func (a adminAPI) redirectReceiptDownload(w http.ResponseWriter, r *http.Request) {
	item, _, ok := a.authorizedReceiptFile(w, r)
	if !ok {
		return
	}
	if item.ScanStatus != "clean" || receiptStorageRoot() == "" {
		writeAPIError(w, r, 409, "FILE_NOT_READY", "Kuitansi belum dapat diunduh.")
		return
	}
	expires := time.Now().Add(5 * time.Minute).Unix()
	url := fmt.Sprintf("/api/v1/files/%s/content?expires=%d&signature=%s", item.ID, expires, receiptDownloadSignature(cookieToken(r), item.ID, expires))
	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, url, http.StatusFound)
}

func (a adminAPI) getReceiptContent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	expires, err := strconv.ParseInt(r.URL.Query().Get("expires"), 10, 64)
	if !fileIDPattern.MatchString(id) || err != nil || expires < time.Now().Unix() || expires > time.Now().Add(5*time.Minute).Unix() {
		writeAPIError(w, r, 403, "DOWNLOAD_EXPIRED", "Tautan unduh tidak valid atau kedaluwarsa.")
		return
	}
	provided, err := hex.DecodeString(r.URL.Query().Get("signature"))
	expected, _ := hex.DecodeString(receiptDownloadSignature(cookieToken(r), id, expires))
	if err != nil || len(provided) != len(expected) || !hmac.Equal(provided, expected) {
		writeAPIError(w, r, 403, "DOWNLOAD_INVALID", "Tautan unduh tidak sah.")
		return
	}
	item, state, ok := a.authorizedReceiptFile(w, r)
	if !ok {
		return
	}
	if item.ScanStatus != "clean" || receiptStorageRoot() == "" {
		writeAPIError(w, r, 409, "FILE_NOT_READY", "Kuitansi belum dapat diunduh.")
		return
	}
	file, err := os.Open(receiptFilePath(receiptStorageRoot(), state.SelectedTenantID, id))
	if err != nil {
		writeAPIError(w, r, 409, "FILE_UNAVAILABLE", "Isi kuitansi tidak tersedia.")
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || stat.Size() != item.SizeBytes {
		writeAPIError(w, r, 409, "FILE_UNAVAILABLE", "Isi kuitansi tidak sesuai metadata.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat unduhan.")
		return
	}
	defer tx.Rollback()
	if err = audit(r.Context(), tx, state.SelectedTenantID, state.User.ID, "download", "files", id, nil, map[string]any{"purpose": item.Purpose}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat unduhan.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat unduhan.")
		return
	}
	w.Header().Set("Content-Type", item.MimeType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": item.OriginalName}))
	w.Header().Set("Content-Length", strconv.FormatInt(item.SizeBytes, 10))
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(w, file)
}

func (a adminAPI) deleteReceiptFile(w http.ResponseWriter, r *http.Request) {
	if receiptStorageRoot() == "" {
		writeAPIError(w, r, 503, "STORAGE_UNAVAILABLE", "Penyimpanan kuitansi staging belum dikonfigurasi.")
		return
	}
	item, state, ok := a.authorizedReceiptFile(w, r)
	if !ok {
		return
	}
	if item.UploadedBy != state.User.ID {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hanya pengunggah yang dapat menghapus berkas bebas.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus berkas.")
		return
	}
	defer tx.Rollback()
	var bound int
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM files WHERE tenant_id=? AND id=? AND uploaded_by=? AND deleted_at IS NULL FOR UPDATE`, state.SelectedTenantID, item.ID, state.User.ID).Scan(&item.ID); err != nil {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas tidak ditemukan.")
		return
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM reimbursement_receipts WHERE tenant_id=? AND file_id=?`, state.SelectedTenantID, item.ID).Scan(&bound); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa tautan berkas.")
		return
	}
	if item.Purpose == "selfie" && bound == 0 {
		err = tx.QueryRowContext(r.Context(), `SELECT (SELECT COUNT(*) FROM attendance_events WHERE tenant_id=? AND selfie_file_id=?) + (SELECT COUNT(*) FROM attendance_corrections WHERE tenant_id=? AND attachment_file_id=?)`, state.SelectedTenantID, item.ID, state.SelectedTenantID, item.ID).Scan(&bound)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa tautan selfie.")
			return
		}
	}
	if bound != 0 {
		writeAPIError(w, r, 409, "FILE_ATTACHED", "Berkas yang terikat ke data tidak dapat dihapus.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE files SET deleted_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, state.SelectedTenantID, item.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus berkas.")
		return
	}
	if err = audit(r.Context(), tx, state.SelectedTenantID, state.User.ID, "delete", "files", item.ID, nil, map[string]any{"deleted": true}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat penghapusan.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus berkas.")
		return
	}
	_ = os.Remove(receiptFilePath(receiptStorageRoot(), state.SelectedTenantID, item.ID))
	w.WriteHeader(204)
}
