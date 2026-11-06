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
	"digaji.id/hris/internal/ids"
)

func TestStagingAuthContract(t *testing.T) {
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
	id, _ := ids.New()
	email := "qa-contract-" + id + "@example.invalid"
	password := "Qa-Contract-Password-29"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id,email,email_normalized,display_name,password_hash) VALUES (?,?,?,?,?)`, id, email, email, "QA Contract", hash); err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, query := range []string{`DELETE FROM user_preferences WHERE user_id=?`, `DELETE FROM login_events WHERE user_id=?`, `DELETE FROM sessions WHERE user_id=?`, `DELETE FROM users WHERE id=?`} {
			if _, err := db.ExecContext(context.Background(), query, id); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	}()
	server := httptest.NewServer(Router(db, Options{CookieSecure: false}))
	defer server.Close()
	newClient := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, Timeout: 10 * time.Second}
	}
	first, second := newClient(), newClient()
	for _, client := range []*http.Client{first, second} {
		doJSON(t, client, "POST", server.URL+"/api/v1/auth/login", map[string]any{"email": email, "password": password}, 200, nil)
	}
	var sessions struct {
		Data []struct {
			ID      string `json:"id"`
			Current bool   `json:"current"`
		} `json:"data"`
	}
	doJSON(t, first, "GET", server.URL+"/api/v1/me/sessions", nil, 200, &sessions)
	if len(sessions.Data) != 2 {
		t.Fatalf("sessions: %d", len(sessions.Data))
	}
	var otherID string
	for _, item := range sessions.Data {
		if !item.Current {
			otherID = item.ID
		}
	}
	if otherID == "" {
		t.Fatal("other device missing")
	}
	missingCSRF, _ := http.NewRequest("PATCH", server.URL+"/api/v1/me/preferences", nil)
	denied, err := first.Do(missingCSRF)
	if err != nil {
		t.Fatal(err)
	}
	denied.Body.Close()
	if denied.StatusCode != 403 {
		t.Fatalf("missing CSRF: %d", denied.StatusCode)
	}
	doJSON(t, first, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "wrong"}, 403, nil)
	var reauth struct {
		Data struct {
			ValidUntil time.Time `json:"valid_until"`
		} `json:"data"`
	}
	doJSON(t, first, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": password}, 200, &reauth)
	if reauth.Data.ValidUntil.Before(time.Now().UTC()) {
		t.Fatal("reauth proof expired immediately")
	}
	valid, err := (auth.Service{DB: db}).RecentlyReauthenticated(ctx, cookieFromJar(t, first, server.URL))
	if err != nil || !valid {
		t.Fatalf("reauth proof: %v %v", valid, err)
	}
	var pref struct {
		Data map[string]string `json:"data"`
	}
	doJSON(t, first, "PATCH", server.URL+"/api/v1/me/preferences", map[string]string{"theme_mode": "dark", "theme_color": "default"}, 200, &pref)
	if pref.Data["theme_mode"] != "dark" {
		t.Fatalf("preference: %+v", pref)
	}
	doJSON(t, first, "DELETE", server.URL+"/api/v1/me/sessions/"+otherID, nil, 204, nil)
	doJSON(t, second, "GET", server.URL+"/api/v1/auth/me", nil, 401, nil)
}

func cookieFromJar(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	request, _ := http.NewRequest("GET", base, nil)
	for _, cookie := range client.Jar.Cookies(request.URL) {
		if cookie.Name == sessionCookieName {
			return cookie.Value
		}
	}
	t.Fatal("session cookie missing")
	return ""
}

func TestStagingOrganizationContract(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := httptest.NewServer(Router(db, Options{CookieSecure: false, DemoLogin: true}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, nil)
	var oldPage struct {
		Items []orgNode `json:"items"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/organization/regions?pageSize=2", nil, 200, &oldPage)
	var newPage struct {
		Data []map[string]any `json:"data"`
		Meta map[string]int   `json:"meta"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/regions?per_page=2", nil, 200, &newPage)
	if len(oldPage.Items) != len(newPage.Data) || newPage.Meta["per_page"] != 2 {
		t.Fatalf("organization alias differs: old=%d new=%d meta=%+v", len(oldPage.Items), len(newPage.Data), newPage.Meta)
	}
	for i, item := range oldPage.Items {
		if item.ID != newPage.Data[i]["id"] {
			t.Fatalf("item %d mismatch", i)
		}
	}
	if len(newPage.Data) > 0 {
		var detail struct {
			Data map[string]any `json:"data"`
		}
		doJSON(t, client, "GET", server.URL+"/api/v1/admin/regions/"+oldPage.Items[0].ID, nil, 200, &detail)
		if detail.Data["id"] != oldPage.Items[0].ID {
			t.Fatal("organization detail mismatch")
		}
	}
	otherJar, _ := cookiejar.New(nil)
	other := &http.Client{Jar: otherJar, Timeout: 10 * time.Second}
	doJSON(t, other, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, nil)
	doJSON(t, other, "GET", server.URL+"/api/v1/admin/regions", nil, 403, nil)
}

func TestStagingPasswordResetContract(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	t.Setenv("SMTP_HOST", "")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, _ := ids.New()
	email := "qa-reset-" + id + "@example.invalid"
	oldPassword := "Qa-Reset-Password-29"
	hash, err := auth.HashPassword(oldPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id,email,email_normalized,display_name,password_hash) VALUES (?,?,?,'QA Reset',?)`, id, email, email, hash); err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, query := range []string{`DELETE FROM password_reset_tokens WHERE user_id=?`, `DELETE FROM login_events WHERE user_id=?`, `DELETE FROM sessions WHERE user_id=?`, `DELETE FROM users WHERE id=?`} {
			if _, err := db.ExecContext(context.Background(), query, id); err != nil {
				t.Errorf("cleanup reset: %v", err)
			}
		}
	}()
	server := httptest.NewServer(Router(db, Options{CookieSecure: false}))
	defer server.Close()
	client := &http.Client{Timeout: 10 * time.Second}
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/password/forgot", map[string]string{"email": email}, 503, nil)
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM password_reset_tokens WHERE user_id=?`, id).Scan(&count); err != nil || count != 0 {
		t.Fatal("forgot created token without mail configuration")
	}
	var token string
	service := auth.Service{DB: db}
	if err := service.IssuePasswordReset(ctx, email, func(to, value string) error {
		if to != email {
			t.Fatal("wrong recipient")
		}
		token = value
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("reset token missing")
	}
	if err := service.ResetPassword(ctx, token, "Qa-Reset-New-29"); err != nil {
		t.Fatal(err)
	}
	if err := service.ResetPassword(ctx, token, "Qa-Reset-Again-29"); err != auth.ErrResetTokenInvalid {
		t.Fatalf("single use token: %v", err)
	}
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/login", map[string]string{"email": email, "password": oldPassword}, 401, nil)
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/login", map[string]string{"email": email, "password": "Qa-Reset-New-29"}, 200, nil)
}

func TestStagingAccessReadContract(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
	var oldPage struct {
		Items []accessUser `json:"items"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/access/users?pageSize=2", nil, 200, &oldPage)
	var page struct {
		Data []map[string]any `json:"data"`
		Meta map[string]int   `json:"meta"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/users?per_page=2", nil, 200, &page)
	if len(oldPage.Items) != len(page.Data) || page.Meta["per_page"] != 2 {
		t.Fatal("canonical user page differs")
	}
	for index, item := range oldPage.Items {
		if page.Data[index]["id"] != item.ID {
			t.Fatal("canonical user differs")
		}
	}
	var registry struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/permissions", nil, 200, &registry)
	if len(registry.Data) == 0 {
		t.Fatal("permission registry empty")
	}
	var roles struct {
		Data []roleRecord `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/roles", nil, 200, &roles)
	if len(roles.Data) == 0 || roles.Data[0].Version == 0 {
		t.Fatal("canonical roles missing")
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/roles/"+roles.Data[0].ID, nil, 200, nil)
	var detail struct {
		Data map[string]any `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/users/"+state.User.ID, nil, 200, &detail)
	if detail.Data["user"].(map[string]any)["id"] != state.User.ID {
		t.Fatal("wrong user detail")
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/users/"+state.User.ID+"/effective-access", nil, 200, &detail)
	if detail.Data["panel.admin"] == nil {
		t.Fatal("effective access missing")
	}
}
