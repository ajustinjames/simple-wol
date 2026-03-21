package main

import "testing"

func TestCreateDevice(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	id, err := CreateDevice(db, Device{Name: "My PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.4.100", Port: 9})
	if err != nil { t.Fatalf("CreateDevice failed: %v", err) }
	if id == 0 { t.Fatal("expected non-zero ID") }
}

func TestListDevices(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	CreateDevice(db, Device{Name: "PC1", MACAddress: "AA:BB:CC:DD:EE:01", IPAddress: "192.168.4.1", Port: 9})
	CreateDevice(db, Device{Name: "PC2", MACAddress: "AA:BB:CC:DD:EE:02", IPAddress: "192.168.4.2", Port: 9})
	devices, err := ListDevices(db)
	if err != nil { t.Fatalf("ListDevices failed: %v", err) }
	if len(devices) != 2 { t.Fatalf("expected 2 devices, got %d", len(devices)) }
}

func TestGetDevice(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	id, _ := CreateDevice(db, Device{Name: "My PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.4.100", Port: 9})
	d, err := GetDevice(db, id)
	if err != nil { t.Fatalf("GetDevice failed: %v", err) }
	if d.Name != "My PC" { t.Fatalf("expected 'My PC', got '%s'", d.Name) }
}

func TestUpdateDevice(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	id, _ := CreateDevice(db, Device{Name: "My PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.4.100", Port: 9})
	err := UpdateDevice(db, id, Device{Name: "Gaming PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.4.100", Port: 7})
	if err != nil { t.Fatalf("UpdateDevice failed: %v", err) }
	d, _ := GetDevice(db, id)
	if d.Name != "Gaming PC" { t.Fatalf("expected 'Gaming PC', got '%s'", d.Name) }
	if d.Port != 7 { t.Fatalf("expected port 7, got %d", d.Port) }
}

func TestDeleteDevice(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	id, _ := CreateDevice(db, Device{Name: "My PC", MACAddress: "AA:BB:CC:DD:EE:FF", IPAddress: "192.168.4.100", Port: 9})
	if err := DeleteDevice(db, id); err != nil { t.Fatalf("DeleteDevice failed: %v", err) }
	_, err := GetDevice(db, id)
	if err == nil { t.Fatal("expected error getting deleted device") }
}

func TestValidateMAC(t *testing.T) {
	for _, mac := range []string{"AA:BB:CC:DD:EE:FF", "aa:bb:cc:dd:ee:ff", "AA-BB-CC-DD-EE-FF"} {
		if err := ValidateMAC(mac); err != nil { t.Errorf("expected %q valid: %v", mac, err) }
	}
	for _, mac := range []string{"not-a-mac", "AA:BB:CC:DD:EE", "GG:HH:II:JJ:KK:LL", ""} {
		if err := ValidateMAC(mac); err == nil { t.Errorf("expected %q invalid", mac) }
	}
}

func TestNormalizeMAC(t *testing.T) {
	tests := []struct{ input, expected string }{
		{"aa:bb:cc:d:ee:ff", "aa:bb:cc:0d:ee:ff"},
		{"a:b:c:d:e:f", "0a:0b:0c:0d:0e:0f"},
		{"AA:BB:CC:DD:EE:FF", "aa:bb:cc:dd:ee:ff"},
		{"AA-BB-CC-DD-EE-FF", "aa:bb:cc:dd:ee:ff"},
		{"aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff"},
	}
	for _, tc := range tests {
		got := NormalizeMAC(tc.input)
		if got != tc.expected { t.Errorf("NormalizeMAC(%q) = %q, want %q", tc.input, got, tc.expected) }
	}
}

func TestCreateDeviceNormalizesMAC(t *testing.T) {
	db, _ := InitDB(":memory:"); defer db.Close()
	id, err := CreateDevice(db, Device{Name: "Test PC", MACAddress: "aa:bb:cc:d:ee:ff", IPAddress: "192.168.4.100", Port: 9})
	if err != nil { t.Fatalf("CreateDevice failed: %v", err) }
	d, _ := GetDevice(db, id)
	if d.MACAddress != "aa:bb:cc:0d:ee:ff" { t.Fatalf("expected normalized MAC, got %q", d.MACAddress) }
}

func TestValidateIP(t *testing.T) {
	for _, ip := range []string{"192.168.4.100", "10.0.0.1"} {
		if err := ValidateIP(ip); err != nil { t.Errorf("expected %q valid: %v", ip, err) }
	}
	for _, ip := range []string{"not-an-ip", "999.999.999.999", "", "::1", "fe80::1"} {
		if err := ValidateIP(ip); err == nil { t.Errorf("expected %q invalid", ip) }
	}
}
