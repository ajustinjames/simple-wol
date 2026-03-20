# Simple WoL — Design Spec

## Overview

A self-hosted Wake-on-LAN web application deployable to a Proxmox home server. Provides a web GUI to send WoL magic packets to local devices and verify they come online. Primary use case: waking a Windows PC remotely via Tailscale.

## Stack

- **Language:** Go
- **Web framework:** `net/http` (standard library)
- **Database:** SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **Frontend:** Vanilla HTML/CSS/JS embedded in the binary via `embed` package
- **Deployment:** Docker container with optional Proxmox LXC provisioning scripts

## Project Structure

```
simple-wol/
├── main.go              # Entry point, server setup, routing
├── auth.go              # Password hashing, login, session middleware
├── auth_test.go         # Auth tests
├── devices.go           # Device CRUD handlers
├── devices_test.go      # Device CRUD tests
├── wol.go               # Wake-on-LAN packet sending logic
├── wol_test.go          # WoL packet tests
├── scan.go              # Network scanning (ping sweep + ARP table)
├── scan_test.go         # Network scan tests
├── db.go                # SQLite setup and queries
├── db_test.go           # DB tests
├── static/              # Embedded frontend files
│   ├── index.html       # Single page — device list + wake buttons
│   ├── login.html       # Login page
│   ├── style.css
│   └── app.js           # Minimal JS for API calls
├── Dockerfile
├── docker-compose.yml
├── .gitignore
├── CLAUDE.md
├── README.md
├── go.mod
├── go.sum
└── proxmox/
    ├── ct/simple-wol.sh              # LXC container creation script (runs on Proxmox host)
    └── install/simple-wol-install.sh  # App install script (runs inside the LXC)
```

## Authentication & Sessions

- Single user account (username + password).
- On first run, the app has no user. The UI shows a setup form to create one.
- Password hashed with **bcrypt** and stored in SQLite.
- Login via `POST /api/login` with username, password, and optional `remember` flag.
- Session token: cryptographically random, stored in SQLite, returned as a **secure HTTP-only cookie**.
- Default session expiry: **24 hours**. With "remember me": **30 days**.
- Auth middleware protects all `/api/*` routes except `/api/login` and `/api/setup`.
- Logout via `POST /api/logout` invalidates the session.
- **Login rate limiting:** 5 failed attempts within a 10-minute window locks out the IP for 15 minutes. Returns `429 Too Many Requests`. Tracked in-memory by IP address (resets on server restart). Resets on successful login.

### Database Schema — Users

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL
);
```

### Database Schema — Sessions

```sql
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id),
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## Device Management

- Devices stored in SQLite with name, MAC address, IP address, and WoL port.
- MAC address validated on input (format: `AA:BB:CC:DD:EE:FF` or `AA-BB-CC-DD-EE-FF`).
- IP address validated on input.
- Device names sanitized to prevent XSS.
- Port defaults to `9` but is configurable per device.

### Database Schema — Devices

```sql
CREATE TABLE devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    mac_address TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 9,
    status_port INTEGER NOT NULL DEFAULT 3389,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

- `port`: WoL magic packet destination port (default 9).
- `status_port`: TCP port used for online status checks (default 3389/RDP for Windows PCs).

## API Routes

All routes behind auth middleware unless noted.

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/api/setup` | Create initial user (first run only; returns `409 Conflict` if a user already exists) | No |
| POST | `/api/login` | Authenticate, return session cookie | No |
| POST | `/api/logout` | Invalidate session | Yes |
| GET | `/api/devices` | List all devices | Yes |
| POST | `/api/devices` | Add a device | Yes |
| PUT | `/api/devices/{id}` | Edit a device | Yes |
| DELETE | `/api/devices/{id}` | Delete a device | Yes |
| POST | `/api/devices/{id}/wake` | Send WoL packet | Yes |
| GET | `/api/devices/{id}/status` | Ping device, return online/offline | Yes |
| POST | `/api/network/scan` | Ping sweep local subnet, return discovered devices (IP + MAC) | Yes |

## Wake-on-LAN Logic

- Magic packet: 6 bytes of `0xFF` followed by the target MAC address repeated 16 times (102 bytes total).
- Sent as a **UDP broadcast** to `255.255.255.255` on the device's configured port (default `9`). The app assumes the host is on the same broadcast domain as target devices.
- `POST /api/devices/{id}/wake` sends the packet and returns immediately.
- The frontend then polls `GET /api/devices/{id}/status` every 3 seconds for up to 60 seconds.
- Status progression in the UI: **offline → waking → online** (or timeout).

