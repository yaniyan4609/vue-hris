package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
)

func TestStagingReimbursementWorkflow(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	t.Setenv("FILE_STORAGE_DIR", t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	var existingDefault int
	if err := db.QueryRow(`SELECT COUNT(*) FROM approval_flows WHERE tenant_id=? AND request_type='reimbursement' AND is_default=TRUE`, tenantID).Scan(&existingDefault); err != nil {
		t.Fatal(err)
	}
	if existingDefault != 0 {
		t.Skip("staging already has a reimbursement default flow")
	}
	categoryID, flowID, stepID := "", "", ""
	claimID, requestID, fileID, extraFileID, orphanFileID := "", "", "", "", ""
	t.Cleanup(func() {
		if requestID == "" && categoryID != "" {
			_ = db.QueryRow(`SELECT id,request_id FROM reimbursements WHERE tenant_id=? AND category_id=? ORDER BY id DESC LIMIT 1`, tenantID, categoryID).Scan(&claimID, &requestID)
		}
		if requestID != "" {
			var instanceID string
			_ = db.QueryRow(`SELECT id FROM approval_instances WHERE tenant_id=? AND request_id=?`, tenantID, requestID).Scan(&instanceID)
			for _, entry := range []struct{ query, id string }{
				{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='requests' AND entity_id=?`, requestID},
				{`DELETE FROM notification_outbox WHERE tenant_id=? AND aggregate_type='request' AND aggregate_id=?`, requestID},
				{`DELETE FROM request_revisions WHERE tenant_id=? AND request_id=?`, requestID},
				{`DELETE FROM approval_actions WHERE tenant_id=? AND instance_id=?`, instanceID},
				{`DELETE FROM approval_tasks WHERE tenant_id=? AND instance_id=?`, instanceID},
				{`DELETE FROM approval_instances WHERE tenant_id=? AND id=?`, instanceID},
				{`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='reimbursements' AND entity_id=?`, claimID},
				{`DELETE FROM reimbursement_receipts WHERE tenant_id=? AND reimbursement_id=?`, claimID},
				{`DELETE FROM reimbursements WHERE tenant_id=? AND id=?`, claimID},
				{`DELETE FROM requests WHERE tenant_id=? AND id=?`, requestID},
			} {
				if _, e := db.Exec(entry.query, tenantID, entry.id); e != nil {
					t.Errorf("cleanup claim: %v", e)
				}
			}
		}
		if fileID != "" {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='files' AND entity_id=?`, tenantID, fileID); e != nil {
				t.Errorf("cleanup file audit: %v", e)
			}
			if _, e := db.Exec(`DELETE FROM files WHERE tenant_id=? AND id=?`, tenantID, fileID); e != nil {
				t.Errorf("cleanup file: %v", e)
			}
			_ = os.Remove(filepath.Join(receiptStorageRoot(), tenantID, fileID))
		}
		if extraFileID != "" {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='files' AND entity_id=?`, tenantID, extraFileID); e != nil {
				t.Errorf("cleanup extra file audit: %v", e)
			}
			if _, e := db.Exec(`DELETE FROM files WHERE tenant_id=? AND id=?`, tenantID, extraFileID); e != nil {
				t.Errorf("cleanup extra file: %v", e)
			}
			_ = os.Remove(filepath.Join(receiptStorageRoot(), tenantID, extraFileID))
		}
		if orphanFileID != "" {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='files' AND entity_id=?`, tenantID, orphanFileID); e != nil {
				t.Errorf("cleanup orphan audit: %v", e)
			}
			if _, e := db.Exec(`DELETE FROM files WHERE tenant_id=? AND id=?`, tenantID, orphanFileID); e != nil {
				t.Errorf("cleanup orphan: %v", e)
			}
		}
		if categoryID != "" {
			if _, e := db.Exec(`DELETE FROM audit_logs WHERE tenant_id=? AND entity_type='reimbursement_categories' AND entity_id=?`, tenantID, categoryID); e != nil {
				t.Errorf("cleanup category audit: %v", e)
			}
			if _, e := db.Exec(`DELETE FROM reimbursement_categories WHERE tenant_id=? AND id=?`, tenantID, categoryID); e != nil {
				t.Errorf("cleanup category: %v", e)
			}
		}
		if flowID != "" {
			if _, e := db.Exec(`DELETE FROM approval_flow_steps WHERE tenant_id=? AND flow_id=?`, tenantID, flowID); e != nil {
				t.Errorf("cleanup steps: %v", e)
			}
			if _, e := db.Exec(`DELETE FROM approval_flows WHERE tenant_id=? AND id=?`, tenantID, flowID); e != nil {
				t.Errorf("cleanup flow: %v", e)
			}
		}
	})
	flowID, _ = ids.New()
	stepID, _ = ids.New()
	if _, err = db.Exec(`INSERT INTO approval_flows (tenant_id,id,request_type,name,is_default,is_active,version,valid_from) VALUES (?,?,'reimbursement','QA Reimbursement',TRUE,TRUE,1,'2000-01-01')`, tenantID, flowID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO approval_flow_steps (tenant_id,id,flow_id,level_no,approver_kind,approver_ref_id,required_permission) VALUES (?,?,?,1,'user',?,'reimbursement.approve')`, tenantID, stepID, flowID, adminState.User.ID); err != nil {
		t.Fatal(err)
	}
	uid, _ := ids.New()
	var category struct {
		Data reimbursementCategory `json:"data"`
	}
	doJSON(t, admin, "POST", server.URL+"/api/v1/admin/reimbursement-categories", map[string]any{"code": "QA_" + uid, "name": "QA Expense", "monthly_limit": 5000}, 201, &category)
	categoryID = category.Data.ID
	var categories struct {
		Data []lookupItem `json:"data"`
	}
	doJSON(t, ess, "GET", server.URL+"/api/v1/lookups/reimbursement-categories?q="+uid, nil, 200, &categories)
	if len(categories.Data) != 1 || categories.Data[0].ID != categoryID {
		t.Fatalf("ESS category lookup: %#v", categories.Data)
	}
	picture := image.NewRGBA(image.Rect(0, 0, 1, 1))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	var raw bytes.Buffer
	if err = png.Encode(&raw, picture); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw.Bytes())
	var upload struct {
		Data struct {
			FileID    string `json:"file_id"`
			UploadURL string `json:"upload_url"`
		} `json:"data"`
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/files/uploads", map[string]any{"purpose": "receipt", "original_name": "qa.png", "mime_type": "image/png", "size_bytes": raw.Len(), "sha256": hex.EncodeToString(hash[:])}, 201, &upload)
	fileID = upload.Data.FileID
	doJSON(t, ess, "POST", server.URL+"/api/v1/files/"+fileID+"/complete", map[string]any{}, 409, nil)
	req, _ := http.NewRequest("PUT", server.URL+"/api/v1/files/"+fileID+"/content", bytes.NewReader(raw.Bytes()))
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("X-CSRF-Token", csrfToken(cookieFromJar(t, ess, server.URL)))
	response, err := ess.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 204 {
		t.Fatalf("upload status: %d", response.StatusCode)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/files/"+fileID+"/complete", map[string]any{}, 200, nil)
	doJSON(t, ess, "GET", server.URL+"/api/v1/files/"+fileID, nil, 200, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/files/"+fileID, nil, 404, nil)
	doJSON(t, ess, "GET", server.URL+"/api/v1/files/"+fileID+"/content", nil, 403, nil)
	download, err := ess.Get(server.URL + "/api/v1/files/" + fileID + "/download")
	if err != nil {
		t.Fatal(err)
	}
	downloaded, err := io.ReadAll(download.Body)
	download.Body.Close()
	if err != nil || download.StatusCode != 200 || !bytes.Equal(downloaded, raw.Bytes()) {
		t.Fatalf("ESS download: status=%d err=%v", download.StatusCode, err)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/files/uploads", map[string]any{"purpose": "receipt", "original_name": "unused.png", "mime_type": "image/png", "size_bytes": raw.Len(), "sha256": hex.EncodeToString(hash[:])}, 201, &upload)
	orphanFileID = upload.Data.FileID
	doJSON(t, ess, "DELETE", server.URL+"/api/v1/files/"+orphanFileID, nil, 204, nil)
	doJSON(t, ess, "GET", server.URL+"/api/v1/files/"+orphanFileID, nil, 404, nil)
	input := map[string]any{"category_id": categoryID, "trx_date": time.Now().Format("2006-01-02"), "amount": 1500, "description": "QA expense", "file_ids": []string{fileID}}
	input["amount"] = 6000
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/reimbursements", input, 409, nil)
	input["amount"] = 1500
	var submitted struct {
		Data struct {
			ID        string `json:"id"`
			RequestID string `json:"request_id"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/reimbursements", input, 201, &submitted)
	claimID, requestID = submitted.Data.ID, submitted.Data.RequestID
	if submitted.Data.Status != "in_review" {
		t.Fatalf("claim status: %s", submitted.Data.Status)
	}
	doJSON(t, ess, "DELETE", server.URL+"/api/v1/files/"+fileID, nil, 409, nil)
	doJSON(t, admin, "GET", server.URL+"/api/v1/files/"+fileID, nil, 200, nil)
	download, err = admin.Get(server.URL + "/api/v1/files/" + fileID + "/download")
	if err != nil {
		t.Fatal(err)
	}
	downloaded, err = io.ReadAll(download.Body)
	download.Body.Close()
	if err != nil || download.StatusCode != 200 || !bytes.Equal(downloaded, raw.Bytes()) {
		t.Fatalf("Admin download: status=%d err=%v", download.StatusCode, err)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/reimbursements", input, 409, nil)
	actionURL := server.URL + "/api/v1/admin/approvals/" + requestID + "/actions"
	doJSON(t, admin, "POST", actionURL, map[string]any{"action": "return", "reason": "Kuitansi tambahan dibutuhkan", "level_no": 1, "version": 1}, 200, nil)
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/reimbursements/"+claimID+"/receipts", map[string]any{"file_ids": []string{fileID}}, 409, nil)
	doJSON(t, ess, "POST", server.URL+"/api/v1/files/uploads", map[string]any{"purpose": "receipt", "original_name": "qa-extra.png", "mime_type": "image/png", "size_bytes": raw.Len(), "sha256": hex.EncodeToString(hash[:])}, 201, &upload)
	extraFileID = upload.Data.FileID
	req, _ = http.NewRequest("PUT", server.URL+"/api/v1/files/"+extraFileID+"/content", bytes.NewReader(raw.Bytes()))
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("X-CSRF-Token", csrfToken(cookieFromJar(t, ess, server.URL)))
	response, err = ess.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 204 {
		t.Fatalf("extra upload status: %d", response.StatusCode)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/files/"+extraFileID+"/complete", map[string]any{}, 200, nil)
	duplicateInput := map[string]any{"category_id": categoryID, "trx_date": input["trx_date"], "amount": 1500, "description": "QA duplicate", "file_ids": []string{extraFileID}}
	var suspected struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/reimbursements", duplicateInput, 409, &suspected)
	if suspected.Error.Code != "DUPLICATE_SUSPECTED" {
		t.Fatalf("duplicate code: %s", suspected.Error.Code)
	}
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/reimbursements/"+claimID+"/receipts", map[string]any{"file_ids": []string{extraFileID}}, 200, nil)
	doJSON(t, ess, "POST", server.URL+"/api/v1/ess/requests/"+requestID+"/resubmit", map[string]any{"version": 2, "correction": map[string]any{"note": "Kuitansi tambahan terlampir"}}, 200, nil)
	doJSON(t, admin, "POST", actionURL, map[string]any{"action": "approve", "level_no": 1, "version": 3}, 200, nil)
	var status string
	if err = db.QueryRow(`SELECT status FROM reimbursements WHERE tenant_id=? AND id=?`, tenantID, claimID).Scan(&status); err != nil || status != "approved" {
		t.Fatalf("approval domain: %s %v", status, err)
	}
	payURL := server.URL + "/api/v1/admin/reimbursements/" + claimID + "/pay"
	doJSON(t, admin, "POST", payURL, map[string]string{"payment_ref": "QA-" + uid}, 403, nil)
	doJSON(t, admin, "POST", server.URL+"/api/v1/auth/reauth", map[string]string{"password": "password"}, 200, nil)
	doJSON(t, admin, "POST", payURL, map[string]string{"payment_ref": "QA-" + uid}, 200, nil)
	doJSON(t, admin, "POST", payURL, map[string]string{"payment_ref": "QA-" + uid}, 200, nil)
	doJSON(t, admin, "POST", payURL, map[string]string{"payment_ref": "OTHER-" + uid}, 409, nil)
}
