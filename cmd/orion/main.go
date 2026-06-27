package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/myth-tools/orion/internal/doctor"
	"github.com/myth-tools/orion/internal/passive"
	"github.com/myth-tools/orion/internal/runner"
	"github.com/myth-tools/orion/internal/styler"
	"github.com/myth-tools/orion/internal/types"
)

var (
	version     = "dev"
	programName = "orion"
)

const torProxy = "socks5://127.0.0.1:9050"

type cliFlags struct {
	domain           string
	domainFile       string
	outputFile       string
	outputDir        string
	threads          int
	timeout          int
	silent           bool
	verbose          bool
	json             bool
	hostIP           bool
	captureSources   bool
	removeWildcard   bool
	statistics       bool
	listSources      bool
	bruteforce       bool
	wordlist         string
	passiveOnly      bool
	resolvers        string
	permute          bool
	permuteLevel     int
	nsecWalk         bool
	doh              bool
	maxWordlistSize  int
	proxyURL         string
	torMode          bool
	noProxyPool      bool
	providerConfig   string
	sources          string
	excludeSources   string
	match            string
	filter           string
	rateLimit        int
	rateLimits       string
	showVersion      bool
	showHelp         bool
	deep             bool
	dnsRate          int
	dnsRetries       int
	dnsTimeout       int
	activeTimeout    int
	proxyPool        bool
	proxyPoolMin     int
	proxyPoolRefresh int
}

var defaultThreads = runtime.NumCPU() * 10

func main() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if v := info.Main.Version; v != "" && v != "(devel)" {
				version = v
			}
		}
	}

	passive.EnsureConfig("")

	doctorArgs, mainArgs := splitArgs()

	// Print banner before any command — but not in silent / JSON mode
	if !hasRawFlag("-silent") && !hasRawFlag("-json") &&
		!hasRawFlag("--silent") && !hasRawFlag("--json") &&
		!hasRawFlag("-version") && !hasRawFlag("--version") {
		fmt.Fprintln(os.Stderr, getToolBanner())
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr)
	}

	f := parseFlags(mainArgs)

	if doctorArgs != nil {
		os.Exit(runDoctorInner(doctorArgs, f))
	}
	if f.showVersion {
		styler.Fprint(os.Stderr, styler.BoldCyan, programName, " ", version)
		fmt.Fprintln(os.Stderr)
		os.Exit(0)
	}
	if f.listSources {
		listSources(f.providerConfig)
		os.Exit(0)
	}
	runScan(f)
}

func splitArgs() (doctorArgs, mainArgs []string) {
	for i, a := range os.Args[1:] {
		if a == "doctor" && !strings.HasPrefix(a, "-") {
			return os.Args[i+2:], os.Args[1 : i+1]
		}
	}
	return nil, os.Args[1:]
}

func listSources(providerConfig string) {
	styler.Fprintf(os.Stderr, styler.BoldCyan, "[+] Available passive sources:\n\n")
	providers, _ := passive.LoadProviders(providerConfig)
	for _, s := range passive.AllSources(providers) {
		mark := " "
		if s.NeedsKey() {
			mark = "*"
		}
		styler.Fprintf(os.Stderr, styler.White, "  %s %s\n", mark, s.Name())
	}
	styler.Fprintf(os.Stderr, styler.Dim, "\n  * = requires API key\n")
}

func applyDeepMode(f *cliFlags) {
	f.permute = true
	f.permuteLevel = 3
	f.nsecWalk = true
	if f.timeout < 60 {
		f.timeout = 60
	}
	if f.proxyURL == "" {
		f.torMode = true
	}
	if !f.silent {
		styler.FmtInfo(os.Stderr, "Deep mode: permute + NSEC + Tor + %ds timeout", f.timeout)
	}
}

func runScan(f cliFlags) {
	if f.deep {
		applyDeepMode(&f)
	}
	setupProxy(&f)

	if f.noProxyPool {
		f.proxyPool = false
	}
	f.threads = max(min(f.threads, runtime.NumCPU()*10), 1)

	resolverList := strings.Split(f.resolvers, ",")
	for i := range resolverList {
		resolverList[i] = strings.TrimSpace(resolverList[i])
	}

	domains := loadDomains(f.domain, f.domainFile)
	if len(domains) == 0 && !runner.HasStdin() {
		styler.FmtError(os.Stderr, "target domain required")
		os.Exit(1)
	}

	pl := max(1, min(3, f.permuteLevel))

	providers, err := passive.LoadProviders(f.providerConfig)
	if err != nil && !f.silent {
		styler.FmtWarn(os.Stderr, "Warning loading provider config: %v", err)
	}

	if len(domains) == 0 {
		domains = append(domains, "")
	}

	for _, domain := range domains {
		config := buildConfig(f, domain, resolverList, pl)
		r := runner.New(config, providers)
		if err := r.Run(); err != nil {
			styler.FmtError(os.Stderr, "%v", err)
			os.Exit(1)
		}
	}
}

