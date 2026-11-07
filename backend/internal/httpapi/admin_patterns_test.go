package httpapi

import "testing"

func TestPatternSlotsRequireCompleteGrid(t *testing.T) {
	weekly := make([]patternSlot, 0, 7)
	for day := 1; day <= 7; day++ {
		weekly = append(weekly, patternSlot{WeekNo: 1, Weekday: day, DayType: "off"})
	}
	if !validatePatternSlots("weekly", weekly) {
		t.Fatal("complete weekly pattern rejected")
	}
	if validatePatternSlots("weekly", weekly[:6]) {
		t.Fatal("incomplete weekly pattern accepted")
	}
	monthly := make([]patternSlot, 0, 35)
	for week := 1; week <= 5; week++ {
		for day := 1; day <= 7; day++ {
			monthly = append(monthly, patternSlot{WeekNo: week, Weekday: day, DayType: "off"})
		}
	}
	if !validatePatternSlots("monthly", monthly) {
		t.Fatal("complete monthly pattern rejected")
	}
	monthly[34] = monthly[0]
	if validatePatternSlots("monthly", monthly) {
		t.Fatal("duplicate cell accepted")
	}
}
