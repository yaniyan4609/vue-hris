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

func TestStagingRoleDelegationAuditAndTenantBoundary(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("integration test requires the named staging database")
	}
	t.Setenv("SMTP_HOST", "")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	newID := func() string {
		id, err := ids.New()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	actorID, targetID, foreignID, foreignTenantID, qaRoleID := newID(), newID(), newID(), newID(), newID()
	inviteEmail := strings.ToLower("qa-invite-" + actorID + "@example.invalid")
	var tenantID, adminRoleID string
	if err := db.QueryRow(`SELECT t.id,r.id FROM tenants t JOIN roles r ON r.tenant_id=t.id AND r.code='ADMIN_TENANT' WHERE t.slug='client-pertama'`).Scan(&tenantID, &adminRoleID); err != nil {
		t.Fatal(err)
	}
	var createdRoleID string
	t.Cleanup(func() {
		var inviteID string
		if err := db.QueryRow(`SELECT id FROM users WHERE email_normalized=?`, inviteEmail).Scan(&inviteID); err == nil {
			for _, query := range []string{`DELETE FROM password_reset_tokens WHERE user_id=?`, `DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, `DELETE FROM users WHERE id=?`} {
				if strings.Contains(query, "tenant_memberships") {
					_, err = db.Exec(query, tenantID, inviteID)
				} else {
					_, err = db.Exec(query, inviteID)
				}
				if err != nil {
					t.Errorf("cleanup invited user: %v", err)
				}
			}
		}
		if createdRoleID != "" {
			for _, query := range []string{`DELETE v FROM role_scope_values v JOIN role_scope_rules rule ON rule.tenant_id=v.tenant_id AND rule.id=v.rule_id WHERE rule.tenant_id=? AND rule.role_id=?`, `DELETE FROM role_scope_rules WHERE tenant_id=? AND role_id=?`, `DELETE FROM role_permissions WHERE tenant_id=? AND role_id=?`, `DELETE FROM roles WHERE tenant_id=? AND id=?`} {
				if _, err := db.Exec(query, tenantID, createdRoleID); err != nil {
					t.Errorf("cleanup custom role: %v", err)
				}
			}
		}
		steps := []struct {
			query string
			args  []any
		}{
			{`DELETE FROM audit_logs WHERE tenant_id=? AND actor_user_id=?`, []any{tenantID, actorID}},
			{`DELETE v FROM user_scope_values v JOIN user_scope_rules sr ON sr.tenant_id=v.tenant_id AND sr.id=v.rule_id WHERE sr.tenant_id=? AND sr.user_id=?`, []any{tenantID, targetID}},
			{`DELETE FROM user_scope_rules WHERE tenant_id=? AND user_id=?`, []any{tenantID, targetID}},
			{`DELETE FROM user_scope_overrides WHERE tenant_id=? AND user_id=?`, []any{tenantID, targetID}},
			{`DELETE FROM user_permission_overrides WHERE tenant_id=? AND user_id=?`, []any{tenantID, targetID}},
			{`DELETE FROM login_events WHERE user_id IN (?,?,?)`, []any{actorID, targetID, foreignID}},
			{`DELETE FROM sessions WHERE user_id IN (?,?,?)`, []any{actorID, targetID, foreignID}},
			{`DELETE FROM user_roles WHERE tenant_id=? AND user_id IN (?,?)`, []any{tenantID, actorID, targetID}},
			{`DELETE FROM role_permissions WHERE tenant_id=? AND role_id=?`, []any{tenantID, qaRoleID}},
			{`DELETE FROM roles WHERE tenant_id=? AND id=?`, []any{tenantID, qaRoleID}},
			{`DELETE FROM permission_registry WHERE code='qa_access.private'`, nil},
			{`DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id IN (?,?,?)`, []any{tenantID, actorID, targetID, foreignID}},
			{`DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id IN (?,?)`, []any{foreignTenantID, foreignID, targetID}},
			{`DELETE FROM users WHERE id IN (?,?,?)`, []any{actorID, targetID, foreignID}},
			{`DELETE FROM tenants WHERE id=?`, []any{foreignTenantID}},
		}
		for _, step := range steps {
			if _, err := db.Exec(step.query, step.args...); err != nil {
				t.Errorf("cleanup access test: %v", err)
			}
		}
	})
	password := "Qa-Access-Password-29"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, name string }{{actorID, "QA Access Admin"}, {targetID, "QA Access Target"}, {foreignID, "QA Foreign Target"}} {
		email := strings.ToLower("qa-access-" + item.id + "@example.invalid")
		if _, err := db.Exec(`INSERT INTO users (id,email,email_normalized,display_name,password_hash) VALUES (?,?,?,?,?)`, item.id, email, email, item.name, hash); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO tenants (id,slug,name,status) VALUES (?,?,?,'active')`, foreignTenantID, strings.ToLower("qa-"+foreignTenantID), "QA Foreign Tenant"); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ tenant, user string }{{tenantID, actorID}, {tenantID, targetID}, {foreignTenantID, foreignID}} {
		if _, err := db.Exec(`INSERT INTO tenant_memberships (tenant_id,user_id,status) VALUES (?,?,'active')`, item.tenant, item.user); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO user_roles (tenant_id,id,user_id,role_id,valid_from) VALUES (?,?,?,?,CURRENT_DATE())`, tenantID, newID(), actorID, adminRoleID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO permission_registry (code,module,action,requires_scope) VALUES ('qa_access.private','qa_access','private',FALSE)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO roles (tenant_id,id,code,name) VALUES (?,?,?,?)`, tenantID, qaRoleID, "QA_PRIVATE", "QA Private"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO role_permissions (tenant_id,role_id,permission_code) VALUES (?,?,'qa_access.private')`, tenantID, qaRoleID); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(Router(db, Options{CookieSecure: false}))
	defer server.Close()
	newClient := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, Timeout: 10 * time.Second}
	}
	actor, target := newClient(), newClient()
	actorEmail := strings.ToLower("qa-access-" + actorID + "@example.invalid")
	targetEmail := strings.ToLower("qa-access-" + targetID + "@example.invalid")
	var actorState, targetState auth.State
	doJSON(t, actor, "POST", server.URL+"/api/v1/auth/login", map[string]any{"email": actorEmail, "password": password}, 200, &actorState)
	doJSON(t, target, "POST", server.URL+"/api/v1/auth/login", map[string]any{"email": targetEmail, "password": password}, 200, &targetState)
	if !actorState.Tenants[0].Admin || targetState.Tenants[0].Admin {
		t.Fatal("incorrect initial Admin access")
	}
	var members pageResult[accessUser]
	doJSON(t, actor, "GET", server.URL+"/api/v1/admin/access/users", nil, 200, &members)
	for _, item := range members.Items {
		if item.ID == foreignID {
			t.Fatal("foreign tenant user leaked")
		}
	}
	base := server.URL + "/api/v1/admin/access/users/"
	grant := map[string]string{"roleId": adminRoleID, "currentPassword": password}
	doJSON(t, actor, "POST", base+actorID+"/roles", grant, 403, nil)
	doJSON(t, actor, "POST", base+foreignID+"/roles", grant, 404, nil)
	doJSON(t, actor, "POST", base+targetID+"/roles", map[string]string{"roleId": qaRoleID, "currentPassword": password}, 403, nil)
	doJSON(t, actor, "POST", base+targetID+"/roles", map[string]string{"roleId": adminRoleID, "currentPassword": "wrong-password"}, 403, nil)
	var assigned struct {
		AssignmentID string `json:"assignmentId"`
	}
	doJSON(t, actor, "POST", base+targetID+"/roles", grant, 201, &assigned)
	if assigned.AssignmentID == "" {
		t.Fatal("missing assignment ID")
	}
	doJSON(t, actor, "POST", base+targetID+"/roles", grant, 409, nil)
	doJSON(t, target, "GET", server.URL+"/api/v1/auth/me", nil, 200, &targetState)
	if !targetState.Tenants[0].Admin {
		t.Fatal("new Admin role was not effective on next request")
	}
	var detail accessUserDetail
	doJSON(t, actor, "GET", base+targetID, nil, 200, &detail)
	if detail.User.AccessVersion != 2 || len(detail.Roles) != 1 {
		t.Fatalf("role detail after grant: %#v", detail)
	}
	permissionURL := base + targetID + "/permissions/panel.admin"
	doJSON(t, actor, "PUT", base+actorID+"/permissions/panel.admin", map[string]string{"decision": "deny", "currentPassword": password}, 403, nil)
	doJSON(t, actor, "PUT", base+foreignID+"/permissions/panel.admin", map[string]string{"decision": "deny", "currentPassword": password}, 404, nil)
	doJSON(t, actor, "PUT", base+targetID+"/permissions/karyawan.lihat", map[string]string{"decision": "allow", "currentPassword": password}, 400, nil)
	doJSON(t, actor, "PUT", permissionURL, map[string]string{"decision": "deny", "currentPassword": password}, 204, nil)
	doJSON(t, target, "GET", server.URL+"/api/v1/auth/me", nil, 200, &targetState)
	if targetState.Tenants[0].Admin {
		t.Fatal("user deny did not close Admin Panel")
	}
	doJSON(t, actor, "PUT", permissionURL, map[string]string{"decision": "inherit", "currentPassword": password}, 204, nil)
	doJSON(t, target, "GET", server.URL+"/api/v1/auth/me", nil, 200, &targetState)
	if !targetState.Tenants[0].Admin {
		t.Fatal("inherited role did not restore Admin Panel")
	}
	doJSON(t, actor, "DELETE", base+targetID+"/roles/"+assigned.AssignmentID, map[string]string{"currentPassword": password}, 204, nil)
	doJSON(t, target, "GET", server.URL+"/api/v1/auth/me", nil, 200, &targetState)
	if targetState.Tenants[0].Admin {
		t.Fatal("revoked Admin role remained effective")
	}
	doJSON(t, target, "GET", server.URL+"/api/v1/admin/access/users", nil, 403, nil)
	doJSON(t, actor, "GET", base+targetID, nil, 200, &detail)
	if detail.User.AccessVersion != 5 || detail.Roles[0].RevokedAt == nil {
		t.Fatalf("role detail after revoke: %#v", detail)
	}
	var auditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE tenant_id=? AND actor_user_id=? AND entity_id=?`, tenantID, actorID, assigned.AssignmentID).Scan(&auditCount); err != nil || auditCount != 2 {
		t.Fatalf("role audit count=%d err=%v", auditCount, err)
	}
	var deniedCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE tenant_id=? AND actor_user_id=? AND action='pengguna.akses_ditolak'`, tenantID, actorID).Scan(&deniedCount); err != nil || deniedCount != 3 {
		t.Fatalf("denied access audit=%d err=%v", deniedCount, err)
	}
	var passwordLeak int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE tenant_id=? AND actor_user_id=? AND (LOCATE(?,COALESCE(before_json,''))>0 OR LOCATE(?,COALESCE(after_json,''))>0)`, tenantID, actorID, password, password).Scan(&passwordLeak); err != nil || passwordLeak != 0 {
		t.Fatalf("password in audit=%d err=%v", passwordLeak, err)
	}
	scopeURL := base + targetID + "/scopes/karyawan.lihat"
	doJSON(t, actor, "PUT", base+actorID+"/scopes/karyawan.lihat", map[string]any{"rules": []map[string]bool{{"allTenant": true}}, "currentPassword": password}, 403, nil)
	doJSON(t, actor, "PUT", scopeURL, map[string]any{"rules": []map[string]bool{{"allTenant": true}}, "currentPassword": password}, 204, nil)
	doJSON(t, actor, "PUT", base+targetID+"/permissions/karyawan.lihat", map[string]string{"decision": "allow", "currentPassword": password}, 204, nil)
	var targetAccess accessState
	doJSON(t, target, "GET", server.URL+"/api/v1/me/access", nil, 200, &targetAccess)
	if !targetAccess.Permissions["karyawan.lihat"].Allowed || len(targetAccess.Permissions["karyawan.lihat"].Scopes) != 1 {
		t.Fatal("scope override and allow did not take effect")
	}
	doJSON(t, actor, "DELETE", scopeURL, map[string]string{"currentPassword": password}, 204, nil)
	doJSON(t, target, "GET", server.URL+"/api/v1/me/access", nil, 200, &targetAccess)
	if targetAccess.Permissions["karyawan.lihat"].Allowed {
		t.Fatal("cleared scope left scoped permission open")
	}
	doJSON(t, actor, "PUT", base+targetID+"/permissions/karyawan.lihat", map[string]string{"decision": "inherit", "currentPassword": password}, 204, nil)
	canonicalRole := server.URL + "/api/v1/admin/users/" + targetID + "/roles"
	var reauthError struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	doJSON(t, actor, "POST", canonicalRole, map[string]string{"role_id": adminRoleID}, 403, &reauthError)
	if reauthError.Error.Code != "REAUTH_REQUIRED" {
		t.Fatalf("reauth code: %s", reauthError.Error.Code)
	}
	doJSON(t, actor, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": password}, 200, nil)
	doJSON(t, actor, "PUT", server.URL+"/api/v1/admin/roles/"+qaRoleID+"/scopes", map[string]any{"version": 1, "scopes": map[string]any{}}, 403, nil)
	var canonical struct {
		Data struct {
			AssignmentID string `json:"assignment_id"`
		} `json:"data"`
	}
	doJSON(t, actor, "POST", canonicalRole, map[string]string{"role_id": adminRoleID}, 201, &canonical)
	if canonical.Data.AssignmentID == "" {
		t.Fatal("canonical role assignment missing")
	}
	doJSON(t, actor, "PATCH", canonicalRole+"/"+canonical.Data.AssignmentID, map[string]string{"valid_from": time.Now().AddDate(0, 0, -1).Format("2006-01-02"), "valid_until": time.Now().AddDate(0, 0, 7).Format("2006-01-02")}, 200, nil)
	doJSON(t, actor, "DELETE", canonicalRole+"/"+canonical.Data.AssignmentID, nil, 204, nil)
	doJSON(t, target, "GET", server.URL+"/api/v1/auth/me", nil, 200, &targetState)
	if targetState.Tenants[0].Admin {
		t.Fatal("canonical revoke did not apply")
	}
	canonicalScope := server.URL + "/api/v1/admin/users/" + targetID + "/scopes/karyawan.lihat"
	doJSON(t, actor, "PUT", canonicalScope, map[string]any{"rules": []map[string]bool{{"all_tenant": true}}}, 204, nil)
	var canonicalScopes struct {
		Data map[string]any `json:"data"`
	}
	doJSON(t, actor, "GET", server.URL+"/api/v1/admin/users/"+targetID+"/scopes", nil, 200, &canonicalScopes)
	if canonicalScopes.Data["karyawan.lihat"] == nil {
		t.Fatalf("canonical scope missing: %+v", canonicalScopes.Data)
	}
	doJSON(t, actor, "DELETE", canonicalScope, nil, 204, nil)
	bulkURL := server.URL + "/api/v1/admin/users/" + targetID + "/permission-overrides"
	doJSON(t, actor, "PUT", bulkURL, map[string]any{"overrides": []map[string]string{{"permission_code": "panel.admin", "decision": "deny"}, {"permission_code": "qa_access.missing", "decision": "allow"}}}, 404, nil)
	var partial int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_permission_overrides WHERE tenant_id=? AND user_id=? AND permission_code='panel.admin'`, tenantID, targetID).Scan(&partial); err != nil || partial != 0 {
		t.Fatal("bulk override wrote partial data")
	}
	doJSON(t, actor, "PUT", bulkURL, map[string]any{"overrides": []map[string]string{{"permission_code": "panel.admin", "decision": "deny"}}}, 204, nil)
	doJSON(t, actor, "PUT", bulkURL, map[string]any{"overrides": []map[string]string{{"permission_code": "panel.admin", "decision": "inherit"}}}, 204, nil)
	var createdRole struct {
		Data roleRecord `json:"data"`
	}
	rolesURL := server.URL + "/api/v1/admin/roles"
	doJSON(t, actor, "POST", rolesURL, map[string]string{"code": "QA_NEW_" + actorID[:6], "name": "QA New Role"}, 201, &createdRole)
	createdRoleID = createdRole.Data.ID
	if createdRole.Data.ID == "" || createdRole.Data.Version != 1 {
		t.Fatal("custom role creation failed")
	}
	roleURL := rolesURL + "/" + createdRole.Data.ID
	var roleResult struct {
		Data struct {
			Version uint32 `json:"version"`
		} `json:"data"`
	}
	doJSON(t, actor, "PUT", roleURL+"/permissions", map[string]any{"version": 1, "permissions": []string{"panel.admin", "karyawan.lihat"}}, 200, &roleResult)
	if roleResult.Data.Version != 2 {
		t.Fatal("role permission version did not advance")
	}
	doJSON(t, actor, "PUT", roleURL+"/scopes", map[string]any{"version": 2, "scopes": map[string]any{"karyawan.lihat": []map[string]bool{{"all_tenant": true}}}}, 200, &roleResult)
	if roleResult.Data.Version != 3 {
		t.Fatal("role scope version did not advance")
	}
	doJSON(t, actor, "PATCH", roleURL, map[string]any{"version": 2, "name": "Stale", "description": ""}, 409, nil)
	doJSON(t, actor, "PATCH", roleURL, map[string]any{"version": 3, "name": "QA Updated Role", "description": "temporary"}, 200, &roleResult)
	if roleResult.Data.Version != 4 {
		t.Fatal("role update version did not advance")
	}
	var roleDetail struct {
		Data struct {
			Version uint32         `json:"version"`
			Scopes  map[string]any `json:"scopes"`
		} `json:"data"`
	}
	doJSON(t, actor, "GET", roleURL, nil, 200, &roleDetail)
	if roleDetail.Data.Version != 4 || roleDetail.Data.Scopes["karyawan.lihat"] == nil {
		t.Fatal("role detail differs")
	}
	doJSON(t, actor, "DELETE", roleURL, nil, 204, nil)
	userURL := server.URL + "/api/v1/admin/users/" + targetID
	var targetSessions struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, actor, "GET", userURL+"/sessions", nil, 200, &targetSessions)
	if len(targetSessions.Data) == 0 {
		t.Fatal("target session missing")
	}
	doJSON(t, actor, "DELETE", userURL+"/sessions/"+targetSessions.Data[0]["id"].(string), nil, 204, nil)
	doJSON(t, target, "GET", server.URL+"/api/v1/auth/me", nil, 401, nil)
	doJSON(t, target, "POST", server.URL+"/api/v1/auth/login", map[string]any{"email": targetEmail, "password": password}, 200, &targetState)
	doJSON(t, actor, "POST", userURL+"/disable", map[string]any{}, 204, nil)
	doJSON(t, target, "GET", server.URL+"/api/v1/auth/me", nil, 401, nil)
	var disabledPage struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, actor, "GET", server.URL+"/api/v1/admin/users?status=disabled&q="+targetEmail, nil, 200, &disabledPage)
	foundDisabled := false
	for _, item := range disabledPage.Data {
		foundDisabled = foundDisabled || item["id"] == targetID
	}
	if !foundDisabled {
		t.Fatal("disabled member missing from user list")
	}
	doJSON(t, actor, "POST", userURL+"/enable", map[string]any{}, 204, nil)
	doJSON(t, target, "POST", server.URL+"/api/v1/auth/login", map[string]any{"email": targetEmail, "password": password}, 200, &targetState)
	if targetState.SelectedTenantID != tenantID {
		t.Fatal("member did not regain tenant")
	}
	doJSON(t, actor, "PATCH", userURL, map[string]string{"name": "QA Changed Target", "phone": "+628123"}, 200, nil)
	doJSON(t, actor, "POST", userURL+"/reset-password", map[string]any{}, 503, nil)
	if _, err := db.Exec(`INSERT INTO tenant_memberships (tenant_id,user_id,status) VALUES (?,?,'active')`, foreignTenantID, targetID); err != nil {
		t.Fatal(err)
	}
	doJSON(t, actor, "PATCH", userURL, map[string]string{"name": "Should not spread"}, 409, nil)
	doJSON(t, actor, "POST", server.URL+"/api/v1/admin/users", map[string]string{"email": inviteEmail, "name": "QA Invited User"}, 503, nil)
	var absent int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE email_normalized=?`, inviteEmail).Scan(&absent); err != nil || absent != 0 {
		t.Fatal("user was created without SMTP configuration")
	}
	t.Setenv("SMTP_HOST", "127.0.0.1")
	t.Setenv("SMTP_PORT", "1")
	t.Setenv("SMTP_FROM", "noreply@example.invalid")
	t.Setenv("APP_ORIGIN", server.URL)
	var invitation struct {
		Data struct {
			ID               string `json:"id"`
			InvitationStatus string `json:"invitation_status"`
		} `json:"data"`
	}
	doJSON(t, actor, "POST", server.URL+"/api/v1/admin/users", map[string]string{"email": inviteEmail, "name": "QA Invited User"}, 201, &invitation)
	if invitation.Data.ID == "" || invitation.Data.InvitationStatus != "failed" {
		t.Fatal("failed SMTP delivery was not reported")
	}
	var existingInvite struct {
		Data struct {
			ID          string `json:"id"`
			NewIdentity bool   `json:"new_identity"`
		} `json:"data"`
	}
	doJSON(t, actor, "POST", server.URL+"/api/v1/admin/users", map[string]string{"email": strings.ToLower("qa-access-" + foreignID + "@example.invalid"), "name": "Foreign Name"}, 201, &existingInvite)
	if existingInvite.Data.ID != foreignID || existingInvite.Data.NewIdentity {
		t.Fatal("existing identity was duplicated")
	}
}
