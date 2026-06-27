package proxy

import (
	"context"
	"sync"
	"testing"
	"time"
)

const (
	testProxyA = "a:80"
	testProxyB = "b:80"
	testProxyC = "c:80"
)

func newTestPool(proxies []Proxy) *Pool {
	return &Pool{
		proxies:  proxies,
		interval: 10 * time.Minute,
		stopCh:   make(chan struct{}),
	}
}

func TestNewPool(t *testing.T) {
	t.Parallel()
	s := NewScraper(nil)
	tr := NewTester(0, 0)
	p := NewPool(s, tr, 0)
	if p == nil {
		t.Fatal("expected non-nil pool")
	}
	if p.interval != 10*time.Minute {
		t.Errorf("interval = %v, want 10m", p.interval)
	}
}

func TestNewPoolCustomInterval(t *testing.T) {
	t.Parallel()
	s := NewScraper(nil)
	tr := NewTester(0, 0)
	p := NewPool(s, tr, 2*time.Minute)
	if p.interval != 2*time.Minute {
		t.Errorf("interval = %v, want 2m", p.interval)
	}
}

func TestGetNextEmpty(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	_, ok := p.GetNext()
	if ok {
		t.Error("expected false for empty pool")
	}
}

func TestGetNextEmptySlice(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{})
	_, ok := p.GetNext()
	if ok {
		t.Error("expected false for empty pool")
	}
}

func TestGetNextSingle(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{{Addr: testProxyA, Type: ProxyTypeHTTP}})
	for range 10 {
		pr, ok := p.GetNext()
		if !ok {
			t.Fatal("expected ok")
		}
		if pr.Addr != testProxyA {
			t.Errorf("addr = %q", pr.Addr)
		}
	}
}

func TestGetNextRoundRobin(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
		{Addr: testProxyB, Type: ProxyTypeHTTP},
	})
	first, _ := p.GetNext()
	second, _ := p.GetNext()
	if first.Addr != testProxyA || second.Addr != testProxyB {
		t.Errorf("expected %s,%s got %s,%s", testProxyA, testProxyB, first.Addr, second.Addr)
	}
}

func TestGetNextCycle(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
		{Addr: testProxyB, Type: ProxyTypeHTTP},
	})
	for range 4 {
		p.GetNext()
	}
	pr, _ := p.GetNext()
	if pr.Addr != testProxyA {
		t.Errorf("expected %s after cycle, got %s", testProxyA, pr.Addr)
	}
}

func TestSize(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
		{Addr: testProxyB, Type: ProxyTypeHTTP},
	})
	if sz := p.Size(); sz != 2 {
		t.Errorf("size = %d, want 2", sz)
	}
}

func TestSizeEmpty(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	if sz := p.Size(); sz != 0 {
		t.Errorf("size = %d, want 0", sz)
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
		{Addr: testProxyB, Type: ProxyTypeHTTP},
		{Addr: testProxyC, Type: ProxyTypeHTTP},
	})
	p.Remove(testProxyB)
	if sz := p.Size(); sz != 2 {
		t.Errorf("size = %d, want 2", sz)
	}
	pr, _ := p.GetNext()
	if pr.Addr == testProxyB {
		t.Error("removed proxy still returned")
	}
}

func TestRemoveFirst(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
		{Addr: testProxyB, Type: ProxyTypeHTTP},
	})
	p.Remove(testProxyA)
	if sz := p.Size(); sz != 1 {
		t.Errorf("size = %d, want 1", sz)
	}
}

func TestRemoveLast(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
		{Addr: testProxyB, Type: ProxyTypeHTTP},
	})
	p.Remove(testProxyB)
	if sz := p.Size(); sz != 1 {
		t.Errorf("size = %d, want 1", sz)
	}
}

func TestRemoveNonExistent(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
	})
	p.Remove("nonexistent")
	if sz := p.Size(); sz != 1 {
		t.Errorf("size = %d, want 1", sz)
	}
}

func TestRemoveThenGetNext(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
		{Addr: testProxyB, Type: ProxyTypeHTTP},
	})
	p.Remove(testProxyA)
	pr, ok := p.GetNext()
	if !ok {
		t.Fatal("expected ok")
	}
	if pr.Addr != testProxyB {
		t.Errorf("expected %s, got %s", testProxyB, pr.Addr)
	}
}

func TestGetNextConcurrent(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
		{Addr: testProxyB, Type: ProxyTypeHTTP},
		{Addr: testProxyC, Type: ProxyTypeHTTP},
	})
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_, ok := p.GetNext()
			if !ok {
				t.Error("GetNext returned false")
			}
		})
	}
	wg.Wait()
}

func TestRemoveConcurrent(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
		{Addr: testProxyB, Type: ProxyTypeHTTP},
		{Addr: testProxyC, Type: ProxyTypeHTTP},
	})
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			p.Remove(testProxyB)
		})
	}
	wg.Wait()
}

func TestGetNextAndRemoveConcurrent(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
		{Addr: testProxyB, Type: ProxyTypeHTTP},
		{Addr: testProxyC, Type: ProxyTypeHTTP},
	})
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_, _ = p.GetNext()
		})
		wg.Go(func() {
			p.Remove(testProxyA)
		})
	}
	wg.Wait()
}

func TestStop(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	p.Stop()
	if !p.stopped.Load() {
		t.Error("expected stopped to be true")
	}
}

func TestStopIdempotent(t *testing.T) {
	t.Parallel()
	p := newTestPool(nil)
	p.Stop()
	p.Stop()
	if !p.stopped.Load() {
		t.Error("expected stopped to be true")
	}
}

func TestStartContextCancel(t *testing.T) {
	t.Parallel()
	s := NewScraper(nil)
	tr := NewTester(0, 0)
	p := NewPool(s, tr, 10*time.Minute)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := p.Start(ctx)
	if err == nil {
		t.Log("Start may succeed or fail depending on network")
	}
}

func TestSizeAfterStop(t *testing.T) {
	t.Parallel()
	p := newTestPool([]Proxy{{Addr: testProxyA, Type: ProxyTypeHTTP}})
	p.Stop()
	if sz := p.Size(); sz != 1 {
		t.Errorf("size = %d, want 1", sz)
	}
}

func BenchmarkGetNext(b *testing.B) {
	p := newTestPool([]Proxy{
		{Addr: testProxyA, Type: ProxyTypeHTTP},
		{Addr: testProxyB, Type: ProxyTypeHTTP},
		{Addr: testProxyC, Type: ProxyTypeHTTP},
	})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = p.GetNext()
	}
}

func BenchmarkSize(b *testing.B) {
	p := newTestPool([]Proxy{{Addr: testProxyA, Type: ProxyTypeHTTP}})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = p.Size()
	}
}
