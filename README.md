# Simple WoL

A lightweight Wake-on-LAN web application for waking devices on your local network. Built as a single Go binary with an embedded web UI — no external dependencies.

## Features

- Wake devices via magic packet (WoL)
- Device management with status monitoring (online/offline/waking)
- Network scanning to discover devices on your LAN
- Dark-themed responsive web UI
- Single-user authentication with session management
- Login rate limiting
- SQLite database (no external DB required)
- Docker and Proxmox LXC deployment support

## Quick Start

### Docker (recommended)

**Production (Linux):**

```bash
git clone https://github.com/ajustinjames/simple-wol.git
cd simple-wol
docker compose up -d
```

> **Note:** `network_mode: host` is required for WoL broadcast packets to reach your LAN.

**Local development (macOS/Windows):**

```bash
docker compose -f docker-compose.local.yml up -d
```

> `network_mode: host` doesn't work on Docker Desktop. The local config uses port mapping instead. WoL packets won't reach your LAN, but the UI is fully functional.

**Local development with WoL (macOS/Windows):**

Docker Desktop runs containers in a Linux VM, so broadcast packets can never reach your physical LAN. To test WoL locally, run the server natively:

```bash
go run .
```

Open `http://localhost:8080` and create your admin account.

### Proxmox LXC

Run on your Proxmox host:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/ajustinjames/simple-wol/main/proxmox/ct/simple-wol.sh)"
```

This creates a privileged LXC container with Docker and starts Simple WoL automatically.

### From Source

```bash
go build -o simple-wol .
./simple-wol
```

## Configuration

| Variable   | Default | Description              |
|------------|---------|--------------------------|
| `PORT`     | `8080`  | HTTP listen port         |
| `DATA_DIR` | `data`  | Directory for SQLite DB  |
| `TLS_CERT` | —       | Path to TLS certificate file (enables HTTPS) |
| `TLS_KEY`  | —       | Path to TLS private key file (enables HTTPS) |

## Deployment

The application listens on HTTP by default (port 8080). For production use, you should serve it over HTTPS using one of these approaches:

### Reverse Proxy (recommended)

Place Simple WoL behind a TLS-terminating reverse proxy. The `Secure` cookie flag is automatically set when the proxy sends `X-Forwarded-Proto: https`.

**Caddy:**

```
wol.example.com {
    reverse_proxy localhost:8080
}
```

**nginx:**

```nginx
server {
    listen 443 ssl;
    server_name wol.example.com;
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Host $host;
    }
}
```

### Native TLS

Set both `TLS_CERT` and `TLS_KEY` environment variables to enable HTTPS directly:

```bash
TLS_CERT=/path/to/cert.pem TLS_KEY=/path/to/key.pem ./simple-wol
```

Both variables must be set together — setting only one will cause the server to exit with an error.

## Development

```bash
go run .              # Run the server
go test ./...         # Run all tests
go build -o simple-wol .  # Build the binary
```
