package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
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
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		slog.Error("failed to create data directory", "path", dataDir, "error", err)
		os.Exit(1)
	}
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

	app.startSessionCleanup()
	app.limiter.StartCleanup()

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

	handler := securityHeaders(mux)
	addr := ":" + port

	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")

	if tlsCert != "" && tlsKey != "" {
		slog.Info("simple-wol starting with TLS", "port", port)
		if err := http.ListenAndServeTLS(addr, tlsCert, tlsKey, handler); err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	} else if tlsCert != "" || tlsKey != "" {
		slog.Error("both TLS_CERT and TLS_KEY must be set for TLS")
		os.Exit(1)
	} else {
		slog.Info("simple-wol starting", "port", port)
		if err := http.ListenAndServe(addr, handler); err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}
}

const maxRequestBody = 1024 // 1KB

func limitBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func (app *App) startSessionCleanup() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			count, err := CleanExpiredSessions(app.db)
			if err != nil {
				slog.Error("session cleanup failed", "error", err)
			} else if count > 0 {
				slog.Info("cleaned expired sessions", "count", count)
			}
		}
	}()
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		_, err = ValidateSession(app.db, cookie.Value)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		if r.Method != http.MethodGet && r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"forbidden"}`))
			return
		}
		next(w, r)
	}
}

// --- Auth Handlers ---

func (app *App) handleSetup(w http.ResponseWriter, r *http.Request) {
	limitBody(w, r)
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

	if err := ValidatePassword(req.Password); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
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
	limitBody(w, r)
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
	if err := tmpl.Execute(w, nil); err != nil {
		slog.Error("template execution failed", "error", err)
	}
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
	if err := tmpl.Execute(w, data); err != nil {
		slog.Error("template execution failed", "error", err)
	}
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
	limitBody(w, r)
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
	limitBody(w, r)
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
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		}
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
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, `{"error":"device not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		}
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
	if CheckDeviceStatus(device.IPAddress) {
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
