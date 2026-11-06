package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestStagingNotificationRealtimeAndOwnerBoundary(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("requires named staging database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false, AllowedOrigin: ""}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var admin auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &admin)
	if admin.SelectedTenantID == "" {
		t.Fatal("admin tenant missing")
	}
	accessResponse, err := client.Get(server.URL + "/api/v1/me/access")
	if err != nil {
		t.Fatal(err)
	}
	etag := accessResponse.Header.Get("ETag")
	version := accessResponse.Header.Get("X-Access-Version")
	accessResponse.Body.Close()
	if accessResponse.StatusCode != 200 || etag == "" || version == "" {
		t.Fatalf("access cache headers: %d %q %q", accessResponse.StatusCode, etag, version)
	}
	request304, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/me/access", nil)
	if err != nil {
		t.Fatal(err)
	}
	request304.Header.Set("If-None-Match", etag)
	response304, err := client.Do(request304)
	if err != nil {
		t.Fatal(err)
	}
	response304.Body.Close()
	if response304.StatusCode != 304 {
		t.Fatalf("unchanged access: %d", response304.StatusCode)
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/realtime/ws"
	conn, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{server.URL}}})
	if err != nil {
		if response != nil {
			t.Fatalf("websocket %d: %v", response.StatusCode, err)
		}
		t.Fatal(err)
	}
	defer conn.CloseNow()
	var first realtimeEvent
	if err := wsjson.Read(ctx, conn, &first); err != nil || first.Event != "notification.unread_count" {
		t.Fatalf("first frame: %+v, %v", first, err)
	}
	id, _ := ids.New()
	_, err = db.ExecContext(ctx, `INSERT INTO notifications (tenant_id,id,recipient_user_id,category,event_type,title,body,dedupe_key) VALUES (?,?,?,'system','test.integration','Tes realtime','Hanya akun admin',?)`, admin.SelectedTenantID, id, admin.User.ID, id)
	if err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(context.Background(), `DELETE FROM notifications WHERE tenant_id=? AND id=?`, admin.SelectedTenantID, id)
	var event realtimeEvent
	if err := wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatal(err)
	}
	if event.Event != "notification.created" || event.Data["id"] != id {
		t.Fatalf("wrong event: %+v", event)
	}
	resp, err := client.Get(server.URL + "/api/v1/notifications/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("admin detail: %d", resp.StatusCode)
	}
	var detail struct {
		Data notificationItem `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil || detail.Data.ID != id {
		t.Fatalf("detail: %+v, %v", detail, err)
	}
	readURL := server.URL + "/api/v1/notifications/" + id + "/read"
	withoutToken, err := http.NewRequest(http.MethodPost, readURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	denied, err := client.Do(withoutToken)
	if err != nil {
		t.Fatal(err)
	}
	denied.Body.Close()
	if denied.StatusCode != 403 {
		t.Fatalf("mutation without CSRF: %d", denied.StatusCode)
	}
	csrfResponse, err := client.Get(server.URL + "/api/v1/auth/csrf")
	if err != nil {
		t.Fatal(err)
	}
	var csrf struct {
		Data struct {
			Token string `json:"csrf_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(csrfResponse.Body).Decode(&csrf); err != nil {
		t.Fatal(err)
	}
	csrfResponse.Body.Close()
	if csrfResponse.StatusCode != 200 || csrf.Data.Token == "" {
		t.Fatalf("csrf endpoint: %d", csrfResponse.StatusCode)
	}
	withToken, err := http.NewRequest(http.MethodPost, readURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	withToken.Header.Set("X-CSRF-Token", csrf.Data.Token)
	marked, err := client.Do(withToken)
	if err != nil {
		t.Fatal(err)
	}
	marked.Body.Close()
	if marked.StatusCode != 204 {
		t.Fatalf("mark read: %d", marked.StatusCode)
	}
	otherJar, _ := cookiejar.New(nil)
	other := &http.Client{Jar: otherJar, Timeout: 10 * time.Second}
	var ess auth.State
	doJSON(t, other, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &ess)
	if ess.SelectedTenantID != admin.SelectedTenantID {
		t.Skip("demo ESS is assigned to a different tenant")
	}
	respOther, err := other.Get(server.URL + "/api/v1/notifications/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer respOther.Body.Close()
	if respOther.StatusCode != 404 {
		t.Fatalf("other user saw notification: %d", respOther.StatusCode)
	}
}
