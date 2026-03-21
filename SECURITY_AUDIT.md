# Security Audit Findings

Audit performed on 2026-03-21 against commit `08b7aac`.

---

## Issue 1: No Password Strength Requirements

**Severity:** Medium
**File:** `auth.go:26-36`, `main.go:134-163`

The `/api/setup` endpoint accepts any non-empty password with no minimum length, complexity, or entropy requirements. Additionally, bcrypt silently truncates passwords longer than 72 bytes, meaning a user who enters a 100-character password only has the first 72 bytes verified on login.

**Recommendations:**
- Enforce a minimum password length (e.g., 8-12 characters)
- Reject or warn on passwords exceeding 72 bytes
- Consider checking against common password lists

---

## Issue 2: Missing HTTP Security Headers

**Severity:** High
**File:** `main.go`

No security headers are set on any response. The application is missing:

- `Content-Security-Policy` — no CSP means inline scripts (used in `login.html`) run without restriction
- `X-Content-Type-Options: nosniff` — browsers may MIME-sniff responses
- `X-Frame-Options: DENY` — the app can be embedded in iframes (clickjacking)
- `Strict-Transport-Security` — no HSTS even when served behind TLS
- `Referrer-Policy` — referrer may leak session info in URLs

**Recommendations:**
- Add a middleware that sets security headers on all responses
- Set a strict CSP that disallows `unsafe-inline` (move inline scripts in `login.html` to a separate file or use nonces)

---

## Issue 3: No CSRF Protection

**Severity:** Medium
**File:** `main.go:112-130`

State-changing endpoints (`POST /api/devices`, `DELETE /api/devices/{id}`, `POST /api/devices/{id}/wake`, etc.) rely solely on session cookies for authentication with no CSRF token validation. The `SameSite=Strict` cookie attribute provides partial mitigation but is not supported consistently across all browsers and does not protect against same-site attacks.

**Recommendations:**
- Implement CSRF tokens (e.g., synchronizer token pattern or double-submit cookie)
- Alternatively, require a custom header (e.g., `X-Requested-With`) that cannot be set by simple cross-origin form submissions

---

## Issue 4: No Request Body Size Limits

**Severity:** Medium
**File:** `main.go:144`, `main.go:178`, `main.go:282`, `main.go:310`

All JSON request body parsing uses `json.NewDecoder(r.Body).Decode()` with no size limit. An attacker can send arbitrarily large request bodies to exhaust server memory.

**Recommendations:**
- Wrap `r.Body` with `http.MaxBytesReader(w, r.Body, maxSize)` before decoding
- A reasonable limit for this application would be 1MB or less

---

## Issue 5: Expired Sessions Never Cleaned from Database

**Severity:** Low
**File:** `auth.go:65-89`

Sessions are created with an expiration time and validated against it, but expired rows are never deleted from the `sessions` table. Over time this causes unbounded database growth. The session tokens themselves remain in the database indefinitely.

**Recommendations:**
- Add a periodic background goroutine that runs `DELETE FROM sessions WHERE expires_at < NOW()` on a regular interval (e.g., hourly)
- Alternatively, clean up expired sessions during login or validation

---

## Issue 6: In-Memory Rate Limiter Memory Leak and Reset on Restart

**Severity:** Medium
**File:** `auth.go:96-158`

The `RateLimiter` stores login attempts in an in-memory map with no eviction of old entries. If many distinct IPs trigger failed logins, the map grows without bound. Additionally, all rate limiting state is lost when the server restarts, allowing an attacker to bypass lockouts by waiting for a restart.

**Recommendations:**
- Add a periodic cleanup goroutine that evicts entries older than the window/lockout duration
- Consider persisting rate limit state to the database for durability across restarts
- Consider limiting the maximum number of tracked IPs to prevent memory exhaustion

---

## Issue 7: Docker Container Runs as Root

**Severity:** Medium
**File:** `Dockerfile`

The runtime container does not create or switch to a non-root user. The application runs as root inside the container, which increases the blast radius if the application is compromised.

**Recommendations:**
- Add a non-root user in the Dockerfile and switch to it:
  ```dockerfile
  RUN addgroup -S appgroup && adduser -S appuser -G appgroup
  USER appuser
  ```
- Ensure the data volume is owned by the non-root user

---

## Issue 8: No TLS / HTTPS Configuration

**Severity:** Medium
**File:** `main.go:81`

The server listens on plain HTTP (`http.ListenAndServe`). Session cookies, passwords, and all API traffic are transmitted in cleartext. While the cookie has a conditional `Secure` flag based on `X-Forwarded-Proto`, there is no built-in TLS support or documentation about requiring a reverse proxy.

**Recommendations:**
- Add native TLS support via `http.ListenAndServeTLS` with configurable cert/key paths
- Alternatively, document that a TLS-terminating reverse proxy (nginx, Caddy, Traefik) is required for production use
- Consider redirecting HTTP to HTTPS when TLS is enabled

---

## Issue 9: SanitizeName Double-Encoding Causes Display Bugs

**Severity:** Low
**File:** `devices.go:57-64`, `static/app.js:80`

`SanitizeName()` performs server-side HTML entity encoding (`<` to `&lt;`, etc.) before storing in the database. The frontend renders device names using `textContent`, which auto-escapes HTML. This double-encoding means a device named `My <PC>` is stored as `My &lt;PC&gt;` and displayed literally as `My &lt;PC&gt;` instead of `My <PC>`.

**Recommendations:**
- Remove the HTML encoding from `SanitizeName()` — just trim whitespace and optionally restrict to allowed characters
- Let the frontend handle escaping at render time (which it already does via `textContent`)
- If server-side rendering is needed in the future, encode at render time, not storage time

---

## Issue 10: Set Up Dependabot and GitHub Security Tools

**Severity:** Informational
**Category:** DevOps / Supply Chain Security

The repository has no automated dependency monitoring or security scanning configured. The following free GitHub tools should be enabled:

### Dependabot
- **Dependabot version updates:** Automatically create PRs to keep Go modules up to date
- **Dependabot security alerts:** Get notified of known vulnerabilities in dependencies
- Add a `.github/dependabot.yml` file:
  ```yaml
  version: 2
  updates:
    - package-ecosystem: "gomod"
      directory: "/"
      schedule:
        interval: "weekly"
    - package-ecosystem: "docker"
      directory: "/"
      schedule:
        interval: "weekly"
  ```

### CodeQL / Code Scanning
- Enable GitHub's CodeQL analysis for Go to catch security issues in PRs
- Add a `.github/workflows/codeql.yml` workflow

### Branch Protection
- Enable branch protection on `main` requiring PR reviews and status checks
- Require signed commits

### Secret Scanning
- Enable GitHub secret scanning to detect accidentally committed credentials
- Enable push protection to block pushes containing secrets

### Additional Recommendations
- Add a `SECURITY.md` file with a vulnerability disclosure policy
- Consider enabling GitHub's dependency review action to flag vulnerable dependencies in PRs
