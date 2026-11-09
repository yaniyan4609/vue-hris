package httpapi

import (
	"context"
	"database/sql"
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

func TestStagingEmployeeAssets(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	admin := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var state auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &state)
	tenantID := state.SelectedTenantID
	var seedEmployee string
	if err = db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? AND status='active' AND deleted_at IS NULL AND id IN (SELECT employee_id FROM employee_assignments WHERE tenant_id=? AND current_flag=1) LIMIT 1`, tenantID, tenantID).Scan(&seedEmployee); err != nil {
		t.Fatal(err)
	}
	qaEmployee, _ := ids.New()
	qaAssignment, _ := ids.New()
	assetID := ""
	termID := ""
	loanIDs := []string{}
	t.Cleanup(func() {
		if termID != "" {
			for _, q := range []string{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='terminations' AND entity_id=?`, `DELETE FROM termination_checklist WHERE tenant_id=? AND termination_id=?`, `DELETE FROM terminations WHERE tenant_id=? AND id=?`} {
				if _, e := db.Exec(q, tenantID, termID); e != nil {
					t.Error(e)
				}
			}
		}
		for _, id := range loanIDs {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='employee_assets' AND entity_id=?`, tenantID, id); e != nil {
				t.Error(e)
			}
			if _, e := db.Exec(`DELETE FROM employee_assets WHERE tenant_id=? AND id=?`, tenantID, id); e != nil {
				t.Error(e)
			}
		}
		if assetID != "" {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='assets' AND entity_id=?`, tenantID, assetID); e != nil {
				t.Error(e)
			}
			if _, e := db.Exec(`DELETE FROM assets WHERE tenant_id=? AND id=?`, tenantID, assetID); e != nil {
				t.Error(e)
			}
		}
		if _, e := db.Exec(`DELETE FROM employee_assignments WHERE tenant_id=? AND id=?`, tenantID, qaAssignment); e != nil {
			t.Error(e)
		}
		if _, e := db.Exec(`DELETE FROM employees WHERE tenant_id=? AND id=?`, tenantID, qaEmployee); e != nil {
			t.Error(e)
		}
	})
	if _, err = db.Exec(`INSERT INTO employees (tenant_id,id,employee_number,display_name,status,base_office_id,position_id) SELECT ?,?,?,'QA Asset Employee','active',base_office_id,position_id FROM employees WHERE tenant_id=? AND id=?`, tenantID, qaEmployee, "QA-"+qaEmployee, tenantID, seedEmployee); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,valid_from,current_flag,created_by) SELECT ?,?,?,region_id,office_id,division_id,department_id,position_id,reports_to_employee_id,grade,UTC_TIMESTAMP(6)-INTERVAL 7 DAY,1,? FROM employee_assignments WHERE tenant_id=? AND employee_id=? AND current_flag=1`, tenantID, qaAssignment, qaEmployee, state.User.ID, tenantID, seedEmployee); err != nil {
		t.Fatal(err)
	}
	root := server.URL + "/api/v1/admin"
	code := "QA-" + qaEmployee
	var made struct {
		Data struct {
			ID      string `json:"id"`
			Version uint64 `json:"version"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", root+"/assets", map[string]any{"code": code, "name": "Laptop QA Aset", "category": "Laptop"}, 201, &made)
	assetID = made.Data.ID
	doJSON(t, admin, "POST", root+"/assets", map[string]any{"code": code, "name": "Laptop duplikat QA"}, 409, nil)
	doJSON(t, admin, "GET", root+"/assets?q="+code, nil, 200, nil)
	patch := func(status string, version uint64) {
		t.Helper()
		doJSON(t, admin, "PATCH", root+"/assets/"+assetID, map[string]any{"code": code, "name": "Laptop QA Aset", "category": "Laptop", "status": status, "version": version}, 200, nil)
	}
	patch("maintenance", 1)
	issueBody := map[string]any{"employee_id": qaEmployee, "asset_id": assetID, "note": "Aset untuk kerja QA"}
	doJSON(t, admin, "POST", root+"/employee-assets", issueBody, 409, nil)
	patch("available", 2)
	var issue struct {
		Data struct {
			ID      string `json:"id"`
			Version uint64 `json:"version"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", root+"/employee-assets", issueBody, 201, &issue)
	loanIDs = append(loanIDs, issue.Data.ID)
	doJSON(t, admin, "POST", root+"/employee-assets", issueBody, 409, nil)
	doJSON(t, admin, "GET", root+"/employee-assets?employee_id="+qaEmployee+"&status=active", nil, 200, nil)
	var term struct {
		Data struct {
			ID      string `json:"id"`
			Version uint64 `json:"version"`
		} `json:"data"`
	}
	tomorrow := time.Now().In(time.FixedZone("WIB", 7*3600)).AddDate(0, 0, 1).Format("2006-01-02")
	doJSON(t, admin, "POST", root+"/terminations", map[string]any{"employee_id": qaEmployee, "termination_type": "resign", "effective_date": tomorrow, "reason": "Pemeriksaan checklist aset QA"}, 201, &term)
	termID = term.Data.ID
	checklist := map[string]any{"version": term.Data.Version, "items": []map[string]any{{"code": "assets_returned", "is_done": true}, {"code": "handover_complete", "is_done": true}}}
	doJSON(t, admin, "PUT", root+"/terminations/"+termID+"/checklist", checklist, 409, nil)
	doJSON(t, admin, "DELETE", root+"/assets/"+assetID, nil, 403, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "password"}, 200, nil)
	doJSON(t, admin, "DELETE", root+"/assets/"+assetID, nil, 409, nil)
	doJSON(t, admin, "POST", root+"/employee-assets/"+issue.Data.ID+"/return", map[string]any{"return_condition": "damaged", "note": "Layar rusak saat kembali", "version": issue.Data.Version}, 200, nil)
	doJSON(t, admin, "POST", root+"/employee-assets/"+issue.Data.ID+"/return", map[string]any{"return_condition": "good", "version": 2}, 409, nil)
	doJSON(t, admin, "PUT", root+"/terminations/"+termID+"/checklist", checklist, 200, nil)
	doJSON(t, admin, "POST", root+"/employee-assets", issueBody, 409, nil)
	checklist = map[string]any{"version": uint64(2), "items": []map[string]any{{"code": "assets_returned", "is_done": false}, {"code": "handover_complete", "is_done": true}}}
	doJSON(t, admin, "PUT", root+"/terminations/"+termID+"/checklist", checklist, 200, nil)
	patch("available", 5)
	doJSON(t, admin, "POST", root+"/employee-assets", issueBody, 201, &issue)
	loanIDs = append(loanIDs, issue.Data.ID)
	doJSON(t, admin, "POST", root+"/employee-assets/"+issue.Data.ID+"/return", map[string]any{"return_condition": "good", "version": 1}, 200, nil)
	var listed struct {
		Data []employeeAssetRecord `json:"data"`
	}
	doJSON(t, admin, "GET", root+"/employee-assets?employee_id="+qaEmployee+"&status=returned", nil, 200, &listed)
	if len(listed.Data) != 2 {
		t.Fatalf("history lost: %#v", listed.Data)
	}
	doJSON(t, admin, "DELETE", root+"/assets/"+assetID, nil, 204, nil)
	doJSON(t, admin, "PATCH", root+"/assets/"+assetID, map[string]any{"code": code, "name": "Archived QA", "version": 7}, 409, nil)
	var archived struct {
		Data []assetRecord `json:"data"`
	}
	doJSON(t, admin, "GET", root+"/assets?status=archived&q="+code, nil, 200, &archived)
	if len(archived.Data) != 1 || archived.Data[0].ID != assetID {
		t.Fatal("archived asset missing")
	}
	var raw sql.NullString
	if err = db.QueryRow(`SELECT CAST(GROUP_CONCAT(COALESCE(after_json,'') SEPARATOR ' ') AS CHAR) FROM audit_logs WHERE tenant_id=? AND entity_type='employee_assets' AND entity_id=?`, tenantID, loanIDs[0]).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw.String, "Layar rusak") || strings.Contains(raw.String, "Aset untuk kerja") {
		t.Fatal("sensitive note leaked to audit")
	}
}
