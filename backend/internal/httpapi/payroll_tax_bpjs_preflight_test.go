package httpapi

import "testing"

func TestPayrollBPJSProgramCoverageRequiresMatchingProgramAndDate(t *testing.T) {
	rules := []payrollBPJSRuleStatus{
		{Program: "JHT", From: "2026-01-01", To: "2026-01-15"},
		{Program: "JHT", From: "2026-01-16", To: ""},
		{Program: "JP", From: "2026-01-01", To: ""},
	}
	for _, day := range []string{"2026-01-01", "2026-01-15", "2026-01-16", "2026-01-31"} {
		if !payrollBPJSProgramCovered(rules, "JHT", day) {
			t.Fatalf("JHT version missing on %s", day)
		}
	}
	if payrollBPJSProgramCovered(rules, "JHT", "2025-12-31") || payrollBPJSProgramCovered(rules, "JKK", "2026-01-15") {
		t.Fatal("uncovered day or different program treated as covered")
	}
}
