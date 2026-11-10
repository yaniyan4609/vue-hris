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

type payrollRunView struct {
	ID               string     `json:"id"`
	PeriodID         string     `json:"period_id"`
	RunNo            int        `json:"run_no"`
	Status           string     `json:"status"`
	CalculationState string     `json:"calculation_state"`
	TotalGross       *string    `json:"total_gross"`
	TotalNet         *string    `json:"total_net"`
	CreatedAt        time.Time  `json:"created_at"`
	CalculatedAt     *time.Time `json:"calculated_at"`
	FinalizedAt      *time.Time `json:"finalized_at"`
}

const payrollRunColumns = `id,period_id,run_no,status,calculation_state,CAST(total_gross AS CHAR),CAST(total_net AS CHAR),created_at,calculated_at,finalized_at`

func scanPayrollRun(row interface{ Scan(...any) error }) (payrollRunView, error) {
	var item payrollRunView
	var gross, net string
	var calculated, finalized sql.NullTime
	err := row.Scan(&item.ID, &item.PeriodID, &item.RunNo, &item.Status, &item.CalculationState, &gross, &net, &item.CreatedAt, &calculated, &finalized)
	if err != nil {
		return item, err
	}
	if item.CalculationState == "completed" {
		item.TotalGross = &gross
		item.TotalNet = &net
	}
	if calculated.Valid {
		item.CalculatedAt = &calculated.Time
	}
	if finalized.Valid {
		item.FinalizedAt = &finalized.Time
	}
	return item, nil
}

func (a adminAPI) createPayrollDraftRun(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.proses")
	if !ok {
		return
	}
	periodID := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(periodID) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
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
	var body struct{}
	if !readJSON(w, r, &body) {
		return
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat draft payroll.")
		return
	}
	defer tx.Rollback()
	var periodStatus string
	err = tx.QueryRowContext(r.Context(), `SELECT status FROM payroll_periods WHERE tenant_id=? AND id=? FOR UPDATE`, c.tenant.ID, periodID).Scan(&periodStatus)
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Periode payroll tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca periode payroll.")
		return
	}
	item, err := scanPayrollRun(tx.QueryRowContext(r.Context(), `SELECT `+payrollRunColumns+` FROM payroll_runs WHERE tenant_id=? AND period_id=? AND idempotency_key=?`, c.tenant.ID, periodID, key))
	if err == nil {
		w.Header().Set("Idempotent-Replay", "true")
		writeAPIData(w, 200, item, nil)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca draft payroll.")
		return
	}
	if periodStatus != "open" {
		writeAPIError(w, r, 409, "PERIOD_NOT_OPEN", "Periode payroll tidak terbuka.")
		return
	}
	var runNo int
	if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(run_no),0)+1 FROM payroll_runs WHERE tenant_id=? AND period_id=?`, c.tenant.ID, periodID).Scan(&runNo); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menentukan nomor run.")
		return
	}
	if runNo > 65535 {
		writeAPIError(w, r, 409, "RUN_LIMIT", "Batas nomor run tercapai.")
		return
	}
	id, err := ids.New()
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat ID run.")
		return
	}
	_, err = tx.ExecContext(r.Context(), `INSERT INTO payroll_runs (tenant_id,id,period_id,run_no,status,calculation_state,idempotency_key,created_by) VALUES (?,?,?,?,'draft','blocked',?,?)`, c.tenant.ID, id, periodID, runNo, key, c.state.User.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat draft payroll.")
		return
	}
	item, err = scanPayrollRun(tx.QueryRowContext(r.Context(), `SELECT `+payrollRunColumns+` FROM payroll_runs WHERE tenant_id=? AND id=?`, c.tenant.ID, id))
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca draft payroll.")
		return
	}
	if err = audit(r.Context(), tx, c.tenant.ID, c.state.User.ID, "create", "payroll_runs", id, nil, map[string]any{"period_id": periodID, "run_no": runNo, "calculation_state": "blocked"}); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal mencatat draft payroll.")
		return
	}
	if err = tx.Commit(); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membuat draft payroll.")
		return
	}
	writeAPIData(w, 202, item, nil)
}

func (a adminAPI) listPayrollRuns(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	page, ok := canonicalLookupPage(w, r)
	if !ok {
		return
	}
	where := "tenant_id=?"
	args := []any{c.tenant.ID}
	if period := r.URL.Query().Get("period_id"); period != "" {
		if !fileIDPattern.MatchString(period) {
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter periode tidak valid.")
			return
		}
		where += " AND period_id=?"
		args = append(args, period)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		switch status {
		case "draft", "calculated", "in_review", "finalized", "cancelled":
		default:
			writeAPIError(w, r, 400, "VALIDATION_FAILED", "Filter status tidak valid.")
			return
		}
		where += " AND status=?"
		args = append(args, status)
	}
	var total int
	if err := a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM payroll_runs WHERE `+where, args...).Scan(&total); err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca run payroll.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+payrollRunColumns+` FROM payroll_runs WHERE `+where+` ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`, append(args, page.PageSize, page.Offset)...)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca run payroll.")
		return
	}
	defer rows.Close()
	items := []payrollRunView{}
	for rows.Next() {
		item, e := scanPayrollRun(rows)
		if e != nil {
			writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca run payroll.")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca run payroll.")
		return
	}
	writeAPIData(w, 200, items, map[string]int{"page": page.Page, "per_page": page.PageSize, "total": total})
}

func (a adminAPI) getPayrollRun(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "penggajian.lihat")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Run payroll tidak ditemukan.")
		return
	}
	item, err := scanPayrollRun(a.db.QueryRowContext(r.Context(), `SELECT `+payrollRunColumns+` FROM payroll_runs WHERE tenant_id=? AND id=?`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Run payroll tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca run payroll.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
