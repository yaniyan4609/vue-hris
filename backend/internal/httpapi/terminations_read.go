package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type exitItem struct {
	Code      string    `json:"code"`
	Done      bool      `json:"is_done"`
	Note      *string   `json:"note"`
	UpdatedBy *string   `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

type termination struct {
	ID                 string     `json:"id"`
	EmployeeID         string     `json:"employee_id"`
	Type               string     `json:"termination_type"`
	NoticeDate         *string    `json:"notice_date"`
	EffectiveDate      string     `json:"effective_date"`
	Reason             string     `json:"reason"`
	Status             string     `json:"status"`
	RequestID          *string    `json:"request_id"`
	SourceAssignmentID *string    `json:"source_assignment_id"`
	Version            uint64     `json:"version"`
	ApplyError         *string    `json:"apply_error"`
	ProcessedAt        *time.Time `json:"processed_at"`
	CreatedBy          *string    `json:"created_by"`
	CreatedAt          time.Time  `json:"created_at"`
	Checklist          []exitItem `json:"checklist,omitempty"`
	Approval           any        `json:"approval,omitempty"`
}

const terminationColumns = `x.id,x.employee_id,x.termination_type,DATE_FORMAT(x.notice_date,'%Y-%m-%d'),DATE_FORMAT(x.effective_date,'%Y-%m-%d'),x.reason,x.status,x.request_id,x.source_assignment_id,x.version,x.apply_error,x.processed_at,x.created_by,x.created_at`
const terminationJoin = ` FROM terminations x JOIN employees e ON e.tenant_id=x.tenant_id AND e.id=x.employee_id` + employeeScopeJoin

func scanTermination(row interface{ Scan(...any) error }) (termination, error) {
	var item termination
	var notice, reason, requestID, sourceID, applyError, creator sql.NullString
	var processed sql.NullTime
	err := row.Scan(&item.ID, &item.EmployeeID, &item.Type, &notice, &item.EffectiveDate, &reason, &item.Status, &requestID, &sourceID, &item.Version, &applyError, &processed, &creator, &item.CreatedAt)
	item.Reason = reason.String
	if notice.Valid {
		item.NoticeDate = &notice.String
	}
	if requestID.Valid {
		item.RequestID = &requestID.String
	}
	if sourceID.Valid {
		item.SourceAssignmentID = &sourceID.String
	}
	if applyError.Valid {
		item.ApplyError = &applyError.String
	}
	if processed.Valid {
		item.ProcessedAt = &processed.Time
	}
	if creator.Valid {
		item.CreatedBy = &creator.String
	}
	return item, err
}

func validTerminationType(value string) bool {
	return value == "resign" || value == "dismissal" || value == "retirement"
}
func validTerminationStatus(value string) bool {
	return value == "draft" || value == "in_review" || value == "returned" || value == "approved" || value == "rejected" || value == "cancelled" || value == "processed"
}

func (a adminAPI) listTerminations(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "terminasi.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	where := `x.tenant_id=? AND ` + scope
	args := append([]any{c.tenant.ID}, scopeArgs...)
	if value := r.URL.Query().Get("employee_id"); value != "" {
		if !fileIDPattern.MatchString(value) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter karyawan tidak valid.")
			return
		}
		where += ` AND x.employee_id=?`
		args = append(args, value)
	}
	if value := r.URL.Query().Get("termination_type"); value != "" {
		if !validTerminationType(value) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jenis tidak valid.")
			return
		}
		where += ` AND x.termination_type=?`
		args = append(args, value)
	}
	if value := r.URL.Query().Get("status"); value != "" {
		if !validTerminationStatus(value) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Status tidak valid.")
			return
		}
		where += ` AND x.status=?`
		args = append(args, value)
	}
	for _, f := range []struct{ name, op string }{{"effective_from", ">="}, {"effective_to", "<="}} {
		if value := r.URL.Query().Get(f.name); value != "" {
			if !validChangeDate(value) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal filter tidak valid.")
				return
			}
			where += ` AND x.effective_date` + f.op + `?`
			args = append(args, value)
		}
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		if len([]rune(q)) > 100 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Pencarian terlalu panjang.")
			return
		}
		where += ` AND e.display_name LIKE ?`
		args = append(args, "%"+q+"%")
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+terminationJoin+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung terminasi.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+terminationColumns+terminationJoin+` WHERE `+where+` ORDER BY x.created_at DESC,x.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca terminasi.")
		return
	}
	defer rows.Close()
	items := []termination{}
	for rows.Next() {
		item, e := scanTermination(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca terminasi.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca terminasi.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) getTermination(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "terminasi.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Terminasi tidak ditemukan.")
		return
	}
	scope, args := access.EmployeeWhere(c.subject, c.effective)
	item, err := scanTermination(a.db.QueryRowContext(r.Context(), `SELECT `+terminationColumns+terminationJoin+` WHERE x.tenant_id=? AND x.id=? AND `+scope, append([]any{c.tenant.ID, id}, args...)...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Terminasi tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca terminasi.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT item_code,is_done,note,updated_by,updated_at FROM termination_checklist WHERE tenant_id=? AND termination_id=? ORDER BY item_code`, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca checklist.")
		return
	}
	defer rows.Close()
	item.Checklist = []exitItem{}
	for rows.Next() {
		var e exitItem
		var note, actor sql.NullString
		if err = rows.Scan(&e.Code, &e.Done, &note, &actor, &e.UpdatedAt); err != nil {
			break
		}
		if note.Valid {
			e.Note = &note.String
		}
		if actor.Valid {
			e.UpdatedBy = &actor.String
		}
		item.Checklist = append(item.Checklist, e)
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca checklist.")
		return
	}
	if item.RequestID != nil {
		request, e := scanRequest(a.db.QueryRowContext(r.Context(), `SELECT `+requestColumns+requestJoin+` WHERE q.tenant_id=? AND q.id=?`, c.tenant.ID, *item.RequestID))
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca approval.")
			return
		}
		item.Approval = request.Approval
	}
	writeAPIData(w, 200, item, nil)
}
