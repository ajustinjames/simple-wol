package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
)

type PacketSender interface {
	SendMagicPacket(macAddress string, broadcastAddr string, port int) error
}

type UDPSender struct{}

func (u *UDPSender) SendMagicPacket(macAddress, broadcastAddr string, port int) error {
	packet, err := BuildMagicPacket(macAddress)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf("%s:%d", broadcastAddr, port)
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return fmt.Errorf("dial udp: %w", err)
	}
	defer conn.Close()
	_, err = conn.Write(packet)
	if err != nil {
		return fmt.Errorf("send packet: %w", err)
	}
	return nil
}

func BuildMagicPacket(mac string) ([]byte, error) {
	mac = strings.ReplaceAll(mac, "-", ":")
	parts := strings.Split(mac, ":")
	if len(parts) != 6 {
		return nil, fmt.Errorf("invalid MAC address: %s", mac)
	}
	macBytes := make([]byte, 6)
	for i, part := range parts {
		b, err := hex.DecodeString(part)
		if err != nil || len(b) != 1 {
			return nil, fmt.Errorf("invalid MAC address: %s", mac)
		}
		macBytes[i] = b[0]
	}
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], macBytes)
	}
	return packet, nil
}

func WakeDevice(sender PacketSender, mac, broadcastAddr string, port int) error {
	return sender.SendMagicPacket(mac, broadcastAddr, port)
}

func CheckDeviceStatus(ip string) bool {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("ping", "-c", "1", "-W", "1000", ip)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}
	return cmd.Run() == nil
}
