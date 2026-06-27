package dns

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

const (
	testIP        = "1.2.3.4"
	testDomainStr = "example.com"
)

func newDNSResponse(ips ...string) []byte {
	domain := dns.Fqdn(testDomainStr)
	m := new(dns.Msg)
	m.SetQuestion(domain, dns.TypeA)
	reply := new(dns.Msg)
	reply.SetReply(m)
	reply.Answer = make([]dns.RR, 0, len(ips))
	for _, ip := range ips {
		if strings.Contains(ip, ":") {
			reply.Answer = append(reply.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
				AAAA: net.ParseIP(ip),
			})
		} else {
			reply.Answer = append(reply.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
				A:   net.ParseIP(ip),
			})
		}
	}
	wire, _ := reply.Pack()
	return wire
}

func newDNSNXDOMAIN() []byte {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(testDomainStr), dns.TypeA)
	reply := new(dns.Msg)
	reply.SetReply(m)
	reply.Rcode = dns.RcodeNameError
	wire, _ := reply.Pack()
	return wire
}

func newDNSServer(tb testing.TB) *httptest.Server {
	tb.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		if r.Method == http.MethodPost {
			var err error
			body, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
		} else if r.URL.Query().Get("dns") == "" {
			http.Error(w, "missing dns param", http.StatusBadRequest)
			return
		}

		msg := new(dns.Msg)
		if err := msg.Unpack(body); err != nil {
			http.Error(w, "unpack fail", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/dns-message")
		if len(msg.Question) > 0 && msg.Question[0].Qtype == dns.TypeAAAA {
			_, _ = w.Write(newDNSResponse("2001:db8::1"))
			return
		}
		_, _ = w.Write(newDNSResponse(testIP))
	}))
}

func TestNewResolver(t *testing.T) {
	t.Parallel()
	r := NewResolver(nil, 0)
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
}

func TestNewResolverDefaults(t *testing.T) {
	t.Parallel()
	r := NewResolver(nil, 0)
	if r.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", r.timeout)
	}
}

func TestNewResolverWithClient(t *testing.T) {
	t.Parallel()
	client := &http.Client{Timeout: 10 * time.Second}
	r := NewResolver(client, 3*time.Second)
	if r.client != client {
		t.Error("client not stored")
	}
}

func TestAddProvider(t *testing.T) {
	t.Parallel()
	r := NewResolver(nil, 0)
	initial := len(r.providers)
	r.AddProvider(Provider{Name: "test", URL: "https://test/dns-query", Method: http.MethodPost})
	if len(r.providers) != initial+1 {
		t.Errorf("providers = %d, want %d", len(r.providers), initial+1)
	}
}

func TestSetProviders(t *testing.T) {
	t.Parallel()
	r := NewResolver(nil, 0)
	r.SetProviders([]Provider{{Name: "a", URL: "a", Method: "POST"}})
	if len(r.providers) != 1 || r.providers[0].Name != "a" {
		t.Error("SetProviders did not replace providers")
	}
}

func TestNextProviderRoundRobin(t *testing.T) {
	t.Parallel()
	r := NewResolver(nil, 0)
	r.SetProviders([]Provider{
		{Name: "a", URL: "a", Method: "POST"},
		{Name: "b", URL: "b", Method: "POST"},
	})
	first := r.nextProvider()
	second := r.nextProvider()
	third := r.nextProvider()
	if first.Name != "a" || second.Name != "b" || third.Name != "a" {
		t.Errorf("expected a, b, a got %s, %s, %s", first.Name, second.Name, third.Name)
	}
}

func TestNextProviderSingle(t *testing.T) {
	t.Parallel()
	r := NewResolver(nil, 0)
	r.SetProviders([]Provider{{Name: "only", URL: "u", Method: "POST"}})
	for range 10 {
		p := r.nextProvider()
		if p.Name != "only" {
			t.Fatal("expected only provider")
		}
	}
}

func TestLookupContextCancel(t *testing.T) {
	t.Parallel()
	r := NewResolver(nil, 5*time.Second)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := r.LookupA(ctx, testDomainStr)
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestLookupAlreadyCancelled(t *testing.T) {
	t.Parallel()
	res := NewResolver(nil, 5*time.Second)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := res.Lookup(ctx, testDomainStr, dns.TypeA)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestLookupAWithHTTPServer(t *testing.T) {
	t.Parallel()
	srv := newDNSServer(t)
	defer srv.Close()

	r := NewResolver(srv.Client(), 5*time.Second)
	r.SetProviders([]Provider{
		{Name: "test", URL: srv.URL, Method: http.MethodPost},
	})

	ips, err := r.LookupA(t.Context(), testDomainStr)
	if err != nil {
		t.Fatalf("LookupA: %v", err)
	}
	if len(ips) != 1 || ips[0] != testIP {
		t.Errorf("expected [%s], got %v", testIP, ips)
	}
}

func TestLookupAAAAWithHTTPServer(t *testing.T) {
	t.Parallel()
	srv := newDNSServer(t)
	defer srv.Close()

	r := NewResolver(srv.Client(), 5*time.Second)
	r.SetProviders([]Provider{
		{Name: "test", URL: srv.URL, Method: http.MethodPost},
	})

	ips, err := r.LookupAAAA(t.Context(), testDomainStr)
	if err != nil {
		t.Fatalf("LookupAAAA: %v", err)
	}
	if len(ips) != 1 || ips[0] != "2001:db8::1" {
		t.Errorf("expected [2001:db8::1], got %v", ips)
	}
}

func TestLookupIPReturnsBoth(t *testing.T) {
	t.Parallel()
	srv := newDNSServer(t)
	defer srv.Close()

	r := NewResolver(srv.Client(), 5*time.Second)
	r.SetProviders([]Provider{
		{Name: "test", URL: srv.URL, Method: http.MethodPost},
	})

	ips, err := r.LookupIP(t.Context(), testDomainStr)
	if err != nil {
		t.Fatalf("LookupIP: %v", err)
	}
	if len(ips) != 2 {
		t.Errorf("expected 2 IPs, got %d: %v", len(ips), ips)
	}
}

func TestLookupNXDOMAIN(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(newDNSNXDOMAIN())
	}))
	defer srv.Close()

	r := NewResolver(srv.Client(), 5*time.Second)
	r.SetProviders([]Provider{
		{Name: "test", URL: srv.URL, Method: http.MethodPost},
	})

	ips, err := r.LookupA(t.Context(), "nonexistent.example.com")
	if err != nil {
		t.Fatalf("unexpected error for NXDOMAIN: %v", err)
	}
	if len(ips) != 0 {
		t.Errorf("expected 0 IPs for NXDOMAIN, got %d", len(ips))
	}
}

