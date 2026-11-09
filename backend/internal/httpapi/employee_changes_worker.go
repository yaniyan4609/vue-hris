package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"digaji.id/hris/internal/ids"
)

// RunDueEmployeeChanges is safe to invoke from multiple API processes. Each
// change and employee is locked before assignment history is updated.
func RunDueEmployeeChanges(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT x.tenant_id,x.id,t.timezone FROM employee_changes x JOIN tenants t ON t.id=x.tenant_id WHERE x.status='approved' AND x.source_assignment_id IS NOT NULL AND x.created_by IS NOT NULL AND x.request_id IS NOT NULL AND x.new_office_id IS NOT NULL AND x.new_position_id IS NOT NULL ORDER BY (x.apply_error IS NOT NULL),x.effective_date,x.id LIMIT 500`)
	if err != nil {
		return 0, err
	}
	type due struct{ tenantID, id, zone string }
	items := []due{}
	for rows.Next() {
		var item due
		if err = rows.Scan(&item.tenantID, &item.id, &item.zone); err != nil {
			break
		}
		items = append(items, item)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, item := range items {
		done, e := applyDueEmployeeChange(ctx, db, item.tenantID, item.id, item.zone)
		if e != nil {
			return applied, e
		}
		if done {
			applied++
		}
	}
	return applied, nil
}

func applyDueEmployeeChange(ctx context.Context, db *sql.DB, tenantID, id, zoneName string) (bool, error) {
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		return false, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var employeeID, requestID, status, sourceID, officeID, positionID, actorID, kind string
	var managerID sql.NullString
	var effectiveDate time.Time
	err = tx.QueryRowContext(ctx, `SELECT employee_id,request_id,status,effective_date,source_assignment_id,new_office_id,new_position_id,new_reports_to_id,created_by,change_type FROM employee_changes WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, id).Scan(&employeeID, &requestID, &status, &effectiveDate, &sourceID, &officeID, &positionID, &managerID, &actorID, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if status != "approved" || effectiveDate.Format("2006-01-02") > time.Now().In(zone).Format("2006-01-02") {
		return false, nil
	}
	block := func(code string) (bool, error) {
		_, e := tx.ExecContext(ctx, `UPDATE employee_changes SET apply_error=? WHERE tenant_id=? AND id=? AND status='approved'`, code, tenantID, id)
		if e != nil {
			return false, e
		}
		return false, tx.Commit()
	}
	var employeeStatus, currentID string
	var deleted sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT e.status,e.deleted_at,a.id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1 WHERE e.tenant_id=? AND e.id=? FOR UPDATE`, tenantID, employeeID).Scan(&employeeStatus, &deleted, &currentID)
	if err != nil || deleted.Valid || employeeStatus == "resigned" || employeeStatus == "terminated" {
		return block("EMPLOYEE_INACTIVE")
	}
	if currentID != sourceID {
		return block("ASSIGNMENT_CHANGED")
	}
	var sourceFrom time.Time
	var historyID sql.NullString
	if err = tx.QueryRowContext(ctx, `SELECT valid_from,source_history_id FROM employee_assignments WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, sourceID).Scan(&sourceFrom, &historyID); err != nil {
		return false, err
	}
	localDate, err := time.ParseInLocation("2006-01-02", effectiveDate.Format("2006-01-02"), zone)
	if err != nil {
		return false, err
	}
	effectiveAt := localDate.UTC()
	if !effectiveAt.After(sourceFrom) {
		return block("DATE_CONFLICT")
	}
	var regionID, divisionID, departmentID, grade string
	err = tx.QueryRowContext(ctx, `SELECT o.region_id,d.division_id,p.department_id,COALESCE(p.grade,'') FROM offices o JOIN regions r ON r.tenant_id=o.tenant_id AND r.id=o.region_id JOIN positions p ON p.tenant_id=o.tenant_id AND p.id=? JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id JOIN divisions v ON v.tenant_id=d.tenant_id AND v.id=d.division_id WHERE o.tenant_id=? AND o.id=? AND o.is_active=TRUE AND o.deleted_at IS NULL AND r.deleted_at IS NULL AND p.deleted_at IS NULL AND d.deleted_at IS NULL AND v.deleted_at IS NULL`, positionID, tenantID, officeID).Scan(&regionID, &divisionID, &departmentID, &grade)
	if errors.Is(err, sql.ErrNoRows) {
		return block("ORGANIZATION_UNAVAILABLE")
	}
	if err != nil {
		return false, err
	}
	if managerID.Valid {
		var managerStatus string
		if err = tx.QueryRowContext(ctx, `SELECT status FROM employees WHERE tenant_id=? AND id=? AND deleted_at IS NULL`, tenantID, managerID.String).Scan(&managerStatus); err != nil || managerStatus == "resigned" || managerStatus == "terminated" {
			return block("MANAGER_INVALID")
		}
		manager := managerID.String
		for depth := 0; depth < 100 && manager != ""; depth++ {
			if manager == employeeID {
				return block("MANAGER_CYCLE")
			}
			var next sql.NullString
			if err = tx.QueryRowContext(ctx, `SELECT reports_to_id FROM employees WHERE tenant_id=? AND id=?`, tenantID, manager).Scan(&next); err != nil {
				return block("MANAGER_INVALID")
			}
			manager = next.String
		}
		if manager != "" {
			return block("MANAGER_CYCLE")
		}
	}
	var conflicts int
	if err = tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM work_schedules WHERE tenant_id=? AND employee_id=? AND status='published' AND work_date>=?)+(SELECT COUNT(*) FROM payroll_runs pr JOIN payroll_periods pp ON pp.tenant_id=pr.tenant_id AND pp.id=pr.period_id WHERE pr.tenant_id=? AND pr.status='finalized' AND pp.end_date>=?)`, tenantID, employeeID, effectiveDate, tenantID, effectiveDate).Scan(&conflicts); err != nil {
		return false, err
	}
	if conflicts != 0 {
		return block("HISTORY_LOCKED")
	}
	endAt := effectiveAt.Add(-time.Microsecond)
	if _, err = tx.ExecContext(ctx, `UPDATE employee_assignments SET valid_to=?,current_flag=NULL WHERE tenant_id=? AND id=? AND current_flag=1`, endAt, tenantID, sourceID); err != nil {
		return false, err
	}
	if historyID.Valid {
		if _, err = tx.ExecContext(ctx, `UPDATE employee_assignment_history SET valid_until=? WHERE tenant_id=? AND id=?`, endAt, tenantID, historyID.String); err != nil {
			return false, err
		}
	}
	newHistoryID, err := ids.New()
	if err != nil {
		return false, err
	}
	assignmentID, err := ids.New()
	if err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO employee_assignment_history (tenant_id,id,employee_id,base_office_id,position_id,reports_to_id,valid_from) VALUES (?,?,?,?,?,?,?)`, tenantID, newHistoryID, employeeID, officeID, positionID, managerID, effectiveAt); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,valid_from,current_flag,source_history_id,created_by) VALUES (?,?,?,?,?,?,?,?,?,?,?,1,?,?)`, tenantID, assignmentID, employeeID, regionID, officeID, divisionID, departmentID, positionID, managerID, nullID(grade), effectiveAt, newHistoryID, actorID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE employees SET base_office_id=?,position_id=?,reports_to_id=?,version=version+1 WHERE tenant_id=? AND id=?`, officeID, positionID, managerID, tenantID, employeeID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE employee_changes SET status='applied',applied_assignment_id=?,applied_at=UTC_TIMESTAMP(6),apply_error=NULL,version=version+1 WHERE tenant_id=? AND id=? AND status='approved'`, assignmentID, tenantID, id); err != nil {
		return false, err
	}
	outboxID, err := ids.New()
	if err != nil {
		return false, err
	}
	payload, _ := json.Marshal(map[string]any{"employee_change_id": id, "employee_id": employeeID, "request_id": requestID, "assignment_id": assignmentID, "change_type": kind})
	if _, err = tx.ExecContext(ctx, `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,'employee_change.applied','employee_change',?,?,?)`, tenantID, outboxID, id, string(payload), "employee_change.applied:"+id); err != nil {
		return false, err
	}
	if err = audit(ctx, tx, tenantID, actorID, "apply_system", "employee_changes", id, map[string]any{"status": "approved", "source_assignment_id": sourceID}, map[string]any{"status": "applied", "assignment_id": assignmentID, "effective_date": effectiveDate.Format("2006-01-02"), "applied_by": "system"}); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
