package access

import (
	"testing"
	"time"
)

func TestEffectivePermissionAndScope(t *testing.T) {
	day := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	registry := Registry{
		"absensi.lihat": {Code: "absensi.lihat", Entitlement: "absensi", RequiresScope: true},
	}
	subject := Subject{
		TenantID: "tenant-a", UserID: "u-1", EmployeeID: "e-1", MemberActive: true,
		Entitlements: map[string]bool{"absensi": true},
		Roles: []Role{
			{TenantID: "tenant-a", Code: "KARYAWAN", Grants: []Grant{{Permission: "absensi.lihat", Scopes: []ScopeRule{{Self: true}}}}},
			{TenantID: "tenant-a", Code: "SPV", ValidFrom: day, Grants: []Grant{{Permission: "absensi.lihat", Scopes: []ScopeRule{{DirectReports: true}}}}},
		},
	}
	effective := Evaluate(day, "tenant-a", subject, registry, "absensi.lihat")
	if !effective.Allowed || len(effective.Scopes) != 2 {
		t.Fatalf("expected union of own and direct reports, got %+v", effective)
	}
	for _, target := range []Target{
		{TenantID: "tenant-a", UserID: "u-1"},
		{TenantID: "tenant-a", UserID: "u-2", ReportsToID: "e-1"},
	} {
		if !effective.Includes(subject, target) {
			t.Fatalf("expected target %+v to be included", target)
		}
	}
	for _, target := range []Target{
		{TenantID: "tenant-b", UserID: "u-1"},
		{TenantID: "tenant-a", UserID: "u-3", ReportsToID: "u-9"},
	} {
		if effective.Includes(subject, target) {
			t.Fatalf("expected target %+v to be excluded", target)
		}
	}
	if Evaluate(day, "tenant-b", subject, registry, "absensi.lihat").Allowed {
		t.Fatal("cross-tenant membership was accepted")
	}
	if Evaluate(day, "tenant-a", subject, registry, "unknown.lihat").Allowed {
		t.Fatal("unknown permission was accepted")
	}
	subject.Roles[0].TenantID = "tenant-b"
	subject.Roles[1].TenantID = "tenant-b"
	if Evaluate(day, "tenant-a", subject, registry, "absensi.lihat").Allowed {
		t.Fatal("role from another tenant was accepted")
	}
	subject.Roles[0].TenantID = "tenant-a"
	subject.Roles[1].TenantID = "tenant-a"

	subject.Overrides = map[string]Override{"absensi.lihat": {Decision: Deny}}
	if Evaluate(day, "tenant-a", subject, registry, "absensi.lihat").Allowed {
		t.Fatal("user deny did not override roles")
	}
	delete(subject.Overrides, "absensi.lihat")
	subject.Entitlements["absensi"] = false
	if Evaluate(day, "tenant-a", subject, registry, "absensi.lihat").Allowed {
		t.Fatal("missing entitlement was accepted")
	}
}

func TestScopeOverrideReplacesRoleScope(t *testing.T) {
	day := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	registry := Registry{"karyawan.lihat": {Code: "karyawan.lihat", RequiresScope: true}}
	newScopes := []ScopeRule{{Offices: []string{"office-1", "office-2"}, Divisions: []string{"division-1"}}}
	subject := Subject{
		TenantID: "t", UserID: "u", MemberActive: true,
		Roles:     []Role{{TenantID: "t", Code: "HR", Grants: []Grant{{Permission: "karyawan.lihat", Scopes: []ScopeRule{{AllTenant: true}}}}}},
		Overrides: map[string]Override{"karyawan.lihat": {Decision: Inherit, Scopes: &newScopes}},
	}
	effective := Evaluate(day, "t", subject, registry, "karyawan.lihat")
	if !effective.Allowed || len(effective.Scopes) != 1 {
		t.Fatalf("expected one replacement rule, got %+v", effective)
	}
	if !effective.Includes(subject, Target{TenantID: "t", BaseOfficeID: "office-2", DivisionID: "division-1"}) {
		t.Fatal("OR values in office dimension should match")
	}
	if effective.Includes(subject, Target{TenantID: "t", BaseOfficeID: "office-2", DivisionID: "division-2"}) {
		t.Fatal("AND between dimensions was not enforced")
	}
	empty := []ScopeRule{}
	subject.Overrides["karyawan.lihat"] = Override{Decision: Allow, Scopes: &empty}
	if Evaluate(day, "t", subject, registry, "karyawan.lihat").Allowed {
		t.Fatal("empty scope override must fail closed")
	}
}

func TestRoleDateBoundariesAndAllowWithoutScope(t *testing.T) {
	start := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	registry := Registry{"cuti.approve": {Code: "cuti.approve", RequiresScope: true}}
	subject := Subject{
		TenantID: "t", UserID: "u", MemberActive: true,
		Roles: []Role{{TenantID: "t", Code: "SPV", ValidFrom: start, ValidUntil: &end, Grants: []Grant{{Permission: "cuti.approve", Scopes: []ScopeRule{{DirectReports: true}}}}}},
	}
	if Evaluate(start.AddDate(0, 0, -1), "t", subject, registry, "cuti.approve").Allowed {
		t.Fatal("future role was active")
	}
	if !Evaluate(end, "t", subject, registry, "cuti.approve").Allowed {
		t.Fatal("end date should be inclusive")
	}
	if Evaluate(end.AddDate(0, 0, 1), "t", subject, registry, "cuti.approve").Allowed {
		t.Fatal("expired role was active")
	}
	subject.Roles = nil
	subject.Overrides = map[string]Override{"cuti.approve": {Decision: Allow}}
	if Evaluate(start, "t", subject, registry, "cuti.approve").Allowed {
		t.Fatal("user allow without scope should fail closed")
	}
}

func TestInvalidScopeFailsClosed(t *testing.T) {
	registry := Registry{"karyawan.lihat": {Code: "karyawan.lihat", RequiresScope: true}}
	subject := Subject{
		TenantID: "t", UserID: "u", MemberActive: true,
		Roles: []Role{{TenantID: "t", Code: "HR", Grants: []Grant{{Permission: "karyawan.lihat", Scopes: []ScopeRule{{AllTenant: true, Offices: []string{"office-1"}}}}}}},
	}
	result := Evaluate(time.Now(), "t", subject, registry, "karyawan.lihat")
	if result.Allowed || result.Reason != "scope_invalid" {
		t.Fatalf("mixed all-tenant and office scope should fail closed: %+v", result)
	}
}
