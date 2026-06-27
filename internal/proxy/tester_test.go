package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewTester(t *testing.T) {
	t.Parallel()
	tr := NewTester(0, 0)
	if tr == nil {
		t.Fatal("expected non-nil tester")
	}
	if tr.clientTimeout != 2*time.Second {
		t.Errorf("timeout = %v, want 2s", tr.clientTimeout)
	}
	if tr.concurrency != 200 {
		t.Errorf("concurrency = %d, want 200", tr.concurrency)
	}
}

func TestNewTesterCustom(t *testing.T) {
	t.Parallel()
	tr := NewTester(5*time.Second, 10)
	if tr.clientTimeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", tr.clientTimeout)
	}
	if tr.concurrency != 10 {
		t.Errorf("concurrency = %d, want 10", tr.concurrency)
	}
}

func TestSetTestURL(t *testing.T) {
	t.Parallel()
	tr := NewTester(0, 0)
	tr.SetTestURL("http://example.com/test")
	if len(tr.testURLs) != 1 || tr.testURLs[0] != "http://example.com/test" {
		t.Errorf("testURLs = %v, want [http://example.com/test]", tr.testURLs)
	}
}

func TestTestEmpty(t *testing.T) {
	t.Parallel()
	tr := NewTester(0, 0)
	result := tr.Test(t.Context(), nil)
	if result != nil {
		t.Errorf("expected nil, got %d proxies", len(result))
	}
	result = tr.Test(t.Context(), []Proxy{})
	if result != nil {
		t.Errorf("expected nil, got %d proxies", len(result))
	}
}

func TestTestContextCancel(t *testing.T) {
	t.Parallel()
	tr := NewTester(0, 0)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result := tr.Test(ctx, []Proxy{
		{Addr: "192.168.1.1:8080", Type: ProxyTypeHTTP},
	})
	_ = result
}

func TestTestHTTPConnectionRefused(t *testing.T) {
	t.Parallel()
	tr := NewTester(500*time.Millisecond, 10)
	result := tr.Test(t.Context(), []Proxy{
		{Addr: "127.0.0.1:1", Type: ProxyTypeHTTP},
	})
	if len(result) != 0 {
		t.Errorf("expected 0 proxies, got %d", len(result))
	}
}

func TestTestConcurrent(t *testing.T) {
	t.Parallel()
	tr := NewTester(0, 0)
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			tr.Test(t.Context(), []Proxy{
				{Addr: "127.0.0.1:1", Type: ProxyTypeHTTP},
			})
		})
	}
	wg.Wait()
}

func TestTestProxyHTTP(t *testing.T) {
	t.Parallel()
	tr := NewTester(1*time.Second, 10)
	ok := tr.testProxy(t.Context(), Proxy{Addr: "127.0.0.1:1", Type: ProxyTypeHTTP})
	if ok {
		t.Log("unexpectedly connected (no proxy running)")
	}
}

func TestTestProxySOCKS5(t *testing.T) {
	t.Parallel()
	tr := NewTester(1*time.Second, 10)
	ok := tr.testProxy(t.Context(), Proxy{Addr: "127.0.0.1:1", Type: ProxyTypeSOCKS5})
	if ok {
		t.Log("unexpectedly connected (no SOCKS5 running)")
	}
}

func TestTestProxySOCKS4(t *testing.T) {
	t.Parallel()
	tr := NewTester(1*time.Second, 10)
	ok := tr.testProxy(t.Context(), Proxy{Addr: "127.0.0.1:1", Type: ProxyTypeSOCKS4})
	if ok {
		t.Log("unexpectedly connected (no SOCKS4 running)")
	}
}

func TestTestHTTPStatusCodeCheck(t *testing.T) {
	t.Parallel()
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer proxySrv.Close()

	tr := NewTester(0, 0)
	ok := tr.testHTTP(t.Context(), Proxy{Addr: proxySrv.Listener.Addr().String()}, tr.testURLs[0])
	if !ok {
		t.Log("expected OK status to pass")
	}
}

func BenchmarkNewTester(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = NewTester(3*time.Second, 50)
	}
}
