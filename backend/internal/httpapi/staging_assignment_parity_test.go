package httpapi

import (
	"context"
	"os"
	"testing"
	"time"

	"digaji.id/hris/internal/database"
)

func TestStagingCurrentAssignmentParity(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("integration test requires the named staging database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var mismatches int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM employees e
		JOIN offices o ON o.tenant_id=e.tenant_id AND o.id=e.base_office_id
		JOIN positions p ON p.tenant_id=e.tenant_id AND p.id=e.position_id
		JOIN departments d ON d.tenant_id=p.tenant_id AND d.id=p.department_id
		LEFT JOIN employee_assignments a ON a.tenant_id=e.tenant_id AND a.employee_id=e.id AND a.current_flag=1
		WHERE a.id IS NULL OR a.office_id<>e.base_office_id OR a.position_id<>e.position_id
		OR NOT (a.reports_to_employee_id <=> e.reports_to_id)
		OR a.region_id<>o.region_id OR a.department_id<>p.department_id OR a.division_id<>d.division_id`).Scan(&mismatches)
	if err != nil {
		t.Fatal(err)
	}
	if mismatches != 0 {
		t.Fatalf("%d current assignments differ from legacy employee placement", mismatches)
	}
}
