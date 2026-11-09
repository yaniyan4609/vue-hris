package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type disciplinaryInput struct {
	EmployeeID  string  `json:"employee_id"`
	Severity    string  `json:"severity"`
	IssuedDate  string  `json:"issued_date"`
	ValidUntil  *string `json:"valid_until"`
	Description string  `json:"description"`
	FileID      *string `json:"file_id"`
	Version     uint64  `json:"version,omitempty"`
}

func validDisciplinarySeverity(value string) bool {
	return value == "KONSELING" || value == "SP1" || value == "SP2" || value == "SP3"
}

func normalizeDisciplinaryInput(in *disciplinaryInput, zone string) bool {
	in.Severity = strings.ToUpper(strings.TrimSpace(in.Severity))
	in.Description = strings.TrimSpace(in.Description)
	if !validDisciplinarySeverity(in.Severity) || !validChangeDate(in.IssuedDate) || in.IssuedDate > tenantToday(zone) || len([]rune(in.Description)) < 10 || len([]rune(in.Description)) > 5000 {
		return false
	}
	if in.ValidUntil != nil && (!validChangeDate(*in.ValidUntil) || *in.ValidUntil < in.IssuedDate) {
		return false
	}
	if in.Severity != "KONSELING" && in.ValidUntil == nil {
		return false
	}
	if in.FileID != nil && !fileIDPattern.MatchString(*in.FileID) {
		return false
	}
	return true
}

func disciplineFileReady(ctx context.Context, tx *sql.Tx, tenantID string, fileID *string) (bool, error) {
	if fileID == nil {
		return true, nil
	}
	var found string
	err := tx.QueryRowContext(ctx, `SELECT id FROM files WHERE tenant_id=? AND id=? AND purpose='discipline' AND scan_status='clean' AND deleted_at IS NULL FOR UPDATE`, tenantID, *fileID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func previousDisciplineLevel(severity string) string {
	if severity == "SP2" {
		return "SP1"
	}
	if severity == "SP3" {
		return "SP2"
	}
	return ""
}
func nextDisciplineLevel(severity string) string {
	if severity == "SP1" {
		return "SP2"
	}
	if severity == "SP2" {
		return "SP3"
	}
	return ""
}

func checkDisciplineSequence(ctx context.Context, tx *sql.Tx, tenantID, employeeID, severity, issuedDate string, validUntil *string, excludeID string) (string, error) {
	if severity == "KONSELING" {
		return "", nil
	}
	if prior := previousDisciplineLevel(severity); prior != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM disciplinary_actions WHERE tenant_id=? AND employee_id=? AND severity=? AND status='active' AND issued_date<=? AND valid_until>=?`, tenantID, employeeID, prior, issuedDate, issuedDate).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			return "PRIOR_WARNING_REQUIRED", nil
		}
	}
	var overlap int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM disciplinary_actions WHERE tenant_id=? AND employee_id=? AND severity=? AND status='active' AND id<>? AND issued_date<=? AND valid_until>=?`, tenantID, employeeID, severity, excludeID, *validUntil, issuedDate).Scan(&overlap); err != nil {
		return "", err
	}
	if overlap != 0 {
		return "WARNING_OVERLAP", nil
	}
	return "", nil
}

func disciplineSuccessorConflict(ctx context.Context, tx *sql.Tx, tenantID, employeeID, severity, oldFrom string, oldUntil *string, newFrom string, newUntil *string) (bool, error) {
	next := nextDisciplineLevel(severity)
	if next == "" || oldUntil == nil {
		return false, nil
	}
	var count int
	where := `tenant_id=? AND employee_id=? AND severity=? AND status='active' AND issued_date BETWEEN ? AND ?`
	args := []any{tenantID, employeeID, next, oldFrom, *oldUntil}
	if newUntil != nil {
		where += ` AND (issued_date<? OR issued_date>?)`
		args = append(args, newFrom, *newUntil)
	}
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM disciplinary_actions WHERE `+where, args...).Scan(&count)
	return count != 0, err
}

// Serialize changes to one employee's SP chain before locking an individual action.
// Creation uses the same employee row lock, so sequence checks see a stable chain.
func lockDisciplineEmployee(ctx context.Context, tx *sql.Tx, tenantID, actionID, scope string, scopeArgs []any) error {
	params := append([]any{tenantID, actionID}, scopeArgs...)
	var employeeID string
	if err := tx.QueryRowContext(ctx, `SELECT d.employee_id`+disciplinaryJoin+` WHERE d.tenant_id=? AND d.id=? AND `+scope, params...).Scan(&employeeID); err != nil {
		return err
	}
	return tx.QueryRowContext(ctx, `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, employeeID).Scan(&employeeID)
}