### Packet Sender Interface

```go
type PacketSender interface {
    SendMagicPacket(macAddress string, broadcastAddr string, port int) error
}
```

The broadcast address is passed as a parameter to allow flexibility. The default implementation sends to `255.255.255.255`. This also allows tests to mock UDP sending.

### Status Check

- Uses TCP connect probe to a known port (e.g., RDP 3389 for Windows, or a configurable port per device) to check if a device is reachable. TCP connect does not require elevated privileges unlike ICMP ping.
- `GET /api/devices/{id}/status` returns `{"status": "online"}` or `{"status": "offline"}`.

## Network Scanning

- Manual only — triggered by the user clicking "Scan Network" in the UI.
- The server auto-detects its local subnet by reading the host's network interfaces (`net.Interfaces()`), using the first non-loopback interface with an IPv4 address and subnet mask.
- Performs a ping sweep across the detected subnet (e.g., `192.168.4.1-254` for a `/24`).
- After the sweep, reads `/proc/net/arp` to collect IP + MAC pairs for responding devices.
- Returns a list of discovered devices to the UI.
- The UI displays results in a table. The user selects devices to add, provides a name, and saves them. Port fields (WoL port, status port) use defaults and can be edited after.
- No automatic or periodic scanning — only runs when the user explicitly requests it.

## Frontend

### Login Page (`login.html`)

- Username and password fields.
- "Remember me" checkbox.
- On first run (no user exists), shows a "Create Account" form instead.

### Main Page (`index.html`)

- Device table: name, MAC, IP, port, status indicator, "Wake" button, edit/delete actions.
- "Add Device" button opens an inline form (name, MAC, IP, port with default 9, status port with default 3389).
- "Scan Network" button triggers a scan, shows discovered devices in a selectable list to add.
- Status indicators: grey (offline), pulsing yellow (waking), green (online).
- Logout button.
- Responsive layout for mobile use (Tailscale from phone).

### Tech

- Vanilla HTML/CSS/JS.
- `fetch` API for all backend calls.
- No build step.

## Docker Deployment

### Dockerfile

Multi-stage build:
1. **Builder stage:** Go image, compiles the binary.
2. **Final stage:** Alpine with `ca-certificates` and `tzdata` installed, copies the binary. All timestamps use UTC.

### docker-compose.yml

```yaml
services:
  simple-wol:
    build: .
    network_mode: host
    cap_add:
      - NET_RAW
    volumes:
      - ./data:/data
    environment:
      - PORT=8080
    restart: unless-stopped
```

- `network_mode: host` is required for WoL broadcast packets to reach the local LAN.
- SQLite database stored in `/data/simple-wol.db` (volume-mounted for persistence).
- `PORT` env var configures the listen port (default `8080`).

## Proxmox LXC Scripts

### `proxmox/ct/simple-wol.sh`

- Runs on the Proxmox host.
- Creates a Debian LXC container with defaults: 1 CPU, 256MB RAM, 1GB disk.
- Follows community-scripts conventions: color output, default variables, user prompts.
- Kicks off the install script inside the container.

### `proxmox/install/simple-wol-install.sh`

- Runs inside the LXC container.
- Installs Docker and Docker Compose.
- Pulls the simple-wol Docker image and starts it.
- Idempotent — safe to re-run for updates.

## Logging

- Uses Go's standard `log/slog` package (structured JSON logging).
- Logs to **stdout** — Docker captures via `docker logs`, no file management needed.
- Key events logged: server start/stop, login attempts (success/failure), WoL packets sent (device name + MAC), network scan requests, device CRUD operations, errors.
- No sensitive data in logs (no passwords, no session tokens).

## Testing

- Go standard `testing` package.
- Test files alongside source (`auth_test.go`, `devices_test.go`, `wol_test.go`, `db_test.go`).
- Tests cover: auth logic, device CRUD, WoL packet construction, DB operations.
- WoL send mocked via `PacketSender` interface.
- DB tests use in-memory SQLite.

## Security Considerations

- Passwords hashed with bcrypt (cost 10+).
- Session tokens are cryptographically random.
- Cookies: `HttpOnly`, `SameSite=Strict`. The `Secure` flag is set only when the request arrives over HTTPS (detected via `r.TLS != nil` or `X-Forwarded-Proto` header); over plain HTTP it is omitted to allow local network access.
- Input validation on MAC addresses, IP addresses, and port numbers.
- XSS prevention via input sanitization.
- No encryption at rest for device data (MAC addresses and names are non-sensitive; rely on host-level disk encryption if needed).
- Setup endpoint disabled after first user is created.
