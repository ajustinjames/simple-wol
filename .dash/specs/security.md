# Security

Living spec for authentication, authorization, and hardening decisions.

## Key Decisions

- **Hard reject passwords over 72 bytes** — No warning, just reject. Prevents bcrypt truncation surprise. No complexity requirements per NIST SP 800-63B; length is the primary factor. Validation in a standalone function for testability. (from GH-3)

- **Single security headers middleware wrapping the mux** — Ensures no endpoint is missed. Inline JS moved to separate file to avoid CSP nonces. HSTS only set when request is behind TLS. (from GH-4)

- **CSRF via `X-Requested-With` custom header in `requireAuth`** — No synchronizer tokens needed for a single-page JSON API. Check lives in `requireAuth` to avoid a second middleware. `X-Requested-With` chosen over custom `X-CSRF-Token` as a well-known convention. (from GH-5)

- **1KB body size limit via inline `MaxBytesReader`** — Largest payload is ~200 bytes. No middleware — inline calls are explicit and allow per-endpoint tuning. Generic "invalid request" error on overflow. (from GH-6)

- **Session cleanup via background goroutine at 1-hour intervals** — Keeps hot path fast. No graceful shutdown (app has none). Sessions last 24h so hourly is sufficient. (from GH-7)

- **Rate limiter: no IP cap, 10-minute cleanup, no persistence** — For a home tool, distinct attacker IPs are few. Cleanup interval matches window/lockout durations. No DB persistence — attacker who can restart server already has more access. (from GH-8)

- **Optional env-var-based TLS, no HTTP redirect, no ACME** — Documentation-first with ~10 lines of optional native TLS. Redirect and Let's Encrypt are out of scope for a LAN tool. (from GH-10)

- **SanitizeName: remove encoding entirely** — No server-side HTML rendering of device names. JS uses `textContent` which is XSS-safe. No character allowlist. No DB migration for existing encoded data. (from GH-11)
