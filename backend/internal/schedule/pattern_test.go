package schedule

import (
	"testing"
	"time"
)

func TestMonthlySixthRowRepeatsWeekFive(t *testing.T) {
	date := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	got, err := SlotForDate("monthly", date)
	if err != nil || got != (Slot{WeekNo: 5, Weekday: 1}) {
		t.Fatalf("August 31 slot = %+v, %v", got, err)
	}
	first, err := SlotForDate("monthly", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || first != (Slot{WeekNo: 1, Weekday: 6}) {
		t.Fatalf("August 1 slot = %+v, %v", first, err)
	}
	weekly, err := SlotForDate("weekly", date)
	if err != nil || weekly != (Slot{WeekNo: 1, Weekday: 1}) {
		t.Fatalf("weekly slot = %+v, %v", weekly, err)
	}
}

func TestAttendanceWindowIsSeparateFromLateTolerance(t *testing.T) {
	date := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	got, err := MaterializeTimes(date, "Asia/Jakarta", ShiftTimes{
		Start: "09:00", End: "17:00",
		ClockInStart: LocalBound{Clock: "08:00"}, ClockInEnd: LocalBound{Clock: "09:30"},
		ClockOutStart: LocalBound{Clock: "16:30"}, ClockOutEnd: LocalBound{Clock: "19:00"},
	})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 5, 2, 20, 0, 0, time.UTC) // 09:20 Jakarta
	if !got.AllowsClockIn(at) || got.LateMinutes(at, 15) != 20 {
		t.Fatalf("09:20 should be accepted but late: %+v", got)
	}
	if got.AllowsClockIn(at.Add(11 * time.Minute)) {
		t.Fatal("09:31 must fall outside the clock-in window")
	}
	if !got.AllowsClockIn(got.ClockInStart) || !got.AllowsClockIn(got.ClockInEnd) {
		t.Fatal("window boundaries should be inclusive")
	}
}

func TestOvernightShiftUsesStartWorkDate(t *testing.T) {
	got, err := MaterializeTimes(time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC), "Asia/Jakarta", ShiftTimes{
		Start: "22:00", End: "06:00", CrossesMidnight: true,
		ClockInStart: LocalBound{Clock: "21:30"}, ClockInEnd: LocalBound{Clock: "22:30"},
		ClockOutStart: LocalBound{Clock: "05:30", DayOffset: 1}, ClockOutEnd: LocalBound{Clock: "07:00", DayOffset: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ExpectedStart.Format(time.RFC3339) != "2026-11-05T15:00:00Z" || got.ExpectedEnd.Format(time.RFC3339) != "2026-11-05T23:00:00Z" {
		t.Fatalf("overnight UTC window = %+v", got)
	}
	if !got.AllowsClockOut(time.Date(2026, 11, 5, 23, 0, 0, 0, time.UTC)) {
		t.Fatal("06:00 next local day should belong to the shift")
	}
}
