package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type employeeChange struct {
	ID                  string     `json:"id"`
	EmployeeID          string     `json:"employee_id"`
	RequestID           *string    `json:"request_id"`
	ChangeType          string     `json:"change_type"`
	Status              string     `json:"status"`
	EffectiveDate       string     `json:"effective_date"`
	SourceAssignmentID  string     `json:"source_assignment_id"`
	OfficeID            string     `json:"office_id"`
	PositionID          string     `json:"position_id"`
	ReportsToEmployeeID *string    `json:"reports_to_employee_id"`
	Reason              string     `json:"reason"`
	Version             uint64     `json:"version"`
	AppliedAssignmentID *string    `json:"applied_assignment_id"`
	AppliedAt           *time.Time `json:"applied_at"`
	ApplyError          *string    `json:"apply_error"`
	CreatedBy           string     `json:"created_by"`
	CreatedAt           time.Time  `json:"created_at"`
	Approval            any        `json:"approval,omitempty"`
}

const employeeChangeColumns = `x.id,x.employee_id,x.request_id,x.change_type,x.status,DATE_FORMAT(x.effective_date,'%Y-%m-%d'),x.source_assignment_id,x.new_office_id,x.new_position_id,x.new_reports_to_id,x.reason,x.version,x.applied_assignment_id,x.applied_at,x.apply_error,x.created_by,x.created_at`

func scanEmployeeChange(row interface{ Scan(...any) error }) (employeeChange, error) {
	var item employeeChange
	var requestID, managerID, appliedID, applyError, sourceID, officeID, positionID, createdBy sql.NullString
	var appliedAt sql.NullTime
	err := row.Scan(&item.ID, &item.EmployeeID, &requestID, &item.ChangeType, &item.Status, &item.EffectiveDate, &sourceID, &officeID, &positionID, &managerID, &item.Reason, &item.Version, &appliedID, &appliedAt, &applyError, &createdBy, &item.CreatedAt)
	item.SourceAssignmentID, item.OfficeID, item.PositionID, item.CreatedBy = sourceID.String, officeID.String, positionID.String, createdBy.String
	if requestID.Valid {
		item.RequestID = &requestID.String
	}
	if managerID.Valid {
		item.ReportsToEmployeeID = &managerID.String
	}
	if appliedID.Valid {
		item.AppliedAssignmentID = &appliedID.String
	}
	if appliedAt.Valid {
		item.AppliedAt = &appliedAt.Time
	}
	if applyError.Valid {
		item.ApplyError = &applyError.String
	}
	return item, err
}

func employeeChangeScope(c adminContext) (string, []any) {
	return access.EmployeeWhere(c.subject, c.effective)
}

func (a adminAPI) listEmployeeChanges(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "mutasi.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	scope, scopeArgs := employeeChangeScope(c)
	where := `x.tenant_id=? AND ` + scope
	args := append([]any{c.tenant.ID}, scopeArgs...)
	if kind := r.URL.Query().Get("change_type"); kind != "" {
		if !validChangeType(kind) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jenis perubahan tidak valid.")
			return
		}
		where += ` AND x.change_type=?`
		args = append(args, kind)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		if !validChangeStatus(status) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status perubahan tidak valid.")
			return
		}
		where += ` AND x.status=?`
		args = append(args, status)
	}
	for _, filter := range []struct{ name, op string }{{"effective_from", ">="}, {"effective_to", "<="}} {
		if value := r.URL.Query().Get(filter.name); value != "" {
			if !validChangeDate(value) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal filter tidak valid.")
				return
			}
			where += ` AND x.effective_date` + filter.op + `?`
			args = append(args, value)
		}
	}
	if id := r.URL.Query().Get("employee_id"); id != "" {
		if !fileIDPattern.MatchString(id) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID karyawan tidak valid.")
			return
		}
		where += ` AND x.employee_id=?`
		args = append(args, id)
	}
	join := ` FROM employee_changes x JOIN employees e ON e.tenant_id=x.tenant_id AND e.id=x.employee_id LEFT JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1`
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+join+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung perubahan karyawan.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+employeeChangeColumns+join+` WHERE `+where+` ORDER BY x.created_at DESC,x.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca perubahan karyawan.")
		return
	}
	defer rows.Close()
	items := []employeeChange{}
	for rows.Next() {
		item, e := scanEmployeeChange(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca perubahan karyawan.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca perubahan karyawan.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) getEmployeeChange(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "mutasi.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Perubahan karyawan tidak ditemukan.")
		return
	}
	scope, args := employeeChangeScope(c)
	params := append([]any{c.tenant.ID, id}, args...)
	item, err := scanEmployeeChange(a.db.QueryRowContext(r.Context(), `SELECT `+employeeChangeColumns+` FROM employee_changes x JOIN employees e ON e.tenant_id=x.tenant_id AND e.id=x.employee_id LEFT JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE x.tenant_id=? AND x.id=? AND `+scope, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Perubahan karyawan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca perubahan karyawan.")
		return
	}
	if item.RequestID != nil {
		request, err := scanRequest(a.db.QueryRowContext(r.Context(), `SELECT `+requestColumns+requestJoin+` WHERE q.tenant_id=? AND q.id=?`, c.tenant.ID, *item.RequestID))
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca rantai approval.")
			return
		}
		chain := map[string]any{"state": request.Approval, "tasks": []map[string]any{}, "actions": []map[string]any{}}
		if request.Approval != nil {
			tasks := []map[string]any{}
			rows, e := a.db.QueryContext(r.Context(), `SELECT id,attempt_no,level_no,assignee_user_id,status,created_at,decided_at FROM approval_tasks WHERE tenant_id=? AND instance_id=? ORDER BY attempt_no,level_no,created_at,id`, c.tenant.ID, request.Approval.ID)
			if e != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tugas approval.")
				return
			}
			for rows.Next() {
				var taskID, assignee, status string
				var attempt, level int
				var created time.Time
				var decided sql.NullTime
				if e = rows.Scan(&taskID, &attempt, &level, &assignee, &status, &created, &decided); e != nil {
					break
				}
				var decidedAt any
				if decided.Valid {
					decidedAt = decided.Time
				}
				tasks = append(tasks, map[string]any{"id": taskID, "attempt_no": attempt, "level_no": level, "assignee_user_id": assignee, "status": status, "created_at": created, "decided_at": decidedAt})
			}
			if e == nil {
				e = rows.Err()
			}
			rows.Close()
			if e != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tugas approval.")
				return
			}
			chain["tasks"] = tasks
			actions := []map[string]any{}
			rows, e = a.db.QueryContext(r.Context(), `SELECT id,task_id,level_no,actor_user_id,action,reason,created_at FROM approval_actions WHERE tenant_id=? AND instance_id=? ORDER BY created_at,id`, c.tenant.ID, request.Approval.ID)
			if e != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aksi approval.")
				return
			}
			for rows.Next() {
				var actionID, taskID, actor, action string
				var level int
				var reason sql.NullString
				var created time.Time
				if e = rows.Scan(&actionID, &taskID, &level, &actor, &action, &reason, &created); e != nil {
					break
				}
				actions = append(actions, map[string]any{"id": actionID, "task_id": taskID, "level_no": level, "actor_user_id": actor, "action": action, "reason": reason.String, "created_at": created})
			}
			if e == nil {
				e = rows.Err()
			}
			rows.Close()
			if e != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aksi approval.")
				return
			}
			chain["actions"] = actions
		}
		item.Approval = chain
	}
	writeAPIData(w, 200, item, nil)
}
