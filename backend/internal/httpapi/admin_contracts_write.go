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

func validContract(item contract) bool {
	if item.ContractType != "PKWT" && item.ContractType != "PKWTT" && item.ContractType != "PROBASI" {
		return false
	}
	start, err := time.Parse("2006-01-02", item.StartDate)
	if err != nil || start.Format("2006-01-02") != item.StartDate {
		return false
	}
	if item.EndDate == nil {
		return item.ContractType == "PKWTT"
	}
	end, err := time.Parse("2006-01-02", *item.EndDate)
	return err == nil && end.Format("2006-01-02") == *item.EndDate && !end.Before(start)
}

func (a adminAPI) contractFileAllowed(r *http.Request, tenantID string, fileID *string) (bool, error) {
	if fileID == nil {
		return true, nil
	}
	if len(*fileID) != 26 {
		return false, nil
	}
	var count int
	err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM files WHERE tenant_id=? AND id=? AND purpose='contract' AND scan_status='clean' AND deleted_at IS NULL`, tenantID, *fileID).Scan(&count)
	return count == 1, err
}

func (a adminAPI) contractEmployeeAllowed(r *http.Request, c adminContext, employeeID string) (bool, error) {
	if len(employeeID) != 26 {
		return false, sql.ErrNoRows
	}
	e, err := scanEmployee(a.db.QueryRowContext(r.Context(), employeeSelect+` WHERE e.tenant_id=? AND e.id=?`, c.tenant.ID, employeeID))
	if err != nil {
		return false, err
	}
	target, err := a.targetFor(r.Context(), c.tenant.ID, e)
	if err != nil {
		return false, err
	}
	return c.effective.Includes(c.subject, target), nil
}

func (a adminAPI) createContract(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kontrak.buat")
	if !ok {
		return
	}
	var body struct {
		EmployeeID   string  `json:"employee_id"`
		ContractType string  `json:"contract_type"`
		StartDate    string  `json:"start_date"`
		EndDate      *string `json:"end_date"`
		FileID       *string `json:"file_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	item := contract{EmployeeID: body.EmployeeID, ContractType: body.ContractType, StartDate: body.StartDate, EndDate: body.EndDate, Status: "draft", FileID: body.FileID}
	if !validContract(item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jenis atau tanggal kontrak tidak valid.")
		return
	}
	allowed, err := a.contractEmployeeAllowed(r, c, item.EmployeeID)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam tenant.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return
	}
	if !allowed {
		writeAPIError(w, r, 403, "SCOPE_DENIED", "Karyawan di luar scope.")
		return
	}
	fileAllowed, err := a.contractFileAllowed(r, c.tenant.ID, item.FileID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa berkas.")
		return
	}
	if !fileAllowed {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas kontrak belum tersedia atau belum aman.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat kontrak.")
		return
	}
	item.ID = id
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontrak.")
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO contracts (tenant_id,id,employee_id,contract_type,start_date,end_date,status,file_id) VALUES (?,?,?,?,?,?,'draft',?)`, c.tenant.ID, id, item.EmployeeID, item.ContractType, item.StartDate, item.EndDate, item.FileID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontrak.")
		return
	}
	if err := tx.QueryRowContext(r.Context(), `SELECT created_at FROM contracts WHERE tenant_id=? AND id=?`, c.tenant.ID, id).Scan(&item.CreatedAt); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontrak.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "contracts", id, nil, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontrak.")
		return
	}
	writeAPIData(w, 201, item, nil)
}

func contractPatchText(raw json.RawMessage) (*string, error) {
	if raw == nil || string(raw) == "null" {
		return nil, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

func (a adminAPI) updateContract(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kontrak.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontrak tidak ditemukan.")
		return
	}
	var body struct {
		ContractType *string         `json:"contract_type"`
		StartDate    *string         `json:"start_date"`
		EndDate      json.RawMessage `json:"end_date"`
		Status       *string         `json:"status"`
		FileID       json.RawMessage `json:"file_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.ContractType == nil && body.StartDate == nil && body.EndDate == nil && body.Status == nil && body.FileID == nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tidak ada perubahan.")
		return
	}
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, id}, args...)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontrak.")
		return
	}
	defer tx.Rollback()
	old, err := scanContract(tx.QueryRowContext(r.Context(), `SELECT `+contractColumns+contractJoin+` WHERE c.tenant_id=? AND c.id=? AND `+filter+` FOR UPDATE`, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontrak tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontrak.")
		return
	}
	if old.Status != "draft" {
		writeAPIError(w, r, 409, "CONTRACT_LOCKED", "Hanya kontrak draft dapat diubah.")
		return
	}
	item := old
	if body.ContractType != nil {
		item.ContractType = *body.ContractType
	}
	if body.StartDate != nil {
		item.StartDate = *body.StartDate
	}
	if body.EndDate != nil {
		item.EndDate, err = contractPatchText(body.EndDate)
		if err != nil {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal akhir tidak valid.")
			return
		}
	}
	if body.FileID != nil {
		item.FileID, err = contractPatchText(body.FileID)
		if err != nil {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID berkas tidak valid.")
			return
		}
	}
	if body.Status != nil {
		if *body.Status != "draft" && *body.Status != "active" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status kontrak tidak valid.")
			return
		}
		item.Status = *body.Status
	}
	if !validContract(item) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jenis atau tanggal kontrak tidak valid.")
		return
	}
	fileAllowed, err := a.contractFileAllowed(r, c.tenant.ID, item.FileID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa berkas.")
		return
	}
	if !fileAllowed {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas kontrak belum tersedia atau belum aman.")
		return
	}
	if item.Status == "active" {
		var locked string
		if err := tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, item.EmployeeID).Scan(&locked); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kontrak aktif.")
			return
		}
		var overlapping int
		if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM contracts WHERE tenant_id=? AND employee_id=? AND id<>? AND status='active' AND (end_date IS NULL OR end_date>=?) AND (? IS NULL OR start_date<=?)`, c.tenant.ID, item.EmployeeID, id, item.StartDate, item.EndDate, item.EndDate).Scan(&overlapping); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kontrak aktif.")
			return
		}
		if overlapping > 0 {
			writeAPIError(w, r, 409, "DATE_OVERLAP", "Kontrak aktif karyawan bertumpang tindih.")
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE contracts SET contract_type=?,start_date=?,end_date=?,status=?,file_id=? WHERE tenant_id=? AND id=? AND status='draft'`, item.ContractType, item.StartDate, item.EndDate, item.Status, item.FileID, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontrak.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "contracts", id, old, item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan kontrak.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) deleteContract(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kontrak.hapus")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontrak tidak ditemukan.")
		return
	}
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, id}, args...)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus kontrak.")
		return
	}
	defer tx.Rollback()
	old, err := scanContract(tx.QueryRowContext(r.Context(), `SELECT `+contractColumns+contractJoin+` WHERE c.tenant_id=? AND c.id=? AND `+filter+` FOR UPDATE`, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontrak tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontrak.")
		return
	}
	if old.Status != "draft" {
		writeAPIError(w, r, 409, "CONTRACT_LOCKED", "Hanya kontrak draft dapat dihapus.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM contracts WHERE tenant_id=? AND id=? AND status='draft'`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus kontrak.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "delete", "contracts", id, old, nil); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus kontrak.")
		return
	}
	w.WriteHeader(204)
}

