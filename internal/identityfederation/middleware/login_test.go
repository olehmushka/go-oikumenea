package middleware

import (
	"net/http"
	"testing"
)

// TestClientIP pins the client-IP extraction the login security log depends on (M37 /
// D-LoginSecurityLog): RemoteAddr by default; the facade-set X-Forwarded-For ONLY when trust is on
// (else a client could spoof its own logged IP — D-HeadlessTopology amended).
func TestClientIP(t *testing.T) {
	hdr := func(xff string) http.Header {
		h := http.Header{}
		if xff != "" {
			h.Set("X-Forwarded-For", xff)
		}
		return h
	}
	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		trust      bool
		want       string
	}{
		{"trust off, no xff -> remoteaddr host", "198.51.100.9:54321", "", false, "198.51.100.9"},
		{"trust off, xff present -> IGNORED (not trusted)", "198.51.100.9:54321", "203.0.113.5", false, "198.51.100.9"},
		{"trust on, single xff -> xff", "10.0.0.2:9", "203.0.113.5", true, "203.0.113.5"},
		{"trust on, xff chain -> leftmost (original client)", "10.0.0.2:9", "203.0.113.5, 10.0.0.1", true, "203.0.113.5"},
		{"trust on, no xff -> remoteaddr host", "198.51.100.9:9", "", true, "198.51.100.9"},
		{"remoteaddr without port -> as-is", "198.51.100.9", "", false, "198.51.100.9"},
		{"ipv6 remoteaddr host", "[2001:db8::1]:443", "", false, "2001:db8::1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &http.Request{RemoteAddr: c.remoteAddr, Header: hdr(c.xff)}
			if got := clientIP(r, c.trust); got != c.want {
				t.Errorf("clientIP = %q, want %q", got, c.want)
			}
		})
	}
}
