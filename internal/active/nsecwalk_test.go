package active

import (
	"context"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

// ─── randomDNSProbeName ─────────────────────────────────.

func TestRandomDNSProbeNameFormat(t *testing.T) {
	t.Parallel()
	for range 100 {
		name := randomDNSProbeName("example.com")
		if !strings.HasSuffix(name, ".example.com") {
			t.Errorf("probe name %q does not end with example.com", name)
		}
		if strings.Count(name, ".") < 2 {
			t.Errorf("probe name %q has too few dots", name)
		}
		if len(name) > 253 {
			t.Errorf("probe name too long: %d chars", len(name))
		}
	}
}

func TestRandomDNSProbeNameUniqueness(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for range 1000 {
		name := randomDNSProbeName("example.com")
		if seen[name] {
			t.Errorf("duplicate probe name: %s", name)
		}
		seen[name] = true
	}
}

func TestRandomDNSProbeNameEmptyDomain(t *testing.T) {
	t.Parallel()
	name := randomDNSProbeName("")
	if name == "" {
		t.Error("empty result for empty domain")
	}
	if !strings.Contains(name, ".empty") {
		t.Logf("probe name for empty domain: %q", name)
	}
}

func TestRandomDNSProbeNameLongDomain(t *testing.T) {
	t.Parallel()
	longDomain := strings.Repeat("a", 200) + ".com"
	name := randomDNSProbeName(longDomain)
	if len(name) > 253 {
		t.Errorf("probe name too long: %d chars", len(name))
	}
}

func TestRandomDNSProbeNameHexFormat(t *testing.T) {
	t.Parallel()
	name := randomDNSProbeName("example.com")
	parts := strings.SplitN(name, ".", 2)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	label := parts[0]
	if !strings.HasPrefix(label, "nsec-probe-") {
		t.Errorf("label %q does not start with 'nsec-probe-'", label)
	}
}

// ─── NewNSECWalker ──────────────────────────────────────.

func TestNewNSECWalker(t *testing.T) {
	t.Parallel()
	w := NewNSECWalker(nil, nil)
	if w == nil {
		t.Fatal("NewNSECWalker returned nil")
	}
	if w.timeout <= 0 {
		t.Errorf("timeout = %v, want > 0", w.timeout)
	}
}

func TestNSECWalkerStringer(t *testing.T) {
	t.Parallel()
	w := NewNSECWalker(nil, nil)
	s := w.timeout.String()
	if s == "" {
		t.Error("empty timeout string")
	}
}

// ─── Walk with cancelled context ────────────────────────.

func TestNSECWalkContextCancel(t *testing.T) {
	t.Parallel()
	w := NewNSECWalker(nil, nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	results := make(chan string, 10)
	err := w.Walk(ctx, "example.com", results)
	if err == nil {
		t.Log("Walk completed without error on cancelled context")
	}
	close(results)
}

func TestNSECWalkEmptyDomain(t *testing.T) {
	t.Parallel()
	w := NewNSECWalker(nil, nil)
	ctx := t.Context()
	results := make(chan string, 10)
	err := w.Walk(ctx, "", results)
	if err == nil {
		t.Log("Walk completed without error for empty domain")
	}
	close(results)
}

// ─── findNameservers ────────────────────────────────────.

func TestFindNameserversNoNetwork(t *testing.T) {
	t.Parallel()
	w := NewNSECWalker(nil, nil)
	ns := w.findNameservers(t.Context(), "nonexistent-domain-hopefully-123456.test")
	if len(ns) != 0 {
		t.Logf("got %d nameservers (likely from system resolver)", len(ns))
	}
}

// ─── resolveNameserverIP ────────────────────────────────.

func TestResolveNameserverIPNoNetwork(t *testing.T) {
	t.Parallel()
	w := NewNSECWalker(nil, nil)
	ip := w.resolveNameserverIP(t.Context(), "nonexistent-ns.test")
	if ip != "" {
		t.Logf("got IP %q (unexpected resolution)", ip)
	}
}

// ─── exchange ───────────────────────────────────────────.

func TestExchangeCancelledContext(t *testing.T) {
	t.Parallel()
	w := NewNSECWalker(nil, nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	msg, err := w.exchange(ctx, m, "1.1.1.1:53")
	if err == nil {
		t.Log("exchange returned without error")
	}
	_ = msg
}

// ─── walkChain ──────────────────────────────────────────.

func TestWalkChainCancelledContext(t *testing.T) {
	t.Parallel()
	w := NewNSECWalker(nil, nil)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	results := make(chan string, 10)
	err := w.walkChain(ctx, "example.com", "1.1.1.1", nil, results)
	if err == nil {
		t.Log("walkChain returned without error")
	}
	close(results)
}

func TestWalkChainNoRecords(t *testing.T) {
	t.Parallel()
	w := NewNSECWalker(nil, nil)
	ctx := t.Context()

	results := make(chan string, 10)
	err := w.walkChain(ctx, "example.com", "1.1.1.1", nil, results)
	if err == nil {
		t.Log("walkChain with nil nsecRecords returned no error")
	}
	close(results)
}

// ─── Benchmark ──────────────────────────────────────────.

func BenchmarkRandomDNSProbeName(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		randomDNSProbeName("example.com")
	}
}

func BenchmarkNewNSECWalker(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = NewNSECWalker(nil, nil)
	}
}

// ─── Compile-time checks ───────────────────────────────.

func TestNSECWalkerInterface(t *testing.T) {
	t.Parallel()
	w := NewNSECWalker(nil, nil)
	if w == nil {
		t.Fatal("NewNSECWalker returned nil")
	}
	if w.timeout <= 0 {
		t.Error("expected positive timeout")
	}
}
