package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type notificationRuleConfig struct {
	Enabled          bool   `json:"enabled"`
	OffsetMinutes    int    `json:"offset_minutes"`
	RepeatMinutes    int    `json:"repeat_minutes"`
	MaxRepeats       int    `json:"max_repeats"`
	QuietStart       string `json:"quiet_start"`
	QuietEnd         string `json:"quiet_end"`
	SummaryRecipient string `json:"summary_recipient"`
}

var notificationEventTypes = []string{
	"approval.submitted", "approval.resubmitted", "approval.reassigned", "approval.returned",
	"approval.approved", "approval.rejected", "approval.cancelled", "employee_change.applied",
	"termination.processed", "loan.disbursed", "loan.settled", "attendance.clock_in_reminder",
	"attendance.clock_out_reminder", "broadcast.published", "system.security",
}

func validNotificationEvent(value string) bool {
	for _, item := range notificationEventTypes {
		if item == value {
			return true
		}
	}
	return false
}

func validQuietClock(value string) bool {
	if value == "" {
		return true
	}
	_, err := time.Parse("15:04", value)
	return err == nil
}

func validNotificationRule(rule notificationRuleConfig) bool {
	return rule.OffsetMinutes >= -1440 && rule.OffsetMinutes <= 1440 &&
		rule.RepeatMinutes >= 0 && rule.RepeatMinutes <= 1440 &&
		rule.MaxRepeats >= 0 && rule.MaxRepeats <= 8 &&
		(rule.MaxRepeats == 0 || rule.RepeatMinutes > 0) &&
		validQuietClock(rule.QuietStart) && validQuietClock(rule.QuietEnd) &&
		((rule.QuietStart == "") == (rule.QuietEnd == "")) &&
		(rule.SummaryRecipient == "" || rule.SummaryRecipient == "manager" || rule.SummaryRecipient == "hr")
}

func (a adminAPI) mountNotificationAdmin(r chi.Router, config Options) {
	r.Get("/notification-event-types", a.notificationEventCatalog)
	r.Get("/notification-rules", a.listNotificationRules)
	r.Get("/notification-rules/{eventType}", a.getNotificationRule)
	r.With(checkOrigin(config)).Put("/notification-rules/{eventType}", a.putNotificationRule)
	r.Get("/notification-templates", a.listNotificationTemplates)
	r.With(checkOrigin(config)).Put("/notification-templates/{id}", a.putNotificationTemplate)
	r.With(checkOrigin(config)).Post("/notification-templates/{id}/preview", a.previewNotificationTemplate)
	r.Get("/notification-deliveries", a.listNotificationDeliveries)
	r.With(checkOrigin(config)).Post("/notification-deliveries/{id}/retry", a.retryNotificationDelivery)
	r.Get("/notification-stats", a.notificationStats)
	r.Get("/reminders/summary", a.reminderSummary)
}

func (a adminAPI) notificationEventCatalog(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.authorize(w, r, "notifikasi.kelola"); !ok {
		return
	}
	items := make([]map[string]any, 0, len(notificationEventTypes))
	for _, event := range notificationEventTypes {
		items = append(items, map[string]any{"event_type": event, "reminder": strings.HasSuffix(event, "_reminder"), "mandatory": event == "system.security"})
	}
	writeAPIData(w, 200, items, nil)
}

