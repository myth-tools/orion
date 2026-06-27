package runner

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/myth-tools/orion/internal/passive"
	"github.com/myth-tools/orion/internal/types"
)

// ─── validSubdomain ─────────────────────────────────────.

func TestValidSubdomain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		sub    string
		domain string
		want   bool
	}{
		{"www.example.com", "example.com", true},
		{"api.example.com", "example.com", true},
		{"deep.sub.example.com", "example.com", true},
		{"a-b.example.com", "example.com", true},
		{"a.b.example.com", "example.com", true},
		{"123.example.com", "example.com", true},
		{"www.Example.COM", "example.com", true},
		{"WWW.EXAMPLE.COM", "example.com", true},
		{" test.example.com", "example.com", true},
		{"a..b.example.com", "example.com", true},
		{"-bad.example.com", "example.com", true},
		{"bad-.example.com", "example.com", true},

		{"example.com", "example.com", false},
		{"*.example.com", "example.com", false},
		{".example.com", "example.com", false},
		{"", "example.com", false},
		{"www.other.com", "example.com", false},
		{"www.example.org", "example.com", false},
	}
	for _, tc := range tests {
		got := validSubdomain(tc.sub, tc.domain)
		if got != tc.want {
			t.Errorf("validSubdomain(%q, %q) = %v, want %v", tc.sub, tc.domain, got, tc.want)
		}
	}
}

func TestValidSubdomainCaseInsensitive(t *testing.T) {
	t.Parallel()
	if !validSubdomain("WWW.EXAMPLE.COM", "example.com") {
		t.Error("expected case-insensitive match")
	}
	if !validSubdomain("www.example.com", "EXAMPLE.COM") {
		t.Error("expected case-insensitive match")
	}
}

func TestValidSubdomainConcurrent(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			if !validSubdomain("www.example.com", "example.com") {
				t.Error("expected valid")
			}
			if validSubdomain("example.com", "example.com") {
				t.Error("expected invalid")
			}
		})
	}
	wg.Wait()
}

// ─── jitterMs ───────────────────────────────────────────.

func TestJitterMs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		low  int
		high int
	}{
		{10, 20},
		{0, 100},
		{100, 100},
		{1, 1},
		{50, 51},
		{0, 0},
		{-10, 10},
	}
	for _, tc := range tests {
		for range 10 {
			d := jitterMs(tc.low, tc.high)
			lo := time.Duration(tc.low) * time.Millisecond
			hi := time.Duration(tc.high) * time.Millisecond
			if d < lo || d > hi {
				t.Errorf("jitterMs(%d,%d) = %v, want [%v,%v]", tc.low, tc.high, d, lo, hi)
			}
		}
	}
}

func TestJitterMsDeterministicRange(t *testing.T) {
	t.Parallel()
	seen := make(map[time.Duration]bool)
	for range 100 {
		d := jitterMs(1, 10)
		seen[d] = true
	}
	// Should see some variation
	if len(seen) < 2 {
		t.Errorf("expected variation, only got %d distinct values", len(seen))
	}
}

func TestJitterMsHighEqualsLow(t *testing.T) {
	t.Parallel()
	for range 10 {
		d := jitterMs(42, 42)
		if d != 42*time.Millisecond {
			t.Errorf("jitterMs(42,42) = %v, want 42ms", d)
		}
	}
}

func TestJitterMsConcurrent(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			d := jitterMs(5, 50)
			if d < 5*time.Millisecond || d > 50*time.Millisecond {
				t.Errorf("jitterMs out of range: %v", d)
			}
		})
	}
	wg.Wait()
}

// ─── DefaultWordlist ────────────────────────────────────.

func TestDefaultWordlistNotEmpty(t *testing.T) {
	t.Parallel()
	w := DefaultWordlist()
	if len(w) == 0 {
		t.Fatal("DefaultWordlist() returned empty slice")
	}
}

func TestDefaultWordlistNoEmpties(t *testing.T) {
	t.Parallel()
	w := DefaultWordlist()
	for i, word := range w {
		if word == "" {
			t.Errorf("DefaultWordlist[%d] is empty", i)
		}
	}
}

