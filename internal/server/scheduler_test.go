package server

import (
	"testing"
	"time"

	"onvif-visor/internal/store"
)

func TestScheduledWindowEndSameDay(t *testing.T) {
	loc, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 14, 10, 30, 0, 0, loc) // Monday
	cam := store.Camera{
		ScheduleEnabled:  true,
		ScheduleDays:     []int{int(time.Monday)},
		ScheduleStart:    "09:00",
		ScheduleEnd:      "12:00",
		ScheduleTimezone: "America/Mexico_City",
	}
	end, active := scheduledWindowEnd(cam, now)
	if !active {
		t.Fatal("schedule should be active")
	}
	want := time.Date(2026, time.September, 14, 12, 0, 0, 0, loc)
	if !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}
}

func TestScheduledWindowEndOvernight(t *testing.T) {
	loc, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 15, 1, 0, 0, 0, loc) // Tuesday, Monday schedule still active
	cam := store.Camera{
		ScheduleEnabled:  true,
		ScheduleDays:     []int{int(time.Monday)},
		ScheduleStart:    "22:00",
		ScheduleEnd:      "06:00",
		ScheduleTimezone: "America/Mexico_City",
	}
	end, active := scheduledWindowEnd(cam, now)
	if !active {
		t.Fatal("overnight schedule should be active")
	}
	want := time.Date(2026, time.September, 15, 6, 0, 0, 0, loc)
	if !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}
}

func TestContinuousScheduleAndDailyWindow(t *testing.T) {
	loc, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		t.Fatal(err)
	}
	cam := store.Camera{
		ScheduleEnabled: true, ScheduleDays: []int{0, 1, 2, 3, 4, 5, 6},
		ScheduleStart: "00:00", ScheduleEnd: "00:00", ScheduleTimezone: "America/Mexico_City",
	}
	if !continuousSchedule(cam) {
		t.Fatal("expected 24/7 schedule to be continuous")
	}
	now := time.Date(2026, time.September, 22, 14, 30, 0, 0, loc)
	end, active := scheduledWindowEnd(cam, now)
	if !active {
		t.Fatal("continuous schedule should be active")
	}
	want := time.Date(2026, time.September, 23, 0, 0, 0, 0, loc)
	if !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}
	if !sameScheduleDate(now.Add(-2*time.Hour), now, loc) {
		t.Fatal("times on the same local day should match")
	}
	if sameScheduleDate(now.Add(-24*time.Hour), now, loc) {
		t.Fatal("times on different local days should not match")
	}
}

func TestScheduledWindowInactiveOutsideSelectedDay(t *testing.T) {
	now := time.Date(2026, time.September, 15, 10, 0, 0, 0, time.UTC) // Tuesday
	cam := store.Camera{
		ScheduleEnabled: true,
		ScheduleDays:    []int{int(time.Monday)},
		ScheduleStart:   "09:00",
		ScheduleEnd:     "12:00",
	}
	if _, active := scheduledWindowEnd(cam, now); active {
		t.Fatal("schedule should be inactive")
	}
}
