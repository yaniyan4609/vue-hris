package httpapi

import (
	"encoding/json"
	"testing"
)

func TestValidateBPJSWageBasis(t *testing.T) {
	data := bpjsWageBasisData{OfficeMinimums: []bpjsOfficeMinimum{{OfficeID: "office-a", Amount: json.Number("3000000")}, {OfficeID: "office-b", Amount: json.Number("4000000")}}}
	for _, program := range requiredPayrollBPJSPrograms {
		data.Programs = append(data.Programs, bpjsWageProgram{Program: program, ComponentIDs: []string{"salary", "fixed-allowance"}})
	}
	components := map[string]bool{"salary": true, "fixed-allowance": true}
	offices := map[string]bool{"office-a": true, "office-b": true}
	if !validateBPJSWageBasis(data, components, offices) {
		t.Fatal("complete wage basis rejected")
	}
	data.Programs[0].ComponentIDs = []string{"salary", "salary"}
	if validateBPJSWageBasis(data, components, offices) {
		t.Fatal("duplicate component accepted")
	}
	data.Programs[0].ComponentIDs = []string{"salary"}
	data.OfficeMinimums[1].Amount = json.Number("0")
	if validateBPJSWageBasis(data, components, offices) {
		t.Fatal("missing minimum wage accepted")
	}
	data.OfficeMinimums[1].Amount = json.Number("4000000")
	data.Programs[4].Program = "JHT"
	if validateBPJSWageBasis(data, components, offices) {
		t.Fatal("duplicate program accepted")
	}
}

func TestBPJSWageBasisJSONContainsPolicyFields(t *testing.T) {
	var input struct {
		EffectiveFrom string `json:"effective_from"`
		bpjsWageBasisData
	}
	if err := json.Unmarshal([]byte(`{"effective_from":"2027-01-01","programs":[{"program":"JHT","component_ids":["salary"]}],"office_minimums":[{"office_id":"office-a","amount":3000000}]}`), &input); err != nil {
		t.Fatal(err)
	}
	if len(input.Programs) != 1 || len(input.OfficeMinimums) != 1 || input.OfficeMinimums[0].Amount.String() != "3000000" {
		t.Fatalf("policy JSON did not decode: %+v", input)
	}
	encoded, err := json.Marshal(bpjsWageBasisView{bpjsWageBasisData: input.bpjsWageBasisData})
	if err != nil || !json.Valid(encoded) {
		t.Fatalf("policy JSON did not encode: %s, %v", encoded, err)
	}
}
