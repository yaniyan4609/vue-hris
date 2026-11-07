package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"digaji.id/hris/internal/access"
	"github.com/go-chi/chi/v5"
)

type contract struct {
	ID           string    `json:"id"`
	EmployeeID   string    `json:"employee_id"`
	ContractType string    `json:"contract_type"`
	StartDate    string    `json:"start_date"`
	EndDate      *string   `json:"end_date"`
	Status       string    `json:"status"`
	FileID       *string   `json:"file_id"`
	CreatedAt    time.Time `json:"created_at"`
}

const contractColumns = `c.id,c.employee_id,c.contract_type,c.start_date,c.end_date,c.status,c.file_id,c.created_at`
const contractJoin = ` FROM contracts c JOIN employees e ON e.tenant_id=c.tenant_id AND e.id=c.employee_id JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=TRUE`

func scanContract(row interface{ Scan(...any) error }) (contract, error) {
	var item contract
	var start time.Time
	var end sql.NullTime
	var file sql.NullString
	err := row.Scan(&item.ID, &item.EmployeeID, &item.ContractType, &start, &end, &item.Status, &file, &item.CreatedAt)
	if err != nil {
		return item, err
	}
	item.StartDate = start.Format("2006-01-02")
	if end.Valid {
		value := end.Time.Format("2006-01-02")
		item.EndDate = &value
	}
	if file.Valid {
		item.FileID = &file.String
	}
	return item, nil
}

func (a adminAPI) mountContracts(r chi.Router, config Options) {
	r.Get("/contracts", a.listContracts)
	r.With(checkOrigin(config)).Post("/contracts", a.createContract)
	r.Get("/contracts/expiring", a.listExpiringContracts)
	r.Get("/contracts/{id}", a.getContract)
	r.With(checkOrigin(config)).Patch("/contracts/{id}", a.updateContract)
	r.With(checkOrigin(config)).Delete("/contracts/{id}", a.deleteContract)
	r.Get("/employees/{id}/contracts", a.listEmployeeContracts)
}

func (a adminAPI) contractPage(w http.ResponseWriter, r *http.Request, c adminContext, extra string, extraArgs []any) {
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	filter, scopeArgs := access.EmployeeWhere(c.subject, c.effective)
	where := "c.tenant_id=? AND " + filter + extra
	args := append([]any{c.tenant.ID}, scopeArgs...)
	args = append(args, extraArgs...)
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*)`+contractJoin+` WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontrak.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+contractColumns+contractJoin+` WHERE `+where+` ORDER BY c.end_date IS NULL,c.end_date,c.id LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontrak.")
		return
	}
	defer rows.Close()
	items := []contract{}
	for rows.Next() {
		item, err := scanContract(rows)
		if err != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontrak.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontrak.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) listContracts(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kontrak.lihat")
	if !ok {
		return
	}
	extra := ""
	args := []any{}
	for _, entry := range []struct{ param, column string }{{"employee_id", "c.employee_id"}, {"contract_type", "c.contract_type"}, {"status", "c.status"}} {
		if value := r.URL.Query().Get(entry.param); value != "" {
			if len(value) > 32 || (entry.param == "employee_id" && len(value) != 26) {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter kontrak tidak valid.")
				return
			}
			extra += " AND " + entry.column + "=?"
			args = append(args, value)
		}
	}
	for _, entry := range []struct{ param, op string }{{"end_from", ">="}, {"end_to", "<="}} {
		if value := r.URL.Query().Get(entry.param); value != "" {
			date, err := time.Parse("2006-01-02", value)
			if err != nil || date.Format("2006-01-02") != value {
				writeAPIError(w, r, 400, "VALIDATION_FAILED", "Tanggal filter tidak valid.")
				return
			}
			extra += " AND c.end_date" + entry.op + "?"
			args = append(args, value)
		}
	}
	a.contractPage(w, r, c, extra, args)
}

func (a adminAPI) listExpiringContracts(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kontrak.lihat")
	if !ok {
		return
	}
	days := 30
	if value := r.URL.Query().Get("days"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 || parsed > 365 {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Jumlah hari harus 0 sampai 365.")
			return
		}
		days = parsed
	}
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	today := time.Now().In(zone)
	from := today.Format("2006-01-02")
	to := today.AddDate(0, 0, days).Format("2006-01-02")
	a.contractPage(w, r, c, ` AND c.end_date>=? AND c.end_date<=? AND c.status='active'`, []any{from, to})
}

func (a adminAPI) getContract(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kontrak.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontrak tidak ditemukan dalam scope.")
		return
	}
	filter, args := access.EmployeeWhere(c.subject, c.effective)
	params := append([]any{c.tenant.ID, id}, args...)
	item, err := scanContract(a.db.QueryRowContext(r.Context(), `SELECT `+contractColumns+contractJoin+` WHERE c.tenant_id=? AND c.id=? AND `+filter, params...))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Kontrak tidak ditemukan dalam scope.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kontrak.")
		return
	}
	writeAPIData(w, 200, item, nil)
}

func (a adminAPI) listEmployeeContracts(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "kontrak.lihat")
	if !ok {
		return
	}
	employeeID, ok := a.scopedEmployee(w, r, c)
	if !ok {
		return
	}
	a.contractPage(w, r, c, " AND c.employee_id=?", []any{employeeID})
}
