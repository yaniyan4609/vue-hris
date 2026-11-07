package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type approvalState struct {
	ID           string `json:"id"`
	FlowID       string `json:"flow_id"`
	CurrentLevel int    `json:"current_level"`
	Status       string `json:"status"`
	Version      uint64 `json:"version"`
	AttemptNo    uint64 `json:"attempt_no"`
}
type requestRevision struct {
	ID          string          `json:"id"`
	AttemptNo   uint64          `json:"attempt_no"`
	Correction  json.RawMessage `json:"correction"`
	SubmittedBy string          `json:"submitted_by"`
	SubmittedAt time.Time       `json:"submitted_at"`
}
type requestView struct {
	ID                string            `json:"id"`
	RequestType       string            `json:"request_type"`
	SubjectEmployeeID *string           `json:"subject_employee_id"`
	RequestedBy       string            `json:"requested_by"`
	RefTable          *string           `json:"ref_table"`
	RefID             *string           `json:"ref_id"`
	Status            string            `json:"status"`
	AttemptNo         uint64            `json:"attempt_no"`
	Version           uint64            `json:"version"`
	LatestCorrection  json.RawMessage   `json:"latest_correction,omitempty"`
	Revisions         []requestRevision `json:"revisions,omitempty"`
	SubmittedAt       *time.Time        `json:"submitted_at"`
	FinalAt           *time.Time        `json:"final_at"`
	CreatedAt         time.Time         `json:"created_at"`
	Approval          *approvalState    `json:"approval"`
}

const requestColumns = `q.id,q.request_type,q.subject_employee_id,q.requested_by,q.ref_table,q.ref_id,q.status,q.attempt_no,q.version,q.submitted_at,q.final_at,q.created_at,i.id,i.flow_id,i.current_level,i.status,i.version,i.attempt_no`
const requestJoin = ` FROM requests q LEFT JOIN approval_instances i ON i.tenant_id=q.tenant_id AND i.request_id=q.id LEFT JOIN employees e ON e.tenant_id=q.tenant_id AND e.id=q.subject_employee_id LEFT JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`

func scanRequest(row interface{ Scan(...any) error }) (requestView, error) {
	var item requestView
	var subject, refTable, refID sql.NullString
	var submitted, final sql.NullTime
	var instanceID, flowID, instanceStatus sql.NullString
	var level, version, attempt sql.NullInt64
	err := row.Scan(&item.ID, &item.RequestType, &subject, &item.RequestedBy, &refTable, &refID, &item.Status, &item.AttemptNo, &item.Version, &submitted, &final, &item.CreatedAt, &instanceID, &flowID, &level, &instanceStatus, &version, &attempt)
	if err != nil {
		return item, err
	}
	if subject.Valid {
		item.SubjectEmployeeID = &subject.String
	}
	if refTable.Valid {
		item.RefTable = &refTable.String
	}
	if refID.Valid {
		item.RefID = &refID.String
	}
	if submitted.Valid {
		item.SubmittedAt = &submitted.Time
	}
	if final.Valid {
		item.FinalAt = &final.Time
	}
	if instanceID.Valid {
		item.Approval = &approvalState{ID: instanceID.String, FlowID: flowID.String, CurrentLevel: int(level.Int64), Status: instanceStatus.String, Version: uint64(version.Int64), AttemptNo: uint64(attempt.Int64)}
	}
	return item, nil
}

