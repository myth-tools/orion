package passive

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// ─── randomUA ───────────────────────────────────────────.

func TestRandomUANotEmpty(t *testing.T) {
	t.Parallel()
	ua := randomUA()
	if ua == "" {
		t.Error("randomUA() returned empty string")
	}
}

func TestRandomUADeterministicDistribution(t *testing.T) {
	t.Parallel()
	seen := make(map[string]int)
	for range 1000 {
		ua := randomUA()
		seen[ua]++
	}
	if len(seen) < 10 {
		t.Errorf("only got %d unique UAs out of 1000, expected many", len(seen))
	}
}

func TestRandomUAAllValidFormat(t *testing.T) {
	t.Parallel()
	for range 200 {
		ua := randomUA()
		if len(ua) < 10 {
			t.Errorf("UA too short: %q", ua)
		}
	}
}

// ─── parseRetryAfter ────────────────────────────────────.

func TestParseRetryAfterSeconds(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"120"}},
	}
	d := parseRetryAfter(resp)
	// parseRetryAfter caps at 10 seconds
	if d != 10*time.Second {
		t.Errorf("got %v, want 10s (capped)", d)
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	t.Parallel()
	future := time.Now().Add(5 * time.Second).Format(time.RFC1123)
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{future}},
	}
	d := parseRetryAfter(resp)
	if d <= 0 {
		t.Errorf("expected positive duration, got %v", d)
	}
	if d > 10*time.Second {
		t.Errorf("expected <= 10s, got %v", d)
	}
}

func TestParseRetryAfterEmpty(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		Header: http.Header{},
	}
	d := parseRetryAfter(resp)
	if d != 0 {
		t.Errorf("got %v, want 0", d)
	}
}

func TestParseRetryAfterInvalid(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"garbage"}},
	}
	d := parseRetryAfter(resp)
	if d != 0 {
		t.Errorf("got %v, want 0", d)
	}
}

func TestParseRetryAfterZero(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"0"}},
	}
	d := parseRetryAfter(resp)
	if d != 0 {
		t.Errorf("got %v, want 0", d)
	}
}

func TestParseRetryAfterNegative(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"-5"}},
	}
	d := parseRetryAfter(resp)
	if d < 0 {
		t.Errorf("got negative duration: %v", d)
	}
}

// ─── AllSources ─────────────────────────────────────────.

func TestAllSourcesNotEmpty(t *testing.T) {
	t.Parallel()
	sources := AllSources(nil)
	if len(sources) == 0 {
		t.Fatal("AllSources() returned empty slice")
	}
}

func TestAllSourcesUniqueNames(t *testing.T) {
	t.Parallel()
	sources := AllSources(nil)
	names := make(map[string]int)
	for _, s := range sources {
		n := s.Name()
		if n == "" {
			t.Error("source returned empty Name()")
		}
		names[n]++
	}
	for n, count := range names {
		if count > 1 {
			t.Errorf("duplicate source name %q (%d occurrences)", n, count)
		}
	}
}

func TestAllSourcesContextCancellation(t *testing.T) {
	t.Parallel()
	sources := AllSources(nil)
	for _, s := range sources {
		t.Run(s.Name(), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			results := make(chan string, 10)
			err := s.Fetch(ctx, "example.com", results)
			if err == nil {
				t.Logf("source %q: no error on cancelled context", s.Name())
			}
		})
	}
}

func TestAllSourcesClosedChannel(t *testing.T) {
	// NOTE: no t.Parallel() — CommonCrawl panics on send to closed channel.
	// Running concurrently with tests that set sharedClient would cause a panic.
	sources := AllSources(nil)
	for _, s := range sources {
		t.Run(s.Name(), func(t *testing.T) {
			ctx := t.Context()
			results := make(chan string)
			close(results)

			err := s.Fetch(ctx, "example.com", results)
			if err == nil {
				t.Logf("source %q: no error on closed channel", s.Name())
			}
		})
	}
}

