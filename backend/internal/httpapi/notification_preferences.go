package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type notificationPreference struct {
	Category string `json:"category"`
	Channel  string `json:"channel"`
	Enabled  bool   `json:"enabled"`
}

func validNotificationCategory(category string) bool {
	switch category {
	case "request", "reminder", "broadcast", "system":
		return true
	}
	return false
}

func validNotificationChannel(channel string) bool {
	switch channel {
	case "inapp", "push", "email":
		return true
	}
	return false
}

func mountNotificationPreferences(r chi.Router, db *sql.DB, config Options) {
	r.Get("/preferences", func(w http.ResponseWriter, req *http.Request) {
		p, ok := notificationIdentity(w, req, db)
		if !ok {
			return
		}
		rows, err := db.QueryContext(req.Context(), `SELECT category,channel,enabled FROM notification_preferences WHERE tenant_id=? AND user_id=? ORDER BY category,channel`, p.TenantID, p.UserID)
		if err != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca preferensi.")
			return
		}
		defer rows.Close()
		overrides := map[string]bool{}
		for rows.Next() {
			var item notificationPreference
			if rows.Scan(&item.Category, &item.Channel, &item.Enabled) != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca preferensi.")
				return
			}
			overrides[item.Category+":"+item.Channel] = item.Enabled
		}
		if rows.Err() != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca preferensi.")
			return
		}
		items := make([]notificationPreference, 0, 12)
		for _, category := range []string{"request", "reminder", "broadcast", "system"} {
			for _, channel := range []string{"inapp", "push", "email"} {
				enabled, found := overrides[category+":"+channel]
				if !found {
					enabled = channel == "inapp"
				}
				if category == "system" {
					enabled = true
				}
				items = append(items, notificationPreference{Category: category, Channel: channel, Enabled: enabled})
			}
		}
		writeAPIData(w, 200, items, nil)
	})
	r.With(checkOrigin(config), requireCSRF).Put("/preferences", func(w http.ResponseWriter, req *http.Request) {
		p, ok := notificationIdentity(w, req, db)
		if !ok {
			return
		}
		var body struct {
			Items []notificationPreference `json:"items"`
		}
		if !readJSON(w, req, &body) {
			return
		}
		if len(body.Items) > 12 {
			writeAPIError(w, req, 422, "VALIDATION_FAILED", "Terlalu banyak preferensi.")
			return
		}
		seen := map[string]bool{}
		for _, item := range body.Items {
			if !validNotificationCategory(item.Category) || !validNotificationChannel(item.Channel) {
				writeAPIError(w, req, 422, "VALIDATION_FAILED", "Jenis atau kanal tidak valid.")
				return
			}
			if item.Category == "system" && !item.Enabled {
				writeAPIError(w, req, 422, "MANDATORY_NOTIFICATION", "Notifikasi wajib tidak dapat dimatikan.")
				return
			}
			key := item.Category + ":" + item.Channel
			if seen[key] {
				writeAPIError(w, req, 422, "VALIDATION_FAILED", "Preferensi ganda.")
				return
			}
			seen[key] = true
		}
		tx, err := db.BeginTx(req.Context(), nil)
		if err != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal menyimpan preferensi.")
			return
		}
		defer tx.Rollback()
		for _, item := range body.Items {
			_, err = tx.ExecContext(req.Context(), `INSERT INTO notification_preferences (tenant_id,user_id,category,channel,enabled) VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE enabled=VALUES(enabled)`, p.TenantID, p.UserID, item.Category, item.Channel, item.Enabled)
			if err != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal menyimpan preferensi.")
				return
			}
		}
		if err = tx.Commit(); err != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal menyimpan preferensi.")
			return
		}
		writeAPIData(w, 200, body.Items, nil)
	})
}