func TestLookupGETMethod(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Query().Get("dns") == "" {
			t.Error("expected dns query param")
		}
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(newDNSResponse(testIP))
	}))
	defer srv.Close()

	r := NewResolver(srv.Client(), 5*time.Second)
	r.SetProviders([]Provider{
		{Name: "test", URL: srv.URL, Method: http.MethodGet},
	})

	ips, err := r.LookupA(t.Context(), testDomainStr)
	if err != nil {
		t.Fatalf("LookupA: %v", err)
	}
	if len(ips) != 1 || ips[0] != testIP {
		t.Errorf("expected [%s], got %v", testIP, ips)
	}
}

func TestLookupHTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	r := NewResolver(srv.Client(), 5*time.Second)
	r.SetProviders([]Provider{
		{Name: "test", URL: srv.URL, Method: http.MethodPost},
	})

	_, err := r.LookupA(t.Context(), testDomainStr)
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}

func TestLookupNonDNSResponse(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write([]byte("not a dns message"))
	}))
	defer srv.Close()

	r := NewResolver(srv.Client(), 5*time.Second)
	r.SetProviders([]Provider{
		{Name: "test", URL: srv.URL, Method: http.MethodPost},
	})

	_, err := r.LookupA(t.Context(), testDomainStr)
	if err == nil {
		t.Error("expected error for non-DNS response")
	}
}

func TestLookupTimeout(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(newDNSResponse(testIP))
	}))
	defer srv.Close()

	r := NewResolver(srv.Client(), 1*time.Nanosecond)
	r.SetProviders([]Provider{
		{Name: "test", URL: srv.URL, Method: http.MethodPost},
	})

	_, err := r.LookupA(t.Context(), testDomainStr)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestLookupCNAME(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(testDomainStr), dns.TypeA)
		reply := new(dns.Msg)
		reply.SetReply(m)
		reply.Answer = append(reply.Answer, &dns.CNAME{
			Hdr:    dns.RR_Header{Name: dns.Fqdn(testDomainStr), Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
			Target: "target.example.com.",
		})
		wire, _ := reply.Pack()
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(wire)
	}))
	defer srv.Close()

	r := NewResolver(srv.Client(), 5*time.Second)
	r.SetProviders([]Provider{
		{Name: "test", URL: srv.URL, Method: http.MethodPost},
	})

	ips, err := r.LookupCNAME(t.Context(), testDomainStr)
	if err != nil {
		t.Fatalf("LookupCNAME: %v", err)
	}
	if len(ips) != 1 || ips[0] != "target.example.com" {
		t.Errorf("expected [target.example.com], got %v", ips)
	}
}

func TestLookupConcurrent(t *testing.T) {
	t.Parallel()
	srv := newDNSServer(t)
	defer srv.Close()

	r := NewResolver(srv.Client(), 5*time.Second)
	r.SetProviders([]Provider{
		{Name: "test", URL: srv.URL, Method: http.MethodPost},
	})

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			ips, err := r.LookupA(t.Context(), testDomainStr)
			if err != nil {
				t.Errorf("concurrent lookup: %v", err)
			}
			if len(ips) != 1 || ips[0] != testIP {
				t.Errorf("unexpected result: %v", ips)
			}
		})
	}
	wg.Wait()
}

func TestLookupEmptyDomain(t *testing.T) {
	t.Parallel()
	r := NewResolver(nil, 5*time.Second)
	_, err := r.LookupA(t.Context(), "")
	if err != nil {
		t.Logf("empty domain lookup error (expected with no network): %v", err)
	}
}

func BenchmarkNewResolver(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = NewResolver(nil, 5*time.Second)
	}
}

func BenchmarkNextProvider(b *testing.B) {
	r := NewResolver(nil, 5*time.Second)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		r.nextProvider()
	}
}

func BenchmarkLookupA(b *testing.B) {
	srv := newDNSServer(b)
	defer srv.Close()

	r := NewResolver(srv.Client(), 5*time.Second)
	r.SetProviders([]Provider{
		{Name: "test", URL: srv.URL, Method: http.MethodPost},
	})

	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = r.LookupA(ctx, testDomainStr)
	}
}
