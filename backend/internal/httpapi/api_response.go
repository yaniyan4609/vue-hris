package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

func writeAPIData(w http.ResponseWriter, code int, data any, meta any) {
	body := map[string]any{"data": data}
	if meta != nil {
		body["meta"] = meta
	}
	writeJSON(w, code, body)
}

func writeAPIError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("X-Request-Id", middleware.GetReqID(r.Context()))
	writeJSON(w, status, map[string]any{"error": map[string]any{
		"code": code, "message": message, "request_id": middleware.GetReqID(r.Context()),
	}})
}
