package schedule

import (
	"errors"
	"fmt"
	"time"
)

// Slot identifies a reusable weekly/monthly pattern cell. Week 1 is the
// Monday-Sunday calendar row containing the first day of the month. The sixth
// calendar row repeats week 5, as agreed for monthly patterns.
type Slot struct {
	Weekday int
	WeekNo  int
}

func SlotForDate(patternType string, date time.Time) (Slot, error) {
	weekday := (int(date.Weekday())+6)%7 + 1 // Monday=1
	if patternType == "weekly" {
		return Slot{Weekday: weekday, WeekNo: 1}, nil
	}
	if patternType != "monthly" {
		return Slot{}, errors.New("unknown schedule pattern type")
	}
	first := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	firstOffset := (int(first.Weekday()) + 6) % 7
	weekNo := (firstOffset+date.Day()-1)/7 + 1
	if weekNo > 5 {
		weekNo = 5
	}
	return Slot{Weekday: weekday, WeekNo: weekNo}, nil
}

type LocalBound struct {
	Clock     string // HH:mm, local to the roster office
	DayOffset int    // relative to work_date
}

type ShiftTimes struct {
	Start           string
	End             string
	CrossesMidnight bool
	ClockInStart    LocalBound
	ClockInEnd      LocalBound
	ClockOutStart   LocalBound
	ClockOutEnd     LocalBound
}

type InstanceTimes struct {
	ExpectedStart time.Time
	ExpectedEnd   time.Time
	ClockInStart  time.Time
	ClockInEnd    time.Time
	ClockOutStart time.Time
	ClockOutEnd   time.Time
}

// MaterializeTimes converts office-local shift rules into UTC instants for an
// immutable shift instance. workDate is the local date on which the shift starts.
func MaterializeTimes(workDate time.Time, timezone string, shift ShiftTimes) (InstanceTimes, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return InstanceTimes{}, fmt.Errorf("invalid office timezone: %w", err)
	}
	date := time.Date(workDate.Year(), workDate.Month(), workDate.Day(), 0, 0, 0, 0, location)
	bound := func(value LocalBound) (time.Time, error) {
		if value.DayOffset < -1 || value.DayOffset > 2 {
			return time.Time{}, errors.New("day offset outside -1..2")
		}
		parsed, err := time.Parse("15:04", value.Clock)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid clock %q: %w", value.Clock, err)
		}
		day := date.AddDate(0, 0, value.DayOffset)
		return time.Date(day.Year(), day.Month(), day.Day(), parsed.Hour(), parsed.Minute(), 0, 0, location).UTC(), nil
	}
	start, err := bound(LocalBound{Clock: shift.Start})
	if err != nil {
		return InstanceTimes{}, err
	}
	endOffset := 0
	if shift.CrossesMidnight {
		endOffset = 1
	}
	end, err := bound(LocalBound{Clock: shift.End, DayOffset: endOffset})
	if err != nil {
		return InstanceTimes{}, err
	}
	inStart, err := bound(shift.ClockInStart)
	if err != nil {
		return InstanceTimes{}, err
	}
	inEnd, err := bound(shift.ClockInEnd)
	if err != nil {
		return InstanceTimes{}, err
	}
	outStart, err := bound(shift.ClockOutStart)
	if err != nil {
		return InstanceTimes{}, err
	}
	outEnd, err := bound(shift.ClockOutEnd)
	if err != nil {
		return InstanceTimes{}, err
	}
	if !end.After(start) || inEnd.Before(inStart) || outEnd.Before(outStart) {
		return InstanceTimes{}, errors.New("invalid shift/window order")
	}
	return InstanceTimes{ExpectedStart: start, ExpectedEnd: end, ClockInStart: inStart, ClockInEnd: inEnd, ClockOutStart: outStart, ClockOutEnd: outEnd}, nil
}

func (t InstanceTimes) AllowsClockIn(at time.Time) bool {
	return !at.Before(t.ClockInStart) && !at.After(t.ClockInEnd)
}

func (t InstanceTimes) AllowsClockOut(at time.Time) bool {
	return !at.Before(t.ClockOutStart) && !at.After(t.ClockOutEnd)
}

func (t InstanceTimes) LateMinutes(at time.Time, toleranceMinutes int) int {
	deadline := t.ExpectedStart.Add(time.Duration(toleranceMinutes) * time.Minute)
	if !at.After(deadline) {
		return 0
	}
	return int(at.Sub(t.ExpectedStart).Minutes())
}