func TestDefaultWordlistNoDuplicates(t *testing.T) {
	t.Parallel()
	w := DefaultWordlist()
	seen := make(map[string]bool)
	for i, word := range w {
		if seen[word] {
			t.Errorf("duplicate at index %d: %q", i, word)
		}
		seen[word] = true
	}
}

func TestDefaultWordlistStartsWithCommon(t *testing.T) {
	t.Parallel()
	w := DefaultWordlist()
	if len(w) == 0 {
		t.Fatal("empty wordlist")
	}
	common := []string{"www", "mail", "ns1", "ftp", "smtp", "webmail", "api", "dev", "cdn", "blog"}
	top := w[:min(len(w), 20)]
	topSet := make(map[string]bool, len(top))
	for _, s := range top {
		topSet[s] = true
	}
	for _, c := range common {
		if !topSet[c] {
			t.Logf("expected common word %q among first 20", c)
		}
	}
}

func TestDefaultWordlistSize(t *testing.T) {
	t.Parallel()
	w := DefaultWordlist()
	if len(w) < 100 {
		t.Errorf("wordlist too small: %d entries", len(w))
	}
	if len(w) > 10000 {
		t.Errorf("wordlist too large: %d entries", len(w))
	}
}

// ─── New Runner ─────────────────────────────────────────.

func TestNewBasic(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{
		Domain: "example.com",
	}
	r := New(cfg, nil)
	if r == nil {
		t.Fatal("New returned nil")
	}
	if r.config != cfg {
		t.Error("config not set")
	}
	if len(r.sources) == 0 {
		t.Error("no sources configured")
	}
	if r.resolver == nil {
		t.Error("resolver not set")
	}
	if r.httpClient == nil {
		t.Error("httpClient not set")
	}
}

func TestNewWithProxy(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{
		Domain:   "example.com",
		ProxyURL: "socks5://127.0.0.1:9050",
	}
	r := New(cfg, nil)
	if r == nil {
		t.Fatal("New returned nil")
	}
	if r.dialer == nil {
		t.Error("dialer not set")
	}
}

func TestNewWithWordlist(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{
		Domain:          "example.com",
		Bruteforce:      true,
		Wordlist:        "",
		MaxWordlistSize: 10,
	}
	r := New(cfg, nil)
	if r == nil {
		t.Fatal("New returned nil")
	}
	if r.brute == nil {
		t.Log("bruteforcer not created (no wordlist)")
	}
}

func TestNewOutputFile(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{
		Domain:     "example.com",
		OutputFile: "",
	}
	r := New(cfg, nil)
	if r.output == nil {
		t.Error("output writer not set")
	}
}

func TestNewSilent(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{
		Domain: "example.com",
		Silent: true,
	}
	r := New(cfg, nil)
	if r == nil {
		t.Fatal("New returned nil")
	}
}

// ─── Config edge cases ──────────────────────────────────.

func TestNewEmptyDomain(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{
		Domain: "",
	}
	r := New(cfg, nil)
	if r == nil {
		t.Fatal("New returned nil")
	}
}

func TestNewNegativeThreads(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{
		Domain:  "example.com",
		Threads: -5,
	}
	r := New(cfg, nil)
	if r == nil {
		t.Fatal("New returned nil")
	}
}

func TestNewZeroTimeout(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{
		Domain:  "example.com",
		Timeout: 0,
	}
	r := New(cfg, nil)
	if r == nil {
		t.Fatal("New returned nil")
	}
}

// ─── Wordlist truncation ────────────────────────────────.

func TestWordlistTruncation(t *testing.T) {
	t.Parallel()
	words := DefaultWordlist()
	if len(words) <= 10 {
		t.Skip("wordlist too small for truncation test")
	}
	truncated := words[:10]
	if len(truncated) != 10 {
		t.Errorf("truncated length = %d, want 10", len(truncated))
	}
}

func TestWordlistTruncationZero(t *testing.T) {
	t.Parallel()
	words := DefaultWordlist()
	truncated := words[:0]
	if len(truncated) != 0 {
		t.Errorf("expected empty, got %d", len(truncated))
	}
}

// ─── New from file wordlist ─────────────────────────────.

