package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
)

func emitNotificationStream(ctx context.Context, tx *sql.Tx, tenantID, userID, event string, notificationID *string, data map[string]any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_stream_events (tenant_id,recipient_user_id,event_type,notification_id,payload_json) VALUES (?,?,?,?,?)`, tenantID, userID, event, notificationID, string(raw))
	return err
}
