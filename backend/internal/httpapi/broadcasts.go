package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type broadcastTarget struct {
	Type  string  `json:"target_type"`
	RefID *string `json:"ref_id"`
}
type broadcastInput struct {
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Targets     []broadcastTarget `json:"targets"`
	CombineMode string            `json:"combine_mode"`
	Version     uint64            `json:"version,omitempty"`
}
type broadcastRecord struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	Body             string            `json:"body"`
	Status           string            `json:"status"`
	SendAt           *time.Time        `json:"send_at,omitempty"`
	CombineMode      string            `json:"combine_mode"`
	Version          uint64            `json:"version"`
	RecipientsCount  int               `json:"recipients_count"`
	ProcessedCount   int               `json:"processed_count"`
	FailedCount      int               `json:"failed_count"`
	AttachmentFileID *string           `json:"attachment_file_id,omitempty"`
	Targets          []broadcastTarget `json:"targets"`
}

func (a adminAPI) mountBroadcasts(r chi.Router, config Options) {
	r.Get("/broadcasts", a.listBroadcasts)
	r.With(checkOrigin(config)).Post("/broadcasts", a.createBroadcast)
	r.With(checkOrigin(config)).Post("/broadcasts/audience-preview", a.previewBroadcastInput)
	r.Get("/broadcasts/{id}", a.getBroadcast)
	r.With(checkOrigin(config)).Patch("/broadcasts/{id}", a.updateBroadcast)
	r.With(checkOrigin(config)).Delete("/broadcasts/{id}", a.deleteBroadcast)
	r.With(checkOrigin(config)).Post("/broadcasts/{id}/audience-preview", a.previewBroadcastSaved)
	r.With(checkOrigin(config)).Post("/broadcasts/{id}/attachments", a.attachBroadcastFile)
	r.With(checkOrigin(config)).Post("/broadcasts/{id}/send", a.sendBroadcast)
	r.With(checkOrigin(config)).Post("/broadcasts/{id}/schedule", a.scheduleBroadcast)
	r.With(checkOrigin(config)).Post("/broadcasts/{id}/cancel", a.cancelBroadcast)
	r.Get("/broadcasts/{id}/recipients", a.listBroadcastRecipients)
	r.Get("/broadcasts/{id}/stats", a.broadcastStats)
}

func normalizeBroadcast(in *broadcastInput) bool {
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	if in.Title == "" || in.Body == "" || len([]rune(in.Title)) > 255 || len(in.Body) > 20000 {
		return false
	}
	return normalizeBroadcastTargets(in)
}

func normalizeBroadcastTargets(in *broadcastInput) bool {
	if len(in.Targets) == 0 || len(in.Targets) > 30 {
		return false
	}
	if in.CombineMode == "" {
		in.CombineMode = "or"
	}
	if in.CombineMode != "or" && in.CombineMode != "and" {
		return false
	}
	seen := map[string]bool{}
	for _, target := range in.Targets {
		var valid bool
		switch target.Type {
		case "all":
			valid = target.RefID == nil
		case "region", "office", "division", "department", "position", "user":
			valid = target.RefID != nil && fileIDPattern.MatchString(*target.RefID)
		}
		if !valid {
			return false
		}
		key := target.Type + ":"
		if target.RefID != nil {
			key += *target.RefID
		}
		if seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

func readBroadcastJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeAPIError(w, r, 415, "UNSUPPORTED_MEDIA_TYPE", "Content-Type harus application/json.")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "JSON broadcast tidak valid.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "JSON broadcast tidak valid.")
		return false
	}
	return true
}

func (a adminAPI) broadcastActor(w http.ResponseWriter, r *http.Request, permission string) (adminContext, bool) {
	return a.authorize(w, r, permission)
}

