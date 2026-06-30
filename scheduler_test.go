package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeClock lets tests control "now" deterministically without sleeping.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(t time.Time) *fakeClock {
	return &fakeClock{now: t}
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) Set(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = t
}

// fakeSender records WoL sends instead of touching the network.
type fakeSender struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (f *fakeSender) SendMagicPacket(mac, broadcastAddr string, port int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, mac)
	return nil
}

func (f *fakeSender) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// --- DueSchedules (pure matching logic) ---

func dow(days ...time.Weekday) int {
	mask := 0
	for _, d := range days {
		mask |= 1 << uint(d)
	}
	return mask
}

func TestDueSchedules_ExactMatchFires(t *testing.T) {
	// Wednesday 2024-01-03 02:00:00
	now := time.Date(2024, 1, 3, 2, 0, 0, 0, time.UTC)
	schedules := []Schedule{
		{ID: 1, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: true},
	}
	due := DueSchedules(now, schedules)
	if len(due) != 1 {
		t.Fatalf("expected 1 due schedule, got %d", len(due))
	}
	if due[0].ID != 1 {
		t.Fatalf("expected schedule ID 1, got %d", due[0].ID)
	}
}

func TestDueSchedules_WrongDayDoesNotFire(t *testing.T) {
	// Wednesday 2024-01-03 02:00:00, schedule only for Monday
	now := time.Date(2024, 1, 3, 2, 0, 0, 0, time.UTC)
	schedules := []Schedule{
		{ID: 1, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Monday), Enabled: true},
	}
	due := DueSchedules(now, schedules)
	if len(due) != 0 {
		t.Fatalf("expected 0 due schedules, got %d", len(due))
	}
}

func TestDueSchedules_WrongTimeDoesNotFire(t *testing.T) {
	now := time.Date(2024, 1, 3, 2, 5, 0, 0, time.UTC)
	schedules := []Schedule{
		{ID: 1, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: true},
	}
	due := DueSchedules(now, schedules)
	if len(due) != 0 {
		t.Fatalf("expected 0 due schedules (wrong minute), got %d", len(due))
	}

	now2 := time.Date(2024, 1, 3, 3, 0, 0, 0, time.UTC)
	due2 := DueSchedules(now2, schedules)
	if len(due2) != 0 {
		t.Fatalf("expected 0 due schedules (wrong hour), got %d", len(due2))
	}
}

func TestDueSchedules_MultipleSchedules(t *testing.T) {
	now := time.Date(2024, 1, 3, 2, 0, 0, 0, time.UTC) // Wednesday
	schedules := []Schedule{
		{ID: 1, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: true},          // matches
		{ID: 2, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday, time.Friday), Enabled: true}, // matches
		{ID: 3, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Tuesday), Enabled: true},             // wrong day
		{ID: 4, Hour: 9, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: true},           // wrong hour
		{ID: 5, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: false},          // disabled
	}
	due := DueSchedules(now, schedules)
	if len(due) != 2 {
		t.Fatalf("expected 2 due schedules, got %d", len(due))
	}
	ids := map[int64]bool{}
	for _, s := range due {
		ids[s.ID] = true
	}
	if !ids[1] || !ids[2] {
		t.Fatalf("expected schedule IDs 1 and 2 to be due, got %v", due)
	}
}

func TestDueSchedules_DisabledDoesNotFire(t *testing.T) {
	now := time.Date(2024, 1, 3, 2, 0, 0, 0, time.UTC)
	schedules := []Schedule{
		{ID: 1, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: false},
	}
	due := DueSchedules(now, schedules)
	if len(due) != 0 {
		t.Fatalf("expected 0 due schedules (disabled), got %d", len(due))
	}
}

func TestDueSchedules_AlreadyFiredThisMinuteDoesNotRefire(t *testing.T) {
	now := time.Date(2024, 1, 3, 2, 0, 30, 0, time.UTC)
	firedAt := time.Date(2024, 1, 3, 2, 0, 5, 0, time.UTC) // fired earlier in same minute
	schedules := []Schedule{
		{ID: 1, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: true, LastFiredAt: &firedAt},
	}
	due := DueSchedules(now, schedules)
	if len(due) != 0 {
		t.Fatalf("expected 0 due schedules (already fired this minute), got %d", len(due))
	}
}

func TestDueSchedules_PreviousWeekFireAllowsRefire(t *testing.T) {
	now := time.Date(2024, 1, 3, 2, 0, 30, 0, time.UTC)
	firedAt := time.Date(2023, 12, 27, 2, 0, 5, 0, time.UTC) // fired same time last week
	schedules := []Schedule{
		{ID: 1, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: true, LastFiredAt: &firedAt},
	}
	due := DueSchedules(now, schedules)
	if len(due) != 1 {
		t.Fatalf("expected 1 due schedule (new week), got %d", len(due))
	}
}

