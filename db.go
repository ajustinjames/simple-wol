package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func InitDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT NOT NULL UNIQUE,
			user_id INTEGER NOT NULL REFERENCES users(id),
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			mac_address TEXT NOT NULL,
			ip_address TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 9,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, q := range tables {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// runMigrations applies schema changes to existing databases that predate a
// given column/table. Each migration is safe to run repeatedly: it checks
// for the column/table's existence before attempting to add it.
func runMigrations(db *sql.DB) error {
	hasCol, err := columnExists(db, "devices", "broadcast_address")
	if err != nil {
		return err
	}
	if !hasCol {
		if _, err := db.Exec(`ALTER TABLE devices ADD COLUMN broadcast_address TEXT NOT NULL DEFAULT '255.255.255.255'`); err != nil {
			return fmt.Errorf("add broadcast_address column: %w", err)
		}
	}
	return nil
}

// columnExists reports whether the given column is present on the table.
func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