func setupProxy(f *cliFlags) {
	if f.torMode && f.proxyURL == "" {
		f.proxyURL = torProxy
	}
	if f.proxyURL == "" || f.silent {
		return
	}
	if f.torMode {
		styler.FmtWarn(os.Stderr, "Tor mode enabled — routing via %s", f.proxyURL)
		styler.FmtWarn(os.Stderr, "Ensure Tor daemon is running (tor.service or tor)")
	} else {
		styler.FmtInfo(os.Stderr, "Proxy: %s", f.proxyURL)
	}
}

func hasRawFlag(name string) bool {
	return slices.Contains(os.Args[1:], name)
}

func getToolBanner() string {
	raw := "" +
		"   ____  ____  ________  _   __\n" +
		"  / __ \\/ __ \\/  _/ __ \\| | / /\n" +
		" / / / / /_/ // // / / /  |/ / \n" +
		"/ /_/ / _, _// // /_/ / /|  /  \n" +
		"\\____/_/ |_/___/\\____/_/ |_/   "

	lines := strings.Split(raw, "\n")
	lines[len(lines)-1] += styler.Bold.Render(version)
	mw := 0
	for _, l := range lines {
		if len(l) > mw {
			mw = len(l)
		}
	}
	tw := terminalWidth()
	pad := 0
	if tw > mw {
		pad = (tw - mw) / 2
	}
	pre := strings.Repeat(" ", pad)
	for i, l := range lines {
		lines[i] = pre + l
	}
	return styler.BoldRed.Render(strings.Join(lines, "\n"))
}

func terminalWidth() int {
	if w := os.Getenv("COLUMNS"); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			return n
		}
	}
	return 80
}

