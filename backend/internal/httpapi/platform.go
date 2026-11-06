package httpapi

import (
	"database/sql"
	"net/http"
	"strings"

	"digaji.id/hris/internal/auth"
	"github.com/go-chi/chi/v5"
)

func mountPlatform(r chi.Router, db *sql.DB, config Options) {
	service := auth.Service{DB: db}
	r.Get("/api/v1/platform/staff", func(w http.ResponseWriter, req *http.Request) { listPlatformStaff(w, req, db) })
	r.With(checkOrigin(config)).Post("/api/v1/platform/staff", func(w http.ResponseWriter, req *http.Request) { createPlatformStaff(w, req, db) })
	r.Get("/api/v1/platform/staff/roles", func(w http.ResponseWriter, req *http.Request) { listPlatformRoles(w, req, db) })
	r.Get("/api/v1/platform/staff/{id}", func(w http.ResponseWriter, req *http.Request) { getPlatformStaff(w, req, db) })
	r.With(checkOrigin(config)).Post("/api/v1/platform/staff/roles", func(w http.ResponseWriter, req *http.Request) { assignPlatformRole(w, req, db) })
	r.With(checkOrigin(config)).Delete("/api/v1/platform/staff/{id}/roles/{code}", func(w http.ResponseWriter, req *http.Request) { revokePlatformRole(w, req, db) })
	r.With(checkOrigin(config)).Put("/api/v1/platform/staff/{id}/permissions/{code}", func(w http.ResponseWriter, req *http.Request) { updatePlatformOverride(w, req, db) })
	r.With(checkOrigin(config)).Put("/api/v1/platform/staff/{id}/tenants", func(w http.ResponseWriter, req *http.Request) { updatePlatformTenants(w, req, db) })
	r.Get("/api/v1/platform/tenants", func(w http.ResponseWriter, req *http.Request) {
		state, err := service.Current(req.Context(), cookieToken(req))
		if err != nil {
			writeAuthError(w, err)
			return
		}
		if !state.Platform {
			writeJSON(w, 403, map[string]string{"error": "Akses platform tidak diizinkan"})
			return
		}
		grants, superadmin, err := platformAccess(req.Context(), db, state.User.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal memeriksa akses"})
			return
		}
		if !grants["platform.perusahaan.lihat"].Allowed {
			writeJSON(w, 403, map[string]string{"error": "Akses perusahaan tidak diizinkan"})
			return
		}
		page, ok := readPage(w, req)
		if !ok {
			return
		}
		q, ok := searchTerm(w, req)
		if !ok {
			return
		}
		where := "1=1"
		args := []any{}
		if !superadmin {
			where = "EXISTS (SELECT 1 FROM platform_user_tenants put WHERE put.tenant_id=tenants.id AND put.user_id=?)"
			args = append(args, state.User.ID)
		}
		if q != "" {
			where += " AND (LOCATE(LOWER(?), LOWER(name))>0 OR LOCATE(LOWER(?), LOWER(slug))>0)"
			args = append(args, q, q)
		}
		status := strings.TrimSpace(req.URL.Query().Get("status"))
		if status != "" {
			where += " AND status=?"
			args = append(args, status)
		}
		var total int
		if err := db.QueryRowContext(req.Context(), "SELECT COUNT(*) FROM tenants WHERE "+where, args...).Scan(&total); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca jumlah perusahaan"})
			return
		}
		rows, err := db.QueryContext(req.Context(), "SELECT id, slug, name, status FROM tenants WHERE "+where+" ORDER BY name,id LIMIT ? OFFSET ?", append(args, page.PageSize, page.Offset)...)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca perusahaan"})
			return
		}
		defer rows.Close()
		type tenant struct {
			ID     string `json:"id"`
			Slug   string `json:"slug"`
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		items := []tenant{}
		for rows.Next() {
			var item tenant
			if err := rows.Scan(&item.ID, &item.Slug, &item.Name, &item.Status); err != nil {
				writeJSON(w, 500, map[string]string{"error": "Gagal membaca perusahaan"})
				return
			}
			items = append(items, item)
		}
		if rows.Err() != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca perusahaan"})
			return
		}
		writeJSON(w, 200, pageResult[tenant]{Items: items, Total: total, Page: page.Page, PageSize: page.PageSize})
	})
}
