package payrollcalc

import (
	"errors"
	"math"
)

var ErrInvalidBPJSInput = errors.New("invalid BPJS calculation input")

var requiredBPJSPrograms = []string{"KESEHATAN", "JHT", "JKK", "JKM", "JP"}

// BPJSInput is a single month's contribution basis and the tenant rule version
// selected for that period. WageCents must already reflect the program's wage
// definition; WageCapCents is zero when no cap applies.
type BPJSInput struct {
	Program         string
	WageCents       int64
	WageCapCents    int64
	EmployeeRatePPM int64
	EmployerRatePPM int64
}

type BPJSLine struct {
	Program       string
	BaseCents     int64
	EmployeeCents int64
	EmployerCents int64
}

type BPJSResult struct {
	Lines              []BPJSLine
	TotalEmployeeCents int64
	TotalEmployerCents int64
}

// MonthlyBPJS calculates the five configured programs without hardcoding any
// tenant rate or cap. It does not decide participation or a minimum wage base.
func MonthlyBPJS(inputs []BPJSInput) (BPJSResult, error) {
	if len(inputs) != len(requiredBPJSPrograms) {
		return BPJSResult{}, ErrInvalidBPJSInput
	}
	byProgram := make(map[string]BPJSInput, len(inputs))
	for _, input := range inputs {
		if _, exists := byProgram[input.Program]; exists || input.WageCents < 0 || input.WageCapCents < 0 || input.EmployeeRatePPM < 0 || input.EmployeeRatePPM > rateScale || input.EmployerRatePPM < 0 || input.EmployerRatePPM > rateScale {
			return BPJSResult{}, ErrInvalidBPJSInput
		}
		byProgram[input.Program] = input
	}
	result := BPJSResult{Lines: make([]BPJSLine, 0, len(inputs))}
	for _, program := range requiredBPJSPrograms {
		input, exists := byProgram[program]
		if !exists {
			return BPJSResult{}, ErrInvalidBPJSInput
		}
		base := input.WageCents
		if input.WageCapCents > 0 && base > input.WageCapCents {
			base = input.WageCapCents
		}
		employee, err := multiplyRate(base, input.EmployeeRatePPM)
		if err != nil {
			return BPJSResult{}, ErrInvalidBPJSInput
		}
		employer, err := multiplyRate(base, input.EmployerRatePPM)
		if err != nil || employee > math.MaxInt64-result.TotalEmployeeCents || employer > math.MaxInt64-result.TotalEmployerCents {
			return BPJSResult{}, ErrInvalidBPJSInput
		}
		result.Lines = append(result.Lines, BPJSLine{Program: program, BaseCents: base, EmployeeCents: employee, EmployerCents: employer})
		result.TotalEmployeeCents += employee
		result.TotalEmployerCents += employer
	}
	return result, nil
}
