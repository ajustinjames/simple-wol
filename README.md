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

```bash
git clone https://github.com/ajustinjames/simple-wol.git
cd simple-wol
docker compose up -d
```

Open `http://localhost:8080` and create your admin account.

> **Note:** `network_mode: host` is required for WoL broadcast packets to reach your LAN.

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

## Development

```bash
go run .              # Run the server
go test ./...         # Run all tests
go build -o simple-wol .  # Build the binary
```
