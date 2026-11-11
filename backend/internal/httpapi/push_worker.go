package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/ids"
)

func queuePushDelivery(ctx context.Context, tx *sql.Tx, tenantID, userID, notificationID, category string) error {
	var enabled bool
	err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT enabled FROM notification_preferences WHERE tenant_id=? AND user_id=? AND category=? AND channel='push'),TRUE)`, tenantID, userID, category).Scan(&enabled)
	if err != nil || !enabled {
		return err
	}
	var devices int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM push_subscriptions WHERE tenant_id=? AND user_id=? AND revoked_at IS NULL`, tenantID, userID).Scan(&devices); err != nil || devices == 0 {
		return err
	}
	var existing int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_deliveries WHERE tenant_id=? AND notification_id=? AND channel='push'`, tenantID, notificationID).Scan(&existing); err != nil || existing > 0 {
		return err
	}
	id, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_deliveries (tenant_id,id,notification_id,channel,status,next_attempt_at) VALUES (?,?,?,'push','pending',UTC_TIMESTAMP(6))`, tenantID, id, notificationID)
	return err
}

func RunPushDeliveries(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT tenant_id,id FROM notification_deliveries WHERE channel='push' AND ((status='pending' AND (next_attempt_at IS NULL OR next_attempt_at<=UTC_TIMESTAMP(6))) OR (status='processing' AND attempted_at<UTC_TIMESTAMP(6)-INTERVAL 5 MINUTE)) ORDER BY attempted_at,id LIMIT 100`)
	if err != nil {
		return 0, err
	}
	type key struct{ tenant, id string }
	keys := []key{}
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.tenant, &k.id); err != nil {
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
	processed := 0
	for _, k := range keys {
		ok, err := processPushDelivery(ctx, db, k.tenant, k.id)
		if err != nil {
			return processed, err
		}
		if ok {
			processed++
		}
	}
	return processed, nil
}

func processPushDelivery(ctx context.Context, db *sql.DB, tenantID, id string) (bool, error) {
	res, err := db.ExecContext(ctx, `UPDATE notification_deliveries SET status='processing',attempted_at=UTC_TIMESTAMP(6),attempt_no=attempt_no+1 WHERE tenant_id=? AND id=? AND channel='push' AND (status='pending' OR (status='processing' AND attempted_at<UTC_TIMESTAMP(6)-INTERVAL 5 MINUTE))`, tenantID, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}
	var userID, notificationID, title string
	var link sql.NullString
	err = db.QueryRowContext(ctx, `SELECT n.recipient_user_id,n.id,n.title,n.deep_link FROM notification_deliveries d JOIN notifications n ON n.tenant_id=d.tenant_id AND n.id=d.notification_id WHERE d.tenant_id=? AND d.id=?`, tenantID, id).Scan(&userID, &notificationID, &title, &link)
	if err != nil {
		return false, err
	}
	rows, err := db.QueryContext(ctx, `SELECT id FROM push_subscriptions WHERE tenant_id=? AND user_id=? AND revoked_at IS NULL ORDER BY id`, tenantID, userID)
	if err != nil {
		return false, err
	}
	devices := []string{}
	for rows.Next() {
		var device string
		if err := rows.Scan(&device); err != nil {
			rows.Close()
			return false, err
		}
		devices = append(devices, device)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost", nil)
	delivered := 0
	for _, device := range devices {
		if err := sendPushToSubscription(req, db, tenantID, userID, device, pushPayload(title, link.String)); err == nil {
			delivered++
		}
	}
	status := "failed"
	code := "PUSH_DELIVERY_FAILED"
	var deliveredAt any
	if delivered > 0 {
		status = "delivered"
		code = ""
		deliveredAt = time.Now().UTC()
	}
	var next any
	if status == "failed" {
		var attempt int
		if err := db.QueryRowContext(ctx, `SELECT attempt_no FROM notification_deliveries WHERE tenant_id=? AND id=?`, tenantID, id).Scan(&attempt); err != nil {
			return false, err
		}
		if attempt < 8 {
			status = "pending"
			next = time.Now().UTC().Add(time.Duration(1<<min(attempt, 6)) * time.Minute)
		}
	}
	_, err = db.ExecContext(ctx, `UPDATE notification_deliveries SET status=?,error_code=?,next_attempt_at=?,delivered_at=? WHERE tenant_id=? AND id=? AND status='processing'`, status, code, next, deliveredAt, tenantID, id)
	return true, err
}

func retryPushDelivery(ctx context.Context, db *sql.DB, tenantID, id string) error {
	res, err := db.ExecContext(ctx, `UPDATE notification_deliveries SET status='pending',next_attempt_at=UTC_TIMESTAMP(6),error_code=NULL WHERE tenant_id=? AND id=? AND channel='push' AND status='failed'`, tenantID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("not retryable")
	}
	return nil
}
