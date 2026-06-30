package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestGenerateAPIToken(t *testing.T) {
	tok1, err := generateAPIToken()
	if err != nil {
		t.Fatalf("generateAPIToken failed: %v", err)
	}
	tok2, err := generateAPIToken()
	if err != nil {
		t.Fatalf("generateAPIToken failed: %v", err)
	}
	if tok1 == "" || tok2 == "" {
		t.Fatal("expected non-empty tokens")
	}
	if tok1 == tok2 {
		t.Error("expected distinct tokens on each call")
	}
	if !strings.HasPrefix(tok1, "wol_") {
		t.Errorf("expected token to have wol_ prefix, got %q", tok1)
	}
}

func TestHashAPIToken_Deterministic(t *testing.T) {
	h1 := hashAPIToken("abc123")
	h2 := hashAPIToken("abc123")
	if h1 != h2 {
		t.Error("expected hash to be deterministic for same input")
	}
	if hashAPIToken("different") == h1 {
		t.Error("expected different inputs to hash differently")
	}
}

func TestCreateAPIToken(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := CreateUser(db, "alice", "password123"); err != nil {
		t.Fatal(err)
	}
	userID, _ := AuthenticateUser(db, "alice", "password123")

	raw, tok, err := CreateAPIToken(db, userID, "home-assistant")
	if err != nil {
		t.Fatalf("CreateAPIToken failed: %v", err)
	}
	if raw == "" {
		t.Fatal("expected non-empty raw token")
	}
	if tok.ID == 0 {
		t.Error("expected non-zero token ID")
	}
	if tok.Name != "home-assistant" {
		t.Errorf("expected name 'home-assistant', got %q", tok.Name)
	}

	// Raw token must not be stored verbatim.
	var storedHash string
	err = db.QueryRow("SELECT token_hash FROM api_tokens WHERE id = ?", tok.ID).Scan(&storedHash)
	if err != nil {
		t.Fatalf("failed to query stored token: %v", err)
	}
	if storedHash == raw {
		t.Error("raw token must not be stored verbatim")
	}
	if storedHash != hashAPIToken(raw) {
		t.Error("stored hash does not match expected hash of raw token")
	}
}

func TestValidateAPIToken_Valid(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	CreateUser(db, "alice", "password123")
	userID, _ := AuthenticateUser(db, "alice", "password123")
	raw, _, err := CreateAPIToken(db, userID, "test-token")
	if err != nil {
		t.Fatal(err)
	}

	gotUserID, err := ValidateAPIToken(db, raw)
	if err != nil {
		t.Fatalf("ValidateAPIToken failed for valid token: %v", err)
	}
	if gotUserID != userID {
		t.Errorf("expected userID %d, got %d", userID, gotUserID)
	}
}

func TestValidateAPIToken_UpdatesLastUsed(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	CreateUser(db, "alice", "password123")
	userID, _ := AuthenticateUser(db, "alice", "password123")
	raw, tok, err := CreateAPIToken(db, userID, "test-token")
	if err != nil {
		t.Fatal(err)
	}

	var lastUsedBefore *string
	db.QueryRow("SELECT last_used_at FROM api_tokens WHERE id = ?", tok.ID).Scan(&lastUsedBefore)
	if lastUsedBefore != nil {
		t.Fatal("expected last_used_at to be NULL before first use")
	}

	if _, err := ValidateAPIToken(db, raw); err != nil {
		t.Fatalf("ValidateAPIToken failed: %v", err)
	}

	var lastUsedAfter *string
	db.QueryRow("SELECT last_used_at FROM api_tokens WHERE id = ?", tok.ID).Scan(&lastUsedAfter)
	if lastUsedAfter == nil {
		t.Error("expected last_used_at to be set after validation")
	}
}

func TestValidateAPIToken_Revoked(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	CreateUser(db, "alice", "password123")
	userID, _ := AuthenticateUser(db, "alice", "password123")
	raw, tok, err := CreateAPIToken(db, userID, "test-token")
	if err != nil {
		t.Fatal(err)
	}

	if err := RevokeAPIToken(db, tok.ID); err != nil {
		t.Fatalf("RevokeAPIToken failed: %v", err)
	}

	if _, err := ValidateAPIToken(db, raw); err == nil {
		t.Fatal("expected ValidateAPIToken to fail for revoked token")
	}
}

func TestValidateAPIToken_InvalidToken(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := ValidateAPIToken(db, "not-a-real-token"); err == nil {
		t.Fatal("expected ValidateAPIToken to fail for unknown token")
	}
}

func TestRevokeAPIToken_NotFound(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := RevokeAPIToken(db, 9999); err == nil {
		t.Fatal("expected RevokeAPIToken to fail for nonexistent token")
	}
}

