package httpapi

import (
	"database/sql"
	"net/http"
	"strings"

	"digaji.id/hris/internal/auth"
)

func mountPlatform(r interface {
	Get(string, http.HandlerFunc)
}, db *sql.DB) {
	service := auth.Service{DB: db}
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
		if q != "" {
			where = "(LOCATE(LOWER(?), LOWER(name))>0 OR LOCATE(LOWER(?), LOWER(slug))>0)"
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
