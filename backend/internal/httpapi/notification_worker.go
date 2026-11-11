package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
)

type outboxRecord struct {
	TenantID    string
	ID          string
	EventType   string
	AggregateID string
	DedupeKey   string
	Payload     []byte
}

func RunNotificationOutbox(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT tenant_id,id FROM notification_outbox WHERE status='pending' AND (next_attempt_at IS NULL OR next_attempt_at<=UTC_TIMESTAMP(6)) AND (event_type LIKE 'approval.%' OR event_type IN ('employee_change.applied','termination.processed','loan.disbursed','loan.settled')) ORDER BY created_at,id LIMIT 100`)
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
		ok, err := processNotificationOutbox(ctx, db, k.tenant, k.id)
		if err != nil {
			return processed, err
		}
		if ok {
			processed++
		}
	}
	return processed, nil
}

func processNotificationOutbox(ctx context.Context, db *sql.DB, tenantID, id string) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var item outboxRecord
	var attempts int
	var status string
	err = tx.QueryRowContext(ctx, `SELECT tenant_id,id,event_type,aggregate_id,dedupe_key,payload_json,attempts,status FROM notification_outbox WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, id).Scan(&item.TenantID, &item.ID, &item.EventType, &item.AggregateID, &item.DedupeKey, &item.Payload, &attempts, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if status != "pending" && status != "failed" {
		return false, nil
	}
	var ruleEnabled bool
	err = tx.QueryRowContext(ctx, `SELECT enabled FROM notification_rules WHERE tenant_id=? AND event_type=?`, item.TenantID, item.EventType).Scan(&ruleEnabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if err == nil && !ruleEnabled {
		_, err = tx.ExecContext(ctx, `UPDATE notification_outbox SET status='processed',attempts=attempts+1,processed_at=UTC_TIMESTAMP(6),next_attempt_at=NULL WHERE tenant_id=? AND id=?`, tenantID, id)
		if err != nil {
			return false, err
		}
		return true, tx.Commit()
	}
	var recipients []string
	if len(item.EventType) >= 9 && item.EventType[:9] == "approval." {
		recipients, err = approvalOutboxRecipients(ctx, tx, db, item)
	} else {
		recipients, err = employeeOutboxRecipients(ctx, tx, item)
	}
	if err != nil {
		return false, retryOutbox(ctx, tx, item, attempts, err)
	}
	for _, userID := range recipients {
		var zone string
		err = tx.QueryRowContext(ctx, `SELECT t.timezone FROM tenant_memberships m JOIN tenants t ON t.id=m.tenant_id WHERE m.tenant_id=? AND m.user_id=? AND m.status='active'`, item.TenantID, userID).Scan(&zone)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, err
		}
		_, decision, e := (access.Store{DB: db}).Decide(ctx, item.TenantID, userID, zone, "notifikasi.lihat_sendiri")
		if e != nil {
			return false, e
		}
		if !decision.Allowed {
			continue
		}
		if err = insertOutboxNotification(ctx, tx, item, userID); err != nil {
			return false, retryOutbox(ctx, tx, item, attempts, err)
		}
	}
	if err = emitClosedApprovalTasks(ctx, tx, item); err != nil {
		return false, retryOutbox(ctx, tx, item, attempts, err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE notification_outbox SET status='processed',attempts=attempts+1,processed_at=UTC_TIMESTAMP(6),next_attempt_at=NULL WHERE tenant_id=? AND id=?`, tenantID, id)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func emitClosedApprovalTasks(ctx context.Context, tx *sql.Tx, item outboxRecord) error {
	switch item.EventType {
	case "approval.returned", "approval.approved", "approval.rejected", "approval.reassigned", "approval.cancelled":
	default:
		return nil
	}
	var payload struct {
		ActionID   string `json:"action_id"`
		FromUserID string `json:"from_user_id"`
	}
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return err
	}
	users := map[string]bool{}
	if payload.FromUserID != "" {
		users[payload.FromUserID] = true
	}
	if payload.ActionID != "" {
		var actor string
		err := tx.QueryRowContext(ctx, `SELECT actor_user_id FROM approval_actions WHERE tenant_id=? AND id=?`, item.TenantID, payload.ActionID).Scan(&actor)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil {
			users[actor] = true
		}
	}
	if item.EventType == "approval.cancelled" {
		rows, err := tx.QueryContext(ctx, `SELECT DISTINCT t.assignee_user_id FROM approval_tasks t JOIN approval_instances i ON i.tenant_id=t.tenant_id AND i.id=t.instance_id WHERE i.tenant_id=? AND i.request_id=? AND t.status='cancelled'`, item.TenantID, item.AggregateID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var userID string
			if err := rows.Scan(&userID); err != nil {
				rows.Close()
				return err
			}
			users[userID] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	var requestType string
	if err := tx.QueryRowContext(ctx, `SELECT request_type FROM requests WHERE tenant_id=? AND id=?`, item.TenantID, item.AggregateID).Scan(&requestType); err != nil {
		return err
	}
	for userID := range users {
		if err := emitNotificationStream(ctx, tx, item.TenantID, userID, "approval.task.closed", nil, map[string]any{"request_id": item.AggregateID, "request_type": requestType}); err != nil {
			return err
		}
	}
	return nil
}

func retryOutbox(ctx context.Context, tx *sql.Tx, item outboxRecord, attempts int, cause error) error {
	delay := time.Duration(1<<min(attempts, 6)) * time.Minute
	status := "pending"
	if attempts >= 7 {
		status = "failed"
	}
	_, err := tx.ExecContext(ctx, `UPDATE notification_outbox SET status=?,attempts=attempts+1,next_attempt_at=? WHERE tenant_id=? AND id=?`, status, time.Now().UTC().Add(delay), item.TenantID, item.ID)
	if err != nil {
		return errors.Join(cause, err)
	}
	if err = tx.Commit(); err != nil {
		return errors.Join(cause, err)
	}
	return nil
}

func approvalOutboxRecipients(ctx context.Context, tx *sql.Tx, db *sql.DB, item outboxRecord) ([]string, error) {
	var requester, requestType string
	var employeeID sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT requested_by,request_type,subject_employee_id FROM requests WHERE tenant_id=? AND id=?`, item.TenantID, item.AggregateID).Scan(&requester, &requestType, &employeeID)
	if errors.Is(err, sql.ErrNoRows) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	users := map[string]bool{}
	if item.EventType != "approval.submitted" && item.EventType != "approval.resubmitted" && item.EventType != "approval.reassigned" {
		users[requester] = true
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT t.assignee_user_id FROM approval_tasks t JOIN approval_instances i ON i.tenant_id=t.tenant_id AND i.id=t.instance_id WHERE i.tenant_id=? AND i.request_id=? AND i.status='in_review' AND t.status='pending'`, item.TenantID, item.AggregateID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			return nil, err
		}
		if uid != requester {
			users[uid] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := []string{}
	for uid := range users {
		if uid != requester {
			perm := approvalPermissions[requestType]
			if perm == "" {
				continue
			}
			var zone string
			if err := tx.QueryRowContext(ctx, `SELECT timezone FROM tenants WHERE id=?`, item.TenantID).Scan(&zone); err != nil {
				return nil, err
			}
			subject, decision, err := (access.Store{DB: db}).Decide(ctx, item.TenantID, uid, zone, perm)
			if err != nil {
				return nil, err
			}
			if !decision.Allowed {
				continue
			}
			if employeeID.Valid {
				var target access.Target
				var employeeUser sql.NullString
				var manager sql.NullString
				err = tx.QueryRowContext(ctx, `SELECT e.user_id,a.reports_to_employee_id,a.office_id,a.position_id,a.region_id,a.division_id,a.department_id FROM employees e JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE e.tenant_id=? AND e.id=?`, item.TenantID, employeeID.String).Scan(&employeeUser, &manager, &target.BaseOfficeID, &target.PositionID, &target.RegionID, &target.DivisionID, &target.DepartmentID)
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				if err != nil {
					return nil, err
				}
				target.TenantID = item.TenantID
				target.EmployeeID = employeeID.String
				target.UserID = employeeUser.String
				target.ReportsToID = manager.String
				if !decision.Includes(subject, target) {
					continue
				}
			}
		}
		result = append(result, uid)
	}
	return result, nil
}

func employeeOutboxRecipients(ctx context.Context, tx *sql.Tx, item outboxRecord) ([]string, error) {
	var payload map[string]any
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return nil, err
	}
	employeeID, _ := payload["employee_id"].(string)
	if employeeID == "" {
		return []string{}, nil
	}
	var userID sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT user_id FROM employees WHERE tenant_id=? AND id=?`, item.TenantID, employeeID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !userID.Valid {
		return []string{}, nil
	}
	return []string{userID.String}, nil
}

func insertOutboxNotification(ctx context.Context, tx *sql.Tx, item outboxRecord, userID string) error {
	title := "Pembaruan aktivitas"
	category := "system"
	link := "/ess/pengajuan"
	if len(item.EventType) >= 9 && item.EventType[:9] == "approval." {
		title = "Pembaruan pengajuan"
		category = "request"
	}
	var requester string
	if category == "request" {
		_ = tx.QueryRowContext(ctx, `SELECT requested_by FROM requests WHERE tenant_id=? AND id=?`, item.TenantID, item.AggregateID).Scan(&requester)
	}
	if category == "request" && userID != requester {
		title = "Pengajuan menunggu persetujuan"
		link = "/admin/approvals"
	}
	messageTitle, messageBody, err := renderedNotification(ctx, tx, item.TenantID, item.EventType, "inapp", title)
	if err != nil {
		return err
	}
	notificationID, err := ids.New()
	if err != nil {
		return err
	}
	dedupe := fmt.Sprintf("outbox:%s:%s", item.ID, userID)
	payload, _ := json.Marshal(map[string]string{"request_id": item.AggregateID})
	_, err = tx.ExecContext(ctx, `INSERT IGNORE INTO notifications (tenant_id,id,recipient_user_id,category,event_type,title,body,deep_link,payload_json,dedupe_key) VALUES (?,?,?,?,?,?,?,?,?,?)`, item.TenantID, notificationID, userID, category, item.EventType, messageTitle, messageBody, link, string(payload), dedupe)
	if err != nil {
		return err
	}
	var actual string
	if err = tx.QueryRowContext(ctx, `SELECT id FROM notifications WHERE tenant_id=? AND recipient_user_id=? AND dedupe_key=?`, item.TenantID, userID, dedupe).Scan(&actual); err != nil {
		return err
	}
	deliveryID, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_deliveries (tenant_id,id,notification_id,channel,status,delivered_at) SELECT ?,?,?,'inapp','delivered',UTC_TIMESTAMP(6) WHERE NOT EXISTS (SELECT 1 FROM notification_deliveries WHERE tenant_id=? AND notification_id=? AND channel='inapp')`, item.TenantID, deliveryID, actual, item.TenantID, actual)
	if err != nil {
		return err
	}
	if category == "request" {
		event := "approval.request.updated"
		if userID != requester {
			event = "approval.task.created"
		}
		if err = emitNotificationStream(ctx, tx, item.TenantID, userID, event, &actual, map[string]any{"request_id": item.AggregateID}); err != nil {
			return err
		}
	}
	if err = queuePushDelivery(ctx, tx, item.TenantID, userID, actual, category); err != nil {
		return err
	}
	return queueEmailDelivery(ctx, tx, item.TenantID, userID, actual, category)
}