func buildConfig(f cliFlags, domain string, resolvers []string, pl int) *types.Config {
	return &types.Config{
		Domain:           domain,
		DomainsFile:      f.domainFile,
		Threads:          f.threads,
		Timeout:          f.timeout,
		OutputFile:       f.outputFile,
		OutputDirectory:  f.outputDir,
		Silent:           f.silent,
		Verbose:          f.verbose,
		JSON:             f.json,
		HostIP:           f.hostIP,
		CaptureSources:   f.captureSources,
		RemoveWildcard:   f.removeWildcard || f.hostIP,
		Statistics:       f.statistics,
		Bruteforce:       f.bruteforce && !f.passiveOnly,
		Wordlist:         f.wordlist,
		Resolvers:        resolvers,
		PassiveOnly:      f.passiveOnly,
		Permute:          f.permute,
		PermuteLevel:     pl,
		NSECWalk:         f.nsecWalk,
		DoH:              f.doh,
		MaxWordlistSize:  f.maxWordlistSize,
		ProxyURL:         f.proxyURL,
		TorMode:          f.torMode,
		ProviderConfig:   f.providerConfig,
		RateLimit:        f.rateLimit,
		RateLimits:       parseRateLimits(f.rateLimits),
		Match:            splitCSV(f.match),
		Filter:           splitCSV(f.filter),
		Sources:          splitCSV(f.sources),
		ExcludeSources:   splitCSV(f.excludeSources),
		DNSPersecond:     f.dnsRate,
		DNSRetries:       f.dnsRetries,
		DNSTimeout:       f.dnsTimeout,
		ActiveTimeout:    f.activeTimeout,
		ProxyPool:        f.proxyPool,
		ProxyPoolMin:     f.proxyPoolMin,
		ProxyPoolRefresh: f.proxyPoolRefresh,
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func parseRateLimits(s string) map[string]types.RateLimitEntry {
	if s == "" {
		return nil
	}
	result := make(map[string]types.RateLimitEntry)
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		before, after, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		source := strings.TrimSpace(before)
		value := strings.TrimSpace(after)
		before, after, ok = strings.Cut(value, "/")
		if !ok {
			continue
		}
		count, err := strconv.Atoi(strings.TrimSpace(before))
		if err != nil || count <= 0 {
			continue
		}
		var duration time.Duration
		switch strings.TrimSpace(after) {
		case "ms":
			duration = time.Millisecond
		case "s":
			duration = time.Second
		case "m":
			duration = time.Minute
		case "h":
			duration = time.Hour
		case "d":
			duration = 24 * time.Hour
		default:
			continue
		}
		result[strings.ToLower(source)] = types.RateLimitEntry{
			MaxCount: uint(count),
			Duration: duration,
		}
	}
	return result
}

func parseFlags(args []string) cliFlags {
	var f cliFlags
	fs := flag.NewFlagSet("orion", flag.ContinueOnError)
	fs.Usage = printMainHelp
	fs.SetOutput(os.Stderr)

	fs.StringVar(&f.domain, "d", "", "Target domain")
	fs.StringVar(&f.domainFile, "dL", "", "File with list of domains")
	fs.StringVar(&f.outputFile, "o", "", "Output file (default: stdout)")
	fs.StringVar(&f.outputDir, "oD", "", "Output directory (creates per-domain files)")
	fs.IntVar(&f.threads, "t", defaultThreads, "Concurrent threads")
	fs.IntVar(&f.timeout, "timeout", 30, "HTTP timeout (seconds)")
	fs.BoolVar(&f.silent, "silent", false, "Suppress logs (subdomains only)")
	fs.BoolVar(&f.verbose, "v", false, "Verbose output")
	fs.BoolVar(&f.json, "json", false, "JSON output (one JSON object per line)")
	fs.BoolVar(&f.hostIP, "ip", false, "Include host IP in output (implies -nW)")
	fs.BoolVar(&f.captureSources, "cs", false, "Include sources in output")
	fs.BoolVar(&f.removeWildcard, "nW", false, "Remove wildcard/dead subdomains (requires DNS resolution)")
	fs.BoolVar(&f.statistics, "stats", false, "Show per-source statistics")
	fs.BoolVar(&f.listSources, "ls", false, "List all available sources")
	fs.BoolVar(&f.bruteforce, "b", true, "DNS brute-force")
	fs.StringVar(&f.wordlist, "w", "", "Custom wordlist file")
	fs.BoolVar(&f.passiveOnly, "passive", false, "Passive only (no brute-force)")
	fs.StringVar(&f.resolvers, "r", "1.1.1.1:53,8.8.8.8:53,9.9.9.9:53", "DNS resolvers (unused in DoH mode)")
	fs.BoolVar(&f.permute, "permute", false, "Enable permutation engine on discovered subs")
	fs.IntVar(&f.permuteLevel, "permute-level", 2, "Permutation level: 1=basic, 2=aggressive, 3=extreme")
	fs.BoolVar(&f.nsecWalk, "nsec", false, "Enable NSEC zone walking (queries target DNS directly)")
	fs.BoolVar(&f.doh, "doh", true, "Use DNS-over-HTTPS (default: on)")
	fs.IntVar(&f.maxWordlistSize, "max-words", 0, "Max wordlist entries (0=all)")
	fs.StringVar(&f.proxyURL, "proxy", "", "SOCKS5 proxy (e.g., socks5://127.0.0.1:9050)")
	fs.BoolVar(&f.torMode, "tor", false, "Route all traffic through Tor (127.0.0.1:9050)")
	fs.BoolVar(&f.noProxyPool, "no-proxy-pool", false, "Disable default rotating proxy pool; run all traffic directly")
	defaultProviderConfig := fmt.Sprintf(
		"Path to provider config file (default: ~/.config/%s/provider-config.yaml)",
		programName,
	)
	fs.StringVar(&f.providerConfig, "provider-config", "", defaultProviderConfig)
	fs.BoolVar(&f.deep, "deep", false, "Full deep scan: permute+nsec+max timeout")
	fs.StringVar(&f.sources, "sources", "", "Comma-separated list of sources to use")
	fs.StringVar(&f.excludeSources, "es", "", "Comma-separated list of sources to exclude")
	fs.StringVar(&f.match, "match", "", "Comma-separated patterns to match (glob-style)")
	fs.StringVar(&f.filter, "filter", "", "Comma-separated patterns to filter out (glob-style)")
	fs.IntVar(&f.rateLimit, "rl", 0, "Max HTTP requests per second (global)")
	fs.StringVar(&f.rateLimits, "rls", "", "Per-source rate limits (e.g., censys=10/m,leakix=10/m)")
	fs.IntVar(&f.dnsRate, "dns-rate", 0, "Max DNS lookups per second (0=auto)")
	fs.IntVar(&f.dnsRetries, "dns-retries", 2, "Retries per failed DNS lookup")
	fs.IntVar(&f.dnsTimeout, "dns-timeout", 5, "Per-request DNS timeout (seconds)")
	fs.IntVar(&f.activeTimeout, "active-timeout", 300, "Max duration for active scanning (seconds)")
	fs.BoolVar(&f.proxyPool, "proxy-pool", true, "Enable free rotating proxy pool for all traffic")
	fs.IntVar(&f.proxyPoolMin, "proxy-pool-min", 5, "Minimum proxies before starting scan")
	fs.IntVar(&f.proxyPoolRefresh, "proxy-pool-refresh", 5, "Proxy pool refresh interval (minutes)")
	fs.BoolVar(&f.showVersion, "version", false, "Show version")
	fs.BoolVar(&f.showHelp, "h", false, "Show help")
	fs.BoolVar(&f.showHelp, "help", false, "Show help")

	if err := fs.Parse(args); err != nil {
		if f.showHelp {
			printMainHelp()
			os.Exit(0)
		}
		styler.FmtError(os.Stderr, "%v", err)
		styler.Fprintf(os.Stderr, styler.Yellow, "Try '%s -h' for usage information.\n", programName)
		os.Exit(2)
	}
	if f.showHelp {
		printMainHelp()
		os.Exit(0)
	}
	return f
}

func loadDomains(domain, domainFile string) []string {
	var domains []string
	if domain != "" {
		domains = append(domains, domain)
	}
	if domainFile != "" {
		data, err := os.ReadFile(filepath.Clean(domainFile))
		if err != nil {
			styler.Fprint(os.Stderr, styler.BoldRed, "✗ Error reading domain file")
			fmt.Fprintf(os.Stderr, ": %v\n", err)
			os.Exit(1)
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			domains = append(domains, line)
		}
	}
	return domains
}

func runDoctorInner(args []string, f cliFlags) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.Usage = func() {
		printDoctorHelp()
	}
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "Output as JSON")
	verbose := fs.Bool("v", false, "Verbose output with details")
	help := fs.Bool("h", false, "Show help")
	help2 := fs.Bool("help", false, "Show help")

	if err := fs.Parse(args); err != nil {
		if *help || *help2 {
			printDoctorHelp()
			return 0
		}
		styler.FmtError(os.Stderr, "%v", err)
		styler.Fprintf(os.Stderr, styler.Yellow, "Try '%s doctor -h' for usage information.\n", programName)
		return 2
	}
	if *help || *help2 {
		printDoctorHelp()
		return 0
	}

	proxyURL := f.proxyURL
	if f.torMode && proxyURL == "" {
		proxyURL = torProxy
	}

	cfg := &types.Config{ProxyURL: proxyURL}
	opts := doctor.Options{Verbose: *verbose || f.verbose, JSON: *jsonOut}

	if !opts.JSON {
		setupDoctorStreaming(&opts)
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	results := doctor.Run(ctx, cfg, opts)
	return printDoctorResults(results, opts, time.Since(start))
}

