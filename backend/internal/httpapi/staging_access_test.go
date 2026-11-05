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
	var tenantID, adminRoleID string
	if err := db.QueryRow(`SELECT t.id,r.id FROM tenants t JOIN roles r ON r.tenant_id=t.id AND r.code='ADMIN_TENANT' WHERE t.slug='client-pertama'`).Scan(&tenantID, &adminRoleID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
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
			{`DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id IN (?,?)`, []any{tenantID, actorID, targetID}},
			{`DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, []any{foreignTenantID, foreignID}},
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
}
