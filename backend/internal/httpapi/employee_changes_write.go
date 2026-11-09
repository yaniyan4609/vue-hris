package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type employeeChangeInput struct {
	EmployeeID          string `json:"employee_id"`
	ChangeType          string `json:"change_type"`
	EffectiveDate       string `json:"effective_date"`
	OfficeID            string `json:"office_id"`
	PositionID          string `json:"position_id"`
	ReportsToEmployeeID string `json:"reports_to_employee_id"`
	Reason              string `json:"reason"`
	Version             uint64 `json:"version,omitempty"`
}

func validChangeType(kind string) bool {
	return kind == "mutasi" || kind == "promosi" || kind == "demosi"
}
func validChangeStatus(status string) bool {
	switch status {
	case "draft", "in_review", "returned", "approved", "rejected", "cancelled", "applied":
		return true
	}
	return false
}
func validChangeDate(value string) bool {
	d, err := time.Parse("2006-01-02", value)
	return err == nil && d.Format("2006-01-02") == value
}
func changeRequestType(kind string) string {
	if kind == "promosi" {
		return "promotion"
	}
	return "mutation"
}

func validEmployeeChangeInput(in *employeeChangeInput) bool {
	in.Reason = strings.TrimSpace(in.Reason)
	return fileIDPattern.MatchString(in.EmployeeID) && validChangeType(in.ChangeType) && validChangeDate(in.EffectiveDate) && fileIDPattern.MatchString(in.OfficeID) && fileIDPattern.MatchString(in.PositionID) && (in.ReportsToEmployeeID == "" || fileIDPattern.MatchString(in.ReportsToEmployeeID)) && in.ReportsToEmployeeID != in.EmployeeID && len([]rune(in.Reason)) >= 10 && len([]rune(in.Reason)) <= 500
}

func (a adminAPI) validateChangeDestination(w http.ResponseWriter, r *http.Request, c adminContext, in employeeChangeInput) bool {
	if in.EffectiveDate < tenantToday(c.tenant.Timezone) {
		writeAPIError(w, r, 409, "DATE_CONFLICT", "Tanggal efektif tidak boleh di masa lalu.")
		return false
	}
	targetEmployee := employee{ID: in.EmployeeID, BaseOfficeID: in.OfficeID, PositionID: in.PositionID, ReportsToID: in.ReportsToEmployeeID}
	target, err := a.targetFor(r.Context(), c.tenant.ID, targetEmployee)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 409, "ORGANIZATION_UNAVAILABLE", "Kantor atau jabatan tujuan tidak aktif.")
		return false
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa organisasi tujuan.")
		return false
	}
	if !c.effective.Includes(c.subject, target) {
		writeAPIError(w, r, 403, "SCOPE_DENIED", "Penempatan tujuan di luar scope.")
		return false
	}
	if in.ReportsToEmployeeID != "" {
		var status string
		if err := a.db.QueryRowContext(r.Context(), `SELECT status FROM employees WHERE tenant_id=? AND id=? AND deleted_at IS NULL`, c.tenant.ID, in.ReportsToEmployeeID).Scan(&status); err != nil || status == "resigned" || status == "terminated" {
			writeAPIError(w, r, 409, "MANAGER_INVALID", "Atasan tujuan tidak aktif dalam tenant.")
			return false
		}
		manager := in.ReportsToEmployeeID
		for depth := 0; depth < 100 && manager != ""; depth++ {
			if manager == in.EmployeeID {
				writeAPIError(w, r, 409, "MANAGER_CYCLE", "Rantai atasan membentuk siklus.")
				return false
			}
			var next sql.NullString
			if err := a.db.QueryRowContext(r.Context(), `SELECT reports_to_id FROM employees WHERE tenant_id=? AND id=?`, c.tenant.ID, manager).Scan(&next); err != nil {
				writeAPIError(w, r, 409, "MANAGER_INVALID", "Rantai atasan tidak tersedia.")
				return false
			}
			manager = next.String
		}
		if manager != "" {
			writeAPIError(w, r, 409, "MANAGER_CYCLE", "Rantai atasan terlalu panjang.")
			return false
		}
	}
	return true
}

