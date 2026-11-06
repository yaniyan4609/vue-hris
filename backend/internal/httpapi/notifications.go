package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/auth"
	"github.com/go-chi/chi/v5"
)

type notificationPrincipal struct {
	TenantID string
	UserID   string
	Version  uint64
}

type notificationItem struct {
	ID        string     `json:"id"`
	Category  string     `json:"category"`
	EventType string     `json:"event_type"`
	Title     string     `json:"title"`
	Body      string     `json:"body,omitempty"`
	DeepLink  *string    `json:"deep_link,omitempty"`
	Priority  string     `json:"priority"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`
	Archived  bool       `json:"archived"`
	Obsolete  bool       `json:"obsolete"`
	Sequence  uint64     `json:"-"`
}

func notificationIdentity(w http.ResponseWriter, r *http.Request, db *sql.DB) (notificationPrincipal, bool) {
	state, err := (auth.Service{DB: db}).Current(r.Context(), cookieToken(r))
	if err != nil {
		writeAPIError(w, r, http.StatusUnauthorized, "AUTH_SESSION_INVALID", "Sesi tidak sah atau berakhir.")
		return notificationPrincipal{}, false
	}
	var tenant auth.Tenant
	for _, item := range state.Tenants {
		if item.ID == state.SelectedTenantID {
			tenant = item
			break
		}
	}
	if tenant.ID == "" {
		writeAPIError(w, r, http.StatusForbidden, "PERMISSION_DENIED", "Pilih perusahaan terlebih dahulu.")
		return notificationPrincipal{}, false
	}
	_, decision, err := (access.Store{DB: db}).Decide(r.Context(), tenant.ID, state.User.ID, tenant.Timezone, "notifikasi.lihat_sendiri")
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Gagal memeriksa akses.")
		return notificationPrincipal{}, false
	}
	if !decision.Allowed {
		code := "PERMISSION_DENIED"
		if decision.Reason == "entitlement_missing" {
			code = "ENTITLEMENT_REQUIRED"
		}
		writeAPIError(w, r, http.StatusForbidden, code, "Akses notifikasi tidak diizinkan.")
		return notificationPrincipal{}, false
	}
	var version uint64
	if err := db.QueryRowContext(r.Context(), `SELECT access_version FROM tenant_memberships WHERE tenant_id=? AND user_id=? AND status='active'`, tenant.ID, state.User.ID).Scan(&version); err != nil {
		writeAPIError(w, r, http.StatusForbidden, "PERMISSION_DENIED", "Keanggotaan tidak aktif.")
		return notificationPrincipal{}, false
	}
	w.Header().Set("X-Access-Version", strconv.FormatUint(version, 10))
	return notificationPrincipal{TenantID: tenant.ID, UserID: state.User.ID, Version: version}, true
}

func mountNotifications(r chi.Router, db *sql.DB, config Options) {
	r.Route("/api/v1/notifications", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) { listNotifications(w, req, db, false) })
		r.Get("/sync", func(w http.ResponseWriter, req *http.Request) { listNotifications(w, req, db, true) })
		r.Get("/unread-count", func(w http.ResponseWriter, req *http.Request) {
			identity, ok := notificationIdentity(w, req, db)
			if !ok {
				return
			}
			count, err := unreadCount(req, db, identity)
			if err != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal menghitung notifikasi.")
				return
			}
			writeAPIData(w, 200, map[string]int{"count": count}, nil)
		})
		r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
			identity, ok := notificationIdentity(w, req, db)
			if !ok {
				return
			}
			item, err := notificationByID(req, db, identity, chi.URLParam(req, "id"))
			if errors.Is(err, sql.ErrNoRows) {
				writeAPIError(w, req, 404, "NOT_FOUND", "Notifikasi tidak ditemukan.")
				return
			}
			if err != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca notifikasi.")
				return
			}
			writeAPIData(w, 200, item, nil)
		})
		r.With(checkOrigin(config), requireCSRF).Post("/{id}/read", func(w http.ResponseWriter, req *http.Request) { mutateNotification(w, req, db, "read") })
		r.With(checkOrigin(config), requireCSRF).Post("/{id}/archive", func(w http.ResponseWriter, req *http.Request) { mutateNotification(w, req, db, "archive") })
		r.With(checkOrigin(config), requireCSRF).Post("/read-all", func(w http.ResponseWriter, req *http.Request) {
			identity, ok := notificationIdentity(w, req, db)
			if !ok {
				return
			}
			category := strings.TrimSpace(req.URL.Query().Get("category"))
			if len(category) > 48 {
				writeAPIError(w, req, 400, "VALIDATION_FAILED", "Kategori tidak valid.")
				return
			}
			query := `UPDATE notifications SET read_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND recipient_user_id=? AND read_at IS NULL AND archived_at IS NULL`
			args := []any{identity.TenantID, identity.UserID}
			if category != "" {
				query += " AND category=?"
				args = append(args, category)
			}
			if _, err := db.ExecContext(req.Context(), query, args...); err != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal menandai notifikasi.")
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})
}

