package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// Run with HRIS_PROXY_URL=http://127.0.0.1:<vite-port> against a local
// staging frontend and API. It verifies Vite forwards WebSocket upgrades.
func TestViteWebSocketProxy(t *testing.T) {
	base := os.Getenv("HRIS_PROXY_URL")
	if base == "" {
		t.Skip("local Vite proxy test is opt-in")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "http" || !strings.HasPrefix(u.Host, "127.0.0.1:") {
		t.Fatal("proxy URL must be a local HTTP address")
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	login, err := client.Post(base+"/api/v1/auth/demo-login", "application/json", bytes.NewBufferString(`{"account":"admin"}`))
	if err != nil {
		t.Fatal(err)
	}
	login.Body.Close()
	if login.StatusCode != 200 {
		t.Fatalf("demo login: %d", login.StatusCode)
	}
	defer func() {
		request, _ := http.NewRequest(http.MethodPost, base+"/api/v1/auth/logout", bytes.NewBufferString(`{}`))
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err == nil {
			response.Body.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(base, "http") + "/api/v1/realtime/ws"
	conn, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{base}}})
	if err != nil {
		if response != nil {
			t.Fatalf("upgrade status %d: %v", response.StatusCode, err)
		}
		t.Fatal(err)
	}
	defer conn.CloseNow()
	var frame struct {
		Event string `json:"event"`
	}
	if err := wsjson.Read(ctx, conn, &frame); err != nil {
		t.Fatal(err)
	}
	if frame.Event != "notification.unread_count" {
		raw, _ := json.Marshal(frame)
		t.Fatalf("wrong first frame: %s", raw)
	}
}