func (a adminAPI) createEmployeeChange(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "mutasi.buat")
	if !ok {
		return
	}
	var in employeeChangeInput
	if !readJSON(w, r, &in) {
		return
	}
	if !validEmployeeChangeInput(&in) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data perubahan karyawan tidak valid.")
		return
	}
	if !a.validateChangeDestination(w, r, c, in) {
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat draft.")
		return
	}
	defer tx.Rollback()
	scope, args := employeeChangeScope(c)
	params := append([]any{c.tenant.ID, in.EmployeeID}, args...)
	var employeeStatus, sourceID, oldOffice, oldPosition, oldManager string
	var deleted sql.NullTime
	var sourceFrom time.Time
	err = tx.QueryRowContext(r.Context(), `SELECT e.status,e.deleted_at,a.id,a.valid_from,a.office_id,a.position_id,COALESCE(a.reports_to_employee_id,'') FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE e.tenant_id=? AND e.id=? AND `+scope+` FOR UPDATE`, params...).Scan(&employeeStatus, &deleted, &sourceID, &sourceFrom, &oldOffice, &oldPosition, &oldManager)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca penugasan karyawan.")
		return
	}
	if deleted.Valid || employeeStatus == "resigned" || employeeStatus == "terminated" {
		writeAPIError(w, r, 409, "EMPLOYEE_INACTIVE", "Karyawan tidak aktif.")
		return
	}
	if oldOffice == in.OfficeID && oldPosition == in.PositionID && oldManager == in.ReportsToEmployeeID {
		writeAPIError(w, r, 409, "NO_CHANGE", "Tujuan sama dengan penugasan saat ini.")
		return
	}
	if in.EffectiveDate <= changeLocalDate(sourceFrom, c.tenant.Timezone) {
		writeAPIError(w, r, 409, "DATE_CONFLICT", "Tanggal efektif harus setelah awal penugasan saat ini.")
		return
	}
	var active int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employee_changes WHERE tenant_id=? AND employee_id=? AND status IN ('draft','returned','in_review','approved')`, c.tenant.ID, in.EmployeeID).Scan(&active); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa perubahan aktif.")
		return
	}
	if active != 0 {
		writeAPIError(w, r, 409, "CHANGE_CONFLICT", "Karyawan sudah memiliki perubahan aktif.")
		return
	}
	if err = lockEmployeeOrganization(r.Context(), tx, c.tenant.ID, in.OfficeID, in.PositionID); err != nil {
		writeAPIError(w, r, 409, "ORGANIZATION_UNAVAILABLE", "Kantor atau jabatan tujuan tidak aktif.")
		return
	}
	var divisionID, departmentID string
	if err = tx.QueryRowContext(r.Context(), `SELECT d.division_id,p.department_id FROM positions p JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id WHERE p.tenant_id=? AND p.id=?`, c.tenant.ID, in.PositionID).Scan(&divisionID, &departmentID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengambil snapshot organisasi.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID perubahan.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO employee_changes (tenant_id,id,employee_id,change_type,effective_date,source_assignment_id,new_office_id,new_division_id,new_department_id,new_position_id,new_reports_to_id,reason,created_by) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.tenant.ID, id, in.EmployeeID, in.ChangeType, in.EffectiveDate, sourceID, in.OfficeID, divisionID, departmentID, in.PositionID, nullID(in.ReportsToEmployeeID), in.Reason, c.state.User.ID)
	if err != nil {
		writeAPIError(w, r, 409, "CHANGE_CONFLICT", "Karyawan sudah memiliki perubahan aktif atau tujuan tidak tersedia.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "employee_changes", id, nil, map[string]any{"employee_id": in.EmployeeID, "change_type": in.ChangeType, "effective_date": in.EffectiveDate, "source_assignment_id": sourceID, "office_id": in.OfficeID, "position_id": in.PositionID, "reports_to_employee_id": in.ReportsToEmployeeID}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat draft.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "status": "draft", "version": 1}, nil)
}

func changeLocalDate(at time.Time, zoneName string) string {
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		zone = time.UTC
	}
	return at.In(zone).Format("2006-01-02")
}

func (a adminAPI) patchEmployeeChange(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "mutasi.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Perubahan tidak ditemukan.")
		return
	}
	var in employeeChangeInput
	if !readJSON(w, r, &in) {
		return
	}
	if !validEmployeeChangeInput(&in) || in.Version == 0 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Data atau versi perubahan tidak valid.")
		return
	}
	if !a.validateChangeDestination(w, r, c, in) {
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah draft.")
		return
	}
	defer tx.Rollback()
	scope, args := employeeChangeScope(c)
	params := append([]any{c.tenant.ID, id}, args...)
	var old employeeChange
	var sourceFrom time.Time
	var oldOffice, oldPosition, oldManager string
	var requestID sql.NullString
	err = tx.QueryRowContext(r.Context(), `SELECT x.employee_id,x.change_type,x.status,x.version,x.request_id,x.source_assignment_id,a.valid_from,a.office_id,a.position_id,COALESCE(a.reports_to_employee_id,'') FROM employee_changes x JOIN employees e ON e.tenant_id=x.tenant_id AND e.id=x.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE x.tenant_id=? AND x.id=? AND `+scope+` FOR UPDATE`, params...).Scan(&old.EmployeeID, &old.ChangeType, &old.Status, &old.Version, &requestID, &old.SourceAssignmentID, &sourceFrom, &oldOffice, &oldPosition, &oldManager)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Perubahan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca draft.")
		return
	}
	if old.Status != "draft" && old.Status != "returned" {
		writeAPIError(w, r, 409, "CHANGE_NOT_EDITABLE", "Hanya draft atau pengajuan dikembalikan yang dapat diubah.")
		return
	}
	if old.Version != in.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Draft sudah berubah.")
		return
	}
	if old.EmployeeID != in.EmployeeID || old.ChangeType != in.ChangeType {
		writeAPIError(w, r, 409, "IDENTITY_IMMUTABLE", "Karyawan dan jenis perubahan tidak dapat diganti.")
		return
	}
	if oldOffice == in.OfficeID && oldPosition == in.PositionID && oldManager == in.ReportsToEmployeeID {
		writeAPIError(w, r, 409, "NO_CHANGE", "Tujuan sama dengan penugasan saat ini.")
		return
	}
	if in.EffectiveDate <= changeLocalDate(sourceFrom, c.tenant.Timezone) {
		writeAPIError(w, r, 409, "DATE_CONFLICT", "Tanggal efektif harus setelah awal penugasan saat ini.")
		return
	}
	if err = lockEmployeeOrganization(r.Context(), tx, c.tenant.ID, in.OfficeID, in.PositionID); err != nil {
		writeAPIError(w, r, 409, "ORGANIZATION_UNAVAILABLE", "Kantor atau jabatan tujuan tidak aktif.")
		return
	}
	var divisionID, departmentID string
	if err = tx.QueryRowContext(r.Context(), `SELECT d.division_id,p.department_id FROM positions p JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id WHERE p.tenant_id=? AND p.id=?`, c.tenant.ID, in.PositionID).Scan(&divisionID, &departmentID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengambil snapshot organisasi.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE employee_changes SET effective_date=?,new_office_id=?,new_division_id=?,new_department_id=?,new_position_id=?,new_reports_to_id=?,reason=?,version=version+1,apply_error=NULL WHERE tenant_id=? AND id=? AND version=? AND status IN ('draft','returned')`, in.EffectiveDate, in.OfficeID, divisionID, departmentID, in.PositionID, nullID(in.ReportsToEmployeeID), in.Reason, c.tenant.ID, id, in.Version)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah draft.")
		return
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Draft sudah berubah.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "employee_changes", id, map[string]any{"version": in.Version}, map[string]any{"version": in.Version + 1, "effective_date": in.EffectiveDate, "office_id": in.OfficeID, "position_id": in.PositionID, "reports_to_employee_id": in.ReportsToEmployeeID}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah draft.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "status": old.Status, "version": in.Version + 1, "request_id": requestID.String}, nil)
}

func (a adminAPI) deleteEmployeeChange(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "mutasi.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Draft tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus draft.")
		return
	}
	defer tx.Rollback()
	scope, args := employeeChangeScope(c)
	params := append([]any{c.tenant.ID, id}, args...)
	var status string
	err = tx.QueryRowContext(r.Context(), `SELECT x.status FROM employee_changes x JOIN employees e ON e.tenant_id=x.tenant_id AND e.id=x.employee_id LEFT JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE x.tenant_id=? AND x.id=? AND `+scope+` FOR UPDATE`, params...).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Draft tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca draft.")
		return
	}
	if status != "draft" {
		writeAPIError(w, r, 409, "CHANGE_NOT_EDITABLE", "Hanya draft yang dapat dihapus.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "delete", "employee_changes", id, map[string]any{"status": "draft"}, nil); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM employee_changes WHERE tenant_id=? AND id=? AND status='draft'`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus draft.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus draft.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
