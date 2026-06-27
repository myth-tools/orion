package runner

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/myth-tools/orion/internal/active"
	"github.com/myth-tools/orion/internal/passive"
	"github.com/myth-tools/orion/internal/proxy"
	"github.com/myth-tools/orion/internal/styler"
	"github.com/myth-tools/orion/internal/types"
)

var (
	errCircuitTripped = errors.New("proxy circuit breaker: traffic blocked — check your proxy/Tor connection")
	errNoInput        = errors.New("no input provided (use -d, -dL, or pipe stdin)")
)

type Runner struct {
	config      *types.Config
	sources     []passive.Source
	rateLimiter *passive.MultiLimiter
	resolver    *active.Resolver
	brute       *active.Bruteforcer
	output      io.Writer
	outputFile  *os.File
	dialer      *active.ProxyDialer
	httpClient  *http.Client

	matchRegexes  []*regexp.Regexp
	filterRegexes []*regexp.Regexp

	proxyPool      *proxy.Pool
	proxyTransport *proxy.RotatingTransport

	sourceStats   map[string]*types.SourceStat
	sourceStatsMu sync.Mutex
}

func initResolver(config *types.Config, httpClient *http.Client) (*active.Resolver, *proxy.Pool, *proxy.RotatingTransport) {
	var proxyPool *proxy.Pool
	var proxyTransport *proxy.RotatingTransport
	dohClient := httpClient

	if config.ProxyPool && config.ProxyURL == "" {
		scraper := proxy.NewScraper(nil)
		tester := proxy.NewTester(2*time.Second, 200)
		refreshInterval := time.Duration(config.ProxyPoolRefresh) * time.Minute
		if refreshInterval <= 0 {
			refreshInterval = 10 * time.Minute
		}
		proxyPool = proxy.NewPool(scraper, tester, refreshInterval)
		proxyTransport = proxy.NewRotatingTransport(proxyPool)
		dnsTimeout := time.Duration(config.DNSTimeout) * time.Second
		if dnsTimeout <= 0 {
			dnsTimeout = 5 * time.Second
		}
		dohClient = proxyTransport.NewHTTPClient(dnsTimeout)
	}

	dnsTimeout := time.Duration(config.DNSTimeout) * time.Second
	if dnsTimeout <= 0 {
		dnsTimeout = 5 * time.Second
	}
	return active.NewResolver(dohClient, dnsTimeout, config.DoH, config.Resolvers), proxyPool, proxyTransport
}

func initBruteforcer(config *types.Config, resolver *active.Resolver) *active.Bruteforcer {
	words := DefaultWordlist()
	if config.MaxWordlistSize > 0 && config.MaxWordlistSize < len(words) {
		words = words[:config.MaxWordlistSize]
	}
	if config.Bruteforce && config.Wordlist != "" {
		brute, err := active.NewBruteforcerFromFile(config.Wordlist, resolver,
			config.DNSPersecond, config.DNSTimeout, config.DNSRetries)
		if err != nil {
			styler.FmtWarn(os.Stderr, "Failed to load wordlist %s: %v — using default wordlist", config.Wordlist, err)
		} else {
			return brute
		}
	}
	if config.Bruteforce {
		return active.NewBruteforcer(words, resolver,
			config.DNSPersecond, config.DNSTimeout, config.DNSRetries)
	}
	return nil
}

func initMatchFilters(config *types.Config) (match, filter []*regexp.Regexp) {
	match = make([]*regexp.Regexp, 0, len(config.Match))
	filter = make([]*regexp.Regexp, 0, len(config.Filter))
	for _, m := range config.Match {
		re, err := regexp.Compile(globToRegex(m))
		if err != nil {
			styler.FmtWarn(os.Stderr, "Invalid match pattern %q: %v", m, err)
			continue
		}
		match = append(match, re)
	}
	for _, f := range config.Filter {
		re, err := regexp.Compile(globToRegex(f))
		if err != nil {
			styler.FmtWarn(os.Stderr, "Invalid filter pattern %q: %v", f, err)
			continue
		}
		filter = append(filter, re)
	}
	return
}

