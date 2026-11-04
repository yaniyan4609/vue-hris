package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"digaji.id/hris/internal/auth"
	"digaji.id/hris/internal/database"
	"digaji.id/hris/internal/ids"
	"golang.org/x/term"
)

const demoDatabase = "staging-saas-hris"

func main() {
	config := database.FromEnv()
	if os.Getenv("APP_ENV") != "staging" || config.Name != demoDatabase {
		slog.Error("seed demo hanya dapat dijalankan pada staging")
		os.Exit(1)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		slog.Error("seed demo memerlukan terminal interaktif")
		os.Exit(1)
	}
	if config.Password == "" {
		config.Password = readSecret("Kata sandi database: ")
	}
	demoPassword := readSecret("Kata sandi bersama untuk tiga akun demo: ")
	if demoPassword == "" {
		slog.Error("kata sandi demo tidak boleh kosong")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, config)
	if err != nil {
		slog.Error("database tidak tersedia", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := seed(ctx, db, demoPassword); err != nil {
		slog.Error("seed demo gagal", "error", err)
		os.Exit(1)
	}
	fmt.Println("Akun demo staging siap: super@digaji.id, admin@digaji.id, ess@digaji.id")
}

func readSecret(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	secret, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		slog.Error("gagal membaca kata sandi", "error", err)
		os.Exit(1)
	}
	return string(secret)
}

func seed(ctx context.Context, db *sql.DB, password string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tenantID, err := ensureTenant(ctx, tx)
	if err != nil {
		return err
	}
	superID, err := ensureUser(ctx, tx, "super@digaji.id", "Superadmin Digaji.id", password)
	if err != nil {
		return err
	}
	adminID, err := ensureUser(ctx, tx, "admin@digaji.id", "Admin Tenant", password)
	if err != nil {
		return err
	}
	essID, err := ensureUser(ctx, tx, "ess@digaji.id", "Karyawan Demo", password)
	if err != nil {
		return err
	}
	if err := ensureMembership(ctx, tx, tenantID, adminID); err != nil {
		return err
	}
	if err := ensureMembership(ctx, tx, tenantID, essID); err != nil {
		return err
	}
	platformRoleID, err := ensurePlatformRole(ctx, tx)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO platform_user_roles (user_id, role_id, valid_from) VALUES (?, ?, CURRENT_DATE())`, superID, platformRoleID); err != nil {
		return err
	}
	adminRoleID, err := ensureTenantRole(ctx, tx, tenantID, "ADMIN_TENANT", "Admin Tenant")
	if err != nil {
		return err
	}
	employeeRoleID, err := ensureTenantRole(ctx, tx, tenantID, "KARYAWAN", "Karyawan")
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO role_permissions (tenant_id, role_id, permission_code) VALUES (?, ?, 'panel.admin')`, tenantID, adminRoleID); err != nil {
		return err
	}
	for _, assignment := range [][2]string{{adminID, adminRoleID}, {essID, employeeRoleID}} {
		if err := ensureUserRole(ctx, tx, tenantID, assignment[0], assignment[1]); err != nil {
			return err
		}
	}
	regionID, err := ensureSimpleNode(ctx, tx, "regions", tenantID, "DEMO", "Wilayah Demo")
	if err != nil {
		return err
	}
	officeID, err := ensureChildNode(ctx, tx, "offices", "region_id", tenantID, regionID, "PUSAT", "Kantor Pusat")
	if err != nil {
		return err
	}
	divisionID, err := ensureSimpleNode(ctx, tx, "divisions", tenantID, "UMUM", "Divisi Umum")
	if err != nil {
		return err
	}
	departmentID, err := ensureChildNode(ctx, tx, "departments", "division_id", tenantID, divisionID, "OPERASIONAL", "Departemen Operasional")
	if err != nil {
		return err
	}
	positionID, err := ensureChildNode(ctx, tx, "positions", "department_id", tenantID, departmentID, "STAFF", "Staf Operasional")
	if err != nil {
		return err
	}
	if err := ensureEmployee(ctx, tx, tenantID, essID, officeID, positionID); err != nil {
		return err
	}
	return tx.Commit()
}

func ensureTenant(ctx context.Context, tx *sql.Tx) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM tenants WHERE slug='client-pertama'`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id, err = ids.New()
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO tenants (id, slug, name, status) VALUES (?, 'client-pertama', 'PT CLIENT PERTAMA', 'active')`, id)
	return id, err
}

