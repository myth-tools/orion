package passive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	errSharedClientNotSet = errors.New("passive: shared HTTP client not set (call SetSharedClient)")
	errRateLimited        = errors.New("rate limited (429)")
	errHTTPStatus         = errors.New("unexpected HTTP status")
)

type sourceContextKey string

const (
	httpClientKey  sourceContextKey = "http-client"
	rateLimiterKey sourceContextKey = "rate-limiter"
	sourceNameKey  sourceContextKey = "source-name"
)

type Source interface {
	Name() string
	Fetch(ctx context.Context, domain string, results chan<- string) error
	NeedsKey() bool
	SetKeys(keys []string)
}

var (
	sharedClient   *http.Client //nolint:gochecknoglobals
	sharedClientMu sync.RWMutex //nolint:gochecknoglobals
)

func SetSharedClient(c *http.Client) {
	sharedClientMu.Lock()
	sharedClient = c
	sharedClientMu.Unlock()
}

func GetSharedClient() *http.Client {
	sharedClientMu.RLock()
	c := sharedClient
	sharedClientMu.RUnlock()
	return c
}

func WithHTTPClient(ctx context.Context, client *http.Client) context.Context {
	return context.WithValue(ctx, httpClientKey, client)
}

func getHTTPClient(ctx context.Context) *http.Client {
	if c, ok := ctx.Value(httpClientKey).(*http.Client); ok {
		return c
	}
	sharedClientMu.RLock()
	c := sharedClient
	sharedClientMu.RUnlock()
	return c
}

func WithRateLimiter(ctx context.Context, rl *MultiLimiter) context.Context {
	return context.WithValue(ctx, rateLimiterKey, rl)
}

func WithSourceName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, sourceNameKey, name)
}

func getRateLimiter(ctx context.Context) *MultiLimiter {
	if rl, ok := ctx.Value(rateLimiterKey).(*MultiLimiter); ok {
		return rl
	}
	return nil
}

func getSourceName(ctx context.Context) string {
	if name, ok := ctx.Value(sourceNameKey).(string); ok {
		return name
	}
	return ""
}

func takeRateLimit(ctx context.Context) {
	if rl := getRateLimiter(ctx); rl != nil {
		rl.Take(getSourceName(ctx))
	}
}

var userAgents = []string{ //nolint:gochecknoglobals
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:122.0) Gecko/20100101 Firefox/122.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:120.0) Gecko/20100101 Firefox/120.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:122.0) Gecko/20100101 Firefox/122.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:122.0) Gecko/20100101 Firefox/122.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.230 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.6167.143 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 14; Samsung Galaxy S24) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.6261.64 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.230 Mobile Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 Edg/121.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/122.0.0.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 OPR/107.0.0.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Vivaldi/6.4",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 Vivaldi/6.5",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Vivaldi/6.4",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Brave/120.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Brave/120.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Brave/120.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:115.0) Gecko/20100101 Firefox/115.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:115.0) Gecko/20100101 Firefox/115.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.2 Safari/605.1.15",
}

func randomUA() string {
	return userAgents[rand.Intn(len(userAgents))]
}

func parseRetryAfter(resp *http.Response) time.Duration {
	h := resp.Header.Get("Retry-After")
	if h == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(h); err == nil {
		if seconds <= 0 {
			return 0
		}
		d := min(time.Duration(seconds)*time.Second, 10*time.Second)
		return d
	}
	if t, err := time.Parse(time.RFC1123, h); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		if d > 10*time.Second {
			d = 10 * time.Second
		}
		return d
	}
	return 0
}

//nolint:gocognit
func fetch(ctx context.Context, url string) ([]byte, error) {
	client := getHTTPClient(ctx)
	if client == nil {
		return nil, errSharedClientNotSet
	}

	var lastErr error
	backoff := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt := range 4 {
		if attempt > 0 {
			idx := attempt - 1
			if idx >= len(backoff) {
				idx = len(backoff) - 1
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff[idx]):
			}
		}

		takeRateLimit(ctx)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("request creation failed: %w", err)
		}

		req.Header.Set("User-Agent", randomUA())
		req.Header.Set("Accept", "application/json,text/html,*/*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			delay := parseRetryAfter(resp)
			_ = resp.Body.Close()
			lastErr = errRateLimited
			if attempt+1 < 4 && delay > 0 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if len(body) > 200 {
				body = body[:200]
			}
			lastErr = fmt.Errorf("%w: %d %s", errHTTPStatus, resp.StatusCode, strings.TrimSpace(string(body)))
			if resp.StatusCode < 500 {
				return nil, lastErr
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read failed: %w", err)
			continue
		}

		return body, nil
	}

	return nil, lastErr
}