func setupDoctorStreaming(opts *doctor.Options) {
	styler.Fprintf(os.Stderr, styler.Bold, "  %s doctor — full system diagnostics\n", programName)
	fmt.Fprintln(os.Stderr)

	var lastSection string
	var mu sync.Mutex
	opts.OnResult = func(r doctor.Result) {
		mu.Lock()
		defer mu.Unlock()
		if r.Section != lastSection {
			if lastSection != "" {
				fmt.Fprintln(os.Stderr)
			}
			fmt.Fprintf(os.Stderr, "  %s\n", styler.Bold.Render(r.Section))
			lastSection = r.Section
		}
		var icon string
		switch r.Status {
		case doctor.StatusPass:
			icon = styler.BoldGreen.Render("[PASS]")
		case doctor.StatusWarn:
			icon = styler.BoldYellow.Render("[WARN]")
		case doctor.StatusFail:
			icon = styler.BoldRed.Render("[FAIL]")
		}
		fmt.Fprintf(os.Stderr, "    %s %-20s %s\n", icon, r.Name, r.Message)
		if r.Detail != "" {
			for line := range strings.SplitSeq(r.Detail, "\n") {
				fmt.Fprintf(os.Stderr, "           %s\n", truncateStr(line, 80))
			}
		}
	}
}

func printDoctorResults(results []doctor.Result, opts doctor.Options, elapsed time.Duration) int {
	pass, warn, fail := doctor.CountResults(results)

	if opts.JSON {
		j, err := doctor.FormatJSON(results, pass, warn, fail)
		if err != nil {
			fmt.Fprintf(os.Stderr, "JSON output error: %v\n", err)
			fail++
		} else {
			fmt.Println(j)
		}
	} else {
		fmt.Fprintf(os.Stderr, "\n  %s: %d pass, %d warn, %d fail  (%s)\n",
			styler.Bold.Render("Summary"), pass, warn, fail, elapsed.Round(time.Millisecond))
		switch {
		case pass > 0 && warn == 0 && fail == 0:
			styler.Fprintln(os.Stderr, styler.Green, "  All systems operational")
		case fail > 0:
			styler.Fprintln(os.Stderr, styler.Red, "  Issues found that must be resolved")
		case warn > 0:
			styler.Fprintln(os.Stderr, styler.Yellow, "  Non-critical issues detected")
		}
	}

	switch {
	case fail > 0:
		return 2
	case warn > 0:
		return 1
	}
	return 0
}

func truncateStr(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
