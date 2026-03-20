package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type ARPEntry struct {
	IP  string `json:"ip"`
	MAC string `json:"mac"`
}

// ParseARPTableLinux parses /proc/net/arp format:
// IP address       HW type     Flags       HW address            Mask     Device
// 192.168.4.1      0x1         0x2         aa:bb:cc:dd:ee:01     *        eth0
func ParseARPTableLinux(content string) []ARPEntry {
	var entries []ARPEntry
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines[1:] { // Skip header
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		mac := fields[3]
		if mac == "00:00:00:00:00:00" {
			continue
		}
		entries = append(entries, ARPEntry{IP: fields[0], MAC: mac})
	}
	return entries
}

// ParseARPTableDarwin parses macOS `arp -a` format:
// ? (192.168.4.1) at aa:bb:cc:dd:ee:01 on en0 ifscope [ethernet]
func ParseARPTableDarwin(content string) []ARPEntry {
	var entries []ARPEntry
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[1] == "" {
			continue
		}
		// Extract IP from parentheses: (192.168.4.1)
		ip := strings.Trim(fields[1], "()")
		mac := fields[3]
		if mac == "(incomplete)" || mac == "00:00:00:00:00:00" {
			continue
		}
		entries = append(entries, ARPEntry{IP: ip, MAC: mac})
	}
	return entries
}

// ParseARPTable dispatches to the platform-specific parser.
func ParseARPTable(content string) []ARPEntry {
	if runtime.GOOS == "darwin" {
		return ParseARPTableDarwin(content)
	}
	return ParseARPTableLinux(content)
}

// SubnetIPs returns all usable host IPs for a given network address and prefix length.
// It excludes the network address and the broadcast address.
func SubnetIPs(networkAddr string, prefixLen int) []string {
	ip := net.ParseIP(networkAddr).To4()
	if ip == nil {
		return nil
	}

	ipInt := binary.BigEndian.Uint32(ip)
	hostBits := 32 - prefixLen
	numHosts := (1 << hostBits) - 2 // Exclude network and broadcast

	ips := make([]string, 0, numHosts)
	for i := 1; i <= numHosts; i++ {
		hostIP := make(net.IP, 4)
		binary.BigEndian.PutUint32(hostIP, ipInt+uint32(i))
		ips = append(ips, hostIP.String())
	}
	return ips
}

// DetectSubnet finds the first non-loopback, active IPv4 interface and returns
// its network address and prefix length.
func DetectSubnet() (networkAddr string, prefixLen int, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", 0, err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			ones, _ := ipNet.Mask.Size()
			network := ipNet.IP.Mask(ipNet.Mask)
			return network.String(), ones, nil
		}
	}
	return "", 0, fmt.Errorf("no suitable network interface found")
}

// PingSweep sends ICMP pings to all given IPs concurrently to populate the ARP table.
func PingSweep(ips []string) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 50) // Limit concurrency

	for _, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()

			var cmd *exec.Cmd
			if runtime.GOOS == "darwin" {
				cmd = exec.Command("ping", "-c", "1", "-W", "1000", ip)
			} else {
				cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
			}
			cmd.Run() // We don't care about the result; it populates the ARP table
		}(ip)
	}
	wg.Wait()
}

// ReadARPTable reads the system ARP table and returns its raw content.
func ReadARPTable() (string, error) {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("arp", "-a").Output()
		return string(out), err
	}
	// Linux: read /proc/net/arp directly
	out, err := os.ReadFile("/proc/net/arp")
	return string(out), err
}

// ScanNetwork performs a full network scan: detect subnet, ping sweep, read ARP table.
func ScanNetwork() ([]ARPEntry, error) {
	networkAddr, prefixLen, err := DetectSubnet()
	if err != nil {
		return nil, err
	}

	ips := SubnetIPs(networkAddr, prefixLen)
	PingSweep(ips)

	// Small delay to let ARP table populate
	time.Sleep(500 * time.Millisecond)

	content, err := ReadARPTable()
	if err != nil {
		return nil, fmt.Errorf("read arp table: %w", err)
	}

	return ParseARPTable(content), nil
}
