# Simple WoL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a self-hosted Wake-on-LAN web app with auth, device management, WoL sending, status checks, network scanning, and Docker deployment.

**Architecture:** Single Go binary using `net/http`, SQLite via `modernc.org/sqlite`, and vanilla HTML/CSS/JS embedded with `embed`. All state in one SQLite file. Deployed via Docker with `network_mode: host` for LAN broadcast access.

**Tech Stack:** Go 1.22+, `modernc.org/sqlite`, `golang.org/x/crypto/bcrypt`, `log/slog`, Docker, Alpine Linux

**Spec:** `docs/superpowers/specs/2026-03-20-simple-wol-design.md`

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `.gitignore`
- Create: `CLAUDE.md`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd /Users/ajames/workspace/ajustinjames/simple-wol
go mod init github.com/ajustinjames/simple-wol
```

- [ ] **Step 2: Create .gitignore**

Create `.gitignore`:
```
# Binary
simple-wol

# Data
data/
*.db

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
```

- [ ] **Step 3: Create CLAUDE.md**

Create `CLAUDE.md`:
```markdown
# Simple WoL

## Build & Run
- `go build -o simple-wol .` — build the binary
- `go run .` — run the server (default port 8080)
- `go test ./...` — run all tests
- `go test -v -run TestName .` — run a specific test

## Architecture
- Single Go package (`main`) in the project root
- `net/http` for routing, no external web framework
- SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- Frontend: vanilla HTML/CSS/JS in `static/`, embedded via `embed` package
- Database file: `data/simple-wol.db` (created automatically)

## Conventions
- Structured logging via `log/slog` (JSON to stdout)
- Tests alongside source files (`*_test.go`)
- All API routes prefixed with `/api/`
- Auth middleware on all `/api/*` routes except `/api/login` and `/api/setup`
```

- [ ] **Step 4: Create minimal main.go**

Create `main.go`:
```go
package main