func New(config *types.Config, providers *passive.Providers) *Runner {
	dialer, err := active.NewProxyDialer(config.ProxyURL)
	if err != nil {
		styler.Fprintln(os.Stderr, styler.Red, "✗ Proxy setup failed:"+err.Error())
		os.Exit(1)
	}

	httpClient := active.NewProxiedHTTPClient(dialer, config.Timeout)

	resolver, proxyPool, proxyTransport := initResolver(config, httpClient)

	if proxyTransport != nil && config.ProxyURL == "" {
		timeout := time.Duration(config.Timeout) * time.Second
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		httpClient = proxyTransport.NewHTTPClient(timeout)
	}

	passive.SetSharedClient(httpClient)
	sources := passive.AllSources(providers)
	sources = filterSources(sources, config.Sources, config.ExcludeSources)

	output := io.Writer(os.Stdout)
	var outputFile *os.File
	if config.OutputFile != "" {
		f, err := os.Create(config.OutputFile)
		if err != nil {
			styler.FmtWarn(os.Stderr, "Cannot create output file %s: %v — falling back to stdout",
				config.OutputFile, err)
		} else {
			output = f
			outputFile = f
		}
	}

	brute := initBruteforcer(config, resolver)
	matchRegexes, filterRegexes := initMatchFilters(config)

	rateLimiter := buildRateLimiter(config, sources)

	return &Runner{
		config:         config,
		sources:        sources,
		rateLimiter:    rateLimiter,
		resolver:       resolver,
		brute:          brute,
		output:         output,
		outputFile:     outputFile,
		dialer:         dialer,
		httpClient:     httpClient,
		matchRegexes:   matchRegexes,
		filterRegexes:  filterRegexes,
		proxyPool:      proxyPool,
		proxyTransport: proxyTransport,
		sourceStats:    make(map[string]*types.SourceStat),
	}
}

func filterSources(all []passive.Source, include, exclude []string) []passive.Source {
	if len(include) > 0 {
		incSet := make(map[string]bool, len(include))
		for _, s := range include {
			incSet[strings.ToLower(s)] = true
		}
		var filtered []passive.Source
		for _, s := range all {
			if incSet[strings.ToLower(s.Name())] {
				filtered = append(filtered, s)
			}
		}
		return filtered
	}
	if len(exclude) > 0 {
		excSet := make(map[string]bool, len(exclude))
		for _, s := range exclude {
			excSet[strings.ToLower(s)] = true
		}
		var filtered []passive.Source
		for _, s := range all {
			if !excSet[strings.ToLower(s.Name())] {
				filtered = append(filtered, s)
			}
		}
		return filtered
	}
	return all
}

func buildRateLimiter(config *types.Config, sources []passive.Source) *passive.MultiLimiter {
	if config.RateLimit <= 0 && len(config.RateLimits) == 0 {
		return nil
	}
	ml := passive.NewMultiLimiter()
	for _, src := range sources {
		maxCount, duration := resolveRateLimit(src.Name(), config.RateLimit, config.RateLimits)
		ml.Add(src.Name(), maxCount, duration)
	}
	return ml
}

func resolveRateLimit(name string, global int, perSource map[string]types.RateLimitEntry) (uint, time.Duration) {
	lower := strings.ToLower(name)
	if entry, ok := perSource[lower]; ok && entry.MaxCount > 0 {
		return entry.MaxCount, entry.Duration
	}
	if global > 0 {
		return uint(global), time.Second
	}
	if def, ok := passive.DefaultRateLimits[lower]; ok && def.MaxCount > 0 {
		return def.MaxCount, def.Duration
	}
	return 0, 0
}

func globToRegex(pattern string) string {
	pattern = strings.ReplaceAll(pattern, ".", "\\.")
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	return "^" + pattern + "$"
}

func (r *Runner) filterAndMatchSubdomain(subdomain string) bool {
	for _, filter := range r.filterRegexes {
		if filter.MatchString(subdomain) {
			return false
		}
	}
	if len(r.matchRegexes) > 0 {
		for _, match := range r.matchRegexes {
			if match.MatchString(subdomain) {
				return true
			}
		}
		return false
	}
	return true
}

func validSubdomain(sub, domain string) bool {
	if sub == "" || strings.HasPrefix(sub, "*.") || strings.HasPrefix(sub, ".") {
		return false
	}
	sub = strings.ToLower(sub)
	domain = strings.ToLower(domain)
	return strings.HasSuffix(sub, "."+domain)
}

func (r *Runner) log(format string, args ...any) {
	if r.config.Silent {
		return
	}
	msg := fmt.Sprintf(format, args...)
	switch {
	case strings.HasPrefix(msg, "[+]"):
		msg = styler.Green.Render("[+]") + msg[3:]
	case strings.HasPrefix(msg, "[!]"):
		msg = styler.Yellow.Render("[!]") + msg[3:]
	case strings.HasPrefix(msg, "warning:"):
		msg = styler.Yellow.Render("warning:") + msg[8:]
	}
	fmt.Fprint(os.Stderr, msg)
}

func jitterMs(low, high int) time.Duration {
	diff := int64(high - low + 1)
	n, err := rand.Int(rand.Reader, big.NewInt(diff))
	if err != nil {
		return time.Duration(low) * time.Millisecond
	}
	return time.Duration(int64(low)+n.Int64()) * time.Millisecond
}

func (r *Runner) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if r.rateLimiter != nil {
		defer r.rateLimiter.Stop()
	}

	if r.dialer.Tripped() {
		return fmt.Errorf("%w at %s", errCircuitTripped, r.config.ProxyURL)
	}

	r.initProxyPool(ctx)

	domains, err := r.loadDomains()
	if err != nil {
		return err
	}

	startTime := time.Now()
	r.handleSignals(cancel)
	r.printBanner()

	for i, domain := range domains {
		if i > 0 {
			fmt.Fprintln(os.Stderr)
		}
		r.enumerateDomain(ctx, domain)
	}

	if r.config.Statistics {
		r.printStats()
	}

	elapsed := time.Since(startTime)
	if !r.config.Silent && len(domains) > 1 {
		styler.FmtInfo(os.Stderr, "Total time: %s", elapsed.Round(time.Millisecond))
	}

	if r.outputFile != nil {
		if err := r.outputFile.Close(); err != nil {
			r.log("[!] Error closing output file: %v\n", err)
		}
	}
	return nil
}

