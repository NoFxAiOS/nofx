package security

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSecurityDialPinsValidatedIP(t *testing.T) {
	lookups := 0
	lookup := func(ctx context.Context, host string) ([]net.IPAddr, error) {
		lookups++
		if host != "rebind.example" {
			t.Fatalf("unexpected host %s", host)
		}
		if lookups > 1 {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.9")}, {IP: net.ParseIP("2001:db8::1")}}, nil
	}
	var dialed []string
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialed = append(dialed, addr)
		return nil, errors.New("synthetic connection failure")
	}
	_, err := dialPublicAddress(context.Background(), "tcp", "rebind.example:443", lookup, dial)
	if err == nil || lookups != 1 {
		t.Fatalf("error=%v lookups=%d", err, lookups)
	}
	if strings.Join(dialed, ",") != "203.0.113.9:443,[2001:db8::1]:443" {
		t.Fatalf("must dial checked literals only: %v", dialed)
	}
}

func TestSecurityDialRejectsUnsafeAnswers(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "::", "::1", "::ffff:127.0.0.1", "fd00::1", "fe80::1", "ff02::1", "224.0.0.1"} {
		t.Run(ip, func(t *testing.T) {
			lookup := func(context.Context, string) ([]net.IPAddr, error) {
				return []net.IPAddr{{IP: net.ParseIP("203.0.113.9")}, {IP: net.ParseIP(ip)}}, nil
			}
			_, err := dialPublicAddress(context.Background(), "tcp", "unsafe.example:80", lookup, func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("must reject mixed DNS answers before any connection")
				return nil, nil
			})
			if err == nil {
				t.Fatal("unsafe answer accepted")
			}
		})
	}
}

func TestSecurityDialFailsClosedOnDNSAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		ips []net.IPAddr
		err error
	}{
		{nil, errors.New("DNS unavailable")}, {nil, nil},
		{[]net.IPAddr{{IP: net.ParseIP("203.0.113.9"), Zone: "en0"}}, nil},
	} {
		_, err := dialPublicAddress(context.Background(), "tcp", "test.example:443", func(context.Context, string) ([]net.IPAddr, error) { return tc.ips, tc.err }, func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("unexpected connection")
			return nil, nil
		})
		if err == nil {
			t.Fatal("invalid DNS result accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := dialPublicAddress(ctx, "tcp", "test.example:443", func(ctx context.Context, _ string) ([]net.IPAddr, error) { return nil, ctx.Err() }, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestSecuritySafeClientBlocksLocalAndRedirect(t *testing.T) {
	hits := 0
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; w.WriteHeader(200) }))
	defer internal.Close()
	client := SafeHTTPClient(time.Second)
	defer client.CloseIdleConnections()
	if _, err := client.Get(internal.URL); err == nil {
		t.Fatal("local server was reachable")
	}
	if hits != 0 {
		t.Fatal("private target received a request")
	}
	r, _ := http.NewRequest("GET", internal.URL, nil)
	if err := client.CheckRedirect(r, nil); err == nil {
		t.Fatal("private redirect accepted")
	}
	if err := ValidateURL("http://[::]/"); err == nil {
		t.Fatal("unspecified IPv6 accepted")
	}
}

func TestSecurityPinnedConnectionPreservesHost(t *testing.T) {
	// A fake public destination is routed to an isolated fixture by the TEST
	// dialer, so no real external service is contacted.
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "public.example" {
			t.Errorf("Host changed to %s", r.Host)
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer fixture.Close()
	d := net.Dialer{}
	client := SafeHTTPClient(time.Second)
	defer client.CloseIdleConnections()
	client.Transport.(*http.Transport).DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialPublicAddress(ctx, network, addr, func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("203.0.113.9")}}, nil
		}, func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr != "203.0.113.9:80" {
				t.Fatalf("destination not pinned: %s", addr)
			}
			return d.DialContext(ctx, network, strings.TrimPrefix(fixture.URL, "http://"))
		})
	}
	res, err := client.Get("http://public.example")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if string(b) != "ok" {
		t.Fatal(string(b))
	}
}
