// Package access evaluates tenant permissions and scope rules without depending
// on HTTP or storage. Persistence and SQL scope filters are added in phase 1.
package access

import "time"

type DecisionKind string

const (
	Inherit DecisionKind = "inherit"
	Allow   DecisionKind = "allow"
	Deny    DecisionKind = "deny"
)

type Permission struct {
	Code          string
	Entitlement   string
	RequiresScope bool
}

type Registry map[string]Permission

type ScopeRule struct {
	AllTenant     bool
	Self          bool
	DirectReports bool
	Regions       []string
	Offices       []string
	Divisions     []string
	Departments   []string
	Positions     []string
}

type Grant struct {
	Permission string
	Scopes     []ScopeRule
}

type Role struct {
	Code       string
	ValidFrom  time.Time
	ValidUntil *time.Time // inclusive calendar date; nil means no end date
	Grants     []Grant
}

type Override struct {
	Decision DecisionKind
	Scopes   *[]ScopeRule // nil inherits role scope; non-nil replaces it
}

type Subject struct {
	TenantID     string
	UserID       string
	MemberActive bool
	Roles        []Role
	Overrides    map[string]Override
	Entitlements map[string]bool
}

type Effective struct {
	Allowed bool
	Reason  string
	Sources []string
	Scopes  []ScopeRule // rules are OR; dimensions inside each rule are AND
}

// Evaluate returns a permission decision for one verified tenant membership.
// Unknown permissions and missing scope fail closed. A user allow still needs a
// scope for permissions tied to employee data.
func Evaluate(now time.Time, tenantID string, subject Subject, registry Registry, code string) Effective {
	if tenantID == "" || subject.TenantID != tenantID || !subject.MemberActive || subject.UserID == "" {
		return Effective{Reason: "tenant_or_membership_invalid"}
	}
	permission, known := registry[code]
	if !known || code == "" || permission.Code != code {
		return Effective{Reason: "permission_unknown"}
	}
	if permission.Entitlement != "" && !subject.Entitlements[permission.Entitlement] {
		return Effective{Reason: "entitlement_missing"}
	}

	override := subject.Overrides[permission.Code]
	if override.Decision != "" && override.Decision != Inherit && override.Decision != Allow && override.Decision != Deny {
		return Effective{Reason: "override_invalid"}
	}
	if override.Decision == Deny {
		return Effective{Reason: "user_deny"}
	}

	result := Effective{}
	for _, role := range subject.Roles {
		if !roleActive(now, role) {
			continue
		}
		for _, grant := range role.Grants {
			if grant.Permission != permission.Code {
				continue
			}
			result.Allowed = true
			result.Sources = append(result.Sources, "role:"+role.Code)
			result.Scopes = append(result.Scopes, grant.Scopes...)
		}
	}
	if override.Decision == Allow {
		result.Allowed = true
		result.Sources = append(result.Sources, "user:allow")
	}
	if !result.Allowed {
		return Effective{Reason: "permission_missing"}
	}
	if override.Scopes != nil {
		result.Scopes = append([]ScopeRule(nil), (*override.Scopes)...)
		result.Sources = append(result.Sources, "user:scope_override")
	}
	if permission.RequiresScope && len(result.Scopes) == 0 {
		return Effective{Reason: "scope_missing"}
	}
	for _, rule := range result.Scopes {
		if !rule.valid() {
			return Effective{Reason: "scope_invalid"}
		}
	}
	result.Reason = "allowed"
	return result
}

func roleActive(now time.Time, role Role) bool {
	date := now.Format("2006-01-02")
	if !role.ValidFrom.IsZero() && date < role.ValidFrom.Format("2006-01-02") {
		return false
	}
	return role.ValidUntil == nil || date <= role.ValidUntil.Format("2006-01-02")
}

type Target struct {
	TenantID     string
	EmployeeID   string
	UserID       string
	ReportsToID  string
	RegionID     string
	BaseOfficeID string
	DivisionID   string
	DepartmentID string
	PositionID   string
}

// Includes checks one target. Collection endpoints must translate the same
// rules into tenant-bound SQL instead of filtering rows in application memory.
func (e Effective) Includes(subject Subject, target Target) bool {
	if !e.Allowed || subject.TenantID == "" || target.TenantID != subject.TenantID {
		return false
	}
	for _, rule := range e.Scopes {
		if rule.matches(subject, target) {
			return true
		}
	}
	return false
}

func (rule ScopeRule) matches(subject Subject, target Target) bool {
	if rule.AllTenant {
		return true
	}
	if rule.Self || rule.DirectReports {
		return (rule.Self && target.UserID == subject.UserID) ||
			(rule.DirectReports && target.ReportsToID == subject.UserID)
	}
	return matchesDimension(rule.Regions, target.RegionID) &&
		matchesDimension(rule.Offices, target.BaseOfficeID) &&
		matchesDimension(rule.Divisions, target.DivisionID) &&
		matchesDimension(rule.Departments, target.DepartmentID) &&
		matchesDimension(rule.Positions, target.PositionID)
}

func (rule ScopeRule) valid() bool {
	dimensions := [][]string{rule.Regions, rule.Offices, rule.Divisions, rule.Departments, rule.Positions}
	specials := 0
	if rule.AllTenant {
		specials++
	}
	if rule.Self {
		specials++
	}
	if rule.DirectReports {
		specials++
	}
	for _, values := range dimensions {
		if len(values) > 0 && specials > 0 {
			return false
		}
		for _, value := range values {
			if value == "" {
				return false
			}
		}
	}
	if specials > 1 {
		return false
	}
	if specials == 1 {
		return true
	}
	for _, values := range dimensions {
		if len(values) > 0 {
			return true
		}
	}
	return false
}

func matchesDimension(allowed []string, actual string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, id := range allowed {
		if id != "" && id == actual {
			return true
		}
	}
	return false
}
