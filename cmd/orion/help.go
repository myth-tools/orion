package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/myth-tools/orion/internal/passive"
	"github.com/myth-tools/orion/internal/styler"
)

func note(w *os.File, text string) {
	fmt.Fprintf(w, "  %s %s\n", styler.Cyan.Render("•"), styler.Dim.Render(text))
}

func printMainHelpIntro(w *os.File) {
	styler.Fprintln(w, styler.Bold, "  Fast, multi-threaded subdomain enumeration")
	fmt.Fprintln(w, "  combining passive intelligence from 38 sources with")
	fmt.Fprintln(w, "  active DNS brute-forcing, permutation engines, and NSEC")
	fmt.Fprintln(w, "  zone walking.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, styler.Section("Quick Start"))
	fmt.Fprintf(w, "  %s %s -d example.com -deep%s\n",
		styler.Green.Render("$"), programName, styler.Dim.Render("          one-command deep scan"))
	fmt.Fprintf(w, "  %s %s -d example.com%s\n",
		styler.Green.Render("$"), programName, styler.Dim.Render("              passive + brute with defaults"))
	fmt.Fprintf(w, "  %s %s -d example.com -passive%s\n",
		styler.Green.Render("$"), programName, styler.Dim.Render("       passive sources only"))
	fmt.Fprintf(w, "  %s %s doctor%s\n\n",
		styler.Green.Render("$"), programName, styler.Dim.Render("                        check your environment"))
}

func printMainHelpFooter(w *os.File) {
	fmt.Fprintln(w, styler.Section("Important Notes"))
	note(w, "NSEC walking (-nsec) queries the target's authoritative DNS — your source IP is visible without a proxy")
	note(w, "Use -tor or -proxy to anonymize all traffic; without it DNS queries reveal your IP")
	note(w, "The default wordlist is fast but shallow; use -w with a larger list for deeper coverage")
	note(w, "The -deep flag enables permute (level 3) + NSEC + Tor + 60s timeout — one-flag comprehensive scan")
	note(w, "Use -rl (global) or -rls (per-source) to throttle request rates proactively")
	note(w, "Rate-limited responses (429) trigger automatic exponential backoff with Retry-After")
	fmt.Fprintln(w)

	fmt.Fprintln(w, styler.Section("Environment Variables"))
	configPath := passive.DefaultConfigPath()
	if configPath == "" {
		configPath = "~/.config/" + programName + "/provider-config.yaml"
	}
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("API keys are read from the provider config file:"))
	fmt.Fprintf(w, "    %s\n", styler.Yellow.Render(configPath))
	fmt.Fprintf(w, "  %s\n\n", styler.Dim.Render("Fill in keys for the sources you want to use; all others are optional."))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Environment variables (override config file):"))
	fmt.Fprintf(w, "    %s     VirusTotal API key for certificate and domain lookup\n", styler.Yellow.Render("VIRUSTOTAL_API_KEY"))
	fmt.Fprintf(w, "    %s    CertSpotter API key for certificate transparency logs\n", styler.Yellow.Render("CERTSPOTTER_API_KEY"))
	fmt.Fprintf(w, "    %s           GitHub API key for code search (higher rate limits)\n\n", styler.Yellow.Render("GITHUB_TOKEN"))
}

func printMainHelp() {
	w := os.Stderr

	note(w, fmt.Sprintf("Run %s to verify your setup before scanning.", styler.Yellow.Render(programName+" doctor")))
	fmt.Fprintln(w)

	printMainHelpIntro(w)
	printMainHelpUsage(w)
	printMainHelpRequired(w)
	printMainHelpOutput(w)
	printMainHelpSources(w)
	printMainHelpDiscovery(w)
	printMainHelpDNS(w)
	printMainHelpProxy(w)
	printMainHelpRateLimit(w)
	printMainHelpMeta(w)
	printMainHelpSubcommands(w)
	printMainHelpExitCodes(w)
	printMainHelpFooter(w)
	printMainExamples(w)

	fmt.Fprintf(w, "  %s\n",
		styler.Sprintf(styler.Dim, "Built with Go %s · %d CPU cores detected · %d passive sources",
			runtime.Version()[2:], runtime.NumCPU(), len(passive.AllSources(nil))))
}

