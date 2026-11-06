package httpapi

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAPIDocsRoutes(t *testing.T) {
	server := httptest.NewServer(Router(nil))
	defer server.Close()
	for _, route := range []string{"/api-docs/", "/api-docs/init.js", "/api-docs/swagger-ui.css", "/api-docs/swagger-ui-bundle.js", "/api/v1/docs/", "/api/v1/docs/openapi.yaml"} {
		response, err := http.Get(server.URL + route)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK || len(body) == 0 {
			t.Fatalf("%s: status=%d bytes=%d error=%v", route, response.StatusCode, len(body), err)
		}
	}
	response, err := http.Get(server.URL + "/api-docs/unknown.js")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown asset status=%d", response.StatusCode)
	}
}

func TestEmbeddedOpenAPIMatchesSource(t *testing.T) {
	source, err := os.ReadFile("../../../docs/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := apiDocs.ReadFile("apidocs/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(source, embedded) {
		t.Fatal("embedded OpenAPI is stale; copy docs/openapi.yaml to backend/internal/httpapi/apidocs/openapi.yaml")
	}
}
