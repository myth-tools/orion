package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewScraper(t *testing.T) {
	t.Parallel()
	s := NewScraper(nil)
	if s == nil {
		t.Fatal("expected non-nil scraper")
	}
	if s.client.Timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", s.client.Timeout)
	}
}

func TestNewScraperWithClient(t *testing.T) {
	t.Parallel()
	client := &http.Client{Timeout: 30 * time.Second}
	s := NewScraper(client)
	if s.client.Timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", s.client.Timeout)
	}
}

func TestAddSource(t *testing.T) {
	t.Parallel()
	s := NewScraper(nil)
	initial := len(s.sources)
	s.AddSource("https://example.com/proxies.txt")
	if len(s.sources) != initial+1 {
		t.Errorf("sources = %d, want %d", len(s.sources), initial+1)
	}
}

func TestDetectTypeHTTP(t *testing.T) {
	t.Parallel()
	if got := detectType("https://example.com/http.txt"); got != ProxyTypeHTTP {
		t.Errorf("detectType = %q, want %q", got, ProxyTypeHTTP)
	}
}

func TestDetectTypeSOCKS4(t *testing.T) {
	t.Parallel()
	if got := detectType("https://example.com/socks4.txt"); got != ProxyTypeSOCKS4 {
		t.Errorf("detectType = %q, want %q", got, ProxyTypeSOCKS4)
	}
}

func TestDetectTypeSOCKS5(t *testing.T) {
	t.Parallel()
	if got := detectType("https://example.com/socks5.txt"); got != ProxyTypeSOCKS5 {
		t.Errorf("detectType = %q, want %q", got, ProxyTypeSOCKS5)
	}
}

func TestDetectTypeCaseInsensitive(t *testing.T) {
	t.Parallel()
	if got := detectType("https://example.com/SOCKS5/list.txt"); got != ProxyTypeSOCKS5 {
		t.Errorf("detectType = %q, want %q", got, ProxyTypeSOCKS5)
	}
}

func TestDetectTypeDefault(t *testing.T) {
	t.Parallel()
	if got := detectType("https://example.com/proxies.txt"); got != ProxyTypeHTTP {
		t.Errorf("detectType = %q, want %q", got, ProxyTypeHTTP)
	}
}

func TestProxyLineReValid(t *testing.T) {
	t.Parallel()
	cases := []string{
		"192.168.1.1:8080",
		"10.0.0.1:3128",
		"1.2.3.4:80",
		"255.255.255.255:65535",
	}
	for _, c := range cases {
		if !proxyLineRe.MatchString(c) {
			t.Errorf("expected match for %q", c)
		}
	}
}

func TestProxyLineReInvalid(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"not a proxy",
		"192.168.1.1",
		"192.168.1.1:abc",
		"user:pass@proxy:8080",
	}
	for _, c := range cases {
		if proxyLineRe.MatchString(c) {
			t.Errorf("expected no match for %q", c)
		}
	}
}

func TestFetchProxies(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("192.168.1.1:8080\n10.0.0.1:3128\n# comment\ninvalid\n"))
	}))
	defer srv.Close()

	s := NewScraper(nil)
	s.sources = []string{srv.URL}

	proxies, err := s.fetchProxies(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("fetchProxies: %v", err)
	}
	if len(proxies) != 2 {
		t.Errorf("expected 2 proxies, got %d: %v", len(proxies), proxies)
	}
	if proxies[0].Addr != "192.168.1.1:8080" || proxies[1].Addr != "10.0.0.1:3128" {
		t.Errorf("unexpected proxies: %v", proxies)
	}
}

func TestFetchProxiesHTTPError(t *testing.T) {
	t.Parallel()
	s := NewScraper(nil)
	_, err := s.fetchProxies(t.Context(), "http://127.0.0.1:1/nonexistent")
	if err == nil {
		t.Error("expected error for unreachable URL")
	}
}

func TestFetchProxiesTypeDetection(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("192.168.1.1:8080\n10.0.0.1:3128\n"))
	}))
	defer srv.Close()

	s := NewScraper(nil)
	proxies, err := s.fetchProxies(t.Context(), srv.URL+"/socks5")
	if err != nil {
		t.Fatalf("fetchProxies: %v", err)
	}
	for _, p := range proxies {
		if p.Type != ProxyTypeSOCKS5 {
			t.Errorf("expected SOCKS5 type, got %q", p.Type)
		}
	}
}

func TestFetchProxiesSchemePrefix(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("http://192.168.1.1:8080\nsocks5://10.0.0.1:1080\nhttps://10.0.0.2:443\n"))
	}))
	defer srv.Close()

	s := NewScraper(nil)
	proxies, err := s.fetchProxies(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("fetchProxies: %v", err)
	}
	if len(proxies) != 3 {
		t.Fatalf("expected 3 proxies, got %d: %v", len(proxies), proxies)
	}
	expected := map[string]bool{
		"192.168.1.1:8080": true,
		"10.0.0.1:1080":    true,
		"10.0.0.2:443":     true,
	}
	for _, p := range proxies {
		if !expected[p.Addr] {
			t.Errorf("unexpected proxy addr: %q", p.Addr)
		}
	}
}

func TestScrapeContextCancel(t *testing.T) {
	t.Parallel()
	s := NewScraper(nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := s.Scrape(ctx)
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestScrapeDeduplicates(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("192.168.1.1:8080\n192.168.1.1:8080\n"))
	}))
	defer srv.Close()

	s := NewScraper(nil)
	s.sources = []string{srv.URL, srv.URL}

	proxies, err := s.Scrape(t.Context())
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}
	seen := make(map[string]bool)
	for _, p := range proxies {
		key := p.Type + "://" + p.Addr
		if seen[key] {
			t.Errorf("duplicate found: %s", key)
		}
		seen[key] = true
	}
	if len(proxies) != 1 {
		t.Errorf("expected 1 unique proxy, got %d", len(proxies))
	}
}

func TestFetchProxiesEmptyLine(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("192.168.1.1:8080\n\n10.0.0.1:3128\n"))
	}))
	defer srv.Close()

	s := NewScraper(nil)
	proxies, err := s.fetchProxies(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("fetchProxies: %v", err)
	}
	if len(proxies) != 2 {
		t.Errorf("expected 2 proxies, got %d", len(proxies))
	}
}

func BenchmarkDetectType(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		detectType("https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt")
	}
}
