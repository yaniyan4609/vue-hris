package httpapi

import (
	"encoding/json"
	"testing"
)

func TestValidateStructuredPayrollTaxRule(t *testing.T) {
	in := payrollTaxRuleInput{
		EffectiveFrom:   "2099-01-01",
		SourceReference: "QA only",
		PTKP:            []payrollTaxPTKP{{Status: "TK/0", TERCategory: "A", AnnualAmount: json.Number("54000000")}},
		TERBrackets: []payrollTaxBracket{
			{Category: "A", LowerBound: json.Number("0"), UpperBoundExclusive: numberPtr("5000000"), Rate: json.Number("0")},
			{Category: "A", LowerBound: json.Number("5000000"), Rate: json.Number("0.05")},
			{Category: "B", LowerBound: json.Number("0"), Rate: json.Number("0.05")},
			{Category: "C", LowerBound: json.Number("0"), Rate: json.Number("0.05")},
		},
		AnnualBrackets: []payrollTaxBracket{{LowerBound: json.Number("0"), Rate: json.Number("0.05")}},
	}
	if !validatePayrollTaxRule(&in) {
		t.Fatal("valid complete rule rejected")
	}
	in.TERBrackets[1].LowerBound = json.Number("5000001")
	if validatePayrollTaxRule(&in) {
		t.Fatal("gap in TER brackets accepted")
	}
	in.TERBrackets[1].LowerBound = json.Number("5000000")
	in.TERBrackets[1].Rate = json.Number("1.000001")
	if validatePayrollTaxRule(&in) {
		t.Fatal("rate over 100 percent accepted")
	}
}

func numberPtr(raw string) *json.Number {
	value := json.Number(raw)
	return &value
}