// ─── AllSources named ───────────────────────────────────.

func TestAllSourcesContainsExpected(t *testing.T) {
	t.Parallel()
	sources := AllSources(nil)
	names := make(map[string]bool)
	for _, s := range sources {
		names[s.Name()] = true
	}

	expected := []string{
		"crt.sh", "ALIENVAULT_API_KEY", "wayback", "urlscan",
		"anubis", "CERTSPOTTER_API_KEY", "commoncrawl",
		"bing", "baidu", "google", "threatminer",
		"shodanct", "rapiddns",
		"thc", "hudsonrock",
		"submd", "reconeer", "robtex", "hackertarget",
		"VIRUSTOTAL_API_KEY", "FULLHUNT_API_KEY",
		"BUFFEROVER_API_KEY", "LEAKIX_API_KEY", "DNSDUMPSTER_API_KEY", "WHOISXML_API_KEY",
		"INTELX_API_KEY", "CENSYS_API_KEY", "GITHUB_API_KEY",
		"BEVIGIL_API_KEY", "BUILTWITH_API_KEY", "CHAOS_API_KEY",
		"MERKLEMAP_API_KEY", "NETLAS_API_KEY",
		"RSECLOUD_API_KEY", "THREATBOOK_API_KEY", "WINDVANE_API_KEY",
		"ZOOMEYE_API_KEY", "DIGITALYAMA_API_KEY",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing source %q", name)
		}
	}
}

// ─── HTTP Client (context-based) ────────────────────────.

func TestSetSharedClient(t *testing.T) {
	t.Parallel()
	orig := GetSharedClient()
	t.Cleanup(func() { SetSharedClient(orig) })

	c := &http.Client{Timeout: 5 * time.Second}
	SetSharedClient(c)
	if GetSharedClient() != c {
		t.Error("sharedClient not set")
	}
}

func TestWithHTTPClient(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	c := &http.Client{Timeout: 1 * time.Second}
	ctx2 := WithHTTPClient(ctx, c)
	if ctx2 == ctx {
		t.Error("WithHTTPClient returned same context")
	}
}

func TestGetHTTPClientFromContext(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	want := &http.Client{Timeout: 3 * time.Second}
	ctx = WithHTTPClient(ctx, want)
	got := getHTTPClient(ctx)
	if got != want {
		t.Error("getHTTPClient did not return context client")
	}
}

func TestGetHTTPClientFallback(t *testing.T) {
	t.Parallel()
	orig := GetSharedClient()
	t.Cleanup(func() { SetSharedClient(orig) })

	want := &http.Client{Timeout: 7 * time.Second}
	SetSharedClient(want)

	ctx := t.Context()
	got := getHTTPClient(ctx)
	if got != want {
		t.Error("getHTTPClient did not return shared client")
	}
}

func TestGetHTTPClientNil(t *testing.T) {
	t.Parallel()
	orig := GetSharedClient()
	t.Cleanup(func() { SetSharedClient(orig) })

	SetSharedClient(nil)
	ctx := t.Context()
	got := getHTTPClient(ctx)
	if got != nil {
		t.Error("expected nil client")
	}
}

func TestFetchNoClient(t *testing.T) {
	t.Parallel()
	ctx := WithHTTPClient(t.Context(), nil)
	_, err := fetch(ctx, "http://example.com")
	if !errors.Is(err, errSharedClientNotSet) {
		t.Errorf("expected errSharedClientNotSet, got %v", err)
	}
}

// ─── fetchSliceSource ───────────────────────────────────.

func TestFetchSliceSourceNoClient(t *testing.T) {
	t.Parallel()
	ctx := WithHTTPClient(t.Context(), nil)
	results := make(chan string, 10)
	err := fetchSliceSource(ctx, "test", "http://example.com/api", results)
	if !errors.Is(err, errSharedClientNotSet) {
		t.Errorf("expected errSharedClientNotSet, got %v", err)
	}
}

func TestFetchSliceSourceContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	results := make(chan string, 10)
	err := fetchSliceSource(ctx, "test", "http://example.com/api", results)
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

// ─── Source interface conformance ───────────────────────.

func TestAllSourcesImplementSource(t *testing.T) {
	t.Parallel()
	sources := AllSources(nil)
	for _, s := range sources {
		t.Run(s.Name(), func(t *testing.T) {
			t.Parallel()
			if _, ok := any(s).(Source); !ok {
				t.Errorf("%T does not implement Source", s)
			}
			if s.Name() == "" {
				t.Error("source Name() must not be empty")
			}
		})
	}
}

// ─── SourceTestURLs ─────────────────────────────────────.

// noTestURLSources are sources that don't implement TestURL because
// their APIs require POST requests and the doctor only tests via GET.
var noTestURLSources = map[string]bool{
	"thc": true,
}

func TestSourceTestURLsMatchAllSources(t *testing.T) {
	t.Parallel()
	urls := SourceTestURLs()
	sources := AllSources(nil)

	urlNames := make(map[string]bool)
	for _, u := range urls {
		urlNames[u.Name] = true
	}
	for _, s := range sources {
		if noTestURLSources[s.Name()] {
			continue
		}
		if !urlNames[s.Name()] {
			t.Errorf("source %q has no TestURL", s.Name())
		}
	}
}

func TestSourceTestURLsFormat(t *testing.T) {
	t.Parallel()
	urls := SourceTestURLs()
	for _, u := range urls {
		t.Run(u.Name, func(t *testing.T) {
			t.Parallel()
			url := u.URL("example.com")
			if url == "" {
				t.Errorf("TestURL returned empty string")
			}
			if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
				t.Errorf("TestURL %q does not start with http(s)://", url)
			}
			if !strings.Contains(url, "example.com") {
				t.Errorf("TestURL %q does not contain 'example.com'", url)
			}
		})
	}
}

func TestSourceTestURLsUnique(t *testing.T) {
	t.Parallel()
	urls := SourceTestURLs()
	names := make(map[string]int)
	for _, u := range urls {
		names[u.Name]++
	}
	for name, count := range names {
		if count > 1 {
			t.Errorf("duplicate test URL name %q", name)
		}
	}
}

// ─── Context cancellation on fetcher ────────────────────.

func TestFetchContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := fetch(ctx, "http://example.com")
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

// ─── Concurrent safety ──────────────────────────────────.

func TestConcurrentRandomUA(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			ua := randomUA()
			if ua == "" {
				t.Error("empty UA from concurrent call")
			}
		})
	}
	wg.Wait()
}

func TestConcurrentGetHTTPClient(t *testing.T) {
	orig := GetSharedClient()
	t.Cleanup(func() { SetSharedClient(orig) })

	SetSharedClient(&http.Client{Timeout: 1 * time.Second})

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			c := getHTTPClient(t.Context())
			if c == nil {
				t.Error("nil client from concurrent call")
			}
		})
	}
	wg.Wait()
}

// ─── Parse retry-after edge cases ───────────────────────.

func TestParseRetryAfterPastDate(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"Mon, 01 Jan 2000 00:00:00 GMT"}},
	}
	d := parseRetryAfter(resp)
	if d < 0 {
		t.Errorf("negative duration for past date: %v", d)
	}
}

func TestParseRetryAfterLargeValue(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"999999999"}},
	}
	d := parseRetryAfter(resp)
	if d <= 0 {
		t.Errorf("expected positive duration, got %v", d)
	}
}

// ─── fetch retry behavior ───────────────────────────────.

var errConnectionRefused = errors.New("connection refused")

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

type errorTransport struct {
	err error
}

func (t *errorTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, t.err
}

func TestFetchNetworkError(t *testing.T) {
	t.Parallel()
	client := &http.Client{
		Transport: &errorTransport{err: errConnectionRefused},
		Timeout:   5 * time.Second,
	}
	ctx := WithHTTPClient(t.Context(), client)

	body, err := fetch(ctx, "http://example.com/api")
	if err == nil {
		t.Error("expected error for network failure")
	}
	if body != nil {
		t.Errorf("expected nil body, got %d bytes", len(body))
	}
}

