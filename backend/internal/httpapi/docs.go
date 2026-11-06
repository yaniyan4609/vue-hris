package httpapi

import (
	"bytes"
	"embed"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

//go:embed apidocs/index.html apidocs/init.js apidocs/openapi.yaml apidocs/swagger-ui.css apidocs/swagger-ui-bundle.js
var apiDocs embed.FS

func mountAPIDocs(r chi.Router) {
	r.Get("/api-docs", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/api-docs/", http.StatusPermanentRedirect)
	})
	r.Get("/api/v1/docs", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/api/v1/docs/", http.StatusPermanentRedirect)
	})
	for _, prefix := range []string{"/api-docs/", "/api/v1/docs/"} {
		prefix := prefix
		r.Get(prefix, func(w http.ResponseWriter, req *http.Request) {
			serveAPIDoc(w, req, "index.html")
		})
		r.Get(prefix+"{file}", func(w http.ResponseWriter, req *http.Request) {
			serveAPIDoc(w, req, chi.URLParam(req, "file"))
		})
	}
}

func serveAPIDoc(w http.ResponseWriter, r *http.Request, name string) {
	var contentType string
	switch name {
	case "index.html":
		contentType = "text/html; charset=utf-8"
	case "init.js", "swagger-ui-bundle.js":
		contentType = "text/javascript; charset=utf-8"
	case "swagger-ui.css":
		contentType = "text/css; charset=utf-8"
	case "openapi.yaml":
		contentType = "application/yaml; charset=utf-8"
	default:
		http.NotFound(w, r)
		return
	}
	data, err := apiDocs.ReadFile("apidocs/" + name)
	if err != nil {
		http.Error(w, "API documentation unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasSuffix(name, ".html") || name == "openapi.yaml" {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
