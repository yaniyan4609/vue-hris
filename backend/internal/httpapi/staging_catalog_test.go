package httpapi

import (
	"context"
	"os"
	"testing"
	"time"

	"digaji.id/hris/internal/access"
	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
)

func TestStagingCatalogAccessMatchesSingleDecisions(t *testing.T) {
	if os.Getenv("HRIS_INTEGRATION") != "1" {
		t.Skip("staging integration test is opt-in")
	}
	if os.Getenv("APP_ENV") != "staging" || os.Getenv("DB_NAME") != "staging-saas-hris" {
		t.Fatal("named staging database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.FromEnv())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	var tenantID, userID string
	if err := db.QueryRowContext(ctx, `SELECT t.id,u.id FROM tenants t JOIN tenant_memberships tm ON tm.tenant_id=t.id JOIN users u ON u.id=tm.user_id WHERE t.slug='client-pertama' AND u.email_normalized='admin@digaji.id'`).Scan(&tenantID, &userID); err != nil {
		t.Fatal(err)
	}
	store := access.Store{DB: db}
	stale, err := db.QueryContext(ctx, `SELECT id FROM users WHERE email_normalized LIKE 'qa-catalog-%@example.invalid'`)
	if err != nil {
		t.Fatal(err)
	}
	var staleIDs []string
	for stale.Next() {
		var id string
		if err := stale.Scan(&id); err != nil {
			stale.Close()
			t.Fatal(err)
		}
		staleIDs = append(staleIDs, id)
	}
	if err := stale.Err(); err != nil {
		stale.Close()
		t.Fatal(err)
	}
	stale.Close()
	for _, id := range staleIDs {
		if _, err := db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=?`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM tenant_memberships WHERE user_id=?`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id); err != nil {
			t.Fatal(err)
		}
	}
	batch, entitlements, err := store.DecideAll(ctx, tenantID, userID, "Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	if !entitlements["core_hr"] || !entitlements["payroll"] {
		t.Fatalf("staging entitlements absent: %#v", entitlements)
	}
	var menuCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM permission_registry WHERE code LIKE 'menu.%.lihat'`).Scan(&menuCount); err != nil {
		t.Fatal(err)
	}
	if menuCount != 51 {
		t.Fatalf("Admin menu registry has %d entries, want 51", menuCount)
	}
	for _, check := range []struct {
		role, code string
		want       bool
	}{
		{"ADMIN_TENANT", "menu.performance.lihat", true},
		{"ADMIN_TENANT", "menu.bank-export.lihat", false},
		{"HR_STAFF", "menu.leave-policy.lihat", true},
		{"HR_STAFF", "cuti.kelola", false},
		{"HR_STAFF", "notifikasi.kelola", false},
		{"SPV", "menu.roster.lihat", true},
		{"SPV", "menu.shifts.lihat", false},
		{"FINANCE_PAYROLL", "menu.bank-export.lihat", true},
		{"FINANCE_PAYROLL", "menu.leave-policy.lihat", false},
		{"OWNER", "menu.users-access.lihat", true},
		{"OWNER", "menu.company-profile.lihat", false},
	} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM role_permissions rp JOIN roles r ON r.tenant_id=rp.tenant_id AND r.id=rp.role_id WHERE r.tenant_id=? AND r.code=? AND rp.permission_code=?`, tenantID, check.role, check.code).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if (count == 1) != check.want {
			t.Fatalf("preset %s / %s: found %d, want grant=%t", check.role, check.code, count, check.want)
		}
	}
	for _, code := range []string{"panel.admin", "karyawan.lihat", "kontrak.lihat", "absensi.lihat", "penggajian.lihat", "pengaturan.lihat"} {
		_, single, err := store.Decide(ctx, tenantID, userID, "Asia/Jakarta", code)
		if err != nil {
			t.Fatal(err)
		}
		many, ok := batch[code]
		if !ok || single.Allowed != many.Allowed || single.Reason != many.Reason || len(single.Scopes) != len(many.Scopes) {
			t.Fatalf("batch mismatch for %s: single=%+v batch=%+v", code, single, many)
		}
	}
	// Dua role aktif harus menghasilkan gabungan scope yang sama pada kedua jalur evaluasi.
	tempUser, err := ids.New()
	if err != nil {
		t.Fatal(err)
	}
	firstRole, err := ids.New()
	if err != nil {
		t.Fatal(err)
	}
	secondRole, err := ids.New()
	if err != nil {
		t.Fatal(err)
	}
	email := "qa-catalog-" + tempUser + "@example.invalid"
	if _, err := db.ExecContext(ctx, `INSERT INTO users (id,email,email_normalized,display_name) VALUES (?,?,?,'QA Catalog')`, tempUser, email, email); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM user_roles WHERE tenant_id=? AND user_id=?`, tenantID, tempUser); err != nil {
			t.Errorf("cleanup roles: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM tenant_memberships WHERE tenant_id=? AND user_id=?`, tenantID, tempUser); err != nil {
			t.Errorf("cleanup membership: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM users WHERE id=?`, tempUser); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})
	if _, err := db.ExecContext(ctx, `INSERT INTO tenant_memberships (tenant_id,user_id,status) VALUES (?,?,'active')`, tenantID, tempUser); err != nil {
		t.Fatal(err)
	}
	for _, role := range []struct{ id, code string }{{firstRole, "HR_STAFF"}, {secondRole, "SPV"}} {
		if _, err := db.ExecContext(ctx, `INSERT INTO user_roles (tenant_id,id,user_id,role_id,valid_from) SELECT ?,?,?,r.id,CURRENT_DATE() FROM roles r WHERE r.tenant_id=? AND r.code=?`, tenantID, role.id, tempUser, tenantID, role.code); err != nil {
			t.Fatal(err)
		}
	}
	multi, _, err := store.DecideAll(ctx, tenantID, tempUser, "Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	_, singleMulti, err := store.Decide(ctx, tenantID, tempUser, "Asia/Jakarta", "karyawan.lihat")
	if err != nil {
		t.Fatal(err)
	}
	if !multi["karyawan.lihat"].Allowed || len(multi["karyawan.lihat"].Scopes) != len(singleMulti.Scopes) || len(singleMulti.Scopes) < 2 {
		t.Fatalf("multi-role scope mismatch: batch=%+v single=%+v", multi["karyawan.lihat"], singleMulti)
	}
	var superID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE email_normalized='super@digaji.id'`).Scan(&superID); err != nil {
		t.Fatal(err)
	}
	platform, superadmin, err := platformAccess(ctx, db, superID)
	if err != nil {
		t.Fatal(err)
	}
	if !superadmin || !platform["platform.pengguna.kelola"].Allowed || !platform["platform.perusahaan.lihat"].Allowed {
		t.Fatal("superadmin platform access absent")
	}
}
