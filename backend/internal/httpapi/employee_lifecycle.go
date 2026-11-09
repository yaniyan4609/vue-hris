package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

func (a adminAPI) employeeInScope(w http.ResponseWriter, r *http.Request, c adminContext, id string) bool {
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return false
	}
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	var found int
	err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employees e`+employeeScopeJoin+` WHERE e.tenant_id=? AND e.id=? AND `+scope, append([]any{c.tenant.ID, id}, args...)...).Scan(&found)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return false
	}
	if found == 0 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return false
	}
	return true
}

func (a adminAPI) getEmployeeSummary(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !a.employeeInScope(w, r, c, id) {
		return
	}
	var number, name, status, officeID, officeName, positionID, positionName, regionID, divisionID, departmentID string
	var archived sql.NullTime
	err := a.db.QueryRowContext(r.Context(), `SELECT e.employee_number,e.display_name,e.status,a.office_id,o.name,a.position_id,p.name,a.region_id,a.division_id,a.department_id,e.deleted_at FROM employees e`+employeeScopeJoin+` JOIN offices o ON o.tenant_id=a.tenant_id AND o.id=a.office_id JOIN positions p ON p.tenant_id=a.tenant_id AND p.id=a.position_id WHERE e.tenant_id=? AND e.id=?`, c.tenant.ID, id).Scan(&number, &name, &status, &officeID, &officeName, &positionID, &positionName, &regionID, &divisionID, &departmentID, &archived)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ringkasan karyawan.")
		return
	}
	var contractID, contractType, contractStatus, start string
	var end sql.NullString
	err = a.db.QueryRowContext(r.Context(), `SELECT id,contract_type,status,DATE_FORMAT(start_date,'%Y-%m-%d'),DATE_FORMAT(end_date,'%Y-%m-%d') FROM contracts WHERE tenant_id=? AND employee_id=? ORDER BY start_date DESC,id DESC LIMIT 1`, c.tenant.ID, id).Scan(&contractID, &contractType, &contractStatus, &start, &end)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontrak karyawan.")
		return
	}
	var contract any
	if err == nil {
		contract = map[string]any{"id": contractID, "contract_type": contractType, "status": contractStatus, "start_date": start, "end_date": nil}
		if end.Valid {
			contract.(map[string]any)["end_date"] = end.String
		}
	}
	var archivedAt any
	if archived.Valid {
		archivedAt = archived.Time.UTC().Format(time.RFC3339Nano)
	}
	writeAPIData(w, 200, map[string]any{"id": id, "employee_number": number, "display_name": name, "status": status, "archived_at": archivedAt, "assignment": map[string]any{"region_id": regionID, "office_id": officeID, "office_name": officeName, "division_id": divisionID, "department_id": departmentID, "position_id": positionID, "position_name": positionName}, "contract": contract}, nil)
}

type employeeTimelineEvent struct {
	At          time.Time      `json:"at"`
	Type        string         `json:"type"`
	ReferenceID string         `json:"reference_id"`
	Details     map[string]any `json:"details"`
}

func (a adminAPI) getEmployeeTimeline(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !a.employeeInScope(w, r, c, id) {
		return
	}
	events := []employeeTimelineEvent{}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,valid_from,office_id,position_id,valid_to FROM employee_assignments WHERE tenant_id=? AND employee_id=? ORDER BY valid_from DESC LIMIT 100`, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca timeline penugasan.")
		return
	}
	for rows.Next() {
		var ref, office, position string
		var at time.Time
		var until sql.NullTime
		if err = rows.Scan(&ref, &at, &office, &position, &until); err != nil {
			break
		}
		details := map[string]any{"office_id": office, "position_id": position, "valid_to": nil}
		if until.Valid {
			details["valid_to"] = until.Time.UTC().Format(time.RFC3339Nano)
		}
		events = append(events, employeeTimelineEvent{at, "assignment", ref, details})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca timeline penugasan.")
		return
	}
	rows, err = a.db.QueryContext(r.Context(), `SELECT id,created_at,contract_type,status,DATE_FORMAT(start_date,'%Y-%m-%d') FROM contracts WHERE tenant_id=? AND employee_id=? ORDER BY created_at DESC LIMIT 100`, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca timeline kontrak.")
		return
	}
	for rows.Next() {
		var ref, kind, status, start string
		var at time.Time
		if err = rows.Scan(&ref, &at, &kind, &status, &start); err != nil {
			break
		}
		events = append(events, employeeTimelineEvent{at, "contract", ref, map[string]any{"contract_type": kind, "status": status, "start_date": start}})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca timeline kontrak.")
		return
	}
	rows, err = a.db.QueryContext(r.Context(), `SELECT id,created_at,change_type,status,DATE_FORMAT(effective_date,'%Y-%m-%d'),applied_at FROM employee_changes WHERE tenant_id=? AND employee_id=? ORDER BY created_at DESC LIMIT 100`, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca timeline mutasi.")
		return
	}
	for rows.Next() {
		var ref, kind, status, effective string
		var at time.Time
		var applied sql.NullTime
		if err = rows.Scan(&ref, &at, &kind, &status, &effective, &applied); err != nil {
			break
		}
		details := map[string]any{"change_type": kind, "status": status, "effective_date": effective, "applied_at": nil}
		if applied.Valid {
			details["applied_at"] = applied.Time
		}
		events = append(events, employeeTimelineEvent{at, "employee_change_request", ref, details})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca timeline mutasi.")
		return
	}
	rows, err = a.db.QueryContext(r.Context(), `SELECT id,created_at,action FROM audit_logs WHERE tenant_id=? AND entity_type='employees' AND entity_id=? ORDER BY created_at DESC LIMIT 100`, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca timeline status.")
		return
	}
	for rows.Next() {
		var ref, action string
		var at time.Time
		if err = rows.Scan(&ref, &at, &action); err != nil {
			break
		}
		events = append(events, employeeTimelineEvent{at, "employee_change", ref, map[string]any{"action": action}})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca timeline status.")
		return
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].At.Equal(events[j].At) {
			return events[i].ReferenceID > events[j].ReferenceID
		}
		return events[i].At.After(events[j].At)
	})
	if len(events) > 200 {
		events = events[:200]
	}
	writeAPIData(w, 200, map[string]any{"employee_id": id, "events": events}, nil)
}

