package handlers

import (
	"net"
	"net/http"
	"os"
	"strings"
)

func getClientIP(r *http.Request) string {
	remoteIP := parseRemoteIP(r.RemoteAddr)

	if remoteIP != nil && isTrustedProxy(remoteIP) {
		if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
			parts := strings.Split(forwardedFor, ",")
			if len(parts) > 0 {
				ip := strings.TrimSpace(parts[0])
				if ip != "" {
					return ip
				}
			}
		}

		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}

	if remoteIP != nil {
		return remoteIP.String()
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}

	return strings.TrimSpace(r.RemoteAddr)
}

func parseRemoteIP(remoteAddr string) net.IP {
	trimmed := strings.TrimSpace(remoteAddr)
	if trimmed == "" {
		return nil
	}

	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		return net.ParseIP(host)
	}

	return net.ParseIP(trimmed)
}

func isTrustedProxy(ip net.IP) bool {
	if ip == nil {
		return false
	}

	trusted := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_IPS"))
	if trusted == "" {
		return false
	}

	entries := strings.Split(trusted, ",")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err != nil {
				continue
			}
			if network.Contains(ip) {
				return true
			}
			continue
		}

		if parsed := net.ParseIP(entry); parsed != nil && parsed.Equal(ip) {
			return true
		}
	}

	return false
}
