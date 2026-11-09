package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type terminationInput struct {
	EmployeeID    string  `json:"employee_id"`
	Type          string  `json:"termination_type"`
	NoticeDate    *string `json:"notice_date"`
	EffectiveDate string  `json:"effective_date"`
	Reason        string  `json:"reason"`
	Version       uint64  `json:"version,omitempty"`
}

func validTerminationInput(in *terminationInput, zone string) bool {
	in.Reason = strings.TrimSpace(in.Reason)
	if !validTerminationType(in.Type) || !validChangeDate(in.EffectiveDate) || in.EffectiveDate < tenantToday(zone) || len([]rune(in.Reason)) < 10 || len([]rune(in.Reason)) > 2000 {
		return false
	}
	if in.NoticeDate != nil && (!validChangeDate(*in.NoticeDate) || *in.NoticeDate > in.EffectiveDate) {
		return false
	}
	return true
}

func (a adminAPI) createTermination(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "terminasi.buat")
	if !ok {
		return
	}
	var in terminationInput
	if !readJSON(w, r, &in) {
		return
	}
	if !fileIDPattern.MatchString(in.EmployeeID) || !validTerminationInput(&in, c.tenant.Timezone) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jenis, tanggal, atau alasan tidak valid.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat terminasi.")
		return
	}
	defer tx.Rollback()
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	var status, sourceID string
	var deleted sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT e.status,e.deleted_at,a.id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE e.tenant_id=? AND e.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, in.EmployeeID}, args...)...).Scan(&status, &deleted, &sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca karyawan.")
		return
	}
	if deleted.Valid || status == "resigned" || status == "terminated" {
		writeAPIError(w, r, 409, "EMPLOYEE_INACTIVE", "Karyawan tidak aktif.")
		return
	}
	var count int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM terminations WHERE tenant_id=? AND employee_id=? AND status IN ('draft','in_review','returned','approved')`, c.tenant.ID, in.EmployeeID).Scan(&count); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa proses aktif.")
		return
	}
	if count > 0 {
		writeAPIError(w, r, 409, "TERMINATION_ACTIVE", "Karyawan sudah memiliki proses terminasi aktif.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO terminations (tenant_id,id,employee_id,termination_type,notice_date,effective_date,reason,source_assignment_id,created_by) VALUES (?,?,?,?,?,?,?,?,?)`, c.tenant.ID, id, in.EmployeeID, in.Type, in.NoticeDate, in.EffectiveDate, in.Reason, sourceID, c.state.User.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan terminasi.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO termination_checklist (tenant_id,termination_id,item_code) VALUES (?,?,'assets_returned'),(?,?,'handover_complete')`, c.tenant.ID, id, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat checklist.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "terminations", id, nil, map[string]any{"employee_id": in.EmployeeID, "termination_type": in.Type, "effective_date": in.EffectiveDate, "status": "draft", "version": 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat terminasi.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "status": "draft", "version": 1}, nil)
}

// Lock the employee first, shared by creation, correction, submission and cancellation.
func lockTerminationEmployee(r *http.Request, tx *sql.Tx, c adminContext, id string) (termination, error) {
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	var employeeID string
	err := tx.QueryRowContext(r.Context(), `SELECT x.employee_id`+terminationJoin+` WHERE x.tenant_id=? AND x.id=? AND `+scope, append([]any{c.tenant.ID, id}, args...)...).Scan(&employeeID)
	if err != nil {
		return termination{}, err
	}
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, employeeID).Scan(&employeeID); err != nil {
		return termination{}, err
	}
	return scanTermination(tx.QueryRowContext(r.Context(), `SELECT `+terminationColumns+terminationJoin+` WHERE x.tenant_id=? AND x.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, id}, args...)...))
}

func (a adminAPI) patchTermination(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "terminasi.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Terminasi tidak ditemukan.")
		return
	}
	var in terminationInput
	if !readJSON(w, r, &in) {
		return
	}
	if in.Version == 0 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi wajib diisi.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah terminasi.")
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
	if old.Status != "draft" && old.Status != "returned" {
		writeAPIError(w, r, 409, "TERMINATION_LOCKED", "Hanya draft atau pengajuan dikembalikan yang dapat dikoreksi.")
		return
	}
	if old.Version != in.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Terminasi sudah berubah.")
		return
	}
	in.Type = old.Type
	if !validTerminationInput(&in, c.tenant.Timezone) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal atau alasan tidak valid.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE terminations SET notice_date=?,effective_date=?,reason=?,version=version+1 WHERE tenant_id=? AND id=? AND version=?`, in.NoticeDate, in.EffectiveDate, in.Reason, c.tenant.ID, id, in.Version)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah terminasi.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "terminations", id, map[string]any{"effective_date": old.EffectiveDate, "version": old.Version}, map[string]any{"effective_date": in.EffectiveDate, "version": old.Version + 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah terminasi.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "status": old.Status, "version": old.Version + 1}, nil)
}

func (a adminAPI) putTerminationChecklist(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "terminasi.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Terminasi tidak ditemukan.")
		return
	}
	var body struct {
		Version uint64 `json:"version"`
		Items   []struct {
			Code string `json:"code"`
			Done bool   `json:"is_done"`
			Note string `json:"note"`
		} `json:"items"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Version == 0 || len(body.Items) != 2 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Versi dan dua item checklist wajib diisi.")
		return
	}
	seen := map[string]bool{}
	for _, item := range body.Items {
		if (item.Code != "assets_returned" && item.Code != "handover_complete") || seen[item.Code] || len([]rune(item.Note)) > 500 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Item checklist tidak valid.")
			return
		}
		seen[item.Code] = true
	}
	tx, err := a.db.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah checklist.")
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
	if old.Status == "cancelled" || old.Status == "rejected" {
		writeAPIError(w, r, 409, "TERMINATION_LOCKED", "Proses sudah ditutup.")
		return
	}
	for _, item := range body.Items {
		if item.Code == "assets_returned" && item.Done {
			var active int
			if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employee_assets WHERE tenant_id=? AND employee_id=? AND active_flag=1`, c.tenant.ID, old.EmployeeID).Scan(&active); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa aset karyawan.")
				return
			}
			if active > 0 {
				writeAPIError(w, r, 409, "ASSETS_OUTSTANDING", "Masih ada aset yang belum dikembalikan.")
				return
			}
		}
	}
	for _, item := range body.Items {
		if _, err = tx.ExecContext(r.Context(), `UPDATE termination_checklist SET is_done=?,note=?,updated_by=? WHERE tenant_id=? AND termination_id=? AND item_code=?`, item.Done, nullableString(strings.TrimSpace(item.Note)), c.state.User.ID, c.tenant.ID, id, item.Code); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan checklist.")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE terminations SET version=version+1 WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui versi.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "checklist", "terminations", id, map[string]any{"version": old.Version}, map[string]any{"version": old.Version + 1, "items": []string{"assets_returned", "handover_complete"}}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah checklist.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "version": old.Version + 1}, nil)
}