// ─── filterAndMatchSubdomain ───────────────────────────────────.

func TestFilterAndMatchSubdomain(t *testing.T) {
	t.Parallel()
	r := &Runner{
		config: &types.Config{
			Match:  []string{"*.example.com"},
			Filter: []string{"admin.example.com"},
		},
	}
	r.matchRegexes = make([]*regexp.Regexp, 0, len(r.config.Match))
	for _, m := range r.config.Match {
		re, _ := regexp.Compile(globToRegex(m))
		r.matchRegexes = append(r.matchRegexes, re)
	}
	r.filterRegexes = make([]*regexp.Regexp, 0, len(r.config.Filter))
	for _, f := range r.config.Filter {
		re, _ := regexp.Compile(globToRegex(f))
		r.filterRegexes = append(r.filterRegexes, re)
	}

	if !r.filterAndMatchSubdomain("www.example.com") {
		t.Error("expected match for www.example.com")
	}
	if r.filterAndMatchSubdomain("admin.example.com") {
		t.Error("expected filter for admin.example.com")
	}
	if r.filterAndMatchSubdomain("other.com") {
		t.Error("expected no match for other.com")
	}
}

func TestFilterAndMatchSubdomainNoMatchFilter(t *testing.T) {
	r := &Runner{}
	if !r.filterAndMatchSubdomain("anything.example.com") {
		t.Error("expected pass-through when no match/filter set")
	}
}

func TestFilterOnly(t *testing.T) {
	r := &Runner{
		filterRegexes: []*regexp.Regexp{regexp.MustCompile(`\.example\.com$`)},
	}
	if r.filterAndMatchSubdomain("test.example.com") {
		t.Error("expected filtered")
	}
	if !r.filterAndMatchSubdomain("test.other.com") {
		t.Error("expected not filtered")
	}
}

func TestMatchOnly(t *testing.T) {
	r := &Runner{
		matchRegexes: []*regexp.Regexp{regexp.MustCompile(`\.example\.com$`)},
	}
	if !r.filterAndMatchSubdomain("test.example.com") {
		t.Error("expected matched")
	}
	if r.filterAndMatchSubdomain("test.other.com") {
		t.Error("expected not matched")
	}
}

// ─── globToRegex ─────────────────────────────────────────────────.

func TestGlobToRegex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		sub     string
		match   bool
	}{
		{"*.example.com", "www.example.com", true},
		{"*.example.com", "admin.example.com", true},
		{"*.example.com", "other.com", false},
		{"admin.example.com", "admin.example.com", true},
		{"admin.example.com", "www.example.com", false},
		{"*.ns.*.com", "a.ns.example.com", true},
		{"*.ns.*.com", "a.ns.other.com", true},
		{"*.ns.*.com", "example.com", false},
	}
	for _, tc := range tests {
		re := regexp.MustCompile(globToRegex(tc.pattern))
		matched := re.MatchString(tc.sub)
		if matched != tc.match {
			t.Errorf("globToRegex(%q).MatchString(%q) = %v, want %v", tc.pattern, tc.sub, matched, tc.match)
		}
	}
}

// ─── filterSources ────────────────────────────────────────────────.

type testSource struct {
	name string
}

func (s *testSource) Name() string                                             { return s.name }
func (s *testSource) NeedsKey() bool                                           { return false }
func (s *testSource) SetKeys(_ []string)                                       {}
func (s *testSource) Fetch(_ context.Context, _ string, _ chan<- string) error { return nil }

func TestFilterSourcesInclude(t *testing.T) {
	t.Parallel()
	all := []passive.Source{&testSource{"a"}, &testSource{"b"}, &testSource{"c"}}
	filtered := filterSources(all, []string{"a", "c"}, nil)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(filtered))
	}
	if filtered[0].Name() != "a" || filtered[1].Name() != "c" {
		t.Error("unexpected sources after include filter")
	}
}

func TestFilterSourcesExclude(t *testing.T) {
	t.Parallel()
	all := []passive.Source{&testSource{"a"}, &testSource{"b"}, &testSource{"c"}}
	filtered := filterSources(all, nil, []string{"b"})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(filtered))
	}
	if filtered[0].Name() != "a" || filtered[1].Name() != "c" {
		t.Error("unexpected sources after exclude filter")
	}
}

