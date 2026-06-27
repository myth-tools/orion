package doctor

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/myth-tools/orion/internal/active"
	"github.com/myth-tools/orion/internal/dns"
	"github.com/myth-tools/orion/internal/passive"
	"github.com/myth-tools/orion/internal/proxy"
	"github.com/myth-tools/orion/internal/runner"
	"github.com/myth-tools/orion/internal/styler"
	"github.com/myth-tools/orion/internal/types"
)

// ─── Types ───────────────────────────────────────────.

type Status int

const (
	StatusPass Status = iota
	StatusWarn
	StatusFail
)

const (
	statusPass = "PASS"
	statusWarn = "WARN"
	statusFail = "FAIL"
)

var errUnknownStatus = errors.New("unknown status")

func (s Status) String() string {
	switch s {
	case StatusPass:
		return statusPass
	case StatusWarn:
		return statusWarn
	case StatusFail:
		return statusFail
	}
	return "UNKN"
}

func (s Status) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *Status) UnmarshalText(text []byte) error {
	switch string(text) {
	case statusPass:
		*s = StatusPass
	case statusWarn:
		*s = StatusWarn
	case statusFail:
		*s = StatusFail
	default:
		return fmt.Errorf("%w: %q", errUnknownStatus, string(text))
	}
	return nil
}

type Result struct {
	Section string        `json:"section"`
	Name    string        `json:"name"`
	Status  Status        `json:"status"`
	Message string        `json:"message"`
	Detail  string        `json:"detail,omitempty"`
	Latency time.Duration `json:"latency,omitempty"`
}

type Options struct {
	Verbose  bool
	JSON     bool
	OnResult func(Result)
}

const testDomain = "yourchoose.top"

// ─── Run ────────────────────────────────────────────.

func Run(ctx context.Context, cfg *types.Config, opts Options) []Result {
	httpClient := &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxConnsPerHost:     50,
			MaxIdleConnsPerHost: 50,
			IdleConnTimeout:     10 * time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			TLSHandshakeTimeout: 3 * time.Second,
			ForceAttemptHTTP2:   true,
			DisableCompression:  false,
			WriteBufferSize:     8192,
			ReadBufferSize:      8192,
		},
	}

	all := make([]Result, 0, 120)

	emit := func(rr []Result) {
		for _, r := range rr {
			if opts.OnResult != nil {
				opts.OnResult(r)
			}
			all = append(all, r)
		}
	}

	proxyClient := httpClient
	if cfg != nil && cfg.ProxyURL != "" {
		dialer, err := active.NewProxyDialer(cfg.ProxyURL)
		if err == nil && !dialer.Tripped() {
			proxyClient = active.NewProxiedHTTPClient(dialer, 20)
		}
	}

	emit(checkSystem())
	emit(checkGoVersion())
	emit(checkGoMod())
	emit(checkDependencies())
	emit(checkBuild())
	emit(checkPermissions())
	emit(checkExternalTools())
	emit(checkWordlist())

	providerConfig := ""
	var proxyURL string
	if cfg != nil {
		providerConfig = cfg.ProviderConfig
		proxyURL = cfg.ProxyURL
	}
	all = runChecksConcurrently(ctx, httpClient, proxyClient, opts, all, providerConfig, proxyURL)

	if cfg != nil {
		emit(checkConfig(cfg))
	}

	return all
}

func runChecksConcurrently(
	ctx context.Context,
	httpClient, proxyClient *http.Client,
	opts Options, all []Result,
	providerConfig, proxyURL string,
) []Result {
	var mu sync.Mutex
	var wg sync.WaitGroup

	bufOpts := opts
	var collected []Result
	bufOpts.OnResult = func(r Result) {
		mu.Lock()
		collected = append(collected, r)
		mu.Unlock()
	}

	launch := func(fn func()) {
		wg.Go(fn)
	}

	launch(func() { checkPassiveSources(ctx, httpClient, bufOpts, providerConfig) })
	launch(func() { checkDoHEndpoints(ctx, httpClient, bufOpts) })
	launch(func() { checkNetwork(ctx, httpClient, bufOpts) })
	launch(func() { checkDNSBenchmark(ctx, httpClient, bufOpts) })
	launch(func() { checkProxyPool(ctx, bufOpts) })

	if proxyURL != "" {
		launch(func() {
			r := checkProxy(ctx, proxyURL)
			bufOpts.OnResult(r)
		})
		if proxyClient != httpClient {
			launch(func() {
				for _, r := range checkPassiveSourcesViaProxy(ctx, proxyClient, bufOpts) {
					bufOpts.OnResult(r)
				}
			})
			launch(func() {
				for _, r := range checkDoHEndpointsViaProxy(ctx, proxyClient, bufOpts) {
					bufOpts.OnResult(r)
				}
			})
			launch(func() {
				for _, r := range checkNetworkViaProxy(ctx, proxyClient, bufOpts) {
					bufOpts.OnResult(r)
				}
			})
		}
	}

	wg.Wait()

	sectionOrder := []string{"Passive Sources", "DoH Resolvers", "Network", "DNS Benchmark", "Proxy Pool", "Proxy"}
	for _, section := range sectionOrder {
		for _, r := range collected {
			if r.Section == section {
				if opts.OnResult != nil {
					opts.OnResult(r)
				}
				all = append(all, r)
			}
		}
	}

	return all
}

