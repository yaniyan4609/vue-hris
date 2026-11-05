package access

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Store reloads access on each request so role and override changes take effect
// without a new login. A transaction can be supplied for access mutations.
type Store struct {
	DB interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}
}

func (s Store) Decide(ctx context.Context, tenantID, userID, timezone, code string) (Subject, Effective, error) {
	var permission Permission
	var entitlement sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT code, requires_scope, entitlement_code FROM permission_registry WHERE code=?`, code).Scan(&permission.Code, &permission.RequiresScope, &entitlement)
	if errors.Is(err, sql.ErrNoRows) {
		return Subject{}, Effective{Reason: "permission_unknown"}, nil
	}
	if err != nil {
		return Subject{}, Effective{}, err
	}
	permission.Entitlement = entitlement.String
	subject := Subject{TenantID: tenantID, UserID: userID, Overrides: map[string]Override{}, Entitlements: map[string]bool{}}
	var status string
	err = s.DB.QueryRowContext(ctx, `SELECT tm.status FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id WHERE tm.tenant_id=? AND tm.user_id=? AND t.status IN ('trial','active')`, tenantID, userID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return subject, Effective{Reason: "tenant_or_membership_invalid"}, nil
	}
	if err != nil {
		return Subject{}, Effective{}, err
	}
	subject.MemberActive = status == "active"
	if !subject.MemberActive {
		return subject, Effective{Reason: "tenant_or_membership_invalid"}, nil
	}
	_ = s.DB.QueryRowContext(ctx, `SELECT id FROM employees WHERE tenant_id=? AND user_id=? AND status IN ('active','probation','leave')`, tenantID, userID).Scan(&subject.EmployeeID)
	if permission.Entitlement != "" {
		var enabled int
		err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM tenant_subscriptions ts JOIN plan_entitlements pe ON pe.plan_id=ts.plan_id WHERE ts.tenant_id=? AND ts.status='active' AND pe.entitlement_code=?`, tenantID, permission.Entitlement).Scan(&enabled)
		if err != nil {
			return Subject{}, Effective{}, err
		}
		subject.Entitlements[permission.Entitlement] = enabled > 0
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.UTC
	}
	now := time.Now().In(location)
	date := now.Format("2006-01-02")
	rows, err := s.DB.QueryContext(ctx, `SELECT r.id, r.code, ur.valid_from, ur.valid_until FROM user_roles ur JOIN roles r ON r.tenant_id=ur.tenant_id AND r.id=ur.role_id JOIN role_permissions rp ON rp.tenant_id=ur.tenant_id AND rp.role_id=ur.role_id WHERE ur.tenant_id=? AND ur.user_id=? AND ur.revoked_at IS NULL AND rp.permission_code=? AND ur.valid_from<=? AND (ur.valid_until IS NULL OR ur.valid_until>=?)`, tenantID, userID, code, date, date)
	if err != nil {
		return Subject{}, Effective{}, err
	}
	type roleRow struct {
		id, code string
		from     time.Time
		until    sql.NullTime
	}
	var roleRows []roleRow
	for rows.Next() {
		var item roleRow
		if err := rows.Scan(&item.id, &item.code, &item.from, &item.until); err != nil {
			rows.Close()
			return Subject{}, Effective{}, err
		}
		roleRows = append(roleRows, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Subject{}, Effective{}, err
	}
	rows.Close()
	for _, item := range roleRows {
		role := Role{TenantID: tenantID, Code: item.code, ValidFrom: item.from}
		if item.until.Valid {
			role.ValidUntil = &item.until.Time
		}
		scopes, err := s.loadScopes(ctx, "role", tenantID, item.id, code)
		if err != nil {
			return Subject{}, Effective{}, err
		}
		role.Grants = []Grant{{Permission: code, Scopes: scopes}}
		subject.Roles = append(subject.Roles, role)
	}
	var decision string
	err = s.DB.QueryRowContext(ctx, `SELECT decision FROM user_permission_overrides WHERE tenant_id=? AND user_id=? AND permission_code=?`, tenantID, userID, code).Scan(&decision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Subject{}, Effective{}, err
	}
	override := Override{Decision: DecisionKind(decision)}
	var hasHeader int
	err = s.DB.QueryRowContext(ctx, `SELECT 1 FROM user_scope_overrides WHERE tenant_id=? AND user_id=? AND permission_code=?`, tenantID, userID, code).Scan(&hasHeader)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Subject{}, Effective{}, err
	}
	if err == nil {
		scopes, err := s.loadScopes(ctx, "user", tenantID, userID, code)
		if err != nil {
			return Subject{}, Effective{}, err
		}
		override.Scopes = &scopes
	}
	subject.Overrides[code] = override
	return subject, Evaluate(now, tenantID, subject, Registry{code: permission}, code), nil
}