func TestFetchStatusRetryBehavior(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCount  int
	}{
		{
			name:       "400 no retry",
			statusCode: http.StatusBadRequest,
			body:       `{"error":"bad request"}`,
			wantCount:  1,
		},
		{
			name:       "500 retry",
			statusCode: http.StatusInternalServerError,
			body:       "server error",
			wantCount:  4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			count := 0
			transport := roundTripFunc(func(_ *http.Request) *http.Response {
				count++
				return &http.Response{
					StatusCode: tt.statusCode,
					Body:       io.NopCloser(strings.NewReader(tt.body)),
					Header:     make(http.Header),
				}
			})
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			ctx := WithHTTPClient(t.Context(), client)
			_, err := fetch(ctx, "http://example.com/api")
			if err == nil {
				t.Error("expected error")
			}
			if count != tt.wantCount {
				t.Errorf("expected %d attempts, got %d", tt.wantCount, count)
			}
		})
	}
}

func TestFetch429WithRetryAfter(t *testing.T) {
	t.Parallel()
	count := 0
	transport := roundTripFunc(func(_ *http.Request) *http.Response {
		count++
		header := make(http.Header)
		header.Set("Retry-After", "0")
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(strings.NewReader("rate limited")),
			Header:     header,
		}
	})

	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	ctx := WithHTTPClient(t.Context(), client)

	_, err := fetch(ctx, "http://example.com/api")
	if err == nil {
		t.Error("expected error for persistent 429")
	}
	if count != 4 {
		t.Errorf("expected 4 attempts, got %d", count)
	}
}

func TestFetch429WithoutRetryAfter(t *testing.T) {
	t.Parallel()
	count := 0
	transport := roundTripFunc(func(_ *http.Request) *http.Response {
		count++
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(strings.NewReader("rate limited")),
			Header:     make(http.Header),
		}
	})

	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	ctx := WithHTTPClient(t.Context(), client)

	_, err := fetch(ctx, "http://example.com/api")
	if err == nil {
		t.Error("expected error for persistent 429")
	}
}

func TestFetchSuccess(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(_ *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`["sub1.example.com","sub2.example.com"]`)),
			Header:     make(http.Header),
		}
	})

	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	ctx := WithHTTPClient(t.Context(), client)

	body, err := fetch(ctx, "http://example.com/api")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(body) == 0 {
		t.Error("empty body")
	}
}

func TestFetchContextCancelledMidRetry(t *testing.T) {
	t.Parallel()
	slowTransport := roundTripFunc(func(_ *http.Request) *http.Response {
		time.Sleep(100 * time.Millisecond)
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(strings.NewReader("rate limited")),
			Header:     make(http.Header),
		}
	})

	client := &http.Client{Transport: slowTransport, Timeout: 5 * time.Second}
	ctx := WithHTTPClient(t.Context(), client)

	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	_, err := fetch(ctx, "http://example.com/api")
	if err == nil {
		t.Error("expected error for timeout")
	}
}

// ─── Benchmark ──────────────────────────────────────────.

func BenchmarkRandomUA(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		randomUA()
	}
}

func BenchmarkAllSources(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		_ = AllSources(nil)
	}
}

func BenchmarkFetchSuccess(b *testing.B) {
	transport := roundTripFunc(func(_ *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`["a.example.com","b.example.com"]`)),
			Header:     make(http.Header),
		}
	})
	ctx := WithHTTPClient(b.Context(), &http.Client{Transport: transport, Timeout: 5 * time.Second})

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = fetch(ctx, "http://example.com/api")
	}
}

func BenchmarkParseRetryAfter(b *testing.B) {
	resp := &http.Response{
		Header: http.Header{"Retry-After": []string{"120"}},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		parseRetryAfter(resp)
	}
}
