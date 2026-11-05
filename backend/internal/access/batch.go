package access

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// DecideAll loads grants and scope rules in batches for menus and access matrices.
func (s Store) DecideAll(ctx context.Context, tenantID, userID, timezone string) (map[string]Effective, map[string]bool, error) {
	registry := Registry{}
	rows, err := s.DB.QueryContext(ctx, `SELECT code,COALESCE(entitlement_code,''),requires_scope FROM permission_registry`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.Code, &p.Entitlement, &p.RequiresScope); err != nil {
			rows.Close()
			return nil, nil, err
		}
		registry[p.Code] = p
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()
	subject := Subject{TenantID: tenantID, UserID: userID, Overrides: map[string]Override{}, Entitlements: map[string]bool{}}
	var memberStatus string
	err = s.DB.QueryRowContext(ctx, `SELECT tm.status FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id WHERE tm.tenant_id=? AND tm.user_id=? AND t.status IN ('trial','active')`, tenantID, userID).Scan(&memberStatus)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, err
	}
	subject.MemberActive = err == nil && memberStatus == "active"
	if subject.MemberActive {
		_ = s.DB.QueryRowContext(ctx, `SELECT id FROM employees WHERE tenant_id=? AND user_id=? AND status IN ('active','probation','leave')`, tenantID, userID).Scan(&subject.EmployeeID)
		rows, err = s.DB.QueryContext(ctx, `SELECT pe.entitlement_code FROM tenant_subscriptions ts JOIN plan_entitlements pe ON pe.plan_id=ts.plan_id WHERE ts.tenant_id=? AND ts.status='active'`, tenantID)
		if err != nil {
			return nil, nil, err
		}
		for rows.Next() {
			var code string
			if err := rows.Scan(&code); err != nil {
				rows.Close()
				return nil, nil, err
			}
			subject.Entitlements[code] = true
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, nil, err
		}
		rows.Close()
		location, err := time.LoadLocation(timezone)
		if err != nil {
			location = time.UTC
		}
		date := time.Now().In(location).Format("2006-01-02")
		rows, err = s.DB.QueryContext(ctx, `SELECT ur.id,r.code,ur.valid_from,ur.valid_until,rp.permission_code,sr.id,sr.kind,sv.dimension,sv.node_id FROM user_roles ur JOIN roles r ON r.tenant_id=ur.tenant_id AND r.id=ur.role_id JOIN role_permissions rp ON rp.tenant_id=ur.tenant_id AND rp.role_id=ur.role_id LEFT JOIN role_scope_rules sr ON sr.tenant_id=rp.tenant_id AND sr.role_id=rp.role_id AND sr.permission_code=rp.permission_code LEFT JOIN role_scope_values sv ON sv.tenant_id=sr.tenant_id AND sv.rule_id=sr.id WHERE ur.tenant_id=? AND ur.user_id=? AND ur.revoked_at IS NULL AND ur.valid_from<=? AND (ur.valid_until IS NULL OR ur.valid_until>=?) ORDER BY ur.id,rp.permission_code,sr.id`, tenantID, userID, date, date)
		if err != nil {
			return nil, nil, err
		}
		type roleBundle struct {
			role   Role
			grants map[string]*Grant
			rules  map[string]*ScopeRule
		}
		bundles := map[string]*roleBundle{}
		for rows.Next() {
			var assignment, roleCode, permissionCode string
			var from time.Time
			var until sql.NullTime
			var ruleID, kind, dimension, node sql.NullString
			if err := rows.Scan(&assignment, &roleCode, &from, &until, &permissionCode, &ruleID, &kind, &dimension, &node); err != nil {
				rows.Close()
				return nil, nil, err
			}
			bundle := bundles[assignment]
			if bundle == nil {
				bundle = &roleBundle{role: Role{TenantID: tenantID, Code: roleCode, ValidFrom: from}, grants: map[string]*Grant{}, rules: map[string]*ScopeRule{}}
				if until.Valid {
					bundle.role.ValidUntil = &until.Time
				}
				bundles[assignment] = bundle
			}
			grant := bundle.grants[permissionCode]
			if grant == nil {
				grant = &Grant{Permission: permissionCode}
				bundle.grants[permissionCode] = grant
			}
			if ruleID.Valid {
				rule := bundle.rules[ruleID.String]
				if rule == nil {
					rule = &ScopeRule{AllTenant: kind.String == "all_tenant", Self: kind.String == "self", DirectReports: kind.String == "direct_reports"}
					bundle.rules[ruleID.String] = rule
					grant.Scopes = append(grant.Scopes, *rule)
				}
				if dimension.Valid && node.Valid {
					appendDimension(&grant.Scopes[len(grant.Scopes)-1], dimension.String, node.String)
				}
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, nil, err
		}
		rows.Close()
		for _, bundle := range bundles {
			for _, grant := range bundle.grants {
				bundle.role.Grants = append(bundle.role.Grants, *grant)
			}
			subject.Roles = append(subject.Roles, bundle.role)
		}
		rows, err = s.DB.QueryContext(ctx, `SELECT permission_code,decision FROM user_permission_overrides WHERE tenant_id=? AND user_id=?`, tenantID, userID)
		if err != nil {
			return nil, nil, err
		}
		for rows.Next() {
			var code, decision string
			if err := rows.Scan(&code, &decision); err != nil {
				rows.Close()
				return nil, nil, err
			}
			subject.Overrides[code] = Override{Decision: DecisionKind(decision)}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, nil, err
		}
		rows.Close()
		rows, err = s.DB.QueryContext(ctx, `SELECT h.permission_code,r.id,r.kind,v.dimension,v.node_id FROM user_scope_overrides h LEFT JOIN user_scope_rules r ON r.tenant_id=h.tenant_id AND r.user_id=h.user_id AND r.permission_code=h.permission_code LEFT JOIN user_scope_values v ON v.tenant_id=r.tenant_id AND v.rule_id=r.id WHERE h.tenant_id=? AND h.user_id=? ORDER BY h.permission_code,r.id`, tenantID, userID)
		if err != nil {
			return nil, nil, err
		}
		userRules := map[string]map[string]int{}
		for rows.Next() {
			var code string
			var id, kind, dimension, node sql.NullString
			if err := rows.Scan(&code, &id, &kind, &dimension, &node); err != nil {
				rows.Close()
				return nil, nil, err
			}
			override := subject.Overrides[code]
			if override.Scopes == nil {
				empty := []ScopeRule{}
				override.Scopes = &empty
			}
			if id.Valid {
				if userRules[code] == nil {
					userRules[code] = map[string]int{}
				}
				idx, exists := userRules[code][id.String]
				if !exists {
					idx = len(*override.Scopes)
					userRules[code][id.String] = idx
					*override.Scopes = append(*override.Scopes, ScopeRule{AllTenant: kind.String == "all_tenant", Self: kind.String == "self", DirectReports: kind.String == "direct_reports"})
				}
				if dimension.Valid && node.Valid {
					appendDimension(&(*override.Scopes)[idx], dimension.String, node.String)
				}
			}
			subject.Overrides[code] = override
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, nil, err
		}
		rows.Close()
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.UTC
	}
	now := time.Now().In(location)
	result := map[string]Effective{}
	for code := range registry {
		result[code] = Evaluate(now, tenantID, subject, registry, code)
	}
	return result, subject.Entitlements, nil
}

func appendDimension(rule *ScopeRule, dimension, node string) {
	switch dimension {
	case "region":
		rule.Regions = append(rule.Regions, node)
	case "office":
		rule.Offices = append(rule.Offices, node)
	case "division":
		rule.Divisions = append(rule.Divisions, node)
	case "department":
		rule.Departments = append(rule.Departments, node)
	case "position":
		rule.Positions = append(rule.Positions, node)
	}
}
