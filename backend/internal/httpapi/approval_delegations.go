package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type approvalDelegation struct {
	ID          string     `json:"id"`
	FromUserID  string     `json:"from_user_id"`
	ToUserID    string     `json:"to_user_id"`
	RequestType *string    `json:"request_type"`
	ValidFrom   string     `json:"valid_from"`
	ValidTo     *string    `json:"valid_to"`
	Reason      string     `json:"reason"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
}

func delegationAllTenant(c adminContext) bool {
	for _, rule := range c.effective.Scopes {
		if rule.AllTenant {
			return true
		}
	}
	return false
}

func (a adminAPI) listApprovalDelegations(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "persetujuan.delegasikan")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	where := `tenant_id=?`
	args := []any{c.tenant.ID}
	if !delegationAllTenant(c) {
		where += ` AND created_by=?`
		args = append(args, c.state.User.ID)
	}
	if from := r.URL.Query().Get("from_user_id"); from != "" {
		if !fileIDPattern.MatchString(from) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter pemberi tidak valid.")
			return
		}
		where += ` AND from_user_id=?`
		args = append(args, from)
	}
	if kind := r.URL.Query().Get("request_type"); kind != "" {
		if !approvalRequestTypes[kind] {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jenis pengajuan tidak valid.")
			return
		}
		where += ` AND (request_type=? OR request_type IS NULL)`
		args = append(args, kind)
	}
	if r.URL.Query().Get("active") == "true" {
		today := tenantToday(c.tenant.Timezone)
		where += ` AND revoked_at IS NULL AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?)`
		args = append(args, today, today)
	} else if v := r.URL.Query().Get("active"); v != "" && v != "false" {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter aktif tidak valid.")
		return
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM approval_delegations WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung delegasi.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,from_user_id,to_user_id,request_type,DATE_FORMAT(valid_from,'%Y-%m-%d'),DATE_FORMAT(valid_to,'%Y-%m-%d'),reason,COALESCE(created_by,''),created_at,revoked_at FROM approval_delegations WHERE `+where+` ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca delegasi.")
		return
	}
	defer rows.Close()
	items := []approvalDelegation{}
	for rows.Next() {
		var item approvalDelegation
		var kind, until sql.NullString
		var revoked sql.NullTime
		if err = rows.Scan(&item.ID, &item.FromUserID, &item.ToUserID, &kind, &item.ValidFrom, &until, &item.Reason, &item.CreatedBy, &item.CreatedAt, &revoked); err != nil {
			break
		}
		if kind.Valid {
			item.RequestType = &kind.String
		}
		if until.Valid {
			item.ValidTo = &until.String
		}
		if revoked.Valid {
			item.RevokedAt = &revoked.Time
		}
		items = append(items, item)
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca delegasi.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) createApprovalDelegation(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "persetujuan.delegasikan")
	if !ok {
		return
	}
	var body struct {
		FromUserID  string  `json:"from_user_id"`
		ToUserID    string  `json:"to_user_id"`
		RequestType string  `json:"request_type"`
		ValidFrom   string  `json:"valid_from"`
		ValidTo     *string `json:"valid_to"`
		Reason      string  `json:"reason"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if !fileIDPattern.MatchString(body.FromUserID) || !fileIDPattern.MatchString(body.ToUserID) || body.FromUserID == body.ToUserID || body.RequestType != "" && !approvalRequestTypes[body.RequestType] || !validRuleDates(body.ValidFrom, body.ValidTo) || len([]rune(body.Reason)) < 10 || len([]rune(body.Reason)) > 500 {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Pihak, jenis, masa berlaku, atau alasan delegasi tidak valid.")
		return
	}
	verified, err := a.auth.RecentlyReauthenticated(r.Context(), cookieToken(r))
	if err != nil || !verified {
		writeAPIError(w, r, 403, "REAUTH_REQUIRED", "Verifikasi ulang sebelum membuat delegasi.")
		return
	}
	if body.RequestType != "" {
		permission := approvalPermissions[body.RequestType]
		for _, userID := range []string{c.state.User.ID, body.FromUserID, body.ToUserID} {
			_, right, e := a.access.Decide(r.Context(), c.tenant.ID, userID, c.tenant.Timezone, permission)
			if e != nil {
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak approve.")
				return
			}
			if !right.Allowed {
				writeAPIError(w, r, 403, "DELEGATION_EXCEEDS_RIGHTS", "Pihak delegasi harus memiliki hak approve jenis ini.")
				return
			}
		}
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat delegasi.")
		return
	}
	defer tx.Rollback()
	var locked string
	if err = tx.QueryRowContext(r.Context(), `SELECT id FROM tenants WHERE id=? FOR UPDATE`, c.tenant.ID).Scan(&locked); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mengunci tenant.")
		return
	}
	var members int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM tenant_memberships WHERE tenant_id=? AND user_id IN (?,?) AND status='active'`, c.tenant.ID, body.FromUserID, body.ToUserID).Scan(&members); err != nil || members != 2 {
		writeAPIError(w, r, 409, "MEMBERSHIP_UNAVAILABLE", "Pemberi atau penerima tidak aktif pada tenant.")
		return
	}
	var overlap int
	var end any = "9999-12-31"
	if body.ValidTo != nil {
		end = *body.ValidTo
	}
	err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM approval_delegations WHERE tenant_id=? AND from_user_id=? AND revoked_at IS NULL AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) AND (request_type IS NULL OR ?='' OR request_type=?)`, c.tenant.ID, body.FromUserID, end, body.ValidFrom, body.RequestType, body.RequestType).Scan(&overlap)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa masa delegasi.")
		return
	}
	if overlap != 0 {
		writeAPIError(w, r, 409, "DELEGATION_OVERLAP", "Delegasi pemberi dan jenis ini sudah berlaku pada tanggal yang sama.")
		return
	}
	var reverse int
	err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM approval_delegations WHERE tenant_id=? AND from_user_id=? AND to_user_id=? AND revoked_at IS NULL AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) AND (request_type IS NULL OR ?='' OR request_type=?)`, c.tenant.ID, body.ToUserID, body.FromUserID, end, body.ValidFrom, body.RequestType, body.RequestType).Scan(&reverse)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa siklus delegasi.")
		return
	}
	if reverse != 0 {
		writeAPIError(w, r, 409, "DELEGATION_CYCLE", "Delegasi timbal balik tidak diperbolehkan.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID delegasi.")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO approval_delegations (tenant_id,id,from_user_id,to_user_id,request_type,valid_from,valid_to,reason,created_by) VALUES (?,?,?,?,?,?,?,?,?)`, c.tenant.ID, id, body.FromUserID, body.ToUserID, nullableString(body.RequestType), body.ValidFrom, body.ValidTo, body.Reason, c.state.User.ID); err != nil {
		writeAPIError(w, r, 409, "DELEGATION_CONFLICT", "Delegasi tidak dapat dibuat.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "approval_delegations", id, nil, map[string]any{"from_user_id": body.FromUserID, "to_user_id": body.ToUserID, "request_type": body.RequestType, "valid_from": body.ValidFrom, "valid_to": body.ValidTo}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit delegasi.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat delegasi.")
		return
	}
	writeAPIData(w, 201, map[string]any{"id": id, "from_user_id": body.FromUserID, "to_user_id": body.ToUserID, "request_type": body.RequestType, "valid_from": body.ValidFrom, "valid_to": body.ValidTo}, nil)
}

func (a adminAPI) revokeApprovalDelegation(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "persetujuan.delegasikan")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Delegasi tidak ditemukan.")
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencabut delegasi.")
		return
	}
	defer tx.Rollback()
	var creator string
	var revoked sql.NullTime
	err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(created_by,''),revoked_at FROM approval_delegations WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, id).Scan(&creator, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Delegasi tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca delegasi.")
		return
	}
	if creator != c.state.User.ID && !delegationAllTenant(c) {
		writeAPIError(w, r, 403, "SCOPE_DENIED", "Delegasi di luar kewenangan.")
		return
	}
	if revoked.Valid {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE approval_delegations SET revoked_at=UTC_TIMESTAMP(6),revoked_by=? WHERE tenant_id=? AND id=?`, c.state.User.ID, c.tenant.ID, id); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencabut delegasi.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "revoke", "approval_delegations", id, nil, map[string]any{"revoked": true}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat audit delegasi.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencabut delegasi.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