func fetchSliceSource(ctx context.Context, name, url string, results chan<- string) error {
	body, err := fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	var subs []string
	if err := json.Unmarshal(body, &subs); err != nil {
		return fmt.Errorf("%s parse: %w", name, err)
	}
	for _, sub := range subs {
		sub = strings.TrimSpace(sub)
		if sub == "" {
			continue
		}
		select {
		case results <- sub:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func AllSources(providers *Providers) []Source {
	sources := []Source{
		&CRTSH{},
		&Wayback{},
		&URLScan{},
		&Anubis{},
		&CommonCrawl{},
		&Bing{},
		&Baidu{},
		&Google{},
		&ThreatMiner{},
		&ShodanCT{},
		&RapidDNS{},
		&THC{},
		&HudsonRock{},
		&Submd{},
		&Reconeer{},
		&Robtex{},
		&Hackertarget{},
	}

	keyedSources := []Source{
		&AlienVault{},
		&CertSpotter{},
		&VirusTotal{},
		&FullHunt{},
		&BufferOver{},
		&LeakIX{},
		&DNSDumpster{},
		&WhoisXMLAPI{},
		&IntelX{},
		&Censys{},
		&GitHub{},
		&Bevigil{},
		&BuiltWith{},
		&Chaos{},
		&MerkleMap{},
		&Netlas{},
		&RSECloud{},
		&ThreatBook{},
		&WindVane{},
		&ZoomEyeAPI{},
		&DigitalYama{},
	}

	for _, s := range keyedSources {
		if providers != nil {
			s.SetKeys(GetProviderKeys(providers, s.Name()))
		}
		sources = append(sources, s)
	}

	return sources
}

var providerKeysMap = map[string]func(*Providers) []string{ //nolint:gochecknoglobals
	"VIRUSTOTAL_API_KEY":  func(p *Providers) []string { return p.VirusTotal },
	"FULLHUNT_API_KEY":    func(p *Providers) []string { return p.FullHunt },
	"BUFFEROVER_API_KEY":  func(p *Providers) []string { return p.BufferOver },
	"LEAKIX_API_KEY":      func(p *Providers) []string { return p.LeakIX },
	"WHOISXML_API_KEY":    func(p *Providers) []string { return p.WhoisXMLAPI },
	"INTELX_API_KEY":      func(p *Providers) []string { return p.IntelX },
	"CENSYS_API_KEY":      func(p *Providers) []string { return p.Censys },
	"GITHUB_API_KEY":      func(p *Providers) []string { return p.GitHub },
	"ALIENVAULT_API_KEY":  func(p *Providers) []string { return p.AlienVault },
	"URLSCAN_API_KEY":     func(p *Providers) []string { return p.URLScan },
	"CERTSPOTTER_API_KEY": func(p *Providers) []string { return p.CertSpotter },

	"BEVIGIL_API_KEY":     func(p *Providers) []string { return p.Bevigil },
	"BUILTWITH_API_KEY":   func(p *Providers) []string { return p.BuiltWith },
	"CHAOS_API_KEY":       func(p *Providers) []string { return p.Chaos },
	"DNSDUMPSTER_API_KEY": func(p *Providers) []string { return p.DNSDumpster },
	"MERKLEMAP_API_KEY":   func(p *Providers) []string { return p.MerkleMap },
	"NETLAS_API_KEY":      func(p *Providers) []string { return p.Netlas },

	"RSECLOUD_API_KEY":    func(p *Providers) []string { return p.Rsecloud },
	"THREATBOOK_API_KEY":  func(p *Providers) []string { return p.ThreatBook },
	"WINDVANE_API_KEY":    func(p *Providers) []string { return p.WindVane },
	"ZOOMEYE_API_KEY":     func(p *Providers) []string { return p.ZoomEyeAPI },
	"DIGITALYAMA_API_KEY": func(p *Providers) []string { return p.DigitalYama },
}

func GetProviderKeys(providers *Providers, name string) []string {
	if providers == nil {
		return nil
	}
	if fn, ok := providerKeysMap[name]; ok {
		return fn(providers)
	}
	return nil
}

func fetchWithHeader(ctx context.Context, url, headerKey, headerValue string) ([]byte, error) {
	client := getHTTPClient(ctx)
	if client == nil {
		return nil, errSharedClientNotSet
	}

	takeRateLimit(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %w", err)
	}

	req.Header.Set("User-Agent", randomUA())
	req.Header.Set("Accept", "application/json,*/*")
	req.Header.Set(headerKey, headerValue)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", errHTTPStatus, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

type fetchWithHeadersResult struct {
	body  []byte
	err   error
	retry bool
	delay time.Duration
}

func fetchWithHeadersAttempt(
	ctx context.Context, client *http.Client,
	url, method string, headers map[string]string,
	body io.Reader,
) fetchWithHeadersResult {
	takeRateLimit(ctx)

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fetchWithHeadersResult{err: fmt.Errorf("request creation failed: %w", err)}
	}

	req.Header.Set("User-Agent", randomUA())
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fetchWithHeadersResult{err: fmt.Errorf("request failed: %w", err), retry: true}
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		delay := parseRetryAfter(resp)
		resp.Body.Close()
		return fetchWithHeadersResult{err: errRateLimited, retry: true, delay: delay}
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		httpErr := fmt.Errorf("%w: %d", errHTTPStatus, resp.StatusCode)
		return fetchWithHeadersResult{err: httpErr, retry: resp.StatusCode >= 500}
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fetchWithHeadersResult{err: fmt.Errorf("read failed: %w", err), retry: true}
	}

	return fetchWithHeadersResult{body: bodyBytes}
}

func fetchWithHeaders(
	ctx context.Context, url, method string,
	headers map[string]string, body io.Reader,
) ([]byte, error) {
	client := getHTTPClient(ctx)
	if client == nil {
		return nil, errSharedClientNotSet
	}

	backoff := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second, 4 * time.Second}

	var lastErr error
	for attempt := range 4 {
		if attempt > 0 {
			idx := min(attempt-1, len(backoff)-1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff[idx]):
			}
		}

		res := fetchWithHeadersAttempt(ctx, client, url, method, headers, body)
		if res.err == nil {
			return res.body, nil
		}
		if !res.retry {
			return nil, res.err
		}
		lastErr = res.err
		if attempt+1 < 4 && res.delay > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(res.delay):
			}
		}
	}

	return nil, lastErr
}
