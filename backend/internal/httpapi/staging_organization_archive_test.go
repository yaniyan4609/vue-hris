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

func TestStagingOrganizationArchiveComplete(t *testing.T) {
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
	server := httptest.NewServer(Router(db, Options{CookieSecure: false, DemoLogin: true}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var state auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &state)
	tenantID := state.SelectedTenantID
	if tenantID == "" {
		t.Fatal("admin tenant not selected")
	}
	created := map[string]string{}
	codes := map[string]string{}
	t.Cleanup(func() {
		for _, kind := range []string{"positions", "departments", "divisions", "offices", "regions"} {
			id := created[kind]
			if id == "" {
				continue
			}
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type=? AND entity_id=?`, tenantID, kind, id); e != nil {
				t.Errorf("cleanup organization audit: %v", e)
			}
			if _, e := db.Exec("DELETE FROM "+kind+" WHERE tenant_id=? AND id=?", tenantID, id); e != nil {
				t.Errorf("cleanup organization fixture: %v", e)
			}
		}
	})
	base := server.URL + "/api/v1/admin/"
	create := func(kind, parent string) string {
		t.Helper()
		uid, _ := ids.New()
		code := "QA" + uid
		body := map[string]any{"code": code, "name": "QA Archive " + kind}
		if parent != "" {
			body["parent_id"] = parent
		}
		if kind == "offices" {
			body["timezone"] = "Asia/Jakarta"
		}
		var response struct {
			Data struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"data"`
		}
		doJSON(t, client, "POST", base+kind, body, 201, &response)
		if response.Data.ID == "" || response.Data.Status != "active" {
			t.Fatalf("invalid %s: %#v", kind, response.Data)
		}
		created[kind] = response.Data.ID
		codes[kind] = code
		return response.Data.ID
	}
	region := create("regions", "")
	office := create("offices", region)
	division := create("divisions", "")
	department := create("departments", division)
	position := create("positions", department)
	var filtered struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, client, "GET", base+"offices?filter%5Bregion_id%5D="+region, nil, 200, &filtered)
	if len(filtered.Data) != 1 || filtered.Data[0]["id"] != office {
		t.Fatalf("office parent filter failed: %#v", filtered.Data)
	}
	for kind, id := range map[string]string{"regions": region, "divisions": division, "departments": department} {
		doJSON(t, client, "DELETE", base+kind+"/"+id, nil, 409, nil)
	}
	employeeID := ""
	cleanupEmployee := func() {
		if employeeID == "" {
			return
		}
		for _, query := range []string{
			`DELETE FROM employee_assignments WHERE tenant_id=? AND employee_id=?`,
			`DELETE FROM employee_assignment_history WHERE tenant_id=? AND employee_id=?`,
			`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employees' AND entity_id=?`,
			`DELETE FROM employees WHERE tenant_id=? AND id=?`,
		} {
			if _, e := db.Exec(query, tenantID, employeeID); e != nil {
				t.Errorf("cleanup QA employee: %v", e)
			}
		}
		employeeID = ""
	}
	t.Cleanup(cleanupEmployee)
	uid, _ := ids.New()
	var employeeCreated employee
	doJSON(t, client, "POST", base+"employees", map[string]any{"employeeNumber": "QA-" + uid, "displayName": "QA Organization Employee", "status": "active", "baseOfficeId": office, "positionId": position}, 201, &employeeCreated)
	employeeID = employeeCreated.ID
	if employeeID == "" {
		t.Fatal("QA employee ID missing")
	}
	doJSON(t, client, "DELETE", base+"positions/"+position, nil, 409, nil)
	doJSON(t, client, "DELETE", base+"offices/"+office, nil, 409, nil)
	cleanupEmployee()
	essJar, _ := cookiejar.New(nil)
	ess := &http.Client{Jar: essJar, Timeout: 10 * time.Second}
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, nil)
	doJSON(t, ess, "DELETE", base+"positions/"+position, nil, 403, nil)
	for _, node := range []struct{ kind, id string }{{"positions", position}, {"departments", department}, {"divisions", division}, {"offices", office}, {"regions", region}} {
		doJSON(t, client, "DELETE", base+node.kind+"/"+node.id, nil, 204, nil)
		doJSON(t, client, "DELETE", base+node.kind+"/"+node.id, nil, 204, nil)
		var detail struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		doJSON(t, client, "GET", base+node.kind+"/"+node.id, nil, 200, &detail)
		if detail.Data.Status != "archived" {
			t.Fatalf("%s not archived", node.kind)
		}
	}
	var active struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, client, "GET", base+"regions?q="+codes["regions"], nil, 200, &active)
	if len(active.Data) != 0 {
		t.Fatalf("archived region in active list: %#v", active.Data)
	}
	var archived struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, client, "GET", base+"regions?status=archived&q="+codes["regions"], nil, 200, &archived)
	found := false
	for _, item := range archived.Data {
		if item["id"] == region {
			found = true
		}
	}
	if !found {
		t.Fatal("archived region missing from archived filter")
	}
	doJSON(t, client, "POST", base+"offices", map[string]any{"code": "QA_CHILD_" + office, "name": "Invalid child", "parent_id": region, "timezone": "Asia/Jakarta"}, 409, nil)
	doJSON(t, client, "PATCH", base+"regions/"+region, map[string]any{"code": "QA_OLD", "name": "Invalid edit"}, 409, nil)
	var lookup struct {
		Data []map[string]any `json:"data"`
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/lookups/regions?q="+codes["regions"], nil, 200, &lookup)
	if len(lookup.Data) != 0 {
		t.Fatalf("archived region in lookup: %#v", lookup.Data)
	}
	var locations struct {
		Data []orgTreeNode `json:"data"`
	}
	doJSON(t, client, "GET", base+"locations/tree", nil, 200, &locations)
	for _, node := range locations.Data {
		if node.ID == region {
			t.Fatal("archived region in locations tree")
		}
	}
	var organization struct {
		Data []orgTreeNode `json:"data"`
	}
	doJSON(t, client, "GET", base+"organization/tree", nil, 200, &organization)
	for _, node := range organization.Data {
		if node.ID == division {
			t.Fatal("archived division in organization tree")
		}
	}
	doJSON(t, client, "POST", base+"employees", map[string]any{"employeeNumber": "QA-ARCHIVED-" + region, "displayName": "Invalid Employee", "status": "active", "baseOfficeId": office, "positionId": position}, 400, nil)
	doJSON(t, client, "GET", base+"regions?status=bad", nil, 400, nil)
	doJSON(t, client, "DELETE", base+"regions/"+position, nil, 404, nil)
}
