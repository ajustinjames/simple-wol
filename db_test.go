package main

import (
	"testing"
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