func (r *Runner) initProxyPool(ctx context.Context) {
	if r.proxyPool == nil || r.config.ProxyURL != "" {
		return
	}
	if !r.config.Silent {
		r.log("[+] Initializing proxy pool (scraping + testing)...\n")
	}
	if err := r.proxyPool.Start(ctx); err != nil {
		r.log("[!] Proxy pool init failed: %v — continuing without rotating proxies\n", err)
		r.proxyPool = nil
		r.proxyTransport = nil
		return
	}

	size := r.proxyPool.Size()
	if r.config.ProxyPoolMin > 0 && size < r.config.ProxyPoolMin {
		r.log("[!] Proxy pool size (%d) below minimum (%d) — continuing with available proxies\n", size, r.config.ProxyPoolMin)
	}
	if !r.config.Silent && size > 0 {
		r.log("[+] Proxy pool ready: %d working proxies\n", size)
	}
}

func (r *Runner) loadDomains() ([]string, error) {
	var domains []string
	if r.config.Domain != "" {
		domains = append(domains, preprocessDomain(r.config.Domain))
	}
	if r.config.DomainsFile != "" {
		f, err := os.Open(filepath.Clean(r.config.DomainsFile))
		if err != nil {
			return nil, fmt.Errorf("opening domain file: %w", err)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			d := preprocessDomain(scanner.Text())
			if d != "" {
				domains = append(domains, d)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading domain file: %w", err)
		}
	}
	if HasStdin() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			d := preprocessDomain(scanner.Text())
			if d != "" {
				domains = append(domains, d)
			}
		}
	}
	if len(domains) == 0 {
		return nil, errNoInput
	}
	return domains, nil
}

func preprocessDomain(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.NewReplacer(
		"http://", "", "https://", "", "*.", "", "/", "",
	).Replace(s)
	return s
}

func HasStdin() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice == 0
}

func (r *Runner) enumerateDomain(ctx context.Context, domain string) {
	startTime := time.Now()
	r.log("[+] Enumerating subdomains for %s\n", domain)

	outputCh := make(chan string, 10000)
	seen := make(map[string]bool)
	uniqueMap := make(map[string]HostEntry)
	sourceMap := make(map[string]map[string]struct{})
	var mu sync.Mutex

	outputDone := make(chan struct{})
	go func() {
		for sub := range outputCh {
			mu.Lock()
			if seen[sub] {
				mu.Unlock()
				continue
			}
			seen[sub] = true
			if _, ok := uniqueMap[sub]; !ok {
				uniqueMap[sub] = HostEntry{Host: sub, Domain: domain}
			}
			mu.Unlock()

			if !r.config.JSON && !r.config.HostIP && !r.config.CaptureSources {
				fmt.Fprintln(r.output, sub)
			}
		}
		close(outputDone)
	}()

	var passiveWg sync.WaitGroup
	passiveResults := make(chan string, 10000)

	r.runPassiveSources(ctx, &mu, &passiveWg, passiveResults, sourceMap)
	r.runNSECWalkPhase(ctx, &mu, &passiveWg, passiveResults, sourceMap)

	discoveredList := r.waitPassivePhase(ctx, &passiveWg, passiveResults, outputCh, sourceMap, &mu)

	if r.config.Permute && len(discoveredList) > 0 {
		r.runPermutationPhase(ctx, outputCh, discoveredList, sourceMap)
	}

	if !r.config.PassiveOnly && r.brute != nil {
		r.runBruteforcePhase(ctx, outputCh, sourceMap)
	}

	close(outputCh)
	<-outputDone

	mu.Lock()
	for sub := range uniqueMap {
		if !validSubdomain(sub, domain) {
			delete(uniqueMap, sub)
			continue
		}
		entry := uniqueMap[sub]
		if srcs, ok := sourceMap[sub]; ok {
			srcsList := mapsKeys(srcs)
			if len(srcsList) > 0 {
				entry.Source = srcsList[0]
			}
		}
		uniqueMap[sub] = entry
	}

	if r.config.RemoveWildcard {
		r.resolveAndFilterWildcards(ctx, domain, uniqueMap, sourceMap)
	}
	mu.Unlock()

	r.writeOutput(domain, uniqueMap, sourceMap)

	elapsed := time.Since(startTime)
	r.log("[+] Found %d subdomains for %s in %s\n", len(uniqueMap), domain, elapsed.Round(time.Millisecond))
	r.printSummary(domain, len(uniqueMap), elapsed, sourceMap)
}

func mapsKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (r *Runner) resolveAndFilterWildcards(
	ctx context.Context, domain string,
	uniqueMap map[string]HostEntry,
	sourceMap map[string]map[string]struct{},
) {
	r.log("[+] Wildcard detection and resolution...\n")
	wcCtx, wcCancel := context.WithTimeout(ctx, 15*time.Second)
	hasWildcard, err := r.resolver.DetectWildcard(wcCtx, domain)
	wcCancel()
	if err != nil && ctx.Err() == nil {
		r.log("[!] Wildcard detection: %v\n", err)
	}
	if hasWildcard {
		r.log("[!] Wildcard DNS detected — filtering wildcard responses\n")
	}

	sem := make(chan struct{}, r.config.Threads)
	var resolveWg sync.WaitGroup
	var resolveMu sync.Mutex

	for host, entry := range uniqueMap {
		sem <- struct{}{}
		resolveWg.Add(1)
		go func(h string, e HostEntry) {
			defer resolveWg.Done()
			defer func() { <-sem }()

			subCtx, subCancel := context.WithTimeout(ctx, 5*time.Second)
			defer subCancel()

			ips, err := r.resolver.LookupSingle(subCtx, h)
			if err != nil || len(ips) == 0 {
				resolveMu.Lock()
				delete(uniqueMap, h)
				delete(sourceMap, h)
				resolveMu.Unlock()
				return
			}

			e.IPs = ips
			if hasWildcard {
				var nonWildcard []string
				for _, ip := range ips {
					if !r.resolver.IsWildcardIP(ip) {
						nonWildcard = append(nonWildcard, ip)
					}
				}
				if len(nonWildcard) == 0 {
					resolveMu.Lock()
					delete(uniqueMap, h)
					delete(sourceMap, h)
					resolveMu.Unlock()
					return
				}
				e.IPs = nonWildcard
			}
			resolveMu.Lock()
			uniqueMap[h] = e
			resolveMu.Unlock()
		}(host, entry)
	}
	resolveWg.Wait()
}

func (r *Runner) writeOutput(domain string, uniqueMap map[string]HostEntry, sourceMap map[string]map[string]struct{}) {
	writers := []io.Writer{r.output}

	if r.config.OutputDirectory != "" {
		baseName := domain
		if r.config.HostIP {
			baseName += "-ip"
		}
		ext := ".txt"
		if r.config.JSON {
			ext = ".json"
		}
		outPath := filepath.Join(r.config.OutputDirectory, baseName+ext)
		f, err := NewOutputWriter(r.config.JSON).createFile(outPath, false)
		if err != nil {
			r.log("[!] Cannot create output file %s: %v\n", outPath, err)
		} else {
			writers = append(writers, f)
			defer f.Close()
		}
	}

	ow := NewOutputWriter(r.config.JSON)
	for _, w := range writers {
		var err error
		switch {
		case r.config.CaptureSources:
			err = ow.WriteSources(domain, sourceMap, w)
		case r.config.HostIP:
			err = ow.WriteHostIP(domain, uniqueMap, w)
		default:
			err = ow.Write(domain, uniqueMap, w)
		}
		if err != nil {
			r.log("[!] Output write error: %v\n", err)
		}
	}
}

func (r *Runner) handleSignals(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT)
	go func() {
		<-sigCh
		r.log("\n[!] Interrupted — flushing results… (Ctrl+C again to abort immediately)\n")
		cancel()
		<-sigCh
		r.log("\n[!] Forced exit.\n")
		os.Exit(1)
	}()
}

func (r *Runner) printBanner() {
	if r.config.Silent || r.config.JSON {
		return
	}
	r.log("[+] Target: %s\n", r.config.Domain)
	if len(r.config.Sources) > 0 {
		r.log("[!] Using specific sources: %s\n", strings.Join(r.config.Sources, ", "))
	}
	if len(r.config.ExcludeSources) > 0 {
		r.log("[!] Excluding sources: %s\n", strings.Join(r.config.ExcludeSources, ", "))
	}
	if r.config.ProxyURL != "" {
		r.log("[!] All traffic routed through proxy: %s\n", r.config.ProxyURL)
	}
	if r.config.TorMode {
		r.log("[!] Tor circuit isolation active — each request exits from different IP\n")
	}
	if r.config.ProxyPool && r.proxyPool != nil {
		r.log("[!] Rotating proxy pool: enabled (%d proxies)\n", r.proxyPool.Size())
	}
	r.log("[!] DNS via DoH: enabled (multi-provider round-robin)\n")
	if r.config.NSECWalk {
		r.log("[!] NSEC walking sends DNS queries to target's authoritative nameservers\n")
		r.log("[!] Without proxy (--tor/--proxy), your IP will be visible to the target\n")
		if r.config.ProxyURL == "" && r.proxyPool == nil {
			r.log("[!] WARNING: No proxy configured — NSEC queries will expose your IP to target DNS\n")
		}
	}
	if r.config.RemoveWildcard {
		r.log("[!] Wildcard filtering: enabled\n")
	}
	if r.config.Statistics {
		r.log("[!] Source statistics: enabled\n")
	}
}

