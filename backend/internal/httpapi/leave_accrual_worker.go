package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"digaji.id/hris/internal/ids"
)

type annualAccrualCandidate struct {
	EmployeeID  string
	LeaveTypeID string
	PolicyID    string
	Quota       string
	JoinDate    time.Time
}

const annualAccrualFrom = ` FROM employees e JOIN leave_policies p ON p.tenant_id=e.tenant_id AND p.accrual_type='annual' AND p.valid_from<=? AND (p.valid_to IS NULL OR p.valid_to>=?) JOIN leave_types lt ON lt.tenant_id=p.tenant_id AND lt.id=p.leave_type_id AND lt.is_active=TRUE LEFT JOIN leave_accrual_grants g ON g.tenant_id=e.tenant_id AND g.employee_id=e.id AND g.leave_type_id=p.leave_type_id AND g.period_year=? WHERE e.tenant_id=? AND e.status IN ('active','probation','leave') AND e.join_date IS NOT NULL AND e.join_date<=? AND g.id IS NULL`

func annualCandidate(ctx context.Context, db *sql.DB, tenantID string, year int, asOf string) (annualAccrualCandidate, error) {
	var item annualAccrualCandidate
	jan1 := fmt.Sprintf("%04d-01-01", year)
	err := db.QueryRowContext(ctx, `SELECT e.id,p.leave_type_id,p.id,CAST(p.quota_days AS CHAR),e.join_date`+annualAccrualFrom+` ORDER BY e.id,p.leave_type_id LIMIT 1`, jan1, jan1, year, tenantID, asOf).Scan(&item.EmployeeID, &item.LeaveTypeID, &item.PolicyID, &item.Quota, &item.JoinDate)
	return item, err
}

func annualGrantDays(quotaHundredths int64, joinDate time.Time, year int) int64 {
	months := 12
	if joinDate.Year() == year {
		months = 13 - int(joinDate.Month())
	}
	if months < 1 {
		return 0
	}
	return (quotaHundredths*int64(months) + 6) / 12 // nearest hundredth, half up
}

func grantAnnualLeave(ctx context.Context, db *sql.DB, tenantID, runID string, year int, asOf string, c annualAccrualCandidate) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var joinDate time.Time
	var status string
	err = tx.QueryRowContext(ctx, `SELECT join_date,status FROM employees WHERE tenant_id=? AND id=? AND join_date<=? FOR UPDATE`, tenantID, c.EmployeeID, asOf).Scan(&joinDate, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if status != "active" && status != "probation" && status != "leave" {
		return false, nil
	}
	var exists int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM leave_accrual_grants WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND period_year=?`, tenantID, c.EmployeeID, c.LeaveTypeID, year).Scan(&exists); err != nil {
		return false, err
	}
	if exists != 0 {
		return false, nil
	}
	jan1 := fmt.Sprintf("%04d-01-01", year)
	var quota string
	err = tx.QueryRowContext(ctx, `SELECT CAST(quota_days AS CHAR) FROM leave_policies WHERE tenant_id=? AND id=? AND leave_type_id=? AND accrual_type='annual' AND valid_from<=? AND (valid_to IS NULL OR valid_to>=?) FOR UPDATE`, tenantID, c.PolicyID, c.LeaveTypeID, jan1, jan1).Scan(&quota)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	hundredths, ok := leaveHundredths(quota)
	if !ok || hundredths < 100 {
		return false, fmt.Errorf("invalid annual quota")
	}
	amount := annualGrantDays(hundredths, joinDate, year)
	if amount <= 0 {
		return false, nil
	}
	grantID, err := ids.New()
	if err != nil {
		return false, err
	}
	ledgerID, err := ids.New()
	if err != nil {
		return false, err
	}
	effective := jan1
	if joinDate.Year() == year {
		effective = joinDate.Format("2006-01-02")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO leave_balance_ledger (tenant_id,id,employee_id,leave_type_id,entry_type,days,effective_date,ref_type,ref_id,note) VALUES (?,?,?,?,'accrual',?,?,'leave_accrual_grant',?,?)`, tenantID, ledgerID, c.EmployeeID, c.LeaveTypeID, leaveDecimal(amount), effective, grantID, fmt.Sprintf("Akrual tahunan %d", year)); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO leave_accrual_grants (tenant_id,id,employee_id,leave_type_id,period_year,policy_id,run_id,ledger_id,days) VALUES (?,?,?,?,?,?,?,?,?)`, tenantID, grantID, c.EmployeeID, c.LeaveTypeID, year, c.PolicyID, runID, ledgerID, leaveDecimal(amount)); err != nil {
		return false, err
	}
	from, to := jan1, fmt.Sprintf("%04d-01-01", year+1)
	var balance string
	if err = tx.QueryRowContext(ctx, `SELECT CAST(COALESCE(SUM(days),0) AS CHAR) FROM leave_balance_ledger WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND effective_date>=? AND effective_date<?`, tenantID, c.EmployeeID, c.LeaveTypeID, from, to).Scan(&balance); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO leave_balances (tenant_id,employee_id,leave_type_id,period_year,balance_days,computed_at) VALUES (?,?,?,?,?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE balance_days=VALUES(balance_days),computed_at=VALUES(computed_at)`, tenantID, c.EmployeeID, c.LeaveTypeID, year, balance); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// RunDueLeaveAccruals is safe to call on every minute tick and on multiple API