func printMainHelpExitCodes(w *os.File) {
	fmt.Fprintln(w, styler.Section("Exit Codes"))
	fmt.Fprintf(w, "    %s   All checks passed — scan completed without errors\n", styler.Green.Render("0"))
	fmt.Fprintf(w, "    %s   Warnings present — non-critical issues (rate limits, slow sources)\n", styler.Yellow.Render("1"))
	fmt.Fprintf(w, "    %s   Fatal error — bad flags, runtime failure, or network unreachable\n\n", styler.Red.Render("2"))
}

func printMainExamples(w *os.File) {
	p := programName
	fmt.Fprintln(w, styler.Section("Examples"))
	fmt.Fprintln(w)

	exampleGroup(
		w, "Basic enumeration",
		p+" -d example.com",
		p+" -d example.com -o results.txt",
		p+" -dL domains.txt",
		p+" -dL targets.txt -o all-results.txt",
	)

	exampleGroup(
		w, "Deep mode — one flag, full coverage + anonymity",
		p+" -d example.com -deep",
		p+" -d example.com -deep -o results.txt",
		p+" -dL domains.txt -deep -silent",
	)

	exampleGroup(
		w, "Passive only (no contact with target DNS)",
		p+" -d example.com -passive",
		p+" -d example.com -passive -permute",
		p+" -d example.com -passive -permute -o passive-only.txt",
	)

	exampleGroup(
		w, "Brute-force with custom wordlist",
		p+" -d example.com -w ~/wordlists/big.txt",
		p+" -d example.com -w custom.txt -max-words 10000",
		p+" -d example.com -w ~/lists/all.txt -max-words 50000 -t 500",
	)

	exampleGroup(
		w, "Permutation engine (learns + guesses patterns)",
		p+" -d example.com -permute",
		p+" -d example.com -permute -permute-level 3",
		p+" -d example.com -deep -permute-level 2",
	)

	exampleGroup(
		w, "NSEC zone walking (DNSSEC-enabled targets)",
		p+" -d example.com -nsec",
		p+" -d example.com -nsec -proxy socks5://127.0.0.1:9050",
	)

	exampleGroup(
		w, "Custom resolvers and DNS-over-HTTPS",
		p+" -d example.com -doh=true",
		p+" -d example.com -doh=false -r 8.8.8.8:53,1.1.1.1:53",
		p+" -d example.com -t 200 -timeout 60",
	)

	exampleGroup(
		w, "Tor / proxy — anonymous scanning",
		p+" -d example.com -tor",
		p+" -d example.com -proxy socks5://127.0.0.1:9050",
		p+" -d example.com -tor -deep",
	)

	printMainExamplesAdvanced(w)
	fmt.Fprintln(w)
}

func printMainExamplesAdvanced(w *os.File) {
	p := programName
	exampleGroup(
		w, "Automation and piping",
		p+" -d example.com -silent | tee -a results.txt",
		p+" -d example.com -silent -passive | sort -u | httpx",
		p+" -d example.com -silent -o results.txt && wc -l results.txt",
	)

	exampleGroup(
		w, "High-performance / production scan",
		p+" -d example.com -deep -t 500 -timeout 120",
		p+" -dL domains.txt -deep -silent -o bulk-results.txt",
		p+" -d example.com -w huge.txt -max-words 100000 -permute -nsec -tor",
	)

	exampleGroup(
		w, "Diagnostics and troubleshooting",
		p+" doctor",
		p+" doctor -v",
		p+" doctor -json",
		p+" doctor -proxy socks5://127.0.0.1:9050",
	)
}

func printMainHelpUsage(w *os.File) {
	fmt.Fprintln(w, styler.Section("Usage"))
	fmt.Fprintf(w, "    %s %s %s%s\n",
		styler.Yellow.Render(programName),
		styler.Dim.Render("[FLAGS]"),
		styler.Green.Render("-d <domain>"),
		styler.Dim.Render("           Single target"))
	fmt.Fprintf(w, "    %s %s %s%s\n",
		styler.Yellow.Render(programName), styler.Dim.Render("[FLAGS]"), styler.Green.Render("-dL <domains.txt>"),
		styler.Dim.Render("      Batch targets, one per line"))
	fmt.Fprintf(w, "    %s %s %s %s%s\n",
		styler.Yellow.Render(programName), styler.Dim.Render("[FLAGS]"), styler.Green.Render("-d <dom>"),
		styler.Green.Render("-o <file>"), styler.Dim.Render("  Output to file"))
	fmt.Fprintf(w, "    %s doctor %s%s\n\n",
		styler.Yellow.Render(programName), styler.Dim.Render("[FLAGS]"), styler.Dim.Render("               Run diagnostics"))
}

