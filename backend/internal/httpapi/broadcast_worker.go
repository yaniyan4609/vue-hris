package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
)

func RunDueBroadcasts(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT tenant_id,id FROM broadcasts WHERE status='sending' OR (status IN ('queued','scheduled') AND send_at<=UTC_TIMESTAMP(6)) ORDER BY send_at,id LIMIT 20`)
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
	processed := 0
	for _, k := range keys {
		ok, err := processDueBroadcast(ctx, db, k.Tenant, k.ID)
		if err != nil {
			return processed, err
		}
		if ok {
			processed++
		}
	}
	return processed, nil
}

func processDueBroadcast(ctx context.Context, db *sql.DB, tenantID, id string) (bool, error) {
	api := adminAPI{db: db, access: access.Store{DB: db}}
	var status, sender, timezone string
	err := db.QueryRowContext(ctx, `SELECT b.status,b.sent_by,t.timezone FROM broadcasts b JOIN tenants t ON t.id=b.tenant_id WHERE b.tenant_id=? AND b.id=?`, tenantID, id).Scan(&status, &sender, &timezone)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if status == "queued" || status == "scheduled" {
		var mode string
		err = db.QueryRowContext(ctx, `SELECT combine_mode FROM broadcasts WHERE tenant_id=? AND id=?`, tenantID, id).Scan(&mode)
		if err != nil {
			return false, err
		}
		rows, err := db.QueryContext(ctx, `SELECT target_type,ref_id FROM broadcast_targets WHERE tenant_id=? AND broadcast_id=? ORDER BY id`, tenantID, id)
		if err != nil {
			return false, err
		}
		targets := []broadcastTarget{}
		for rows.Next() {
			var t broadcastTarget
			var ref sql.NullString
			if err := rows.Scan(&t.Type, &ref); err != nil {
				rows.Close()
				return false, err
			}
			if ref.Valid {
				t.RefID = &ref.String
			}
			targets = append(targets, t)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return false, err
		}
		_, permission, err := (access.Store{DB: db}).Decide(ctx, tenantID, sender, timezone, "broadcast.kirim")
		if err != nil {
			return false, err
		}
		allowed := permission.Allowed
		for _, t := range targets {
			if t.Type == "all" {
				_, decision, e := (access.Store{DB: db}).Decide(ctx, tenantID, sender, timezone, "broadcast.target_semua")
				if e != nil {
					return false, e
				}
				allowed = allowed && decision.Allowed
			}
		}
		var audience broadcastAudience
		if allowed {
			audience, err = api.resolveBroadcastAudience(ctx, tenantID, sender, timezone, targets, mode)
			if err != nil {
				return false, err
			}
		}
		if !allowed || audience.RecipientsCount == 0 {
			_, err = db.ExecContext(ctx, `UPDATE broadcasts SET status='failed',completed_at=UTC_TIMESTAMP(6),version=version+1 WHERE tenant_id=? AND id=? AND status IN ('queued','scheduled')`, tenantID, id)
			return err == nil, err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return false, err
		}
		defer tx.Rollback()
		var current string
		if err = tx.QueryRowContext(ctx, `SELECT status FROM broadcasts WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, id).Scan(&current); err != nil {
			return false, err
		}
		if current != "queued" && current != "scheduled" {
			return false, nil
		}
		for _, recipient := range audience.Recipients {
			recipientID, e := ids.New()
			if e != nil {
				return false, e
			}
			_, e = tx.ExecContext(ctx, `INSERT IGNORE INTO broadcast_recipients (tenant_id,id,broadcast_id,user_id,employee_id) VALUES (?,?,?,?,?)`, tenantID, recipientID, id, recipient.UserID, recipient.EmployeeID)
			if e != nil {
				return false, e
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE broadcasts SET status='sending',snapshot_at=UTC_TIMESTAMP(6),recipients_count=?,version=version+1 WHERE tenant_id=? AND id=?`, audience.RecipientsCount, tenantID, id)
		if err != nil {
			return false, err
		}
		if err = tx.Commit(); err != nil {
			return false, err
		}
	}
	return deliverBroadcastBatch(ctx, db, tenantID, id)
}

func deliverBroadcastBatch(ctx context.Context, db *sql.DB, tenantID, id string) (bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,user_id FROM broadcast_recipients WHERE tenant_id=? AND broadcast_id=? AND delivery_status='pending' ORDER BY id LIMIT 100`, tenantID, id)
	if err != nil {
		return false, err
	}
	type recipient struct{ ID, UserID string }
	recipients := []recipient{}
	for rows.Next() {
		var item recipient
		if err := rows.Scan(&item.ID, &item.UserID); err != nil {
			rows.Close()
			return false, err
		}
		recipients = append(recipients, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	for _, item := range recipients {
		if err := deliverBroadcastRecipient(ctx, db, tenantID, id, item.ID, item.UserID); err != nil {
			return false, err
		}
	}
	var pending int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM broadcast_recipients WHERE tenant_id=? AND broadcast_id=? AND delivery_status='pending'`, tenantID, id).Scan(&pending)
	if err != nil {
		return false, err
	}
	if pending == 0 {
		_, err = db.ExecContext(ctx, `UPDATE broadcasts SET status='sent',completed_at=UTC_TIMESTAMP(6),version=version+1 WHERE tenant_id=? AND id=? AND status='sending'`, tenantID, id)
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func deliverBroadcastRecipient(ctx context.Context, db *sql.DB, tenantID, broadcastID, recipientID, userID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status, title string
	err = tx.QueryRowContext(ctx, `SELECT status,title FROM broadcasts WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, broadcastID).Scan(&status, &title)
	if err != nil {
		return err
	}
	if status != "sending" {
		return nil
	}
	var recipientStatus string
	err = tx.QueryRowContext(ctx, `SELECT delivery_status FROM broadcast_recipients WHERE tenant_id=? AND id=? AND broadcast_id=? AND user_id=? FOR UPDATE`, tenantID, recipientID, broadcastID, userID).Scan(&recipientStatus)
	if err != nil {
		return err
	}
	if recipientStatus != "pending" {
		return nil
	}
	var memberStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, tenantID, userID).Scan(&memberStatus)
	if err != nil {
		return err
	}
	if memberStatus != "active" {
		_, err = tx.ExecContext(ctx, `UPDATE broadcast_recipients SET delivery_status='failed' WHERE tenant_id=? AND id=?`, tenantID, recipientID)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE broadcasts SET failed_count=failed_count+1 WHERE tenant_id=? AND id=?`, tenantID, broadcastID)
		}
		if err != nil {
			return err
		}
		if err = emitBroadcastProgress(ctx, tx, tenantID, broadcastID); err != nil {
			return err
		}
		return tx.Commit()
	}
	notificationID, err := ids.New()
	if err != nil {
		return err
	}
	dedupe := fmt.Sprintf("broadcast:%s", broadcastID)
	_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO notifications (tenant_id,id,recipient_user_id,category,event_type,title,body,deep_link,dedupe_key) VALUES (?,?,?,'broadcast','broadcast.published',?,'Pengumuman baru tersedia.',?,?)`, tenantID, notificationID, userID, title, "/ess/announcements/"+broadcastID, dedupe)
	if err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM notifications WHERE tenant_id=? AND recipient_user_id=? AND dedupe_key=?`, tenantID, userID, dedupe).Scan(&notificationID); err != nil {
		return err
	}
	deliveryID, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_deliveries (tenant_id,id,notification_id,channel,status,delivered_at) SELECT ?,?,?,'inapp','delivered',UTC_TIMESTAMP(6) WHERE NOT EXISTS (SELECT 1 FROM notification_deliveries WHERE tenant_id=? AND notification_id=? AND channel='inapp')`, tenantID, deliveryID, notificationID, tenantID, notificationID)
	if err != nil {
		return err
	}
	if err = queuePushDelivery(ctx, tx, tenantID, userID, notificationID, "broadcast"); err != nil {
		return err
	}
	if err = queueEmailDelivery(ctx, tx, tenantID, userID, notificationID, "broadcast"); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE broadcast_recipients SET notification_id=?,delivery_status='delivered' WHERE tenant_id=? AND id=? AND delivery_status='pending'`, notificationID, tenantID, recipientID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE broadcasts SET processed_count=processed_count+1 WHERE tenant_id=? AND id=?`, tenantID, broadcastID)
	if err != nil {
		return err
	}
	if err = emitBroadcastProgress(ctx, tx, tenantID, broadcastID); err != nil {
		return err
	}
	return tx.Commit()
}

