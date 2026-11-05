package access

import (
	"reflect"
	"strings"
	"testing"
)

func TestEmployeeWhereCombinesScopeWithoutLosingTenantFilter(t *testing.T) {
	subject := Subject{TenantID: "tenant-a", UserID: "user-a", EmployeeID: "employee-a"}
	decision := Effective{Allowed: true, Scopes: []ScopeRule{
		{Offices: []string{"office-a", "office-b"}, Departments: []string{"department-a"}},
		{DirectReports: true},
	}}
	where, args := EmployeeWhere(subject, decision)
	if !strings.Contains(where, "a.office_id IN (?,?) AND a.department_id IN (?)") || !strings.Contains(where, " OR a.reports_to_employee_id=?") {
		t.Fatalf("unexpected predicate: %s", where)
	}
	if !reflect.DeepEqual(args, []any{"office-a", "office-b", "department-a", "employee-a"}) {
		t.Fatalf("unexpected args: %#v", args)
	}
	if strings.Contains(where, "tenant-a") {
		t.Fatal("tenant must be bound separately")
	}
}

func TestEmployeeWhereFailsClosed(t *testing.T) {
	where, args := EmployeeWhere(Subject{}, Effective{Allowed: true})
	if where != "1=0" || len(args) != 0 {
		t.Fatalf("got %q %#v", where, args)
	}
}
