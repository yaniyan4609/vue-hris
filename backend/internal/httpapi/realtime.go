package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/auth"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type realtimeInput struct {
	Type               string `json:"type"`
	AfterID            string `json:"after_id"`
	LastNotificationID string `json:"last_notification_id"`
	EventID            string `json:"event_id"`
}

type realtimeEvent struct {
	Type  string         `json:"type"`
	ID    string         `json:"id"`
	Event string         `json:"event"`
	TS    time.Time      `json:"ts"`
	Data  map[string]any `json:"data"`
}

func allowedRealtimeOrigin(r *http.Request, configured string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if configured != "" {
		return strings.EqualFold(origin, configured)
	}
	return strings.EqualFold(u.Host, r.Host)
}

func mountRealtime(r interface {
	Get(string, http.HandlerFunc)
}, db *sql.DB, config Options) {
	r.Get("/api/v1/realtime/ws", func(w http.ResponseWriter, req *http.Request) {
		if !allowedRealtimeOrigin(req, config.AllowedOrigin) {
			writeAPIError(w, req, 403, "ORIGIN_FORBIDDEN", "Asal koneksi ditolak.")
			return
		}
		p, ok := notificationIdentity(w, req, db)
		if !ok {
			return
		}
		var seq uint64
		if err := db.QueryRowContext(req.Context(), `SELECT COALESCE(MAX(sequence_no),0) FROM notifications WHERE tenant_id=? AND recipient_user_id=?`, p.TenantID, p.UserID).Scan(&seq); err != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Koneksi real-time tidak tersedia.")
			return
		}
		conn, err := websocket.Accept(w, req, &websocket.AcceptOptions{OriginPatterns: []string{req.Header.Get("Origin")}})
		if err != nil {
			return
		}
		defer conn.CloseNow()
		conn.SetReadLimit(4096)
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		input := make(chan realtimeInput, 8)
		readErr := make(chan error, 1)
		go func() {
			for {
				var msg realtimeInput
				if err := wsjson.Read(ctx, conn, &msg); err != nil {
					readErr <- err
					return
				}
				select {
				case input <- msg:
				case <-ctx.Done():
					return
				}
			}
		}()
		count, err := unreadCount(req, db, p)
		if err != nil {
			_ = conn.Close(1011, "unread count failed")
			return
		}
		if err := wsjson.Write(ctx, conn, realtimeEvent{Type: "event", ID: "", Event: "notification.unread_count", TS: time.Now().UTC(), Data: map[string]any{"count": count}}); err != nil {
			return
		}
		poll := time.NewTicker(5 * time.Second)
		defer poll.Stop()
		verify := time.NewTicker(25 * time.Second)
		defer verify.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-readErr:
				return
			case msg := <-input:
				switch msg.Type {
				case "hello", "resume":
					id := msg.AfterID
					if id == "" {
						id = msg.LastNotificationID
					}
					if id != "" {
						cursor, err := notificationCursor(req, db, p, id)
						if err != nil {
							_ = conn.Close(1008, "invalid cursor")
							return
						}
						seq = cursor
					}
				case "ping":
					if err := wsjson.Write(ctx, conn, map[string]any{"type": "pong"}); err != nil {
						return
					}
				case "ack": // Delivery is recovered through REST; no server mutation.
				default:
					_ = conn.Close(1008, "invalid message")
					return
				}
			case <-poll.C:
				if err := streamNotifications(ctx, conn, db, p, &seq); err != nil {
					return
				}
			case <-verify.C:
				if code := realtimeAccessStatus(ctx, db, cookieToken(req), p); code != 0 {
					_ = conn.Close(websocket.StatusCode(code), "access changed")
					return
				}
				pingCtx, done := context.WithTimeout(ctx, 5*time.Second)
				err := conn.Ping(pingCtx)
				done()
				if err != nil {
					return
				}
			}
		}
	})
}

func realtimeAccessStatus(ctx context.Context, db *sql.DB, token string, p notificationPrincipal) int {
	state, err := (auth.Service{DB: db}).Peek(ctx, token)
	if err != nil || state.User.ID != p.UserID || state.SelectedTenantID != p.TenantID {
		return 4401
	}
	var timezone string
	for _, tenant := range state.Tenants {
		if tenant.ID == p.TenantID {
			timezone = tenant.Timezone
			break
		}
	}
	if timezone == "" {
		return 4401
	}
	var version uint64
	if err := db.QueryRowContext(ctx, `SELECT access_version FROM tenant_memberships WHERE tenant_id=? AND user_id=? AND status='active'`, p.TenantID, p.UserID).Scan(&version); err != nil {
		return 4401
	}
	if version != p.Version {
		return 4408
	}
	_, decision, err := (access.Store{DB: db}).Decide(ctx, p.TenantID, p.UserID, timezone, "notifikasi.lihat_sendiri")
	if err != nil || !decision.Allowed {
		return 4408
	}
	return 0
}

func streamNotifications(ctx context.Context, conn *websocket.Conn, db *sql.DB, p notificationPrincipal, seq *uint64) error {
	// Bounded batches keep a slow client from holding a DB cursor indefinitely.
	for batch := 0; batch < 4; batch++ {
		rows, err := db.QueryContext(ctx, `SELECT id,sequence_no,category,title,deep_link,created_at FROM notifications WHERE tenant_id=? AND recipient_user_id=? AND sequence_no>? ORDER BY sequence_no ASC LIMIT 100`, p.TenantID, p.UserID, *seq)
		if err != nil {
			return err
		}
		type item struct {
			ID              string
			Sequence        uint64
			Category, Title string
			Link            sql.NullString
			Created         time.Time
		}
		items := []item{}
		for rows.Next() {
			var n item
			if err := rows.Scan(&n.ID, &n.Sequence, &n.Category, &n.Title, &n.Link, &n.Created); err != nil {
				rows.Close()
				return err
			}
			items = append(items, n)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		var unread int
		if len(items) > 0 {
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE tenant_id=? AND recipient_user_id=? AND read_at IS NULL AND archived_at IS NULL AND obsolete_at IS NULL`, p.TenantID, p.UserID).Scan(&unread); err != nil {
				return err
			}
		}
		for _, n := range items {
			data := map[string]any{"id": n.ID, "category": n.Category, "title": n.Title}
			data["unread_count"] = unread
			if n.Link.Valid {
				data["deep_link"] = n.Link.String
			}
			if err := wsjson.Write(ctx, conn, realtimeEvent{Type: "event", ID: "evt_" + n.ID, Event: "notification.created", TS: n.Created.UTC(), Data: data}); err != nil {
				return err
			}
			*seq = n.Sequence
		}
		if len(items) < 100 {
			break
		}
	}
	return nil
}