func (a adminAPI) checkBroadcastTargets(w http.ResponseWriter, r *http.Request, c adminContext, targets []broadcastTarget) bool {
	for _, target := range targets {
		if target.Type == "all" {
			_, decision, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "broadcast.target_semua")
			if err != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa target.")
				return false
			}
			if !decision.Allowed {
				writeAPIError(w, r, 403, "TARGET_ALL_FORBIDDEN", "Target seluruh tenant tidak diizinkan.")
				return false
			}
			continue
		}
		table := map[string]string{"region": "regions", "office": "offices", "division": "divisions", "department": "departments", "position": "positions", "user": "tenant_memberships"}[target.Type]
		column := "id"
		if target.Type == "user" {
			column = "user_id"
		}
		var count int
		if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM `+table+` WHERE tenant_id=? AND `+column+`=?`, c.tenant.ID, *target.RefID).Scan(&count); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa target.")
			return false
		}
		if count == 0 {
			writeAPIError(w, r, 422, "VALIDATION_FAILED", "Target tidak ditemukan dalam tenant.")
			return false
		}
	}
	return true
}

func (a adminAPI) insertBroadcastTargets(r *http.Request, tx *sql.Tx, tenant, id string, targets []broadcastTarget) error {
	for _, target := range targets {
		targetID, err := ids.New()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(r.Context(), `INSERT INTO broadcast_targets (tenant_id,id,broadcast_id,target_type,ref_id) VALUES (?,?,?,?,?)`, tenant, targetID, id, target.Type, target.RefID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a adminAPI) readBroadcast(r *http.Request, tenant, id string) (broadcastRecord, error) {
	var b broadcastRecord
	var sendAt sql.NullTime
	var attachment sql.NullString
	err := a.db.QueryRowContext(r.Context(), `SELECT id,title,body,status,send_at,combine_mode,version,recipients_count,processed_count,failed_count,attachment_file_id FROM broadcasts WHERE tenant_id=? AND id=?`, tenant, id).Scan(&b.ID, &b.Title, &b.Body, &b.Status, &sendAt, &b.CombineMode, &b.Version, &b.RecipientsCount, &b.ProcessedCount, &b.FailedCount, &attachment)
	if err != nil {
		return b, err
	}
	if sendAt.Valid {
		b.SendAt = &sendAt.Time
	}
	if attachment.Valid {
		b.AttachmentFileID = &attachment.String
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT target_type,ref_id FROM broadcast_targets WHERE tenant_id=? AND broadcast_id=? ORDER BY id`, tenant, id)
	if err != nil {
		return b, err
	}
	defer rows.Close()
	b.Targets = []broadcastTarget{}
	for rows.Next() {
		var target broadcastTarget
		var ref sql.NullString
		if err := rows.Scan(&target.Type, &ref); err != nil {
			return b, err
		}
		if ref.Valid {
			target.RefID = &ref.String
		}
		b.Targets = append(b.Targets, target)
	}
	return b, rows.Err()
}

func (a adminAPI) listBroadcasts(w http.ResponseWriter, r *http.Request) {
	c, ok := a.broadcastActor(w, r, "broadcast.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	status := r.URL.Query().Get("status")
	if status != "" && status != "draft" && status != "scheduled" && status != "queued" && status != "sending" && status != "sent" && status != "cancelled" && status != "failed" {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status tidak valid.")
		return
	}
	where := ` FROM broadcasts WHERE tenant_id=?`
	args := []any{c.tenant.ID}
	if status != "" {
		where += ` AND status=?`
		args = append(args, status)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca broadcast.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,title,body,status,send_at,combine_mode,version,recipients_count,processed_count,failed_count,attachment_file_id`+where+` ORDER BY updated_at DESC,id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca broadcast.")
		return
	}
	defer rows.Close()
	items := []broadcastRecord{}
	for rows.Next() {
		var b broadcastRecord
		var send sql.NullTime
		var file sql.NullString
		if rows.Scan(&b.ID, &b.Title, &b.Body, &b.Status, &send, &b.CombineMode, &b.Version, &b.RecipientsCount, &b.ProcessedCount, &b.FailedCount, &file) != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca broadcast.")
			return
		}
		if send.Valid {
			b.SendAt = &send.Time
		}
		if file.Valid {
			b.AttachmentFileID = &file.String
		}
		items = append(items, b)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca broadcast.")
		return
	}
	writeAPIData(w, 200, items, map[string]any{"total": total, "page": page.Page, "page_size": page.PageSize})
}

func (a adminAPI) createBroadcast(w http.ResponseWriter, r *http.Request) {
	c, ok := a.broadcastActor(w, r, "broadcast.buat")
	if !ok {
		return
	}
	var in broadcastInput
	if !readBroadcastJSON(w, r, &in) {
		return
	}
	if !normalizeBroadcast(&in) {
		writeAPIError(w, r, 422, "VALIDATION_FAILED", "Broadcast tidak valid.")
		return
	}
	if !a.checkBroadcastTargets(w, r, c, in.Targets) {
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat broadcast.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat broadcast.")
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), `INSERT INTO broadcasts (tenant_id,id,title,body,combine_mode,created_by) VALUES (?,?,?,?,?,?)`, c.tenant.ID, id, in.Title, in.Body, in.CombineMode, c.state.User.ID)
	if err == nil {
		err = a.insertBroadcastTargets(r, tx, c.tenant.ID, id, in.Targets)
	}
	if err == nil {
		err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "broadcasts", id, nil, map[string]any{"title": in.Title, "target_count": len(in.Targets)})
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat broadcast.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "status": "draft", "version": 1}, nil)
}

