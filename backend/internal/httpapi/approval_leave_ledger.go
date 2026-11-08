package httpapi

import (
	"database/sql"
	"fmt"
	"net/http"

	"digaji.id/hris/internal/ids"
)

// A request may cross 31 December. Each allocation has its own unique ledger
// reference, so retries cannot debit either year twice.
func approveLeaveLedger(r *http.Request, tx *sql.Tx, tenantID string, d overtimeDecision) error {
	var employeeID, leaveTypeID string
	if err := tx.QueryRowContext(r.Context(), `SELECT employee_id,leave_type_id FROM leave_requests WHERE tenant_id=? AND id=? AND request_id=?`, tenantID, d.RefID, d.RequestID).Scan(&employeeID, &leaveTypeID); err != nil {
		return err
	}
	var locked string
	if err := tx.QueryRowContext(r.Context(), `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, employeeID).Scan(&locked); err != nil {
		return err
	}
	rows, err := tx.QueryContext(r.Context(), `SELECT id,period_year,first_work_date,CAST(ROUND(days*100) AS SIGNED) FROM leave_request_allocations WHERE tenant_id=? AND leave_request_id=? ORDER BY period_year`, tenantID, d.RefID)
	if err != nil {
		return err
	}
	type allocation struct {
		id         string
		year       int
		first      string
		hundredths int64
	}
	allocations := []allocation{}
	for rows.Next() {
		var a allocation
		var firstTime sql.NullTime
		if err = rows.Scan(&a.id, &a.year, &firstTime, &a.hundredths); err != nil {
			break
		}
		if !firstTime.Valid {
			err = fmt.Errorf("missing allocation date")
			break
		}
		a.first = firstTime.Time.Format("2006-01-02")
		allocations = append(allocations, a)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	if len(allocations) == 0 {
		return leaveConflict("LEAVE_ALLOCATION_MISSING", "Alokasi tahun pengajuan cuti tidak tersedia.")
	}
	for _, a := range allocations {
		from, to := fmt.Sprintf("%04d-01-01", a.year), fmt.Sprintf("%04d-01-01", a.year+1)
		var balance, otherReserved int64
		if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(CAST(ROUND(SUM(days)*100) AS SIGNED),0) FROM leave_balance_ledger WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND effective_date>=? AND effective_date<?`, tenantID, employeeID, leaveTypeID, from, to).Scan(&balance); err != nil {
			return err
		}
		if err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(CAST(ROUND(SUM(a.days)*100) AS SIGNED),0) FROM leave_request_allocations a JOIN leave_requests lr ON lr.tenant_id=a.tenant_id AND lr.id=a.leave_request_id WHERE a.tenant_id=? AND lr.employee_id=? AND lr.leave_type_id=? AND a.period_year=? AND lr.status IN ('submitted','in_review') AND lr.id<>?`, tenantID, employeeID, leaveTypeID, a.year, d.RefID).Scan(&otherReserved); err != nil {
			return err
		}
		if balance-otherReserved < a.hundredths {
			return leaveConflict("INSUFFICIENT_LEAVE_BALANCE", fmt.Sprintf("Saldo cuti tahun %d tidak cukup untuk persetujuan akhir.", a.year))
		}
		ledgerID, e := ids.New()
		if e != nil {
			return e
		}
		usage := fmt.Sprintf("-%d.%02d", a.hundredths/100, a.hundredths%100)
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO leave_balance_ledger (tenant_id,id,employee_id,leave_type_id,entry_type,days,effective_date,ref_type,ref_id,note) VALUES (?,?,?,?,'usage',?,?,'leave_request_allocation',?,?)`, tenantID, ledgerID, employeeID, leaveTypeID, usage, a.first, a.id, "Persetujuan cuti "+d.RequestID); err != nil {
			return err
		}
		remaining := balance - a.hundredths
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO leave_balances (tenant_id,employee_id,leave_type_id,period_year,balance_days,computed_at) VALUES (?,?,?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE balance_days=VALUES(balance_days),computed_at=VALUES(computed_at)`, tenantID, employeeID, leaveTypeID, a.year, fmt.Sprintf("%d.%02d", remaining/100, remaining%100)); err != nil {
			return err
		}
	}
	return nil
}