func TestRevokeAPIToken_Idempotent(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	CreateUser(db, "alice", "password123")
	userID, _ := AuthenticateUser(db, "alice", "password123")
	_, tok, err := CreateAPIToken(db, userID, "test-token")
	if err != nil {
		t.Fatal(err)
	}

	if err := RevokeAPIToken(db, tok.ID); err != nil {
		t.Fatalf("first revoke failed: %v", err)
	}
	if err := RevokeAPIToken(db, tok.ID); err != nil {
		t.Fatalf("second revoke (idempotent) should not fail: %v", err)
	}
}

func TestListAPITokens(t *testing.T) {
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	CreateUser(db, "alice", "password123")
	userID, _ := AuthenticateUser(db, "alice", "password123")
	CreateAPIToken(db, userID, "token-a")
	CreateAPIToken(db, userID, "token-b")

	tokens, err := ListAPITokens(db)
	if err != nil {
		t.Fatalf("ListAPITokens failed: %v", err)
	}
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}

// --- HTTP-level tests ---

func TestRequireAuth_BearerToken_Valid(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	userID, _ := AuthenticateUser(app.db, "admin", "testpass")
	raw, _, err := CreateAPIToken(app.db, userID, "ci-script")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/api/devices/1/wake", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	app.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for valid bearer token, got %d", w.Code)
	}
}

func TestRequireAuth_BearerToken_Revoked(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	userID, _ := AuthenticateUser(app.db, "admin", "testpass")
	raw, tok, err := CreateAPIToken(app.db, userID, "ci-script")
	if err != nil {
		t.Fatal(err)
	}
	if err := RevokeAPIToken(app.db, tok.ID); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/api/devices/1/wake", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	app.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for revoked bearer token, got %d", w.Code)
	}
}

func TestRequireAuth_BearerToken_Missing(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest("POST", "/api/devices/1/wake", nil)
	w := httptest.NewRecorder()
	app.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing auth, got %d", w.Code)
	}
}

func TestRequireAuth_BearerToken_Invalid(t *testing.T) {
	app := setupTestApp(t)

	req := httptest.NewRequest("POST", "/api/devices/1/wake", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	w := httptest.NewRecorder()
	app.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid bearer token, got %d", w.Code)
	}
}

func TestRequireAuth_BearerToken_BypassesCSRFCheck(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	userID, _ := AuthenticateUser(app.db, "admin", "testpass")
	raw, _, err := CreateAPIToken(app.db, userID, "ci-script")
	if err != nil {
		t.Fatal(err)
	}

	// POST request with Bearer auth and NO X-Requested-With header should still succeed.
	req := httptest.NewRequest("POST", "/api/devices/1/wake", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	app.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected token auth to bypass X-Requested-With check, got %d", w.Code)
	}
}

func TestRequireAuth_CookieAuth_StillEnforcesCSRF(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	uid, _ := AuthenticateUser(app.db, "admin", "testpass")
	sessionToken, _ := CreateSession(app.db, uid, false)

	// Cookie-authenticated POST without X-Requested-With must still be rejected.
	req := httptest.NewRequest("POST", "/api/devices", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	app.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected cookie auth to still enforce CSRF check, got %d", w.Code)
	}
}

// --- Token endpoint handler tests ---

func TestHandleCreateToken(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	userID, _ := AuthenticateUser(app.db, "admin", "testpass")

	body := `{"name":"my-script"}`
	req := httptest.NewRequest("POST", "/api/tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.handleCreateToken(w, req, userID)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected raw token in response")
	}
	if resp.Name != "my-script" {
		t.Errorf("expected name 'my-script', got %q", resp.Name)
	}
}

func TestHandleCreateToken_MissingName(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	userID, _ := AuthenticateUser(app.db, "admin", "testpass")

	body := `{"name":""}`
	req := httptest.NewRequest("POST", "/api/tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.handleCreateToken(w, req, userID)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", w.Code)
	}
}

func TestHandleListTokens(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	userID, _ := AuthenticateUser(app.db, "admin", "testpass")
	CreateAPIToken(app.db, userID, "token-a")

	req := httptest.NewRequest("GET", "/api/tokens", nil)
	w := httptest.NewRecorder()
	app.handleListTokens(w, req, userID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var tokens []APIToken
	if err := json.NewDecoder(w.Body).Decode(&tokens); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(tokens))
	}
}

func TestHandleRevokeToken(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	userID, _ := AuthenticateUser(app.db, "admin", "testpass")
	_, tok, _ := CreateAPIToken(app.db, userID, "token-a")

	idStr := strconv.FormatInt(tok.ID, 10)
	req := httptest.NewRequest("DELETE", "/api/tokens/"+idStr, nil)
	req.SetPathValue("id", idStr)
	w := httptest.NewRecorder()
	app.handleRevokeToken(w, req, userID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleRevokeToken_NotFound(t *testing.T) {
	app := setupTestApp(t)
	CreateUser(app.db, "admin", "testpass")
	userID, _ := AuthenticateUser(app.db, "admin", "testpass")

	req := httptest.NewRequest("DELETE", "/api/tokens/9999", nil)
	req.SetPathValue("id", "9999")
	w := httptest.NewRecorder()
	app.handleRevokeToken(w, req, userID)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
