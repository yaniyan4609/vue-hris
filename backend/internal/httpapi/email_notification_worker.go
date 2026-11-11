package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
)

func queueEmailDelivery(ctx context.Context, tx *sql.Tx, tenantID, userID, notificationID, category string) error {
	var enabled bool
	err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT enabled FROM notification_preferences WHERE tenant_id=? AND user_id=? AND category=? AND channel='email'),FALSE)`, tenantID, userID, category).Scan(&enabled)
	if err != nil || !enabled {
		return err
	}
	var existing int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_deliveries WHERE tenant_id=? AND notification_id=? AND channel='email'`, tenantID, notificationID).Scan(&existing); err != nil || existing > 0 {
		return err
	}
	id, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_deliveries (tenant_id,id,notification_id,channel,status,next_attempt_at) VALUES (?,?,?,'email','pending',UTC_TIMESTAMP(6))`, tenantID, id, notificationID)
	return err
}

func RunEmailDeliveries(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT tenant_id,id FROM notification_deliveries WHERE channel='email' AND ((status='pending' AND (next_attempt_at IS NULL OR next_attempt_at<=UTC_TIMESTAMP(6))) OR (status='processing' AND attempted_at<UTC_TIMESTAMP(6)-INTERVAL 5 MINUTE)) ORDER BY attempted_at,id LIMIT 50`)
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
		ok, err := processEmailDelivery(ctx, db, k.Tenant, k.ID)
		if err != nil {
			return count, err
		}
		if ok {
			count++
		}
	}
	return count, nil
}

func processEmailDelivery(ctx context.Context, db *sql.DB, tenantID, id string) (bool, error) {
	res, err := db.ExecContext(ctx, `UPDATE notification_deliveries SET status='processing',attempted_at=UTC_TIMESTAMP(6),attempt_no=attempt_no+1 WHERE tenant_id=? AND id=? AND channel='email' AND (status='pending' OR (status='processing' AND attempted_at<UTC_TIMESTAMP(6)-INTERVAL 5 MINUTE))`, tenantID, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}
	var address, event, title, link string
	err = db.QueryRowContext(ctx, `SELECT u.email,n.event_type,n.title,COALESCE(n.deep_link,'') FROM notification_deliveries d JOIN notifications n ON n.tenant_id=d.tenant_id AND n.id=d.notification_id JOIN users u ON u.id=n.recipient_user_id WHERE d.tenant_id=? AND d.id=?`, tenantID, id).Scan(&address, &event, &title, &link)
	if err != nil {
		return false, err
	}
	subject, body, err := renderedNotification(ctx, db, tenantID, event, "email", title)
	if err != nil {
		return false, err
	}
	if link != "" && strings.HasPrefix(link, "/") && !strings.HasPrefix(link, "//") {
		body += "\n\n" + strings.TrimRight(os.Getenv("APP_ORIGIN"), "/") + link
	}
	deliveryErr := errors.New("smtp unavailable")
	if smtpConfigured() {
		deliveryErr = sendSMTPText(ctx, address, subject, body)
	}
	status := "delivered"
	code := ""
	var next, delivered any = time.Now().UTC(), time.Now().UTC()
	if deliveryErr != nil {
		status = "failed"
		code = "EMAIL_DELIVERY_FAILED"
		delivered = nil
		next = nil
		var attempts int
		if err := db.QueryRowContext(ctx, `SELECT attempt_no FROM notification_deliveries WHERE tenant_id=? AND id=?`, tenantID, id).Scan(&attempts); err != nil {
			return false, err
		}
		if attempts < 8 {
			status = "pending"
			next = time.Now().UTC().Add(time.Duration(1<<min(attempts, 6)) * time.Minute)
		}
	}
	_, err = db.ExecContext(ctx, `UPDATE notification_deliveries SET status=?,error_code=?,next_attempt_at=?,delivered_at=? WHERE tenant_id=? AND id=? AND status='processing'`, status, code, next, delivered, tenantID, id)
	return true, err
}
