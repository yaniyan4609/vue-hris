package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"digaji.id/hris/internal/ids"
)

// RunDueTerminations applies approved exits using each tenant's local date.
// A per-record transaction makes retries safe across API processes.
func RunDueTerminations(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT x.tenant_id,x.id,t.timezone FROM terminations x JOIN tenants t ON t.id=x.tenant_id WHERE x.status='approved' AND x.request_id IS NOT NULL AND x.source_assignment_id IS NOT NULL AND x.created_by IS NOT NULL ORDER BY (x.apply_error IS NOT NULL),x.effective_date,x.id LIMIT 500`)
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
	count := 0
	for _, item := range items {
		done, e := applyDueTermination(ctx, db, item.tenantID, item.id, item.zone)
		if e != nil {
			return count, e
		}
		if done {
			count++
		}
	}
	return count, nil
}

func applyDueTermination(ctx context.Context, db *sql.DB, tenantID, id, zoneName string) (bool, error) {
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		return false, err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var employeeID, employeeStatus string
	var deleted sql.NullTime
	if err = tx.QueryRowContext(ctx, `SELECT id,status,deleted_at FROM employees WHERE tenant_id=? AND id=(SELECT employee_id FROM terminations WHERE tenant_id=? AND id=?) FOR UPDATE`, tenantID, tenantID, id).Scan(&employeeID, &employeeStatus, &deleted); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var status, kind, sourceID, actorID, requestID string
	var effectiveDate time.Time
	err = tx.QueryRowContext(ctx, `SELECT status,termination_type,source_assignment_id,created_by,request_id,effective_date FROM terminations WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, id).Scan(&status, &kind, &sourceID, &actorID, &requestID, &effectiveDate)
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
		_, e := tx.ExecContext(ctx, `UPDATE terminations SET apply_error=? WHERE tenant_id=? AND id=? AND status='approved'`, code, tenantID, id)
		if e != nil {
			return false, e
		}
		return false, tx.Commit()
	}
	if deleted.Valid || employeeStatus == "resigned" || employeeStatus == "terminated" {
		return block("EMPLOYEE_INACTIVE")
	}
	var currentID string
	var sourceFrom time.Time
	var historyID sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id,valid_from,source_history_id FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=1 FOR UPDATE`, tenantID, employeeID).Scan(&currentID, &sourceFrom, &historyID)
	if errors.Is(err, sql.ErrNoRows) || currentID != sourceID {
		return block("ASSIGNMENT_CHANGED")
	}
	if err != nil {
		return false, err
	}
	localDate, err := time.ParseInLocation("2006-01-02", effectiveDate.Format("2006-01-02"), zone)
	if err != nil {
		return false, err
	}
	effectiveAt := localDate.UTC()
	if effectiveDate.Format("2006-01-02") == time.Now().In(zone).Format("2006-01-02") && !effectiveAt.After(sourceFrom) {
		if err = tx.QueryRowContext(ctx, `SELECT UTC_TIMESTAMP(6)`).Scan(&effectiveAt); err != nil {
			return false, err
		}
	}
	if !effectiveAt.After(sourceFrom) {
		return block("DATE_CONFLICT")
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
	newStatus := "terminated"
	if kind == "resign" || kind == "retirement" {
		newStatus = "resigned"
	}
	var userID sql.NullString
	if err = tx.QueryRowContext(ctx, `SELECT user_id FROM employees WHERE tenant_id=? AND id=?`, tenantID, employeeID).Scan(&userID); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE employees SET status=?,end_date=?,version=version+1 WHERE tenant_id=? AND id=?`, newStatus, effectiveDate, tenantID, employeeID); err != nil {
		return false, err
	}
	if userID.Valid {
		if _, err = tx.ExecContext(ctx, `UPDATE user_roles SET revoked_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND user_id=? AND revoked_at IS NULL`, tenantID, userID.String); err != nil {
			return false, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE tenant_memberships SET status='inactive',ended_at=UTC_TIMESTAMP(6),access_version=access_version+1 WHERE tenant_id=? AND user_id=? AND status='active'`, tenantID, userID.String); err != nil {
			return false, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=UTC_TIMESTAMP(6),revoked_reason='resign' WHERE user_id=? AND revoked_at IS NULL`, userID.String); err != nil {
			return false, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE terminations SET status='processed',processed_at=UTC_TIMESTAMP(6),apply_error=NULL,version=version+1 WHERE tenant_id=? AND id=? AND status='approved'`, tenantID, id); err != nil {
		return false, err
	}
	for _, event := range []string{"termination.processed", "payroll.final_requested"} {
		outboxID, e := ids.New()
		if e != nil {
			return false, e
		}
		payload, _ := json.Marshal(map[string]any{"termination_id": id, "employee_id": employeeID, "request_id": requestID, "effective_date": effectiveDate.Format("2006-01-02"), "termination_type": kind})
		if _, err = tx.ExecContext(ctx, `INSERT INTO notification_outbox (tenant_id,id,event_type,aggregate_type,aggregate_id,payload_json,dedupe_key) VALUES (?,?,?,'termination',?,?,?)`, tenantID, outboxID, event, id, string(payload), event+":"+id); err != nil {
			return false, err
		}
	}
	if err = audit(ctx, tx, tenantID, actorID, "apply_system", "terminations", id, map[string]any{"status": "approved"}, map[string]any{"status": "processed", "employee_status": newStatus, "effective_date": effectiveDate.Format("2006-01-02")}); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
