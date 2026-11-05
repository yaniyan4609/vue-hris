package auth

import (
	"context"
	"database/sql"
	"strings"
	"unicode/utf8"

	"digaji.id/hris/internal/ids"
)

type eventWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func safeUserAgent(raw string) string {
	value := strings.ToValidUTF8(raw, "")
	for len(value) > 300 {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}

func deviceIP(device SessionDevice) any {
	if parsed := device.IP.To16(); parsed != nil {
		return []byte(parsed)
	}
	return nil
}

func logLoginEvent(ctx context.Context, writer eventWriter, userID, tenantID, event string, device SessionDevice) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	var user, tenant any
	if userID != "" {
		user = userID
	}
	if tenantID != "" {
		tenant = tenantID
	}
	_, err = writer.ExecContext(ctx, `INSERT INTO login_events (id,user_id,tenant_id,event,ip,user_agent) VALUES (?,?,?,?,?,?)`, id, user, tenant, event, deviceIP(device), safeUserAgent(device.UserAgent))
	return err
}
