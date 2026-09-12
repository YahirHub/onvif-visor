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
