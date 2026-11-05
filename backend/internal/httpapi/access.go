package httpapi

import (
	"database/sql"
	"net/http"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/auth"
	"github.com/go-chi/chi/v5"
)

type accessEntry struct {
	Allowed bool               `json:"allowed"`
	Reason  string             `json:"reason"`
	Sources []string           `json:"sources,omitempty"`
	Scopes  []access.ScopeRule `json:"scopes,omitempty"`
}

type accessState struct {
	TenantID    string                 `json:"tenantId,omitempty"`
	Admin       bool                   `json:"admin"`
	ESS         bool                   `json:"ess"`
	Platform    bool                   `json:"platform"`
	Permissions map[string]accessEntry `json:"permissions"`
}

func mountAccess(r chi.Router, db *sql.DB) {
	service := auth.Service{DB: db}
	store := access.Store{DB: db}
	r.Get("/api/v1/me/access", func(w http.ResponseWriter, req *http.Request) {
		state, err := service.Current(req.Context(), cookieToken(req))
		if err != nil {
			writeAuthError(w, err)
			return
		}
		result := accessState{Platform: state.Platform, Permissions: map[string]accessEntry{}}
		var tenant auth.Tenant
		for _, item := range state.Tenants {
			if item.ID == state.SelectedTenantID {
				tenant = item
				break
			}
		}
		if tenant.ID == "" {
			writeJSON(w, http.StatusOK, result)
			return
		}
		result.TenantID, result.Admin, result.ESS = tenant.ID, tenant.Admin, tenant.ESS
		rows, err := db.QueryContext(req.Context(), "SELECT code FROM permission_registry ORDER BY code")
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca hak akses"})
			return
		}
		var codes []string
		for rows.Next() {
			var code string
			if err := rows.Scan(&code); err != nil {
				rows.Close()
				writeJSON(w, 500, map[string]string{"error": "Gagal membaca hak akses"})
				return
			}
			codes = append(codes, code)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca hak akses"})
			return
		}
		rows.Close()
		for _, code := range codes {
			_, decision, err := store.Decide(req.Context(), tenant.ID, state.User.ID, tenant.Timezone, code)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "Gagal membaca hak akses"})
				return
			}
			result.Permissions[code] = accessEntry{Allowed: decision.Allowed, Reason: decision.Reason, Sources: decision.Sources, Scopes: decision.Scopes}
		}
		writeJSON(w, 200, result)
	})
}