func printMainHelpRequired(w *os.File) {
	fmt.Fprintln(w, styler.Section("Required  %s", styler.Dim.Render("(one of)")))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Pick one target source:"))
	printFlag(w, "-d <domain>", "Target domain to enumerate (e.g. example.com)")
	printFlag(w, "-dL <file>", "Path to file with domains, one per line; lines starting with # are skipped")
	fmt.Fprintln(w)
}

func printMainHelpOutput(w *os.File) {
	fmt.Fprintln(w, styler.Section("Output Control"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("By default, found subdomains are written to stdout — pipe to a file or tool."))
	printFlag(w, "-o <file>", "Write results to file instead of stdout; creates or overwrites")
	printFlag(w, "-silent", "Suppress banner and progress; emit only discovered subdomains")
	printFlag(w, "-v", "Show verbose per-source logging and discovery progress")
	fmt.Fprintln(w)

	fmt.Fprintln(w, styler.Section("Output Format"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Control how results are displayed and stored:"))
	printFlag(w, "-json", "NDJSON output: one JSON object per subdomain on stdout")
	printFlag(w, "-ip", "Include resolved IPs in output (implies -nW)")
	printFlag(w, "-cs", "Show source name(s) that discovered each subdomain")
	printFlag(w, "-stats", "Print per-source statistics after scan")
	printFlagl(w, "-oD", "<dir>", "Output directory — creates per-domain files there", "")
	printFlag(w, "-nW", "Remove wildcard/dead subdomains via DNS resolution")
	fmt.Fprintln(w)
}

func printMainHelpSources(w *os.File) {
	fmt.Fprintln(w, styler.Section("Source Selection"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Filter which passive sources are used:"))
	printFlag(w, "-ls", "List all available passive sources and exit")
	printFlagl(w, "-provider-config", "<path>", "Config file for API keys", fmt.Sprintf("~/.config/%s/provider-config.yaml", programName))
	printFlagl(w, "-sources", "<list>", "Comma-separated source names to use (default: all)", "all")
	printFlagl(w, "-es", "<list>", "Comma-separated source names to exclude", "")
	printFlagl(w, "-match", "<pattern>", "Glob patterns — only include matching subdomains", "")
	printFlagl(w, "-filter", "<pattern>", "Glob patterns — exclude matching subdomains", "")
	fmt.Fprintln(w)
}

func printMainHelpDiscovery(w *os.File) {
	fmt.Fprintln(w, styler.Section("Discovery Techniques"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Control what methods are used to find subdomains:"))
	printFlag(w, "-b", dft("DNS brute-force: test every word in wordlist against target", "true"))
	printFlag(w, "-passive", "Disable active techniques; gather from 38 internet sources only")
	printFlag(w, "-permute", "Learn naming patterns from found subdomains and generate variations")
	printFlagl(w, "-permute-level", "<n>", "Permutation depth: 1=basic (prefix/suffix), 2=aggressive (leet+swap), 3=extreme (all combos)", "2")
	printFlag(w, "-nsec", "NSEC zone walk: query target's DNS for DNSSEC next-secure records")
	fmt.Fprintln(w)
}

func printMainHelpDNS(w *os.File) {
	fmt.Fprintln(w, styler.Section("DNS & Performance"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Fine-tune resolution speed, concurrency, and resolver selection:"))
	printFlagr(w, "-r", "<list>", "Custom DNS resolvers, comma-separated (host:port)", "1.1.1.1:53,8.8.8.8:53,9.9.9.9:53")
	printFlag(w, "-doh", dft("Resolve via DNS-over-HTTPS (10 providers, round-robin); falls back to UDP if unavailable", "true"))
	printFlagt(w, "-timeout", "Max seconds to wait per source HTTP request", "30")
	printFlagt(w, "-t", "Number of concurrent goroutines for brute-force and permutation", "NumCPU×10")
	printFlag(w, "-w <file>", "Custom wordlist path for brute-force; overrides embedded default")
	printFlagt(w, "-max-words", "Limit wordlist to first N entries (0 = all)", "0")
	printFlagt(w, "-dns-rate", "Max DNS lookups per second (0 = auto)", "0 (auto)")
	printFlagt(w, "-dns-timeout", "Per-request DNS timeout (seconds)", "5")
	printFlagt(w, "-dns-retries", "Retries per failed DNS lookup", "2")
	printFlagt(w, "-active-timeout", "Max seconds for brute+permute phase", "300")
	fmt.Fprintln(w)
}

func printMainHelpProxy(w *os.File) {
	fmt.Fprintln(w, styler.Section("Proxy & Anonymity"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Route traffic through a SOCKS5 proxy or Tor for anonymity:"))
	printFlag(w, "-proxy <url>", "SOCKS5 proxy URL (e.g. socks5://127.0.0.1:9050)")
	printFlag(w, "-tor", "Enable Tor mode: proxy defaults to 127.0.0.1:9050; circuits rotate per request")
	fmt.Fprintf(w, "\n  %s\n", styler.Dim.Render("Tor mode requires the Tor daemon running (systemctl start tor)."))
	fmt.Fprintln(w)

	fmt.Fprintln(w, styler.Section("Rotating Proxy Pool"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Free rotating proxy pool for ALL traffic (enabled by default):"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Scrapes free HTTP/SOCKS5 proxies from 18 public lists,"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("tests them concurrently with a 3s timeout, and rotates through"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("working proxies in a thread-safe round-robin. All HTTP"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("requests, DNS queries, and NSEC walk traffic are routed"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("through the proxy pool. Dead proxies are detected and removed."))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Use --no-proxy-pool to disable and run all traffic directly."))
	printFlag(w, "-proxy-pool", "Enable rotating free proxy pool (default: on)")
	printFlagt(w, "-proxy-pool-min", "Minimum working proxies before scan starts", "5")
	printFlagt(w, "-proxy-pool-refresh", "Proxy pool refresh interval (minutes)", "5")
	printFlag(w, "-no-proxy-pool", "Disable default rotating proxy pool; run all traffic directly")
	fmt.Fprintln(w)
}

func printMainHelpRateLimit(w *os.File) {
	fmt.Fprintln(w, styler.Section("Rate Limiting"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Throttle HTTP requests to avoid being blocked:"))
	printFlagt(w, "-rl", "Global max HTTP requests/sec per source (0 = use per-source defaults)", "0")
	printFlagl(w, "-rls", "<spec>", "Per-source overrides: src=count/dur,src2=count/dur", "")
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("    Durations: ms/s/m/h/d  (e.g., hackertarget=10/m,censys=10/m)"))
	fmt.Fprintln(w)
}

func printMainHelpMeta(w *os.File) {
	fmt.Fprintln(w, styler.Section("Meta Flags"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Tool information and convenience flags:"))
	printFlag(w, "-deep", "Full deep scan: permute (level 3) + NSEC + Tor + 60s timeout — one-flag comprehensive")
	printFlag(w, "-version", "Print version string and exit")
	printFlag(w, "-h, -help", "Display this help message")
	fmt.Fprintln(w)
}

func printMainHelpSubcommands(w *os.File) {
	fmt.Fprintln(w, styler.Section("Subcommands"))
	fmt.Fprintf(w, "    %s%s %s\n",
		styler.Yellow.Render("doctor"),
		styler.Dim.Render("           Run full system diagnostics  "),
		styler.Dim.Render("("+programName+" doctor -h)"))
	fmt.Fprintf(w, "                    %s %s\n",
		styler.Bold.Render("Checks:"), styler.Dim.Render("Go env, build, passive sources, DoH, DNS,"))
	fmt.Fprintln(w, "                    network, Tor, wordlist, permissions, concurrency")
	fmt.Fprintln(w)
}

func printDoctorHelp() {
	doctorChecks := []string{
		"Platform & Go runtime",
		"Go version, go.mod, dependencies, build",
		"Working/temp directory permissions, file handles",
		"External tools: gofumpt, staticcheck, gosec, golangci-lint, shadow",
		"All 11 passive sources reachable",
		"DoH resolvers (Cloudflare, Google)",
		"System DNS, IPv4, Internet, Tor daemon",
		"DNS benchmark (system vs DoH)",
		"18 proxy pool sources (per-source: reachable + count)",
		"Proxy pool total candidates and working proxies",
		"Built-in wordlist integrity",
		"Conflicting flag / configuration detection",
		"Proxy connectivity (if configured)",
	}

	doctorExamples := []string{
		programName + " doctor",
		programName + " doctor -v",
		programName + " doctor -json",
		programName + " doctor -proxy socks5://127.0.0.1:9050",
	}

	w := os.Stderr

	styler.Fprintf(w, styler.Bold, "  Comprehensive system diagnostics for %s\n", programName)
	fmt.Fprintln(w, "  Verifies every component is working before you run a real scan —")
	fmt.Fprintln(w, "  good for CI pipelines, new installations, and troubleshooting.")
	fmt.Fprintln(w)

	fmt.Fprintln(w, styler.Section("Usage"))
	fmt.Fprintf(w, "    %s doctor %s\n\n", styler.Yellow.Render(programName), styler.Dim.Render("[FLAGS]"))

	fmt.Fprintln(w, styler.Section("Doctor Flags"))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("These are specific to the doctor subcommand:"))
	printFlag(w, "-h, -help", "Show this help message")
	printFlag(w, "-json", "Output results as structured JSON for programmatic consumption")
	printFlag(w, "-v", "Print detailed per-check results alongside the summary")
	fmt.Fprintln(w)

	fmt.Fprintln(w, styler.Section("Global Flags  %s", styler.Dim.Render("(accepted by doctor)")))
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("These work the same as in the main command:"))
	printFlag(w, "-proxy <url>", "SOCKS5 proxy URL for network tests (e.g. socks5://127.0.0.1:9050)")
	printFlag(w, "-tor", "Test Tor connectivity (proxy defaults to 127.0.0.1:9050)")
	printFlag(w, "-silent", "Suppress the banner header")
	fmt.Fprintln(w)

	fmt.Fprintln(w, styler.Section("Checks Performed"))
	fmt.Fprintf(w, "  %s\n",
		styler.Sprintf(styler.Dim, "Each check reports %s / %s / %s:",
			styler.Green.Render("PASS"),
			styler.Yellow.Render("WARN"),
			styler.Red.Render("FAIL")))
	for _, c := range doctorChecks {
		fmt.Fprintf(w, "    %s %s\n", styler.Cyan.Render("•"), c)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, styler.Section("Exit Codes"))
	fmt.Fprintf(w, "    %s   All checks passed — system is ready\n", styler.Green.Render("0"))
	fmt.Fprintf(w, "    %s   Warnings (non-critical: rate limits, optional tools missing)\n", styler.Yellow.Render("1"))
	fmt.Fprintf(w, "    %s   Failures — one or more components broken or unreachable\n\n", styler.Red.Render("2"))

	fmt.Fprintln(w, styler.Section("Examples"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s\n", styler.Dim.Render("Run before every first scan to confirm your environment:"))
	for _, ex := range doctorExamples {
		fmt.Fprintf(w, "    %s %s\n", styler.Green.Render("$"), ex)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s\n",
		styler.Sprintf(styler.Dim, "Built with Go %s · %d CPU cores detected",
			runtime.Version()[2:], runtime.NumCPU()))
}

func printFlag(w *os.File, name, desc string) {
	fmt.Fprintf(w, "    %s    %s\n", styler.Yellow.Render(name), styler.Dim.Render(desc))
}

func printFlagl(w *os.File, name, param, desc, dflt string) {
	fmt.Fprintf(w, "    %s    %s  %s\n",
		styler.Yellow.Render(name+" "+styler.Magenta.Render(param)),
		styler.Dim.Render(desc),
		styler.Sprintf(styler.Dim, "(default: %s)", dflt))
}

func printFlagt(w *os.File, name, desc, dflt string) {
	fmt.Fprintf(w, "    %s %s    %s  %s\n",
		styler.Yellow.Render(name), styler.Magenta.Render("<n>"),
		styler.Dim.Render(desc),
		styler.Sprintf(styler.Dim, "(default: %s)", dflt))
}

func printFlagr(w *os.File, name, param, desc, dflt string) {
	fmt.Fprintf(w, "    %s    %s  %s\n",
		styler.Yellow.Render(name+" "+styler.Magenta.Render(param)),
		styler.Dim.Render(desc),
		styler.Sprintf(styler.Dim, "(default: %s)", dflt))
}

func dft(desc, dflt string) string {
	return fmt.Sprintf("%s  %s", desc, styler.Sprintf(styler.Dim, "(default: %s)", dflt))
}

func exampleGroup(w *os.File, title string, cmds ...string) {
	fmt.Fprintf(w, "  %s %s\n", styler.Dim.Render(strings.Repeat("╌", 3)), styler.Bold.Render(title))
	for _, c := range cmds {
		fmt.Fprintf(w, "    %s %s\n", styler.Green.Render("$"), c)
	}
	fmt.Fprintln(w)
}
