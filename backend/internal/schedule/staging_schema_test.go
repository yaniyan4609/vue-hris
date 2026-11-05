package schedule

import (
	"context"
	"os"
	"testing"
	"time"

	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
)

// This test uses only temporary rows in one transaction and rolls them back.
func TestStagingRosterOneShiftAndTenantForeignKeys(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("integration test requires the named staging database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	newID := func() string {
		id, err := ids.New()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	var tenantID, employeeID, officeID string
	if err := tx.QueryRowContext(ctx, `SELECT tenant_id,id,base_office_id FROM employees LIMIT 1`).Scan(&tenantID, &employeeID, &officeID); err != nil {
		t.Fatal(err)
	}
	shiftID, patternID, slotID, rosterID, foreignTenantID, foreignPatternID := newID(), newID(), newID(), newID(), newID(), newID()
	_, err = tx.ExecContext(ctx, `INSERT INTO shift_templates
		(tenant_id,id,code,name,start_time,end_time,clock_in_window_start_time,clock_in_window_end_time,clock_out_window_start_time,clock_out_window_end_time)
		VALUES (?,?,?,'QA Shift','09:00:00','17:00:00','08:00:00','09:30:00','16:30:00','19:00:00')`, tenantID, shiftID, shiftID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO schedule_patterns (tenant_id,id,code,name,pattern_type) VALUES (?,?,?,'QA Pattern','monthly')`, tenantID, patternID, patternID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO schedule_pattern_slots (tenant_id,id,pattern_id,week_no,weekday,day_type,shift_template_id) VALUES (?,?,?,5,1,'shift',?)`, tenantID, slotID, patternID, shiftID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO work_schedules (tenant_id,id,employee_id,work_date,shift_template_id,day_type,office_id,source_pattern_id,source_pattern_version) VALUES (?,?,?,'2099-08-31',?,'shift',?,?,1)`, tenantID, rosterID, employeeID, shiftID, officeID, patternID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO work_schedules (tenant_id,id,employee_id,work_date,shift_template_id,day_type,office_id) VALUES (?,?,?,'2099-08-31',?,'shift',?)`, tenantID, newID(), employeeID, shiftID, officeID); err == nil {
		t.Fatal("duplicate shift for the same employee/work_date was accepted")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO tenants (id,slug,name,status) VALUES (?,?,?,'active')`, foreignTenantID, "qa-"+foreignTenantID, "QA Foreign Tenant")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO schedule_patterns (tenant_id,id,code,name,pattern_type) VALUES (?,?,?,'QA Foreign Pattern','monthly')`, foreignTenantID, foreignPatternID, foreignPatternID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schedule_pattern_slots (tenant_id,id,pattern_id,week_no,weekday,day_type,shift_template_id) VALUES (?,?,?,5,1,'shift',?)`, foreignTenantID, newID(), foreignPatternID, shiftID); err == nil {
		t.Fatal("cross-tenant shift reference was accepted")
	}
}
