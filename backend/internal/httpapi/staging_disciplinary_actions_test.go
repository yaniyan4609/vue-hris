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

func TestStagingDisciplinaryActions(t *testing.T) {
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
	newClient := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, Timeout: 10 * time.Second}
	}
	admin, ess := newClient(), newClient()
	var adminState, essState auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &adminState)
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &essState)
	tenantID := adminState.SelectedTenantID
	var employeeID string
	if err = db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? AND user_id=? AND deleted_at IS NULL`, tenantID, essState.User.ID).Scan(&employeeID); err != nil {
		t.Fatal(err)
	}
	idsMade := []string{}
	t.Cleanup(func() {
		for _, id := range idsMade {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='disciplinary_actions' AND entity_id=?`, tenantID, id); e != nil {
				t.Error(e)
			}
			if _, e := db.Exec(`DELETE FROM disciplinary_actions WHERE tenant_id=? AND id=?`, tenantID, id); e != nil {
				t.Error(e)
			}
		}
	})
	today := tenantToday("Asia/Jakarta")
	parsed, _ := time.Parse("2006-01-02", today)
	yesterday := parsed.AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := parsed.AddDate(0, 0, 1).Format("2006-01-02")
	create := func(severity, issued string, until *string) string {
		t.Helper()
		var made struct {
			Data struct {
				ID      string `json:"id"`
				Version uint64 `json:"version"`
			} `json:"data"`
		}
		doJSON(t, admin, "POST", server.URL+"/api/v1/admin/disciplinary-actions", map[string]any{"employee_id": employeeID, "severity": severity, "issued_date": issued, "valid_until": until, "description": "Catatan pelanggaran QA dan tindak lanjut"}, 201, &made)
		if made.Data.ID == "" || made.Data.Version != 1 {
			t.Fatalf("create %s: %#v", severity, made)
		}
		idsMade = append(idsMade, made.Data.ID)
		return made.Data.ID
	}
	sp2 := map[string]any{"employee_id": employeeID, "severity": "SP2", "issued_date": today, "valid_until": tomorrow, "description": "Peringatan kedua QA tanpa SP pertama"}
	doJSON(t, ess, "GET", server.URL+"/api/v1/admin/disciplinary-actions", nil, 403, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/disciplinary-actions", sp2, 409, nil)
	badFile, _ := ids.New()
	bad := map[string]any{"employee_id": employeeID, "severity": "SP1", "issued_date": yesterday, "valid_until": tomorrow, "description": "Peringatan pertama QA dengan file", "file_id": badFile}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/disciplinary-actions", bad, 404, nil)
	sp1ID := create("SP1", yesterday, &tomorrow)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/disciplinary-actions", map[string]any{"employee_id": employeeID, "severity": "SP1", "issued_date": today, "valid_until": tomorrow, "description": "Duplikat SP pertama QA"}, 409, nil)
	sp2ID := create("SP2", today, &tomorrow)
	sp3ID := create("SP3", today, &tomorrow)
	counselID := create("konseling", today, nil)
	var listed struct {
		Data []disciplinaryAction `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/disciplinary-actions?employee_id="+employeeID+"&severity=SP1&status=active", nil, 200, &listed)
	found := false
	for _, item := range listed.Data {
		if item.ID == sp1ID {
			found = true
		}
	}
	if !found {
		t.Fatal("SP1 missing from filtered list")
	}
	var detail struct {
		Data disciplinaryAction `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/disciplinary-actions/"+sp1ID, nil, 200, &detail)
	if detail.Data.Severity != "SP1" || len(detail.Data.History) != 1 {
		t.Fatalf("detail: %#v", detail.Data)
	}
	patch := map[string]any{"issued_date": yesterday, "valid_until": tomorrow, "description": "Uraian SP pertama QA setelah koreksi", "version": 1}
	doJSON(t, admin, "PATCH", server.URL+"/api/v1/admin/disciplinary-actions/"+sp1ID, patch, 200, nil)
	doJSON(t, admin, "PATCH", server.URL+"/api/v1/admin/disciplinary-actions/"+sp1ID, patch, 409, nil)
	patch["version"] = 2
	patch["valid_until"] = yesterday
	doJSON(t, admin, "PATCH", server.URL+"/api/v1/admin/disciplinary-actions/"+sp1ID, patch, 409, nil)
	doJSON(t, admin, "DELETE", server.URL+"/api/v1/admin/disciplinary-actions/"+sp1ID, nil, 403, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "password"}, 200, nil)
	doJSON(t, admin, "DELETE", server.URL+"/api/v1/admin/disciplinary-actions/"+sp1ID, map[string]any{"version": 2, "reason": "Pembatalan SP pertama QA"}, 409, nil)
	for _, id := range []string{sp3ID, sp2ID, sp1ID, counselID} {
		doJSON(t, admin, "DELETE", server.URL+"/api/v1/admin/disciplinary-actions/"+id, map[string]any{"reason": "Pembatalan tindakan QA"}, 204, nil)
	}
	doJSON(t, admin, "DELETE", server.URL+"/api/v1/admin/disciplinary-actions/"+sp1ID, nil, 409, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/disciplinary-actions/"+sp1ID, nil, 200, &detail)
	if detail.Data.Status != "cancelled" || len(detail.Data.History) != 3 {
		t.Fatalf("cancel history: %#v", detail.Data)
	}
	var auditRaw sql.NullString
	if err = db.QueryRow(`SELECT CAST(GROUP_CONCAT(COALESCE(before_json,'') SEPARATOR ' ') AS CHAR) FROM audit_logs WHERE tenant_id=? AND entity_type='disciplinary_actions' AND entity_id=?`, tenantID, sp1ID).Scan(&auditRaw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(auditRaw.String, "Uraian SP pertama QA") || strings.Contains(auditRaw.String, "Catatan pelanggaran QA") {
		t.Fatal("sensitive description leaked to audit")
	}
}