func emitBroadcastProgress(ctx context.Context, tx *sql.Tx, tenantID, broadcastID string) error {
	var creator, status string
	var total, sent, failed int
	if err := tx.QueryRowContext(ctx, `SELECT created_by,status,recipients_count,processed_count,failed_count FROM broadcasts WHERE tenant_id=? AND id=?`, tenantID, broadcastID).Scan(&creator, &status, &total, &sent, &failed); err != nil {
		return err
	}
	if err := emitNotificationStream(ctx, tx, tenantID, creator, "broadcast.progress", nil, map[string]any{"id": broadcastID, "sent": sent, "total": total, "failed": failed}); err != nil {
		return err
	}
	return emitNotificationStream(ctx, tx, tenantID, creator, "job.updated", nil, map[string]any{"job_id": broadcastID, "status": status, "progress": sent + failed, "total": total})
}

func BroadcastJobStatus(ctx context.Context, db *sql.DB, tenantID, userID, id string) (map[string]any, error) {
	var status string
	var total, sent, failed int
	var actor string
	var updated time.Time
	err := db.QueryRowContext(ctx, `SELECT status,recipients_count,processed_count,failed_count,created_by,updated_at FROM broadcasts WHERE tenant_id=? AND id=?`, tenantID, id).Scan(&status, &total, &sent, &failed, &actor, &updated)
	if err != nil {
		return nil, err
	}
	if actor != userID {
		return nil, sql.ErrNoRows
	}
	return map[string]any{"id": id, "type": "broadcast", "status": status, "total": total, "processed": sent, "failed": failed, "updated_at": updated}, nil
}
