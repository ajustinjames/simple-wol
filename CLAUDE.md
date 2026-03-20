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
