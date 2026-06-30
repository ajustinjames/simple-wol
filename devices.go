package main

import (
	"database/sql"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

const DefaultBroadcastAddress = "255.255.255.255"

type Device struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	MACAddress       string    `json:"mac_address"`
	IPAddress        string    `json:"ip_address"`
	Port             int       `json:"port"`
	BroadcastAddress string    `json:"broadcast_address"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

var macRegex = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}$`)

// NormalizeMAC pads single-digit hex octets (e.g. "d" -> "0d") and lowercases.
func NormalizeMAC(mac string) string {
	sep := ":"
	if strings.Contains(mac, "-") {
		sep = "-"
	}
	parts := strings.Split(mac, sep)
	if len(parts) != 6 {
		return mac
	}
	for i, p := range parts {
		if len(p) == 1 {
			parts[i] = "0" + p
		}
	}
	return strings.ToLower(strings.Join(parts, ":"))
}

func ValidateMAC(mac string) error {
	if !macRegex.MatchString(mac) {
		return fmt.Errorf("invalid MAC address: %s", mac)
	}
	return nil
}

func ValidateIP(ip string) error {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return fmt.Errorf("invalid IPv4 address: %s", ip)
	}
	return nil
}

func SanitizeName(name string) string {
	return strings.TrimSpace(name)
}

// ResolveBroadcastAddress validates an optional per-device broadcast address.
// An empty string falls back to the default limited broadcast address.
func ResolveBroadcastAddress(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return DefaultBroadcastAddress, nil
	}
	if err := ValidateIP(addr); err != nil {
		return "", fmt.Errorf("invalid broadcast address: %s", addr)
	}
	return addr, nil
}

func CreateDevice(db *sql.DB, d Device) (int64, error) {
	d.MACAddress = NormalizeMAC(d.MACAddress)
	if err := ValidateMAC(d.MACAddress); err != nil {
		return 0, err
	}
	if err := ValidateIP(d.IPAddress); err != nil {
		return 0, err
	}
	broadcastAddr, err := ResolveBroadcastAddress(d.BroadcastAddress)
	if err != nil {
		return 0, err
	}
	d.Name = SanitizeName(d.Name)
	result, err := db.Exec("INSERT INTO devices (name, mac_address, ip_address, port, broadcast_address) VALUES (?, ?, ?, ?, ?)", d.Name, d.MACAddress, d.IPAddress, d.Port, broadcastAddr)
	if err != nil {
		return 0, fmt.Errorf("create device: %w", err)
	}
	return result.LastInsertId()
}

func ListDevices(db *sql.DB) ([]Device, error) {
	rows, err := db.Query("SELECT id, name, mac_address, ip_address, port, broadcast_address, created_at, updated_at FROM devices ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.MACAddress, &d.IPAddress, &d.Port, &d.BroadcastAddress, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func GetDevice(db *sql.DB, id int64) (Device, error) {
	var d Device
	err := db.QueryRow("SELECT id, name, mac_address, ip_address, port, broadcast_address, created_at, updated_at FROM devices WHERE id = ?", id).Scan(&d.ID, &d.Name, &d.MACAddress, &d.IPAddress, &d.Port, &d.BroadcastAddress, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return d, fmt.Errorf("device not found: %w", err)
	}
	return d, nil
}

func UpdateDevice(db *sql.DB, id int64, d Device) error {
	d.MACAddress = NormalizeMAC(d.MACAddress)
	if err := ValidateMAC(d.MACAddress); err != nil {
		return err
	}
	if err := ValidateIP(d.IPAddress); err != nil {
		return err
	}
	broadcastAddr, err := ResolveBroadcastAddress(d.BroadcastAddress)
	if err != nil {
		return err
	}
	d.Name = SanitizeName(d.Name)
	result, err := db.Exec("UPDATE devices SET name=?, mac_address=?, ip_address=?, port=?, broadcast_address=?, updated_at=? WHERE id=?", d.Name, d.MACAddress, d.IPAddress, d.Port, broadcastAddr, time.Now(), id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("device not found")
	}
	return nil
}

func DeleteDevice(db *sql.DB, id int64) error {
	result, err := db.Exec("DELETE FROM devices WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("device not found")
	}
	return nil
}
