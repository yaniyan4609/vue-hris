package httpapi

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"digaji.id/hris/internal/database"
)

func TestStagingLookups(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, nil)
	for _, kind := range []string{"regions", "offices", "divisions", "departments", "positions", "roles"} {
		var result struct {
			Data []lookupItem `json:"data"`
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}
		doJSON(t, client, "GET", server.URL+"/api/v1/lookups/"+kind+"?per_page=2", nil, 200, &result)
		if len(result.Data) > 2 || result.Meta.Total < len(result.Data) {
			t.Fatalf("invalid %s pagination: %#v", kind, result)
		}
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/lookups/employees", nil, 400, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/lookups/employees?for=roster&per_page=2", nil, 200, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/lookups/offices?per_page=101", nil, 400, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/lookups/unknown", nil, 404, nil)
}
