package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestInitDB_CreatesTables(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tableName)
	if err != nil {
		t.Fatal("users table not created")
	}

	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&tableName)
	if err != nil {
		t.Fatal("sessions table not created")
	}

	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='devices'").Scan(&tableName)
	if err != nil {
		t.Fatal("devices table not created")
	}
}

func TestInitDB_DevicesHasBroadcastAddressColumn(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	hasCol, err := columnExists(db, "devices", "broadcast_address")
	if err != nil {
		t.Fatalf("columnExists failed: %v", err)
	}
	if !hasCol {
		t.Fatal("expected devices table to have broadcast_address column")
	}
}

func TestRunMigrations_AddsBroadcastAddressToExistingTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Simulate a pre-migration devices table (no broadcast_address column).
	_, err = db.Exec(`CREATE TABLE devices (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		mac_address TEXT NOT NULL,
		ip_address TEXT NOT NULL,
		port INTEGER NOT NULL DEFAULT 9,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("create legacy devices table: %v", err)
	}

	_, err = db.Exec("INSERT INTO devices (name, mac_address, ip_address, port) VALUES (?, ?, ?, ?)",
		"Legacy PC", "aa:bb:cc:dd:ee:ff", "192.168.1.50", 9)
	if err != nil {
		t.Fatalf("insert legacy device: %v", err)
	}

	if err := runMigrations(db); err != nil {
		t.Fatalf("runMigrations failed: %v", err)
	}

	var broadcastAddr string
	err = db.QueryRow("SELECT broadcast_address FROM devices WHERE name = ?", "Legacy PC").Scan(&broadcastAddr)
	if err != nil {
		t.Fatalf("failed to query migrated column: %v", err)
	}
	if broadcastAddr != DefaultBroadcastAddress {
		t.Errorf("expected migrated row to default to %q, got %q", DefaultBroadcastAddress, broadcastAddr)
	}

	// Running migrations again should be a no-op (idempotent).
	if err := runMigrations(db); err != nil {
		t.Fatalf("second runMigrations failed: %v", err)
	}
}