func (a adminAPI) getBroadcast(w http.ResponseWriter, r *http.Request) {
	c, ok := a.broadcastActor(w, r, "broadcast.lihat")
	if !ok {
		return
	}
	b, err := a.readBroadcast(r, c.tenant.ID, chi.URLParam(r, "id"))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Broadcast tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca broadcast.")
		return
	}
	writeAPIData(w, 200, b, nil)
}

func (a adminAPI) updateBroadcast(w http.ResponseWriter, r *http.Request) {
	c, ok := a.broadcastActor(w, r, "broadcast.ubah")
	if !ok {
		return
	}
	var in broadcastInput
	if !readBroadcastJSON(w, r, &in) {
		return
	}
	if !normalizeBroadcast(&in) || in.Version == 0 {
		writeAPIError(w, r, 422, "VALIDATION_FAILED", "Broadcast atau versi tidak valid.")
		return
	}
	if !a.checkBroadcastTargets(w, r, c, in.Targets) {
		return
	}
	id := chi.URLParam(r, "id")
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah broadcast.")
		return
	}
	defer tx.Rollback()
	var status string
	var version uint64
	err = tx.QueryRowContext(r.Context(), `SELECT status,version FROM broadcasts WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id).Scan(&status, &version)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Broadcast tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah broadcast.")
		return
	}
	if status != "draft" && status != "scheduled" {
		writeAPIError(w, r, 409, "CONFLICT", "Broadcast tidak dapat diubah.")
		return
	}
	if version != in.Version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Versi broadcast berubah.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE broadcasts SET title=?,body=?,combine_mode=?,version=version+1 WHERE tenant_id=? AND id=?`, in.Title, in.Body, in.CombineMode, c.tenant.ID, id)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `DELETE FROM broadcast_targets WHERE tenant_id=? AND broadcast_id=?`, c.tenant.ID, id)
	}
	if err == nil {
		err = a.insertBroadcastTargets(r, tx, c.tenant.ID, id, in.Targets)
	}
	if err == nil {
		err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "update", "broadcasts", id, nil, map[string]any{"version": version + 1, "target_count": len(in.Targets)})
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah broadcast.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "version": version + 1, "status": status}, nil)
}