func (a adminAPI) createDisciplinaryAction(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "disiplin.buat")
	if !ok {
		return
	}
	var in disciplinaryInput
	if !readJSON(w, r, &in) {
		return
	}
	if !fileIDPattern.MatchString(in.EmployeeID) || !normalizeDisciplinaryInput(&in, c.tenant.Timezone) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tingkat, tanggal, uraian, atau lampiran tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menerbitkan tindakan disiplin.")
		return
	}
	defer tx.Rollback()
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, in.EmployeeID}, args...)
	var employeeStatus string
	var deleted sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT e.status,e.deleted_at FROM employees e`+employeeScopeJoin+` WHERE e.tenant_id=? AND e.id=? AND `+scope+` FOR UPDATE`, params...).Scan(&employeeStatus, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
		return
	}
	if deleted.Valid || employeeStatus == "resigned" || employeeStatus == "terminated" {
		writeAPIError(w, r, 409, "EMPLOYEE_INACTIVE", "Karyawan tidak aktif.")
		return
	}
	ready, err := disciplineFileReady(r.Context(), tx, c.tenant.ID, in.FileID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa lampiran.")
		return
	}
	if !ready {
		writeAPIError(w, r, 404, "FILE_NOT_READY", "Lampiran disiplin tidak tersedia atau belum aman.")
		return
	}
	issue, err := checkDisciplineSequence(r.Context(), tx, c.tenant.ID, in.EmployeeID, in.Severity, in.IssuedDate, in.ValidUntil, "")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa tahapan SP.")
		return
	}
	if issue != "" {
		writeAPIError(w, r, 409, issue, "Tahapan SP harus berurutan dan tidak boleh tumpang tindih.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID tindakan disiplin.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO disciplinary_actions (tenant_id,id,employee_id,severity,issued_date,valid_until,description,file_id,created_by) VALUES (?,?,?,?,?,?,?,?,?)`, c.tenant.ID, id, in.EmployeeID, in.Severity, in.IssuedDate, in.ValidUntil, in.Description, in.FileID, c.state.User.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan tindakan disiplin.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "issue", "disciplinary_actions", id, nil, map[string]any{"employee_id": in.EmployeeID, "severity": in.Severity, "issued_date": in.IssuedDate, "valid_until": in.ValidUntil, "file_id": in.FileID, "version": 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menerbitkan tindakan disiplin.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "employee_id": in.EmployeeID, "severity": in.Severity, "status": "active", "version": 1}, nil)
}

