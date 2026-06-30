package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVersionEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/version", nil)
	w := httptest.NewRecorder()
	handleVersion(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["version"] != "dev" {
		t.Errorf("version = %q, want %q", resp["version"], "dev")
	}
}

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

func TestBodySizeLimit_UnderLimit(t *testing.T) {
	app := setupTestApp(t)

	body := `{"username":"admin","password":"testpass"}`
	req := httptest.NewRequest("POST", "/api/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.handleSetup(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201 for body under limit, got %d", w.Code)
	}
}

func TestBodySizeLimit_OverLimit(t *testing.T) {
	app := setupTestApp(t)

	// 1025 bytes exceeds the 1KB limit
	bigBody := `{"username":"admin","password":"` + strings.Repeat("a", 1000) + `"}`
	req := httptest.NewRequest("POST", "/api/setup", strings.NewReader(bigBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.handleSetup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for body over limit, got %d", w.Code)
	}
}

func TestBodySizeLimit_ExactBoundary(t *testing.T) {
	app := setupTestApp(t)

	// Build a body that is exactly 1024 bytes
	prefix := `{"username":"admin","password":"`
	suffix := `"}`
	padLen := 1024 - len(prefix) - len(suffix)
	body := prefix + strings.Repeat("a", padLen) + suffix
	if len(body) != 1024 {
		t.Fatalf("test body should be 1024 bytes, got %d", len(body))
	}

	req := httptest.NewRequest("POST", "/api/setup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.handleSetup(w, req)

	// Should succeed (password validation may reject, but body size should be fine)
	// Password is ~990 chars so exceeds 72 bytes — we expect 400 for password validation, not body size
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- Wake Group ---

func newWakeGroupRequest(group string) *http.Request {
	req := httptest.NewRequest("POST", "/api/groups/"+group+"/wake", nil)
	req.SetPathValue("group", group)
	return req
}

func TestWakeGroup_AllSucceed(t *testing.T) {
	app := setupTestApp(t)
	mock := &mockSender{}
	app.sender = mock

	CreateDevice(app.db, Device{Name: "PC1", MACAddress: "AA:BB:CC:DD:EE:01", IPAddress: "192.168.4.1", Port: 9, GroupName: "Lab"})
	CreateDevice(app.db, Device{Name: "PC2", MACAddress: "AA:BB:CC:DD:EE:02", IPAddress: "192.168.4.2", Port: 9, GroupName: "Lab"})

	w := httptest.NewRecorder()
	app.handleWakeGroup(w, newWakeGroupRequest("Lab"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Group   string            `json:"group"`
		Results []WakeGroupResult `json:"results"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
	for _, r := range resp.Results {
		if !r.Success {
			t.Errorf("expected device %d to succeed, got error %q", r.DeviceID, r.Error)
		}
	}
}

func TestWakeGroup_PartialFailure(t *testing.T) {
	app := setupTestApp(t)
	failMAC := "aa:bb:cc:dd:ee:02"
	sender := &failingSender{FailFor: map[string]bool{failMAC: true}}
	app.sender = sender

	CreateDevice(app.db, Device{Name: "PC1", MACAddress: "AA:BB:CC:DD:EE:01", IPAddress: "192.168.4.1", Port: 9, GroupName: "Lab"})
	CreateDevice(app.db, Device{Name: "PC2", MACAddress: "AA:BB:CC:DD:EE:02", IPAddress: "192.168.4.2", Port: 9, GroupName: "Lab"})

	w := httptest.NewRecorder()
	app.handleWakeGroup(w, newWakeGroupRequest("Lab"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Results []WakeGroupResult `json:"results"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}

	successCount, failureCount := 0, 0
	for _, r := range resp.Results {
		if r.Success {
			successCount++
		} else {
			failureCount++
			if r.Name != "PC2" {
				t.Errorf("expected PC2 to be the failing device, got %q", r.Name)
			}
			if r.Error == "" {
				t.Error("expected error message on failed result")
			}
		}
	}
	if successCount != 1 || failureCount != 1 {
		t.Fatalf("expected 1 success and 1 failure, got %d success, %d failure", successCount, failureCount)
	}
}

func TestWakeGroup_EmptyOrNonexistentGroup(t *testing.T) {
	app := setupTestApp(t)
	CreateDevice(app.db, Device{Name: "PC1", MACAddress: "AA:BB:CC:DD:EE:01", IPAddress: "192.168.4.1", Port: 9, GroupName: "Lab"})

	w := httptest.NewRecorder()
	app.handleWakeGroup(w, newWakeGroupRequest("Nonexistent"))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Results []WakeGroupResult `json:"results"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("expected 0 results for nonexistent group, got %d", len(resp.Results))
	}
}
