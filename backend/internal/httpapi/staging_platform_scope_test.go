package httpapi

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
)

func TestStagingPlatformStaffTenantScopeAndDeny(t *testing.T) {
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
	t.Cleanup(func() { db.Close() })
	userID, err := ids.New()
	if err != nil {
		t.Fatal(err)
	}
	foreignTenantID, err := ids.New()
	if err != nil {
		t.Fatal(err)
	}
	var demoTenantID, supportRoleID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM tenants WHERE slug='client-pertama'`).Scan(&demoTenantID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id FROM platform_roles WHERE code='SUPPORT'`).Scan(&supportRoleID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		steps := []struct {
			query string
			args  []any
		}{
			{`DELETE FROM platform_user_permission_overrides WHERE user_id=?`, []any{userID}},
			{`DELETE FROM platform_user_tenants WHERE user_id=?`, []any{userID}},
			{`DELETE FROM platform_user_roles WHERE user_id=?`, []any{userID}},
			{`DELETE FROM login_events WHERE user_id=?`, []any{userID}},
			{`DELETE FROM sessions WHERE user_id=?`, []any{userID}},
			{`DELETE FROM users WHERE id=?`, []any{userID}},
			{`DELETE FROM tenants WHERE id=?`, []any{foreignTenantID}},
		}
		for _, step := range steps {
			if _, err := db.Exec(step.query, step.args...); err != nil {
				t.Errorf("cleanup platform scope: %v", err)
			}
		}
	})
	password := "Qa-Platform-Password-29"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	email := strings.ToLower("qa-platform-" + userID + "@example.invalid")
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id,email,email_normalized,display_name,password_hash) VALUES (?,?,?,'QA Platform Support',?)`, userID, email, email, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO platform_user_roles (user_id,role_id,valid_from) VALUES (?,?,CURRENT_DATE())`, userID, supportRoleID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO tenants (id,slug,name,status) VALUES (?,?,?,'active')`, foreignTenantID, strings.ToLower("qa-platform-"+foreignTenantID), "QA Foreign Platform Tenant"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(Router(db, Options{CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var state auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/login", map[string]any{"email": email, "password": password}, 200, &state)
	if !state.Platform {
		t.Fatal("support role did not open platform panel")
	}
	var tenants pageResult[struct {
		ID string `json:"id"`
	}]
	doJSON(t, client, "GET", server.URL+"/api/v1/platform/tenants", nil, 200, &tenants)
	if tenants.Total != 0 {
		t.Fatalf("unassigned support saw %d tenants", tenants.Total)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO platform_user_tenants (user_id,tenant_id) VALUES (?,?)`, userID, demoTenantID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/platform/tenants", nil, 200, &tenants)
	if tenants.Total != 1 || len(tenants.Items) != 1 || tenants.Items[0].ID != demoTenantID {
		t.Fatalf("support tenant scope leaked: %+v", tenants)
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/platform/tenants?q=QA%20Foreign", nil, 200, &tenants)
	if tenants.Total != 0 {
		t.Fatalf("tenant search bypassed assignment: %+v", tenants)
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/platform/tenants?q=client-pertama", nil, 200, &tenants)
	if tenants.Total != 1 || tenants.Items[0].ID != demoTenantID {
		t.Fatalf("assigned tenant search failed: %+v", tenants)
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/platform/staff", nil, 403, nil)
	if _, err := db.ExecContext(ctx, `INSERT INTO platform_user_permission_overrides (user_id,permission_code,decision) VALUES (?,'platform.perusahaan.lihat','deny')`, userID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/platform/tenants", nil, 403, nil)
}
