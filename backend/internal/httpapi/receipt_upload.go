package httpapi

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

const maxReceiptBytes int64 = 10 << 20
const maxSelfieBytes int64 = 5 << 20
const stagingTenantStorageBytes int64 = 100 << 20

var fileIDPattern = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
var receiptMimes = map[string]bool{"application/pdf": true, "image/jpeg": true, "image/png": true}

func receiptStorageRoot() string {
	if os.Getenv("APP_ENV") != "staging" {
		return ""
	}
	return os.Getenv("FILE_STORAGE_DIR")
}

func receiptFilePath(root, tenantID, id string) string { return filepath.Join(root, tenantID, id) }

func validReceiptContent(content []byte, mime string) bool {
	if http.DetectContentType(content) != mime {
		return false
	}
	if mime == "application/pdf" {
		return bytes.HasPrefix(content, []byte("%PDF-")) && bytes.Contains(content, []byte("%%EOF"))
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(content))
	return err == nil && config.Width > 0 && config.Height > 0 && config.Width <= 10000 && config.Height <= 10000
}

func (a adminAPI) receiptUploader(w http.ResponseWriter, r *http.Request) (essAttendanceContext, bool) {
	return a.uploaderForPurpose(w, r, "receipt")
}

func (a adminAPI) uploaderForPurpose(w http.ResponseWriter, r *http.Request, purpose string) (essAttendanceContext, bool) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return c, false
	}
	if purpose == "receipt" {
		_, decision, err := a.access.Decide(r.Context(), c.state.SelectedTenantID, c.state.User.ID, c.zone.String(), "reimbursement.buat")
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak unggah kuitansi.")
			return c, false
		}
		if !decision.Allowed {
			writeAPIError(w, r, 403, "PERMISSION_DENIED", "Hak mengajukan reimbursement tidak tersedia.")
			return c, false
		}
	}
	if receiptStorageRoot() == "" {
		writeAPIError(w, r, 503, "STORAGE_UNAVAILABLE", "Penyimpanan kuitansi staging belum dikonfigurasi.")
		return c, false
	}
	return c, true
}

func (a adminAPI) existingUploadPurpose(w http.ResponseWriter, r *http.Request) (string, bool) {
	state, err := a.auth.Current(r.Context(), cookieToken(r))
	if err != nil {
		writeAuthError(w, err)
		return "", false
	}
	var purpose string
	err = a.db.QueryRowContext(r.Context(), `SELECT purpose FROM files WHERE tenant_id=? AND id=? AND uploaded_by=? AND deleted_at IS NULL AND purpose IN ('receipt','selfie')`, state.SelectedTenantID, chi.URLParam(r, "id"), state.User.ID).Scan(&purpose)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas tidak ditemukan.")
		return "", false
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca metadata berkas.")
		return "", false
	}
	return purpose, true
}

func (a adminAPI) createReceiptUpload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Purpose      string `json:"purpose"`
		OriginalName string `json:"original_name"`
		MimeType     string `json:"mime_type"`
		SizeBytes    int64  `json:"size_bytes"`
		SHA256       string `json:"sha256"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	c, ok := a.uploaderForPurpose(w, r, body.Purpose)
	if !ok {
		return
	}
	body.OriginalName = strings.TrimSpace(body.OriginalName)
	maxSize := maxReceiptBytes
	if body.Purpose == "selfie" {
		maxSize = maxSelfieBytes
	}
	if (body.Purpose != "receipt" && body.Purpose != "selfie") || !receiptMimes[body.MimeType] || (body.Purpose == "selfie" && body.MimeType == "application/pdf") || body.SizeBytes < 1 || body.SizeBytes > maxSize || body.OriginalName == "" || len([]rune(body.OriginalName)) > 255 || strings.ContainsAny(body.OriginalName, "\\/\x00\r\n") || len(body.SHA256) != 64 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Metadata kuitansi tidak valid.")
		return
	}
	hash, err := hex.DecodeString(body.SHA256)
	if err != nil || len(hash) != sha256.Size {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Hash kuitansi tidak valid.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID berkas.")
		return
	}
	storageKey := body.Purpose + "/" + c.state.SelectedTenantID + "/" + id
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan metadata kuitansi.")
		return
	}
	defer tx.Rollback()
	var lockedTenant string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM tenants WHERE id=? FOR UPDATE`, c.state.SelectedTenantID).Scan(&lockedTenant); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci kuota tenant.")
		return
	}
	var used int64
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(SUM(size_bytes),0) FROM files WHERE tenant_id=? AND deleted_at IS NULL`, c.state.SelectedTenantID).Scan(&used); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kuota penyimpanan.")
		return
	}
	if used+body.SizeBytes > stagingTenantStorageBytes {
		writeAPIError(w, r, 422, "STORAGE_QUOTA_EXCEEDED", "Kuota kuitansi staging terlampaui.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO files (tenant_id,id,storage_key,original_name,mime_type,size_bytes,sha256,purpose,uploaded_by,scan_status) VALUES (?,?,?,?,?,?,?,?,?,'pending')`, c.state.SelectedTenantID, id, storageKey, body.OriginalName, body.MimeType, body.SizeBytes, hash, body.Purpose, c.state.User.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan metadata kuitansi.")
		return
	}
	if err = audit(r.Context(), tx, c.state.SelectedTenantID, c.state.User.ID, "create", "files", id, nil, map[string]any{"purpose": body.Purpose, "size_bytes": body.SizeBytes}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat unggah kuitansi.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan metadata kuitansi.")
		return
	}
	writeAPIData(w, 201, map[string]any{"file_id": id, "upload_url": "/api/v1/files/" + id + "/content", "method": "PUT", "headers": map[string]string{"Content-Type": body.MimeType, "X-CSRF-Token": csrfToken(cookieToken(r))}, "expires_at": time.Now().UTC().Add(15 * time.Minute)}, nil)
}