func (a adminAPI) listNotificationRules(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "notifikasi.kelola")
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT event_type,enabled,config_json,updated_at FROM notification_rules WHERE tenant_id=? ORDER BY event_type`, c.tenant.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aturan notifikasi.")
		return
	}
	defer rows.Close()
	stored := map[string]map[string]any{}
	for rows.Next() {
		var event string
		var enabled bool
		var raw []byte
		var updated time.Time
		if err := rows.Scan(&event, &enabled, &raw, &updated); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aturan notifikasi.")
			return
		}
		var cfg notificationRuleConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Aturan notifikasi rusak.")
			return
		}
		cfg.Enabled = enabled
		stored[event] = map[string]any{"event_type": event, "config": cfg, "updated_at": updated}
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aturan notifikasi.")
		return
	}
	items := make([]map[string]any, 0, len(notificationEventTypes))
	for _, event := range notificationEventTypes {
		if item, found := stored[event]; found {
			items = append(items, item)
			continue
		}
		items = append(items, map[string]any{"event_type": event, "config": notificationRuleConfig{Enabled: event != "attendance.clock_in_reminder" && event != "attendance.clock_out_reminder"}})
	}
	writeAPIData(w, 200, items, nil)
}

func (a adminAPI) getNotificationRule(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "notifikasi.kelola")
	if !ok {
		return
	}
	event := chi.URLParam(r, "eventType")
	if !validNotificationEvent(event) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Jenis notifikasi tidak ditemukan.")
		return
	}
	var enabled bool
	var raw []byte
	var updated time.Time
	err := a.db.QueryRowContext(r.Context(), `SELECT enabled,config_json,updated_at FROM notification_rules WHERE tenant_id=? AND event_type=?`, c.tenant.ID, event).Scan(&enabled, &raw, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIData(w, 200, map[string]any{"event_type": event, "config": notificationRuleConfig{Enabled: event != "attendance.clock_in_reminder" && event != "attendance.clock_out_reminder"}}, nil)
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aturan notifikasi.")
		return
	}
	var cfg notificationRuleConfig
	if json.Unmarshal(raw, &cfg) != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Aturan notifikasi rusak.")
		return
	}
	cfg.Enabled = enabled
	writeAPIData(w, 200, map[string]any{"event_type": event, "config": cfg, "updated_at": updated}, nil)
}

func (a adminAPI) putNotificationRule(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "notifikasi.kelola")
	if !ok {
		return
	}
	event := chi.URLParam(r, "eventType")
	if !validNotificationEvent(event) || event == "system.security" {
		writeAPIError(w, r, 422, "VALIDATION_FAILED", "Jenis aturan tidak dapat diubah.")
		return
	}
	var cfg notificationRuleConfig
	if !readJSON(w, r, &cfg) {
		return
	}
	if !validNotificationRule(cfg) {
		writeAPIError(w, r, 422, "VALIDATION_FAILED", "Konfigurasi aturan tidak valid.")
		return
	}
	raw, _ := json.Marshal(cfg)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan aturan.")
		return
	}
	defer tx.Rollback()
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan aturan.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO notification_rules (tenant_id,id,event_type,enabled,config_json,updated_by) VALUES (?,?,?,?,?,?) ON DUPLICATE KEY UPDATE enabled=VALUES(enabled),config_json=VALUES(config_json),updated_by=VALUES(updated_by)`, c.tenant.ID, id, event, cfg.Enabled, string(raw), c.state.User.ID)
	if err == nil {
		err = tx.QueryRowContext(r.Context(), `SELECT id FROM notification_rules WHERE tenant_id=? AND event_type=?`, c.tenant.ID, event).Scan(&id)
	}
	if err == nil {
		err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "notification_rules", id, nil, map[string]any{"event_type": event, "enabled": cfg.Enabled})
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan aturan.")
		return
	}
	writeAPIData(w, 200, map[string]any{"event_type": event, "config": cfg}, nil)
}

type notificationTemplateInput struct {
	Title string `json:"title_tpl"`
	Body  string `json:"body_tpl"`
}

