package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
		errMsg   string
	}{
		{"empty password", "", true, "password must be at least 8 characters"},
		{"too short", "abc", true, "password must be at least 8 characters"},
		{"7 chars", "abcdefg", true, "password must be at least 8 characters"},
		{"exactly 8 chars", "abcdefgh", false, ""},
		{"at 72 bytes", strings.Repeat("a", 72), false, ""},
		{"exceeds 72 bytes", strings.Repeat("a", 73), true, "password must not exceed 72 bytes"},
		{"multibyte under 72 chars but over 72 bytes", strings.Repeat("日", 25), true, "password must not exceed 72 bytes"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.password)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if err.Error() != tc.errMsg {
					t.Errorf("expected %q, got %q", tc.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret")); err != nil {
		t.Fatalf("bcrypt.CompareHashAndPassword failed: %v", err)
	}
}

func TestCheckPassword(t *testing.T) {
	hash, err := HashPassword("mypassword")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !CheckPassword(hash, "mypassword") {
		t.Error("expected CheckPassword to return true for correct password")
	}
	if CheckPassword(hash, "wrongpassword") {
		t.Error("expected CheckPassword to return false for wrong password")
	}
}

func TestCreateUser(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	if err := CreateUser(db, "alice", "password123"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	var username string
	err = db.QueryRow("SELECT username FROM users WHERE username = ?", "alice").Scan(&username)
	if err != nil {
		t.Fatalf("user not found in DB: %v", err)
	}
	if username != "alice" {
		t.Errorf("expected username 'alice', got '%s'", username)
	}
}

func TestCreateUser_DuplicateFails(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	if err := CreateUser(db, "alice", "password123"); err != nil {
		t.Fatalf("first CreateUser failed: %v", err)
	}
	if err := CreateUser(db, "alice", "different"); err == nil {
		t.Fatal("expected duplicate CreateUser to fail, but it succeeded")
	}
}

func TestUserExists(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	if UserExists(db) {
		t.Error("expected UserExists to return false on empty DB")
	}

	if err := CreateUser(db, "alice", "password123"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if !UserExists(db) {
		t.Error("expected UserExists to return true after user creation")
	}
}

// --- Task 4: Sessions & Rate Limiting ---

func TestCreateSession(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	if err := CreateUser(db, "alice", "password123"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	userID, err := AuthenticateUser(db, "alice", "password123")
	if err != nil {
		t.Fatalf("AuthenticateUser failed: %v", err)
	}

	token, err := CreateSession(db, userID, false)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty session token")
	}
}

func TestValidateSession(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	if err := CreateUser(db, "alice", "password123"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	userID, err := AuthenticateUser(db, "alice", "password123")
	if err != nil {
		t.Fatalf("AuthenticateUser failed: %v", err)
	}

	token, err := CreateSession(db, userID, false)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	gotUserID, err := ValidateSession(db, token)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}
	if gotUserID != userID {
		t.Errorf("expected userID %d, got %d", userID, gotUserID)
	}
}

func TestValidateSession_InvalidToken(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	_, err = ValidateSession(db, "invalidtoken")
	if err == nil {
		t.Fatal("expected ValidateSession with invalid token to fail")
	}
}

func TestDeleteSession(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	if err := CreateUser(db, "alice", "password123"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	userID, err := AuthenticateUser(db, "alice", "password123")
	if err != nil {
		t.Fatalf("AuthenticateUser failed: %v", err)
	}

	token, err := CreateSession(db, userID, false)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if err := DeleteSession(db, token); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	_, err = ValidateSession(db, token)
	if err == nil {
		t.Fatal("expected ValidateSession to fail after session deleted")
	}
}

func TestCleanExpiredSessions(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	CreateUser(db, "alice", "password123")
	uid, _ := AuthenticateUser(db, "alice", "password123")

	// Create a valid session
	validToken, _ := CreateSession(db, uid, false)

	// Insert an expired session directly
	_, err = db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		"expired-token", uid, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	count, err := CleanExpiredSessions(db)
	if err != nil {
		t.Fatalf("CleanExpiredSessions failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 deleted, got %d", count)
	}

	// Valid session should still exist
	if _, err := ValidateSession(db, validToken); err != nil {
		t.Error("valid session should not have been deleted")
	}

	// Expired session should be gone
	if _, err := ValidateSession(db, "expired-token"); err == nil {
		t.Error("expired session should have been deleted")
	}
}

func TestCleanExpiredSessions_NoExpired(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	count, err := CleanExpiredSessions(db)
	if err != nil {
		t.Fatalf("CleanExpiredSessions failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 deleted, got %d", count)
	}
}

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute, time.Minute)
	ip := "192.168.1.1"

	for i := 0; i < 4; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("expected Allow to return true on attempt %d", i+1)
		}
		rl.RecordFailure(ip)
	}
	if !rl.Allow(ip) {
		t.Error("expected Allow to return true when under failure limit")
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute, time.Minute)
	ip := "192.168.1.2"

	for i := 0; i < 5; i++ {
		rl.RecordFailure(ip)
	}

	if rl.Allow(ip) {
		t.Error("expected Allow to return false after reaching failure limit")
	}
}

func TestRateLimiter_ResetOnSuccess(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute, time.Minute)
	ip := "192.168.1.3"

	for i := 0; i < 5; i++ {
		rl.RecordFailure(ip)
	}
	if rl.Allow(ip) {
		t.Fatal("expected blocked after 5 failures")
	}

	rl.Reset(ip)

	if !rl.Allow(ip) {
		t.Error("expected Allow to return true after reset")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute, time.Minute)
	ip1 := "10.0.0.1"
	ip2 := "10.0.0.2"

	for i := 0; i < 5; i++ {
		rl.RecordFailure(ip1)
	}

	if rl.Allow(ip1) {
		t.Error("expected ip1 to be blocked")
	}
	if !rl.Allow(ip2) {
		t.Error("expected ip2 to be allowed (independent tracking)")
	}
}

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
