package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"digaji.id/hris/internal/ids"
	"github.com/go-chi/chi/v5"
)

type leaveAccrualRun struct {
	ID             string     `json:"id"`
	PeriodYear     int        `json:"period_year"`
	AsOfDate       string     `json:"as_of_date"`
	Source         string     `json:"source"`
	Status         string     `json:"status"`
	ProcessedCount int        `json:"processed_count"`
	CreditedCount  int        `json:"credited_count"`
	CreatedAt      time.Time  `json:"created_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	LastError      *string    `json:"last_error"`
}

func scanLeaveAccrualRun(row interface{ Scan(...any) error }) (leaveAccrualRun, error) {
	var item leaveAccrualRun
	var day time.Time
	var finished sql.NullTime
	var last sql.NullString
	err := row.Scan(&item.ID, &item.PeriodYear, &day, &item.Source, &item.Status, &item.ProcessedCount, &item.CreditedCount, &item.CreatedAt, &finished, &last)
	if err != nil {
		return item, err
	}
	item.AsOfDate = day.Format("2006-01-02")
	if finished.Valid {
		item.FinishedAt = &finished.Time
	}
	if last.Valid {
		item.LastError = &last.String
	}
	return item, nil
}

const leaveAccrualRunColumns = `id,period_year,as_of_date,source,status,processed_count,credited_count,created_at,finished_at,last_error`

func enqueueLeaveAccrualRun(ctx context.Context, db *sql.DB, tenantID string, year int, asOfDate, source, actorID string) (leaveAccrualRun, error) {
	var item leaveAccrualRun
	id, err := ids.New()
	if err != nil {
		return item, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return item, err
	}
	defer tx.Rollback()
	var actor any
	if actorID != "" {
		actor = actorID
	}
	result, err := tx.ExecContext(ctx, `INSERT IGNORE INTO leave_accrual_runs (tenant_id,id,period_year,as_of_date,source,created_by) VALUES (?,?,?,?,?,?)`, tenantID, id, year, asOfDate, source, actor)
	if err != nil {
		return item, err
	}
	inserted, _ := result.RowsAffected()
	requeued := false
	if inserted == 0 {
		item, err = scanLeaveAccrualRun(tx.QueryRowContext(ctx, `SELECT `+leaveAccrualRunColumns+` FROM leave_accrual_runs WHERE tenant_id=? AND period_year=? AND as_of_date=? FOR UPDATE`, tenantID, year, asOfDate))
		if err != nil {
			return item, err
		}
		if item.Status == "completed" {
			jan1 := fmt.Sprintf("%04d-01-01", year)
			var pending int
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*)`+annualAccrualFrom, jan1, jan1, year, tenantID, asOfDate).Scan(&pending); err != nil {
				return item, err
			}
			if pending > 0 {
				if _, err = tx.ExecContext(ctx, `UPDATE leave_accrual_runs SET status='pending',finished_at=NULL WHERE tenant_id=? AND id=?`, tenantID, item.ID); err != nil {
					return item, err
				}
				item.Status = "pending"
				item.FinishedAt = nil
				requeued = true
			}
		}
		if item.Status == "failed" && source == "manual" {
			if _, err = tx.ExecContext(ctx, `UPDATE leave_accrual_runs SET status='pending',lease_until=NULL,last_error=NULL WHERE tenant_id=? AND id=?`, tenantID, item.ID); err != nil {
				return item, err
			}
			item.Status = "pending"
			item.LastError = nil
			requeued = true
		}
	} else {
		item, err = scanLeaveAccrualRun(tx.QueryRowContext(ctx, `SELECT `+leaveAccrualRunColumns+` FROM leave_accrual_runs WHERE tenant_id=? AND id=?`, tenantID, id))
		if err != nil {
			return item, err
		}
	}
	if actorID != "" && (inserted == 1 || requeued) {
		action := "enqueue"
		if requeued {
			action = "retry"
		}
		if err = audit(ctx, tx, tenantID, actorID, action, "leave_accrual_runs", item.ID, nil, map[string]any{"period_year": year, "as_of_date": asOfDate}); err != nil {
			return item, err
		}
	}
	if err = tx.Commit(); err != nil {
		return item, err
	}
	return item, nil
}

func (a adminAPI) startAdminLeaveAccrualRun(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.kelola")
	if !ok {
		return
	}
	var body struct {
		Year int `json:"year"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	zone, err := time.LoadLocation(c.tenant.Timezone)
	if err != nil {
		zone = time.UTC
	}
	now := time.Now().In(zone)
	if body.Year == 0 {
		body.Year = now.Year()
	}
	if body.Year != now.Year() {
		writeAPIError(w, r, 400, "VALIDATION_FAILED", "Akrual hanya dapat dijalankan untuk tahun berjalan dalam zona tenant.")
		return
	}
	jan1 := now.Format("2006") + "-01-01"
	var policies int
	err = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM leave_policies p JOIN leave_types t ON t.tenant_id=p.tenant_id AND t.id=p.leave_type_id AND t.is_active=TRUE WHERE p.tenant_id=? AND p.accrual_type='annual' AND p.valid_from<=? AND (p.valid_to IS NULL OR p.valid_to>=?)`, c.tenant.ID, jan1, jan1).Scan(&policies)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca kebijakan akrual.")
		return
	}
	if policies == 0 {
		writeAPIError(w, r, 409, "ANNUAL_POLICY_NOT_FOUND", "Kebijakan annual yang berlaku pada 1 Januari belum tersedia.")
		return
	}
	item, err := enqueueLeaveAccrualRun(r.Context(), a.db, c.tenant.ID, body.Year, now.Format("2006-01-02"), "manual", c.state.User.ID)
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal menjadwalkan akrual cuti.")
		return
	}
	status := http.StatusAccepted
	if item.Status == "completed" {
		status = http.StatusOK
	}
	writeAPIData(w, status, item, nil)
}

func (a adminAPI) getAdminLeaveAccrualRun(w http.ResponseWriter, r *http.Request) {
	c, ok := a.authorize(w, r, "cuti.kelola")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Proses akrual tidak ditemukan.")
		return
	}
	item, err := scanLeaveAccrualRun(a.db.QueryRowContext(r.Context(), `SELECT `+leaveAccrualRunColumns+` FROM leave_accrual_runs WHERE tenant_id=? AND id=?`, c.tenant.ID, id))
	if errors.Is(err, sql.ErrNoRows) {
		writeAPIError(w, r, 404, "NOT_FOUND", "Proses akrual tidak ditemukan.")
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "INTERNAL_ERROR", "Gagal membaca proses akrual.")
		return
	}
	writeAPIData(w, 200, item, nil)
}
