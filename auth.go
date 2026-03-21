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

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len([]byte(password)) > 72 {
		return fmt.Errorf("password must not exceed 72 bytes")
	}
	return nil
}

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
	_, err = db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return token, nil
}

func ValidateSession(db *sql.DB, token string) (int64, error) {
	var userID int64
	err := db.QueryRow("SELECT user_id FROM sessions WHERE token = ? AND expires_at > ?", token, time.Now()).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("invalid session")
	}
	return userID, nil
}

func CleanExpiredSessions(db *sql.DB) (int64, error) {
	result, err := db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func DeleteSession(db *sql.DB, token string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

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
	if !a.lockedAt.IsZero() {
		if time.Since(a.lockedAt) > rl.lockout {
			delete(rl.attempts, ip)
			return true
		}
		return false
	}
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
