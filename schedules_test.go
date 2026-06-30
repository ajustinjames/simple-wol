package main

import "testing"

func TestCreateSchedule(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	deviceID, _ := CreateDevice(db, Device{Name: "PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.1.1", Port: 9})
	id, err := CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 2, Minute: 30, DaysOfWeek: 0x7F, Enabled: true})
	if err != nil { t.Fatalf("CreateSchedule failed: %v", err) }
	if id == 0 { t.Fatal("expected non-zero ID") }
}

func TestCreateSchedule_InvalidHour(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	deviceID, _ := CreateDevice(db, Device{Name: "PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.1.1", Port: 9})
	_, err := CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 24, Minute: 0, DaysOfWeek: 1})
	if err == nil { t.Fatal("expected error for invalid hour") }
}

func TestCreateSchedule_InvalidDaysOfWeek(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	deviceID, _ := CreateDevice(db, Device{Name: "PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.1.1", Port: 9})
	_, err := CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 1, Minute: 0, DaysOfWeek: 0})
	if err == nil { t.Fatal("expected error for empty days_of_week") }
}

func TestCreateSchedule_DeviceNotFound(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	_, err := CreateSchedule(db, Schedule{DeviceID: 999, Hour: 1, Minute: 0, DaysOfWeek: 1})
	if err == nil { t.Fatal("expected error for missing device") }
}

func TestListSchedulesForDevice(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	deviceID, _ := CreateDevice(db, Device{Name: "PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.1.1", Port: 9})
	CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 2, Minute: 0, DaysOfWeek: 1, Enabled: true})
	CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 9, Minute: 0, DaysOfWeek: 0x7F, Enabled: true})
	schedules, err := ListSchedulesForDevice(db, deviceID)
	if err != nil { t.Fatalf("ListSchedulesForDevice failed: %v", err) }
	if len(schedules) != 2 { t.Fatalf("expected 2 schedules, got %d", len(schedules)) }
}

func TestDeleteSchedule(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	deviceID, _ := CreateDevice(db, Device{Name: "PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.1.1", Port: 9})
	id, _ := CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 2, Minute: 0, DaysOfWeek: 1, Enabled: true})
	if err := DeleteSchedule(db, id); err != nil { t.Fatalf("DeleteSchedule failed: %v", err) }
	schedules, _ := ListSchedulesForDevice(db, deviceID)
	if len(schedules) != 0 { t.Fatalf("expected 0 schedules after delete, got %d", len(schedules)) }
}

func TestDeleteSchedule_NotFound(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	err := DeleteSchedule(db, 999)
	if err == nil { t.Fatal("expected error deleting nonexistent schedule") }
}

func TestDeviceCascadeDeletesSchedules(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	deviceID, _ := CreateDevice(db, Device{Name: "PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.1.1", Port: 9})
	CreateSchedule(db, Schedule{DeviceID: deviceID, Hour: 2, Minute: 0, DaysOfWeek: 1, Enabled: true})
	if err := DeleteDevice(db, deviceID); err != nil { t.Fatalf("DeleteDevice failed: %v", err) }
	schedules, err := ListSchedulesForDevice(db, deviceID)
	if err != nil { t.Fatalf("ListSchedulesForDevice failed: %v", err) }
	if len(schedules) != 0 { t.Fatalf("expected schedules to cascade-delete, got %d", len(schedules)) }
}
