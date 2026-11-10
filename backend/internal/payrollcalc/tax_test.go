package payrollcalc

import (
	"errors"
	"testing"
)

// PP 58/2023, Pasal 2 explanation, Tuan R at PT ABC:
// https://www.pajak.go.id/id/peraturan/tarif-pemotongan-pajak-penghasiian-pasal-21-atas-penghasilan-sehubungan-dengan-pekerjaan
func TestOfficialTuanRFullYear(t *testing.T) {
	monthly, err := MonthlyTER(1_000_000_000, []Bracket{
		{Category: "A", LowerBound: 0, UpperBoundExclusive: 1_000_000_001, RatePPM: 20_000},
		{Category: "A", LowerBound: 1_000_000_001, RatePPM: 30_000},
	})
	if err != nil || monthly != 20_000_000 { // Rp200,000.
		t.Fatalf("monthly TER = %d, %v", monthly, err)
	}
	got, err := FullYearFinal(FullYearInput{
		GrossCents:                   12_000_000_000, // Rp120,000,000.
		DeductibleContributionsCents: 120_000_000,
		PTKPCents:                    5_850_000_000,
		PriorWithheldCents:           monthly * 11,
		AnnualBrackets: []Bracket{
			{LowerBound: 0, UpperBoundExclusive: 6_000_000_000, RatePPM: 50_000},
			{LowerBound: 6_000_000_000, RatePPM: 150_000},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.JobExpenseCents != 600_000_000 || got.TaxableIncomeCents != 5_430_000_000 || got.AnnualTaxCents != 271_500_000 || got.FinalPeriodCents != 51_500_000 {
		t.Fatalf("official Tuan R vector mismatch: %+v", got)
	}
}

func TestTaxBracketBoundaryAndThousandRupiahFloor(t *testing.T) {
	brackets := []Bracket{
		{Category: "A", LowerBound: 0, UpperBoundExclusive: 1_000_000_000, RatePPM: 0},
		{Category: "A", LowerBound: 1_000_000_000, RatePPM: 20_000},
	}
	below, err := MonthlyTER(999_999_999, brackets)
	if err != nil || below != 0 {
		t.Fatalf("below boundary = %d, %v", below, err)
	}
	at, err := MonthlyTER(1_000_000_000, brackets)
	if err != nil || at != 20_000_000 {
		t.Fatalf("at boundary = %d, %v", at, err)
	}
	// PMK 168/2023 requires annual taxable income rounded down to Rp1,000.
	got, err := FullYearFinal(FullYearInput{
		GrossCents:     5_000_000_000,
		PTKPCents:      0,
		AnnualBrackets: []Bracket{{RatePPM: 50_000}},
	})
	if err != nil || got.TaxableIncomeCents != 4_750_000_000 {
		t.Fatalf("full-year baseline = %+v, %v", got, err)
	}
	got, err = FullYearFinal(FullYearInput{
		GrossCents:     5_000_099_999,
		PTKPCents:      0,
		AnnualBrackets: []Bracket{{RatePPM: 50_000}},
	})
	if err != nil || got.TaxableIncomeCents != 4_750_000_000 {
		t.Fatalf("taxable income must be rounded down to Rp1,000: %+v, %v", got, err)
	}
}

func TestFullYearJobExpenseCapAndProgressiveBracket(t *testing.T) {
	got, err := FullYearFinal(FullYearInput{
		GrossCents: 20_000_000_000,
		AnnualBrackets: []Bracket{
			{UpperBoundExclusive: 6_000_000_000, RatePPM: 50_000},
			{LowerBound: 6_000_000_000, RatePPM: 150_000},
		},
	})
	if err != nil || got.JobExpenseCents != 600_000_000 || got.TaxableIncomeCents != 19_400_000_000 || got.AnnualTaxCents != 2_310_000_000 {
		t.Fatalf("annual cap or progressive bracket incorrect: %+v, %v", got, err)
	}
}

func TestYearEndOverWithholdingRemainsNegative(t *testing.T) {
	got, err := FullYearFinal(FullYearInput{
		GrossCents:         100_000_000,
		PTKPCents:          100_000_000,
		PriorWithheldCents: 10_000_000,
		AnnualBrackets:     []Bracket{{RatePPM: 50_000}},
	})
	if err != nil || got.FinalPeriodCents != -10_000_000 {
		t.Fatalf("refund credit lost: %+v, %v", got, err)
	}
}

func TestRejectInvalidTaxInputs(t *testing.T) {
	for _, brackets := range [][]Bracket{
		{{Category: "A", LowerBound: 1, RatePPM: 20_000}},
		{{Category: "A", UpperBoundExclusive: 100, RatePPM: 20_000}, {Category: "A", LowerBound: 101, RatePPM: 20_000}},
		{{Category: "A", RatePPM: 1_000_001}},
		{{Category: "A", UpperBoundExclusive: 100}, {Category: "B", LowerBound: 100}},
	} {
		if _, err := MonthlyTER(100, brackets); !errors.Is(err, ErrInvalidTaxInput) {
			t.Fatalf("invalid monthly brackets accepted: %+v", brackets)
		}
	}
	if _, err := FullYearFinal(FullYearInput{GrossCents: 1, DeductibleContributionsCents: 2, AnnualBrackets: []Bracket{{RatePPM: 50_000}}}); !errors.Is(err, ErrInvalidTaxInput) {
		t.Fatalf("deductions greater than gross accepted: %v", err)
	}
}
