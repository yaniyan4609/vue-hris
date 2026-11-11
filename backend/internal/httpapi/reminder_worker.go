package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"digaji.id/hris/internal/ids"
)

func RunDueReminders(ctx context.Context, db *sql.DB) (int, error) {
	if err := planShiftReminders(ctx, db); err != nil {
		return 0, err
	}
	rows, err := db.QueryContext(ctx, `SELECT tenant_id,id FROM notification_schedules WHERE status='pending' AND fire_at<=UTC_TIMESTAMP(6) ORDER BY fire_at,id LIMIT 100`)
	if err != nil {
		return 0, err
	}
	type key struct{ Tenant, ID string }
	keys := []key{}
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.Tenant, &k.ID); err != nil {
			rows.Close()
			return 0, err
		}
		keys = append(keys, k)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, k := range keys {
		ok, err := sendDueReminder(ctx, db, k.Tenant, k.ID)
		if err != nil {
			return count, err
		}
		if ok {
			count++
		}
	}
	return count, nil
}

func planShiftReminders(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT i.tenant_id,i.id,e.user_id,n.id,n.event_type,n.config_json,i.clock_in_window_start_at,i.clock_in_window_end_at,i.clock_out_window_start_at,i.clock_out_window_end_at,t.timezone FROM shift_instances i JOIN employees e ON e.tenant_id=i.tenant_id AND e.id=i.employee_id JOIN tenants t ON t.id=i.tenant_id JOIN notification_rules n ON n.tenant_id=i.tenant_id AND n.enabled=TRUE AND n.event_type IN ('attendance.clock_in_reminder','attendance.clock_out_reminder') WHERE i.status IN ('scheduled','open') AND i.expected_start BETWEEN UTC_TIMESTAMP(6)-INTERVAL 1 DAY AND UTC_TIMESTAMP(6)+INTERVAL 2 DAY AND e.user_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM notification_schedules s WHERE s.tenant_id=i.tenant_id AND s.subject_type='shift_instance' AND s.subject_id=i.id AND s.rule_id=n.id AND s.attempt_no=0) ORDER BY i.expected_start LIMIT 1000`)
	if err != nil {
		return err
	}
	type item struct {
		tenant, instance, user, rule, event, zone string
		raw                                       []byte
		inStart, inEnd, outStart, outEnd          time.Time
	}
	items := []item{}
	for rows.Next() {
		var v item
		if err := rows.Scan(&v.tenant, &v.instance, &v.user, &v.rule, &v.event, &v.raw, &v.inStart, &v.inEnd, &v.outStart, &v.outEnd, &v.zone); err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range items {
		var cfg notificationRuleConfig
		if json.Unmarshal(v.raw, &cfg) != nil {
			continue
		}
		base := v.inStart
		end := v.inEnd
		if v.event == "attendance.clock_out_reminder" {
			base = v.outStart
			end = v.outEnd
		}
		fire := base.Add(time.Duration(cfg.OffsetMinutes) * time.Minute)
		fire = afterQuietHours(fire, v.zone, cfg)
		if fire.Before(time.Now().UTC()) {
			fire = time.Now().UTC()
		}
		if fire.After(end) {
			continue
		}
		id, err := ids.New()
		if err != nil {
			return err
		}
		dedupe := fmt.Sprintf("reminder:%s:%s:0", v.instance, v.event)
		_, err = db.ExecContext(ctx, `INSERT IGNORE INTO notification_schedules (tenant_id,id,rule_id,subject_type,subject_id,recipient_user_id,fire_at,attempt_no,status,dedupe_key) VALUES (?,?,?,'shift_instance',?,?,?,0,'pending',?)`, v.tenant, id, v.rule, v.instance, v.user, fire.UTC(), dedupe)
		if err != nil {
			return err
		}
	}
	return nil
}

func afterQuietHours(at time.Time, timezone string, cfg notificationRuleConfig) time.Time {
	if cfg.QuietStart == "" || cfg.QuietEnd == "" {
		return at
	}
	zone, err := time.LoadLocation(timezone)
	if err != nil {
		zone = time.UTC
	}
	local := at.In(zone)
	start, _ := time.Parse("15:04", cfg.QuietStart)
	end, _ := time.Parse("15:04", cfg.QuietEnd)
	minute := local.Hour()*60 + local.Minute()
	from := start.Hour()*60 + start.Minute()
	to := end.Hour()*60 + end.Minute()
	quiet := false
	if from < to {
		quiet = minute >= from && minute < to
	} else if from > to {
		quiet = minute >= from || minute < to
	}
	if !quiet {
		return at
	}
	endDay := local
	if from > to && minute >= from {
		endDay = endDay.AddDate(0, 0, 1)
	}
	return time.Date(endDay.Year(), endDay.Month(), endDay.Day(), end.Hour(), end.Minute(), 0, 0, zone).UTC()
}

func sendDueReminder(ctx context.Context, db *sql.DB, tenantID, id string) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var ruleID, instanceID, userID, event, zone, status string
	var ruleEnabled bool
	var attempt int
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT s.rule_id,s.subject_id,s.recipient_user_id,s.attempt_no,s.status,n.event_type,n.config_json,n.enabled,t.timezone FROM notification_schedules s JOIN notification_rules n ON n.tenant_id=s.tenant_id AND n.id=s.rule_id JOIN tenants t ON t.id=s.tenant_id WHERE s.tenant_id=? AND s.id=? FOR UPDATE`, tenantID, id).Scan(&ruleID, &instanceID, &userID, &attempt, &status, &event, &raw, &ruleEnabled, &zone)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if status != "pending" {
		return false, nil
	}
	if !ruleEnabled {
		_, err = tx.ExecContext(ctx, `UPDATE notification_schedules SET status='cancelled' WHERE tenant_id=? AND id=?`, tenantID, id)
		if err != nil {
			return false, err
		}
		return false, tx.Commit()
	}
	var cfg notificationRuleConfig
	if json.Unmarshal(raw, &cfg) != nil {
		return false, errors.New("invalid reminder rule")
	}
	var instanceStatus string
	var inEnd, outEnd time.Time
	var firstIn, lastOut sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT i.status,i.clock_in_window_end_at,i.clock_out_window_end_at,d.first_in,d.last_out FROM shift_instances i LEFT JOIN attendance_daily d ON d.tenant_id=i.tenant_id AND d.shift_instance_id=i.id WHERE i.tenant_id=? AND i.id=?`, tenantID, instanceID).Scan(&instanceStatus, &inEnd, &outEnd, &firstIn, &lastOut)
	if errors.Is(err, sql.ErrNoRows) {
		instanceStatus = "cancelled"
	} else if err != nil {
		return false, err
	}
	completed := instanceStatus == "cancelled" || instanceStatus == "completed" || event == "attendance.clock_in_reminder" && firstIn.Valid || event == "attendance.clock_out_reminder" && lastOut.Valid
	end := inEnd
	if event == "attendance.clock_out_reminder" {
		end = outEnd
	}
	if completed || time.Now().UTC().After(end) {
		_, err = tx.ExecContext(ctx, `UPDATE notification_schedules SET status='cancelled' WHERE tenant_id=? AND id=?`, tenantID, id)
		if err != nil {
			return false, err
		}
		return false, tx.Commit()
	}
	var memberStatus string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, tenantID, userID).Scan(&memberStatus); err != nil {
		return false, err
	}
	if memberStatus != "active" {
		_, err = tx.ExecContext(ctx, `UPDATE notification_schedules SET status='cancelled' WHERE tenant_id=? AND id=?`, tenantID, id)
		if err != nil {
			return false, err
		}
		return false, tx.Commit()
	}
	title := "Pengingat presensi masuk"
	if event == "attendance.clock_out_reminder" {
		title = "Pengingat presensi pulang"
	}
	var notificationID string
	notificationID, err = ids.New()
	if err != nil {
		return false, err
	}
	dedupe := fmt.Sprintf("reminder:%s:%s:%d", instanceID, event, attempt)
	messageTitle, messageBody, err := renderedNotification(ctx, tx, tenantID, event, "inapp", title)
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO notifications (tenant_id,id,recipient_user_id,category,event_type,title,body,deep_link,dedupe_key) VALUES (?,?,?,'reminder',?,?,?,'/ess/absensi',?)`, tenantID, notificationID, userID, event, messageTitle, messageBody, dedupe)
	if err != nil {
		return false, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM notifications WHERE tenant_id=? AND recipient_user_id=? AND dedupe_key=?`, tenantID, userID, dedupe).Scan(&notificationID); err != nil {
		return false, err
	}
	deliveryID, err := ids.New()
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_deliveries (tenant_id,id,notification_id,channel,status,delivered_at) SELECT ?,?,?,'inapp','delivered',UTC_TIMESTAMP(6) WHERE NOT EXISTS (SELECT 1 FROM notification_deliveries WHERE tenant_id=? AND notification_id=? AND channel='inapp')`, tenantID, deliveryID, notificationID, tenantID, notificationID)
	if err != nil {
		return false, err
	}
	if err = queuePushDelivery(ctx, tx, tenantID, userID, notificationID, "reminder"); err != nil {
		return false, err
	}
	if err = queueEmailDelivery(ctx, tx, tenantID, userID, notificationID, "reminder"); err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE notification_schedules SET status='sent' WHERE tenant_id=? AND id=? AND status='pending'`, tenantID, id)
	if err != nil {
		return false, err
	}
	if attempt < cfg.MaxRepeats && cfg.RepeatMinutes > 0 {
		fire := afterQuietHours(time.Now().UTC().Add(time.Duration(cfg.RepeatMinutes)*time.Minute), zone, cfg)
		if fire.Before(end) {
			nextID, e := ids.New()
			if e != nil {
				return false, e
			}
			nextKey := fmt.Sprintf("reminder:%s:%s:%d", instanceID, event, attempt+1)
			_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO notification_schedules (tenant_id,id,rule_id,subject_type,subject_id,recipient_user_id,fire_at,attempt_no,status,dedupe_key) VALUES (?,?,?,'shift_instance',?,?,?,?,'pending',?)`, tenantID, nextID, ruleID, instanceID, userID, fire, attempt+1, nextKey)
			if err != nil {
				return false, err
			}
		}
	}
	return true, tx.Commit()
}
