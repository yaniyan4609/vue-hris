package httpapi

import "testing"

func TestShiftWindowsAndPinValidation(t *testing.T) {
	target := 480
	item := shiftTemplate{Code: "SHIFT_A", Name: "Shift pagi", StartTime: "09:00", EndTime: "17:00", ClockInWindowStartTime: "08:00", ClockInWindowEndTime: "10:00", ClockOutWindowStartTime: "16:00", ClockOutWindowEndTime: "19:00", TargetWorkMinutes: &target, IsActive: true}
	if !validShift(&item) {
		t.Fatal("valid shift rejected")
	}
	if item.StartTime != "09:00:00" {
		t.Fatalf("normalized time: %q", item.StartTime)
	}
	bad := item
	bad.ClockInWindowEndTime = "07:00"
	if validShift(&bad) {
		t.Fatal("reversed clock-in window accepted")
	}
	bad = item
	bad.CrossesMidnight = true
	if validShift(&bad) {
		t.Fatal("inconsistent cross-midnight flag accepted")
	}
	bad = item
	bad.IsPinned = true
	if validShift(&bad) {
		t.Fatal("pinned shift without order accepted")
	}
	bad = item
	bad.ColorHex = "#xyz"
	if validShift(&bad) {
		t.Fatal("invalid color accepted")
	}
}
