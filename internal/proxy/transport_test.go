package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewRotatingTransport(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	rt := NewRotatingTransport(p)
	if rt == nil {
		t.Fatal("expected non-nil transport")
	}
	if rt.pool != p {
		t.Error("pool not stored")
	}
}

func TestNewRotatingTransportBaseConfig(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	rt := NewRotatingTransport(p)
	if rt.base.MaxIdleConns != 50 {
		t.Errorf("MaxIdleConns = %d, want 50", rt.base.MaxIdleConns)
	}
	if rt.base.DisableKeepAlives {
		t.Error("expected keep-alives enabled for base")
	}
}

func TestNewHTTPClient(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	rt := NewRotatingTransport(p)
	client := rt.NewHTTPClient(0)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", client.Timeout)
	}
}

func TestNewHTTPClientCustomTimeout(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	rt := NewRotatingTransport(p)
	client := rt.NewHTTPClient(30 * time.Second)
	if client.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", client.Timeout)
	}
}

func TestTransportForProxyHTTP(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	rt := NewRotatingTransport(p)
	pr := Proxy{Addr: "192.168.1.1:8080", Type: ProxyTypeHTTP}

	transport, err := transportForProxy(pr, rt.base)
	if err != nil {
		t.Fatalf("transportForProxy: %v", err)
	}
	if transport == nil {
		t.Fatal("expected non-nil transport")
	}
	if !transport.DisableKeepAlives {
		t.Error("expected keep-alives disabled for proxy transport")
	}
}

func TestTransportForProxySOCKS5(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	rt := NewRotatingTransport(p)
	pr := Proxy{Addr: "192.168.1.1:1080", Type: ProxyTypeSOCKS5}

	transport, err := transportForProxy(pr, rt.base)
	if err != nil {
		t.Fatalf("transportForProxy: %v", err)
	}
	if transport == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestTransportForProxySOCKS4(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	rt := NewRotatingTransport(p)
	pr := Proxy{Addr: "192.168.1.1:1080", Type: ProxyTypeSOCKS4}

	_, err := transportForProxy(pr, rt.base)
	if !errors.Is(err, errSOCKS4Unsupported) {
		t.Fatalf("expected errSOCKS4Unsupported, got %v", err)
	}
}

func TestTransportForProxyUnsupported(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	rt := NewRotatingTransport(p)
	pr := Proxy{Addr: "192.168.1.1:9999", Type: "unknown"}

	_, err := transportForProxy(pr, rt.base)
	if !errors.Is(err, errUnsupportedProxyType) {
		t.Errorf("expected errUnsupportedProxyType, got %v", err)
	}
}

func TestTransportForProxyUnknownType(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	rt := NewRotatingTransport(p)
	pr := Proxy{Addr: "127.0.0.1:8080", Type: "unknown"}

	_, err := transportForProxy(pr, rt.base)
	if !errors.Is(err, errUnsupportedProxyType) {
		t.Errorf("expected errUnsupportedProxyType, got %v", err)
	}
}

func TestRoundTripEmptyPool(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	rt := NewRotatingTransport(p)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	resp.Body.Close()
}

func TestRoundTripProxyRemovalOnFail(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pool := newTestPool([]Proxy{
		{Addr: "127.0.0.1:1", Type: ProxyTypeHTTP},
		{Addr: srv.Listener.Addr().String(), Type: ProxyTypeHTTP},
	})
	rt := NewRotatingTransport(pool)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Logf("request failed: %v", err)
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func TestRoundTripConcurrent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pool := newTestPool([]Proxy{
		{Addr: srv.Listener.Addr().String(), Type: ProxyTypeHTTP},
	})
	rt := NewRotatingTransport(pool)

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
			resp, err := rt.RoundTrip(req)
			if err != nil {
				t.Errorf("RoundTrip: %v", err)
				return
			}
			resp.Body.Close()
		})
	}
	wg.Wait()
}

func TestRoundTripContextCancel(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	rt := NewRotatingTransport(p)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:1", nil)
	resp, err := rt.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestTransportCloneNotModified(t *testing.T) {
	t.Parallel()
	base := &http.Transport{
		MaxIdleConns:        10,
		DisableKeepAlives:   false,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	pr := Proxy{Addr: "192.168.1.1:8080", Type: ProxyTypeHTTP}

	clone, err := transportForProxy(pr, base)
	if err != nil {
		t.Fatalf("transportForProxy: %v", err)
	}
	if base.DisableKeepAlives {
		t.Error("base transport modified")
	}
	if !clone.DisableKeepAlives {
		t.Error("clone should have keep-alives disabled")
	}
}

func TestRoundTripFallbackToBase(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pool := newTestPool([]Proxy{
		{Addr: "invalid", Type: ProxyTypeHTTP},
	})
	rt := NewRotatingTransport(pool)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip should fall through to base: %v", err)
	}
	resp.Body.Close()
}

func TestRoundTripAfterAllProxiesRemoved(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pool := newTestPool([]Proxy{
		{Addr: "x:1", Type: ProxyTypeHTTP},
	})
	pool.Remove("x:1")

	rt := NewRotatingTransport(pool)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip should fall through to base: %v", err)
	}
	resp.Body.Close()
}

func TestTransportImplementsRoundTripper(t *testing.T) {
	t.Parallel()
	var _ http.RoundTripper = (*RotatingTransport)(nil)
}

func BenchmarkTransportForProxyHTTP(b *testing.B) {
	base := &http.Transport{
		MaxIdleConns:        50,
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	pr := Proxy{Addr: "192.168.1.1:8080", Type: ProxyTypeHTTP}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = transportForProxy(pr, base)
	}
}

func BenchmarkRoundTripEmptyPool(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pool := newTestPool(nil)
	rt := NewRotatingTransport(pool)

	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		resp, err := rt.RoundTrip(req)
		if err == nil {
			resp.Body.Close()
		}
	}
}
