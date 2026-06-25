//go:build linux

// PowerOn - Detects Mac Mini PCI power management chipsets and configures
// auto-power-on after power failure. Works on Home Assistant OS running
// on Apple hardware (Mac Mini 2010-2014).
//
// Supported chipsets:
//   Intel ICH7-M (LPC)    - Mac Mini 2010  (8086:27b8, bus 0, dev 0x1f, fn 0)
//   Nvidia MCP89 (PMC)    - Mac Mini 2011+ (10de:0d94, bus 0, dev 0x0b, fn 0)
//
// The MCP89 PMC register (May 2026 verification):
//   PM_CFG at config space offset 0x90, bit 0 = G3_WAKE
//   0 = wake on power restore, 1 = stay in G3 (powered off)
//
// References:
//   ICH7-M: https://smackerelofopinion.blogspot.com/2011/09/mac-mini-rebooting-tweaks-setpci-s-01f0.html
//   MCP89:  PMC datasheet — PM_CFG register

package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Chipset defines a known Mac Mini PCI power controller.
type Chipset struct {
	// VendorDevice in hex, e.g. "8086:27b8"
	VendorDevice string
	// Bus, device, function for direct PCI config space access
	Bus, Dev, Func uint8
	// Register offset within PCI config space (e.g. 0xa4, 0x90)
	RegOffset uint16
	// Human-readable name
	Name string
}

// pciBus represents /sys/bus/pci/devices/ for device enumeration.
type pciBus struct {
	base string
}

// mmio represents a memory-mapped PCI config space bar.
type mmio struct {
	fd  int
	mem []byte
}

var chipsets = []Chipset{
	{
		VendorDevice: "8086:27b8",
		Bus:          0x00,
		Dev:          0x1f,
		Func:         0x00,
		RegOffset:    0xa4,
		Name:         "ICH7-M",
	},
	{
		VendorDevice: "10de:0d94",
		Bus:          0x00,
		Dev:          0x0b,
		Func:         0x00,
		RegOffset:    0x90,
		Name:         "MCP89",
	},
}

// MCP89 fallback device IDs (any one present = MCP89 Mac Mini).
var mcp89Families = []string{
	"10de:0d93", // ACI (Apple Communication Interface)
	"10de:0d95", // SMBus controller
	"10de:0d96",
	"10de:0d97",
	"10de:0d98",
	"10de:0ac6", // SATA controller
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("")

	// Read device ID from sysfs PCI enumeration.
	pb := pciBus{base: "/sys/bus/pci/devices"}

	// Prefer the explicit chipset list first.
	mc := detectChipset(pb)

	if mc == nil {
		// Fallback: any MCP89 family member implies an MCP89 Mac Mini.
		mc = detectMCp89Fallback(pb)
	}

	if mc == nil {
		log.Println("No supported Mac Mini chipset found")
		os.Exit(0)
	}

	log.Printf("Detected %s (%s) at bus %02x:%02x.%x",
		mc.Name, mc.VendorDevice, mc.Bus, mc.Dev, mc.Func)

	// Open PCI config space via sysfs (portable, no /dev/mem).
	devPath := fmt.Sprintf("/sys/bus/pci/devices/0000:%02x:%02x.%x/config",
		mc.Bus, mc.Dev, mc.Func)

	m, err := mmioOpen(devPath)
	if err != nil {
		log.Fatalf("Cannot open PCI config space %s: %v", devPath, err)
	}
	defer m.close()

	// Mask: clear only bit 0 (G3_WAKE / AFTERG3_EN), preserve all others.
	const mask = ^uint32(1)

	// Apply with retries.
	retries := maxRetries()
	for i := 0; i < retries; i++ {
		if i > 0 {
			interval := retryInterval()
			log.Printf("Retry %d/%d — waiting %v...", i+1, retries, interval)
			time.Sleep(interval)
		}

		val, err := m.read32(mc.RegOffset)
		if err != nil {
			log.Printf("Read failed: %v", err)
			continue
		}

		newVal := val & mask
		if newVal == val {
			log.Printf("Register 0x%02x already configured (0x%08x, bit 0 clear)",
				mc.RegOffset, val)
			log.Println("SUCCESS: power-on after power failure configured")
			return
		}

		if err := m.write32(mc.RegOffset, newVal); err != nil {
			log.Printf("Write failed: %v", err)
			continue
		}

		// Verify.
		verified, err := m.read32(mc.RegOffset)
		if err != nil {
			log.Printf("Verify read failed: %v", err)
			continue
		}
		if verified != newVal {
			log.Printf("Verify mismatch: wrote 0x%08x, read 0x%08x", newVal, verified)
			continue
		}

		log.Printf("Register 0x%02x: 0x%08x -> 0x%08x (mask G3_WAKE bit)",
			mc.RegOffset, val, newVal)
		log.Println("SUCCESS: power-on after power failure configured for", mc.Name)
		return
	}

	log.Printf("ERROR: Gave up after %d attempts", retries)
	os.Exit(1)
}

