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

func TestStagingReimbursementCategories(t *testing.T) {
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
	server := httptest.NewServer(Router(db, Options{DemoLogin: true, CookieSecure: false}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	admin := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	var state auth.State
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "admin"}, 200, &state)
	id, code := "", ""
	claimIDs := []string{}
	fileID, receiptID := "", ""
	t.Cleanup(func() {
		if id == "" {
			return
		}
		if receiptID != "" {
			if _, e := db.Exec(`DELETE FROM reimbursement_receipts WHERE tenant_id=? AND id=?`, state.SelectedTenantID, receiptID); e != nil {
				t.Errorf("cleanup receipt: %v", e)
			}
		}
		for _, claimID := range claimIDs {
			if _, e := db.Exec(`DELETE FROM reimbursements WHERE tenant_id=? AND id=?`, state.SelectedTenantID, claimID); e != nil {
				t.Errorf("cleanup claim: %v", e)
			}
		}
		if fileID != "" {
			if _, e := db.Exec(`DELETE FROM files WHERE tenant_id=? AND id=?`, state.SelectedTenantID, fileID); e != nil {
				t.Errorf("cleanup file: %v", e)
			}
		}
		if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='reimbursement_categories' AND entity_id=?`, state.SelectedTenantID, id); e != nil {
			t.Errorf("cleanup category audit: %v", e)
		}
		if _, e := db.Exec(`DELETE FROM reimbursement_categories WHERE tenant_id=? AND id=?`, state.SelectedTenantID, id); e != nil {
			t.Errorf("cleanup category: %v", e)
		}
	})
	uid, _ := ids.New()
	code = "QA_" + uid
	var created struct {
		Data reimbursementCategory `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/reimbursement-categories", map[string]any{"code": code, "name": "QA Travel", "monthly_limit": 1250000.50}, 201, &created)
	id = created.Data.ID
	if id == "" || created.Data.MonthlyLimit == nil || *created.Data.MonthlyLimit != "1250000.50" {
		t.Fatalf("unexpected category: %#v", created.Data)
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/reimbursement-categories", map[string]any{"code": code, "name": "Duplicate"}, 409, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/reimbursement-categories", map[string]any{"code": code + "X", "name": "Bad limit", "monthly_limit": 0}, 400, nil)
	var listed struct {
		Data []reimbursementCategory `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/reimbursement-categories?q="+code, nil, 200, &listed)
	if len(listed.Data) != 1 || listed.Data[0].ID != id {
		t.Fatalf("unexpected list: %#v", listed.Data)
	}
	doJSON(t, admin, "PATCH", server.URL+"/api/v1/admin/reimbursement-categories/"+id, map[string]any{"monthly_limit": nil, "name": "QA Travel Updated"}, 200, &created)
	if created.Data.MonthlyLimit != nil || created.Data.Name != "QA Travel Updated" {
		t.Fatalf("unexpected update: %#v", created.Data)
	}
	doJSON(t, admin, "DELETE", server.URL+"/api/v1/admin/reimbursement-categories/"+id, nil, 204, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/reimbursement-categories?q="+code, nil, 200, &listed)
	if len(listed.Data) != 1 || listed.Data[0].IsActive {
		t.Fatalf("category history lost: %#v", listed.Data)
	}
	var lookup struct {
		Data []lookupItem `json:"data"`
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/lookups/reimbursement-categories?q="+code, nil, 200, &lookup)
	if len(lookup.Data) != 0 {
		t.Fatalf("inactive category in lookup: %#v", lookup.Data)
	}
	essJar, _ := cookiejar.New(nil)
	ess := &http.Client{Jar: essJar, Timeout: 10 * time.Second}
	var essState auth.State
	doJSON(t, ess, "POST", server.URL+"/api/v1/auth/demo-login", map[string]string{"account": "ess"}, 200, &essState)
	doJSON(t, ess, "GET", server.URL+"/api/v1/admin/reimbursement-categories", nil, 403, nil)
	var ownEmployee string
	if err := db.QueryRow(`SELECT id FROM employees WHERE tenant_id=? AND user_id=?`, state.SelectedTenantID, essState.User.ID).Scan(&ownEmployee); err != nil {
		t.Fatal(err)
	}
	claimID, _ := ids.New()
	if _, err := db.Exec(`INSERT INTO reimbursements (tenant_id,id,employee_id,category_id,trx_date,amount,description,status) VALUES (?,?,?,?,CURRENT_DATE,1000.00,'QA claim','draft')`, state.SelectedTenantID, claimID, ownEmployee, id); err != nil {
		t.Fatal(err)
	}
	claimIDs = append(claimIDs, claimID)
	fileID, _ = ids.New()
	if _, err := db.Exec(`INSERT INTO files (tenant_id,id,storage_key,original_name,mime_type,size_bytes,sha256,purpose,uploaded_by,scan_status) VALUES (?,?,?,?,?,100,UNHEX(SHA2('qa-receipt',256)),'reimbursement_receipt',?,'clean')`, state.SelectedTenantID, fileID, "qa/reimbursement/"+fileID, "receipt.pdf", "application/pdf", essState.User.ID); err != nil {
		t.Fatal(err)
	}
	receiptID, _ = ids.New()
	if _, err := db.Exec(`INSERT INTO reimbursement_receipts (tenant_id,id,reimbursement_id,file_id) VALUES (?,?,?,?)`, state.SelectedTenantID, receiptID, claimIDs[0], fileID); err != nil {
		t.Fatal(err)
	}
	var claims struct {
		Data []reimbursementView `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/reimbursements?category_id="+id, nil, 200, &claims)
	if len(claims.Data) != 1 || claims.Data[0].ID != claimIDs[0] {
		t.Fatalf("ESS claim scope: %#v", claims.Data)
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/reimbursements/00000000000000000000000000", nil, 404, nil)
	var detail struct {
		Data struct {
			Claim    reimbursementView          `json:"claim"`
			Receipts []reimbursementReceiptView `json:"receipts"`
		} `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/ess/reimbursements/"+claimIDs[0], nil, 200, &detail)
	if detail.Data.Claim.ID != claimIDs[0] || len(detail.Data.Receipts) != 1 || detail.Data.Receipts[0].OriginalName != "receipt.pdf" {
		t.Fatalf("ESS claim detail: %#v", detail.Data)
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/reimbursements?category_id="+id, nil, 200, &claims)
	if len(claims.Data) != 1 {
		t.Fatalf("admin claim scope: %#v", claims.Data)
	}
	doJSON(t, admin, "GET", server.URL+"/api/v1/admin/reimbursements/"+claimIDs[0], nil, 200, &detail)
}
