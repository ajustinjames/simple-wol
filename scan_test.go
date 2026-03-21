package main

import (
	"testing"
)

func TestParseARPTableLinux(t *testing.T) {
	// Simulate /proc/net/arp content
	content := `IP address       HW type     Flags       HW address            Mask     Device
192.168.4.1      0x1         0x2         aa:bb:cc:dd:ee:01     *        eth0
192.168.4.100    0x1         0x2         aa:bb:cc:dd:ee:02     *        eth0
192.168.4.200    0x1         0x0         00:00:00:00:00:00     *        eth0
192.168.7.255    0x1         0x2         ff:ff:ff:ff:ff:ff     *        eth0
255.255.255.255  0x1         0x2         ff:ff:ff:ff:ff:ff     *        eth0
224.0.0.251      0x1         0x2         01:00:5e:00:00:fb     *        eth0
239.255.255.250  0x1         0x2         01:00:5e:7f:ff:fa     *        eth0
`

	entries := ParseARPTableLinux(content)
	// Should skip the entry with all-zero MAC (incomplete)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].IP != "192.168.4.1" {
		t.Fatalf("expected IP 192.168.4.1, got %s", entries[0].IP)
	}
	if entries[0].MAC != "aa:bb:cc:dd:ee:01" {
		t.Fatalf("expected MAC aa:bb:cc:dd:ee:01, got %s", entries[0].MAC)
	}
}

func TestParseARPTableDarwin(t *testing.T) {
	content := `? (192.168.4.1) at aa:bb:cc:dd:ee:01 on en0 ifscope [ethernet]
? (192.168.4.100) at aa:bb:cc:dd:ee:02 on en0 ifscope [ethernet]
? (192.168.4.200) at (incomplete) on en0 ifscope [ethernet]
? (192.168.7.255) at ff:ff:ff:ff:ff:ff on en0 ifscope [ethernet]
? (255.255.255.255) at ff:ff:ff:ff:ff:ff on en0 ifscope [ethernet]
? (224.0.0.251) at 1:0:5e:0:0:fb on en0 ifscope [ethernet]
? (239.255.255.250) at 1:0:5e:7f:ff:fa on en0 ifscope [ethernet]
`

	entries := ParseARPTableDarwin(content)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].IP != "192.168.4.1" {
		t.Fatalf("expected IP 192.168.4.1, got %s", entries[0].IP)
	}
	if entries[0].MAC != "aa:bb:cc:dd:ee:01" {
		t.Fatalf("expected MAC aa:bb:cc:dd:ee:01, got %s", entries[0].MAC)
	}
}

func TestParseARPTableDarwinNormalizesMAC(t *testing.T) {
	content := `? (192.168.4.50) at a:bb:c:dd:e:ff on en0 ifscope [ethernet]
`
	entries := ParseARPTableDarwin(content)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].MAC != "0a:bb:0c:dd:0e:ff" {
		t.Fatalf("expected normalized MAC 0a:bb:0c:dd:0e:ff, got %s", entries[0].MAC)
	}
}

func TestParseARPTableLinuxNormalizesMAC(t *testing.T) {
	content := `IP address       HW type     Flags       HW address            Mask     Device
192.168.4.50     0x1         0x2         a:bb:c:dd:e:ff        *        eth0
`
	entries := ParseARPTableLinux(content)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].MAC != "0a:bb:0c:dd:0e:ff" {
		t.Fatalf("expected normalized MAC 0a:bb:0c:dd:0e:ff, got %s", entries[0].MAC)
	}
}

func TestSubnetIPs(t *testing.T) {
	ips := SubnetIPs("192.168.4.0", 24)
	// /24 should give 254 hosts (1-254, excluding .0 and .255)
	if len(ips) != 254 {
		t.Fatalf("expected 254 IPs, got %d", len(ips))
	}
	if ips[0] != "192.168.4.1" {
		t.Fatalf("expected first IP 192.168.4.1, got %s", ips[0])
	}
	if ips[253] != "192.168.4.254" {
		t.Fatalf("expected last IP 192.168.4.254, got %s", ips[253])
	}
}
