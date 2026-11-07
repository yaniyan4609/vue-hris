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

func TestStagingEssAttendanceReads(t *testing.T) {
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
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, nil)
	var today struct {
		Data struct {
			WorkDate string               `json:"work_date"`
			Events   []essAttendanceEvent `json:"events"`
		} `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/ess/attendance/today", nil, 200, &today)
	if today.Data.WorkDate == "" || today.Data.Events == nil {
		t.Fatalf("today response: %#v", today)
	}
	var history struct {
		Data []essAttendanceEvent `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/ess/attendance/history?from=2099-11-11&to=2099-11-11", nil, 200, &history)
	var calendar struct {
		Data []essAttendanceDay `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/ess/attendance/calendar?month=2099-11", nil, 200, &calendar)
	var summary struct {
		Data struct {
			Month       string `json:"month"`
			PresentDays int    `json:"present_days"`
		} `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/ess/attendance/summary?month=2099-11", nil, 200, &summary)
	if summary.Data.Month != "2099-11" {
		t.Fatalf("summary: %#v", summary)
	}
	var location struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/ess/attendance/location-check", map[string]any{"lat": -6.2, "lng": 106.8, "accuracy_m": 12.5}, 200, &location)
	if location.Data.Status != "inside" && location.Data.Status != "outside" && location.Data.Status != "flexible" && location.Data.Status != "unknown" {
		t.Fatalf("location status: %#v", location)
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/ess/attendance/location-check", map[string]any{"lat": -91, "lng": 106.8, "accuracy_m": 12.5}, 400, nil)
	doJSON(t, client, "GET", server.URL+"/api/v1/ess/attendance/history?from=2099-11-11&to=2099-01-01", nil, 400, nil)
}
