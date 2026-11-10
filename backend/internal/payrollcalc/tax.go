// Package payrollcalc contains deterministic payroll calculations. It has no
// database or HTTP dependencies and is not yet used to finalize payroll runs.
package payrollcalc

import (
	"errors"
	"math"
	"math/big"
)

const (
	rateScale      = int64(1_000_000)
	rupiahThousand = int64(100_000) // Amounts are stored in hundredths of a rupiah.
)

var ErrInvalidTaxInput = errors.New("invalid tax calculation input")

// Bracket boundaries are in hundredths of a rupiah. An absent upper bound is
// represented by zero and is only valid for the final bracket.
type Bracket struct {
	Category            string
	LowerBound          int64
	UpperBoundExclusive int64
	RatePPM             int64
}

func validBrackets(brackets []Bracket) bool {
	if len(brackets) == 0 {
		return false
	}
	var next int64
	for i, bracket := range brackets {
		if bracket.LowerBound != next || bracket.RatePPM < 0 || bracket.RatePPM > rateScale {
			return false
		}
		if i == len(brackets)-1 {
			return bracket.UpperBoundExclusive == 0
		}
		if bracket.UpperBoundExclusive <= bracket.LowerBound {
			return false
		}
		next = bracket.UpperBoundExclusive
	}
	return false
}

func multiplyRate(amount, rate int64) (int64, error) {
	value := new(big.Int).Mul(big.NewInt(amount), big.NewInt(rate))
	value.Add(value, big.NewInt(rateScale/2))
	value.Div(value, big.NewInt(rateScale))
	if !value.IsInt64() {
		return 0, ErrInvalidTaxInput
	}
	return value.Int64(), nil
}

// MonthlyTER calculates withholding for a non-final monthly tax period using
// tenant-supplied TER brackets for exactly one category.
func MonthlyTER(grossCents int64, brackets []Bracket) (int64, error) {
	if grossCents < 0 || !validBrackets(brackets) {
		return 0, ErrInvalidTaxInput
	}
	category := brackets[0].Category
	if category != "A" && category != "B" && category != "C" {
		return 0, ErrInvalidTaxInput
	}
	for _, bracket := range brackets {
		if bracket.Category != category {
			return 0, ErrInvalidTaxInput
		}
		if bracket.UpperBoundExclusive == 0 || grossCents < bracket.UpperBoundExclusive {
			return multiplyRate(grossCents, bracket.RatePPM)
		}
	}
	return 0, ErrInvalidTaxInput
}

type FullYearInput struct {
	GrossCents                   int64
	DeductibleContributionsCents int64
	PTKPCents                    int64
	PriorWithheldCents           int64
	AnnualBrackets               []Bracket
}

type FullYearResult struct {
	JobExpenseCents    int64
	TaxableIncomeCents int64
	AnnualTaxCents     int64
	FinalPeriodCents   int64 // Negative means prior periods withheld too much.
}

// FullYearFinal covers a permanent employee whose subjective tax obligation
// and employment at this employer both span the complete calendar year. The
// caller supplies all deductible employee contributions. Partial years,
// transfers and other employee classes require separate calculations.
func FullYearFinal(in FullYearInput) (FullYearResult, error) {
	if in.GrossCents < 0 || in.DeductibleContributionsCents < 0 || in.PTKPCents < 0 || in.PriorWithheldCents < 0 || !validBrackets(in.AnnualBrackets) {
		return FullYearResult{}, ErrInvalidTaxInput
	}
	for _, bracket := range in.AnnualBrackets {
		if bracket.Category != "" {
			return FullYearResult{}, ErrInvalidTaxInput
		}
	}
	jobExpense, err := multiplyRate(in.GrossCents, 50_000) // Five percent.
	if err != nil {
		return FullYearResult{}, err
	}
	if jobExpense > 600_000_000 { // Rp6,000,000 annual cap.
		jobExpense = 600_000_000
	}
	if in.DeductibleContributionsCents > in.GrossCents-jobExpense {
		return FullYearResult{}, ErrInvalidTaxInput
	}
	net := in.GrossCents - jobExpense - in.DeductibleContributionsCents
	taxable := int64(0)
	if net > in.PTKPCents {
		taxable = (net - in.PTKPCents) / rupiahThousand * rupiahThousand
	}
	var annualTax int64
	for _, bracket := range in.AnnualBrackets {
		if taxable <= bracket.LowerBound {
			break
		}
		upper := taxable
		if bracket.UpperBoundExclusive != 0 && upper > bracket.UpperBoundExclusive {
			upper = bracket.UpperBoundExclusive
		}
		part, err := multiplyRate(upper-bracket.LowerBound, bracket.RatePPM)
		if err != nil || part > math.MaxInt64-annualTax {
			return FullYearResult{}, ErrInvalidTaxInput
		}
		annualTax += part
	}
	return FullYearResult{JobExpenseCents: jobExpense, TaxableIncomeCents: taxable, AnnualTaxCents: annualTax, FinalPeriodCents: annualTax - in.PriorWithheldCents}, nil
}
