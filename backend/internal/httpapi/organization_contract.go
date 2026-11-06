package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-chi/chi/v5"
)

// The canonical routes reuse the audited, tenant-scoped organization handlers.
// The adapter exists only while the Vue client still consumes the old DTOs.
func (a adminAPI) mountOrganizationContract(r chi.Router, config Options) {
	for kind := range orgKinds {
		kind := kind
		base := "/" + kind
		r.Get(base, a.orgContract(kind, a.listOrganization))
		r.Get(base+"/{id}", a.getOrganizationContract(kind))
		r.With(checkOrigin(config)).Post(base, a.orgContract(kind, a.createOrganization))
		r.With(checkOrigin(config)).Patch(base+"/{id}", a.orgContract(kind, a.updateOrganization))
	}
}

func (a adminAPI) orgContract(kind string, legacy http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request := r.Clone(r.Context())
		urlCopy := *r.URL
		query := urlCopy.Query()
		if value := query.Get("per_page"); value != "" {
			query.Set("pageSize", value)
		}
		if value := query.Get("parent_id"); value != "" {
			query.Set("parentId", value)
		}
		if sort := query.Get("sort"); strings.HasPrefix(sort, "-") {
			query.Set("sort", strings.TrimPrefix(sort, "-"))
			query.Set("order", "desc")
		}
		urlCopy.RawQuery = query.Encode()
		request.URL = &urlCopy
		chi.RouteContext(request.Context()).URLParams.Add("kind", kind)
		if r.Method == http.MethodPost || r.Method == http.MethodPatch {
			var payload map[string]json.RawMessage
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&payload); err != nil || payload == nil {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "JSON tidak valid.")
				return
			}
			if parent, ok := payload["parent_id"]; ok {
				payload["parentId"] = parent
				delete(payload, "parent_id")
			}
			body, _ := json.Marshal(payload)
			request.Body = io.NopCloser(bytes.NewReader(body))
			request.ContentLength = int64(len(body))
		}
		recorder := httptest.NewRecorder()
		legacy(recorder, request)
		if recorder.Code >= 400 {
			legacyContractError(w, r, recorder.Code, recorder.Body.Bytes())
			return
		}
		if r.Method == http.MethodGet {
			var page pageResult[orgNode]
			if err := json.Unmarshal(recorder.Body.Bytes(), &page); err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca organisasi.")
				return
			}
			items := make([]map[string]any, 0, len(page.Items))
			for _, item := range page.Items {
				items = append(items, organizationData(item))
			}
			writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": page.Total})
			return
		}
		var item orgNode
		if err := json.Unmarshal(recorder.Body.Bytes(), &item); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca organisasi.")
			return
		}
		writeAPIData(w, recorder.Code, organizationData(item), nil)
	}
}

func (a adminAPI) getOrganizationContract(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, ok := a.authorize(w, r, "organisasi.lihat")
		if !ok {
			return
		}
		id := chi.URLParam(r, "id")
		if len(id) != 26 {
			writeAPIError(w, r, 404, "NOT_FOUND", "Data organisasi tidak ditemukan.")
			return
		}
		config := orgKinds[kind]
		columns := "id,code,name"
		if config.parentColumn != "" {
			columns += "," + config.parentColumn
		}
		if config.hasTimezone {
			columns += ",timezone"
		}
		var item orgNode
		dest := []any{&item.ID, &item.Code, &item.Name}
		if config.parentColumn != "" {
			dest = append(dest, &item.ParentID)
		}
		if config.hasTimezone {
			dest = append(dest, &item.Timezone)
		}
		err := a.db.QueryRowContext(r.Context(), "SELECT "+columns+" FROM "+config.table+" WHERE tenant_id=? AND id=?", c.tenant.ID, id).Scan(dest...)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Data organisasi tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca organisasi.")
			return
		}
		writeAPIData(w, 200, organizationData(item), nil)
	}
}

func organizationData(item orgNode) map[string]any {
	result := map[string]any{"id": item.ID, "code": item.Code, "name": item.Name}
	if item.ParentID != "" {
		result["parent_id"] = item.ParentID
	}
	if item.Timezone != "" {
		result["timezone"] = item.Timezone
	}
	return result
}