func (r *Runner) waitPassivePhase(
	_ context.Context,
	passiveWg *sync.WaitGroup,
	passiveResults chan string,
	outputCh chan<- string,
	sourceMap map[string]map[string]struct{},
	mu *sync.Mutex,
) []string {
	passiveDone := make(chan struct{})
	go func() {
		passiveWg.Wait()
		close(passiveResults)
	}()
	go func() {
		for sub := range passiveResults {
			outputCh <- sub
		}
		close(passiveDone)
	}()

	select {
	case <-passiveDone:
	case <-time.After(time.Duration(r.config.Timeout+5) * time.Second):
		r.log("[!] Passive phase timed out\n")
	}

	mu.Lock()
	passiveCount := len(mapsKeys(sourceMap))
	mu.Unlock()
	r.log("[+] Passive: %d unique subdomains\n", passiveCount)

	mu.Lock()
	discoveredList := make([]string, 0, len(mapsKeys(sourceMap)))
	for sub := range sourceMap {
		discoveredList = append(discoveredList, sub)
	}
	mu.Unlock()

	return discoveredList
}

func (r *Runner) runPassiveSources(
	ctx context.Context,
	mu *sync.Mutex,
	passiveWg *sync.WaitGroup,
	passiveResults chan<- string,
	sourceMap map[string]map[string]struct{},
) {
	sourceTimeout := r.config.Timeout
	if sourceTimeout > 35 {
		sourceTimeout = 30
	}

	for i, src := range r.sources {
		time.Sleep(jitterMs(50, 300))

		passiveWg.Add(1)
		go func(s passive.Source, idx int) {
			defer passiveWg.Done()
			r.runPassiveSource(ctx, mu, passiveResults, sourceMap, s, idx, sourceTimeout)
		}(src, i)
	}
}

func (r *Runner) runPassiveSource(
	ctx context.Context,
	mu *sync.Mutex,
	passiveResults chan<- string,
	sourceMap map[string]map[string]struct{},
	src passive.Source,
	idx int,
	timeout int,
) {
	baseCtx := ctx
	if r.httpClient != nil {
		baseCtx = passive.WithHTTPClient(ctx, r.httpClient)
	}
	if r.rateLimiter != nil {
		baseCtx = passive.WithRateLimiter(baseCtx, r.rateLimiter)
		baseCtx = passive.WithSourceName(baseCtx, src.Name())
	}
	srcCtx, srcCancel := context.WithTimeout(baseCtx, time.Duration(timeout)*time.Second)
	defer srcCancel()

	if idx > 0 {
		time.Sleep(jitterMs(100, 500))
	}

	stat := r.getSourceStat(src.Name())
	ch := make(chan string, 1000)
	done := make(chan struct{})

	go r.processPassiveResults(ctx, mu, passiveResults, sourceMap, ch, done, src, stat)

	startFetch := time.Now()
	err := src.Fetch(srcCtx, r.config.Domain, ch)
	close(ch)
	<-done

	r.sourceStatsMu.Lock()
	stat.TimeTaken += time.Since(startFetch)
	stat.Requests++
	if err != nil {
		stat.Errors++
	}
	r.sourceStatsMu.Unlock()

	if err != nil && ctx.Err() == nil {
		r.log("[!] %s: %v\n", src.Name(), err)
	}
}

func (r *Runner) processPassiveResults(
	ctx context.Context,
	mu *sync.Mutex,
	passiveResults chan<- string,
	sourceMap map[string]map[string]struct{},
	ch <-chan string,
	done chan<- struct{},
	src passive.Source,
	stat *types.SourceStat,
) {
	for sub := range ch {
		if !validSubdomain(sub, r.config.Domain) {
			continue
		}
		if !r.filterAndMatchSubdomain(sub) {
			continue
		}

		r.sourceStatsMu.Lock()
		stat.Results++
		r.sourceStatsMu.Unlock()

		mu.Lock()
		if sourceMap[sub] == nil {
			sourceMap[sub] = make(map[string]struct{})
		}
		sourceMap[sub][src.Name()] = struct{}{}
		mu.Unlock()

		select {
		case passiveResults <- sub:
		case <-ctx.Done():
			close(done)
			return
		}
	}
	close(done)
}

func (r *Runner) getSourceStat(name string) *types.SourceStat {
	r.sourceStatsMu.Lock()
	defer r.sourceStatsMu.Unlock()
	if _, ok := r.sourceStats[name]; !ok {
		r.sourceStats[name] = &types.SourceStat{Name: name}
	}
	return r.sourceStats[name]
}

