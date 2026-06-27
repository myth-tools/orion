package active

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestIsWildcardIP(t *testing.T) {
	t.Parallel()
	r := &Resolver{
		wildcardIP: map[string]bool{"1.2.3.4": true},
	}
	tests := []struct {
		ip   string
		want bool
	}{
		{"1.2.3.4", true},
		{"5.6.7.8", false},
		{"", false},
	}
	for _, tc := range tests {
		got := r.IsWildcardIP(tc.ip)
		if got != tc.want {
			t.Errorf("IsWildcardIP(%q) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

func TestIsWildcardIPConcurrent(t *testing.T) {
	t.Parallel()
	r := &Resolver{
		wildcardIP: make(map[string]bool),
	}
	r.wildcardIP["10.0.0.1"] = true

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if !r.IsWildcardIP("10.0.0.1") {
				t.Error("expected true")
			}
			if r.IsWildcardIP("192.168.1.1") {
				t.Error("expected false")
			}
		})
	}
	wg.Wait()
}

func TestIsWildcardIPEmptyMap(t *testing.T) {
	t.Parallel()
	r := &Resolver{
		wildcardIP: make(map[string]bool),
	}
	if r.IsWildcardIP("1.2.3.4") {
		t.Error("expected false for empty map")
	}
}

func TestLookupSingleContextCancel(t *testing.T) {
	t.Parallel()
	r := &Resolver{
		wildcardIP: make(map[string]bool),
	}

	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	_, err := r.LookupSingle(ctx, "example.com")
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestLookupSingleContextAlreadyCancelled(t *testing.T) {
	t.Parallel()
	r := &Resolver{
		wildcardIP: make(map[string]bool),
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := r.LookupSingle(ctx, "example.com")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestNewResolver(t *testing.T) {
	t.Parallel()
	r := NewResolver(nil, 5*time.Second, true, nil)
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
}

func TestNewResolverWithClient(t *testing.T) {
	t.Parallel()
	r := NewResolver(http.DefaultClient, 5*time.Second, true, nil)
	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
}

func TestDetectWildcardErrors(t *testing.T) {
	t.Parallel()
	r := &Resolver{
		wildcardIP: make(map[string]bool),
	}
	_, err := r.DetectWildcard(t.Context(), "")
	if err != nil {
		t.Logf("DetectWildcard error: %v", err)
	}
}

func TestLookupSingleWithNilResolver(t *testing.T) {
	t.Parallel()
	r := &Resolver{
		wildcardIP: make(map[string]bool),
	}
	_, err := r.LookupSingle(t.Context(), "example.com")
	if err == nil {
		t.Error("expected error for nil dohResolver")
	}
}

func BenchmarkIsWildcardIP(b *testing.B) {
	r := &Resolver{
		wildcardIP: map[string]bool{"10.0.0.1": true, "10.0.0.2": true},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		r.IsWildcardIP("10.0.0.1")
	}
}
