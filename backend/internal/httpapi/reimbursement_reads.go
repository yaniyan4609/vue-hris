package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type reimbursementView struct {
	ID          string     `json:"id"`
	RequestID   *string    `json:"request_id"`
	EmployeeID  string     `json:"employee_id"`
	CategoryID  string     `json:"category_id"`
	Category    string     `json:"category_name"`
	TrxDate     string     `json:"trx_date"`
	Amount      string     `json:"amount"`
	Description *string    `json:"description"`
	Status      string     `json:"status"`
	PaymentRef  *string    `json:"payment_ref"`
	PaidAt      *time.Time `json:"paid_at"`
}

type reimbursementReceiptView struct {
	FileID       string    `json:"file_id"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	SizeBytes    uint64    `json:"size_bytes"`
	ScanStatus   string    `json:"scan_status"`
	CreatedAt    time.Time `json:"created_at"`
}

const reimbursementColumns = `r.id,r.request_id,r.employee_id,r.category_id,c.name,r.trx_date,CAST(r.amount AS CHAR),r.description,r.status,r.payment_ref,r.paid_at`
const reimbursementEssFrom = ` FROM reimbursements r JOIN reimbursement_categories c ON c.tenant_id=r.tenant_id AND c.id=r.category_id`
const reimbursementAdminFrom = reimbursementEssFrom + ` JOIN employees e ON e.tenant_id=r.tenant_id AND e.id=r.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`

func scanReimbursement(row interface{ Scan(...any) error }) (reimbursementView, error) {
	var item reimbursementView
	var requestID, description, paymentRef sql.NullString
	var paidAt sql.NullTime
	var date time.Time
	err := row.Scan(&item.ID, &requestID, &item.EmployeeID, &item.CategoryID, &item.Category, &date, &item.Amount, &description, &item.Status, &paymentRef, &paidAt)
	if err != nil {
		return item, err
	}
	item.TrxDate = date.Format("2006-01-02")
	if requestID.Valid {
		item.RequestID = &requestID.String
	}
	if description.Valid {
		item.Description = &description.String
	}
	if paymentRef.Valid {
		item.PaymentRef = &paymentRef.String
	}
	if paidAt.Valid {
		item.PaidAt = &paidAt.Time
	}
	return item, nil
}

func reimbursementFilters(w http.ResponseWriter, r *http.Request, admin bool) (string, []any, bool) {
	where := ""
	args := []any{}
	for _, entry := range []struct{ key, column string }{{"status", "r.status"}, {"category_id", "r.category_id"}} {
		value := r.URL.Query().Get(entry.key)
		if value == "" {
			continue
		}
		if (entry.key == "status" && len(value) > 24) || (entry.key == "category_id" && len(value) != 26) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter klaim tidak valid.")
			return "", nil, false
		}
		where += " AND " + entry.column + "=?"
		args = append(args, value)
	}
	if admin {
		if id := r.URL.Query().Get("employee_id"); id != "" {
			if len(id) != 26 {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "ID karyawan tidak valid.")
				return "", nil, false
			}
			where += " AND r.employee_id=?"
			args = append(args, id)
		}
	}
	for _, entry := range []struct{ key, op string }{{"from", ">="}, {"to", "<="}} {
		value := r.URL.Query().Get(entry.key)
		if value == "" {
			continue
		}
		date, err := time.Parse("2006-01-02", value)
		if err != nil || date.Format("2006-01-02") != value {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal filter tidak valid.")
			return "", nil, false
		}
		where += " AND r.trx_date" + entry.op + "?"
		args = append(args, value)
	}
	return where, args, true
}

func (a adminAPI) reimbursementPage(w http.ResponseWriter, r *http.Request, from, where string, args []any) {
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+from+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca klaim reimbursement.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+reimbursementColumns+from+` WHERE `+where+` ORDER BY r.trx_date DESC,r.id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca klaim reimbursement.")
		return
	}
	defer rows.Close()
	items := []reimbursementView{}
	for rows.Next() {
		item, err := scanReimbursement(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca klaim reimbursement.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca klaim reimbursement.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) reimbursementDetail(w http.ResponseWriter, r *http.Request, from, where string, args []any) {
	item, err := scanReimbursement(a.db.QueryRowContext(r.Context(), `SELECT `+reimbursementColumns+from+` WHERE `+where, args...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Klaim reimbursement tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca klaim reimbursement.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT f.id,f.original_name,f.mime_type,f.size_bytes,f.scan_status,f.created_at FROM reimbursement_receipts rr JOIN files f ON f.tenant_id=rr.tenant_id AND f.id=rr.file_id WHERE rr.tenant_id=? AND rr.reimbursement_id=? AND f.deleted_at IS NULL ORDER BY f.created_at,f.id`, args[0], item.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kuitansi.")
		return
	}
	defer rows.Close()
	receipts := []reimbursementReceiptView{}
	for rows.Next() {
		var receipt reimbursementReceiptView
		if err := rows.Scan(&receipt.FileID, &receipt.OriginalName, &receipt.MimeType, &receipt.SizeBytes, &receipt.ScanStatus, &receipt.CreatedAt); err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kuitansi.")
			return
		}
		receipts = append(receipts, receipt)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kuitansi.")
		return
	}
	writeAPIData(w, 200, map[string]any{"claim": item, "receipts": receipts}, nil)
}

func (a adminAPI) listEssReimbursements(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	filter, extra, ok := reimbursementFilters(w, r, false)
	if !ok {
		return
	}
	a.reimbursementPage(w, r, reimbursementEssFrom, "r.tenant_id=? AND r.employee_id=?"+filter, append([]any{c.state.SelectedTenantID, c.employeeID}, extra...))
}

func (a adminAPI) getEssReimbursement(w http.ResponseWriter, r *http.Request) {
	c, ok := a.essAttendanceContext(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Klaim reimbursement tidak ditemukan.")
		return
	}
	a.reimbursementDetail(w, r, reimbursementEssFrom, "r.tenant_id=? AND r.employee_id=? AND r.id=?", []any{c.state.SelectedTenantID, c.employeeID, id})
}

func (a adminAPI) listAdminReimbursements(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "reimbursement.lihat")
	if !ok {
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	filter, extra, ok := reimbursementFilters(w, r, true)
	if !ok {
		return
	}
	args := append([]any{c.tenant.ID}, scopeArgs...)
	a.reimbursementPage(w, r, reimbursementAdminFrom, "r.tenant_id=? AND "+scope+filter, append(args, extra...))
}

func (a adminAPI) getAdminReimbursement(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "reimbursement.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Klaim reimbursement tidak ditemukan.")
		return
	}
	scope, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	a.reimbursementDetail(w, r, reimbursementAdminFrom, "r.tenant_id=? AND r.id=? AND "+scope, append([]any{c.tenant.ID, id}, scopeArgs...))
}