func (s Store) loadScopes(ctx context.Context, source, tenantID, ownerID, code string) ([]ScopeRule, error) {
	var query string
	if source == "role" {
		query = `SELECT r.id, r.kind, v.dimension, v.node_id FROM role_scope_rules r LEFT JOIN role_scope_values v ON v.tenant_id=r.tenant_id AND v.rule_id=r.id WHERE r.tenant_id=? AND r.role_id=? AND r.permission_code=? ORDER BY r.id`
	} else if source == "user" {
		query = `SELECT r.id, r.kind, v.dimension, v.node_id FROM user_scope_rules r LEFT JOIN user_scope_values v ON v.tenant_id=r.tenant_id AND v.rule_id=r.id WHERE r.tenant_id=? AND r.user_id=? AND r.permission_code=? ORDER BY r.id`
	} else {
		return nil, fmt.Errorf("unknown scope source")
	}
	rows, err := s.DB.QueryContext(ctx, query, tenantID, ownerID, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ScopeRule
	lastID := ""
	for rows.Next() {
		var id, kind string
		var dimension, node sql.NullString
		if err := rows.Scan(&id, &kind, &dimension, &node); err != nil {
			return nil, err
		}
		if id != lastID {
			result = append(result, ScopeRule{AllTenant: kind == "all_tenant", Self: kind == "self", DirectReports: kind == "direct_reports"})
			lastID = id
		}
		if dimension.Valid && node.Valid {
			rule := &result[len(result)-1]
			switch dimension.String {
			case "region":
				rule.Regions = append(rule.Regions, node.String)
			case "office":
				rule.Offices = append(rule.Offices, node.String)
			case "division":
				rule.Divisions = append(rule.Divisions, node.String)
			case "department":
				rule.Departments = append(rule.Departments, node.String)
			case "position":
				rule.Positions = append(rule.Positions, node.String)
			default:
				return nil, fmt.Errorf("unknown scope dimension")
			}
		}
	}
	return result, rows.Err()
}

// EmployeeWhere returns only parameterized predicates for a query that joins
// employees e and their current employee_assignments a within one tenant.
func EmployeeWhere(subject Subject, effective Effective) (string, []any) {
	if !effective.Allowed || len(effective.Scopes) == 0 {
		return "1=0", nil
	}
	var alternatives []string
	var args []any
	for _, rule := range effective.Scopes {
		if rule.AllTenant {
			return "1=1", nil
		}
		if rule.Self {
			alternatives = append(alternatives, "e.user_id=?")
			args = append(args, subject.UserID)
			continue
		}
		if rule.DirectReports {
			alternatives = append(alternatives, "a.reports_to_employee_id=?")
			args = append(args, subject.EmployeeID)
			continue
		}
		var parts []string
		for _, item := range []struct {
			values []string
			column string
		}{
			{rule.Regions, "a.region_id"}, {rule.Offices, "a.office_id"}, {rule.Divisions, "a.division_id"}, {rule.Departments, "a.department_id"}, {rule.Positions, "a.position_id"},
		} {
			if len(item.values) == 0 {
				continue
			}
			parts = append(parts, item.column+" IN ("+strings.TrimSuffix(strings.Repeat("?,", len(item.values)), ",")+")")
			for _, value := range item.values {
				args = append(args, value)
			}
		}
		if len(parts) > 0 {
			alternatives = append(alternatives, "("+strings.Join(parts, " AND ")+")")
		}
	}
	if len(alternatives) == 0 {
		return "1=0", nil
	}
	return "(" + strings.Join(alternatives, " OR ") + ")", args
}
