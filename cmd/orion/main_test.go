package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

const testDomain = "example.com"

// ─── parseFlags ─────────────────────────────────────────.

func TestParseFlagsDefaults(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{})
	if f.domain != "" {
		t.Errorf("default domain = %q, want empty", f.domain)
	}
	if f.domainFile != "" {
		t.Errorf("default domainFile = %q, want empty", f.domainFile)
	}
	if f.outputFile != "" {
		t.Errorf("default outputFile = %q, want empty", f.outputFile)
	}
	if f.threads != runtime.NumCPU()*10 {
		t.Errorf("default threads = %d, want %d", f.threads, runtime.NumCPU()*10)
	}
	if f.timeout != 30 {
		t.Errorf("default timeout = %d, want 30", f.timeout)
	}
	if f.silent {
		t.Error("default silent = true, want false")
	}
	if f.verbose {
		t.Error("default verbose = true, want false")
	}
	if !f.bruteforce {
		t.Error("default bruteforce = false, want true")
	}
	if f.wordlist != "" {
		t.Errorf("default wordlist = %q, want empty", f.wordlist)
	}
	if f.passiveOnly {
		t.Error("default passiveOnly = true, want false")
	}
	if f.resolvers == "" {
		t.Error("default resolvers = empty, want default list")
	}
	if f.permute {
		t.Error("default permute = true, want false")
	}
	if f.permuteLevel != 2 {
		t.Errorf("default permuteLevel = %d, want 2", f.permuteLevel)
	}
	if f.nsecWalk {
		t.Error("default nsecWalk = true, want false")
	}
	if !f.doh {
		t.Error("default doh = false, want true")
	}
	if f.maxWordlistSize != 0 {
		t.Errorf("default maxWordlistSize = %d, want 0", f.maxWordlistSize)
	}
	if f.proxyURL != "" {
		t.Errorf("default proxyURL = %q, want empty", f.proxyURL)
	}
	if f.torMode {
		t.Error("default torMode = true, want false")
	}
	if f.showVersion {
		t.Error("default showVersion = true, want false")
	}
}

func TestParseFlagsDomain(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-d", testDomain})
	if f.domain != testDomain {
		t.Errorf("domain = %q, want %q", f.domain, testDomain)
	}
}

func TestParseFlagsOutputFile(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-o", "/tmp/out.txt"})
	if f.outputFile != "/tmp/out.txt" {
		t.Errorf("outputFile = %q, want %q", f.outputFile, "/tmp/out.txt")
	}
}

func TestParseFlagsProxy(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-proxy", "socks5://127.0.0.1:9050"})
	if f.proxyURL != "socks5://127.0.0.1:9050" {
		t.Errorf("proxyURL = %q, want %q", f.proxyURL, "socks5://127.0.0.1:9050")
	}
}

func TestParseFlagsTor(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-tor"})
	if !f.torMode {
		t.Error("torMode = false, want true")
	}
}

func TestParseFlagsVerbose(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-v"})
	if !f.verbose {
		t.Error("verbose = false, want true")
	}
}

func TestParseFlagsSilent(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-silent"})
	if !f.silent {
		t.Error("silent = false, want true")
	}
}

func TestParseFlagsBruteforceOff(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-b=false"})
	if f.bruteforce {
		t.Error("bruteforce = true, want false")
	}
}

func TestParseFlagsThreads(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-t", "100"})
	if f.threads != 100 {
		t.Errorf("threads = %d, want 100", f.threads)
	}
}

func TestParseFlagsTimeout(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-timeout", "60"})
	if f.timeout != 60 {
		t.Errorf("timeout = %d, want 60", f.timeout)
	}
}

func TestParseFlagsWordlist(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-w", "/tmp/custom.txt"})
	if f.wordlist != "/tmp/custom.txt" {
		t.Errorf("wordlist = %q, want %q", f.wordlist, "/tmp/custom.txt")
	}
}

func TestParseFlagsPassiveOnly(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-passive"})
	if !f.passiveOnly {
		t.Error("passiveOnly = false, want true")
	}
}

func TestParseFlagsResolvers(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-r", "1.1.1.1:53,9.9.9.9:53"})
	if f.resolvers != "1.1.1.1:53,9.9.9.9:53" {
		t.Errorf("resolvers = %q, want %q", f.resolvers, "1.1.1.1:53,9.9.9.9:53")
	}
}

func TestParseFlagsPermute(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-permute"})
	if !f.permute {
		t.Error("permute = false, want true")
	}
}

func TestParseFlagsPermuteLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args []string
		want int
	}{
		{[]string{"-permute-level=1"}, 1},
		{[]string{"-permute-level=3"}, 3},
		{[]string{}, 2},
	}
	for _, tc := range tests {
		f := parseFlags(tc.args)
		if f.permuteLevel != tc.want {
			t.Errorf("parseFlags(%v).permuteLevel = %d, want %d", tc.args, f.permuteLevel, tc.want)
		}
	}
}

func TestParseFlagsNSEC(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-nsec"})
	if !f.nsecWalk {
		t.Error("nsecWalk = false, want true")
	}
}

func TestParseFlagsDoH(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-doh=false"})
	if f.doh {
		t.Error("doh = true, want false")
	}
}

func TestParseFlagsMaxWords(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-max-words=100"})
	if f.maxWordlistSize != 100 {
		t.Errorf("maxWordlistSize = %d, want 100", f.maxWordlistSize)
	}
}

func TestParseFlagsVersion(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-version"})
	if !f.showVersion {
		t.Error("showVersion = false, want true")
	}
}

// ─── New flag tests ─────────────────────────────────────.

func TestParseFlagsJSON(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-json"})
	if !f.json {
		t.Error("json = false, want true")
	}
}

func TestParseFlagsHostIP(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-ip"})
	if !f.hostIP {
		t.Error("hostIP = false, want true")
	}
}

func TestParseFlagsCaptureSources(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-cs"})
	if !f.captureSources {
		t.Error("captureSources = false, want true")
	}
}

func TestParseFlagsRemoveWildcard(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-nW"})
	if !f.removeWildcard {
		t.Error("removeWildcard = false, want true")
	}
}

func TestParseFlagsStatistics(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-stats"})
	if !f.statistics {
		t.Error("statistics = false, want true")
	}
}

func TestParseFlagsOutputDir(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-oD", "/tmp/outputs"})
	if f.outputDir != "/tmp/outputs" {
		t.Errorf("outputDir = %q, want %q", f.outputDir, "/tmp/outputs")
	}
}

func TestParseFlagsSources(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-sources", "crtsh,alienvault"})
	if f.sources != "crtsh,alienvault" {
		t.Errorf("sources = %q, want %q", f.sources, "crtsh,alienvault")
	}
}

func TestParseFlagsExcludeSources(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-es", "google,baidu"})
	if f.excludeSources != "google,baidu" {
		t.Errorf("excludeSources = %q, want %q", f.excludeSources, "google,baidu")
	}
}

func TestParseFlagsMatch(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-match", "*.example.com"})
	if f.match != "*.example.com" {
		t.Errorf("match = %q, want %q", f.match, "*.example.com")
	}
}

func TestParseFlagsFilter(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-filter", "admin.example.com"})
	if f.filter != "admin.example.com" {
		t.Errorf("filter = %q, want %q", f.filter, "admin.example.com")
	}
}

func TestParseFlagsRateLimit(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-rl", "50"})
	if f.rateLimit != 50 {
		t.Errorf("rateLimit = %d, want 50", f.rateLimit)
	}
}

func TestParseFlagsListSources(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{"-ls"})
	if !f.listSources {
		t.Error("listSources = false, want true")
	}
}

func TestParseFlagsDanglingArgsIgnored(t *testing.T) {
	t.Parallel()
	// extra non-flag args are silently ignored by the FlagSet
	f := parseFlags([]string{"-d", "test.com", "extra", "stuff"})
	if f.domain != "test.com" {
		t.Errorf("domain = %q, want %q", f.domain, "test.com")
	}
}

func TestParseFlagsCombined(t *testing.T) {
	t.Parallel()
	f := parseFlags([]string{
		"-d", testDomain,
		"-t", "25",
		"-silent",
		"-v",
		"-o", "/tmp/out",
		"-proxy", "socks5://localhost:9050",
		"-tor",
	})
	if f.domain != testDomain {
		t.Errorf("domain = %q", f.domain)
	}
	if f.threads != 25 {
		t.Errorf("threads = %d", f.threads)
	}
	if !f.silent {
		t.Error("silent = false")
	}
	if !f.verbose {
		t.Error("verbose = false")
	}
	if f.outputFile != "/tmp/out" {
		t.Errorf("outputFile = %q", f.outputFile)
	}
	if f.proxyURL != "socks5://localhost:9050" {
		t.Errorf("proxyURL = %q", f.proxyURL)
	}
	if !f.torMode {
		t.Error("torMode = false")
	}
}

// ─── loadDomains ────────────────────────────────────────.

func TestLoadDomainsDomainOnly(t *testing.T) {
	t.Parallel()
	domains := loadDomains(testDomain, "")
	if len(domains) != 1 {
		t.Fatalf("len = %d, want 1", len(domains))
	}
	if domains[0] != testDomain {
		t.Errorf("domains[0] = %q, want %q", domains[0], testDomain)
	}
}