func checkPassiveSourcesViaProxy(ctx context.Context, client *http.Client, opts Options) []Result {
	sourceURLs := passive.SourceTestURLs()
	count := min(3, len(sourceURLs))
	results := make([]Result, 0, count)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := range count {
		wg.Add(1)
		go func(si passive.SourceTestURL, idx int) {
			defer wg.Done()
			if idx > 0 {
				time.Sleep(time.Duration(idx) * 200 * time.Millisecond)
			}
			r := checkPassiveURL(ctx, client, si.Name+" (via proxy)", si.URL(testDomain), opts)
			r.Section = "Proxy"
			if opts.OnResult != nil {
				opts.OnResult(r)
			}
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(sourceURLs[i], i)
	}
	wg.Wait()
	return results
}

func checkDoHEndpointsViaProxy(ctx context.Context, client *http.Client, opts Options) []Result {
	checks := []struct {
		Name string
		URL  string
	}{
		{"cloudflare-dns.com (via proxy)", "https://cloudflare-dns.com/dns-query"},
		{"dns.google (via proxy)", "https://dns.google/dns-query"},
	}
	var mu sync.Mutex
	results := make([]Result, 0, len(checks))
	var wg sync.WaitGroup

	for _, ep := range checks {
		wg.Add(1)
		go func(name, url string) {
			defer wg.Done()
			r := Result{Section: "Proxy", Name: name}
			res := dns.NewResolver(client, 3*time.Second)
			res.SetProviders([]dns.Provider{
				{Name: name, URL: url, Method: http.MethodPost},
			})
			subCtx, subCancel := context.WithTimeout(ctx, 4*time.Second)
			defer subCancel()
			start := time.Now()
			ips, err := res.LookupA(subCtx, testDomain)
			r.Latency = time.Since(start)
			if err != nil {
				r.Status = StatusFail
				r.Message = fmt.Sprintf("lookup fail (%s)", r.Latency.Round(time.Millisecond))
				if opts.Verbose {
					r.Detail = err.Error()
				}
			} else {
				r.Status = StatusPass
				r.Message = fmt.Sprintf("%s, %d answers", r.Latency.Round(time.Millisecond), len(ips))
			}
			if opts.OnResult != nil {
				opts.OnResult(r)
			}
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(ep.Name, ep.URL)
	}
	wg.Wait()
	return results
}

func checkNetworkViaProxy(ctx context.Context, client *http.Client, _ Options) []Result {
	r := Result{Section: "Proxy", Name: "Internet (via proxy)"}
	subCtx, subCancel := context.WithTimeout(ctx, 5*time.Second)
	defer subCancel()
	res := dns.NewResolver(client, 4*time.Second)
	res.SetProviders([]dns.Provider{
		{Name: "cloudflare", URL: "https://cloudflare-dns.com/dns-query", Method: http.MethodPost},
	})
	start := time.Now()
	_, err := res.LookupA(subCtx, testDomain)
	r.Latency = time.Since(start)
	if err != nil {
		r.Status = StatusFail
		r.Message = fmt.Sprintf("not reachable via proxy (%s)", r.Latency.Round(time.Millisecond))
		r.Detail = err.Error()
	} else {
		r.Status = StatusPass
		r.Message = fmt.Sprintf("reachable via proxy (%s)", r.Latency.Round(time.Millisecond))
	}
	return []Result{r}
}

// ─── System ─────────────────────────────────────────.

func checkSystem() []Result {
	var res []Result

	r := Result{Section: "System", Name: "Platform"}
	r.Status = StatusPass
	r.Message = fmt.Sprintf("%s/%s, %d CPU cores", runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	res = append(res, r)

	r2 := Result{Section: "System", Name: "Go runtime"}
	r2.Status = StatusPass
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	ng := runtime.NumGoroutine()
	gpl := "goroutines"
	if ng == 1 {
		gpl = "goroutine"
	}
	heapMB := m.HeapAlloc / 1024 / 1024
	r2.Message = fmt.Sprintf(
		"v%s, %d %s, %d MB heap",
		strings.TrimPrefix(runtime.Version(), "go"),
		ng, gpl, heapMB,
	)
	res = append(res, r2)

	return res
}

func parseGoVersion(v string) [3]int {
	var res [3]int
	v = strings.TrimPrefix(v, "go")
	parts := strings.SplitN(v, ".", 3)
	for i := range min(len(parts), 3) {
		n, _ := strconv.Atoi(parts[i])
		res[i] = n
	}
	return res
}

func versionAtLeast(cur, minVer [3]int) bool {
	for i := range 3 {
		if cur[i] < minVer[i] {
			return false
		}
		if cur[i] > minVer[i] {
			return true
		}
	}
	return true
}

// ─── Go Environment ─────────────────────────────────.

// lookPathGo finds the Go binary, trying platform-appropriate names.
func lookPathGo() (string, error) {
	if p, err := exec.LookPath("go"); err == nil {
		return p, nil
	}
	return exec.LookPath("go.exe")
}

func checkGoVersion() []Result {
	goBin, err := lookPathGo()
	if err != nil {
		return []Result{{
			Section: "Go Environment", Name: "Go version",
			Status: StatusFail, Message: fmt.Sprintf("not found: %v", err),
		}}
	}
	out, err := exec.Command(goBin, "version").Output()
	if err != nil {
		return []Result{{
			Section: "Go Environment", Name: "Go version",
			Status: StatusFail, Message: fmt.Sprintf("not found: %v", err),
		}}
	}
	ver := strings.TrimSpace(string(out))
	ok := false
	cur := parseGoVersion(runtime.Version())
	for _, m := range []string{"go1.21", "go1.22", "go1.23", "go1.24", "go1.25", "go1.26"} {
		if versionAtLeast(cur, parseGoVersion(m)) {
			ok = true
			break
		}
	}
	if !ok {
		return []Result{{
			Section: "Go Environment", Name: "Go version",
			Status: StatusWarn, Message: fmt.Sprintf("%s (min recommended: go1.21)", ver),
		}}
	}
	return []Result{{
		Section: "Go Environment", Name: "Go version",
		Status: StatusPass, Message: ver,
	}}
}

func checkGoMod() []Result {
	if p := findGoMod(); p != "" {
		return []Result{{
			Section: "Go Environment", Name: "go.mod",
			Status: StatusPass, Message: "valid (" + p + ")",
		}}
	}
	return []Result{{
		Section: "Go Environment", Name: "go.mod",
		Status: StatusFail, Message: "not found",
	}}
}

func findGoMod() string {
	dir, _ := os.Getwd()
	for {
		path := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Join(dir, "..")
		abs, err := filepath.Abs(parent)
		if err != nil || abs == dir {
			return ""
		}
		dir = abs
	}
}

func goCommand(args ...string) *exec.Cmd {
	goBin, _ := lookPathGo()
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, args...)
	if modDir := filepath.Dir(findGoMod()); modDir != "." {
		cmd.Dir = modDir
	}
	return cmd
}

func checkDependencies() []Result {
	res := Result{Section: "Go Environment", Name: "Dependencies"}

	modDir := filepath.Dir(findGoMod())

	if _, err := os.Stat(filepath.Join(modDir, "go.sum")); errors.Is(err, os.ErrNotExist) {
		res.Status = StatusWarn
		res.Message = "go.sum not found (run go mod tidy)"
		return []Result{res}
	}

	out, err := goCommand("mod", "verify").CombinedOutput()
	if err != nil {
		res.Status = StatusWarn
		res.Message = fmt.Sprintf("verify: %s", strings.TrimSpace(string(out)))
		return []Result{res}
	}
	res.Status = StatusPass
	res.Message = "all modules verified"
	return []Result{res}
}

func checkBuild() []Result {
	res := Result{Section: "Go Environment", Name: "Build"}

	out, err := goCommand("build", "./...").CombinedOutput()
	if err != nil {
		detail := truncateStr(string(out), 400)
		res.Status = StatusFail
		res.Message = "compilation failed"
		res.Detail = detail
		return []Result{res}
	}
	res.Status = StatusPass
	res.Message = "compiles successfully"
	return []Result{res}
}

// ─── Permissions ────────────────────────────────────.

func checkPermissions() []Result {
	var res []Result

	r := Result{Section: "Permissions", Name: "Working directory"}
	f, err := os.CreateTemp(".", ".doctor-write-test-*")
	if err != nil {
		r.Status = StatusFail
		r.Message = fmt.Sprintf("cannot write to current dir: %v", err)
	} else {
		p := f.Name()
		_ = f.Close()
		_ = os.Remove(p)
		r.Status = StatusPass
		r.Message = "writable"
	}
	res = append(res, r)

	r2 := Result{Section: "Permissions", Name: "Temp directory"}
	tmp := os.TempDir()
	f2, err := os.CreateTemp(tmp, ".doctor-temp-test-*")
	if err != nil {
		r2.Status = StatusFail
		r2.Message = fmt.Sprintf("cannot write to %s: %v", tmp, err)
	} else {
		p := f2.Name()
		_ = f2.Close()
		_ = os.Remove(p)
		r2.Status = StatusPass
		r2.Message = fmt.Sprintf("writable (%s)", tmp)
	}
	res = append(res, r2)

	r3 := Result{Section: "Permissions", Name: "File handles"}
	limStr := "unlimited"
	if _, shErr := exec.LookPath("sh"); shErr == nil && runtime.GOOS != "windows" {
		out, _ := exec.Command("sh", "-c", "ulimit -n 2>/dev/null || echo unlimited").Output()
		limStr = strings.TrimSpace(string(out))
	}
	r3.Status = StatusPass
	r3.Message = fmt.Sprintf("file handle limit: %s", limStr)
	res = append(res, r3)

	return res
}

// ─── External Tools ─────────────────────────────────.

var devTools = []struct { //nolint:gochecknoglobals
	Name   string
	Pkg    string
	Binary string
}{
	{"gofumpt", "mvdan.cc/gofumpt", "gofumpt"},
	{"staticcheck", "honnef.co/go/tools/cmd/staticcheck", "staticcheck"},
	{"gosec", "github.com/securego/gosec/v2/cmd/gosec", "gosec"},
	{"golangci-lint", "github.com/golangci/golangci-lint/cmd/golangci-lint", "golangci-lint"},
	{"shadow", "golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow", "shadow"},
}

func checkExternalTools() []Result {
	homeDir, _ := os.UserHomeDir()
	goBin := filepath.Join(homeDir, "go", "bin")
	if _, err := os.Stat(goBin); err == nil { // #nosec G703
		cur := os.Getenv("PATH")
		if !strings.Contains(cur, goBin) {
			_ = os.Setenv("PATH", cur+string(os.PathListSeparator)+goBin)
		}
	}

	res := make([]Result, 0, len(devTools))
	for _, tool := range devTools {
		r := Result{Section: "External Tools", Name: tool.Name}
		path, err := exec.LookPath(tool.Binary)
		if err != nil {
			r.Status = StatusWarn
			r.Message = fmt.Sprintf("not installed (%s)", tool.Pkg)
			res = append(res, r)
			continue
		}
		r.Status = StatusPass
		r.Message = "installed"

		ver := toolVersion(tool.Binary, path)
		if ver != "" {
			r.Message = ver
		}
		res = append(res, r)
	}
	return res
}

func toolVersion(binary, path string) string {
	cmd := exec.Command(binary, "version")
	out, _ := cmd.Output()
	ver := strings.TrimSpace(string(out))
	if ver == "" {
		cmd2 := exec.Command(binary, "--version")
		out2, _ := cmd2.Output()
		ver = strings.TrimSpace(string(out2))
	}
	if idx := strings.IndexByte(ver, '\n'); idx != -1 {
		ver = ver[:idx]
	}
	if ver == "" {
		// #nosec G204
		cmd3 := goCommand("version", "-m", path)
		out3, _ := cmd3.Output()
		for line := range strings.SplitSeq(string(out3), "\n") {
			if strings.Contains(line, "mod") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					ver = parts[len(parts)-1]
					break
				}
			}
		}
	}
	if len(ver) > 60 {
		ver = ver[:60] + "..."
	}
	return ver
}

func newSystemDNSClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DisableKeepAlives:   true,
			ForceAttemptHTTP2:   false,
			TLSHandshakeTimeout: 5 * time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}
}

// ─── Passive Sources (parallel) ────────────────────.

func checkPassiveSources(ctx context.Context, _ *http.Client, opts Options, providerConfig string) []Result {
	client := newSystemDNSClient(60 * time.Second)
	sourceURLs := passive.SourceTestURLs()

	providers, _ := passive.LoadProviders(providerConfig)
	allSources := passive.AllSources(providers)

	sourceMap := make(map[string]passive.Source, len(allSources))
	hasKeys := make(map[string]bool)
	for _, s := range allSources {
		sourceMap[s.Name()] = s
		if s.NeedsKey() && len(passive.GetProviderKeys(providers, s.Name())) > 0 {
			hasKeys[s.Name()] = true
		}
	}

	var mu sync.Mutex
	results := make([]Result, 0, len(sourceURLs))
	var wg sync.WaitGroup

	for i, si := range sourceURLs {
		wg.Add(1)
		go func(si passive.SourceTestURL, idx int) {
			defer wg.Done()
			if idx > 0 {
				time.Sleep(time.Duration(idx) * 200 * time.Millisecond)
			}

			var r Result
			if hasKeys[si.Name] {
				r = testKeyedSource(ctx, client, si.Name, sourceMap, providers)
			} else {
				r = checkPassiveURL(ctx, client, si.Name, si.URL(testDomain), opts)
			}

			if opts.OnResult != nil {
				opts.OnResult(r)
			}
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(si, i)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	pass, warn, fail := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case StatusPass:
			pass++
		case StatusWarn:
			warn++
		case StatusFail:
			fail++
		}
	}
	r := Result{
		Section: "Passive Sources",
		Name:    "Sources summary",
		Status:  StatusPass,
		Message: fmt.Sprintf("%d total — %d pass, %d warn, %d fail", len(results), pass, warn, fail),
	}
	if fail > 0 {
		r.Status = StatusWarn
	}
	if opts.OnResult != nil {
		opts.OnResult(r)
	}
	results = append(results, r)

	return results
}

