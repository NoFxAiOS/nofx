// Package security provides security utilities for the application
package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Private/Reserved IP ranges that should be blocked to prevent SSRF
var privateIPBlocks []*net.IPNet

func init() {
	// Initialize private IP blocks
	// These ranges should not be accessible via user-controlled URLs
	privateRanges := []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918 private
		"172.16.0.0/12",  // RFC1918 private
		"192.168.0.0/16", // RFC1918 private
		"169.254.0.0/16", // Link-local / Cloud metadata
		"100.64.0.0/10",  // Shared address space (including carrier/private overlays)
		"0.0.0.0/8",      // Current network
		"224.0.0.0/4",    // Multicast
		"240.0.0.0/4",    // Reserved
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local
	}

	for _, cidr := range privateRanges {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			privateIPBlocks = append(privateIPBlocks, block)
		}
	}
}

// SSRFError represents a Server-Side Request Forgery attempt
type SSRFError struct {
	URL    string
	Reason string
}

func (e *SSRFError) Error() string {
	return fmt.Sprintf("SSRF blocked: %s - %s", e.URL, e.Reason)
}

// isPrivateIP checks if an IP address is in a private/reserved range
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true // Invalid IP, treat as private
	}

	// Check if it's a loopback address
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}

	// Check if it's a link-local address
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check if it's a private address
	if ip.IsPrivate() {
		return true
	}

	// Check against our explicit private ranges
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}

	return false
}

// ValidateURL checks if a URL is safe to request (not pointing to internal networks)
// Returns an error if the URL is potentially dangerous
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return &SSRFError{URL: rawURL, Reason: "empty URL"}
	}

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return &SSRFError{URL: rawURL, Reason: "invalid URL format"}
	}

	// Only allow http and https schemes
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return &SSRFError{URL: rawURL, Reason: fmt.Sprintf("unsupported scheme: %s", scheme)}
	}

	// Extract hostname (without port)
	host := parsedURL.Hostname()
	if host == "" {
		return &SSRFError{URL: rawURL, Reason: "empty hostname"}
	}

	// Block localhost and common internal hostnames
	lowerHost := strings.ToLower(host)
	blockedHosts := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"0.0.0.0",
		"metadata.google.internal",
		"metadata.google",
		"instance-data",
	}
	for _, blocked := range blockedHosts {
		if lowerHost == blocked {
			return &SSRFError{URL: rawURL, Reason: fmt.Sprintf("blocked hostname: %s", host)}
		}
	}

	// Early validation only. SafeHTTPClient also resolves and pins the address
	// when opening the socket; a URL validation alone cannot stop DNS rebinding.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return &SSRFError{URL: rawURL, Reason: "hostname could not be resolved"}
	}
	if len(ips) == 0 {
		return &SSRFError{URL: rawURL, Reason: "hostname has no addresses"}
	}

	// Check all resolved IPs
	for _, ipAddr := range ips {
		if ipAddr.Zone != "" || isPrivateIP(ipAddr.IP) {
			return &SSRFError{URL: rawURL, Reason: fmt.Sprintf("resolves to private IP: %s", ipAddr.IP)}
		}
	}

	return nil
}

// SafeHTTPClient returns an HTTP client with SSRF protection
// It validates URLs and blocks requests to private networks
func SafeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialPublicAddress(ctx, network, addr, net.DefaultResolver.LookupIPAddr, dialer.DialContext)
		},
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}

			// Validate the redirect URL
			if err := ValidateURL(req.URL.String()); err != nil {
				return fmt.Errorf("SSRF protection: redirect blocked - %w", err)
			}

			return nil
		},
	}
}

// Dial the checked literal IP, never the hostname (which would trigger a second
// DNS lookup). The HTTP transport retains the original Host header and TLS SNI.
// All answers are checked before dialing, including fallback addresses.
func dialPublicAddress(ctx context.Context, network, addr string,
	lookup func(context.Context, string) ([]net.IPAddr, error),
	dial func(context.Context, string, string) (net.Conn, error),
) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("SSRF protection: invalid destination: %w", err)
	}
	ips, err := lookup(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("SSRF protection: failed to resolve host: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("SSRF protection: hostname has no addresses")
	}
	for _, ip := range ips {
		if ip.Zone != "" || isPrivateIP(ip.IP) {
			return nil, fmt.Errorf("SSRF protection: blocked non-public destination")
		}
	}
	for _, ip := range ips {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var conn net.Conn
		conn, err = dial(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("SSRF protection: connection failed: %w", err)
}

// SafeGet performs a GET request with SSRF protection
// It validates the URL before making the request and uses a safe HTTP client
func SafeGet(rawURL string, timeout time.Duration) (*http.Response, error) {
	// First validate the URL
	if err := ValidateURL(rawURL); err != nil {
		return nil, err
	}

	// Use the safe HTTP client
	client := SafeHTTPClient(timeout)
	return client.Get(rawURL)
}
