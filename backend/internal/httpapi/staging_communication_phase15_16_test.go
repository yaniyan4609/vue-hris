package httpapi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
	"github.com/SherClockHolmes/webpush-go"
)

func TestStagingBroadcastSnapshotAndAnnouncement(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	adminJar, _ := cookiejar.New(nil)
	admin := &http.Client{Jar: adminJar, Timeout: 10 * time.Second}
	essJar, _ := cookiejar.New(nil)
	ess := &http.Client{Jar: essJar, Timeout: 10 * time.Second}
	var actor, recipient auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &actor)
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &recipient)
	if actor.SelectedTenantID != recipient.SelectedTenantID {
		t.Skip("demo accounts are on different tenants")
	}
	tenant := actor.SelectedTenantID
	var made struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	target := recipient.User.ID
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/broadcasts", broadcastInput{Title: "QA Broadcast", Body: "QA staging announcement", Targets: []broadcastTarget{{Type: "user", RefID: &target}}, CombineMode: "or"}, 201, &made)
	id := made.Data.ID
	defer func() {
		var notification sql.NullString
		_ = db.QueryRow(`SELECT notification_id FROM broadcast_recipients WHERE tenant_id=? AND broadcast_id=? LIMIT 1`, tenant, id).Scan(&notification)
		if notification.Valid {
			_, _ = db.Exec(`DELETE FROM notification_stream_events WHERE tenant_id=? AND notification_id=?`, tenant, notification.String)
			_, _ = db.Exec(`DELETE FROM notification_deliveries WHERE tenant_id=? AND notification_id=?`, tenant, notification.String)
		}
		_, _ = db.Exec(`DELETE FROM notification_stream_events WHERE tenant_id=? AND event_type='broadcast.progress' AND JSON_UNQUOTE(JSON_EXTRACT(payload_json,'$.id'))=?`, tenant, id)
		_, _ = db.Exec(`DELETE FROM broadcast_recipients WHERE tenant_id=? AND broadcast_id=?`, tenant, id)
		if notification.Valid {
			_, _ = db.Exec(`DELETE FROM notifications WHERE tenant_id=? AND id=?`, tenant, notification.String)
		}
		_, _ = db.Exec(`DELETE FROM broadcast_targets WHERE tenant_id=? AND broadcast_id=?`, tenant, id)
		_, _ = db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='broadcasts' AND entity_id=?`, tenant, id)
		_, _ = db.Exec(`DELETE FROM broadcasts WHERE tenant_id=? AND id=?`, tenant, id)
	}()
	var preview struct {
		Data broadcastAudience `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/broadcasts/"+id+"/audience-preview", nil, 200, &preview)
	if preview.Data.RecipientsCount != 1 {
		t.Fatalf("audience count: %d", preview.Data.RecipientsCount)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/broadcasts/"+id+"/schedule", map[string]any{"send_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339), "version": 1}, 202, nil)
	csrfResponse, err := admin.Get(server.URL + "/api/v1/auth/csrf")
	if err != nil {
		t.Fatal(err)
	}
	var csrf struct {
		Data struct {
			Token string `json:"csrf_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(csrfResponse.Body).Decode(&csrf); err != nil {
		csrfResponse.Body.Close()
		t.Fatal(err)
	}
	csrfResponse.Body.Close()
	request, err := http.NewRequest("POST", server.URL+"/api/v1/admin/broadcasts/"+id+"/send", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-CSRF-Token", csrf.Data.Token)
	request.Header.Set("Idempotency-Key", id)
	response, err := admin.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 202 {
		t.Fatalf("send: %d", response.StatusCode)
	}
	replay, err := http.NewRequest("POST", server.URL+"/api/v1/admin/broadcasts/"+id+"/send", nil)
	if err != nil {
		t.Fatal(err)
	}
	replay.Header.Set("X-CSRF-Token", csrf.Data.Token)
	replay.Header.Set("Idempotency-Key", id)
	replayed, err := admin.Do(replay)
	if err != nil {
		t.Fatal(err)
	}
	replayed.Body.Close()
	if replayed.StatusCode != 202 || replayed.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("send replay: %d, %q", replayed.StatusCode, replayed.Header.Get("Idempotent-Replay"))
	}
	processed, err := processDueBroadcast(ctx, db, tenant, id)
	if err != nil || !processed {
		t.Fatalf("worker: %v, %v", processed, err)
	}
	var announcements struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/announcements", nil, 200, &announcements)
	found := false
	for _, item := range announcements.Data {
		if item["id"] == id {
			found = true
		}
	}
	if !found {
		t.Fatal("ESS announcement not delivered")
	}
	var stats struct {
		Data map[string]any `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/broadcasts/"+id+"/stats", nil, 200, &stats)
	if stats.Data["sent"] != float64(1) {
		t.Fatalf("stats: %v", stats.Data)
	}
}

func TestStagingBroadcastScheduleCancel(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var actor auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &actor)
	var made struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	target := actor.User.ID
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/broadcasts", broadcastInput{Title: "QA Cancel", Body: "QA staging", Targets: []broadcastTarget{{Type: "user", RefID: &target}}}, 201, &made)
	id := made.Data.ID
	defer func() {
		_, _ = db.Exec(`DELETE FROM broadcast_targets WHERE tenant_id=? AND broadcast_id=?`, actor.SelectedTenantID, id)
		_, _ = db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='broadcasts' AND entity_id=?`, actor.SelectedTenantID, id)
		_, _ = db.Exec(`DELETE FROM broadcasts WHERE tenant_id=? AND id=?`, actor.SelectedTenantID, id)
	}()
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/broadcasts/"+id+"/schedule", map[string]any{"send_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339), "version": 1}, 202, nil)
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/broadcasts/"+id+"/cancel", nil, 202, nil)
	var got struct {
		Data broadcastRecord `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/broadcasts/"+id, nil, 200, &got)
	if got.Data.Status != "cancelled" || got.Data.RecipientsCount != 0 {
		t.Fatalf("cancel result: %+v", got.Data)
	}
}

func TestStagingNotificationRuleAndTemplate(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var actor auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &actor)
	tenant := actor.SelectedTenantID
	event := "attendance.clock_in_reminder"
	var oldRuleID string
	var oldRuleConfig []byte
	var oldRuleEnabled bool
	ruleErr := db.QueryRowContext(ctx, `SELECT id,config_json,enabled FROM notification_rules WHERE tenant_id=? AND event_type=?`, tenant, event).Scan(&oldRuleID, &oldRuleConfig, &oldRuleEnabled)
	if ruleErr != nil && ruleErr != sql.ErrNoRows {
		t.Fatal(ruleErr)
	}
	defer func() {
		var id string
		_ = db.QueryRow(`SELECT id FROM notification_rules WHERE tenant_id=? AND event_type=?`, tenant, event).Scan(&id)
		if ruleErr == sql.ErrNoRows {
			_, _ = db.Exec(`DELETE FROM notification_schedules WHERE tenant_id=? AND rule_id=?`, tenant, id)
			_, _ = db.Exec(`DELETE FROM notification_rules WHERE tenant_id=? AND event_type=?`, tenant, event)
		} else {
			_, _ = db.Exec(`UPDATE notification_rules SET config_json=?,enabled=? WHERE tenant_id=? AND id=?`, oldRuleConfig, oldRuleEnabled, tenant, oldRuleID)
		}
		_, _ = db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='notification_rules' AND entity_id=?`, tenant, id)
	}()
	rule := notificationRuleConfig{Enabled: false, OffsetMinutes: -5, RepeatMinutes: 30, MaxRepeats: 1, QuietStart: "22:00", QuietEnd: "06:00"}
	doJSON(t, client, "PUT", server.URL+"/api/v1/admin/notification-rules/"+event, rule, 200, nil)
	var readRule struct {
		Data struct {
			Config notificationRuleConfig `json:"config"`
		} `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/notification-rules/"+event, nil, 200, &readRule)
	if readRule.Data.Config.OffsetMinutes != -5 {
		t.Fatalf("rule not persisted: %+v", readRule.Data.Config)
	}
	var spec notificationTemplateSpec
	for _, v := range templateSpecs() {
		if v.EventType == event && v.Channel == "inapp" {
			spec = v
			break
		}
	}
	if spec.ID == "" {
		t.Fatal("template spec missing")
	}
	var oldTitle, oldBody string
	templateErr := db.QueryRowContext(ctx, `SELECT title_tpl,body_tpl FROM notification_templates WHERE tenant_id=? AND id=?`, tenant, spec.ID).Scan(&oldTitle, &oldBody)
	if templateErr != nil && templateErr != sql.ErrNoRows {
		t.Fatal(templateErr)
	}
	defer func() {
		if templateErr == sql.ErrNoRows {
			_, _ = db.Exec(`DELETE FROM notification_templates WHERE tenant_id=? AND id=?`, tenant, spec.ID)
		} else {
			_, _ = db.Exec(`UPDATE notification_templates SET title_tpl=?,body_tpl=? WHERE tenant_id=? AND id=?`, oldTitle, oldBody, tenant, spec.ID)
		}
		_, _ = db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='notification_templates' AND entity_id=?`, tenant, spec.ID)
	}()
	doJSON(t, client, "PUT", server.URL+"/api/v1/admin/notification-templates/"+spec.ID, notificationTemplateInput{Title: "Pembaruan {{event_name}}", Body: "Dari {{company_name}}"}, 200, nil)
	var preview struct {
		Data map[string]string `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/notification-templates/"+spec.ID+"/preview", nil, 200, &preview)
	if preview.Data["body"] != "Dari "+actor.Tenants[0].Name {
		t.Fatalf("template preview: %v", preview.Data)
	}
	var catalog struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/notification-event-types", nil, 200, &catalog)
	if len(catalog.Data) == 0 {
		t.Fatal("event catalog empty")
	}
}

func TestStagingPushSubscriptionEncryptedAndOwned(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	if os.Getenv("TENANT_MASTER_KEY") == "" {
		t.Skip("persistent tenant master key required")
	}
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("VAPID_PRIVATE_KEY", private)
	t.Setenv("VAPID_PUBLIC_KEY", public)
	t.Setenv("VAPID_SUBSCRIBER", "mailto:admin@digaji.id")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var user auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &user)
	var keyCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_crypto_keys WHERE tenant_id=? AND active_slot=1`, user.SelectedTenantID).Scan(&keyCount); err != nil {
		t.Fatal(err)
	}
	if keyCount == 0 {
		t.Skip("tenant key has not been initialized")
	}
	endpoint := fmt.Sprintf("https://fcm.googleapis.com/fcm/send/%s", user.User.ID)
	input := pushInput{Endpoint: endpoint, P256dh: base64.RawURLEncoding.EncodeToString(make([]byte, 65)), Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16))}
	var made struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/push/subscriptions", input, 201, &made)
	id := made.Data.ID
	defer db.Exec(`DELETE FROM push_subscriptions WHERE tenant_id=? AND user_id=? AND id=?`, user.SelectedTenantID, user.User.ID, id)
	var encrypted []byte
	if err := db.QueryRowContext(ctx, `SELECT endpoint_enc FROM push_subscriptions WHERE tenant_id=? AND id=?`, user.SelectedTenantID, id).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if string(encrypted) == endpoint {
		t.Fatal("push endpoint stored as plaintext")
	}
	var listed struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/push/subscriptions", nil, 200, &listed)
	found := false
	for _, item := range listed.Data {
		if item["id"] == id {
			found = true
		}
	}
	if !found {
		t.Fatal("subscription missing")
	}
	doJSON(t, client, "DELETE", server.URL+"/api/v1/push/subscriptions/"+id, nil, 204, nil)
}
