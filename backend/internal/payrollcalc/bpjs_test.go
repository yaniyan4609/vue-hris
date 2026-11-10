package payrollcalc

import (
	"errors"
	"testing"
)

func TestMonthlyBPJSFivePrograms(t *testing.T) {
	// Perpres 64/2020 sets Kesehatan PPU 1% employee, 4% employer,
	// Rp12m cap. BPJS Ketenagakerjaan publishes JHT/JP/JKK/JKM rates;
	// JKK 0.54% is its low-risk example. Values remain test fixtures only.
	inputs := []BPJSInput{
		{Program: "KESEHATAN", WageCents: 600_000_000, WageCapCents: 1_200_000_000, EmployeeRatePPM: 10_000, EmployerRatePPM: 40_000},
		{Program: "JHT", WageCents: 600_000_000, EmployeeRatePPM: 20_000, EmployerRatePPM: 37_000},
		{Program: "JKK", WageCents: 600_000_000, EmployerRatePPM: 5_400},
		{Program: "JKM", WageCents: 600_000_000, EmployerRatePPM: 3_000},
		{Program: "JP", WageCents: 600_000_000, EmployeeRatePPM: 10_000, EmployerRatePPM: 20_000},
	}
	got, err := MonthlyBPJS(inputs)
	if err != nil || len(got.Lines) != 5 || got.TotalEmployeeCents != 24_000_000 || got.TotalEmployerCents != 63_240_000 {
		t.Fatalf("five-program contribution mismatch: %+v, %v", got, err)
	}
	inputs[0].WageCents = 2_000_000_000
	got, err = MonthlyBPJS(inputs)
	if err != nil || got.Lines[0].BaseCents != 1_200_000_000 || got.Lines[0].EmployeeCents != 12_000_000 || got.Lines[0].EmployerCents != 48_000_000 {
		t.Fatalf("health wage cap mismatch: %+v, %v", got, err)
	}
}

func TestMonthlyBPJSRejectsIncompleteOrInvalidRules(t *testing.T) {
	complete := []BPJSInput{{Program: "KESEHATAN"}, {Program: "JHT"}, {Program: "JKK"}, {Program: "JKM"}, {Program: "JP"}}
	for _, invalid := range [][]BPJSInput{
		complete[:4],
		{{Program: "KESEHATAN"}, {Program: "JHT"}, {Program: "JKK"}, {Program: "JKM"}, {Program: "JHT"}},
		{{Program: "KESEHATAN"}, {Program: "JHT"}, {Program: "JKK"}, {Program: "JKM"}, {Program: "OTHER"}},
	} {
		if _, err := MonthlyBPJS(invalid); !errors.Is(err, ErrInvalidBPJSInput) {
			t.Fatalf("invalid BPJS rules accepted: %+v", invalid)
		}
	}
	complete[0].EmployeeRatePPM = 1_000_001
	if _, err := MonthlyBPJS(complete); !errors.Is(err, ErrInvalidBPJSInput) {
		t.Fatalf("rate above 100%% accepted: %v", err)
	}
}