func (a adminAPI) deleteBroadcast(w http.ResponseWriter, r *http.Request) {
	c, ok := a.broadcastActor(w, r, "broadcast.ubah")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus broadcast.")
		return
	}
	defer tx.Rollback()
	var status string
	err = tx.QueryRowContext(r.Context(), `SELECT status FROM broadcasts WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Broadcast tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus broadcast.")
		return
	}
	if status != "draft" {
		writeAPIError(w, r, 409, "CONFLICT", "Hanya draft yang dapat dihapus.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `DELETE FROM broadcast_targets WHERE tenant_id=? AND broadcast_id=?`, c.tenant.ID, id)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), `DELETE FROM broadcasts WHERE tenant_id=? AND id=?`, c.tenant.ID, id)
	}
	if err == nil {
		err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "delete", "broadcasts", id, map[string]any{"status": "draft"}, nil)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghapus broadcast.")
		return
	}
	w.WriteHeader(204)
}

func (a adminAPI) attachBroadcastFile(w http.ResponseWriter, r *http.Request) {
	c, ok := a.broadcastActor(w, r, "broadcast.ubah")
	if !ok {
		return
	}
	var in struct {
		FileID  string `json:"file_id"`
		Version uint64 `json:"version"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if !fileIDPattern.MatchString(in.FileID) || in.Version == 0 {
		writeAPIError(w, r, 422, "VALIDATION_FAILED", "Berkas atau versi tidak valid.")
		return
	}
	var count int
	err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM files WHERE tenant_id=? AND id=? AND uploaded_by=? AND purpose='broadcast' AND scan_status='clean' AND deleted_at IS NULL`, c.tenant.ID, in.FileID, c.state.User.ID).Scan(&count)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa berkas.")
		return
	}
	if count == 0 {
		writeAPIError(w, r, 422, "VALIDATION_FAILED", "Berkas bersih tidak ditemukan.")
		return
	}
	id := chi.URLParam(r, "id")
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal melampirkan berkas.")
		return
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(r.Context(), `UPDATE broadcasts SET attachment_file_id=?,version=version+1 WHERE tenant_id=? AND id=? AND version=? AND status IN ('draft','scheduled')`, in.FileID, c.tenant.ID, id, in.Version)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal melampirkan berkas.")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Broadcast tidak ditemukan atau versinya berubah.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "attach", "broadcasts", id, nil, map[string]any{"attachment_file_id": in.FileID}); err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal melampirkan berkas.")
		return
	}
	writeAPIData(w, 200, map[string]any{"id": id, "attachment_file_id": in.FileID, "version": in.Version + 1}, nil)
}

func (a adminAPI) sendBroadcast(w http.ResponseWriter, r *http.Request) {
	a.transitionBroadcast(w, r, "queued")
}
func (a adminAPI) scheduleBroadcast(w http.ResponseWriter, r *http.Request) {
	a.transitionBroadcast(w, r, "scheduled")
}
func (a adminAPI) cancelBroadcast(w http.ResponseWriter, r *http.Request) {
	a.transitionBroadcast(w, r, "cancelled")
}

func (a adminAPI) transitionBroadcast(w http.ResponseWriter, r *http.Request, next string) {
	c, ok := a.broadcastActor(w, r, "broadcast.kirim")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	sendKey := ""
	if next == "queued" {
		sendKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if len(sendKey) == 0 || len(sendKey) > 128 || strings.IndexFunc(sendKey, func(ch rune) bool { return ch < 33 || ch > 126 }) >= 0 {
			writeAPIError(w, r, 400, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key wajib dan harus valid.")
			return
		}
		var previous sql.NullString
		var currentStatus string
		var currentVersion uint64
		err := a.db.QueryRowContext(r.Context(), `SELECT status,version,send_idempotency_key FROM broadcasts WHERE tenant_id=? AND id=?`, c.tenant.ID, id).Scan(&currentStatus, &currentVersion, &previous)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Broadcast tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca broadcast.")
			return
		}
		if previous.Valid && previous.String == sendKey {
			w.Header().Set("Idempotent-Replay", "true")
			writeAPIData(w, 202, map[string]any{"id": id, "status": currentStatus, "version": currentVersion, "job_id": id}, nil)
			return
		}
	}
	if next == "queued" {
		preview, err := a.readBroadcast(r, c.tenant.ID, id)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, r, 404, "NOT_FOUND", "Broadcast tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca broadcast.")
			return
		}
		if !a.checkBroadcastTargets(w, r, c, preview.Targets) {
			return
		}
		audience, err := a.resolveBroadcastAudience(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, preview.Targets, preview.CombineMode)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung penerima.")
			return
		}
		if audience.RecipientsCount == 0 {
			writeAPIError(w, r, 422, "AUDIENCE_EMPTY", "Tidak ada penerima yang memenuhi target dan scope.")
			return
		}
	}
	var in struct {
		SendAt  string `json:"send_at"`
		Version uint64 `json:"version"`
	}
	if next == "scheduled" {
		if !readJSON(w, r, &in) {
			return
		}
	}
	var at time.Time
	if next == "scheduled" {
		var err error
		at, err = time.Parse(time.RFC3339, in.SendAt)
		if err != nil || at.Before(time.Now().UTC().Add(time.Minute)) || at.After(time.Now().UTC().AddDate(1, 0, 0)) {
			writeAPIError(w, r, 422, "VALIDATION_FAILED", "Waktu kirim tidak valid.")
			return
		}
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah broadcast.")
		return
	}
	defer tx.Rollback()
	var status string
	var version uint64
	var previous sql.NullString
	err = tx.QueryRowContext(r.Context(), `SELECT status,version,send_idempotency_key FROM broadcasts WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id).Scan(&status, &version, &previous)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Broadcast tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca broadcast.")
		return
	}
	if next == "queued" && previous.Valid && previous.String == sendKey {
		w.Header().Set("Idempotent-Replay", "true")
		writeAPIData(w, 202, map[string]any{"id": id, "status": status, "version": version, "job_id": id}, nil)
		return
	}
	if next == "cancelled" {
		if status != "scheduled" && status != "queued" && status != "sending" {
			writeAPIError(w, r, 409, "CONFLICT", "Broadcast tidak dapat dibatalkan.")
			return
		}
	} else if status != "draft" && status != "scheduled" {
		writeAPIError(w, r, 409, "CONFLICT", "Broadcast tidak dapat dikirim.")
		return
	}
	if in.Version != 0 && in.Version != version {
		writeAPIError(w, r, 409, "VERSION_CONFLICT", "Versi broadcast berubah.")
		return
	}
	var sendAt any
	if next == "scheduled" {
		sendAt = at.UTC()
	} else if next == "queued" {
		sendAt = time.Now().UTC()
	}
	if next == "cancelled" {
		_, err = tx.ExecContext(r.Context(), `UPDATE broadcasts SET status='cancelled',version=version+1,completed_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND id=?`, c.tenant.ID, id)
	} else if next == "queued" {
		_, err = tx.ExecContext(r.Context(), `UPDATE broadcasts SET status=?,send_at=?,sent_by=?,send_idempotency_key=?,version=version+1 WHERE tenant_id=? AND id=?`, next, sendAt, c.state.User.ID, sendKey, c.tenant.ID, id)
	} else {
		_, err = tx.ExecContext(r.Context(), `UPDATE broadcasts SET status=?,send_at=?,sent_by=?,version=version+1 WHERE tenant_id=? AND id=?`, next, sendAt, c.state.User.ID, c.tenant.ID, id)
	}
	if err == nil {
		err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, next, "broadcasts", id, map[string]any{"status": status}, map[string]any{"status": next})
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengubah broadcast.")
		return
	}
	writeAPIData(w, 202, map[string]any{"id": id, "status": next, "version": version + 1, "job_id": id}, nil)
}

