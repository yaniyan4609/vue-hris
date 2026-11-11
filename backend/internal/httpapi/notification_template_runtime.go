package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
)

type notificationTemplateSpec struct{ ID, EventType, Channel string }

func templateSpecs() []notificationTemplateSpec {
	items := make([]notificationTemplateSpec, 0, len(notificationEventTypes)*3)
	for _, event := range notificationEventTypes {
		for _, channel := range []string{"inapp", "push", "email"} {
			digest := sha256.Sum256([]byte("notification-template:" + event + ":" + channel))
			items = append(items, notificationTemplateSpec{ID: strings.ToUpper(hex.EncodeToString(digest[:13])), EventType: event, Channel: channel})
		}
	}
	return items
}

func templateSpecByID(id string) (notificationTemplateSpec, bool) {
	for _, item := range templateSpecs() {
		if item.ID == id {
			return item, true
		}
	}
	return notificationTemplateSpec{}, false
}

func defaultTemplate(event, channel string) (string, string) {
	if channel == "push" {
		return "Pembaruan digaji.id", "Buka aplikasi untuk melihat informasi terbaru."
	}
	if channel == "email" {
		return "Pembaruan digaji.id", "Ada pembaruan di {{company_name}}. Buka aplikasi untuk melihat detailnya."
	}
	return "{{event_name}}", "Buka aplikasi untuk melihat detail terbaru."
}

var templateVariable = regexp.MustCompile(`\{\{\s*([a-z_]+)\s*\}\}`)

func validTemplateText(value string) bool {
	for _, match := range templateVariable.FindAllStringSubmatch(value, -1) {
		switch match[1] {
		case "event_name", "company_name":
		default:
			return false
		}
	}
	stripped := templateVariable.ReplaceAllString(value, "")
	return !strings.Contains(stripped, "{{") && !strings.Contains(stripped, "}}")
}

func renderTemplate(value string, vars map[string]string) string {
	return templateVariable.ReplaceAllStringFunc(value, func(match string) string { name := templateVariable.FindStringSubmatch(match)[1]; return vars[name] })
}

func loadNotificationTemplate(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, tenantID, event, channel string) (string, string, error) {
	var title, body string
	err := q.QueryRowContext(ctx, `SELECT title_tpl,body_tpl FROM notification_templates WHERE tenant_id=? AND event_type=? AND channel=?`, tenantID, event, channel).Scan(&title, &body)
	if errors.Is(err, sql.ErrNoRows) {
		title, body = defaultTemplate(event, channel)
		return title, body, nil
	}
	return title, body, err
}

func renderedNotification(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, tenantID, event, channel, eventName string) (string, string, error) {
	title, body, err := loadNotificationTemplate(ctx, q, tenantID, event, channel)
	if err != nil {
		return "", "", err
	}
	var company string
	if err = q.QueryRowContext(ctx, `SELECT name FROM tenants WHERE id=?`, tenantID).Scan(&company); err != nil {
		return "", "", err
	}
	vars := map[string]string{"event_name": eventName, "company_name": company}
	return renderTemplate(title, vars), renderTemplate(body, vars), nil
}