func (r *Runner) runNSECWalkPhase(
	ctx context.Context,
	mu *sync.Mutex,
	passiveWg *sync.WaitGroup,
	passiveResults chan<- string,
	sourceMap map[string]map[string]struct{},
) {
	if !r.config.NSECWalk {
		return
	}

	passiveWg.Go(func() {
		nsecCtx, nsecCancel := context.WithTimeout(ctx, 30*time.Second)
		defer nsecCancel()

		var poolDialer func(ctx context.Context, network, addr string) (net.Conn, error)
		if r.proxyTransport != nil && r.config.ProxyURL == "" {
			poolDialer = r.proxyTransport.DialContext
		}
		walker := active.NewNSECWalker(r.dialer, poolDialer)
		nsecCh := make(chan string, 1000)
		nsecErr := make(chan error, 1)

		go func() {
			nsecErr <- walker.Walk(nsecCtx, r.config.Domain, nsecCh)
		}()

		for sub := range nsecCh {
			if !validSubdomain(sub, r.config.Domain) {
				continue
			}
			if !r.filterAndMatchSubdomain(sub) {
				continue
			}

			mu.Lock()
			if sourceMap[sub] == nil {
				sourceMap[sub] = make(map[string]struct{})
			}
			sourceMap[sub]["nsec"] = struct{}{}
			mu.Unlock()

			select {
			case passiveResults <- sub:
			case <-nsecCtx.Done():
				return
			}
		}

		err := <-nsecErr
		if err != nil && ctx.Err() == nil {
			r.log("[!] NSEC: %v\n", err)
		}
	})
}

func (r *Runner) runPermutationPhase(
	ctx context.Context, outputCh chan<- string,
	discoveredList []string,
	sourceMap map[string]map[string]struct{},
) {
	r.log("[+] Permutation phase (%d base subs, level %d)...\n", len(discoveredList), r.config.PermuteLevel)

	timeout := r.config.ActiveTimeout
	if timeout <= 0 {
		timeout = 180
	}
	permCtx, permCancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer permCancel()

	perm := active.NewPermutator(r.config.PermuteLevel)
	candidates := perm.Generate(discoveredList, r.config.Domain)
	r.log("[+] Permutation: %d candidates, resolving...\n", len(candidates))

	sem := make(chan struct{}, r.config.Threads)
	var permWg sync.WaitGroup
	dnsTimeout := time.Duration(r.config.DNSTimeout) * time.Second
	if dnsTimeout <= 0 {
		dnsTimeout = 6 * time.Second
	}

	for _, candidate := range candidates {
		select {
		case <-permCtx.Done():
			permWg.Wait()
			return
		default:
		}

		sem <- struct{}{}
		permWg.Add(1)
		go func(c string) {
			defer permWg.Done()
			defer func() { <-sem }()

			ips, err := r.resolver.LookupWithRetry(permCtx, c, r.config.DNSRetries, dnsTimeout)
			if err != nil || len(ips) == 0 {
				return
			}

			if sourceMap[c] == nil {
				sourceMap[c] = make(map[string]struct{})
			}
			sourceMap[c]["permutation"] = struct{}{}

			select {
			case outputCh <- c:
			case <-permCtx.Done():
			}
		}(candidate)
	}
	permWg.Wait()
}

func (r *Runner) runBruteforcePhase(ctx context.Context, outputCh chan<- string, sourceMap map[string]map[string]struct{}) {
	if !r.resolver.HasWildcard() {
		r.log("[+] Wildcard detection...\n")
		wcCtx, wcCancel := context.WithTimeout(ctx, 15*time.Second)
		r.resolver.DetectWildcard(wcCtx, r.config.Domain) //nolint:errcheck
		wcCancel()
	}

	if r.resolver.HasWildcard() {
		r.log("[!] Wildcard DNS detected — filtering wildcard responses\n")
	}

	if err := r.resolver.Check(ctx); err != nil {
		r.log("[!] Resolver health check: %v\n", err)
	}

	timeout := r.config.ActiveTimeout
	if timeout <= 0 {
		timeout = 300
	}

	r.log("[+] Brute-force phase (%d words, %d threads)...\n",
		r.brute.WordlistSize(), r.config.Threads)
	bruteStart := time.Now()
	bruteResults := make(chan string, 10000)
	bruteCtx, bruteCancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer bruteCancel()

	var bruteWg sync.WaitGroup
	bruteWg.Go(func() {
		r.brute.Run(bruteCtx, active.Config{
			Domain:  r.config.Domain,
			Threads: r.config.Threads,
			Results: bruteResults,
			LogFn:   r.log,
		})
	})

	progressDone := make(chan struct{})
	go r.trackBruteProgress(bruteResults, bruteStart, progressDone)

	go func() {
		bruteWg.Wait()
		close(bruteResults)
	}()

	var bruteFound int
	for sub := range bruteResults {
		bruteFound++
		if !validSubdomain(sub, r.config.Domain) {
			continue
		}
		if !r.filterAndMatchSubdomain(sub) {
			continue
		}
		if sourceMap[sub] == nil {
			sourceMap[sub] = make(map[string]struct{})
		}
		sourceMap[sub]["bruteforce"] = struct{}{}
		select {
		case outputCh <- sub:
		case <-ctx.Done():
		}
	}
	<-progressDone

	stat := r.brute.Stat()
	r.log("[+] Brute-force done: %d found, %d errors, %d timeouts, %d retries in %s\n",
		bruteFound, stat.Errors, stat.Timeouts, stat.Retries, time.Since(bruteStart))
}