func (a adminAPI) listNotificationTemplates(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "notifikasi.kelola")
	if !ok {
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,title_tpl,body_tpl FROM notification_templates WHERE tenant_id=?`, c.tenant.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca template.")
		return
	}
	overrides := map[string][2]string{}
	for rows.Next() {
		var id, title, body string
		if err = rows.Scan(&id, &title, &body); err != nil {
			break
		}
		overrides[id] = [2]string{title, body}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca template.")
		return
	}
	items := []map[string]string{}
	for _, spec := range templateSpecs() {
		title, body := defaultTemplate(spec.EventType, spec.Channel)
		if override, found := overrides[spec.ID]; found {
			title, body = override[0], override[1]
		}
		items = append(items, map[string]string{"id": spec.ID, "event_type": spec.EventType, "channel": spec.Channel, "title_tpl": title, "body_tpl": body})
	}
	writeAPIData(w, 200, items, nil)
}

func (a adminAPI) templateInput(w http.ResponseWriter, r *http.Request) (notificationTemplateInput, bool) {
	var body notificationTemplateInput
	if !readJSON(w, r, &body) {
		return body, false
	}
	body.Title = strings.TrimSpace(body.Title)
	body.Body = strings.TrimSpace(body.Body)
	if body.Title == "" || body.Body == "" || len([]rune(body.Title)) > 255 || len(body.Body) > 7000 || !validTemplateText(body.Title) || !validTemplateText(body.Body) {
		writeAPIError(w, r, 422, "VALIDATION_FAILED", "Template atau variabelnya tidak valid.")
		return body, false
	}
	return body, true
}

func (a adminAPI) putNotificationTemplate(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "notifikasi.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	spec, exists := templateSpecByID(id)
	if !exists {
		writeAPIError(w, r, 404, "NOT_FOUND", "Template tidak ditemukan.")
		return
	}
	body, ok := a.templateInput(w, r)
	if !ok {
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan template.")
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), `INSERT INTO notification_templates (tenant_id,id,event_type,channel,title_tpl,body_tpl) VALUES (?,?,?,?,?,?) ON DUPLICATE KEY UPDATE title_tpl=VALUES(title_tpl),body_tpl=VALUES(body_tpl)`, c.tenant.ID, id, spec.EventType, spec.Channel, body.Title, body.Body)
	if err == nil {
		err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "notification_templates", id, nil, map[string]any{"event_type": spec.EventType, "channel": spec.Channel})
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan template.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "event_type": spec.EventType, "channel": spec.Channel, "title_tpl": body.Title, "body_tpl": body.Body}, nil)
}

func (a adminAPI) previewNotificationTemplate(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "notifikasi.kelola")
	if !ok {
		return
	}
	spec, exists := templateSpecByID(chi.URLParam(r, "id"))
	if !exists {
		writeAPIError(w, r, 404, "NOT_FOUND", "Template tidak ditemukan.")
		return
	}
	title, body, err := loadNotificationTemplate(r.Context(), a.db, c.tenant.ID, spec.EventType, spec.Channel)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca template.")
		return
	}
	vars := map[string]string{"event_name": "Contoh pembaruan", "company_name": c.tenant.Name}
	writeAPIData(w, 200, map[string]string{"title": renderTemplate(title, vars), "body": renderTemplate(body, vars)}, nil)
}

func (a adminAPI) listNotificationDeliveries(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "notifikasi.kelola")
	if !ok {
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil || v < 1 || v > 100 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Limit tidak valid.")
			return
		}
		limit = v
	}
	query := `SELECT d.id,d.notification_id,d.channel,d.status,d.error_code,d.attempt_no,d.attempted_at,d.delivered_at FROM notification_deliveries d WHERE d.tenant_id=?`
	args := []any{c.tenant.ID}
	for _, field := range []string{"channel", "status"} {
		if value := r.URL.Query().Get(field); value != "" {
			if len(value) > 24 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter tidak valid.")
				return
			}
			query += " AND d." + field + "=?"
			args = append(args, value)
		}
	}
	for _, bound := range []struct{ key, operator string }{{"from", ">="}, {"to", "<="}} {
		if raw := r.URL.Query().Get(bound.key); raw != "" {
			value, parseErr := time.Parse("2006-01-02", raw)
			if parseErr != nil || value.Format("2006-01-02") != raw {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal filter tidak valid.")
				return
			}
			if bound.key == "to" {
				value = value.Add(24 * time.Hour)
				bound.operator = "<"
			}
			query += " AND d.attempted_at" + bound.operator + "?"
			args = append(args, value.UTC())
		}
	}
	query += ` ORDER BY d.attempted_at DESC,d.id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengiriman.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, nid, ch, status string
		var failure sql.NullString
		var attempt int
		var at time.Time
		var delivered sql.NullTime
		if rows.Scan(&id, &nid, &ch, &status, &failure, &attempt, &at, &delivered) != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengiriman.")
			return
		}
		items = append(items, map[string]any{"id": id, "notification_id": nid, "channel": ch, "status": status, "error_code": failure.String, "attempt_no": attempt, "attempted_at": at, "delivered_at": delivered.Time})
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengiriman.")
		return
	}
	writeAPIData(w, 200, items, nil)
}

