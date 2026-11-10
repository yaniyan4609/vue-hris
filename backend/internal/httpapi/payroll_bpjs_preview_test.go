package httpapi

import (
	"database/sql"
	"testing"

	"digaji.id/hris/internal/payrollcalc"
)

func TestResolveBPJSPreviewUsesProgramComponentsAndHealthMinimum(t *testing.T) {
	policy := bpjsWageBasisView{ID: "policy"}
	for _, program := range requiredPayrollBPJSPrograms {
		ids := []string{"salary"}
		if program == "KESEHATAN" {
			ids = append(ids, "allowance")
		}
		policy.Programs = append(policy.Programs, bpjsWageProgram{Program: program, ComponentIDs: ids})
	}
	components := map[string][]bpjsPreviewComponent{
		"salary":    {{ID: "salary", Amount: "2000000.00", From: "2026-01-01", Active: true, Type: "earning", CalcType: "fixed"}},
		"allowance": {{ID: "allowance", Amount: "1000000.00", From: "2026-01-01", Active: true, Type: "earning", CalcType: "fixed"}},
	}
	rules := map[string]bpjsPreviewRule{}
	for _, program := range requiredPayrollBPJSPrograms {
		rules[program] = bpjsPreviewRule{ID: "rule-" + program, Program: program, EmployeeRate: "0", EmployerRate: "0"}
	}
	rules["KESEHATAN"] = bpjsPreviewRule{ID: "health", Program: "KESEHATAN", EmployeeRate: "0.01", EmployerRate: "0.04", WageCap: sql.NullString{String: "12000000.00", Valid: true}}
	resolved := resolveBPJSPreviewInputs(policy, components, rules, 400_000_000, "2026-01-01", "2026-01-31")
	if len(resolved.Issues) != 0 || len(resolved.Inputs) != 5 || resolved.InputWages["KESEHATAN"] != 300_000_000 || resolved.Inputs[0].WageCents != 400_000_000 || resolved.Inputs[1].WageCents != 200_000_000 {
		t.Fatalf("wrong basis selection or minimum: %+v", resolved)
	}
	got, err := payrollcalc.MonthlyBPJS(resolved.Inputs)
	if err != nil || got.Lines[0].EmployeeCents != 4_000_000 || got.Lines[0].EmployerCents != 16_000_000 {
		t.Fatalf("wrong health contribution: %+v, %v", got, err)
	}
	components["salary"] = []bpjsPreviewComponent{{ID: "salary", Amount: "2000000.00", From: "2026-01-15", Active: true, Type: "earning", CalcType: "fixed"}}
	resolved = resolveBPJSPreviewInputs(policy, components, rules, 400_000_000, "2026-01-01", "2026-01-31")
	if !containsPayrollIssue(resolved.Issues, "BPJS_COMPONENT_INCOMPLETE") {
		t.Fatalf("mid-period salary change was accepted: %+v", resolved)
	}
}

func TestPayrollRatePPMExactDecimal(t *testing.T) {
	for raw, expected := range map[string]int64{"0": 0, "0.0054": 5_400, "0.037": 37_000, "1": 1_000_000} {
		actual, ok := payrollRatePPM(raw)
		if !ok || actual != expected {
			t.Fatalf("rate %s = %d, %t", raw, actual, ok)
		}
	}
	for _, raw := range []string{"-0.01", "0.0000001", "1.1"} {
		if _, ok := payrollRatePPM(raw); ok {
			t.Fatalf("invalid rate accepted: %s", raw)
		}
	}
}

func containsPayrollIssue(issues []string, code string) bool {
	for _, issue := range issues {
		if issue == code {
			return true
		}
	}
	return false
}