// instances. Unique grants and a short run lease make retries idempotent.
func RunDueLeaveAccruals(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,timezone FROM tenants WHERE status IN ('trial','active','grace')`)
	if err != nil {
		return 0, err
	}
	type tenant struct{ id, zone string }
	tenants := []tenant{}
	for rows.Next() {
		var item tenant
		if err = rows.Scan(&item.id, &item.zone); err != nil {
			break
		}
		tenants = append(tenants, item)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, item := range tenants {
		zone, e := time.LoadLocation(item.zone)
		if e != nil {
			zone = time.UTC
		}
		now := time.Now().In(zone)
		day := now.Format("2006-01-02")
		if e = expireAnnualLeave(ctx, db, item.id, now.Year()); e != nil {
			return 0, e
		}
		_, e = annualCandidate(ctx, db, item.id, now.Year(), day)
		if errors.Is(e, sql.ErrNoRows) {
			continue
		}
		if e != nil {
			return 0, e
		}
		if _, e = enqueueLeaveAccrualRun(ctx, db, item.id, now.Year(), day, "automatic", ""); e != nil {
			return 0, e
		}
	}
	runs, err := db.QueryContext(ctx, `SELECT tenant_id,id,period_year,as_of_date FROM leave_accrual_runs WHERE status='pending' OR (status='running' AND lease_until<UTC_TIMESTAMP(6)) ORDER BY created_at,id LIMIT 10`)
	if err != nil {
		return 0, err
	}
	type run struct {
		tenantID, id string
		year         int
		day          time.Time
	}
	pending := []run{}
	for runs.Next() {
		var item run
		if err = runs.Scan(&item.tenantID, &item.id, &item.year, &item.day); err != nil {
			break
		}
		pending = append(pending, item)
	}
	if err == nil {
		err = runs.Err()
	}
	runs.Close()
	if err != nil {
		return 0, err
	}
	credited := 0
	for _, item := range pending {
		count, e := processLeaveAccrualRun(ctx, db, item.tenantID, item.id, item.year, item.day.Format("2006-01-02"))
		credited += count
		if e != nil {
			return credited, e
		}
	}
	return credited, nil
}

// expireAnnualLeave records the forfeiture in the ledger and zeros the old
// year's balance. The grant ID is the idempotency key for each annual balance.
func expireAnnualLeave(ctx context.Context, db *sql.DB, tenantID string, currentYear int) error {
	rows, err := db.QueryContext(ctx, `SELECT g.employee_id,g.leave_type_id,g.period_year,g.id FROM leave_accrual_grants g JOIN leave_balances b ON b.tenant_id=g.tenant_id AND b.employee_id=g.employee_id AND b.leave_type_id=g.leave_type_id AND b.period_year=g.period_year WHERE g.tenant_id=? AND g.period_year<? AND b.balance_days>0 ORDER BY g.period_year,g.employee_id LIMIT 100`, tenantID, currentYear)
	if err != nil {
		return err
	}
	type expired struct {
		employeeID, typeID, grantID string
		year                        int
	}
	items := []expired{}
	for rows.Next() {
		var item expired
		if err = rows.Scan(&item.employeeID, &item.typeID, &item.year, &item.grantID); err != nil {
			break
		}
		items = append(items, item)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range items {
		if err = expireAnnualGrant(ctx, db, tenantID, item); err != nil {
			return err
		}
	}
	return nil
}

func expireAnnualGrant(ctx context.Context, db *sql.DB, tenantID string, item struct {
	employeeID, typeID, grantID string
	year                        int
}) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var employee string
	if err = tx.QueryRowContext(ctx, `SELECT id FROM employees WHERE tenant_id=? AND id=? FOR UPDATE`, tenantID, item.employeeID).Scan(&employee); err != nil {
		return err
	}
	var existing int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM leave_balance_ledger WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND entry_type='expiry' AND ref_type='leave_accrual_grant' AND ref_id=?`, tenantID, item.employeeID, item.typeID, item.grantID).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	var balance string
	if err = tx.QueryRowContext(ctx, `SELECT CAST(balance_days AS CHAR) FROM leave_balances WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND period_year=? FOR UPDATE`, tenantID, item.employeeID, item.typeID, item.year).Scan(&balance); err != nil {
		return err
	}
	hundredths, ok := leaveHundredths(balance)
	if !ok {
		return fmt.Errorf("invalid annual balance")
	}
	if hundredths <= 0 {
		return nil
	}
	ledgerID, err := ids.New()
	if err != nil {
		return err
	}
	effective := fmt.Sprintf("%04d-12-31", item.year)
	if _, err = tx.ExecContext(ctx, `INSERT INTO leave_balance_ledger (tenant_id,id,employee_id,leave_type_id,entry_type,days,effective_date,ref_type,ref_id,note) VALUES (?,?,?,?,'expiry',?,?,'leave_accrual_grant',?,?)`, tenantID, ledgerID, item.employeeID, item.typeID, leaveDecimal(-hundredths), effective, item.grantID, "Sisa cuti tahunan hangus"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE leave_balances SET balance_days=0,computed_at=UTC_TIMESTAMP(6) WHERE tenant_id=? AND employee_id=? AND leave_type_id=? AND period_year=?`, tenantID, item.employeeID, item.typeID, item.year); err != nil {
		return err
	}
	return tx.Commit()
}

func processLeaveAccrualRun(ctx context.Context, db *sql.DB, tenantID, runID string, year int, asOf string) (int, error) {
	claim, err := db.ExecContext(ctx, `UPDATE leave_accrual_runs SET status='running',started_at=COALESCE(started_at,UTC_TIMESTAMP(6)),lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 2 MINUTE),last_error=NULL WHERE tenant_id=? AND id=? AND (status='pending' OR (status='running' AND lease_until<UTC_TIMESTAMP(6)))`, tenantID, runID)
	if err != nil {
		return 0, err
	}
	rows, _ := claim.RowsAffected()
	if rows == 0 {
		return 0, nil
	}
	credited := 0
	for processed := 0; processed < 100; processed++ {
		candidate, e := annualCandidate(ctx, db, tenantID, year, asOf)
		if errors.Is(e, sql.ErrNoRows) {
			_, err = db.ExecContext(ctx, `UPDATE leave_accrual_runs SET status='completed',finished_at=UTC_TIMESTAMP(6),lease_until=NULL WHERE tenant_id=? AND id=? AND status='running'`, tenantID, runID)
			return credited, err
		}
		if e != nil {
			markLeaveAccrualFailed(ctx, db, tenantID, runID)
			return credited, e
		}
		granted, e := grantAnnualLeave(ctx, db, tenantID, runID, year, asOf, candidate)
		if e != nil {
			markLeaveAccrualFailed(ctx, db, tenantID, runID)
			return credited, e
		}
		if granted {
			credited++
		}
		_, e = db.ExecContext(ctx, `UPDATE leave_accrual_runs SET processed_count=processed_count+1,credited_count=credited_count+?,lease_until=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 2 MINUTE) WHERE tenant_id=? AND id=? AND status='running'`, granted, tenantID, runID)
		if e != nil {
			return credited, e
		}
	}
	_, err = db.ExecContext(ctx, `UPDATE leave_accrual_runs SET status='pending',lease_until=NULL WHERE tenant_id=? AND id=? AND status='running'`, tenantID, runID)
	return credited, err
}

func markLeaveAccrualFailed(ctx context.Context, db *sql.DB, tenantID, runID string) {
	_, _ = db.ExecContext(ctx, `UPDATE leave_accrual_runs SET status='failed',lease_until=NULL,last_error='Pemrosesan gagal; periksa log server lalu jalankan ulang.' WHERE tenant_id=? AND id=?`, tenantID, runID)
}
