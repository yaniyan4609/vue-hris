package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

var approvalBulkKeyPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type approvalBulkItem struct {
	RequestID string          `json:"request_id"`
	Action    string          `json:"action"`
	Reason    string          `json:"reason"`
	LevelNo   int             `json:"level_no"`
	Version   uint64          `json:"version"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type approvalBulkResult struct {
	RequestID  string          `json:"request_id"`
	StatusCode int             `json:"status_code"`
	Response   json.RawMessage `json:"response"`
}

type approvalBulkItemContext struct {
	TenantID  string
	BatchID   string
	Index     int
	RequestID string
}
type approvalBulkContextKey struct{}

// Success is written before the approval transaction commits. A crash after
// commit therefore cannot lose the first outcome of an idempotent bulk item.
func persistApprovalBulkSuccess(ctx context.Context, tx *sql.Tx, data any) error {
	item, ok := ctx.Value(approvalBulkContextKey{}).(approvalBulkItemContext)
	if !ok {
		return nil
	}
	body, err := json.Marshal(map[string]any{"data": data})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO approval_bulk_items (tenant_id,batch_id,item_index,request_id,status_code,response_json) VALUES (?,?,?,?,200,?)`, item.TenantID, item.BatchID, item.Index, item.RequestID, body)
	return err
}

func (a adminAPI) approvalBulkActions(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "persetujuan.lihat")
	if !ok {
		return
	}
	key := strings.ToLower(strings.TrimSpace(r.Header.Get("Idempotency-Key")))
	if key == "" {
		writeAPIError(w, r, 400, "IDEMPOTENCY_KEY_REQUIRED", "Header Idempotency-Key wajib diisi.")
		return
	}
	if !approvalBulkKeyPattern.MatchString(key) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Idempotency-Key harus UUID.")
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeAPIError(w, r, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type harus application/json.")
		return
	}
	var body struct {
		Items []approvalBulkItem `json:"items"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "JSON aksi massal tidak valid.")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "JSON aksi massal tidak valid.")
		return
	}
	if len(body.Items) == 0 || len(body.Items) > 100 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Daftar keputusan harus berisi 1–100 item.")
		return
	}
	seen := map[string]bool{}
	for _, item := range body.Items {
		if !fileIDPattern.MatchString(item.RequestID) || seen[item.RequestID] {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID pengajuan tidak valid atau berulang.")
			return
		}
		seen[item.RequestID] = true
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca keputusan.")
		return
	}
	hash := sha256.Sum256(canonical)
	batchID, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat batch.")
		return
	}
	_, insertErr := a.db.ExecContext(r.Context(), `INSERT INTO approval_bulk_batches (tenant_id,id,actor_user_id,idempotency_key,request_hash) VALUES (?,?,?,?,?)`, c.tenant.ID, batchID, c.state.User.ID, key, hash[:])
	if insertErr != nil {
		var storedHash []byte
		var existing sql.NullString
		err = a.db.QueryRowContext(r.Context(), `SELECT id,request_hash,response_json FROM approval_bulk_batches WHERE tenant_id=? AND actor_user_id=? AND idempotency_key=?`, c.tenant.ID, c.state.User.ID, key).Scan(&batchID, &storedHash, &existing)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa batch idempoten.")
			return
		}
		if !bytes.Equal(storedHash, hash[:]) {
			writeAPIError(w, r, 409, "IDEMPOTENCY_CONFLICT", "Kunci idempoten sudah dipakai untuk keputusan berbeda.")
			return
		}
		if existing.Valid {
			w.Header().Set("Idempotent-Replay", "true")
			writeBulkRaw(w, []byte(existing.String))
			return
		}
	}
	results := make([]approvalBulkResult, 0, len(body.Items))
	succeeded := 0
	for index, item := range body.Items {
		result, found, err := a.loadApprovalBulkItem(r.Context(), c.tenant.ID, batchID, index)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca hasil keputusan.")
			return
		}
		if !found {
			subBody, _ := json.Marshal(map[string]any{"action": item.Action, "reason": item.Reason, "level_no": item.LevelNo, "version": item.Version, "data": item.Data})
			route := chi.NewRouteContext()
			route.URLParams.Add("requestId", item.RequestID)
			ctx := context.WithValue(r.Context(), chi.RouteCtxKey, route)
			ctx = context.WithValue(ctx, approvalBulkContextKey{}, approvalBulkItemContext{TenantID: c.tenant.ID, BatchID: batchID, Index: index, RequestID: item.RequestID})
			subReq := r.Clone(ctx)
			subReq.Body = io.NopCloser(bytes.NewReader(subBody))
			subReq.Header = r.Header.Clone()
			subReq.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			a.decideApprovalRequest(recorder, subReq)
			if recorder.Code != 200 {
				_, err = a.db.ExecContext(r.Context(), `INSERT IGNORE INTO approval_bulk_items (tenant_id,batch_id,item_index,request_id,status_code,response_json) VALUES (?,?,?,?,?,?)`, c.tenant.ID, batchID, index, item.RequestID, recorder.Code, recorder.Body.Bytes())
				if err != nil {
					writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan hasil keputusan.")
					return
				}
			}
			result, found, err = a.loadApprovalBulkItem(r.Context(), c.tenant.ID, batchID, index)
			if err != nil || !found {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Hasil keputusan tidak tersedia.")
				return
			}
		}
		results = append(results, result)
		if result.StatusCode == 200 {
			succeeded++
		}
	}
	response, err := json.Marshal(map[string]any{"data": map[string]any{"items": results, "succeeded": succeeded, "failed": len(results) - succeeded}})
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyiapkan respons batch.")
		return
	}
	updated, err := a.db.ExecContext(r.Context(), `UPDATE approval_bulk_batches SET response_json=?,completed_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=? AND response_json IS NULL`, response, c.tenant.ID, batchID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menyimpan respons batch.")
		return
	}
	changed, _ := updated.RowsAffected()
	if changed == 0 {
		var prior []byte
		if err = a.db.QueryRowContext(r.Context(), `SELECT response_json FROM approval_bulk_batches WHERE tenant_id=? AND id=?`, c.tenant.ID, batchID).Scan(&prior); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca respons batch.")
			return
		}
		w.Header().Set("Idempotent-Replay", "true")
		response = prior
	}
	writeBulkRaw(w, response)
}

func (a adminAPI) loadApprovalBulkItem(ctx context.Context, tenantID, batchID string, index int) (approvalBulkResult, bool, error) {
	var item approvalBulkResult
	var raw []byte
	err := a.db.QueryRowContext(ctx, `SELECT request_id,status_code,response_json FROM approval_bulk_items WHERE tenant_id=? AND batch_id=? AND item_index=?`, tenantID, batchID, index).Scan(&item.RequestID, &item.StatusCode, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return item, false, nil
	}
	if err != nil {
		return item, false, err
	}
	item.Response = json.RawMessage(raw)
	return item, true, nil
}

func writeBulkRaw(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