import (
	"log/slog"
	"os"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	slog.Info("simple-wol starting")
}
```

- [ ] **Step 5: Verify it compiles**

Run: `go build -o simple-wol .`
Expected: compiles with no errors

- [ ] **Step 6: Commit**

```bash
git add go.mod main.go .gitignore CLAUDE.md
git commit -m "feat: project scaffolding with Go module, main entry point, and CLAUDE.md"
```

---

### Task 2: Database Layer

**Files:**
- Create: `db.go`
- Create: `db_test.go`

- [ ] **Step 1: Install SQLite dependency**

Run:
```bash
go get modernc.org/sqlite
```

- [ ] **Step 2: Write failing test for DB initialization**

Create `db_test.go`:
```go
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

	// Verify users table exists
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tableName)
	if err != nil {
		t.Fatal("users table not created")
	}

	// Verify sessions table exists
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&tableName)
	if err != nil {
		t.Fatal("sessions table not created")
	}

	// Verify devices table exists
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='devices'").Scan(&tableName)
	if err != nil {
		t.Fatal("devices table not created")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test -v -run TestInitDB .`
Expected: FAIL — `InitDB` not defined

- [ ] **Step 4: Implement InitDB**

Create `db.go`:
```go
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
			status_port INTEGER NOT NULL DEFAULT 3389,
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
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -v -run TestInitDB .`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add db.go db_test.go go.mod go.sum
git commit -m "feat: database layer with SQLite schema for users, sessions, and devices"
```

---

### Task 3: Auth — Password Hashing & User Setup

**Files:**
- Create: `auth.go`
- Create: `auth_test.go`

- [ ] **Step 1: Install bcrypt dependency**

Run:
```bash
go get golang.org/x/crypto/bcrypt
```

- [ ] **Step 2: Write failing tests for password hashing and user creation**

Create `auth_test.go`:
```go
package main

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("testpass123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("testpass123")); err != nil {
		t.Fatal("hash does not match password")
	}
}

func TestCheckPassword(t *testing.T) {
	hash, _ := HashPassword("testpass123")

	if !CheckPassword(hash, "testpass123") {
		t.Fatal("expected password to match")
	}
	if CheckPassword(hash, "wrongpassword") {
		t.Fatal("expected password to not match")
	}
}

func TestCreateUser(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = CreateUser(db, "admin", "password123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Verify user exists
	var username string
	err = db.QueryRow("SELECT username FROM users WHERE username = ?", "admin").Scan(&username)
	if err != nil {
		t.Fatal("user not found in database")
	}
}

func TestCreateUser_DuplicateFails(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_ = CreateUser(db, "admin", "password123")
	err = CreateUser(db, "admin", "password456")
	if err == nil {
		t.Fatal("expected error for duplicate user")
	}
}

func TestUserExists(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if UserExists(db) {
		t.Fatal("expected no user to exist")
	}

	_ = CreateUser(db, "admin", "password123")

	if !UserExists(db) {
		t.Fatal("expected user to exist")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test -v -run "TestHash|TestCheck|TestCreate|TestUserExists" .`
Expected: FAIL — functions not defined

- [ ] **Step 4: Implement auth functions**

Create `auth.go`:
```go
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func CreateUser(db *sql.DB, username, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, hash)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func UserExists(db *sql.DB) bool {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count > 0
}

func AuthenticateUser(db *sql.DB, username, password string) (int64, error) {
	var id int64
	var hash string
	err := db.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", username).Scan(&id, &hash)
	if err != nil {
		return 0, fmt.Errorf("invalid credentials")
	}
	if !CheckPassword(hash, password) {
		return 0, fmt.Errorf("invalid credentials")
	}
	return id, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -v -run "TestHash|TestCheck|TestCreate|TestUserExists" .`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add auth.go auth_test.go go.mod go.sum
git commit -m "feat: auth layer with password hashing, user creation, and authentication"
```

---

### Task 4: Auth — Sessions & Rate Limiting

**Files:**
- Modify: `auth.go`
- Modify: `auth_test.go`

- [ ] **Step 1: Write failing tests for session management**

Append to `auth_test.go`:
```go
func TestCreateSession(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_ = CreateUser(db, "admin", "password123")

	token, err := CreateSession(db, 1, false)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestValidateSession(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_ = CreateUser(db, "admin", "password123")
	token, _ := CreateSession(db, 1, false)

	userID, err := ValidateSession(db, token)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}
	if userID != 1 {
		t.Fatalf("expected userID 1, got %d", userID)
	}
}

func TestValidateSession_InvalidToken(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = ValidateSession(db, "invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestDeleteSession(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_ = CreateUser(db, "admin", "password123")
	token, _ := CreateSession(db, 1, false)

	err = DeleteSession(db, token)
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	_, err = ValidateSession(db, token)
	if err == nil {
		t.Fatal("expected session to be invalid after deletion")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run "TestCreateSession|TestValidateSession|TestDeleteSession" .`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement session functions**

Append to `auth.go`:
```go
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func CreateSession(db *sql.DB, userID int64, remember bool) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	duration := 24 * time.Hour
	if remember {
		duration = 30 * 24 * time.Hour
	}
	expiresAt := time.Now().Add(duration)

	_, err = db.Exec(
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		token, userID, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

func ValidateSession(db *sql.DB, token string) (int64, error) {
	var userID int64
	err := db.QueryRow(
		"SELECT user_id FROM sessions WHERE token = ? AND expires_at > ?",
		token, time.Now(),
	).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("invalid session")
	}
	return userID, nil
}

func DeleteSession(db *sql.DB, token string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run "TestCreateSession|TestValidateSession|TestDeleteSession" .`
Expected: PASS

- [ ] **Step 5: Write failing tests for rate limiter**

Append to `auth_test.go`:
```go
func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(5, 10*time.Minute, 15*time.Minute)
	for i := 0; i < 5; i++ {
		if !rl.Allow("192.168.1.1") {
			t.Fatalf("expected allow on attempt %d", i+1)
		}
		rl.RecordFailure("192.168.1.1")
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(5, 10*time.Minute, 15*time.Minute)
	for i := 0; i < 5; i++ {
		rl.RecordFailure("192.168.1.1")
	}
	if rl.Allow("192.168.1.1") {
		t.Fatal("expected block after 5 failures")
	}
}

func TestRateLimiter_ResetOnSuccess(t *testing.T) {
	rl := NewRateLimiter(5, 10*time.Minute, 15*time.Minute)
	for i := 0; i < 4; i++ {
		rl.RecordFailure("192.168.1.1")
	}
	rl.Reset("192.168.1.1")
	if !rl.Allow("192.168.1.1") {
		t.Fatal("expected allow after reset")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(5, 10*time.Minute, 15*time.Minute)
	for i := 0; i < 5; i++ {
		rl.RecordFailure("192.168.1.1")
	}
	if rl.Allow("192.168.1.1") {
		t.Fatal("expected block for 192.168.1.1")
	}
	if !rl.Allow("192.168.1.2") {
		t.Fatal("expected allow for 192.168.1.2")
	}
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `go test -v -run "TestRateLimiter" .`
Expected: FAIL — `NewRateLimiter` not defined

- [ ] **Step 7: Implement rate limiter**

Append to `auth.go`:
```go
type loginAttempt struct {
	failures  int
	firstFail time.Time
	lockedAt  time.Time
}

type RateLimiter struct {
	mu          sync.Mutex
	attempts    map[string]*loginAttempt
	maxFailures int
	window      time.Duration
	lockout     time.Duration
}

func NewRateLimiter(maxFailures int, window, lockout time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts:    make(map[string]*loginAttempt),
		maxFailures: maxFailures,
		window:      window,
		lockout:     lockout,
	}
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	a, ok := rl.attempts[ip]
	if !ok {
		return true
	}

	// Check if locked out
	if !a.lockedAt.IsZero() {
		if time.Since(a.lockedAt) > rl.lockout {
			delete(rl.attempts, ip)
			return true
		}
		return false
	}

	// Check if window has expired
	if time.Since(a.firstFail) > rl.window {
		delete(rl.attempts, ip)
		return true
	}

	return a.failures < rl.maxFailures
}

func (rl *RateLimiter) RecordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	a, ok := rl.attempts[ip]
	if !ok {
		a = &loginAttempt{firstFail: time.Now()}
		rl.attempts[ip] = a
	}

	a.failures++
	if a.failures >= rl.maxFailures {
		a.lockedAt = time.Now()
	}
}

func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test -v -run "TestRateLimiter" .`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add auth.go auth_test.go
git commit -m "feat: session management and login rate limiting"
```

---

### Task 5: Device CRUD

**Files:**
- Create: `devices.go`
- Create: `devices_test.go`

- [ ] **Step 1: Write failing tests for device CRUD**

Create `devices_test.go`:
```go
package main

import (
	"testing"
)

func TestCreateDevice(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	d := Device{
		Name:       "My PC",
		MACAddress: "AA:BB:CC:DD:EE:FF",
		IPAddress:  "192.168.4.100",
		Port:       9,
		StatusPort: 3389,
	}

	id, err := CreateDevice(db, d)
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}
}

func TestListDevices(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, _ = CreateDevice(db, Device{Name: "PC1", MACAddress: "AA:BB:CC:DD:EE:01", IPAddress: "192.168.4.1", Port: 9, StatusPort: 3389})
	_, _ = CreateDevice(db, Device{Name: "PC2", MACAddress: "AA:BB:CC:DD:EE:02", IPAddress: "192.168.4.2", Port: 9, StatusPort: 3389})

	devices, err := ListDevices(db)
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
}

func TestGetDevice(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, _ := CreateDevice(db, Device{Name: "My PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.4.100", Port: 9, StatusPort: 3389})

	d, err := GetDevice(db, id)
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}
	if d.Name != "My PC" {
		t.Fatalf("expected name 'My PC', got '%s'", d.Name)
	}
}

func TestUpdateDevice(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, _ := CreateDevice(db, Device{Name: "My PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.4.100", Port: 9, StatusPort: 3389})

	err = UpdateDevice(db, id, Device{Name: "Gaming PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.4.100", Port: 7, StatusPort: 22})
	if err != nil {
		t.Fatalf("UpdateDevice failed: %v", err)
	}

	d, _ := GetDevice(db, id)
	if d.Name != "Gaming PC" {
		t.Fatalf("expected name 'Gaming PC', got '%s'", d.Name)
	}
	if d.Port != 7 {
		t.Fatalf("expected port 7, got %d", d.Port)
	}
}

func TestDeleteDevice(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, _ := CreateDevice(db, Device{Name: "My PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.4.100", Port: 9, StatusPort: 3389})

	err = DeleteDevice(db, id)
	if err != nil {
		t.Fatalf("DeleteDevice failed: %v", err)
	}

	_, err = GetDevice(db, id)
	if err == nil {
		t.Fatal("expected error getting deleted device")
	}
}

func TestValidateMAC(t *testing.T) {
	valid := []string{"AA:BB:CC:DD:EE:FF", "aa:bb:cc:dd:ee:ff", "AA-BB-CC-DD-EE-FF"}
	for _, mac := range valid {
		if err := ValidateMAC(mac); err != nil {
			t.Errorf("expected %q to be valid: %v", mac, err)
		}
	}

	invalid := []string{"not-a-mac", "AA:BB:CC:DD:EE", "GG:HH:II:JJ:KK:LL", ""}
	for _, mac := range invalid {
		if err := ValidateMAC(mac); err == nil {
			t.Errorf("expected %q to be invalid", mac)
		}
	}
}

func TestValidateIP(t *testing.T) {
	valid := []string{"192.168.4.100", "10.0.0.1"}
	for _, ip := range valid {
		if err := ValidateIP(ip); err != nil {
			t.Errorf("expected %q to be valid: %v", ip, err)
		}
	}

	invalid := []string{"not-an-ip", "999.999.999.999", "", "::1", "fe80::1"}
	for _, ip := range invalid {
		if err := ValidateIP(ip); err == nil {
			t.Errorf("expected %q to be invalid", ip)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run "TestCreateDevice|TestListDevices|TestGetDevice|TestUpdateDevice|TestDeleteDevice|TestValidateMAC|TestValidateIP" .`
Expected: FAIL — types and functions not defined

- [ ] **Step 3: Implement device CRUD and validation**

Create `devices.go`:
```go
package main

import (
	"database/sql"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

type Device struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	MACAddress string    `json:"mac_address"`
	IPAddress  string    `json:"ip_address"`
	Port       int       `json:"port"`
	StatusPort int       `json:"status_port"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

var macRegex = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}$`)

func ValidateMAC(mac string) error {
	if !macRegex.MatchString(mac) {
		return fmt.Errorf("invalid MAC address: %s", mac)
	}
	return nil
}

func ValidateIP(ip string) error {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return fmt.Errorf("invalid IPv4 address: %s", ip)
	}
	return nil
}

func SanitizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "<", "&lt;")
	name = strings.ReplaceAll(name, ">", "&gt;")
	name = strings.ReplaceAll(name, "\"", "&quot;")
	name = strings.ReplaceAll(name, "'", "&#39;")
	return name
}

func CreateDevice(db *sql.DB, d Device) (int64, error) {
	if err := ValidateMAC(d.MACAddress); err != nil {
		return 0, err
	}
	if err := ValidateIP(d.IPAddress); err != nil {
		return 0, err
	}
	d.Name = SanitizeName(d.Name)

	result, err := db.Exec(
		"INSERT INTO devices (name, mac_address, ip_address, port, status_port) VALUES (?, ?, ?, ?, ?)",
		d.Name, d.MACAddress, d.IPAddress, d.Port, d.StatusPort,
	)
	if err != nil {
		return 0, fmt.Errorf("create device: %w", err)
	}
	return result.LastInsertId()
}

func ListDevices(db *sql.DB) ([]Device, error) {
	rows, err := db.Query("SELECT id, name, mac_address, ip_address, port, status_port, created_at, updated_at FROM devices ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.MACAddress, &d.IPAddress, &d.Port, &d.StatusPort, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func GetDevice(db *sql.DB, id int64) (Device, error) {
	var d Device
	err := db.QueryRow(
		"SELECT id, name, mac_address, ip_address, port, status_port, created_at, updated_at FROM devices WHERE id = ?", id,
	).Scan(&d.ID, &d.Name, &d.MACAddress, &d.IPAddress, &d.Port, &d.StatusPort, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return d, fmt.Errorf("device not found: %w", err)
	}
	return d, nil
}

func UpdateDevice(db *sql.DB, id int64, d Device) error {
	if err := ValidateMAC(d.MACAddress); err != nil {
		return err
	}
	if err := ValidateIP(d.IPAddress); err != nil {
		return err
	}
	d.Name = SanitizeName(d.Name)

	result, err := db.Exec(
		"UPDATE devices SET name=?, mac_address=?, ip_address=?, port=?, status_port=?, updated_at=? WHERE id=?",
		d.Name, d.MACAddress, d.IPAddress, d.Port, d.StatusPort, time.Now(), id,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("device not found")
	}
	return nil
}

func DeleteDevice(db *sql.DB, id int64) error {
	result, err := db.Exec("DELETE FROM devices WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("device not found")
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run "TestCreateDevice|TestListDevices|TestGetDevice|TestUpdateDevice|TestDeleteDevice|TestValidateMAC|TestValidateIP" .`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add devices.go devices_test.go
git commit -m "feat: device CRUD with MAC/IP validation"
```

---

### Task 6: Wake-on-LAN Logic

**Files:**
- Create: `wol.go`
- Create: `wol_test.go`

- [ ] **Step 1: Write failing tests for WoL packet construction**

Create `wol_test.go`:
```go
package main

import (
	"testing"
)

func TestBuildMagicPacket(t *testing.T) {
	packet, err := BuildMagicPacket("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("BuildMagicPacket failed: %v", err)
	}

	// Magic packet is 102 bytes: 6 bytes 0xFF + 16 * 6 bytes MAC
	if len(packet) != 102 {
		t.Fatalf("expected 102 bytes, got %d", len(packet))
	}

	// First 6 bytes should be 0xFF
	for i := 0; i < 6; i++ {
		if packet[i] != 0xFF {
			t.Fatalf("byte %d: expected 0xFF, got 0x%02X", i, packet[i])
		}
	}

	// MAC should repeat 16 times starting at byte 6
	expectedMAC := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	for i := 0; i < 16; i++ {
		offset := 6 + i*6
		for j := 0; j < 6; j++ {
			if packet[offset+j] != expectedMAC[j] {
				t.Fatalf("repetition %d byte %d: expected 0x%02X, got 0x%02X", i, j, expectedMAC[j], packet[offset+j])
			}
		}
	}
}

func TestBuildMagicPacket_DashSeparator(t *testing.T) {
	packet, err := BuildMagicPacket("AA-BB-CC-DD-EE-FF")
	if err != nil {
		t.Fatalf("BuildMagicPacket failed: %v", err)
	}
	if len(packet) != 102 {
		t.Fatalf("expected 102 bytes, got %d", len(packet))
	}
}

func TestBuildMagicPacket_InvalidMAC(t *testing.T) {
	_, err := BuildMagicPacket("not-a-mac")
	if err == nil {
		t.Fatal("expected error for invalid MAC")
	}
}

type mockSender struct {
	called        bool
	mac           string
	broadcastAddr string
	port          int
}

func (m *mockSender) SendMagicPacket(mac, broadcastAddr string, port int) error {
	m.called = true
	m.mac = mac
	m.broadcastAddr = broadcastAddr
	m.port = port
	return nil
}

func TestWakeDevice_CallsSender(t *testing.T) {
	mock := &mockSender{}
	err := WakeDevice(mock, "AA:BB:CC:DD:EE:FF", "255.255.255.255", 9)
	if err != nil {
		t.Fatalf("WakeDevice failed: %v", err)
	}
	if !mock.called {
		t.Fatal("expected sender to be called")
	}
	if mock.mac != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("expected MAC AA:BB:CC:DD:EE:FF, got %s", mock.mac)
	}
	if mock.port != 9 {
		t.Fatalf("expected port 9, got %d", mock.port)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run "TestBuildMagicPacket|TestWakeDevice" .`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement WoL logic**

Create `wol.go`:
```go
package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

type PacketSender interface {
	SendMagicPacket(macAddress string, broadcastAddr string, port int) error
}

type UDPSender struct{}

func (u *UDPSender) SendMagicPacket(macAddress, broadcastAddr string, port int) error {
	packet, err := BuildMagicPacket(macAddress)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", broadcastAddr, port)
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return fmt.Errorf("dial udp: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(packet)
	if err != nil {
		return fmt.Errorf("send packet: %w", err)
	}
	return nil
}

func BuildMagicPacket(mac string) ([]byte, error) {
	mac = strings.ReplaceAll(mac, "-", ":")
	parts := strings.Split(mac, ":")
	if len(parts) != 6 {
		return nil, fmt.Errorf("invalid MAC address: %s", mac)
	}

	macBytes := make([]byte, 6)
	for i, part := range parts {
		b, err := hex.DecodeString(part)
		if err != nil || len(b) != 1 {
			return nil, fmt.Errorf("invalid MAC address: %s", mac)
		}
		macBytes[i] = b[0]
	}

	// 6 bytes of 0xFF + 16 repetitions of MAC
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], macBytes)
	}

	return packet, nil
}

func WakeDevice(sender PacketSender, mac, broadcastAddr string, port int) error {
	return sender.SendMagicPacket(mac, broadcastAddr, port)
}

func CheckDeviceStatus(ip string, port int) bool {
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run "TestBuildMagicPacket|TestWakeDevice" .`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add wol.go wol_test.go
git commit -m "feat: Wake-on-LAN magic packet builder and UDP sender with status check"
```

---

### Task 7: Network Scanning

**Files:**
- Create: `scan.go`
- Create: `scan_test.go`

- [ ] **Step 1: Write failing tests for subnet detection and ARP parsing**

Create `scan_test.go`:
```go
package main

import (
	"testing"
)

func TestParseARPTableLinux(t *testing.T) {
	// Simulate /proc/net/arp content
	content := `IP address       HW type     Flags       HW address            Mask     Device
192.168.4.1      0x1         0x2         aa:bb:cc:dd:ee:01     *        eth0
192.168.4.100    0x1         0x2         aa:bb:cc:dd:ee:02     *        eth0
192.168.4.200    0x1         0x0         00:00:00:00:00:00     *        eth0
`

	entries := ParseARPTableLinux(content)
	// Should skip the entry with all-zero MAC (incomplete)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].IP != "192.168.4.1" {
		t.Fatalf("expected IP 192.168.4.1, got %s", entries[0].IP)
	}
	if entries[0].MAC != "aa:bb:cc:dd:ee:01" {
		t.Fatalf("expected MAC aa:bb:cc:dd:ee:01, got %s", entries[0].MAC)
	}
}

func TestParseARPTableDarwin(t *testing.T) {
	content := `? (192.168.4.1) at aa:bb:cc:dd:ee:01 on en0 ifscope [ethernet]
? (192.168.4.100) at aa:bb:cc:dd:ee:02 on en0 ifscope [ethernet]
? (192.168.4.200) at (incomplete) on en0 ifscope [ethernet]
`

	entries := ParseARPTableDarwin(content)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].IP != "192.168.4.1" {
		t.Fatalf("expected IP 192.168.4.1, got %s", entries[0].IP)
	}
	if entries[0].MAC != "aa:bb:cc:dd:ee:01" {
		t.Fatalf("expected MAC aa:bb:cc:dd:ee:01, got %s", entries[0].MAC)
	}
}

func TestSubnetIPs(t *testing.T) {
	ips := SubnetIPs("192.168.4.0", 24)
	// /24 should give 254 hosts (1-254, excluding .0 and .255)
	if len(ips) != 254 {
		t.Fatalf("expected 254 IPs, got %d", len(ips))
	}
	if ips[0] != "192.168.4.1" {
		t.Fatalf("expected first IP 192.168.4.1, got %s", ips[0])
	}
	if ips[253] != "192.168.4.254" {
		t.Fatalf("expected last IP 192.168.4.254, got %s", ips[253])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run "TestParseARP|TestSubnetIPs" .`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement network scanning**

Create `scan.go`:
```go
package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type ARPEntry struct {
	IP  string `json:"ip"`
	MAC string `json:"mac"`
}

// ParseARPTableLinux parses /proc/net/arp format:
// IP address       HW type     Flags       HW address            Mask     Device
// 192.168.4.1      0x1         0x2         aa:bb:cc:dd:ee:01     *        eth0
func ParseARPTableLinux(content string) []ARPEntry {
	var entries []ARPEntry
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines[1:] { // Skip header
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		mac := fields[3]
		if mac == "00:00:00:00:00:00" {
			continue
		}
		entries = append(entries, ARPEntry{IP: fields[0], MAC: mac})
	}
	return entries
}

// ParseARPTableDarwin parses macOS `arp -a` format:
// ? (192.168.4.1) at aa:bb:cc:dd:ee:01 on en0 ifscope [ethernet]
func ParseARPTableDarwin(content string) []ARPEntry {
	var entries []ARPEntry
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[1] == "" {
			continue
		}
		// Extract IP from parentheses: (192.168.4.1)
		ip := strings.Trim(fields[1], "()")
		mac := fields[3]
		if mac == "(incomplete)" || mac == "00:00:00:00:00:00" {
			continue
		}
		entries = append(entries, ARPEntry{IP: ip, MAC: mac})
	}
	return entries
}

func ParseARPTable(content string) []ARPEntry {
	if runtime.GOOS == "darwin" {
		return ParseARPTableDarwin(content)
	}
	return ParseARPTableLinux(content)
}

func SubnetIPs(networkAddr string, prefixLen int) []string {
	ip := net.ParseIP(networkAddr).To4()
	if ip == nil {
		return nil
	}

	ipInt := binary.BigEndian.Uint32(ip)
	hostBits := 32 - prefixLen
	numHosts := (1 << hostBits) - 2 // Exclude network and broadcast

	ips := make([]string, 0, numHosts)
	for i := 1; i <= numHosts; i++ {
		hostIP := make(net.IP, 4)
		binary.BigEndian.PutUint32(hostIP, ipInt+uint32(i))
		ips = append(ips, hostIP.String())
	}
	return ips
}

func DetectSubnet() (networkAddr string, prefixLen int, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", 0, err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			ones, _ := ipNet.Mask.Size()
			network := ipNet.IP.Mask(ipNet.Mask)
			return network.String(), ones, nil
		}
	}
	return "", 0, fmt.Errorf("no suitable network interface found")
}

func PingSweep(ips []string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 50) // Limit concurrency

	for _, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()

			var cmd *exec.Cmd
			if runtime.GOOS == "darwin" {
				cmd = exec.Command("ping", "-c", "1", "-W", "1000", ip)
			} else {
				cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
			}
			cmd.Run() // We don't care about the result; it populates the ARP table
		}(ip)
	}
	wg.Wait()
}

func ReadARPTable() (string, error) {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("arp", "-a").Output()
		return string(out), err
	}
	// Linux: read /proc/net/arp directly
	out, err := os.ReadFile("/proc/net/arp")
	return string(out), err
}

func ScanNetwork() ([]ARPEntry, error) {
	networkAddr, prefixLen, err := DetectSubnet()
	if err != nil {
		return nil, err
	}

	ips := SubnetIPs(networkAddr, prefixLen)
	PingSweep(ips)

	// Small delay to let ARP table populate
	time.Sleep(500 * time.Millisecond)

	content, err := ReadARPTable()
	if err != nil {
		return nil, fmt.Errorf("read arp table: %w", err)
	}

	return ParseARPTable(content), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v -run "TestParseARP|TestSubnetIPs" .`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add scan.go scan_test.go
git commit -m "feat: network scanning with ping sweep and ARP table parsing"
```

---

### Task 8: HTTP Server & Auth Handlers

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Create placeholder static files**

```bash
mkdir -p static
echo '<!DOCTYPE html><html><body>index</body></html>' > static/index.html
echo '<!DOCTYPE html><html><body>login</body></html>' > static/login.html
echo '' > static/style.css
echo '' > static/app.js
```

- [ ] **Step 2: Implement the HTTP server with all routes**

Replace `main.go` with the full server implementation. This includes:
- `App` struct holding `db`, `sender`, and `limiter`
- `embed.FS` for static files
- Route registration with `http.NewServeMux` (Go 1.22+ method-pattern routing)
- `requireAuth` middleware
- `isSecure` and `setSessionCookie` helpers
- Handlers: `handleSetup`, `handleLogin`, `handleLogout`, `handleIndex`, `handleLoginPage`
- Handlers: `handleListDevices`, `handleCreateDevice`, `handleUpdateDevice`, `handleDeleteDevice`
- Handlers: `handleWakeDevice`, `handleDeviceStatus`, `handleNetworkScan`
- Static file serving via `http.FileServer(http.FS(staticFiles))`

```go
package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

type App struct {
	db      *sql.DB
	sender  PacketSender
	limiter *RateLimiter
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	os.MkdirAll(dataDir, 0755)
	dbPath := filepath.Join(dataDir, "simple-wol.db")

	db, err := InitDB(dbPath)
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	app := &App{
		db:      db,
		sender:  &UDPSender{},
		limiter: NewRateLimiter(5, 10*time.Minute, 15*time.Minute),
	}

	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("GET /", app.handleIndex)
	mux.HandleFunc("GET /login", app.handleLoginPage)

	// Auth routes (no auth middleware)
	mux.HandleFunc("POST /api/setup", app.handleSetup)
	mux.HandleFunc("POST /api/login", app.handleLogin)

	// Protected routes
	mux.HandleFunc("POST /api/logout", app.requireAuth(app.handleLogout))
	mux.HandleFunc("GET /api/devices", app.requireAuth(app.handleListDevices))
	mux.HandleFunc("POST /api/devices", app.requireAuth(app.handleCreateDevice))
	mux.HandleFunc("PUT /api/devices/{id}", app.requireAuth(app.handleUpdateDevice))
	mux.HandleFunc("DELETE /api/devices/{id}", app.requireAuth(app.handleDeleteDevice))
	mux.HandleFunc("POST /api/devices/{id}/wake", app.requireAuth(app.handleWakeDevice))
	mux.HandleFunc("GET /api/devices/{id}/status", app.requireAuth(app.handleDeviceStatus))
	mux.HandleFunc("POST /api/network/scan", app.requireAuth(app.handleNetworkScan))

	slog.Info("simple-wol starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// --- Middleware & Helpers ---

func (app *App) isSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}

func (app *App) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, remember bool) {
	maxAge := 86400 // 24 hours
	if remember {
		maxAge = 30 * 86400 // 30 days
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   app.isSecure(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func (app *App) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_, err = ValidateSession(app.db, cookie.Value)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// --- Auth Handlers ---

func (app *App) handleSetup(w http.ResponseWriter, r *http.Request) {
	if UserExists(app.db) {
		http.Error(w, `{"error":"user already exists"}`, http.StatusConflict)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, `{"error":"username and password required"}`, http.StatusBadRequest)
		return
	}

	if err := CreateUser(app.db, req.Username, req.Password); err != nil {
		slog.Error("failed to create user", "error", err)
		http.Error(w, `{"error":"failed to create user"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("user created", "username", req.Username)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !app.limiter.Allow(ip) {
		slog.Warn("login rate limited", "ip", ip)
		http.Error(w, `{"error":"too many attempts, try again later"}`, http.StatusTooManyRequests)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	userID, err := AuthenticateUser(app.db, req.Username, req.Password)
	if err != nil {
		app.limiter.RecordFailure(ip)
		slog.Warn("login failed", "username", req.Username, "ip", ip)
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	token, err := CreateSession(app.db, userID, req.Remember)
	if err != nil {
		slog.Error("failed to create session", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	app.limiter.Reset(ip)
	app.setSessionCookie(w, r, token, req.Remember)
	slog.Info("login successful", "username", req.Username, "ip", ip)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (app *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		DeleteSession(app.db, cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	slog.Info("logout")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// --- Page Handlers ---

func (app *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Redirect to login if no session
	cookie, err := r.Cookie("session")
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if _, err := ValidateSession(app.db, cookie.Value); err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	tmpl, err := template.ParseFS(staticFiles, "static/index.html")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func (app *App) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]bool{
		"NeedsSetup": !UserExists(app.db),
	}
	tmpl, err := template.ParseFS(staticFiles, "static/login.html")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, data)
}

// --- Device Handlers ---

func (app *App) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := ListDevices(app.db)
	if err != nil {
		slog.Error("failed to list devices", "error", err)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

func (app *App) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	var d Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	id, err := CreateDevice(app.db, d)
	if err != nil {
		slog.Error("failed to create device", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	slog.Info("device created", "name", d.Name, "mac", d.MACAddress)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

func (app *App) handleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	var d Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if err := UpdateDevice(app.db, id, d); err != nil {
		slog.Error("failed to update device", "error", err, "id", id)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	slog.Info("device updated", "id", id, "name", d.Name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (app *App) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	if err := DeleteDevice(app.db, id); err != nil {
		slog.Error("failed to delete device", "error", err, "id", id)
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("device deleted", "id", id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (app *App) handleWakeDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	device, err := GetDevice(app.db, id)
	if err != nil {
		http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
		return
	}

	if err := WakeDevice(app.sender, device.MACAddress, "255.255.255.255", device.Port); err != nil {
		slog.Error("failed to send WoL packet", "error", err, "device", device.Name)
		http.Error(w, `{"error":"failed to send WoL packet"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("WoL packet sent", "device", device.Name, "mac", device.MACAddress, "port", device.Port)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (app *App) handleDeviceStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	device, err := GetDevice(app.db, id)
	if err != nil {
		http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
		return
	}

	status := "offline"
	if CheckDeviceStatus(device.IPAddress, device.StatusPort) {
		status = "online"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": status})
}

func (app *App) handleNetworkScan(w http.ResponseWriter, r *http.Request) {
	slog.Info("network scan requested")

	entries, err := ScanNetwork()
	if err != nil {
		slog.Error("network scan failed", "error", err)
		http.Error(w, `{"error":"scan failed"}`, http.StatusInternalServerError)
		return
	}

	slog.Info("network scan complete", "devices_found", len(entries))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build -o simple-wol .`
Expected: compiles

- [ ] **Step 4: Write integration test for auth handlers**

Append to `auth_test.go`:
```go
import (
	"net/http"
	"net/http/httptest"
	"strings"
)

func TestSetupAndLoginFlow(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	app := &App{
		db:      db,
		sender:  &mockSender{},
		limiter: NewRateLimiter(5, 10*time.Minute, 15*time.Minute),
	}

	// Setup
	body := `{"username":"admin","password":"testpass"}`
	req := httptest.NewRequest("POST", "/api/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.handleSetup(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("setup: expected 201, got %d", w.Code)
	}

	// Setup again should fail with 409
	req = httptest.NewRequest("POST", "/api/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	app.handleSetup(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate setup: expected 409, got %d", w.Code)
	}

	// Login with correct credentials
	body = `{"username":"admin","password":"testpass","remember":false}`
	req = httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.1:12345"
	w = httptest.NewRecorder()
	app.handleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", w.Code)
	}

	// Check session cookie was set
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "session" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected session cookie to be set")
	}

	// Login with wrong password
	body = `{"username":"admin","password":"wrong"}`
	req = httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.1:12345"
	w = httptest.NewRecorder()
	app.handleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: expected 401, got %d", w.Code)
	}
}
```

Note: merge the `import` block with existing imports in `auth_test.go`. The `mockSender` type is defined in `wol_test.go` (Task 6) — both are in the `main` package so it's accessible.

- [ ] **Step 5: Run integration test**

Run: `go test -v -run TestSetupAndLoginFlow .`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add main.go static/ auth_test.go
git commit -m "feat: HTTP server with all routes, auth middleware, handlers, and integration tests"
```

---

### Task 9: Frontend — Login Page

**Files:**
- Modify: `static/login.html`
- Modify: `static/style.css`

- [ ] **Step 1: Create login page with setup/login forms**

Replace `static/login.html` with the full login page HTML including:
- Setup form (shown when `NeedsSetup` is true via Go template)
- Login form with username, password, and "remember me" checkbox
- Inline `<script>` for `handleSetup()` and `handleLogin()` functions using `fetch`

See spec section "Frontend > Login Page" for requirements.

- [ ] **Step 2: Create base CSS**

Replace `static/style.css` with styles covering:
- Dark theme (`#1a1a2e` background, `#eee` text, `#e94560` accent)
- Form inputs, buttons (primary, secondary, danger variants)
- Device table, status indicators (offline=grey, waking=pulsing yellow, online=green)
- Toolbar, inline form, scan results, responsive layout
- `@keyframes pulse` animation for waking status

- [ ] **Step 3: Verify login page renders**

Run: `go run .`
Open `http://localhost:8080/login` — should show setup form on first run.
Stop the server.

- [ ] **Step 4: Commit**

```bash
git add static/login.html static/style.css
git commit -m "feat: login page with setup flow and base CSS"
```

---

### Task 10: Frontend — Main Page

**Files:**
- Modify: `static/index.html`
- Modify: `static/app.js`

- [ ] **Step 1: Create main page HTML**

Replace `static/index.html` with the device management page including:
- Header with title and logout button
- Toolbar: "Add Device" and "Scan Network" buttons
- Inline add-device form (hidden by default)
- Scan results section (hidden by default)
- Device table with columns: Status, Name, MAC, IP, Actions (Wake/Edit/Del)
- Empty state message

- [ ] **Step 2: Create app.js**

Replace `static/app.js` with all client-side logic:
- `api()` helper that redirects to `/login` on 401
- `loadDevices()` and `renderDevices()` — fetch and render device list
- `escapeHtml()` — prevent XSS by creating a text node and reading `.innerHTML` (safe DOM-based escaping, not string replacement)
- `addDevice()`, `editDevice()`, `deleteDevice()` — CRUD operations
- `wakeDevice()` — send WoL, start status polling
- `checkStatus()` and `pollStatus()` — TCP status check with 3s interval, 60s timeout
- `scanNetwork()`, `addSelectedDevices()` — network scan UI
- `logout()` — clear session

**XSS Note:** All user-supplied data (device names, MACs, IPs) MUST be escaped before DOM insertion. The `escapeHtml()` function uses safe DOM-based escaping (`textContent` assignment then reading `innerHTML`), not string replacement.

- [ ] **Step 3: Test end-to-end flow**

Run: `go run .`
1. Open `http://localhost:8080` — redirects to `/login`
2. Create account
3. Login
4. Add a device manually
5. Verify device appears in table
Stop the server.

- [ ] **Step 4: Commit**

```bash
git add static/index.html static/app.js
git commit -m "feat: main page with device management, WoL, status polling, and network scan UI"
```

---

### Task 11: Docker & Deployment

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`

- [ ] **Step 1: Create Dockerfile**

Create `Dockerfile` with multi-stage build:
- Stage 1 (`golang:1.22-alpine`): copy sources, `go mod download`, `CGO_ENABLED=0 go build`
- Stage 2 (`alpine:3.19`): `apk add --no-cache ca-certificates tzdata`, copy binary, set `DATA_DIR=/data`, expose 8080

- [ ] **Step 2: Create docker-compose.yml**

Per spec: `network_mode: host`, `cap_add: NET_RAW`, volume `./data:/data`, env `PORT=8080` and `DATA_DIR=/data`, `restart: unless-stopped`.

- [ ] **Step 3: Verify Docker build**

Run: `docker build -t simple-wol .`
Expected: builds successfully

- [ ] **Step 4: Commit**

```bash
git add Dockerfile docker-compose.yml
git commit -m "feat: Docker multi-stage build and docker-compose with host networking"
```

---

### Task 12: Proxmox LXC Scripts

**Files:**
- Create: `proxmox/ct/simple-wol.sh`
- Create: `proxmox/install/simple-wol-install.sh`

- [ ] **Step 1: Create LXC container creation script**

Create `proxmox/ct/simple-wol.sh` following community-scripts conventions:
- Default config: 1 CPU, 256MB RAM, 2GB disk, Debian 12
- Color output, user prompts for customization
- Creates unprivileged=0 (privileged) container with nesting for Docker
- Copies and runs install script inside the container

- [ ] **Step 2: Create install script**

Create `proxmox/install/simple-wol-install.sh`:
- Updates system, installs Docker via `get.docker.com`
- Creates `/opt/simple-wol/docker-compose.yml` pointing to `ghcr.io/ajustinjames/simple-wol:latest`
- Runs `docker compose up -d`
- Idempotent (safe to re-run)

- [ ] **Step 3: Make scripts executable and commit**

```bash
chmod +x proxmox/ct/simple-wol.sh proxmox/install/simple-wol-install.sh
git add proxmox/
git commit -m "feat: Proxmox LXC container creation and install scripts"
```

---

### Task 13: README & Final Docs

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README**

Replace with: project description, features list, quick start (Docker and Proxmox LXC), configuration table (PORT, DATA_DIR), development commands.

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update README with setup instructions and feature list"
```

---

### Task 14: Full Test Suite Verification

- [ ] **Step 1: Run full test suite**

Run: `go test -v ./...`
Expected: ALL PASS

- [ ] **Step 2: Build Docker image**

Run: `docker build -t simple-wol .`
Expected: builds successfully

- [ ] **Step 3: Run go vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 4: Manual smoke test**

Run: `go run .`
Full flow: setup account, login, add device, wake, check status, scan network, logout.
Stop the server.

- [ ] **Step 5: Final commit if any fixes needed**

```bash
git add -A
git commit -m "chore: final cleanup and verification"
```
