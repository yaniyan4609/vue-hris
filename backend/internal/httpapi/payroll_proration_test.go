package httpapi

import "testing"

func TestProrateFixedCentsUsesPublishedWorkDayFraction(t *testing.T) {
	got, ok := prorateFixedCents(100000, 10, 22)
	if !ok || got != 45455 {
		t.Fatalf("10/22 work days: got=%d valid=%t", got, ok)
	}
	got, ok = prorateFixedCents(100001, 22, 22)
	if !ok || got != 100001 {
		t.Fatalf("full month: got=%d valid=%t", got, ok)
	}
	if _, ok = prorateFixedCents(100000, 5, 0); ok {
		t.Fatal("zero work days accepted")
	}
}
