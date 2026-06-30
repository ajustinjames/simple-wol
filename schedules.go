package main

import (
	"database/sql"
	"fmt"
	"time"
)

// Schedule represents a recurring wake spec for a device. DaysOfWeek is a
// bitmask where bit 0 = Sunday ... bit 6 = Saturday (matching time.Weekday
// values 0-6). Hour/Minute are evaluated in the server's local time zone
// (time.Local) - see scheduler.go for details.
type Schedule struct {
	ID          int64      `json:"id"`
	DeviceID    int64      `json:"device_id"`
	Hour        int        `json:"hour"`
	Minute      int        `json:"minute"`
	DaysOfWeek  int        `json:"days_of_week"`
	Enabled     bool       `json:"enabled"`
	LastFiredAt *time.Time `json:"last_fired_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ValidateSchedule checks that the schedule fields are within valid ranges.
func ValidateSchedule(s Schedule) error {
	if s.Hour < 0 || s.Hour > 23 {
		return fmt.Errorf("hour must be between 0 and 23")
	}
	if s.Minute < 0 || s.Minute > 59 {
		return fmt.Errorf("minute must be between 0 and 59")
	}
	if s.DaysOfWeek < 1 || s.DaysOfWeek > 0x7F {
		return fmt.Errorf("days_of_week must select at least one day (bitmask 1-127)")
	}
	return nil
}

func CreateSchedule(db *sql.DB, s Schedule) (int64, error) {
	if err := ValidateSchedule(s); err != nil {
		return 0, err
	}

	// Ensure the device exists to give a clean error instead of an FK violation.
	if _, err := GetDevice(db, s.DeviceID); err != nil {
		return 0, fmt.Errorf("device not found")
	}

	result, err := db.Exec(
		"INSERT INTO schedules (device_id, hour, minute, days_of_week, enabled) VALUES (?, ?, ?, ?, ?)",
		s.DeviceID, s.Hour, s.Minute, s.DaysOfWeek, s.Enabled,
	)
	if err != nil {
		return 0, fmt.Errorf("create schedule: %w", err)
	}
	return result.LastInsertId()
}

func ListSchedulesForDevice(db *sql.DB, deviceID int64) ([]Schedule, error) {
	rows, err := db.Query(
		"SELECT id, device_id, hour, minute, days_of_week, enabled, last_fired_at, created_at FROM schedules WHERE device_id = ? ORDER BY hour, minute",
		deviceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSchedules(rows)
}

func ListAllSchedules(db *sql.DB) ([]Schedule, error) {
	rows, err := db.Query(
		"SELECT id, device_id, hour, minute, days_of_week, enabled, last_fired_at, created_at FROM schedules ORDER BY device_id, hour, minute",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSchedules(rows)
}

func scanSchedules(rows *sql.Rows) ([]Schedule, error) {
	var schedules []Schedule
	for rows.Next() {
		var s Schedule
		var lastFired sql.NullTime
		if err := rows.Scan(&s.ID, &s.DeviceID, &s.Hour, &s.Minute, &s.DaysOfWeek, &s.Enabled, &lastFired, &s.CreatedAt); err != nil {
			return nil, err
		}
		if lastFired.Valid {
			t := lastFired.Time
			s.LastFiredAt = &t
		}
		schedules = append(schedules, s)
	}
	return schedules, rows.Err()
}

func GetSchedule(db *sql.DB, id int64) (Schedule, error) {
	var s Schedule
	var lastFired sql.NullTime
	err := db.QueryRow(
		"SELECT id, device_id, hour, minute, days_of_week, enabled, last_fired_at, created_at FROM schedules WHERE id = ?", id,
	).Scan(&s.ID, &s.DeviceID, &s.Hour, &s.Minute, &s.DaysOfWeek, &s.Enabled, &lastFired, &s.CreatedAt)
	if err != nil {
		return s, fmt.Errorf("schedule not found: %w", err)
	}
	if lastFired.Valid {
		t := lastFired.Time
		s.LastFiredAt = &t
	}
	return s, nil
}

func DeleteSchedule(db *sql.DB, id int64) error {
	result, err := db.Exec("DELETE FROM schedules WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("schedule not found")
	}
	return nil
}

func MarkScheduleFired(db *sql.DB, id int64, firedAt time.Time) error {
	_, err := db.Exec("UPDATE schedules SET last_fired_at = ? WHERE id = ?", firedAt, id)
	return err
}