func (a adminAPI) renewContract(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kontrak.buat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontrak tidak ditemukan.")
		return
	}
	var body struct {
		ContractType string  `json:"contract_type"`
		StartDate    string  `json:"start_date"`
		EndDate      *string `json:"end_date"`
		FileID       *string `json:"file_id"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	newContract := contract{ContractType: body.ContractType, StartDate: body.StartDate, EndDate: body.EndDate, Status: "active", FileID: body.FileID}
	if !validContract(newContract) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jenis atau tanggal kontrak baru tidak valid.")
		return
	}
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	start, _ := time.Parse("2006-01-02", newContract.StartDate)
	if newContract.StartDate < time.Now().In(zone).Format("2006-01-02") {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Perpanjangan tidak boleh berlaku surut.")
		return
	}
	fileAllowed, err := a.contractFileAllowed(r, c.tenant.ID, newContract.FileID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa berkas.")
		return
	}
	if !fileAllowed {
		writeAPIError(w, r, 404, "NOT_FOUND", "Berkas kontrak belum tersedia atau belum aman.")
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, id}, scopeArgs...)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperpanjang kontrak.")
		return
	}
	defer tx.Rollback()
	old, err := scanContract(tx.QueryRowContext(r.Context(), `SELECT `+contractColumns+contractJoin+` WHERE c.tenant_id=? AND c.id=? AND `+filter+` FOR UPDATE`, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontrak tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontrak.")
		return
	}
	if old.Status != "active" {
		writeAPIError(w, r, 409, "CONTRACT_NOT_ACTIVE", "Hanya kontrak aktif dapat diperpanjang.")
		return
	}
	oldStart, _ := time.Parse("2006-01-02", old.StartDate)
	if !start.After(oldStart) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal kontrak baru harus setelah awal kontrak lama.")
		return
	}
	if old.EndDate != nil {
		oldEnd, _ := time.Parse("2006-01-02", *old.EndDate)
		if start.After(oldEnd.AddDate(0, 0, 1)) {
			writeAPIError(w, r, 409, "CONTRACT_GAP", "Ada jeda setelah kontrak lama; buat kontrak baru.")
			return
		}
	}
	var locked string
	if err := tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, old.EmployeeID).Scan(&locked); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci karyawan.")
		return
	}
	var overlap int
	if err := tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM contracts WHERE tenant_id=? AND employee_id=? AND id<>? AND status='active' AND (end_date IS NULL OR end_date>=?) AND (? IS NULL OR start_date<=?)`, c.tenant.ID, old.EmployeeID, old.ID, newContract.StartDate, newContract.EndDate, newContract.EndDate).Scan(&overlap); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa kontrak.")
		return
	}
	if overlap != 0 {
		writeAPIError(w, r, 409, "DATE_OVERLAP", "Sudah ada kontrak aktif pada periode baru.")
		return
	}
	oldAfter := old
	endOld := start.AddDate(0, 0, -1).Format("2006-01-02")
	oldAfter.EndDate = &endOld
	if _, err := tx.ExecContext(r.Context(), `UPDATE contracts SET end_date=? WHERE tenant_id=? AND id=? AND status='active'`, endOld, c.tenant.ID, old.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup periode kontrak lama.")
		return
	}
	newID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat kontrak baru.")
		return
	}
	newContract.ID = newID
	newContract.EmployeeID = old.EmployeeID
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO contracts (tenant_id,id,employee_id,contract_type,start_date,end_date,status,file_id) VALUES (?,?,?,?,?,?,'active',?)`, c.tenant.ID, newID, newContract.EmployeeID, newContract.ContractType, newContract.StartDate, newContract.EndDate, newContract.FileID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat kontrak baru.")
		return
	}
	if err := tx.QueryRowContext(r.Context(), `SELECT created_at FROM contracts WHERE tenant_id=? AND id=?`, c.tenant.ID, newID).Scan(&newContract.CreatedAt); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontrak baru.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "renew_close", "contracts", old.ID, old, oldAfter); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "renew_create", "contracts", newID, nil, newContract); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperpanjang kontrak.")
		return
	}
	writeAPIData(w, 201, map[string]any{"old_contract": oldAfter, "new_contract": newContract}, nil)
}