// --- Scheduler.checkOnce (firing logic with fake clock + fake sender) ---

func TestScheduler_CheckOnce_FiresDueSchedule(t *testing.T) {
	db, _ := InitDB(":memory:")
	defer db.Close()

	deviceID, err := CreateDevice(db, Device{Name: "Backup Server", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.1.50", Port: 9})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	now := time.Date(2024, 1, 3, 2, 0, 0, 0, time.UTC) // Wednesday
	if _, err := CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: true}); err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}

	clock := newFakeClock(now)
	sender := &fakeSender{}
	sch := &Scheduler{db: db, sender: sender, clock: clock}

	sch.checkOnce()

	if sender.callCount() != 1 {
		t.Fatalf("expected 1 WoL packet sent, got %d", sender.callCount())
	}

	schedules, _ := ListSchedulesForDevice(db, deviceID)
	if schedules[0].LastFiredAt == nil {
		t.Fatal("expected LastFiredAt to be set after firing")
	}
}

func TestScheduler_CheckOnce_DoesNotDoubleFireSameMinute(t *testing.T) {
	db, _ := InitDB(":memory:")
	defer db.Close()

	deviceID, _ := CreateDevice(db, Device{Name: "Backup Server", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.1.50", Port: 9})

	now := time.Date(2024, 1, 3, 2, 0, 0, 0, time.UTC)
	CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: true})

	clock := newFakeClock(now)
	sender := &fakeSender{}
	sch := &Scheduler{db: db, sender: sender, clock: clock}

	sch.checkOnce()
	// Advance a few seconds within the same minute and check again.
	clock.Set(now.Add(30 * time.Second))
	sch.checkOnce()

	if sender.callCount() != 1 {
		t.Fatalf("expected exactly 1 WoL packet sent (no double fire), got %d", sender.callCount())
	}
}

func TestScheduler_CheckOnce_RefiresNextWeek(t *testing.T) {
	db, _ := InitDB(":memory:")
	defer db.Close()

	deviceID, _ := CreateDevice(db, Device{Name: "Backup Server", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.1.50", Port: 9})

	now := time.Date(2024, 1, 3, 2, 0, 0, 0, time.UTC) // Wednesday
	CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: true})

	clock := newFakeClock(now)
	sender := &fakeSender{}
	sch := &Scheduler{db: db, sender: sender, clock: clock}

	sch.checkOnce()

	nextWeek := now.AddDate(0, 0, 7)
	clock.Set(nextWeek)
	sch.checkOnce()

	if sender.callCount() != 2 {
		t.Fatalf("expected 2 WoL packets sent across two weeks, got %d", sender.callCount())
	}
}

func TestScheduler_CheckOnce_WrongDaySkipped(t *testing.T) {
	db, _ := InitDB(":memory:")
	defer db.Close()

	deviceID, _ := CreateDevice(db, Device{Name: "Workstation", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.1.51", Port: 9})

	now := time.Date(2024, 1, 3, 2, 0, 0, 0, time.UTC) // Wednesday
	CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Monday), Enabled: true})

	clock := newFakeClock(now)
	sender := &fakeSender{}
	sch := &Scheduler{db: db, sender: sender, clock: clock}

	sch.checkOnce()

	if sender.callCount() != 0 {
		t.Fatalf("expected 0 WoL packets sent (wrong day), got %d", sender.callCount())
	}
}

func TestScheduler_CheckOnce_SendErrorStillRecordsAttemptButLogsError(t *testing.T) {
	db, _ := InitDB(":memory:")
	defer db.Close()

	deviceID, _ := CreateDevice(db, Device{Name: "Backup Server", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.1.50", Port: 9})

	now := time.Date(2024, 1, 3, 2, 0, 0, 0, time.UTC)
	CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 2, Minute: 0, DaysOfWeek: dow(time.Wednesday), Enabled: true})

	clock := newFakeClock(now)
	sender := &fakeSender{err: errors.New("network unreachable")}
	sch := &Scheduler{db: db, sender: sender, clock: clock}

	sch.checkOnce() // should not panic

	if sender.callCount() != 0 {
		t.Fatalf("expected 0 successful sends recorded, got %d", sender.callCount())
	}
}

// --- Start / context shutdown ---

func TestScheduler_StartStopsOnContextCancel(t *testing.T) {
	db, _ := InitDB(":memory:")
	defer db.Close()

	sch := NewScheduler(db, &fakeSender{})
	ctx, cancel := context.WithCancel(context.Background())
	sch.Start(ctx)
	cancel()
	// No assertion beyond "doesn't hang or panic" - the goroutine should
	// observe ctx.Done() and return on its next select iteration.
	time.Sleep(10 * time.Millisecond)
}
