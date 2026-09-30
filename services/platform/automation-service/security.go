package automation

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// NewSafeHTTPClient returns an http.Client configured to prevent SSRF attacks.
// It blocks connections to private networks, loopbacks, and link-local addresses.
func NewSafeHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			// Control the dialing process to validate IPs
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
				Control:   safeSocketControl, // <-- The Security Hook
			}).DialContext,
			// Standard transport settings
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			// Disable dual stack to simplify IP validation logic if needed,
			// but safeSocketControl handles IPv6 correctly so we keep defaults.
		},
	}
}

// safeSocketControl is a low-level hook called after DNS resolution but before
// the connection is fully established. It validates the destination IP.
func safeSocketControl(network, address string, c syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid address format: %w", err)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return errors.New("could not parse IP address")
	}

	if isUnsafeIP(ip) {
		return fmt.Errorf("blocked connection to unsafe IP: %s", ip.String())
	}

	return nil
}

// isUnsafeIP returns true if the IP is Loopback, Private, or Link-Local (Cloud Metadata).
func isUnsafeIP(ip net.IP) bool {
	// 1. Check if Loopback (127.0.0.0/8, ::1)
	if ip.IsLoopback() {
		return true
	}

	// 2. Check if Private (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7)
	// Go 1.17+ has .IsPrivate()
	if ip.IsPrivate() {
		return true
	}

	// 3. Check Unspecified (0.0.0.0)
	if ip.IsUnspecified() {
		return true
	}

	// 4. Check Link-Local Unicast (169.254.0.0/16, fe80::/10)
	// This is critical for blocking AWS/GCP Metadata services (http://169.254.169.254)
	if ip.IsLinkLocalUnicast() {
		return true
	}

	// 5. Check Link-Local Multicast (224.0.0.0/24, ff02::/16)
	if ip.IsLinkLocalMulticast() {
		return true
	}

	return false
}
