package httpapi

import (
	"database/sql"
	"net/http"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/auth"
	"github.com/go-chi/chi/v5"
)

type accessEntry struct {
	Allowed  bool               `json:"allowed"`
	Reason   string             `json:"reason"`
	Sources  []string           `json:"sources,omitempty"`
	Scopes   []access.ScopeRule `json:"scopes,omitempty"`
	Override string             `json:"override,omitempty"`
}

type accessState struct {
	TenantID            string                 `json:"tenantId,omitempty"`
	Admin               bool                   `json:"admin"`
	ESS                 bool                   `json:"ess"`
	Platform            bool                   `json:"platform"`
	PlatformSuperadmin  bool                   `json:"platformSuperadmin"`
	Permissions         map[string]accessEntry `json:"permissions"`
	PlatformPermissions map[string]accessEntry `json:"platformPermissions"`
	Entitlements        map[string]bool        `json:"entitlements"`
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
		result := accessState{Platform: state.Platform, Permissions: map[string]accessEntry{}, PlatformPermissions: map[string]accessEntry{}, Entitlements: map[string]bool{}}
		if state.Platform {
			grants, superadmin, err := platformAccess(req.Context(), db, state.User.ID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "Gagal membaca hak akses platform"})
				return
			}
			result.PlatformPermissions = grants
			result.PlatformSuperadmin = superadmin
		}
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
		decisions, entitlements, err := store.DecideAll(req.Context(), tenant.ID, state.User.ID, tenant.Timezone)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Gagal membaca hak akses"})
			return
		}
		result.Entitlements = entitlements
		for code, decision := range decisions {
			result.Permissions[code] = accessEntry{Allowed: decision.Allowed, Reason: decision.Reason, Sources: decision.Sources, Scopes: decision.Scopes}
		}
		writeJSON(w, 200, result)
	})
}