func (a adminAPI) listBroadcastRecipients(w http.ResponseWriter, r *http.Request) {
	c, ok := a.broadcastActor(w, r, "broadcast.lihat_penerima")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var exists int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM broadcasts WHERE tenant_id=? AND id=?`, c.tenant.ID, id).Scan(&exists); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca broadcast.")
		return
	}
	if exists == 0 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Broadcast tidak ditemukan.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT user_id,employee_id,notification_id,delivery_status,read_at FROM broadcast_recipients WHERE tenant_id=? AND broadcast_id=? ORDER BY id LIMIT ? OFFSET ?`, c.tenant.ID, id, page.PageSize, page.Offset)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca penerima.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var user string
		var emp, nid sql.NullString
		var status string
		var read sql.NullTime
		if rows.Scan(&user, &emp, &nid, &status, &read) != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca penerima.")
			return
		}
		item := map[string]any{"user_id": user, "delivery_status": status}
		if emp.Valid {
			item["employee_id"] = emp.String
		}
		if nid.Valid {
			item["notification_id"] = nid.String
		}
		if read.Valid {
			item["read_at"] = read.Time
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca penerima.")
		return
	}
	writeAPIData(w, 200, items, map[string]any{"page": page.Page, "page_size": page.PageSize})
}