func ensureUser(ctx context.Context, tx *sql.Tx, email, name, password string) (string, error) {
	var id string
	var hash sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT id, password_hash FROM users WHERE email_normalized=?`, strings.ToLower(email)).Scan(&id, &hash)
	if err == nil {
		if !hash.Valid || !auth.VerifyPassword(hash.String, password) {
			return "", fmt.Errorf("akun %s sudah ada dengan kata sandi berbeda; seed tidak menimpa", email)
		}
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id, err = ids.New()
	if err != nil {
		return "", err
	}
	encoded, err := auth.HashPassword(password)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO users (id, email, email_normalized, display_name, password_hash) VALUES (?, ?, ?, ?, ?)`, id, email, strings.ToLower(email), name, encoded)
	return id, err
}

func ensureMembership(ctx context.Context, tx *sql.Tx, tenantID, userID string) error {
	_, err := tx.ExecContext(ctx, `INSERT IGNORE INTO tenant_memberships (tenant_id, user_id, status) VALUES (?, ?, 'active')`, tenantID, userID)
	return err
}

func ensurePlatformRole(ctx context.Context, tx *sql.Tx) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM platform_roles WHERE code='SUPERADMIN'`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id, err = ids.New()
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO platform_roles (id, code, name) VALUES (?, 'SUPERADMIN', 'Superadmin')`, id)
	return id, err
}

func ensureTenantRole(ctx context.Context, tx *sql.Tx, tenantID, code, name string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE tenant_id=? AND code=?`, tenantID, code).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id, err = ids.New()
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO roles (tenant_id, id, code, name, is_system) VALUES (?, ?, ?, ?, TRUE)`, tenantID, id, code, name)
	return id, err
}

func ensureUserRole(ctx context.Context, tx *sql.Tx, tenantID, userID, roleID string) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_roles WHERE tenant_id=? AND user_id=? AND role_id=? AND (valid_until IS NULL OR valid_until>=CURRENT_DATE())`, tenantID, userID, roleID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	id, err := ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_roles (tenant_id, id, user_id, role_id, valid_from) VALUES (?, ?, ?, ?, CURRENT_DATE())`, tenantID, id, userID, roleID)
	return err
}

// Table and parentColumn are fixed literals from this file, never user input.
func ensureSimpleNode(ctx context.Context, tx *sql.Tx, table, tenantID, code, name string) (string, error) {
	return ensureNode(ctx, tx, table, "", tenantID, "", code, name)
}

func ensureChildNode(ctx context.Context, tx *sql.Tx, table, parentColumn, tenantID, parentID, code, name string) (string, error) {
	return ensureNode(ctx, tx, table, parentColumn, tenantID, parentID, code, name)
}

func ensureNode(ctx context.Context, tx *sql.Tx, table, parentColumn, tenantID, parentID, code, name string) (string, error) {
	var id string
	query := fmt.Sprintf("SELECT id FROM %s WHERE tenant_id=? AND code=?", table)
	err := tx.QueryRowContext(ctx, query, tenantID, code).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id, err = ids.New()
	if err != nil {
		return "", err
	}
	if parentColumn == "" {
		query = fmt.Sprintf("INSERT INTO %s (tenant_id, id, code, name) VALUES (?, ?, ?, ?)", table)
		_, err = tx.ExecContext(ctx, query, tenantID, id, code, name)
	} else {
		query = fmt.Sprintf("INSERT INTO %s (tenant_id, id, %s, code, name) VALUES (?, ?, ?, ?, ?)", table, parentColumn)
		_, err = tx.ExecContext(ctx, query, tenantID, id, parentID, code, name)
	}
	return id, err
}

func ensureEmployee(ctx context.Context, tx *sql.Tx, tenantID, userID, officeID, positionID string) error {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM employees WHERE tenant_id=? AND user_id=?`, tenantID, userID).Scan(&id)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	id, err = ids.New()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO employees (tenant_id, id, user_id, employee_number, display_name, status, base_office_id, position_id) VALUES (?, ?, ?, 'DEMO-001', 'Karyawan Demo', 'active', ?, ?)`, tenantID, id, userID, officeID, positionID)
	return err
}
