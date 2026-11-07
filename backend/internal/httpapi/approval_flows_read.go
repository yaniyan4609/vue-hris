package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type approvalFlowStep struct {
	ID                 string  `json:"id"`
	LevelNo            int     `json:"level_no"`
	ApproverKind       string  `json:"approver_kind"`
	ApproverRefID      *string `json:"approver_ref"`
	RequiredPermission *string `json:"required_permission"`
	SLAHours           *int    `json:"sla_hours"`
}
type approvalFlow struct {
	ID             string             `json:"id"`
	RequestType    string             `json:"request_type"`
	Name           string             `json:"name"`
	IsDefault      bool               `json:"is_default"`
	IsActive       bool               `json:"is_active"`
	Version        uint64             `json:"version"`
	ValidFrom      string             `json:"valid_from"`
	ValidTo        *string            `json:"valid_to"`
	FallbackRoleID *string            `json:"fallback_role_id"`
	CreatedAt      time.Time          `json:"created_at"`
	Steps          []approvalFlowStep `json:"steps"`
}

const flowColumns = `id,request_type,name,is_default,is_active,version,valid_from,valid_to,fallback_role_id,created_at`

func scanFlow(row interface{ Scan(...any) error }) (approvalFlow, error) {
	var item approvalFlow
	var from time.Time
	var to sql.NullTime
	var fallback sql.NullString
	err := row.Scan(&item.ID, &item.RequestType, &item.Name, &item.IsDefault, &item.IsActive, &item.Version, &from, &to, &fallback, &item.CreatedAt)
	if err != nil {
		return item, err
	}
	item.ValidFrom = from.Format("2006-01-02")
	if to.Valid {
		value := to.Time.Format("2006-01-02")
		item.ValidTo = &value
	}
	if fallback.Valid {
		item.FallbackRoleID = &fallback.String
	}
	return item, nil
}

func (a adminAPI) listApprovalFlows(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengaturan.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	where := "tenant_id=?"
	args := []any{c.tenant.ID}
	if kind := r.URL.Query().Get("request_type"); kind != "" {
		if len(kind) > 48 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jenis pengajuan tidak valid.")
			return
		}
		where += " AND request_type=?"
		args = append(args, kind)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM approval_flows WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE `+where+` ORDER BY request_type,is_default DESC,valid_from DESC,id LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	defer rows.Close()
	items := []approvalFlow{}
	for rows.Next() {
		item, err := scanFlow(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) getApprovalFlow(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "pengaturan.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Alur approval tidak ditemukan.")
		return
	}
	item, err := scanFlow(a.db.QueryRowContext(r.Context(), `SELECT `+flowColumns+` FROM approval_flows WHERE tenant_id=? AND id=?`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Alur approval tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca alur approval.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,level_no,approver_kind,approver_ref_id,required_permission,sla_hours FROM approval_flow_steps WHERE tenant_id=? AND flow_id=? ORDER BY level_no,id`, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tahap approval.")
		return
	}
	defer rows.Close()
	item.Steps = []approvalFlowStep{}
	for rows.Next() {
		var step approvalFlowStep
		var ref, permission sql.NullString
		var sla sql.NullInt64
		if err := rows.Scan(&step.ID, &step.LevelNo, &step.ApproverKind, &ref, &permission, &sla); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tahap approval.")
			return
		}
		step.ApproverKind = externalApproverKind(step.ApproverKind)
		if ref.Valid {
			step.ApproverRefID = &ref.String
		}
		if permission.Valid {
			step.RequiredPermission = &permission.String
		}
		if sla.Valid {
			value := int(sla.Int64)
			step.SLAHours = &value
		}
		item.Steps = append(item.Steps, step)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tahap approval.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