func (a adminAPI) putReceiptContent(w http.ResponseWriter, r *http.Request) {
	purpose, ok := a.existingUploadPurpose(w, r)
	if !ok {
		return
	}
	c, ok := a.uploaderForPurpose(w, r, purpose)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas tidak ditemukan.")
		return
	}
	root := receiptStorageRoot()
	directory := filepath.Join(root, c.state.SelectedTenantID)
	if err := os.MkdirAll(directory, 0700); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyiapkan penyimpanan.")
		return
	}
	tmp, err := os.CreateTemp(directory, ".upload-*")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menerima berkas.")
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	maxSize := maxReceiptBytes
	if purpose == "selfie" {
		maxSize = maxSelfieBytes
	}
	length, err := io.Copy(tmp, http.MaxBytesReader(w, r.Body, maxSize+1))
	closeErr := tmp.Close()
	if err != nil || closeErr != nil || length < 1 || length > maxSize {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Isi kuitansi tidak valid atau terlalu besar.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan berkas.")
		return
	}
	defer tx.Rollback()
	var declared int64
	var mime, status string
	var created time.Time
	err = tx.QueryRowContext(r.Context(), `SELECT size_bytes,mime_type,scan_status,created_at FROM files WHERE tenant_id=? AND id=? AND uploaded_by=? AND purpose=? AND deleted_at IS NULL FOR UPDATE`, c.state.SelectedTenantID, id, c.state.User.ID, purpose).Scan(&declared, &mime, &status, &created)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca metadata berkas.")
		return
	}
	if status != "pending" || time.Since(created) > 15*time.Minute {
		writeAPIError(w, r, 409, "UPLOAD_EXPIRED", "Sesi unggah sudah selesai atau kedaluwarsa.")
		return
	}
	if length != declared || r.Header.Get("Content-Type") != mime {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Ukuran atau tipe kuitansi tidak sesuai metadata.")
		return
	}
	path := receiptFilePath(root, c.state.SelectedTenantID, id)
	if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengganti unggahan.")
		return
	}
	if err = os.Rename(tmpPath, path); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan berkas.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan berkas.")
		return
	}
	w.WriteHeader(204)
}

func (a adminAPI) completeReceiptUpload(w http.ResponseWriter, r *http.Request) {
	purpose, ok := a.existingUploadPurpose(w, r)
	if !ok {
		return
	}
	c, ok := a.uploaderForPurpose(w, r, purpose)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kuitansi.")
		return
	}
	defer tx.Rollback()
	var declared int64
	var expected []byte
	var mime, status string
	err = tx.QueryRowContext(r.Context(), `SELECT size_bytes,sha256,mime_type,scan_status FROM files WHERE tenant_id=? AND id=? AND uploaded_by=? AND purpose=? AND deleted_at IS NULL FOR UPDATE`, c.state.SelectedTenantID, id, c.state.User.ID, purpose).Scan(&declared, &expected, &mime, &status)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca metadata berkas.")
		return
	}
	if status == "clean" {
		writeAPIData(w, 200, map[string]any{"file_id": id, "scan_status": "clean"}, nil)
		return
	}
	if status != "pending" {
		writeAPIError(w, r, 409, "UPLOAD_INVALID", "Berkas tidak dapat dikonfirmasi.")
		return
	}
	content, err := os.ReadFile(receiptFilePath(receiptStorageRoot(), c.state.SelectedTenantID, id))
	if errors.Is(err, os.ErrNotExist) {
		writeAPIError(w, r, 409, "UPLOAD_INCOMPLETE", "Isi berkas belum diunggah.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca berkas.")
		return
	}
	hash := sha256.Sum256(content)
	if len(content) != int(declared) || !strings.EqualFold(hex.EncodeToString(hash[:]), hex.EncodeToString(expected)) || !validReceiptContent(content, mime) {
		writeAPIError(w, r, 422, "FILE_VERIFICATION_FAILED", "Ukuran, hash, atau tipe isi kuitansi tidak sesuai.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE files SET scan_status='clean' WHERE tenant_id=? AND id=? AND scan_status='pending'`, c.state.SelectedTenantID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengonfirmasi kuitansi.")
		return
	}
	if err = audit(r.Context(), tx, c.state.SelectedTenantID, c.state.User.ID, "complete", "files", id, map[string]any{"scan_status": "pending"}, map[string]any{"scan_status": "clean"}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat konfirmasi berkas.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengonfirmasi kuitansi.")
		return
	}
	writeAPIData(w, 200, map[string]any{"file_id": id, "scan_status": "clean"}, nil)
}