func (a adminAPI) retryNotificationDelivery(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "notifikasi.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengulang pengiriman.")
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), `UPDATE notification_deliveries SET status='pending',error_code=NULL,next_attempt_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=? AND channel IN ('push','email') AND status='failed'`, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengulang pengiriman.")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var status string
		err = tx.QueryRowContext(r.Context(), `SELECT status FROM notification_deliveries WHERE tenant_id=? AND id=?`, c.tenant.ID, id).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Pengiriman tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengiriman.")
			return
		}
		if status != "pending" {
			writeAPIError(w, r, 409, "CONFLICT", "Pengiriman tidak gagal.")
			return
		}
	} else {
		if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "retry", "notification_deliveries", id, map[string]any{"status": "failed"}, map[string]any{"status": "pending"}); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat pengulangan pengiriman.")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengulang pengiriman.")
		return
	}
	writeAPIData(w, 202, map[string]string{"id": id, "status": "pending"}, nil)
}

func (a adminAPI) notificationStats(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "notifikasi.kelola")
	if !ok {
		return
	}
	var sent, failed, pending, unread int
	err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(CASE WHEN status='delivered' THEN 1 END),COUNT(CASE WHEN status='failed' THEN 1 END),COUNT(CASE WHEN status='pending' THEN 1 END) FROM notification_deliveries WHERE tenant_id=?`, c.tenant.ID).Scan(&sent, &failed, &pending)
	if err == nil {
		err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM notifications WHERE tenant_id=? AND read_at IS NULL AND archived_at IS NULL AND obsolete_at IS NULL`, c.tenant.ID).Scan(&unread)
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca statistik.")
		return
	}
	writeAPIData(w, 200, map[string]int{"delivered": sent, "failed": failed, "pending": pending, "unread": unread}, nil)
}

func (a adminAPI) reminderSummary(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "absensi.monitor_bawahan")
	if !ok {
		return
	}
	var in, out int
	// This count is scoped by the same employee predicate as attendance monitors.
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	day := time.Now().In(zone).Format("2006-01-02")
	base := ` FROM shift_instances i JOIN employees e ON e.tenant_id=i.tenant_id AND e.id=i.employee_id` + employeeScopeJoin + ` LEFT JOIN attendance_daily d ON d.tenant_id=i.tenant_id AND d.shift_instance_id=i.id WHERE i.tenant_id=? AND i.work_date=? AND i.status<>'cancelled' AND ` + filter
	params := append([]any{c.tenant.ID, day}, args...)
	err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(CASE WHEN d.first_in IS NULL AND i.clock_in_window_start_at<=UTC_TIMESTAMP(6) THEN 1 END),COUNT(CASE WHEN d.first_in IS NOT NULL AND d.last_out IS NULL AND i.clock_out_window_start_at<=UTC_TIMESTAMP(6) THEN 1 END)`+base, params...).Scan(&in, &out)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ringkasan pengingat.")
		return
	}
	writeAPIData(w, 200, map[string]int{"not_clocked_in": in, "not_clocked_out": out}, nil)
}