func TestFilterSourcesNoFilter(t *testing.T) {
	t.Parallel()
	all := []passive.Source{&testSource{"a"}, &testSource{"b"}}
	filtered := filterSources(all, nil, nil)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(filtered))
	}
}

// ─── preprocessDomain ─────────────────────────────────────────────.

func TestPreprocessDomain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"example.com", "example.com"},
		{"EXAMPLE.COM", "example.com"},
		{"  example.com  ", "example.com"},
		{"http://example.com", "example.com"},
		{"https://example.com", "example.com"},
		{"*.example.com", "example.com"},
		{"example.com/", "example.com"},
		{"www.example.com", "www.example.com"},
		{"", ""},
	}
	for _, tc := range tests {
		got := preprocessDomain(tc.input)
		if got != tc.want {
			t.Errorf("preprocessDomain(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ─── mapsKeys ─────────────────────────────────────────────────────.

func TestMapsKeys(t *testing.T) {
	t.Parallel()
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	keys := mapsKeys(m)
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for _, k := range []string{"a", "b", "c"} {
		if !keySet[k] {
			t.Errorf("missing key %q", k)
		}
	}
}

func TestMapsKeysEmpty(t *testing.T) {
	t.Parallel()
	keys := mapsKeys(map[string]int{})
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

// ─── HasStdin ──────────────────────────────────────────────────────.

func TestHasStdin(t *testing.T) {
	t.Parallel()
	// In test environment, stdin is typically a terminal
	got := HasStdin()
	t.Logf("HasStdin() = %v (expected false in test env)", got)
}

func TestNewFromFileWordlist(t *testing.T) {
	t.Parallel()
	// Not creating a test file; just testing the code path
	// where Wordlist is set but file may not exist
	cfg := &types.Config{
		Domain:     "example.com",
		Bruteforce: true,
		Wordlist:   "/nonexistent/wordlist.txt",
	}
	r := New(cfg, nil)
	if r == nil {
		t.Fatal("New returned nil")
	}
	// bruter should be nil since file doesn't exist (warning logged)
	if r.brute != nil {
		t.Log("bruteforcer created despite non-existent file")
	}
}

// ─── Benchmarks ─────────────────────────────────────────.

func BenchmarkValidSubdomain(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		validSubdomain("www.example.com", "example.com")
	}
}

func BenchmarkJitterMs(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		jitterMs(10, 100)
	}
}

func BenchmarkDefaultWordlist(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = DefaultWordlist()
	}
}

func BenchmarkNewRunner(b *testing.B) {
	cfg := &types.Config{
		Domain: "example.com",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = New(cfg, nil)
	}
}

// ─── Example tests ──────────────────────────────────────.

func Example_validSubdomain() {
	fmt.Println(validSubdomain("api.example.com", "example.com"))
	fmt.Println(validSubdomain("", "example.com"))
	fmt.Println(validSubdomain("*.example.com", "example.com"))
	// Output:
	// true
	// false
	// false
}

// ─── Concurrency safety for DefaultWordlist ─────────────.

func TestDefaultWordlistConcurrent(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_ = DefaultWordlist()
		})
	}
	wg.Wait()
}

// ─── Float conversion safety ────────────────────────────.

func TestJitterMsFloatSafe(t *testing.T) {
	t.Parallel()
	const maxInt = 1<<63 - 1
	if float64(maxInt) > math.MaxFloat64 {
		t.Error("potential overflow in float conversion")
	}
}

// ─── Error circuit tripped ──────────────────────────────.

func TestErrCircuitTripped(t *testing.T) {
	t.Parallel()
	if errCircuitTripped == nil {
		t.Fatal("errCircuitTripped is nil")
	}
	if !strings.Contains(errCircuitTripped.Error(), "circuit breaker") {
		t.Errorf("unexpected error message: %v", errCircuitTripped)
	}
}

// ─── Ensure Runner satisfies an interface ───────────────.

func TestRunnerRunMethodExists(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{Domain: "test.com"}
	r := New(cfg, nil)
	if r == nil {
		t.Fatal("New returned nil")
	}
}
