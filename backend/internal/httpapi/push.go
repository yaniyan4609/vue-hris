package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"digaji.id/hris/internal/cryptodata"
	"digaji.id/hris/internal/ids"
	"github.com/SherClockHolmes/webpush-go"
	"github.com/go-chi/chi/v5"
)

type pushInput struct {
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

func vapidConfig() (public, private, subscriber string, ok bool) {
	public = os.Getenv("VAPID_PUBLIC_KEY")
	private = os.Getenv("VAPID_PRIVATE_KEY")
	subscriber = os.Getenv("VAPID_SUBSCRIBER")
	return public, private, subscriber, public != "" && private != "" && strings.HasPrefix(subscriber, "mailto:")
}

func validPushEndpoint(value string) bool {
	if len(value) < 20 || len(value) > 1900 {
		return false
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Hostname() == "" || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range []string{".push.services.mozilla.com", ".googleapis.com", ".push.apple.com", ".notify.windows.com"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

func validPushKey(value string, min, max int) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(decoded) >= min && len(decoded) <= max
}

func mountPush(r chi.Router, db *sql.DB, config Options) {
	r.Route("/api/v1/push", func(r chi.Router) {
		r.Get("/vapid-public-key", func(w http.ResponseWriter, req *http.Request) {
			if _, ok := notificationIdentity(w, req, db); !ok {
				return
			}
			pub, _, _, ready := vapidConfig()
			if !ready {
				writeAPIError(w, req, 503, "PUSH_UNAVAILABLE", "Web Push belum dikonfigurasi.")
				return
			}
			writeAPIData(w, 200, map[string]string{"public_key": pub}, nil)
		})
		r.Get("/subscriptions", func(w http.ResponseWriter, req *http.Request) {
			p, ok := notificationIdentity(w, req, db)
			if !ok {
				return
			}
			rows, err := db.QueryContext(req.Context(), `SELECT id,user_agent,last_success_at FROM push_subscriptions WHERE tenant_id=? AND user_id=? AND revoked_at IS NULL ORDER BY id`, p.TenantID, p.UserID)
			if err != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca perangkat.")
				return
			}
			defer rows.Close()
			items := []map[string]any{}
			for rows.Next() {
				var id string
				var agent sql.NullString
				var last sql.NullTime
				if rows.Scan(&id, &agent, &last) != nil {
					writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca perangkat.")
					return
				}
				item := map[string]any{"id": id, "user_agent": agent.String}
				if last.Valid {
					item["last_success_at"] = last.Time
				}
				items = append(items, item)
			}
			if rows.Err() != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca perangkat.")
				return
			}
			writeAPIData(w, 200, items, nil)
		})
		r.With(checkOrigin(config), requireCSRF).Post("/subscriptions", func(w http.ResponseWriter, req *http.Request) {
			p, ok := notificationIdentity(w, req, db)
			if !ok {
				return
			}
			if _, _, _, ready := vapidConfig(); !ready {
				writeAPIError(w, req, 503, "PUSH_UNAVAILABLE", "Web Push belum dikonfigurasi.")
				return
			}
			var in pushInput
			if !readJSON(w, req, &in) {
				return
			}
			if !validPushEndpoint(in.Endpoint) || !validPushKey(in.P256dh, 65, 65) || !validPushKey(in.Auth, 16, 32) {
				writeAPIError(w, req, 422, "VALIDATION_FAILED", "Langganan push tidak valid.")
				return
			}
			tx, err := db.BeginTx(req.Context(), nil)
			if err != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal menyimpan perangkat.")
				return
			}
			defer tx.Rollback()
			var tenant string
			if err = tx.QueryRowContext(req.Context(), `SELECT id FROM tenants WHERE id=? FOR UPDATE`, p.TenantID).Scan(&tenant); err != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal menyimpan perangkat.")
				return
			}
			key, version, err := activeTenantDataKey(req.Context(), tx, p.TenantID)
			if err != nil {
				writeAPIError(w, req, 503, "ENCRYPTION_UNAVAILABLE", "Kunci tenant tidak tersedia.")
				return
			}
			hash := sha256.Sum256([]byte(in.Endpoint))
			var id string
			err = tx.QueryRowContext(req.Context(), `SELECT id FROM push_subscriptions WHERE tenant_id=? AND user_id=? AND endpoint_hash=? FOR UPDATE`, p.TenantID, p.UserID, hash[:]).Scan(&id)
			if errors.Is(err, sql.ErrNoRows) {
				id, err = ids.New()
			}
			if err != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal menyimpan perangkat.")
				return
			}
			endpoint, e1 := cryptodata.EncryptField(key, p.TenantID, id, "push.endpoint", version, []byte(in.Endpoint))
			p256dh, e2 := cryptodata.EncryptField(key, p.TenantID, id, "push.p256dh", version, []byte(in.P256dh))
			auth, e3 := cryptodata.EncryptField(key, p.TenantID, id, "push.auth", version, []byte(in.Auth))
			if e1 != nil || e2 != nil || e3 != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal mengenkripsi perangkat.")
				return
			}
			_, err = tx.ExecContext(req.Context(), `INSERT INTO push_subscriptions (tenant_id,id,user_id,endpoint_hash,endpoint_enc,p256dh_enc,auth_enc,key_version,user_agent) VALUES (?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE endpoint_enc=VALUES(endpoint_enc),p256dh_enc=VALUES(p256dh_enc),auth_enc=VALUES(auth_enc),key_version=VALUES(key_version),user_agent=VALUES(user_agent),revoked_at=NULL`, p.TenantID, id, p.UserID, hash[:], endpoint, p256dh, auth, version, req.UserAgent())
			if err == nil {
				err = tx.Commit()
			}
			if err != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal menyimpan perangkat.")
				return
			}
			writeAPIData(w, 201, map[string]string{"id": id}, nil)
		})
		r.With(checkOrigin(config), requireCSRF).Delete("/subscriptions/{id}", func(w http.ResponseWriter, req *http.Request) {
			p, ok := notificationIdentity(w, req, db)
			if !ok {
				return
			}
			res, err := db.ExecContext(req.Context(), `UPDATE push_subscriptions SET revoked_at=COALESCE(revoked_at,UTC_TIMESTAMP(6)) WHERE tenant_id=? AND user_id=? AND id=?`, p.TenantID, p.UserID, chi.URLParam(req, "id"))
			if err != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal mencabut perangkat.")
				return
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				writeAPIError(w, req, 404, "NOT_FOUND", "Perangkat tidak ditemukan.")
				return
			}
			w.WriteHeader(204)
		})
		r.With(checkOrigin(config), requireCSRF).Post("/test", func(w http.ResponseWriter, req *http.Request) {
			p, ok := notificationIdentity(w, req, db)
			if !ok {
				return
			}
			var in struct {
				SubscriptionID string `json:"subscription_id"`
			}
			if !readJSON(w, req, &in) {
				return
			}
			if !fileIDPattern.MatchString(in.SubscriptionID) {
				writeAPIError(w, req, 422, "VALIDATION_FAILED", "Perangkat tidak valid.")
				return
			}
			err := sendPushToSubscription(req, db, p.TenantID, p.UserID, in.SubscriptionID, []byte(`{"title":"Uji notifikasi","body":"Push dari digaji.id berhasil."}`))
			if errors.Is(err, sql.ErrNoRows) {
				writeAPIError(w, req, 404, "NOT_FOUND", "Perangkat tidak ditemukan.")
				return
			}
			if err != nil {
				writeAPIError(w, req, 503, "PUSH_DELIVERY_FAILED", "Push tidak dapat dikirim.")
				return
			}
			writeAPIData(w, 200, map[string]string{"status": "delivered"}, nil)
		})
	})
}