func (a adminAPI) nextEmployeeNumber(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.buat")
	if !ok {
		return
	}
	var max sql.NullInt64
	err := a.db.QueryRowContext(r.Context(), `SELECT MAX(CAST(SUBSTRING(employee_number,5) AS UNSIGNED)) FROM employees WHERE tenant_id=? AND employee_number REGEXP '^EMP-[0-9]{6,}$'`, c.tenant.ID).Scan(&max)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyarankan nomor karyawan.")
		return
	}
	next := int64(1)
	if max.Valid {
		next = max.Int64 + 1
	}
	if next > 999999999 {
		writeAPIError(w, r, 409, "NUMBER_EXHAUSTED", "Rentang nomor otomatis penuh.")
		return
	}
	suggestion := "EMP-" + strings.Repeat("0", maxInt(0, 6-len(strconv.FormatInt(next, 10)))) + strconv.FormatInt(next, 10)
	writeAPIData(w, 200, map[string]any{"suggestion": suggestion, "prefix": "EMP-", "reserved": false}, nil)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (a adminAPI) archiveEmployee(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.hapus")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengarsipkan karyawan.")
		return
	}
	defer tx.Rollback()
	if !archiveEmployeeRow(w, r, tx, c, id) {
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengarsipkan karyawan.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func archiveEmployeeRow(w http.ResponseWriter, r *http.Request, tx *sql.Tx, c adminContext, id string) bool {
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	var status string
	var deleted sql.NullTime
	var userID sql.NullString
	err := tx.QueryRowContext(r.Context(), `SELECT e.status,e.deleted_at,e.user_id FROM employees e`+employeeScopeJoin+` WHERE e.tenant_id=? AND e.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, id}, args...)...).Scan(&status, &deleted, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return false
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return false
	}
	if deleted.Valid {
		return true
	}
	if status != "resigned" && status != "terminated" {
		writeAPIError(w, r, 409, "EMPLOYEE_ACTIVE", "Karyawan aktif harus melalui alur terminasi.")
		return false
	}
	var subordinates int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM employee_assignments a JOIN employees e ON e.tenant_id=a.tenant_id AND e.id=a.employee_id WHERE a.tenant_id=? AND a.reports_to_employee_id=? AND a.current_flag=1 AND e.deleted_at IS NULL AND e.status IN ('active','probation','leave')`, c.tenant.ID, id).Scan(&subordinates); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa bawahan aktif.")
		return false
	}
	if subordinates != 0 {
		writeAPIError(w, r, 409, "IN_USE", "Karyawan masih menjadi atasan karyawan aktif.")
		return false
	}
	// Legacy inactive rows may still carry an open assignment. Close it as part
	// of archival so organization scope and roster never treat it as active.
	if _, err = tx.ExecContext(r.Context(), `UPDATE employee_assignments SET valid_to=UTC_TIMESTAMP(6),current_flag=NULL WHERE tenant_id=? AND employee_id=? AND current_flag=1`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup penugasan karyawan.")
		return false
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE employee_assignment_history SET valid_until=UTC_TIMESTAMP(6) WHERE tenant_id=? AND employee_id=? AND valid_until IS NULL`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menutup riwayat penugasan.")
		return false
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE employees SET deleted_at=UTC_TIMESTAMP(6),version=version+1 WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengarsipkan karyawan.")
		return false
	}
	if userID.Valid {
		for _, step := range []string{
			`UPDATE tenant_memberships SET status='inactive',ended_at=UTC_TIMESTAMP(6),access_version=access_version+1 WHERE tenant_id=? AND user_id=?`,
			`UPDATE user_roles SET revoked_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND user_id=? AND revoked_at IS NULL`,
			`UPDATE sessions SET revoked_at=UTC_TIMESTAMP(6),revoked_reason='employee_archived' WHERE selected_tenant_id=? AND user_id=? AND revoked_at IS NULL`,
		} {
			if _, err = tx.ExecContext(r.Context(), step, c.tenant.ID, userID.String); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menonaktifkan akses karyawan.")
				return false
			}
		}
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "archive", "employees", id, map[string]any{"status": status}, map[string]any{"status": status, "archived": true, "account_disabled": userID.Valid}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat arsip karyawan.")
		return false
	}
	return true
}

func (a adminAPI) restoreEmployee(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.hapus")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memulihkan karyawan.")
		return
	}
	defer tx.Rollback()
	var deleted sql.NullTime
	var status string
	err = tx.QueryRowContext(r.Context(), `SELECT e.deleted_at,e.status FROM employees e`+employeeScopeJoin+` WHERE e.tenant_id=? AND e.id=? AND `+scope+` FOR UPDATE`, append([]any{c.tenant.ID, id}, args...)...).Scan(&deleted, &status)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Karyawan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa karyawan.")
		return
	}
	if deleted.Valid {
		if _, err = tx.ExecContext(r.Context(), `UPDATE employees SET deleted_at=NULL,version=version+1 WHERE tenant_id=? AND id=?`, c.tenant.ID, id); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memulihkan karyawan.")
			return
		}
		if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "restore", "employees", id, map[string]any{"archived": true}, map[string]any{"archived": false, "status": status, "account_reactivated": false}); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat pemulihan.")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memulihkan karyawan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "status": status, "archived": false, "account_reactivated": false}, nil)
}

func (a adminAPI) bulkArchiveEmployees(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "karyawan.hapus")
	if !ok {
		return
	}
	var body struct {
		IDs []string `json:"ids"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if len(body.IDs) == 0 || len(body.IDs) > 100 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Daftar karyawan harus berisi 1–100 ID.")
		return
	}
	seen := map[string]bool{}
	for _, id := range body.IDs {
		if !fileIDPattern.MatchString(id) || seen[id] {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID karyawan tidak valid atau berulang.")
			return
		}
		seen[id] = true
	}
	sort.Strings(body.IDs)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengarsipkan karyawan.")
		return
	}
	defer tx.Rollback()
	for _, id := range body.IDs {
		if !archiveEmployeeRow(w, r, tx, c, id) {
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengarsipkan karyawan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"archived_ids": body.IDs, "count": len(body.IDs)}, nil)
}