//nolint:funlen,gocognit,gocyclo,cyclop
func checkPassiveURL(ctx context.Context, client *http.Client, name, url string, opts Options) Result {
	r := Result{Section: "Passive Sources", Name: name}

	ctx2, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	start := time.Now()

	backoff := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}
	const maxAttempts = 4
	for attempt := range maxAttempts {
		if attempt > 0 {
			idx := attempt - 1
			if idx >= len(backoff) {
				idx = len(backoff) - 1
			}
			select {
			case <-ctx2.Done():
				r.Latency = time.Since(start)
				r.Status = StatusWarn
				r.Message = fmt.Sprintf("unreachable (%s)", r.Latency.Round(time.Millisecond))
				return r
			case <-time.After(backoff[idx]):
			}
		}

		req, err := http.NewRequestWithContext(ctx2, http.MethodGet, url, nil)
		if err != nil {
			r.Status = StatusFail
			r.Message = "bad request"
			r.Detail = err.Error()
			return r
		}
		req.Header.Set("User-Agent",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json,text/html,*/*")

		resp, err := client.Do(req)
		r.Latency = time.Since(start)

		if err != nil {
			if attempt+1 < maxAttempts {
				continue
			}
			r.Status = StatusWarn
			r.Message = fmt.Sprintf("unreachable (%s)", r.Latency.Round(time.Millisecond))
			if opts.Verbose {
				r.Detail = err.Error()
			}
			return r
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		bodyStr := strings.TrimSpace(string(body))

		results := countResults(name, bodyStr)
		if results < 0 {
			r.Status = StatusWarn
			r.Message = fmt.Sprintf("%s: %s", r.Latency.Round(time.Millisecond),
				"API limit exceeded (upgrade or wait)")
			return r
		}

		switch {
		case resp.StatusCode == http.StatusOK && len(bodyStr) > 0:
			r.Status = StatusPass
			r.Message = fmt.Sprintf("%s, %d bytes, %d results", r.Latency.Round(time.Millisecond), len(bodyStr), results)
			if opts.Verbose {
				r.Detail = fmt.Sprintf("HTTP 200, first 200 bytes:\n%s", truncateStr(bodyStr, 200))
			}
			return r
		case resp.StatusCode == http.StatusOK:
			r.Status = StatusPass
			r.Message = fmt.Sprintf("%s, empty body", r.Latency.Round(time.Millisecond))
			return r
		case resp.StatusCode == http.StatusTooManyRequests:
			if attempt+1 < maxAttempts {
				continue
			}
			r.Status = StatusWarn
			r.Message = fmt.Sprintf("%s: rate limited (429)", r.Latency.Round(time.Millisecond))
		case resp.StatusCode >= 500:
			if attempt+1 < maxAttempts {
				continue
			}
			r.Status = StatusWarn
			r.Message = fmt.Sprintf("%s: server error (%d)", r.Latency.Round(time.Millisecond), resp.StatusCode)
		case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized:
			r.Status = StatusWarn
			r.Message = fmt.Sprintf("%s: access denied (%d)", r.Latency.Round(time.Millisecond), resp.StatusCode)
		default:
			r.Status = StatusWarn
			r.Message = fmt.Sprintf("%s: status %d", r.Latency.Round(time.Millisecond), resp.StatusCode)
		}
	}

	return r
}

func testKeyedSource(
	ctx context.Context, client *http.Client, name string,
	sourceMap map[string]passive.Source, providers *passive.Providers,
) Result {
	r := Result{Section: "Passive Sources", Name: name}

	keys := passive.GetProviderKeys(providers, name)
	if len(keys) == 0 {
		r.Status = StatusWarn
		r.Message = "no API key configured"
		return r
	}

	source := sourceMap[name]
	if source == nil {
		r.Status = StatusWarn
		r.Message = "source not found"
		return r
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	ctx = passive.WithHTTPClient(ctx, client)

	results := make(chan string, 500)

	var count int
	var wg sync.WaitGroup
	wg.Go(func() {
		for range results {
			count++
		}
	})

	start := time.Now()
	err := source.Fetch(ctx, testDomain, results)
	close(results)
	wg.Wait()
	latency := time.Since(start)

	if err != nil {
		errStr := err.Error()
		switch {
		case strings.Contains(errStr, "401") || strings.Contains(errStr, "403"):
			r.Status = StatusWarn
			r.Message = fmt.Sprintf("%s: access denied (%s)", latency.Round(time.Millisecond), errStr)
		case strings.Contains(errStr, "429"):
			r.Status = StatusWarn
			r.Message = fmt.Sprintf("%s: rate limited (429)", latency.Round(time.Millisecond))
		case ctx.Err() != nil:
			r.Status = StatusWarn
			r.Message = fmt.Sprintf("unreachable (%s)", latency.Round(time.Millisecond))
		default:
			r.Status = StatusWarn
			r.Message = fmt.Sprintf("%s: %s", latency.Round(time.Millisecond), errStr)
		}
		return r
	}

	r.Status = StatusPass
	r.Message = fmt.Sprintf("%s, 0 bytes, %d results", latency.Round(time.Millisecond), count)
	return r
}

func countResults(name, body string) int {
	switch name {
	case "hackertarget":
		if strings.Contains(body, "API count exceeded") ||
			strings.Contains(body, "Increase Quota") {
			return -1
		}
		n := strings.Count(body, "\n")
		if n > 0 {
			return n
		}
		return 0
	case "wayback", "crt.sh", "urlscan",
		"commoncrawl", "anubis",
		"bing", "baidu", "google", "threatminer",
		"shodanct", "rapiddns",
		"thc", "hudsonrock",
		"submd", "reconeer":
		return strings.Count(body, testDomain)
	default:
		if strings.HasPrefix(body, "[") {
			return strings.Count(body, ",") + 1
		}
		return strings.Count(body, testDomain)
	}
}

// ─── DoH Resolvers ──────────────────────────────────.

func checkDoHEndpoints(ctx context.Context, client *http.Client, opts Options) []Result {
	type dohCheck struct {
		Name string
		URL  string
	}
	checks := []dohCheck{
		{"cloudflare-dns.com", "https://cloudflare-dns.com/dns-query"},
		{"dns.google", "https://dns.google/dns-query"},
	}

	var mu sync.Mutex
	results := make([]Result, 0, len(checks))
	var wg sync.WaitGroup

	for _, ep := range checks {
		wg.Add(1)
		go func(ep dohCheck) {
			defer wg.Done()
			r := Result{Section: "DoH Resolvers", Name: ep.Name}

			res := dns.NewResolver(client, 3*time.Second)
			res.SetProviders([]dns.Provider{
				{Name: ep.Name, URL: ep.URL, Method: http.MethodPost},
			})

			subCtx, subCancel := context.WithTimeout(ctx, 4*time.Second)
			defer subCancel()

			start := time.Now()
			ips, err := res.LookupA(subCtx, testDomain)
			r.Latency = time.Since(start)

			if err != nil {
				r.Status = StatusFail
				r.Message = fmt.Sprintf("lookup fail (%s)", r.Latency.Round(time.Millisecond))
				if opts.Verbose {
					r.Detail = err.Error()
				}
			} else {
				r.Status = StatusPass
				r.Message = fmt.Sprintf("%s, %d answers", r.Latency.Round(time.Millisecond), len(ips))
				if opts.Verbose && len(ips) > 0 {
					r.Detail = strings.Join(ips, ", ")
				}
			}
			if opts.OnResult != nil {
				opts.OnResult(r)
			}
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(ep)
	}
	wg.Wait()
	return results
}

// ─── Network ─────────────────────────────────────────.

type netCheckResult struct {
	r Result
}

func checkNetwork(ctx context.Context, client *http.Client, opts Options) []Result {
	results := make([]Result, 0, 4)
	ch := make(chan netCheckResult, 4)

	go checkInternet(ctx, client, ch)
	go checkSystemDNS(ctx, opts, ch)
	go checkIPv4(ctx, client, ch)

	emit := func(r Result) {
		if opts.OnResult != nil {
			opts.OnResult(r)
		}
		results = append(results, r)
	}

	for range 3 {
		ncr := <-ch
		emit(ncr.r)
	}

	emit(checkTor())
	emit(checkProxyEnv())

	return results
}

func checkInternet(ctx context.Context, client *http.Client, ch chan<- netCheckResult) {
	r := Result{Section: "Network", Name: "Internet"}

	subCtx, subCancel := context.WithTimeout(ctx, 5*time.Second)
	defer subCancel()

	res := dns.NewResolver(client, 4*time.Second)
	res.SetProviders([]dns.Provider{
		{Name: "cloudflare", URL: "https://cloudflare-dns.com/dns-query", Method: http.MethodPost},
		{Name: "google", URL: "https://dns.google/dns-query", Method: http.MethodPost},
	})

	start := time.Now()
	_, err := res.LookupA(subCtx, testDomain)
	r.Latency = time.Since(start)

	if err != nil {
		r.Status = StatusFail
		r.Message = fmt.Sprintf("not reachable (%s)", r.Latency.Round(time.Millisecond))
		r.Detail = err.Error()
	} else {
		r.Status = StatusPass
		r.Message = fmt.Sprintf("reachable (%s)", r.Latency.Round(time.Millisecond))
	}
	ch <- netCheckResult{r: r}
}

func checkSystemDNS(
	_ context.Context, opts Options, ch chan<- netCheckResult,
) {
	r := Result{Section: "Network", Name: "System DNS"}
	start := time.Now()
	addrs, err := net.LookupHost(testDomain)
	r.Latency = time.Since(start)

	if err != nil {
		r.Status = StatusWarn
		r.Message = fmt.Sprintf("resolution failed: %v", err)
	} else {
		r.Status = StatusPass
		r.Message = fmt.Sprintf("resolves to %d IPs (%s)", len(addrs), r.Latency.Round(time.Millisecond))
		if opts.Verbose {
			r.Detail = strings.Join(addrs, ", ")
		}
	}
	ch <- netCheckResult{r: r}
}

func checkIPv4(ctx context.Context, client *http.Client, ch chan<- netCheckResult) {
	r := Result{Section: "Network", Name: "IPv4"}
	ctx2, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx2, http.MethodGet,
		"https://1.1.1.1/dns-query?name="+testDomain+"&type=A", http.NoBody)
	req.Header.Set("Accept", "application/dns-json")
	req.Host = "cloudflare-dns.com"

	resp, err := client.Do(req)
	r.Latency = time.Since(start)
	if err != nil {
		r.Status = StatusWarn
		r.Message = fmt.Sprintf("not reachable (%s)", r.Latency.Round(time.Millisecond))
	} else {
		_ = resp.Body.Close()
		r.Status = StatusPass
		r.Message = fmt.Sprintf("reachable (%s)", r.Latency.Round(time.Millisecond))
	}
	ch <- netCheckResult{r: r}
}

func checkTor() Result {
	r := Result{Section: "Network", Name: "Tor"}
	torPath, err := exec.LookPath("tor")
	if err != nil {
		r.Status = StatusWarn
		r.Message = "not detected (install tor for --tor mode)"
		return r
	}

	// #nosec G204
	cmd := exec.Command(torPath, "--version")
	out, _ := cmd.Output()
	ver := strings.TrimSpace(string(out))
	if idx := strings.Index(ver, "\n"); idx > 0 {
		ver = ver[:idx]
	}
	if len(ver) > 50 {
		ver = truncateStr(ver, 50)
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:9050", 2*time.Second)
	daemonStatus := "daemon not running"
	if err == nil {
		_ = conn.Close()
		daemonStatus = "daemon running"
	}
	r.Status = StatusPass
	r.Message = fmt.Sprintf("found: %s (%s)", ver, daemonStatus)
	return r
}

func checkProxyEnv() Result {
	r := Result{Section: "Network", Name: "HTTP proxy env"}
	httpProxy := os.Getenv("HTTP_PROXY")
	httpsProxy := os.Getenv("HTTPS_PROXY")
	noProxy := os.Getenv("NO_PROXY")

	parts := make([]string, 0, 3)
	if httpProxy != "" {
		parts = append(parts, "HTTP_PROXY="+httpProxy)
	}
	if httpsProxy != "" {
		parts = append(parts, "HTTPS_PROXY="+httpsProxy)
	}
	if noProxy != "" {
		parts = append(parts, "NO_PROXY="+noProxy)
	}

	if len(parts) > 0 {
		r.Status = StatusWarn
		r.Message = strings.Join(parts, ", ")
		r.Detail = "These environment variables affect outbound HTTP connections"
	} else {
		r.Status = StatusPass
		r.Message = "none set"
	}
	return r
}

// ─── DNS Benchmark ──────────────────────────────────.

type benchResult struct {
	name          string
	avg, min, max time.Duration
	n             int
}

func checkDNSBenchmark(ctx context.Context, client *http.Client, opts Options) []Result {
	results := make([]Result, 0, 2)
	ch := make(chan benchResult, 2)

	go benchmarkSystemDNS(ctx, ch)
	go benchmarkDoH(ctx, client, ch)

	for range 2 {
		br := <-ch
		r := Result{Section: "DNS Benchmark", Name: br.name}
		if br.n == 0 {
			r.Status = StatusFail
			r.Message = "no results"
		} else {
			r.Status = StatusPass
			r.Message = fmt.Sprintf(
				"avg %s, min %s, max %s (%d queries)",
				br.avg.Round(time.Millisecond),
				br.min.Round(time.Millisecond),
				br.max.Round(time.Millisecond),
				br.n,
			)
		}
		if opts.OnResult != nil {
			opts.OnResult(r)
		}
		results = append(results, r)
	}

	return results
}

func benchmarkSystemDNS(_ context.Context, ch chan<- benchResult) {
	const n = 5
	durs := make([]time.Duration, 0, n)

	for range n {
		start := time.Now()
		_, err := net.LookupHost(testDomain)
		durs = append(durs, time.Since(start))
		_ = err
	}

	total := time.Duration(0)
	minD := durs[0]
	maxD := durs[0]
	for _, d := range durs {
		total += d
		if d < minD {
			minD = d
		}
		if d > maxD {
			maxD = d
		}
	}
	avg := total / n

	ch <- benchResult{name: "System resolver", avg: avg, min: minD, max: maxD, n: n}
}

func benchmarkDoH(ctx context.Context, client *http.Client, ch chan<- benchResult) {
	res := dns.NewResolver(client, 3*time.Second)
	res.SetProviders([]dns.Provider{
		{Name: "cloudflare", URL: "https://cloudflare-dns.com/dns-query", Method: http.MethodPost},
	})

	const n = 3
	durs := make([]time.Duration, 0, n)

	for range n {
		subCtx, subCancel := context.WithTimeout(ctx, 4*time.Second)

		start := time.Now()
		_, lookErr := res.LookupA(subCtx, testDomain)
		subCancel()

		if lookErr == nil {
			durs = append(durs, time.Since(start))
		}
	}

	if len(durs) == 0 {
		ch <- benchResult{name: "DoH (Cloudflare)"}
		return
	}

	total := time.Duration(0)
	minD := durs[0]
	maxD := durs[0]
	for _, d := range durs {
		total += d
		if d < minD {
			minD = d
		}
		if d > maxD {
			maxD = d
		}
	}
	avg := total / time.Duration(len(durs))

	ch <- benchResult{name: "DoH (Cloudflare)", avg: avg, min: minD, max: maxD, n: len(durs)}
}

// ─── Config Validation ─────────────────────────────.

func checkConfig(cfg *types.Config) []Result {
	var res []Result

	if cfg.PassiveOnly && cfg.Bruteforce {
		r := Result{Section: "Configuration", Name: "Conflicting flags"}
		r.Status = StatusWarn
		r.Message = "--passive disables --brute-force (--brute-force ignored)"
		res = append(res, r)
	}

	if cfg.DoH && len(cfg.Resolvers) > 0 {
		r := Result{Section: "Configuration", Name: "Resolver conflict"}
		r.Status = StatusPass
		r.Message = "--doh enabled; --resolvers list unused"
		res = append(res, r)
	}

	r2 := Result{Section: "Configuration", Name: "Concurrency"}
	r2.Status = StatusPass
	threads := cfg.Threads
	if threads == 0 {
		threads = runtime.NumCPU() * 10
	}
	r2.Message = fmt.Sprintf("%d threads", threads)
	res = append(res, r2)

	if cfg.MaxWordlistSize > 0 {
		r3 := Result{Section: "Configuration", Name: "Wordlist limit"}
		r3.Status = StatusPass
		r3.Message = fmt.Sprintf("capped at %d entries", cfg.MaxWordlistSize)
		res = append(res, r3)
	}

	if cfg.Permute {
		r4 := Result{Section: "Configuration", Name: "Permutation engine"}
		r4.Status = StatusPass
		r4.Message = fmt.Sprintf("enabled at level %d", cfg.PermuteLevel)
		if cfg.PermuteLevel > 2 {
			r4.Detail = "level 3 generates 10K-200K candidates, may be slow"
		}
		res = append(res, r4)
	}

	if cfg.NSECWalk {
		r5 := Result{Section: "Configuration", Name: "NSEC walk"}
		r5.Status = StatusWarn
		r5.Message = "enabled — queries target DNS directly (may be detectable)"
		res = append(res, r5)
	}

	return res
}

// ─── Wordlist ───────────────────────────────────────.

func checkWordlist() []Result {
	r := Result{Section: "Wordlist", Name: "Built-in"}
	start := time.Now()
	words := runner.DefaultWordlist()
	r.Latency = time.Since(start)

	if len(words) == 0 {
		r.Status = StatusFail
		r.Message = "empty"
		return []Result{r}
	}

	samples := make([]string, 0, 3)
	if len(words) >= 3 {
		samples = append(samples, words[0], words[len(words)/2], words[len(words)-1])
	}

	r.Status = StatusPass
	r.Message = fmt.Sprintf("%d entries loaded (%s)", len(words), r.Latency.Round(time.Millisecond))
	if len(samples) > 0 {
		r.Detail = fmt.Sprintf("samples: %s", strings.Join(samples, ", "))
	}
	return []Result{r}
}

// ─── Proxy ──────────────────────────────────────────.

func checkProxy(ctx context.Context, proxyURL string) Result {
	r := Result{Section: "Proxy", Name: proxyURL}

	dialer, err := active.NewProxyDialer(proxyURL)
	if err != nil {
		r.Status = StatusFail
		r.Message = fmt.Sprintf("dialer error: %v", err)
		return r
	}

	if dialer.Tripped() {
		r.Status = StatusFail
		r.Message = "circuit breaker tripped (proxy unreachable)"
		return r
	}

	client := active.NewProxiedHTTPClient(dialer, 8)
	ctx2, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx2, http.MethodGet,
		"https://cloudflare-dns.com/dns-query", http.NoBody)
	req.Header.Set("Accept", "application/dns-json")

	resp, err := client.Do(req)
	r.Latency = time.Since(start)
	if err != nil {
		r.Status = StatusFail
		r.Message = fmt.Sprintf("connection failed: %v", err)
		return r
	}
	_ = resp.Body.Close()

	r.Status = StatusPass
	r.Message = fmt.Sprintf("connected (%s)", r.Latency.Round(time.Millisecond))
	return r
}

// ─── Proxy Pool ─────────────────────────────────────.

func proxySourceName(url string) string {
	u := strings.TrimPrefix(url, "https://raw.githubusercontent.com/")
	u = strings.TrimSuffix(u, ".txt")
	parts := strings.Split(u, "/")
	if len(parts) >= 2 {
		name := parts[0] + "/" + parts[1]
		switch pType := proxy.TypeFromSourceURL(url); pType {
		case proxy.ProxyTypeSOCKS5:
			name += " (SOCKS5)"
		default:
			name += " (HTTP)"
		}
		return name
	}
	return url
}

func checkProxyPoolSource(ctx context.Context, url string, opts Options) Result {
	name := proxySourceName(url)
	r := Result{Section: "Proxy Pool", Name: name}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		r.Status = StatusFail
		r.Message = "bad request"
		return r
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	start := time.Now()
	resp, err := client.Do(req)
	r.Latency = time.Since(start)
	if err != nil {
		r.Status = StatusWarn
		r.Message = fmt.Sprintf("unreachable (%s)", r.Latency.Round(time.Millisecond))
		if opts.Verbose {
			r.Detail = err.Error()
		}
		return r
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.Status = StatusWarn
		r.Message = fmt.Sprintf("HTTP %d (%s)", resp.StatusCode, r.Latency.Round(time.Millisecond))
		return r
	}

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	var count int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}

	r.Status = StatusPass
	r.Message = fmt.Sprintf("%s, %d proxies", r.Latency.Round(time.Millisecond), count)
	return r
}

func checkProxyPool(ctx context.Context, opts Options) []Result {
	scraper := proxy.NewScraper(nil)
	sources := scraper.Sources()
	results := make([]Result, 0, len(sources)+2)

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, src := range sources {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			r := checkProxyPoolSource(ctx, url, opts)
			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(src)
	}
	wg.Wait()

	poolCtx, poolCancel := context.WithTimeout(ctx, 60*time.Second)
	defer poolCancel()

	candidates, err := scraper.Scrape(poolCtx)
	if err != nil {
		r := Result{Section: "Proxy Pool", Name: "Pool summary"}
		r.Status = StatusWarn
		r.Message = fmt.Sprintf("scrape failed: %v", err)
		results = append(results, r)
		emitAll(results, opts)
		return results
	}

	tester := proxy.NewTester(3*time.Second, 50)
	start := time.Now()
	working := tester.Test(poolCtx, candidates)
	elapsed := time.Since(start)

	var httpCount, socks5Count int
	for _, p := range working {
		switch p.Type {
		case proxy.ProxyTypeSOCKS5:
			socks5Count++
		default:
			httpCount++
		}
	}

	r := Result{Section: "Proxy Pool", Name: "Pool summary"}
	if len(working) > 0 {
		r.Status = StatusPass
	} else {
		r.Status = StatusWarn
	}
	r.Message = fmt.Sprintf(
		"%d sources, %d candidates → %d working (%d HTTP, %d SOCKS5) in %s",
		len(sources), len(candidates), len(working), httpCount, socks5Count, elapsed.Round(time.Millisecond),
	)
	results = append(results, r)
	emitAll(results, opts)

	return results
}

func emitAll(results []Result, opts Options) {
	if opts.OnResult == nil {
		return
	}
	for _, r := range results {
		opts.OnResult(r)
	}
}

// ─── Output Helpers ─────────────────────────────────.

func FormatText(results []Result, pass, warn, fail int) string {
	var b strings.Builder
	lastSection := ""

	for _, r := range results {
		if r.Section != lastSection {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			fmt.Fprintf(&b, "  %s\n", styler.Bold.Render(r.Section))
			lastSection = r.Section
		}

		var icon string
		switch r.Status {
		case StatusPass:
			icon = styler.BoldGreen.Render("[PASS]")
		case StatusWarn:
			icon = styler.BoldYellow.Render("[WARN]")
		case StatusFail:
			icon = styler.BoldRed.Render("[FAIL]")
		}

		fmt.Fprintf(&b, "    %s %-20s %s\n", icon, r.Name, r.Message)
		if r.Detail != "" {
			for line := range strings.SplitSeq(r.Detail, "\n") {
				fmt.Fprintf(&b, "           %s\n", truncateStr(line, 80))
			}
		}
	}

	fmt.Fprintf(&b, "\n  %s: %d pass, %d warn, %d fail\n", styler.Bold.Render("Summary"), pass, warn, fail)

	switch {
	case pass > 0 && warn == 0 && fail == 0:
		fmt.Fprintf(&b, "  %s\n", styler.Green.Render("All systems operational"))
	case fail > 0:
		fmt.Fprintf(&b, "  %s\n", styler.Red.Render("Issues found that must be resolved"))
	case warn > 0:
		fmt.Fprintf(&b, "  %s\n", styler.Yellow.Render("Non-critical issues detected"))
	}

	return b.String()
}

type outputJSON struct {
	Results []Result    `json:"results"`
	Summary summaryJSON `json:"summary"`
}

type summaryJSON struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
}

func FormatJSON(results []Result, pass, warn, fail int) (string, error) {
	out := outputJSON{
		Results: results,
		Summary: summaryJSON{pass, warn, fail},
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CountResults(results []Result) (pass, warn, fail int) {
	for _, r := range results {
		switch r.Status {
		case StatusPass:
			pass++
		case StatusWarn:
			warn++
		case StatusFail:
			fail++
		}
	}
	return
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
