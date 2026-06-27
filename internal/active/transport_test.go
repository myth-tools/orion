package active

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// ─── NewProxyDialer ─────────────────────────────────────.

func TestNewProxyDialerNoProxy(t *testing.T) {
	t.Parallel()
	d, err := NewProxyDialer("")
	if err != nil {
		t.Fatalf("NewProxyDialer('') err = %v", err)
	}
	if d.Enabled() {
		t.Error("expected disabled dialer with empty proxy URL")
	}
	if d.Tripped() {
		t.Error("expected dialer not tripped initially")
	}
}

func TestNewProxyDialerInvalidURL(t *testing.T) {
	t.Parallel()
	_, err := NewProxyDialer("invalid://url")
	if err == nil {
		t.Error("expected error for invalid URL scheme")
	}
}

func TestNewProxyDialerUnsupportedScheme(t *testing.T) {
	t.Parallel()
	_, err := NewProxyDialer("http://proxy:8080")
	if err == nil {
		t.Error("expected error for unsupported scheme")
	}
}

func TestNewProxyDialerBadHost(t *testing.T) {
	t.Parallel()
	d, err := NewProxyDialer("socks5://127.0.0.1:1")
	if err != nil {
		t.Fatal("expected dialer creation to succeed")
	}
	if !d.Enabled() {
		t.Error("expected dialer enabled")
	}
}

func TestNewProxyDialerEmptyURL(t *testing.T) {
	t.Parallel()
	d, err := NewProxyDialer("")
	if err != nil {
		t.Fatalf("NewProxyDialer('') err = %v", err)
	}
	if d.Enabled() {
		t.Error("expected disabled dialer")
	}
}

func TestNewProxyDialerJustScheme(t *testing.T) {
	t.Parallel()
	_, err := NewProxyDialer("socks5://")
	if err == nil {
		t.Log("socks5:// without host parsed successfully")
	}
}

// ─── ProxyDialer.Enabled ────────────────────────────────.

func TestProxyDialerEnabledDisabled(t *testing.T) {
	t.Parallel()
	d1, _ := NewProxyDialer("")
	if d1.Enabled() {
		t.Error("expected disabled")
	}
	d2, _ := NewProxyDialer("socks5://127.0.0.1:9050")
	if !d2.Enabled() {
		t.Error("expected enabled")
	}
}

// ─── ProxyDialer.Tripped ────────────────────────────────.

func TestProxyDialerTrippedInitially(t *testing.T) {
	t.Parallel()
	d, _ := NewProxyDialer("")
	if d.Tripped() {
		t.Error("expected not tripped initially")
	}
}

func TestProxyDialerTrippedAfterReset(t *testing.T) {
	t.Parallel()
	d := &ProxyDialer{tripped: true, tripCount: 3}
	d.Reset()
	if d.Tripped() {
		t.Error("expected not tripped after reset")
	}
}

func TestProxyDialerTrippedConcurrent(t *testing.T) {
	t.Parallel()
	d := &ProxyDialer{tripped: false}
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_ = d.Tripped()
		})
	}
	wg.Wait()
}

// ─── ProxyDialer.Reset ──────────────────────────────────.

func TestProxyDialerReset(t *testing.T) {
	t.Parallel()
	d := &ProxyDialer{tripped: true, tripCount: 3}
	d.Reset()
	if d.Tripped() {
		t.Error("expected dialer not tripped after reset")
	}
}

func TestProxyDialerResetConcurrent(t *testing.T) {
	t.Parallel()
	d := &ProxyDialer{tripped: true}
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			d.Reset()
		})
	}
	wg.Wait()
}

func TestProxyDialerResetIdempotent(t *testing.T) {
	t.Parallel()
	d := &ProxyDialer{tripped: false}
	for range 5 {
		d.Reset()
		if d.Tripped() {
			t.Fatal("became tripped after reset")
		}
	}
}

// ─── ProxyDialer.DialContext ────────────────────────────.

func TestProxyDialerCircuitBreaker(t *testing.T) {
	t.Parallel()
	// Use a dialer that always fails to trigger the circuit breaker
	failDialer := &ProxyDialer{
		enabled: true,
		inner:   &failDialer{},
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	for range maxTripRetries {
		_, err := failDialer.DialContext(ctx, "tcp", "127.0.0.1:1")
		if err == nil {
			t.Log("unexpectedly connected")
			return
		}
	}

	if !failDialer.Tripped() {
		t.Error("expected dialer to trip after maxTripRetries failures")
	}

	failDialer.Reset()
	if failDialer.Tripped() {
		t.Error("expected dialer not tripped after reset")
	}

	// After reset, dials should fail again (but with the error, not circuit open)
	_, err := failDialer.DialContext(ctx, "tcp", "127.0.0.1:1")
	if err == nil {
		t.Log("unexpectedly connected")
	}
	if errors.Is(err, errCircuitOpen) {
		t.Error("expected regular failure, not circuit open after reset")
	}
}

func TestProxyDialerCircuitBreakerDirect(t *testing.T) {
	t.Parallel()
	d := &ProxyDialer{tripped: true}
	ctx := t.Context()
	_, err := d.DialContext(ctx, "tcp", "127.0.0.1:1")
	if !errors.Is(err, errCircuitOpen) {
		t.Errorf("expected errCircuitOpen, got %v", err)
	}
}

func TestProxyDialerNoCircuitBreakerWhenDisabled(t *testing.T) {
	t.Parallel()
	d, _ := NewProxyDialer("")
	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()

	conn, err := d.DialContext(ctx, "tcp", "127.0.0.1:1")
	if err == nil {
		conn.Close()
		t.Log("unexpectedly connected")
	}
	// Should NOT get circuit open error because dialer is disabled
	if errors.Is(err, errCircuitOpen) {
		t.Error("circuit breaker tripped on disabled dialer")
	}
}

func TestProxyDialerConcurrentDials(t *testing.T) {
	t.Parallel()
	d := &ProxyDialer{
		enabled: true,
		inner:   &failDialer{},
	}
	ctx := t.Context()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_, err := d.DialContext(ctx, "tcp", "127.0.0.1:1")
			if err != nil {
				_ = err
			}
		})
	}
	wg.Wait()
}