func (a adminAPI) patchDisciplinaryAction(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "disiplin.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Tindakan disiplin tidak ditemukan.")
		return
	}
	var body struct {
		IssuedDate  string  `json:"issued_date"`
		ValidUntil  *string `json:"valid_until"`
		Description string  `json:"description"`
		FileID      *string `json:"file_id"`
		Version     uint64  `json:"version"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	in := disciplinaryInput{IssuedDate: body.IssuedDate, ValidUntil: body.ValidUntil, Description: body.Description, FileID: body.FileID, Version: body.Version}
	if in.Version == 0 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi wajib diisi.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah tindakan disiplin.")
		return
	}
	defer tx.Rollback()
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	if err = lockDisciplineEmployee(r.Context(), tx, c.tenant.ID, id, scope, args); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Tindakan disiplin tidak ditemukan dalam scope.")
		} else {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci karyawan.")
		}
		return
	}
	params := append([]any{c.tenant.ID, id}, args...)
	old, err := scanDisciplinaryAction(tx.QueryRowContext(r.Context(), `SELECT `+disciplinaryColumns+disciplinaryJoin+` WHERE d.tenant_id=? AND d.id=? AND `+scope+` FOR UPDATE`, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Tindakan disiplin tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tindakan disiplin.")
		return
	}
	if old.Status != "active" {
		writeAPIError(w, r, 409, "ACTION_CANCELLED", "Tindakan disiplin sudah dibatalkan.")
		return
	}
	if old.Version != in.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Tindakan disiplin sudah berubah.")
		return
	}
	in.Severity = old.Severity
	if !normalizeDisciplinaryInput(&in, c.tenant.Timezone) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal, uraian, atau lampiran tidak valid.")
		return
	}
	ready, err := disciplineFileReady(r.Context(), tx, c.tenant.ID, in.FileID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa lampiran.")
		return
	}
	if !ready {
		writeAPIError(w, r, 404, "FILE_NOT_READY", "Lampiran disiplin tidak tersedia atau belum aman.")
		return
	}
	issue, err := checkDisciplineSequence(r.Context(), tx, c.tenant.ID, old.EmployeeID, old.Severity, in.IssuedDate, in.ValidUntil, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa tahapan SP.")
		return
	}
	if issue != "" {
		writeAPIError(w, r, 409, issue, "Tahapan SP harus berurutan dan tidak boleh tumpang tindih.")
		return
	}
	orphan, err := disciplineSuccessorConflict(r.Context(), tx, c.tenant.ID, old.EmployeeID, old.Severity, old.IssuedDate, old.ValidUntil, in.IssuedDate, in.ValidUntil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa SP berikutnya.")
		return
	}
	if orphan {
		writeAPIError(w, r, 409, "SUCCESSOR_ACTIVE", "Perubahan ini akan memutus tahapan SP aktif.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE disciplinary_actions SET issued_date=?,valid_until=?,description=?,file_id=?,version=version+1 WHERE tenant_id=? AND id=? AND status='active' AND version=?`, in.IssuedDate, in.ValidUntil, in.Description, in.FileID, c.tenant.ID, id, in.Version)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah tindakan disiplin.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "disciplinary_actions", id, map[string]any{"version": old.Version, "issued_date": old.IssuedDate, "valid_until": old.ValidUntil, "file_id": old.FileID}, map[string]any{"version": old.Version + 1, "issued_date": in.IssuedDate, "valid_until": in.ValidUntil, "file_id": in.FileID}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah tindakan disiplin.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "status": "active", "version": old.Version + 1}, nil)
}

func (a adminAPI) cancelDisciplinaryAction(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "disiplin.hapus")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Tindakan disiplin tidak ditemukan.")
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum membatalkan tindakan disiplin.")
		return
	}
	var body struct {
		Reason  string `json:"reason"`
		Version uint64 `json:"version"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data pembatalan tidak valid.")
		return
	}
	if err == nil {
		if e := decoder.Decode(&struct{}{}); !errors.Is(e, io.EOF) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data pembatalan tidak valid.")
			return
		}
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if body.Reason != "" && (len([]rune(body.Reason)) < 10 || len([]rune(body.Reason)) > 500) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Alasan pembatalan harus 10–500 karakter.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan tindakan disiplin.")
		return
	}
	defer tx.Rollback()
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	if err = lockDisciplineEmployee(r.Context(), tx, c.tenant.ID, id, scope, args); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Tindakan disiplin tidak ditemukan dalam scope.")
		} else {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci karyawan.")
		}
		return
	}
	params := append([]any{c.tenant.ID, id}, args...)
	old, err := scanDisciplinaryAction(tx.QueryRowContext(r.Context(), `SELECT `+disciplinaryColumns+disciplinaryJoin+` WHERE d.tenant_id=? AND d.id=? AND `+scope+` FOR UPDATE`, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Tindakan disiplin tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tindakan disiplin.")
		return
	}
	if old.Status != "active" {
		writeAPIError(w, r, 409, "ACTION_CANCELLED", "Tindakan disiplin sudah dibatalkan.")
		return
	}
	if body.Version != 0 && body.Version != old.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Tindakan disiplin sudah berubah.")
		return
	}
	orphan, err := disciplineSuccessorConflict(r.Context(), tx, c.tenant.ID, old.EmployeeID, old.Severity, old.IssuedDate, old.ValidUntil, "", nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa SP berikutnya.")
		return
	}
	if orphan {
		writeAPIError(w, r, 409, "SUCCESSOR_ACTIVE", "SP berikutnya masih bergantung pada tindakan ini.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE disciplinary_actions SET status='cancelled',cancelled_by=?,cancelled_at=UTC_TIMESTAMP(6),cancellation_reason=?,version=version+1 WHERE tenant_id=? AND id=? AND status='active'`, c.state.User.ID, nullableString(body.Reason), c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan tindakan disiplin.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "cancel", "disciplinary_actions", id, map[string]any{"status": "active", "version": old.Version}, map[string]any{"status": "cancelled", "version": old.Version + 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membatalkan tindakan disiplin.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
