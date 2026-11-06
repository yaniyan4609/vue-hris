package httpapi

import (
	"context"
	"encoding/base64"
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
	t.Setenv("PLATFORM_MFA_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
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
	inviteEmail := strings.ToLower("qa-staff-invite-" + userID + "@example.invalid")
	if err := db.QueryRowContext(ctx, `SELECT id FROM tenants WHERE slug='client-pertama'`).Scan(&demoTenantID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id FROM platform_roles WHERE code='SUPPORT'`).Scan(&supportRoleID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		var invitedID string
		if err := db.QueryRow(`SELECT id FROM users WHERE email_normalized=?`, inviteEmail).Scan(&invitedID); err == nil {
			if _, err := db.Exec(`DELETE FROM platform_audit_logs WHERE target_user_id=?`, invitedID); err != nil {
				t.Errorf("cleanup invited staff audit: %v", err)
			}
			for _, query := range []string{`DELETE FROM password_reset_tokens WHERE user_id=?`, `DELETE FROM platform_user_tenants WHERE user_id=?`, `DELETE FROM platform_user_roles WHERE user_id=?`, `DELETE FROM users WHERE id=?`} {
				if _, err := db.Exec(query, invitedID); err != nil {
					t.Errorf("cleanup invited staff: %v", err)
				}
			}
		}
		steps := []struct {
			query string
			args  []any
		}{
			{`DELETE FROM platform_audit_logs WHERE actor_user_id=?`, []any{userID}},
			{`DELETE FROM platform_user_permission_overrides WHERE user_id=?`, []any{userID}},
			{`DELETE FROM platform_user_tenants WHERE user_id=?`, []any{userID}},
			{`DELETE FROM platform_user_roles WHERE user_id=?`, []any{userID}},
			{`DELETE FROM platform_mfa_recovery_codes WHERE user_id=?`, []any{userID}},
			{`DELETE FROM platform_mfa_credentials WHERE user_id=?`, []any{userID}},
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
	if _, err := db.ExecContext(ctx, `INSERT INTO tenants (id,slug,name,status) VALUES (?,?,?,'active')`, foreignTenantID, strings.ToLower("qa-platform-"+foreignTenantID), "QA Foreign Platform Tenant"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(Router(db, Options{CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var state auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/login", map[string]any{"email": email, "password": password}, 200, &state)
	if state.Platform {
		t.Fatal("new QA user unexpectedly has platform role")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO platform_user_roles (user_id,role_id,valid_from) VALUES (?,?,CURRENT_DATE())`, userID, supportRoleID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/auth/me", nil, 200, &state)
	if !state.MFARequired {
		t.Fatal("role assignment must challenge an existing session")
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/me/access", nil, 403, nil)
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/logout", map[string]any{}, 204, nil)
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/login", map[string]any{"email": email, "password": password}, 200, &state)
	if !state.Platform {
		t.Fatal("support role did not open platform panel")
	}
	if !state.MFARequired {
		t.Fatal("platform login must require MFA")
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/me/access", nil, 403, nil)
	recoveryCode := completeStagingMFA(t, client, server.URL)
	secondJar, _ := cookiejar.New(nil)
	second := &http.Client{Jar: secondJar, Timeout: 10 * time.Second}
	doJSON(t, second, "POST", server.URL+"/api/v1/auth/login", map[string]any{"email": email, "password": password}, 200, &state)
	if !state.MFARequired {
		t.Fatal("second platform session must require MFA")
	}
	doJSON(t, second, "GET", server.URL+"/api/v1/auth/me", nil, 200, &state)
	doJSON(t, second, "POST", server.URL+"/api/v1/auth/mfa/verify", map[string]string{"code": "invalid"}, 403, nil)
	doJSON(t, second, "POST", server.URL+"/api/v1/auth/mfa/verify", map[string]string{"code": recoveryCode}, 200, nil)
	thirdJar, _ := cookiejar.New(nil)
	third := &http.Client{Jar: thirdJar, Timeout: 10 * time.Second}
	doJSON(t, third, "POST", server.URL+"/api/v1/auth/login", map[string]any{"email": email, "password": password}, 200, &state)
	doJSON(t, third, "POST", server.URL+"/api/v1/auth/mfa/verify", map[string]string{"code": recoveryCode}, 403, nil)
	for i := 0; i < 4; i++ {
		doJSON(t, third, "POST", server.URL+"/api/v1/auth/mfa/verify", map[string]string{"code": "000000"}, 403, nil)
	}
	doJSON(t, third, "GET", server.URL+"/api/v1/auth/me", nil, 401, nil)
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
	var superRoleID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM platform_roles WHERE code='SUPERADMIN'`).Scan(&superRoleID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO platform_user_roles (user_id,role_id,valid_from) VALUES (?,?,CURRENT_DATE())`, userID, superRoleID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": password}, 200, nil)
	t.Setenv("SMTP_HOST", "127.0.0.1")
	t.Setenv("SMTP_PORT", "1")
	t.Setenv("SMTP_FROM", "noreply@example.invalid")
	t.Setenv("APP_ORIGIN", server.URL)
	var invited struct {
		Data struct {
			ID               string `json:"id"`
			InvitationStatus string `json:"invitation_status"`
		} `json:"data"`
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/platform/staff", map[string]any{"email": inviteEmail, "name": "QA Invited Staff", "role_code": "SUPPORT", "tenant_ids": []string{demoTenantID}}, 201, &invited)
	if invited.Data.ID == "" || invited.Data.InvitationStatus != "failed" {
		t.Fatal("platform staff invitation result invalid")
	}
}
