package server

import (
	"time"
	_ "time/tzdata"

	onvifsvc "onvif-visor/internal/onvif"
	"onvif-visor/internal/store"
)

const schedulePollInterval = 2 * time.Second

func (a *App) startScheduler() {
	go func() {
		a.applySchedules(time.Now())
		ticker := time.NewTicker(schedulePollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case now := <-ticker.C:
				a.applySchedules(now)
			}
		}
	}()
}

func (a *App) applySchedules(now time.Time) {
	for _, view := range a.store.List() {
		if !view.ScheduleEnabled {
			continue
		}
		cam, err := a.store.Get(view.ID)
		if err != nil {
			continue
		}
		windowEnd, active := scheduledWindowEnd(cam, now)
		if !active {
			continue
		}

		status := a.media.Status(cam.ID)
		if status.Active && continuousSchedule(cam) && !sameScheduleDate(status.StartedAt, now, scheduleLocation(cam)) {
			a.media.Stop(cam.ID)
			status = a.media.Status(cam.ID)
		}
		if status.Active && !status.StopAfter.Before(windowEnd.Add(-2*time.Second)) {
			continue
		}

		streamURI, err := onvifsvc.CredentialedRTSP(cam.RTSPURI, cam.Username, cam.Password)
		if err != nil {
			logCameraError(cam.ID, "schedule-uri", err)
			continue
		}
		duration := windowEnd.Sub(now)
		if duration <= 0 {
			continue
		}
		if _, err := a.media.Trigger(cam.ID, streamURI, duration); err != nil {
			logCameraError(cam.ID, "schedule-record", err)
		}
	}
}

func continuousSchedule(cam store.Camera) bool {
	if !cam.ScheduleEnabled || cam.ScheduleStart != "00:00" || cam.ScheduleEnd != "00:00" || len(cam.ScheduleDays) != 7 {
		return false
	}
	seen := make(map[int]bool, 7)
	for _, day := range cam.ScheduleDays {
		if day < int(time.Sunday) || day > int(time.Saturday) {
			return false
		}
		seen[day] = true
	}
	return len(seen) == 7
}

func scheduleLocation(cam store.Camera) *time.Location {
	location := time.Local
	if cam.ScheduleTimezone != "" && cam.ScheduleTimezone != "Local" {
		if loaded, err := time.LoadLocation(cam.ScheduleTimezone); err == nil {
			location = loaded
		}
	}
	return location
}

func sameScheduleDate(a, b time.Time, location *time.Location) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	a, b = a.In(location), b.In(location)
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func scheduledWindowEnd(cam store.Camera, now time.Time) (time.Time, bool) {
	if !cam.ScheduleEnabled || len(cam.ScheduleDays) == 0 || cam.ScheduleStart == "" || cam.ScheduleEnd == "" {
		return time.Time{}, false
	}

	location := scheduleLocation(cam)
	localNow := now.In(location)
	startHour, startMinute, ok := parseClock(cam.ScheduleStart)
	if !ok {
		return time.Time{}, false
	}
	endHour, endMinute, ok := parseClock(cam.ScheduleEnd)
	if !ok {
		return time.Time{}, false
	}

	selected := make(map[time.Weekday]bool, len(cam.ScheduleDays))
	for _, day := range cam.ScheduleDays {
		if day >= int(time.Sunday) && day <= int(time.Saturday) {
			selected[time.Weekday(day)] = true
		}
	}
	if len(selected) == 0 {
		return time.Time{}, false
	}

	for _, dayOffset := range []int{-1, 0} {
		candidateDate := localNow.AddDate(0, 0, dayOffset)
		if !selected[candidateDate.Weekday()] {
			continue
		}
		start := time.Date(candidateDate.Year(), candidateDate.Month(), candidateDate.Day(), startHour, startMinute, 0, 0, location)
		end := time.Date(candidateDate.Year(), candidateDate.Month(), candidateDate.Day(), endHour, endMinute, 0, 0, location)
		if !end.After(start) {
			end = end.AddDate(0, 0, 1)
		}
		if !localNow.Before(start) && localNow.Before(end) {
			return end, true
		}
	}
	return time.Time{}, false
}

func parseClock(value string) (int, int, bool) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, false
	}
	return parsed.Hour(), parsed.Minute(), true
}
