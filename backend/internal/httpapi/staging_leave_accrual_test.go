package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
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

func TestAnnualGrantDays(t *testing.T) {
	for _, tc := range []struct {
		quota  int64
		joined string
		year   int
		want   int64
	}{
		{1200, "2025-06-15", 2026, 1200},
		{1200, "2026-07-15", 2026, 600},
		{1000, "2026-11-30", 2026, 167},
	} {
		joined, _ := time.Parse("2006-01-02", tc.joined)
		if got := annualGrantDays(tc.quota, joined, tc.year); got != tc.want {
			t.Fatalf("join=%s got=%d want=%d", tc.joined, got, tc.want)
		}
	}
}

func TestStagingLeaveAttachmentUpload(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	t.Setenv("FILE_STORAGE_DIR", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	var fileID string
	t.Cleanup(func() {
		if fileID != "" {
			_, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='files' AND entity_id=?`, state.SelectedTenantID, fileID)
			if e != nil {
				t.Error(e)
			}
			_, e = db.Exec(`DELETE FROM files WHERE tenant_id=? AND id=?`, state.SelectedTenantID, fileID)
			if e != nil {
				t.Error(e)
			}
		}
	})
	picture := image.NewRGBA(image.Rect(0, 0, 1, 1))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	var raw bytes.Buffer
	if err = png.Encode(&raw, picture); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw.Bytes())
	var upload struct {
		Data struct {
			FileID string `json:"file_id"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/files/uploads", map[string]any{"purpose": "leave", "original_name": "qa-leave.png", "mime_type": "image/png", "size_bytes": raw.Len(), "sha256": hex.EncodeToString(hash[:])}, 201, &upload)
	fileID = upload.Data.FileID
	req, _ := http.NewRequest("PUT", server.URL+"/api/v1/files/"+fileID+"/content", bytes.NewReader(raw.Bytes()))
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("X-CSRF-Token", csrfToken(cookieFromJar(t, admin, server.URL)))
	response, err := admin.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 204 {
		t.Fatalf("upload status=%d", response.StatusCode)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/files/"+fileID+"/complete", map[string]any{}, 200, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/files/"+fileID, nil, 200, nil)
	doJSON(t, admin, "DELETE", server.URL+"/api/v1/files/"+fileID, nil, 204, nil)
}

func TestStagingAnnualLeaveAccrual(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	tenant := state.SelectedTenantID
	var office, position, region, division, department string
	if err = db.QueryRowContext(ctx, `SELECT a.office_id,a.position_id,a.region_id,a.division_id,a.department_id FROM employee_assignments a WHERE a.tenant_id=? AND a.current_flag=TRUE LIMIT 1`, tenant).Scan(&office, &position, &region, &division, &department); err != nil {
		t.Fatal(err)
	}
	newID := func() string {
		id, e := ids.New()
		if e != nil {
			t.Fatal(e)
		}
		return id
	}
	typeID, policyID, fullID, partialID, laterID := "", "", newID(), newID(), newID()
	assignmentIDs := []string{newID(), newID(), newID()}
	year := time.Now().In(time.FixedZone("WIB", 7*3600)).Year()
	var runID string
	t.Cleanup(func() {
		cleanup := []struct {
			query string
			args  []any
		}{
			{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='leave_accrual_runs' AND entity_id=?`, []any{tenant, runID}},
			{`DELETE FROM leave_accrual_grants WHERE tenant_id=? AND leave_type_id=?`, []any{tenant, typeID}},
			{`DELETE FROM leave_balance_ledger WHERE tenant_id=? AND leave_type_id=?`, []any{tenant, typeID}},
			{`DELETE FROM leave_balances WHERE tenant_id=? AND leave_type_id=?`, []any{tenant, typeID}},
			{`DELETE FROM leave_accrual_runs WHERE tenant_id=? AND id=?`, []any{tenant, runID}},
			{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='leave_policies' AND entity_id=?`, []any{tenant, policyID}},
			{`DELETE FROM leave_policies WHERE tenant_id=? AND id=?`, []any{tenant, policyID}},
			{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='leave_types' AND entity_id=?`, []any{tenant, typeID}},
			{`DELETE FROM leave_types WHERE tenant_id=? AND id=?`, []any{tenant, typeID}},
			{`DELETE FROM employee_assignments WHERE tenant_id=? AND id IN (?,?,?)`, []any{tenant, assignmentIDs[0], assignmentIDs[1], assignmentIDs[2]}},
			{`DELETE FROM employees WHERE tenant_id=? AND id IN (?,?,?)`, []any{tenant, fullID, partialID, laterID}},
		}
		for _, item := range cleanup {
			if _, e := db.Exec(item.query, item.args...); e != nil {
				t.Errorf("cleanup annual: %v", e)
			}
		}
	})
	insertEmployee := func(id, assignmentID, joined string) {
		if _, err = db.ExecContext(ctx, `INSERT INTO employees (tenant_id,id,employee_number,display_name,base_office_id,position_id,join_date) VALUES (?,?,?,'QA Annual Leave',?,?,?)`, tenant, id, "QA-"+id, office, position, joined); err != nil {
			t.Fatal(err)
		}
		if _, err = db.ExecContext(ctx, `INSERT INTO employee_assignments (tenant_id,id,employee_id,region_id,office_id,division_id,department_id,position_id,valid_from,current_flag) VALUES (?,?,?,?,?,?,?,?,'2020-01-01',1)`, tenant, assignmentID, id, region, office, division, department, position); err != nil {
			t.Fatal(err)
		}
	}
	for i, item := range []struct{ id, joined string }{{fullID, fmt.Sprintf("%04d-01-01", year-1)}, {partialID, fmt.Sprintf("%04d-07-15", year)}} {
		insertEmployee(item.id, assignmentIDs[i], item.joined)
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-types", map[string]any{"code": "QA_" + newID(), "name": "QA Annual Leave", "is_paid": true}, 201, &created)
	typeID = created.Data.ID
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-policies", map[string]any{"leave_type_id": typeID, "accrual_type": "annual", "quota_days": 12, "valid_from": fmt.Sprintf("%04d-01-01", year)}, 201, &created)
	policyID = created.Data.ID
	var run struct {
		Data leaveAccrualRun `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-balances/accrual-run", map[string]int{"year": year}, 202, &run)
	runID = run.Data.ID
	if _, err = RunDueLeaveAccruals(ctx, db); err != nil {
		t.Fatal(err)
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/leave-balances/accrual-runs/"+runID, nil, 200, &run)
	if run.Data.Status != "completed" || run.Data.CreditedCount < 2 {
		t.Fatalf("run: %+v", run.Data)
	}
	for _, item := range []struct{ id, want string }{{fullID, "12.00"}, {partialID, "6.00"}} {
		var days string
		if err = db.QueryRowContext(ctx, `SELECT CAST(days AS CHAR) FROM leave_accrual_grants WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND period_year=?`, tenant, item.id, typeID, year).Scan(&days); err != nil {
			t.Fatal(err)
		}
		if days != item.want {
			t.Fatalf("employee %s days=%s want=%s", item.id, days, item.want)
		}
	}
	if _, err = RunDueLeaveAccruals(ctx, db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM leave_accrual_grants WHERE tenant_id=? AND leave_type_id=? AND employee_id IN (?,?)`, tenant, typeID, fullID, partialID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("grants after retry=%d", count)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-balances/accrual-run", map[string]int{"year": year}, 200, nil)
	insertEmployee(laterID, assignmentIDs[2], fmt.Sprintf("%04d-10-01", year))
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-balances/accrual-run", map[string]int{"year": year}, 202, nil)
	if _, err = RunDueLeaveAccruals(ctx, db); err != nil {
		t.Fatal(err)
	}
	var laterDays string
	if err = db.QueryRowContext(ctx, `SELECT CAST(days AS CHAR) FROM leave_accrual_grants WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND period_year=?`, tenant, laterID, typeID, year).Scan(&laterDays); err != nil {
		t.Fatal(err)
	}
	if laterDays != "3.00" {
		t.Fatalf("same-day new hire days=%s", laterDays)
	}
	previousGrant, previousLedger := newID(), newID()
	if _, err = db.ExecContext(ctx, `INSERT INTO leave_balance_ledger (tenant_id,id,employee_id,leave_type_id,entry_type,days,effective_date,ref_type,ref_id) VALUES (?,?,?,?,'accrual',3,?,'leave_accrual_grant',?)`, tenant, previousLedger, fullID, typeID, fmt.Sprintf("%04d-01-01", year-1), previousGrant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO leave_accrual_grants (tenant_id,id,employee_id,leave_type_id,period_year,policy_id,run_id,ledger_id,days) VALUES (?,?,?,?,?,?,?,?,3)`, tenant, previousGrant, fullID, typeID, year-1, policyID, runID, previousLedger); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO leave_balances (tenant_id,employee_id,leave_type_id,period_year,balance_days) VALUES (?,?,?,?,3)`, tenant, fullID, typeID, year-1); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = expireAnnualLeave(ctx, db, tenant, year); err != nil {
			t.Fatal(err)
		}
	}
	var oldBalance string
	if err = db.QueryRowContext(ctx, `SELECT CAST(balance_days AS CHAR) FROM leave_balances WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND period_year=?`, tenant, fullID, typeID, year-1).Scan(&oldBalance); err != nil {
		t.Fatal(err)
	}
	if oldBalance != "0.00" {
		t.Fatalf("previous year balance=%s", oldBalance)
	}
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM leave_balance_ledger WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND entry_type='expiry' AND ref_id=?`, tenant, fullID, typeID, previousGrant).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expiry ledger count=%d", count)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "password"}, 200, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/leave-balances/adjustments", map[string]any{"employee_id": fullID, "leave_type_id": typeID, "days": 1, "effective_date": fmt.Sprintf("%04d-12-31", year-1), "reason": "QA setelah hangus"}, 409, nil)
}
