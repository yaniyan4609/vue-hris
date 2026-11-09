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

type disciplinaryEvent struct {
	Action      string    `json:"action"`
	ActorUserID string    `json:"actor_user_id"`
	At          time.Time `json:"at"`
}

type disciplinaryAction struct {
	ID                 string              `json:"id"`
	EmployeeID         string              `json:"employee_id"`
	Severity           string              `json:"severity"`
	IssuedDate         string              `json:"issued_date"`
	ValidUntil         *string             `json:"valid_until"`
	Description        string              `json:"description"`
	FileID             *string             `json:"file_id"`
	Status             string              `json:"status"`
	Version            uint64              `json:"version"`
	CreatedBy          *string             `json:"created_by"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
	CancelledBy        *string             `json:"cancelled_by"`
	CancelledAt        *time.Time          `json:"cancelled_at"`
	CancellationReason *string             `json:"cancellation_reason"`
	History            []disciplinaryEvent `json:"history,omitempty"`
}

const disciplinaryColumns = `d.id,d.employee_id,d.severity,DATE_FORMAT(d.issued_date,'%Y-%m-%d'),DATE_FORMAT(d.valid_until,'%Y-%m-%d'),d.description,d.file_id,d.status,d.version,d.created_by,d.created_at,d.updated_at,d.cancelled_by,d.cancelled_at,d.cancellation_reason`
const disciplinaryJoin = ` FROM disciplinary_actions d JOIN employees e ON e.tenant_id=d.tenant_id AND e.id=d.employee_id LEFT JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1`

func scanDisciplinaryAction(row interface{ Scan(...any) error }) (disciplinaryAction, error) {
	var item disciplinaryAction
	var until, fileID, creator, canceller, cancelReason sql.NullString
	var cancelled sql.NullTime
	err := row.Scan(&item.ID, &item.EmployeeID, &item.Severity, &item.IssuedDate, &until, &item.Description, &fileID, &item.Status, &item.Version, &creator, &item.CreatedAt, &item.UpdatedAt, &canceller, &cancelled, &cancelReason)
	if until.Valid {
		item.ValidUntil = &until.String
	}
	if fileID.Valid {
		item.FileID = &fileID.String
	}
	if creator.Valid {
		item.CreatedBy = &creator.String
	}
	if canceller.Valid {
		item.CancelledBy = &canceller.String
	}
	if cancelled.Valid {
		item.CancelledAt = &cancelled.Time
	}
	if cancelReason.Valid {
		item.CancellationReason = &cancelReason.String
	}
	return item, err
}

func (a adminAPI) listDisciplinaryActions(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "disiplin.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	where := `d.tenant_id=? AND ` + scope
	args := append([]any{c.tenant.ID}, scopeArgs...)
	if id := r.URL.Query().Get("employee_id"); id != "" {
		if !fileIDPattern.MatchString(id) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter karyawan tidak valid.")
			return
		}
		where += ` AND d.employee_id=?`
		args = append(args, id)
	}
	if severity := strings.ToUpper(r.URL.Query().Get("severity")); severity != "" {
		if !validDisciplinarySeverity(severity) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter tingkat tidak valid.")
			return
		}
		where += ` AND d.severity=?`
		args = append(args, severity)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		if status != "active" && status != "cancelled" {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter status tidak valid.")
			return
		}
		where += ` AND d.status=?`
		args = append(args, status)
	}
	for _, filter := range []struct{ name, op string }{{"issued_from", ">="}, {"issued_to", "<="}} {
		if value := r.URL.Query().Get(filter.name); value != "" {
			if !validChangeDate(value) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal filter tidak valid.")
				return
			}
			where += ` AND d.issued_date` + filter.op + `?`
			args = append(args, value)
		}
	}
	if value := r.URL.Query().Get("active_on"); value != "" {
		if !validChangeDate(value) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal aktif tidak valid.")
			return
		}
		where += ` AND d.status='active' AND d.issued_date<=? AND (d.valid_until IS NULL OR d.valid_until>=?)`
		args = append(args, value, value)
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
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+disciplinaryJoin+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menghitung tindakan disiplin.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+disciplinaryColumns+disciplinaryJoin+` WHERE `+where+` ORDER BY d.issued_date DESC,d.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tindakan disiplin.")
		return
	}
	defer rows.Close()
	items := []disciplinaryAction{}
	for rows.Next() {
		item, e := scanDisciplinaryAction(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tindakan disiplin.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tindakan disiplin.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) getDisciplinaryAction(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "disiplin.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Tindakan disiplin tidak ditemukan.")
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, id}, scopeArgs...)
	item, err := scanDisciplinaryAction(a.db.QueryRowContext(r.Context(), `SELECT `+disciplinaryColumns+disciplinaryJoin+` WHERE d.tenant_id=? AND d.id=? AND `+scope, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Tindakan disiplin tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca tindakan disiplin.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT action,actor_user_id,created_at FROM audit_logs WHERE tenant_id=? AND entity_type='disciplinary_actions' AND entity_id=? ORDER BY created_at,id`, c.tenant.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat tindakan disiplin.")
		return
	}
	defer rows.Close()
	item.History = []disciplinaryEvent{}
	for rows.Next() {
		var event disciplinaryEvent
		if err = rows.Scan(&event.Action, &event.ActorUserID, &event.At); err != nil {
			break
		}
		item.History = append(item.History, event)
	}
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca riwayat tindakan disiplin.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