func (r *Runner) trackBruteProgress(results <-chan string, start time.Time, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var received int
	for {
		select {
		case _, ok := <-results:
			if !ok {
				r.updateProgressLine(start)
				return
			}
			received++
		case <-ticker.C:
			r.updateProgressLine(start)
		}
	}
}

func (r *Runner) updateProgressLine(start time.Time) {
	stat := r.brute.Stat()
	if stat.Total == 0 {
		return
	}
	elapsed := time.Since(start).Round(time.Second)
	pct := float64(stat.Completed) / float64(stat.Total) * 100
	fmt.Fprintf(os.Stderr, "\r  %s [%d/%d] %3.0f%% — found: %d — err: %d — timeouts: %d — %v        ",
		styler.BoldCyan.Render("▸"), stat.Completed, stat.Total, pct, stat.Found, stat.Errors, stat.Timeouts, elapsed)
	if stat.Completed >= stat.Total {
		fmt.Fprintln(os.Stderr)
	}
	r.sourceStatsMu.Lock()
	if r.sourceStats["bruteforce"] == nil {
		r.sourceStats["bruteforce"] = &types.SourceStat{Name: "bruteforce"}
	}
	r.sourceStats["bruteforce"].Results = stat.Found
	r.sourceStats["bruteforce"].Errors = stat.Errors
	r.sourceStats["bruteforce"].Requests = stat.Completed
	r.sourceStats["bruteforce"].TimeTaken = time.Since(start)
	r.sourceStatsMu.Unlock()
}

type srcEntry struct {
	name string
	stat *types.SourceStat
}

type summaryData struct {
	fromBrute    int
	fromPermute  int
	fromNSEC     int
	techniques   string
	outputTarget string
	errorList    []string
	totalErrors  int
	sorted       []srcEntry
}

func (r *Runner) getOutputTarget() string {
	switch {
	case r.config.OutputFile != "":
		return r.config.OutputFile
	case r.config.OutputDirectory != "":
		return r.config.OutputDirectory + "/"
	default:
		return "stdout"
	}
}

func (r *Runner) sortedSources() []srcEntry {
	r.sourceStatsMu.Lock()
	entries := make([]srcEntry, 0, len(r.sourceStats))
	for name, st := range r.sourceStats {
		entries = append(entries, srcEntry{name, st})
	}
	r.sourceStatsMu.Unlock()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].stat.Results > entries[j].stat.Results
	})
	return entries
}

func (r *Runner) collectTechniques(sourceMap map[string]map[string]struct{}) (brute, permute, nsec, passive int) {
	passiveSrcs := make(map[string]int)
	for _, srcs := range sourceMap {
		for s := range srcs {
			switch s {
			case "bruteforce":
				brute++
			case "permutation":
				permute++
			case "nsec":
				nsec++
			default:
				passiveSrcs[s]++
			}
		}
	}
	for _, s := range r.sources {
		if _, ok := passiveSrcs[s.Name()]; ok {
			passive++
		}
	}
	return
}

func (r *Runner) buildTechniquesString(activePassive, fromBrute, fromPermute, fromNSEC int) string {
	var parts []string
	if len(r.sources) > 0 {
		parts = append(parts, fmt.Sprintf("Passive (%d/%d)", activePassive, len(r.sources)))
	}
	if r.brute != nil && fromBrute > 0 {
		parts = append(parts, "Brute-force")
	}
	if r.config.Permute && fromPermute > 0 {
		parts = append(parts, "Permutation")
	}
	if r.config.NSECWalk && fromNSEC > 0 {
		parts = append(parts, "NSEC")
	}
	if len(parts) == 0 {
		parts = append(parts, "None")
	}
	return strings.Join(parts, " · ")
}

func (r *Runner) collectSummary(sourceMap map[string]map[string]struct{}) summaryData {
	fromBrute, fromPermute, fromNSEC, activePassive := r.collectTechniques(sourceMap)

	var d summaryData
	d.fromBrute = fromBrute
	d.fromPermute = fromPermute
	d.fromNSEC = fromNSEC
	d.techniques = r.buildTechniquesString(activePassive, fromBrute, fromPermute, fromNSEC)
	d.outputTarget = r.getOutputTarget()
	d.sorted = r.sortedSources()

	r.sourceStatsMu.Lock()
	for name, st := range r.sourceStats {
		if st.Errors > 0 {
			d.totalErrors += st.Errors
			if len(d.errorList) < 5 {
				d.errorList = append(d.errorList, name)
			}
		}
	}
	r.sourceStatsMu.Unlock()

	return d
}