// detectChipset scans PCI devices for a known vendor:device pair.
func detectChipset(pb pciBus) *Chipset {
	entries, err := os.ReadDir(pb.base)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		vendor, err := readVendorDevice(pb.base, e.Name())
		if err != nil {
			continue
		}

		for _, cs := range chipsets {
			if vendor == cs.VendorDevice {
				return &cs
			}
		}
	}
	return nil
}

// detectMCp89Fallback returns an MCP89 chipset when any MCP89-family
// device is found, even if the primary PMC (10de:0d94) is absent.
func detectMCp89Fallback(pb pciBus) *Chipset {
	entries, err := os.ReadDir(pb.base)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		vendor, err := readVendorDevice(pb.base, e.Name())
		if err != nil {
			continue
		}
		for _, fam := range mcp89Families {
			if vendor == fam {
				cs := chipsets[1] // MCP89
				return &cs
			}
		}
	}
	return nil
}

func readVendorDevice(root, name string) (vendorDevice string, _ error) {
	v, err := os.ReadFile(root + "/" + name + "/vendor")
	if err != nil {
		return "", err
	}
	d, err := os.ReadFile(root + "/" + name + "/device")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(v)) + ":" + strings.TrimSpace(string(d)), nil
}

func mmioOpen(path string) (*mmio, error) {
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_SYNC, 0)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	fi, err := fstat(fd)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("fstat: %w", err)
	}

	mem, err := syscall.Mmap(fd, 0, int(fi.Size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("mmap: %w", err)
	}

	return &mmio{fd: fd, mem: mem}, nil
}

func (m *mmio) close() {
	syscall.Munmap(m.mem)
	syscall.Close(m.fd)
}

func (m *mmio) read32(offset uint16) (uint32, error) {
	if uint16(offset)+4 > uint16(len(m.mem)) {
		return 0, fmt.Errorf("offset %d beyond mapped region (%d bytes)", offset, len(m.mem))
	}
	data := (*[4]byte)(unsafe.Pointer(&m.mem[offset]))
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24, nil
}

func (m *mmio) write32(offset uint16, val uint32) error {
	if uint16(offset)+4 > uint16(len(m.mem)) {
		return fmt.Errorf("offset %d beyond mapped region (%d bytes)", offset, len(m.mem))
	}
	data := (*[4]byte)(unsafe.Pointer(&m.mem[offset]))
	data[0] = byte(val)
	data[1] = byte(val >> 8)
	data[2] = byte(val >> 16)
	data[3] = byte(val >> 24)
	return nil
}

func maxRetries() int {
	n := os.Getenv("POWERON_RETRY_COUNT")
	if n == "" {
		return 5
	}
	i, err := strconv.Atoi(n)
	if err != nil || i < 1 || i > 100 {
		return 5
	}
	return i
}

func retryInterval() time.Duration {
	n := os.Getenv("POWERON_RETRY_INTERVAL")
	if n == "" {
		return 10 * time.Second
	}
	i, err := strconv.Atoi(n)
	if err != nil || i < 1 || i > 300 {
		return 10 * time.Second
	}
	return time.Duration(i) * time.Second
}

// fstat uses raw syscall to avoid path conflicts with /usr/lib/go/*.
func fstat(fd int) (syscall.Stat_t, error) {
	var s syscall.Stat_t
	return s, syscall.Fstat(fd, &s)
}
