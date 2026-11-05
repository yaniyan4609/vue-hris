package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// Run explicitly against staging with HRIS_INTEGRATION=1 and DB_* credentials.
// All temporary rows are removed before the test returns.
func TestStagingOrganizationEmployeeAuditAndTenantBoundary(t *testing.T) {
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
	defer db.Close()
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var state auth.State
	doJSON(t, client, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &state)
	tenantID := state.SelectedTenantID
	if tenantID == "" {
		t.Fatal("demo admin has no tenant")
	}
	var grants accessState
	doJSON(t, client, "GET", server.URL+"/api/v1/me/access", nil, 200, &grants)
	if !grants.Admin || grants.TenantID != tenantID || !grants.Permissions["panel.admin"].Allowed || !grants.Permissions["karyawan.lihat"].Allowed {
		t.Fatalf("effective access: %#v", grants)
	}
	qaID, err := ids.New()
	if err != nil {
		t.Fatal(err)
	}
	code := "QA_" + qaID
	var region orgNode
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/organization/regions", orgNode{Code: code, Name: "QA Temporary"}, 201, &region)
	defer func() {
		_, err := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='regions' AND entity_id=?`, tenantID, region.ID)
		if err != nil {
			t.Errorf("cleanup region audit: %v", err)
		}
		_, err = db.Exec(`DELETE FROM regions WHERE tenant_id=? AND id=?`, tenantID, region.ID)
		if err != nil {
			t.Errorf("cleanup region: %v", err)
		}
	}()
	var changedRegion orgNode
	doJSON(t, client, "PATCH", server.URL+"/api/v1/admin/organization/regions/"+region.ID, orgNode{Code: code, Name: "QA Updated"}, 200, &changedRegion)
	if changedRegion.Name != "QA Updated" {
		t.Fatalf("region update: %#v", changedRegion)
	}
	var auditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE tenant_id=? AND entity_type='regions' AND entity_id=?`, tenantID, region.ID).Scan(&auditCount); err != nil || auditCount != 2 {
		t.Fatalf("region audit=%d err=%v", auditCount, err)
	}

	var offices, positions pageResult[orgNode]
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/organization/offices", nil, 200, &offices)
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/organization/positions", nil, 200, &positions)
	if len(offices.Items) == 0 || len(positions.Items) == 0 {
		t.Fatal("demo office/position missing")
	}
	var createdEmployee employee
	doJSON(t, client, "POST", server.URL+"/api/v1/admin/employees", employee{EmployeeNumber: code, DisplayName: "QA Temporary", Status: "active", BaseOfficeID: offices.Items[0].ID, PositionID: positions.Items[0].ID}, 201, &createdEmployee)
	defer func() {
		for _, query := range []string{
			`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employees' AND entity_id=?`,
			`DELETE FROM employee_assignment_history WHERE tenant_id=? AND employee_id=?`,
			`DELETE FROM employees WHERE tenant_id=? AND id=?`,
		} {
			if _, err := db.Exec(query, tenantID, createdEmployee.ID); err != nil {
				t.Errorf("cleanup employee: %v", err)
			}
		}
	}()
	var updated employee
	doJSON(t, client, "PATCH", server.URL+"/api/v1/admin/employees/"+createdEmployee.ID, employee{EmployeeNumber: code, DisplayName: "QA Updated", Status: "leave", BaseOfficeID: offices.Items[0].ID, PositionID: positions.Items[0].ID}, 200, &updated)
	if updated.DisplayName != "QA Updated" || updated.Status != "leave" {
		t.Fatalf("employee update: %#v", updated)
	}
	var searched pageResult[employee]
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/employees?q="+code+"&pageSize=1", nil, 200, &searched)
	if searched.Total != 1 || len(searched.Items) != 1 || searched.Items[0].ID != createdEmployee.ID || searched.PageSize != 1 {
		t.Fatalf("scoped search/pagination: %#v", searched)
	}
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/employees?pageSize=101", nil, 400, nil)
	// A direct terminal status would leave Admin roles and sessions alive;
	// the dated termination workflow must perform that transition.
	doJSON(t, client, "PATCH", server.URL+"/api/v1/admin/employees/"+createdEmployee.ID, employee{EmployeeNumber: code, DisplayName: "QA Updated", Status: "terminated", BaseOfficeID: offices.Items[0].ID, PositionID: positions.Items[0].ID}, 400, nil)
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE tenant_id=? AND entity_type='employees' AND entity_id=?`, tenantID, createdEmployee.ID).Scan(&auditCount); err != nil || auditCount != 2 {
		t.Fatalf("employee audit=%d err=%v", auditCount, err)
	}
	var assignmentCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM employee_assignment_history WHERE tenant_id=? AND employee_id=?`, tenantID, createdEmployee.ID).Scan(&assignmentCount); err != nil || assignmentCount != 1 {
		t.Fatalf("assignment history=%d err=%v", assignmentCount, err)
	}

	foreignTenantID, err := ids.New()
	if err != nil {
		t.Fatal(err)
	}
	foreignRegionID, err := ids.New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO tenants (id,slug,name,status) VALUES (?,?,?,'active')`, foreignTenantID, strings.ToLower(code), "QA Foreign Tenant"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(`DELETE FROM regions WHERE tenant_id=? AND id=?`, foreignTenantID, foreignRegionID); err != nil {
			t.Errorf("cleanup foreign region: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM tenants WHERE id=?`, foreignTenantID); err != nil {
			t.Errorf("cleanup foreign tenant: %v", err)
		}
	}()
	if _, err = db.Exec(`INSERT INTO regions (tenant_id,id,code,name) VALUES (?,?,?,?)`, foreignTenantID, foreignRegionID, "QA", "QA Foreign Region"); err != nil {
		t.Fatal(err)
	}
	var visible pageResult[orgNode]
	doJSON(t, client, "GET", server.URL+"/api/v1/admin/organization/regions", nil, 200, &visible)
	for _, item := range visible.Items {
		if item.ID == foreignRegionID {
			t.Fatal("foreign tenant region leaked")
		}
	}
}

func doJSON(t *testing.T, client *http.Client, method, url string, input any, want int, output any) {
	t.Helper()
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != want {
		raw, _ := io.ReadAll(response.Body)
		t.Fatal(fmt.Sprintf("%s %s: got %d, want %d: %s", method, url, response.StatusCode, want, raw))
	}
	if output != nil {
		if err := json.NewDecoder(response.Body).Decode(output); err != nil {
			t.Fatal(err)
		}
	}
}