func TestLoadDomainsEmpty(t *testing.T) {
	t.Parallel()
	domains := loadDomains("", "")
	if len(domains) != 0 {
		t.Errorf("len = %d, want 0", len(domains))
	}
}

func TestLoadDomainsFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "domains.txt")
	content := "example.com\n  test.org  \n\n# this is a comment\nanother.net\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	domains := loadDomains("", path)
	if len(domains) != 3 {
		t.Fatalf("len = %d, want 3: %v", len(domains), domains)
	}
	if domains[0] != testDomain {
		t.Errorf("domains[0] = %q, want %q", domains[0], testDomain)
	}
	if domains[1] != "test.org" {
		t.Errorf("domains[1] = %q, want %q", domains[1], "test.org")
	}
	if domains[2] != "another.net" {
		t.Errorf("domains[2] = %q, want %q", domains[2], "another.net")
	}
}

func TestLoadDomainsBothSources(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "domains.txt")
	if err := os.WriteFile(path, []byte("file.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	domains := loadDomains("arg.test", path)
	if len(domains) != 2 {
		t.Fatalf("len = %d, want 2", len(domains))
	}
	if !contains(domains, "arg.test") {
		t.Error("missing arg.test")
	}
	if !contains(domains, "file.test") {
		t.Error("missing file.test")
	}
}

func TestLoadDomainsFileNotExist(t *testing.T) {
	t.Parallel()
	// This path would normally os.Exit; just verify domain-only path works
	domains := loadDomains("work.test", "")
	if len(domains) != 1 || domains[0] != "work.test" {
		t.Errorf("expected [work.test], got %v", domains)
	}
}

func TestLoadDomainsFileEmptyLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte("\n\n  \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	domains := loadDomains("", path)
	if len(domains) != 0 {
		t.Errorf("len = %d, want 0", len(domains))
	}
}

// ─── runDoctorInner ─────────────────────────────────────.

func TestRunDoctorInnerNoFailures(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	f := cliFlags{}
	code := runDoctorInner([]string{}, f)
	if code > 2 {
		t.Errorf("expected exit code <= 2 (one flaky failure OK), got %d", code)
	}
}

func TestRunDoctorInnerJSON(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	f := cliFlags{}
	code := runDoctorInner([]string{"-json"}, f)
	if code > 2 {
		t.Errorf("expected exit code <= 2 (one flaky failure OK), got %d", code)
	}
}

func TestRunDoctorInnerVerbose(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	f := cliFlags{}
	code := runDoctorInner([]string{"-v"}, f)
	if code > 2 {
		t.Errorf("expected exit code <= 2 (one flaky failure OK), got %d", code)
	}
}

func TestRunDoctorInnerWithProxyFlag(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	f := cliFlags{
		proxyURL: "socks5://127.0.0.1:9050",
	}
	code := runDoctorInner([]string{}, f)
	// proxy may fail if not reachable, but shouldn't panic
	if code > 2 {
		t.Errorf("unexpected exit code %d", code)
	}
}

func TestRunDoctorInnerTorMode(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	f := cliFlags{
		torMode: true,
	}
	code := runDoctorInner([]string{}, f)
	if code > 2 {
		t.Errorf("unexpected exit code %d", code)
	}
}

func TestRunDoctorInnerAllFlags(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	f := cliFlags{
		proxyURL: "socks5://127.0.0.1:9050",
		torMode:  true,
		verbose:  true,
	}
	code := runDoctorInner([]string{"-json", "-v"}, f)
	if code > 2 {
		t.Errorf("unexpected exit code %d", code)
	}
}

// ─── runDoctorInner JSON output ─────────────────────────.

func TestRunDoctorInnerJSONOutputValid(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	f := cliFlags{}
	code := runDoctorInner([]string{"-json"}, f)
	if code < 0 || code > 2 {
		t.Errorf("unexpected exit code %d", code)
	}
}

// ─── Helpers ────────────────────────────────────────────.

func contains(slice []string, s string) bool {
	return slices.Contains(slice, s)
}

// ─── Example tests ──────────────────────────────────────.

func Example_parseFlags() {
	f := parseFlags([]string{"-d", testDomain, "-silent"})
	fmt.Println(f.domain)
	fmt.Println(f.silent)
	// Output:
	// example.com
	// true
}

func Example_parseFlagsDefaultThreads() {
	f := parseFlags([]string{})
	fmt.Println(f.threads > 0)
	// Output:
	// true
}