func sendPushToSubscription(req *http.Request, db *sql.DB, tenantID, userID, id string, payload []byte) error {
	pub, priv, subscriber, ready := vapidConfig()
	if !ready {
		return errors.New("vapid unavailable")
	}
	var endpointEnc, p256dhEnc, authEnc []byte
	var version uint32
	err := db.QueryRowContext(req.Context(), `SELECT endpoint_enc,p256dh_enc,auth_enc,key_version FROM push_subscriptions WHERE tenant_id=? AND user_id=? AND id=? AND revoked_at IS NULL`, tenantID, userID, id).Scan(&endpointEnc, &p256dhEnc, &authEnc, &version)
	if err != nil {
		return err
	}
	key, err := loadTenantDataKey(req.Context(), db, tenantID, version)
	if err != nil {
		return err
	}
	endpoint, err := cryptodata.DecryptField(key, endpointEnc, tenantID, id, "push.endpoint", version)
	if err != nil {
		return err
	}
	p256dh, err := cryptodata.DecryptField(key, p256dhEnc, tenantID, id, "push.p256dh", version)
	if err != nil {
		return err
	}
	auth, err := cryptodata.DecryptField(key, authEnc, tenantID, id, "push.auth", version)
	if err != nil {
		return err
	}
	if !validPushEndpoint(string(endpoint)) {
		return errors.New("invalid endpoint")
	}
	res, err := webpush.SendNotificationWithContext(req.Context(), payload, &webpush.Subscription{Endpoint: string(endpoint), Keys: webpush.Keys{P256dh: string(p256dh), Auth: string(auth)}}, &webpush.Options{Subscriber: subscriber, VAPIDPublicKey: pub, VAPIDPrivateKey: priv, TTL: 60})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusGone || res.StatusCode == http.StatusNotFound {
		_, _ = db.ExecContext(req.Context(), `UPDATE push_subscriptions SET revoked_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, tenantID, id)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return errors.New("push provider rejected request")
	}
	_, err = db.ExecContext(req.Context(), `UPDATE push_subscriptions SET last_success_at=? WHERE tenant_id=? AND id=?`, time.Now().UTC(), tenantID, id)
	return err
}

func pushPayload(title, link string) []byte {
	raw, _ := json.Marshal(map[string]string{"title": title, "body": "Ada pembaruan di digaji.id.", "url": link})
	return raw
}