func (a adminAPI) broadcastStats(w http.ResponseWriter, r *http.Request) {
	c, ok := a.broadcastActor(w, r, "broadcast.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var status string
	var total, sent, failed int
	err := a.db.QueryRowContext(r.Context(), `SELECT status,recipients_count,processed_count,failed_count FROM broadcasts WHERE tenant_id=? AND id=?`, c.tenant.ID, id).Scan(&status, &total, &sent, &failed)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Broadcast tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca statistik.")
		return
	}
	var read int
	err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM broadcast_recipients WHERE tenant_id=? AND broadcast_id=? AND read_at IS NOT NULL`, c.tenant.ID, id).Scan(&read)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca statistik.")
		return
	}
	writeAPIData(w, 200, map[string]any{"status": status, "total": total, "sent": sent, "failed": failed, "read": read}, nil)
}

func mountAnnouncements(r chi.Router, db *sql.DB, config Options) {
	r.Get("/api/v1/ess/announcements", func(w http.ResponseWriter, req *http.Request) {
		p, ok := notificationIdentity(w, req, db)
		if !ok {
			return
		}
		limit := 30
		if raw := req.URL.Query().Get("limit"); raw != "" {
			v, e := strconv.Atoi(raw)
			if e != nil || v < 1 || v > 100 {
				writeAPIError(w, req, 400, "VALIDATION_FAILED", "Limit tidak valid.")
				return
			}
			limit = v
		}
		query := `SELECT b.id,b.title,b.body,b.attachment_file_id,b.snapshot_at,br.read_at FROM broadcast_recipients br JOIN broadcasts b ON b.tenant_id=br.tenant_id AND b.id=br.broadcast_id WHERE br.tenant_id=? AND br.user_id=? AND br.delivery_status='delivered' AND b.status IN ('sending','sent')`
		args := []any{p.TenantID, p.UserID}
		if cursor := req.URL.Query().Get("cursor"); cursor != "" {
			query += ` AND b.id<?`
			args = append(args, cursor)
		}
		query += ` ORDER BY b.id DESC LIMIT ?`
		args = append(args, limit+1)
		rows, err := db.QueryContext(req.Context(), query, args...)
		if err != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca pengumuman.")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, title, body string
			var file sql.NullString
			var snapshot, read sql.NullTime
			if rows.Scan(&id, &title, &body, &file, &snapshot, &read) != nil {
				writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca pengumuman.")
				return
			}
			item := map[string]any{"id": id, "title": title, "body": body, "read": read.Valid}
			if file.Valid {
				item["attachment_file_id"] = file.String
			}
			if snapshot.Valid {
				item["sent_at"] = snapshot.Time
			}
			items = append(items, item)
		}
		if rows.Err() != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca pengumuman.")
			return
		}
		var cursor any
		if len(items) > limit {
			items = items[:limit]
			cursor = items[len(items)-1]["id"]
		}
		writeAPIData(w, 200, items, map[string]any{"next_cursor": cursor})
	})
	r.Get("/api/v1/ess/announcements/{id}", func(w http.ResponseWriter, req *http.Request) {
		p, ok := notificationIdentity(w, req, db)
		if !ok {
			return
		}
		id := chi.URLParam(req, "id")
		tx, err := db.BeginTx(req.Context(), nil)
		if err != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca pengumuman.")
			return
		}
		defer tx.Rollback()
		var title, body string
		var file sql.NullString
		var notification sql.NullString
		err = tx.QueryRowContext(req.Context(), `SELECT b.title,b.body,b.attachment_file_id,br.notification_id FROM broadcast_recipients br JOIN broadcasts b ON b.tenant_id=br.tenant_id AND b.id=br.broadcast_id WHERE br.tenant_id=? AND br.user_id=? AND br.broadcast_id=? AND br.delivery_status='delivered' AND b.status IN ('sending','sent') FOR UPDATE`, p.TenantID, p.UserID, id).Scan(&title, &body, &file, &notification)
		if errors.Is(err, sql.ErrNoRows) {
			writeAPIError(w, req, 404, "NOT_FOUND", "Pengumuman tidak ditemukan.")
			return
		}
		if err != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca pengumuman.")
			return
		}
		_, err = tx.ExecContext(req.Context(), `UPDATE broadcast_recipients SET read_at=COALESCE(read_at,UTC_TIMESTAMP(6)) WHERE tenant_id=? AND broadcast_id=? AND user_id=?`, p.TenantID, id, p.UserID)
		if err == nil && notification.Valid {
			_, err = tx.ExecContext(req.Context(), `UPDATE notifications SET read_at=COALESCE(read_at,UTC_TIMESTAMP(6)) WHERE tenant_id=? AND id=? AND recipient_user_id=?`, p.TenantID, notification.String, p.UserID)
		}
		if err == nil && notification.Valid {
			ref := notification.String
			err = emitNotificationStream(req.Context(), tx, p.TenantID, p.UserID, "notification.updated", &ref, map[string]any{"id": ref, "action": "read"})
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			writeAPIError(w, req, 500, "INTERNAL_ERROR", "Gagal membaca pengumuman.")
			return
		}
		out := map[string]any{"id": id, "title": title, "body": body, "read": true}
		if file.Valid {
			out["attachment_file_id"] = file.String
		}
		writeAPIData(w, 200, out, nil)
	})
}

func encodeBroadcastTargets(targets []broadcastTarget) string {
	raw, _ := json.Marshal(targets)
	return string(raw)
}

func (a adminAPI) broadcastAudienceScope(w http.ResponseWriter, r *http.Request, c adminContext) (access.Subject, access.Effective, bool) {
	subject, decision, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "karyawan.lihat")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa scope.")
		return subject, decision, false
	}
	if !decision.Allowed {
		writeAPIError(w, r, 403, "PERMISSION_DENIED", "Scope karyawan tidak tersedia.")
		return subject, decision, false
	}
	return subject, decision, true
}