func (r *Runner) printSummary(domain string, total int, elapsed time.Duration, sourceMap map[string]map[string]struct{}) {
	if r.config.Silent || r.config.JSON {
		return
	}

	d := r.collectSummary(sourceMap)

	var buf strings.Builder

	// Header
	buf.WriteString(styler.Green.Render("✔"))
	buf.WriteString(" ")
	buf.WriteString(styler.Bold.Render("Scan Complete — " + domain))
	buf.WriteByte('\n')
	buf.WriteByte('\n')

	labelStyle := styler.Dim.Width(14)

	addRow := func(label, value string) {
		buf.WriteString(labelStyle.Render(label))
		buf.WriteString(styler.BoldCyan.Render("│"))
		buf.WriteString(value)
		buf.WriteByte('\n')
	}

	addRow("Subdomains", styler.BoldCyan.Render(fmt.Sprintf("%d", total)))
	addRow("Duration", styler.Bold.Render(elapsed.Round(time.Millisecond).String()))
	addRow("Techniques", styler.White.Render(d.techniques))

	if d.fromBrute > 0 {
		addRow("Brute-force", styler.White.Render(fmt.Sprintf("%d", d.fromBrute)))
	}
	if d.fromPermute > 0 {
		addRow("Permutation", styler.White.Render(fmt.Sprintf("%d", d.fromPermute)))
	}
	if d.fromNSEC > 0 {
		addRow("NSEC Walk", styler.White.Render(fmt.Sprintf("%d", d.fromNSEC)))
	}
	addRow("Output", styler.White.Render(d.outputTarget))

	if r.config.ProxyURL != "" {
		proxyLabel := r.config.ProxyURL
		if r.config.TorMode {
			proxyLabel = "Tor (" + r.config.ProxyURL + ")"
		}
		addRow("Proxy", styler.Yellow.Render(proxyLabel))
	}

	if d.totalErrors > 0 {
		errStr := fmt.Sprintf("%d — %s", d.totalErrors, strings.Join(d.errorList, ", "))
		if len(d.errorList) < d.totalErrors {
			errStr += fmt.Sprintf(" (+%d more)", d.totalErrors-len(d.errorList))
		}
		addRow("Errors", styler.Red.Render(errStr))
	}

	if len(d.sorted) > 0 {
		buf.WriteByte('\n')
		buf.WriteString("  ")
		buf.WriteString(styler.Dim.Render("Top Sources"))
		buf.WriteByte('\n')

		limit := min(5, len(d.sorted))
		for _, s := range d.sorted[:limit] {
			dur := s.stat.TimeTaken.Round(time.Millisecond)
			fmt.Fprintf(&buf, "    %s %s %s\n",
				styler.Cyan.Render(fmt.Sprintf("%-20s", s.name)),
				styler.BoldCyan.Render(fmt.Sprintf("%4d", s.stat.Results)),
				styler.Dim.Render(fmt.Sprintf("in %v (%d req)", dur, s.stat.Requests)))
		}
	}

	fmt.Fprintln(os.Stderr) // blank line before box
	fmt.Fprintln(os.Stderr, styler.BorderedBox(strings.TrimRight(buf.String(), "\n")))
}

func (r *Runner) printStats() {
	if len(r.sourceStats) == 0 {
		return
	}
	r.sourceStatsMu.Lock()
	defer r.sourceStatsMu.Unlock()

	type statLine struct {
		name string
		stat *types.SourceStat
	}
	lines := make([]statLine, 0, len(r.sourceStats))
	for name, stat := range r.sourceStats {
		lines = append(lines, statLine{name, stat})
	}
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].name < lines[j].name
	})

	fmt.Fprintln(os.Stderr)
	styler.Fprintln(os.Stderr, styler.Bold, "  Source Statistics")
	fmt.Fprintf(os.Stderr, "  %-24s %-12s %8s %8s %8s\n",
		styler.Bold.Render("Source"), styler.Bold.Render("Duration"),
		styler.Bold.Render("Results"), styler.Bold.Render("Req"), styler.Bold.Render("Err"))
	styler.Fprintln(os.Stderr, styler.Dim, "  "+strings.Repeat("─", 68))

	var totalResults, totalReqs, totalErrors int
	var totalDuration time.Duration
	for _, line := range lines {
		s := line.stat
		if s.Skipped {
			continue
		}
		dur := s.TimeTaken.Round(time.Millisecond)
		fmt.Fprintf(os.Stderr, "  %-24s %-12s %8d %8d %8d\n", s.Name, dur.String(), s.Results, s.Requests, s.Errors)
		totalResults += s.Results
		totalReqs += s.Requests
		totalErrors += s.Errors
		totalDuration += s.TimeTaken
	}
	styler.Fprintln(os.Stderr, styler.Dim, "  "+strings.Repeat("─", 68))
	totalDur := totalDuration.Round(time.Millisecond).String()
	styler.Fprintf(os.Stderr, styler.BoldCyan, "  %-24s %-12s %8d %8d %8d\n", "Total", totalDur, totalResults, totalReqs, totalErrors)
	fmt.Fprintln(os.Stderr)
}
