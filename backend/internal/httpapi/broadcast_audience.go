package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type broadcastMember struct {
	UserID     string `json:"user_id"`
	EmployeeID string `json:"employee_id"`
}
type broadcastAudience struct {
	Recipients            []broadcastMember `json:"-"`
	RecipientsCount       int               `json:"recipients_count"`
	ExcludedByScope       int               `json:"excluded_by_scope_count"`
	ExcludedByEligibility int               `json:"excluded_by_eligibility_count"`
	PerTarget             []map[string]any  `json:"per_target"`
	Samples               []broadcastMember `json:"samples,omitempty"`
}

func broadcastTargetPredicate(targets []broadcastTarget, mode string) (string, []any) {
	parts := make([]string, 0, len(targets))
	args := []any{}
	columns := map[string]string{"region": "a.region_id", "office": "a.office_id", "division": "a.division_id", "department": "a.department_id", "position": "a.position_id", "user": "m.user_id"}
	for _, target := range targets {
		if target.Type == "all" {
			parts = append(parts, "1=1")
			continue
		}
		parts = append(parts, columns[target.Type]+"=?")
		args = append(args, *target.RefID)
	}
	join := " OR "
	if mode == "and" {
		join = " AND "
	}
	return "(" + strings.Join(parts, join) + ")", args
}

func (a adminAPI) resolveBroadcastAudience(ctx context.Context, tenantID, actorID, timezone string, targets []broadcastTarget, mode string) (broadcastAudience, error) {
	var out broadcastAudience
	out.Recipients = []broadcastMember{}
	out.PerTarget = []map[string]any{}
	subject, decision, err := a.access.Decide(ctx, tenantID, actorID, timezone, "karyawan.lihat")
	if err != nil {
		return out, err
	}
	if !decision.Allowed {
		return out, errors.New("sender scope unavailable")
	}
	scope, scopeArgs := access.EmployeeWhere(subject, decision)
	match, matchArgs := broadcastTargetPredicate(targets, mode)
	base := ` FROM tenant_memberships m JOIN users u ON u.id=m.user_id JOIN employees e ON e.tenant_id=m.tenant_id AND e.user_id=m.user_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE m.tenant_id=? AND m.status='active' AND u.status='active' AND e.deleted_at IS NULL AND e.status NOT IN ('resigned','terminated') AND ` + match
	args := append([]any{tenantID}, matchArgs...)
	var matching int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT m.user_id)`+base, args...).Scan(&matching); err != nil {
		return out, err
	}
	query := `SELECT DISTINCT m.user_id,e.id` + base + ` AND ` + scope + ` ORDER BY m.user_id`
	rows, err := a.db.QueryContext(ctx, query, append(args, scopeArgs...)...)
	if err != nil {
		return out, err
	}
	inside := 0
	for rows.Next() {
		var userID, employeeID string
		if err := rows.Scan(&userID, &employeeID); err != nil {
			rows.Close()
			return out, err
		}
		inside++
		_, allowed, e := a.access.Decide(ctx, tenantID, userID, timezone, "notifikasi.lihat_sendiri")
		if e != nil {
			rows.Close()
			return out, e
		}
		if !allowed.Allowed {
			out.ExcludedByEligibility++
			continue
		}
		out.Recipients = append(out.Recipients, broadcastMember{UserID: userID, EmployeeID: employeeID})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.RecipientsCount = len(out.Recipients)
	out.ExcludedByScope = matching - inside
	for _, target := range targets {
		predicate, partArgs := broadcastTargetPredicate([]broadcastTarget{target}, "or")
		var n int
		partBase := ` FROM tenant_memberships m JOIN users u ON u.id=m.user_id JOIN employees e ON e.tenant_id=m.tenant_id AND e.user_id=m.user_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE WHERE m.tenant_id=? AND m.status='active' AND u.status='active' AND e.deleted_at IS NULL AND ` + predicate + ` AND ` + scope
		if err := a.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT m.user_id)`+partBase, append(append([]any{tenantID}, partArgs...), scopeArgs...)...).Scan(&n); err != nil {
			return out, err
		}
		entry := map[string]any{"target_type": target.Type, "count": n}
		if target.RefID != nil {
			entry["ref_id"] = *target.RefID
		}
		out.PerTarget = append(out.PerTarget, entry)
	}
	return out, nil
}

func (a adminAPI) previewBroadcastInput(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "broadcast.buat")
	if !ok {
		return
	}
	var in broadcastInput
	if !readBroadcastJSON(w, r, &in) {
		return
	}
	if !normalizeBroadcastTargets(&in) {
		writeAPIError(w, r, 422, "VALIDATION_FAILED", "Target broadcast tidak valid.")
		return
	}
	if !a.checkBroadcastTargets(w, r, c, in.Targets) {
		return
	}
	a.previewBroadcastAudience(w, r, c, in.Targets, in.CombineMode)
}

func (a adminAPI) previewBroadcastSaved(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "broadcast.buat")
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
	if !a.checkBroadcastTargets(w, r, c, b.Targets) {
		return
	}
	a.previewBroadcastAudience(w, r, c, b.Targets, b.CombineMode)
}

func (a adminAPI) previewBroadcastAudience(w http.ResponseWriter, r *http.Request, c adminContext, targets []broadcastTarget, mode string) {
	result, err := a.resolveBroadcastAudience(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, targets, mode)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung penerima.")
		return
	}
	_, decision, err := a.access.Decide(r.Context(), c.tenant.ID, c.state.User.ID, c.tenant.Timezone, "broadcast.lihat_penerima")
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal memeriksa hak penerima.")
		return
	}
	if decision.Allowed {
		limit := 10
		if len(result.Recipients) < limit {
			limit = len(result.Recipients)
		}
		result.Samples = result.Recipients[:limit]
	}
	writeAPIData(w, 200, result, nil)
}
