package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSecurityHeaders(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	expected := map[string]string{
		"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Permissions-Policy":      "camera=(), microphone=(), geolocation=()",
	}

	for header, want := range expected {
		got := w.Header().Get(header)
		if got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	if hsts := w.Header().Get("Strict-Transport-Security"); hsts != "" {
		t.Errorf("HSTS should not be set on plain HTTP, got %q", hsts)
	}
}

func TestSecurityHeaders_HSTS(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	if hsts != "max-age=31536000; includeSubDomains" {
		t.Errorf("HSTS = %q, want %q", hsts, "max-age=31536000; includeSubDomains")
	}
}

func setupTestApp(t *testing.T) *App {
	t.Helper()
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &App{
		db:      db,
		sender:  &mockSender{},
		limiter: NewRateLimiter(5, 10*time.Minute, 15*time.Minute),
	}
}

func TestCSRF_PostWithoutHeader(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	uid, _ := AuthenticateUser(app.db, "admin", "testpass")
	token, _ := CreateSession(app.db, uid, false)

	req := httptest.NewRequest("POST", "/api/devices", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	w := httptest.NewRecorder()
	app.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 without X-Requested-With, got %d", w.Code)
	}
}

func TestCSRF_PostWithHeader(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	uid, _ := AuthenticateUser(app.db, "admin", "testpass")
	token, _ := CreateSession(app.db, uid, false)

	req := httptest.NewRequest("POST", "/api/devices", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w := httptest.NewRecorder()
	app.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with X-Requested-With, got %d", w.Code)
	}
}

func TestCSRF_GetWithoutHeader(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	uid, _ := AuthenticateUser(app.db, "admin", "testpass")
	token, _ := CreateSession(app.db, uid, false)

	req := httptest.NewRequest("GET", "/api/devices", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	w := httptest.NewRecorder()
	app.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for GET without header, got %d", w.Code)
	}
}
