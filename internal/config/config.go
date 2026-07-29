package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Host       string
	Port       int
	DataDir    string
	ChromePath string
}

func Load() Config {
	return Config{
		Host:       envOr("HOST", "0.0.0.0"),
		Port:       envIntOr("PORT", 7849),
		DataDir:    envOr("HTML_PREVIEW_DATA", "./data"),
		ChromePath: envOr("CHROME_PATH", ""),
	}
}

func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c Config) DBPath() string {
	return filepath.Join(c.DataDir, "html-preview.db")
}

// LANURLs returns http URLs peers on the LAN can open.
// When listening on all interfaces (0.0.0.0 / ::), it lists non-loopback IPv4 addresses.
// When bound to a specific host, it returns that host only (if not unspecified).
func (c Config) LANURLs() []string {
	port := c.Port
	host := strings.TrimSpace(c.Host)
	if host != "" && host != "0.0.0.0" && host != "::" && host != "[::]" {
		if isLoopbackHost(host) {
			return nil
		}
		return []string{fmt.Sprintf("http://%s:%d", formatHostForURL(host), port)}
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
			continue
		}
		ip := ipNet.IP.To4()
		if ip == nil {
			continue // IPv4 only for simpler LAN sharing
		}
		s := ip.String()
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, fmt.Sprintf("http://%s:%d", s, port))
	}
	return out
}

func isLoopbackHost(host string) bool {
	h := host
	if strings.HasPrefix(h, "[") && strings.HasSuffix(h, "]") {
		h = h[1 : len(h)-1]
	}
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func formatHostForURL(host string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]"
	}
	return host
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
