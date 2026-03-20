#!/usr/bin/env bash

# Simple WoL LXC Container Creation Script
# Compatible with Proxmox VE community-scripts conventions
# Creates a privileged LXC container and installs Simple WoL via Docker

set -euo pipefail

# ── Color helpers ───────────────────────────────────────────────────────────
GN="\033[1;32m"  # green
RD="\033[1;31m"  # red
YW="\033[1;33m"  # yellow
CL="\033[0m"     # reset

msg_ok()   { echo -e "${GN}[OK]${CL} $1"; }
msg_info() { echo -e "${YW}[INFO]${CL} $1"; }
msg_error(){ echo -e "${RD}[ERROR]${CL} $1"; }

# ── Header ──────────────────────────────────────────────────────────────────
echo -e "${GN}"
echo "  ┌──────────────────────────────────────────┐"
echo "  │     Simple WoL LXC Container Creator     │"
echo "  │        Wake-on-LAN Management App        │"
echo "  └──────────────────────────────────────────┘"
echo -e "${CL}"

# ── Verify running on Proxmox host ──────────────────────────────────────────
if ! command -v pct &>/dev/null; then
  msg_error "This script must be run on a Proxmox VE host."
  exit 1
fi

# ── Default configuration ───────────────────────────────────────────────────
NEXT_ID=$(pvesh get /cluster/nextid 2>/dev/null || echo "100")
CT_ID="${NEXT_ID}"
HOSTNAME="simple-wol"
DISK_SIZE="2"       # GB
RAM="256"           # MB
CORES="1"
TEMPLATE="debian-12"
STORAGE="local-lvm"

# ── Resolve template path ──────────────────────────────────────────────────
TEMPLATE_STORAGE="local"

resolve_template() {
  local match
  match=$(pveam list "$TEMPLATE_STORAGE" 2>/dev/null \
    | awk '/debian-12-standard/ {print $1}' \
    | sort -V | tail -n1)
  if [[ -z "$match" ]]; then
    msg_info "Downloading Debian 12 template..."
    pveam update >/dev/null 2>&1
    local dl
    dl=$(pveam available --section system 2>/dev/null \
      | awk '/debian-12-standard/ {print $2}' \
      | sort -V | tail -n1)
    if [[ -z "$dl" ]]; then
      msg_error "Could not find a Debian 12 template to download."
      exit 1
    fi
    pveam download "$TEMPLATE_STORAGE" "$dl" >/dev/null 2>&1
    match="${TEMPLATE_STORAGE}:vztmpl/${dl}"
  fi
  echo "$match"
}

# ── Display defaults and prompt ─────────────────────────────────────────────
echo ""
msg_info "Default configuration:"
echo "  Container ID : ${CT_ID}"
echo "  Hostname     : ${HOSTNAME}"
echo "  Disk size    : ${DISK_SIZE} GB"
echo "  RAM          : ${RAM} MB"
echo "  CPU cores    : ${CORES}"
echo "  Template     : ${TEMPLATE}"
echo "  Storage      : ${STORAGE}"
echo ""

read -rp "Use defaults? (Y/n): " USE_DEFAULTS
if [[ "${USE_DEFAULTS,,}" == "n" ]]; then
  read -rp "Container ID [${CT_ID}]: " input && CT_ID="${input:-$CT_ID}"
  read -rp "Hostname [${HOSTNAME}]: " input && HOSTNAME="${input:-$HOSTNAME}"
  read -rp "Disk size in GB [${DISK_SIZE}]: " input && DISK_SIZE="${input:-$DISK_SIZE}"
  read -rp "RAM in MB [${RAM}]: " input && RAM="${input:-$RAM}"
  read -rp "CPU cores [${CORES}]: " input && CORES="${input:-$CORES}"
  read -rp "Storage [${STORAGE}]: " input && STORAGE="${input:-$STORAGE}"
fi

# ── Locate template ────────────────────────────────────────────────────────
msg_info "Resolving container template..."
TEMPLATE_PATH=$(resolve_template)
msg_ok "Using template: ${TEMPLATE_PATH}"

# ── Create the container (privileged, nesting enabled for Docker) ──────────
msg_info "Creating LXC container ${CT_ID} (${HOSTNAME})..."
pct create "${CT_ID}" "${TEMPLATE_PATH}" \
  --hostname "${HOSTNAME}" \
  --cores "${CORES}" \
  --memory "${RAM}" \
  --rootfs "${STORAGE}:${DISK_SIZE}" \
  --net0 name=eth0,bridge=vmbr0,ip=dhcp \
  --unprivileged 0 \
  --features nesting=1 \
  --onboot 1 \
  --start 0
msg_ok "Container ${CT_ID} created."

# ── Start the container ────────────────────────────────────────────────────
msg_info "Starting container ${CT_ID}..."
pct start "${CT_ID}"
# Give the container a moment to obtain a network address
sleep 5
msg_ok "Container ${CT_ID} started."

# ── Copy and run the install script ─────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_SCRIPT="${SCRIPT_DIR}/../install/simple-wol-install.sh"

if [[ ! -f "${INSTALL_SCRIPT}" ]]; then
  msg_error "Install script not found at ${INSTALL_SCRIPT}"
  exit 1
fi

msg_info "Copying install script into container..."
pct push "${CT_ID}" "${INSTALL_SCRIPT}" /tmp/simple-wol-install.sh --perms 755
msg_ok "Install script copied."

msg_info "Running install script inside container (this may take a few minutes)..."
pct exec "${CT_ID}" -- bash /tmp/simple-wol-install.sh
msg_ok "Installation complete."

# ── Print access information ────────────────────────────────────────────────
CT_IP=$(pct exec "${CT_ID}" -- hostname -I 2>/dev/null | awk '{print $1}')

echo ""
echo -e "${GN}╔══════════════════════════════════════════════╗${CL}"
echo -e "${GN}║         Simple WoL is ready!                 ║${CL}"
echo -e "${GN}╚══════════════════════════════════════════════╝${CL}"
echo ""
echo "  Container ID : ${CT_ID}"
echo "  IP address   : ${CT_IP:-unknown}"
echo ""
if [[ -n "${CT_IP:-}" ]]; then
  echo -e "  Access URL   : ${GN}http://${CT_IP}:8080${CL}"
else
  msg_info "Could not determine IP. Check with: pct exec ${CT_ID} -- hostname -I"
fi
echo ""