func (a adminAPI) attachRequestRevisions(r *http.Request, tenantID string, item *requestView) error {
	rows, err := a.db.QueryContext(r.Context(), `SELECT id,attempt_no,correction_json,submitted_by,submitted_at FROM request_revisions WHERE tenant_id=? AND request_id=? ORDER BY attempt_no,id`, tenantID, item.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	item.Revisions = []requestRevision{}
	for rows.Next() {
		var revision requestRevision
		var raw []byte
		if err := rows.Scan(&revision.ID, &revision.AttemptNo, &raw, &revision.SubmittedBy, &revision.SubmittedAt); err != nil {
			return err
		}
		revision.Correction = json.RawMessage(raw)
		item.LatestCorrection = revision.Correction
		item.Revisions = append(item.Revisions, revision)
	}
	return rows.Err()
}

func approvalScope(c adminContext) (string, []any) {
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	allTenant := false
	for _, rule := range c.effective.Scopes {
		if rule.AllTenant {
			allTenant = true
			break
		}
	}
	if allTenant {
		return "1=1", nil
	}
	return "q.subject_employee_id IS NOT NULL AND " + filter, args
}

func (a adminAPI) mountApprovalReads(r chi.Router) {
	r.Get("/approvals/inbox", a.approvalInbox)
	r.Get("/approvals/summary", a.approvalSummary)
	r.Get("/approvals", a.listApprovals)
	r.Get("/approvals/{requestId}", a.getApproval)
}

func (a adminAPI) listRequestPage(w http.ResponseWriter, r *http.Request, where string, args []any, join string) {
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(DISTINCT q.id)`+join+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT DISTINCT `+requestColumns+join+` WHERE `+where+` ORDER BY q.created_at DESC,q.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan.")
		return
	}
	defer rows.Close()
	items := []requestView{}
	for rows.Next() {
		item, err := scanRequest(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func requestFilters(w http.ResponseWriter, r *http.Request) (string, []any, bool) {
	where := ""
	args := []any{}
	for _, entry := range []struct{ param, column string }{{"request_type", "q.request_type"}, {"status", "q.status"}} {
		if value := r.URL.Query().Get(entry.param); value != "" {
			if len(value) > 48 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter pengajuan tidak valid.")
				return "", nil, false
			}
			where += " AND " + entry.column + "=?"
			args = append(args, value)
		}
	}
	for _, entry := range []struct{ param, operator string }{{"from", ">="}, {"to", "<"}} {
		if value := r.URL.Query().Get(entry.param); value != "" {
			date, err := time.Parse("2006-01-02", value)
			if err != nil || date.Format("2006-01-02") != value {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal filter tidak valid.")
				return "", nil, false
			}
			if entry.param == "to" {
				date = date.AddDate(0, 0, 1)
			}
			where += " AND q.created_at" + entry.operator + "?"
			args = append(args, date)
		}
	}
	return where, args, true
}

func (a adminAPI) listApprovals(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "persetujuan.lihat")
	if !ok {
		return
	}
	scope, scopeArgs := approvalScope(c)
	filters, filterArgs, ok := requestFilters(w, r)
	if !ok {
		return
	}
	args := append([]any{c.tenant.ID}, scopeArgs...)
	args = append(args, filterArgs...)
	a.listRequestPage(w, r, "q.tenant_id=? AND ("+scope+")"+filters, args, requestJoin)
}

func (a adminAPI) approvalInbox(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "persetujuan.lihat")
	if !ok {
		return
	}
	scope, scopeArgs := approvalScope(c)
	filters, filterArgs, ok := requestFilters(w, r)
	if !ok {
		return
	}
	status := r.URL.Query().Get("task_status")
	if status == "" {
		status = "pending"
	}
	if status != "pending" && status != "approved" && status != "rejected" && status != "returned" && status != "skipped" && status != "cancelled" {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status tugas tidak valid.")
		return
	}
	join := requestJoin + ` JOIN approval_tasks t ON t.tenant_id=i.tenant_id AND t.instance_id=i.id`
	args := append([]any{c.tenant.ID, c.state.User.ID, status}, scopeArgs...)
	args = append(args, filterArgs...)
	a.listRequestPage(w, r, "q.tenant_id=? AND t.assignee_user_id=? AND t.status=? AND ("+scope+")"+filters, args, join)
}

func (a adminAPI) approvalSummary(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "persetujuan.lihat")
	if !ok {
		return
	}
	scope, scopeArgs := approvalScope(c)
	args := append([]any{c.tenant.ID, c.state.User.ID}, scopeArgs...)
	rows, err := a.db.QueryContext(r.Context(), `SELECT q.request_type,COUNT(*)`+requestJoin+` JOIN approval_tasks t ON t.tenant_id=i.tenant_id AND t.instance_id=i.id WHERE q.tenant_id=? AND t.assignee_user_id=? AND t.status='pending' AND (`+scope+`) GROUP BY q.request_type ORDER BY q.request_type`, args...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ringkasan approval.")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	total := 0
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ringkasan approval.")
			return
		}
		items = append(items, map[string]any{"request_type": kind, "pending": count})
		total += count
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca ringkasan approval.")
		return
	}
	writeAPIData(w, 200, map[string]any{"total": total, "by_type": items}, nil)
}

func (a adminAPI) getApproval(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "persetujuan.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "requestId")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan dalam scope.")
		return
	}
	scope, scopeArgs := approvalScope(c)
	args := append([]any{c.tenant.ID, id}, scopeArgs...)
	item, err := scanRequest(a.db.QueryRowContext(r.Context(), `SELECT `+requestColumns+requestJoin+` WHERE q.tenant_id=? AND q.id=? AND (`+scope+`)`, args...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan.")
		return
	}
	if err := a.attachRequestRevisions(r, c.tenant.ID, &item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca revisi pengajuan.")
		return
	}
	var tasks []map[string]any = []map[string]any{}
	var actions []map[string]any = []map[string]any{}
	if item.Approval != nil {
		rows, err := a.db.QueryContext(r.Context(), `SELECT id,attempt_no,level_no,assignee_user_id,status,created_at,decided_at FROM approval_tasks WHERE tenant_id=? AND instance_id=? ORDER BY attempt_no,level_no,created_at,id`, c.tenant.ID, item.Approval.ID)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tahapan approval.")
			return
		}
		for rows.Next() {
			var id, user, status string
			var level, attemptNo int
			var created time.Time
			var decided sql.NullTime
			if err := rows.Scan(&id, &attemptNo, &level, &user, &status, &created, &decided); err != nil {
				rows.Close()
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tahapan approval.")
				return
			}
			var decidedAt any
			if decided.Valid {
				decidedAt = decided.Time
			}
			tasks = append(tasks, map[string]any{"id": id, "attempt_no": attemptNo, "level_no": level, "assignee_user_id": user, "status": status, "created_at": created, "decided_at": decidedAt})
		}
		if rows.Err() != nil {
			rows.Close()
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tahapan approval.")
			return
		}
		rows.Close()
		rows, err = a.db.QueryContext(r.Context(), `SELECT id,task_id,level_no,actor_user_id,action,reason,created_at FROM approval_actions WHERE tenant_id=? AND instance_id=? ORDER BY created_at,id`, c.tenant.ID, item.Approval.ID)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aksi approval.")
			return
		}
		for rows.Next() {
			var id, taskID, actor, action string
			var level int
			var reason sql.NullString
			var created time.Time
			if err := rows.Scan(&id, &taskID, &level, &actor, &action, &reason, &created); err != nil {
				rows.Close()
				writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aksi approval.")
				return
			}
			var reasonValue any
			if reason.Valid {
				reasonValue = reason.String
			}
			actions = append(actions, map[string]any{"id": id, "task_id": taskID, "level_no": level, "actor_user_id": actor, "action": action, "reason": reasonValue, "created_at": created})
		}
		if rows.Err() != nil {
			rows.Close()
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca aksi approval.")
			return
		}
		rows.Close()
	}
	writeAPIData(w, 200, map[string]any{"request": item, "tasks": tasks, "actions": actions}, nil)
}

func (a adminAPI) listEssRequests(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	filters, filterArgs, ok := requestFilters(w, r)
	if !ok {
		return
	}
	args := append([]any{c.state.SelectedTenantID, c.state.User.ID}, filterArgs...)
	a.listRequestPage(w, r, "q.tenant_id=? AND q.requested_by=?"+filters, args, requestJoin)
}

func (a adminAPI) getEssRequest(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "requestId")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan.")
		return
	}
	item, err := scanRequest(a.db.QueryRowContext(r.Context(), `SELECT `+requestColumns+requestJoin+` WHERE q.tenant_id=? AND q.requested_by=? AND q.id=?`, c.state.SelectedTenantID, c.state.User.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Pengajuan tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca pengajuan.")
		return
	}
	if err := a.attachRequestRevisions(r, c.state.SelectedTenantID, &item); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca revisi pengajuan.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