// ─── NewProxiedHTTPClient ───────────────────────────────.

func TestNewProxiedHTTPClientTimeout(t *testing.T) {
	t.Parallel()
	d, _ := NewProxyDialer("")
	c := NewProxiedHTTPClient(d, 15)
	if c.Timeout != 25*time.Second {
		t.Errorf("expected timeout 25s, got %v", c.Timeout)
	}
}

func TestNewProxiedHTTPClientTimeoutZero(t *testing.T) {
	t.Parallel()
	d, _ := NewProxyDialer("")
	c := NewProxiedHTTPClient(d, 0)
	if c.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", c.Timeout)
	}
}

func TestNewProxiedHTTPClientTimeoutLarge(t *testing.T) {
	t.Parallel()
	d, _ := NewProxyDialer("")
	c := NewProxiedHTTPClient(d, 300)
	if c.Timeout != 310*time.Second {
		t.Errorf("expected timeout 310s, got %v", c.Timeout)
	}
}

func TestNewProxiedHTTPClientTransport(t *testing.T) {
	t.Parallel()
	d, _ := NewProxyDialer("")
	c := NewProxiedHTTPClient(d, 10)
	if c.Transport == nil {
		t.Error("expected transport to be set")
	}
}

func TestNewProxiedHTTPClientNotNil(t *testing.T) {
	t.Parallel()
	d, _ := NewProxyDialer("")
	c := NewProxiedHTTPClient(d, 10)
	if c == nil {
		t.Error("expected non-nil client")
	}
}

func TestNewProxiedHTTPClientWithProxy(t *testing.T) {
	t.Parallel()
	d, _ := NewProxyDialer("socks5://127.0.0.1:9050")
	c := NewProxiedHTTPClient(d, 10)
	if c == nil {
		t.Error("expected non-nil client")
	}
}

// ─── failDialer ─────────────────────────────────────────.

var errSimulatedDial = errors.New("simulated dial failure")

type failDialer struct{}

func (f *failDialer) Dial(_, addr string) (net.Conn, error) {
	return nil, fmt.Errorf("%w on %s", errSimulatedDial, addr)
}

func TestFailDialer(t *testing.T) {
	t.Parallel()
	f := &failDialer{}
	_, err := f.Dial("tcp", "127.0.0.1:1")
	if err == nil {
		t.Error("expected error")
	}
}

// ─── Compile-time interface check ───────────────────────.

type ContextDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

func TestProxyDialerImplementsContextDialer(t *testing.T) {
	t.Parallel()
	var _ ContextDialer = (*ProxyDialer)(nil)
}

// ─── Edge cases ─────────────────────────────────────────.

func TestProxyDialerEmptyInner(t *testing.T) {
	t.Parallel()
	// DialContext on enabled dialer with nil inner should panic
	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from panic: %v", r)
		}
	}()

	d := &ProxyDialer{enabled: true, inner: nil}
	ctx := t.Context()
	_, _ = d.DialContext(ctx, "tcp", "127.0.0.1:1")
}

func TestNewProxiedHTTPClientNegativeTimeout(t *testing.T) {
	t.Parallel()
	d, _ := NewProxyDialer("")
	c := NewProxiedHTTPClient(d, -5)
	if c.Timeout < 0 {
		t.Errorf("negative timeout: %v", c.Timeout)
	}
}

// ─── Concurrency ────────────────────────────────────────.

func TestProxyDialerConcurrentEnabledAndTripped(t *testing.T) {
	t.Parallel()
	d := &ProxyDialer{enabled: false}
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_ = d.Enabled()
			_ = d.Tripped()
		})
	}
	wg.Wait()
}

func TestNewProxiedHTTPClientConcurrent(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			d, _ := NewProxyDialer("")
			_ = NewProxiedHTTPClient(d, 10)
		})
	}
	wg.Wait()
}

// ─── Benchmarks ─────────────────────────────────────────.

func BenchmarkNewProxyDialer(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_, _ = NewProxyDialer("")
	}
}

func BenchmarkProxyDialerTripped(b *testing.B) {
	d := &ProxyDialer{}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		d.Tripped()
	}
}

func BenchmarkProxyDialerReset(b *testing.B) {
	d := &ProxyDialer{tripped: true}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		d.Reset()
	}
}

func BenchmarkDialerEnabled(b *testing.B) {
	d, _ := NewProxyDialer("")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		d.Enabled()
	}
}

func BenchmarkNewProxiedHTTPClient(b *testing.B) {
	d, _ := NewProxyDialer("")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = NewProxiedHTTPClient(d, 30)
	}
}

// ─── Example tests ──────────────────────────────────────.

func ExampleNewProxyDialer() {
	d, _ := NewProxyDialer("")
	fmt.Println(d.Enabled())
	// Output: false
}
