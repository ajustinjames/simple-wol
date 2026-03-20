package main

import "testing"

type mockSender struct {
	called        bool
	mac           string
	broadcastAddr string
	port          int
}

func (m *mockSender) SendMagicPacket(mac, broadcastAddr string, port int) error {
	m.called = true; m.mac = mac; m.broadcastAddr = broadcastAddr; m.port = port; return nil
}

func TestBuildMagicPacket(t *testing.T) {
	packet, err := BuildMagicPacket("AA:BB:CC:DD:EE:FF")
	if err != nil { t.Fatalf("failed: %v", err) }
	if len(packet) != 102 { t.Fatalf("expected 102 bytes, got %d", len(packet)) }
	for i := 0; i < 6; i++ {
		if packet[i] != 0xFF { t.Fatalf("byte %d: expected 0xFF, got 0x%02X", i, packet[i]) }
	}
	expectedMAC := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	for i := 0; i < 16; i++ {
		for j := 0; j < 6; j++ {
			if packet[6+i*6+j] != expectedMAC[j] { t.Fatalf("rep %d byte %d wrong", i, j) }
		}
	}
}

func TestBuildMagicPacket_DashSeparator(t *testing.T) {
	packet, err := BuildMagicPacket("AA-BB-CC-DD-EE-FF")
	if err != nil { t.Fatalf("failed: %v", err) }
	if len(packet) != 102 { t.Fatalf("expected 102, got %d", len(packet)) }
}

func TestBuildMagicPacket_InvalidMAC(t *testing.T) {
	_, err := BuildMagicPacket("not-a-mac")
	if err == nil { t.Fatal("expected error") }
}

func TestWakeDevice_CallsSender(t *testing.T) {
	mock := &mockSender{}
	err := WakeDevice(mock, "AA:BB:CC:DD:EE:FF", "255.255.255.255", 9)
	if err != nil { t.Fatalf("failed: %v", err) }
	if !mock.called { t.Fatal("expected sender called") }
	if mock.mac != "AA:BB:CC:DD:EE:FF" { t.Fatalf("wrong MAC: %s", mock.mac) }
	if mock.port != 9 { t.Fatalf("wrong port: %d", mock.port) }
}
