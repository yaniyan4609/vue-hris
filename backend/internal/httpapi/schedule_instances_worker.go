package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// RunUpcomingShiftInstances fills gaps in published rosters for the next 30
// tenant-local days. Publish still creates instances immediately; this worker
// covers legacy rows and interrupted materialization without duplicating them.
func RunUpcomingShiftInstances(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT w.tenant_id,w.id FROM work_schedules w JOIN tenants t ON t.id=w.tenant_id LEFT JOIN shift_instances i ON i.tenant_id=w.tenant_id AND i.work_schedule_id=w.id WHERE t.status='active' AND w.status='published' AND w.day_type='shift' AND w.created_by IS NOT NULL AND i.id IS NULL AND w.work_date>=UTC_DATE()-INTERVAL 1 DAY AND w.work_date<=UTC_DATE()+INTERVAL 32 DAY ORDER BY w.work_date,w.tenant_id,w.id LIMIT 201`)
	if err != nil {
		return 0, err
	}
	type job struct{ tenantID, id string }
	jobs := []job{}
	for rows.Next() {
		var x job
		if err = rows.Scan(&x.tenantID, &x.id); err != nil {
			break
		}
		jobs = append(jobs, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, x := range jobs {
		done, e := materializeDueRoster(ctx, db, x.tenantID, x.id)
		if e != nil {
			return processed, e
		}
		if done {
			processed++
		}
	}
	return processed, nil
}

func materializeDueRoster(ctx context.Context, db *sql.DB, tenantID, id string) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	item, err := scanWorkSchedule(tx.QueryRowContext(ctx, `SELECT `+strings.ReplaceAll(rosterColumns, "w.", "")+` FROM work_schedules WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if item.Status != "published" || item.DayType != "shift" {
		return false, nil
	}
	var existing int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM shift_instances WHERE tenant_id=? AND work_schedule_id=?`, tenantID, id).Scan(&existing); err != nil {
		return false, err
	}
	if existing > 0 {
		return false, nil
	}
	var zoneName, actorID string
	if err = tx.QueryRowContext(ctx, `SELECT t.timezone,w.created_by FROM work_schedules w JOIN tenants t ON t.id=w.tenant_id WHERE w.tenant_id=? AND w.id=?`, tenantID, id).Scan(&zoneName, &actorID); err != nil {
		return false, err
	}
	zone, err := time.LoadLocation(zoneName)
	if err != nil {
		return false, err
	}
	today := time.Now().In(zone)
	date, err := time.Parse("2006-01-02", item.WorkDate)
	if err != nil {
		return false, err
	}
	start, _ := time.Parse("2006-01-02", today.Format("2006-01-02"))
	if date.Before(start) || date.After(start.AddDate(0, 0, 30)) {
		return false, nil
	}
	instance, err := materializeRosterShift(ctx, tx, tenantID, item)
	if err != nil {
		return false, err
	}
	if err = audit(ctx, tx, tenantID, actorID, "generate", "shift_instances", instance.ID, nil, map[string]any{"roster_id": id, "work_date": item.WorkDate, "source": "worker"}); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