func notificationCursor(r *http.Request, db *sql.DB, p notificationPrincipal, id string) (uint64, error) {
	var seq uint64
	err := db.QueryRowContext(r.Context(), `SELECT sequence_no FROM notifications WHERE tenant_id=? AND recipient_user_id=? AND id=?`, p.TenantID, p.UserID, id).Scan(&seq)
	return seq, err
}

func listNotifications(w http.ResponseWriter, r *http.Request, db *sql.DB, syncMode bool) {
	p, ok := notificationIdentity(w, r, db)
	if !ok {
		return
	}
	limit := 30
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Limit harus 1–100.")
			return
		}
	}
	seq := uint64(0)
	id := r.URL.Query().Get("cursor")
	if syncMode {
		id = r.URL.Query().Get("after_id")
	}
	if id != "" {
		var err error
		seq, err = notificationCursor(r, db, p, id)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Cursor tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca cursor.")
			return
		}
	}
	query := `SELECT id,sequence_no,category,event_type,title,body,deep_link,priority,created_at,read_at,archived_at,obsolete_at FROM notifications WHERE tenant_id=? AND recipient_user_id=?`
	args := []any{p.TenantID, p.UserID}
	if syncMode {
		query += " AND sequence_no>?"
		args = append(args, seq)
	} else {
		query += " AND archived_at IS NULL"
		if id != "" {
			query += " AND sequence_no<?"
			args = append(args, seq)
		}
		if category := r.URL.Query().Get("category"); category != "" {
			query += " AND category=?"
			args = append(args, category)
		}
		if unread := r.URL.Query().Get("unread"); unread == "true" {
			query += " AND read_at IS NULL"
		} else if unread != "" && unread != "false" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter unread tidak valid.")
			return
		}
	}
	if syncMode {
		query += " ORDER BY sequence_no ASC LIMIT ?"
	} else {
		query += " ORDER BY sequence_no DESC LIMIT ?"
	}
	args = append(args, limit+1)
	rows, err := db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca notifikasi.")
		return
	}
	defer rows.Close()
	items := []notificationItem{}
	for rows.Next() {
		item, err := scanNotification(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca notifikasi.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca notifikasi.")
		return
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	var next any
	if more && len(items) > 0 {
		next = items[len(items)-1].ID
	}
	writeAPIData(w, 200, items, map[string]any{"next_cursor": next})
}

type notificationScanner interface{ Scan(...any) error }

func scanNotification(row notificationScanner) (notificationItem, error) {
	var item notificationItem
	var link sql.NullString
	var read, archived, obsolete sql.NullTime
	err := row.Scan(&item.ID, &item.Sequence, &item.Category, &item.EventType, &item.Title, &item.Body, &link, &item.Priority, &item.CreatedAt, &read, &archived, &obsolete)
	if err != nil {
		return item, err
	}
	if link.Valid {
		item.DeepLink = &link.String
	}
	if read.Valid {
		item.ReadAt = &read.Time
	}
	item.Archived, item.Obsolete = archived.Valid, obsolete.Valid
	return item, nil
}

func notificationByID(r *http.Request, db *sql.DB, p notificationPrincipal, id string) (notificationItem, error) {
	row := db.QueryRowContext(r.Context(), `SELECT id,sequence_no,category,event_type,title,body,deep_link,priority,created_at,read_at,archived_at,obsolete_at FROM notifications WHERE tenant_id=? AND recipient_user_id=? AND id=?`, p.TenantID, p.UserID, id)
	return scanNotification(row)
}

func unreadCount(r *http.Request, db *sql.DB, p notificationPrincipal) (int, error) {
	var count int
	err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM notifications WHERE tenant_id=? AND recipient_user_id=? AND read_at IS NULL AND archived_at IS NULL AND obsolete_at IS NULL`, p.TenantID, p.UserID).Scan(&count)
	return count, err
}

func mutateNotification(w http.ResponseWriter, r *http.Request, db *sql.DB, action string) {
	p, ok := notificationIdentity(w, r, db)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Notifikasi tidak ditemukan.")
		return
	}
	column := "read_at"
	if action == "archive" {
		column = "archived_at"
	}
	result, err := db.ExecContext(r.Context(), `UPDATE notifications SET `+column+`=COALESCE(`+column+`,UTC_TIMESTAMP(6)) WHERE tenant_id=? AND recipient_user_id=? AND id=?`, p.TenantID, p.UserID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah notifikasi.")
		return
	}
	count, err := result.RowsAffected()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah notifikasi.")
		return
	}
	if count == 0 {
		// MySQL may report zero changed rows for an already-read notification.
		if _, err := notificationByID(r, db, p, id); errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Notifikasi tidak ditemukan.")
			return
		} else if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca notifikasi.")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
