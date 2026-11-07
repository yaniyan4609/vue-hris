package httpapi

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
)

func TestStagingGeofenceLifecycle(t *testing.T) {
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
	var state auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &state)
	var offices struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/offices?per_page=1", nil, 200, &offices)
	if len(offices.Data) != 1 {
		t.Fatal("demo office missing")
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/organization/tree", nil, 200, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/locations/tree", nil, 200, nil)
	polygon := geoPolygon{Type: "Polygon", Coordinates: [][][]float64{{{123.9, 48.1}, {124, 48.1}, {124, 48.2}, {123.9, 48.2}, {123.9, 48.1}}}}
	var created struct {
		Data geofence `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/offices/"+offices.Data[0].ID+"/geofences", map[string]any{"name": "QA polygon", "polygon": polygon}, 201, &created)
	defer func() {
		if _, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='office_geofences' AND entity_id=?`, state.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup geofence audit: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM office_geofences WHERE tenant_id=? AND id=?`, state.SelectedTenantID, created.Data.ID); err != nil {
			t.Errorf("cleanup geofence: %v", err)
		}
	}()
	if created.Data.ID == "" || !created.Data.IsActive {
		t.Fatalf("created: %#v", created)
	}
	var detail struct {
		Data geofence `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/geofences/"+created.Data.ID, nil, 200, &detail)
	if detail.Data.OfficeID != offices.Data[0].ID {
		t.Fatal("wrong office")
	}
	var point struct {
		Data struct {
			Inside bool `json:"inside"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/geofences/test-point", map[string]float64{"lat": 48.15, "lng": 123.95}, 200, &point)
	if !point.Data.Inside {
		t.Fatal("point inside polygon was rejected")
	}
	doJSON(t, client, "PATCH", server.URL+"/api/v1/admin/geofences/"+created.Data.ID, map[string]any{"name": "QA changed"}, 200, &detail)
	if detail.Data.Name != "QA changed" {
		t.Fatal("patch not applied")
	}
	doJSON(t, client, "DELETE", server.URL+"/api/v1/admin/geofences/"+created.Data.ID, nil, 204, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/geofences/"+created.Data.ID, nil, 200, &detail)
	if detail.Data.IsActive {
		t.Fatal("delete did not deactivate")
	}
}
