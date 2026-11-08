package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) createEmployeeAssignmentCorrection(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.ubah")
	if !ok {
		return
	}
	employeeID := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(employeeID) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum koreksi penugasan.")
		return
	}
	var body struct {
		CurrentAssignmentID string `json:"current_assignment_id"`
		EffectiveFrom       string `json:"effective_from"`
		OfficeID            string `json:"office_id"`
		PositionID          string `json:"position_id"`
		ReportsToEmployeeID string `json:"reports_to_employee_id"`
		Reason              string `json:"reason"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if !fileIDPattern.MatchString(body.CurrentAssignmentID) || !fileIDPattern.MatchString(body.OfficeID) || !fileIDPattern.MatchString(body.PositionID) || body.ReportsToEmployeeID != "" && !fileIDPattern.MatchString(body.ReportsToEmployeeID) || body.ReportsToEmployeeID == employeeID || len([]rune(body.Reason)) < 10 || len([]rune(body.Reason)) > 500 || !validRuleDates(body.EffectiveFrom, nil) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Penugasan, tanggal, atau alasan koreksi tidak valid.")
		return
	}
	today := tenantToday(c.tenant.Timezone)
	if body.EffectiveFrom > today {
		writeAPIError(w, r, 409, "MUTATION_REQUIRED", "Perubahan terjadwal harus melalui modul Mutasi.")
		return
	}
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	localDate, _ := time.ParseInLocation("2006-01-02", body.EffectiveFrom, zone)
	effectiveAt := localDate.UTC()
	if body.EffectiveFrom == today {
		effectiveAt = time.Now().UTC()
	}
	targetEmployee := employee{ID: employeeID, BaseOfficeID: body.OfficeID, PositionID: body.PositionID, ReportsToID: body.ReportsToEmployeeID}
	target, err := a.targetFor(r.Context(), c.tenant.ID, targetEmployee)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 409, "ORGANIZATION_UNAVAILABLE", "Kantor atau jabatan tidak aktif.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa organisasi.")
		return
	}
	if !c.effective.Includes(c.subject, target) {
		writeAPIError(w, r, 403, "SCOPE_DENIED", "Penempatan baru di luar scope.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengoreksi penugasan.")
		return
	}
	defer tx.Rollback()
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	var currentID, status string
	var deleted sql.NullTime
	var oldVersion uint64
	err = tx.QueryRowContext(r.Context(), `SELECT e.status,e.deleted_at,e.version,a.id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE e.tenant_id=? AND e.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, employeeID}, args...)...).Scan(&status, &deleted, &oldVersion, &currentID)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return
	}
	if deleted.Valid || status == "resigned" || status == "terminated" {
		writeAPIError(w, r, 409, "EMPLOYEE_INACTIVE", "Karyawan tidak aktif.")
		return
	}
	if currentID != body.CurrentAssignmentID {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Penugasan saat ini sudah berubah.")
		return
	}
	var oldFrom time.Time
	var sourceID sql.NullString
	err = tx.QueryRowContext(r.Context(), `SELECT valid_from,source_history_id FROM employee_assignments WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, currentID).Scan(&oldFrom, &sourceID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca penugasan lama.")
		return
	}
	if !effectiveAt.After(oldFrom) {
		writeAPIError(w, r, 409, "DATE_CONFLICT", "Tanggal koreksi harus setelah awal penugasan saat ini.")
		return
	}
	if err = lockEmployeeOrganization(r.Context(), tx, c.tenant.ID, body.OfficeID, body.PositionID); err != nil {
		writeAPIError(w, r, 409, "ORGANIZATION_UNAVAILABLE", "Kantor atau jabatan tidak aktif.")
		return
	}
	if body.ReportsToEmployeeID != "" {
		manager := body.ReportsToEmployeeID
		seen := map[string]bool{}
		for depth := 0; manager != "" && depth < 100; depth++ {
			if manager == employeeID || seen[manager] {
				writeAPIError(w, r, 409, "MANAGER_CYCLE", "Rantai atasan membentuk siklus.")
				return
			}
			seen[manager] = true
			var next sql.NullString
			err = tx.QueryRowContext(r.Context(), `SELECT a.reports_to_employee_id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE e.tenant_id=? AND e.id=? AND e.deleted_at IS NULL AND e.status IN ('active','probation','leave')`, c.tenant.ID, manager).Scan(&next)
			if errors.Is(err, sql.ErrNoRows) {
				writeAPIError(w, r, 409, "MANAGER_UNAVAILABLE", "Atasan tidak aktif atau di luar tenant.")
				return
			}
			if err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa rantai atasan.")
				return
			}
			manager = next.String
		}
		if manager != "" {
			writeAPIError(w, r, 409, "MANAGER_CYCLE", "Rantai atasan terlalu panjang.")
			return
		}
	}
	var locked int
	if err = tx.QueryRowContext(r.Context(), `SELECT (SELECT COUNT(*) FROM work_schedules WHERE tenant_id=? AND employee_id=? AND status='published' AND work_date>=?)+(SELECT COUNT(*) FROM payroll_runs pr JOIN payroll_periods pp ON pp.tenant_id=pr.tenant_id AND pp.id=pr.period_id WHERE pr.tenant_id=? AND pr.status='finalized' AND pp.end_date>=?)`, c.tenant.ID, employeeID, body.EffectiveFrom, c.tenant.ID, body.EffectiveFrom).Scan(&locked); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa jadwal dan payroll.")
		return
	}
	if locked != 0 {
		writeAPIError(w, r, 409, "HISTORY_LOCKED", "Roster terbit atau payroll final sudah memakai tanggal ini.")
		return
	}
	var regionID, divisionID, departmentID, grade string
	err = tx.QueryRowContext(r.Context(), `SELECT o.region_id,d.division_id,p.department_id,IFNULL(p.grade,'') FROM offices o JOIN positions p ON p.tenant_id=o.tenant_id AND p.id=? JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id WHERE o.tenant_id=? AND o.id=?`, body.PositionID, c.tenant.ID, body.OfficeID).Scan(&regionID, &divisionID, &departmentID, &grade)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan snapshot organisasi.")
		return
	}
	until := effectiveAt.Add(-time.Microsecond)
	if _, err = tx.ExecContext(r.Context(), `UPDATE employee_assignments SET valid_to=?,current_flag=NULL WHERE tenant_id=? AND id=?`, until, c.tenant.ID, currentID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup penugasan lama.")
		return
	}
	if sourceID.Valid {
		if _, err = tx.ExecContext(r.Context(), `UPDATE employee_assignment_history SET valid_until=? WHERE tenant_id=? AND id=?`, until, c.tenant.ID, sourceID.String); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup riwayat lama.")
			return
		}
	}
	historyID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID riwayat penugasan.")
		return
	}
	assignmentID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID penugasan.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO employee_assignment_history (tenant_id,id,employee_id,base_office_id,position_id,reports_to_id,valid_from) VALUES (?,?,?,?,?,?,?)`, c.tenant.ID, historyID, employeeID, body.OfficeID, body.PositionID, nullID(body.ReportsToEmployeeID), effectiveAt); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat riwayat baru.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,valid_from,current_flag,source_history_id,created_by) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.tenant.ID, assignmentID, employeeID, regionID, body.OfficeID, divisionID, departmentID, body.PositionID, nullID(body.ReportsToEmployeeID), nullID(grade), effectiveAt, 1, historyID, c.state.User.ID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat penugasan baru.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE employees SET base_office_id=?,position_id=?,reports_to_id=?,version=version+1 WHERE tenant_id=? AND id=?`, body.OfficeID, body.PositionID, nullID(body.ReportsToEmployeeID), c.tenant.ID, employeeID); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memperbarui karyawan.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "correct_assignment", "employees", employeeID, map[string]any{"assignment_id": currentID, "version": oldVersion}, map[string]any{"assignment_id": assignmentID, "effective_from": body.EffectiveFrom, "reason": body.Reason, "version": oldVersion + 1}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit penugasan.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengoreksi penugasan.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": assignmentID, "employee_id": employeeID, "effective_from": effectiveAt.Format(time.RFC3339Nano), "version": oldVersion + 1}, nil)
}
