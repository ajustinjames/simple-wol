package main

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// Clock abstracts time.Now so the scheduler's firing logic can be driven by
// a fake clock in tests instead of relying on real wall-clock sleeps.
type Clock interface {
	Now() time.Time
}

// realClock returns the current server local time. All schedule times
// (hour/minute) are interpreted in time.Local - the time zone the server
// process is running in. Set the TZ environment variable (the Docker image
// already includes tzdata) to control which zone that is; otherwise it
// defaults to UTC in most minimal containers.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// dueWindow is how close to the scheduled minute "now" must fall for a
// schedule to be considered due. It exists so a scheduler tick that runs
// slightly late (e.g. at :00.9 instead of :00.0) still fires, while
// preventing a schedule that already fired this minute from firing again
// until the next matching minute comes around (enforced via LastFiredAt).
const dueWindow = 1 * time.Minute

// DueSchedules returns the schedules that should fire at the given time.
// A schedule is due when: it is enabled, its hour/minute/day-of-week match
// "now" (in now's own location), and it has not already fired during the
// current minute. This is a pure function so it can be unit tested without
// any goroutines, tickers, or real time passing.
func DueSchedules(now time.Time, schedules []Schedule) []Schedule {
	var due []Schedule
	dayBit := 1 << uint(now.Weekday())
	currentMinuteStart := now.Truncate(time.Minute)

	for _, s := range schedules {
		if !s.Enabled {
			continue
		}
		if s.DaysOfWeek&dayBit == 0 {
			continue
		}
		if s.Hour != now.Hour() || s.Minute != now.Minute() {
			continue
		}
		if s.LastFiredAt != nil && !s.LastFiredAt.Before(currentMinuteStart) {
			// Already fired within the current minute window.
			continue
		}
		due = append(due, s)
	}
	return due
}

// Scheduler runs the background loop that checks for due schedules and
// fires wakes for them.
type Scheduler struct {
	db     *sql.DB
	sender PacketSender
	clock  Clock
}

func NewScheduler(db *sql.DB, sender PacketSender) *Scheduler {
	return &Scheduler{db: db, sender: sender, clock: realClock{}}
}

// checkOnce evaluates all schedules against the current time and fires any
// that are due. It is exported as its own method (rather than inlined in
// the ticker loop) so tests can call it directly with a fake clock without
// waiting on a real ticker.
func (sch *Scheduler) checkOnce() {
	now := sch.clock.Now()

	schedules, err := ListAllSchedules(sch.db)
	if err != nil {
		slog.Error("scheduler: failed to list schedules", "error", err)
		return
	}

	due := DueSchedules(now, schedules)
	for _, s := range due {
		device, err := GetDevice(sch.db, s.DeviceID)
		if err != nil {
			slog.Error("scheduler: device not found for schedule", "schedule_id", s.ID, "device_id", s.DeviceID, "error", err)
			continue
		}

		if err := WakeDevice(sch.sender, device.MACAddress, "255.255.255.255", device.Port); err != nil {
			slog.Error("scheduler: failed to send WoL packet", "schedule_id", s.ID, "device", device.Name, "error", err)
		} else {
			slog.Info("scheduler: WoL packet sent", "schedule_id", s.ID, "device", device.Name, "mac", device.MACAddress, "port", device.Port)
		}

		if err := MarkScheduleFired(sch.db, s.ID, now); err != nil {
			slog.Error("scheduler: failed to record schedule fire", "schedule_id", s.ID, "error", err)
		}
	}
}

// Start launches the background goroutine that checks for due schedules
// once per minute. It stops cleanly when ctx is canceled, following the
// same ticker pattern as App.startSessionCleanup and RateLimiter.StartCleanup.
func (sch *Scheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sch.checkOnce()
			}
		}
	}()
}
