package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/myth-tools/orion/internal/types"
)

// ─── Status ─────────────────────────────────────────────.

func TestStatusString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s    Status
		want string
	}{
		{StatusPass, "PASS"},
		{StatusWarn, "WARN"},
		{StatusFail, "FAIL"},
		{Status(99), "UNKN"},
	}
	for _, tc := range tests {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("Status(%d).String() = %q, want %q", tc.s, got, tc.want)
		}
	}
}

func TestStatusMarshalText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s    Status
		want string
	}{
		{StatusPass, "PASS"},
		{StatusWarn, "WARN"},
		{StatusFail, "FAIL"},
	}
	for _, tc := range tests {
		b, err := tc.s.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText: %v", err)
		}
		if string(b) != tc.want {
			t.Errorf("MarshalText = %q, want %q", string(b), tc.want)
		}
	}
}

func TestStatusJSONMarshal(t *testing.T) {
	t.Parallel()
	r := Result{Name: "test", Status: StatusWarn}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Contains(data, []byte(`"WARN"`)) {
		t.Errorf("expected WARN in JSON, got %s", data)
	}
}

// ─── parseGoVersion ─────────────────────────────────────.

func TestParseGoVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  [3]int
	}{
		{"go1.21", [3]int{1, 21, 0}},
		{"go1.22.3", [3]int{1, 22, 3}},
		{"go1.26.4", [3]int{1, 26, 4}},
		{"go1.0", [3]int{1, 0, 0}},
		{"go1.0.0", [3]int{1, 0, 0}},
		{"go1", [3]int{1, 0, 0}},
		{"go1.255", [3]int{1, 255, 0}},
		{"", [3]int{0, 0, 0}},
		{"go", [3]int{0, 0, 0}},
		{"go1.a.b", [3]int{1, 0, 0}},
		{"go0.0.0", [3]int{0, 0, 0}},
		{"go2.0.0", [3]int{2, 0, 0}},
	}
	for _, tc := range tests {
		got := parseGoVersion(tc.input)
		if got != tc.want {
			t.Errorf("parseGoVersion(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// ─── versionAtLeast ─────────────────────────────────────.

func TestVersionAtLeast(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cur  [3]int
		min  [3]int
		want bool
	}{
		{[3]int{1, 26, 0}, [3]int{1, 21, 0}, true},
		{[3]int{1, 21, 0}, [3]int{1, 21, 0}, true},
		{[3]int{1, 20, 0}, [3]int{1, 21, 0}, false},
		{[3]int{1, 26, 4}, [3]int{1, 26, 0}, true},
		{[3]int{1, 19, 0}, [3]int{1, 20, 0}, false},
		{[3]int{2, 0, 0}, [3]int{1, 99, 0}, true},
		{[3]int{0, 0, 0}, [3]int{1, 0, 0}, false},
		{[3]int{1, 0, 0}, [3]int{0, 0, 0}, true},
		{[3]int{1, 26, 0}, [3]int{1, 26, 1}, false},
		{[3]int{1, 26, 5}, [3]int{1, 26, 5}, true},
	}
	for _, tc := range tests {
		got := versionAtLeast(tc.cur, tc.min)
		if got != tc.want {
			t.Errorf("versionAtLeast(%v, %v) = %v, want %v", tc.cur, tc.min, got, tc.want)
		}
	}
}

// ─── truncateStr ────────────────────────────────────────.

func TestTruncateStr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello world", 5, "hello..."},
		{"short", 20, "short"},
		{"", 5, ""},
		{"exact", 5, "exact"},
		{"six", 6, "six"},
		{"hello", 3, "hel..."},
	}
	for _, tc := range tests {
		got := truncateStr(tc.input, tc.maxLen)
		if got != tc.want {
			t.Errorf("truncateStr(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
		}
	}
}

func TestTruncateStrZeroMax(t *testing.T) {
	t.Parallel()
	got := truncateStr("hello", 0)
	if got != "..." {
		t.Errorf("truncateStr(\"hello\", 0) = %q, want %q", got, "...")
	}
}

// ─── CountResults ───────────────────────────────────────.

func TestCountResultsBasic(t *testing.T) {
	t.Parallel()
	pass, warn, fail := CountResults([]Result{
		{Status: StatusPass},
		{Status: StatusWarn},
		{Status: StatusFail},
	})
	if pass != 1 || warn != 1 || fail != 1 {
		t.Errorf("CountResults = (%d,%d,%d), want (1,1,1)", pass, warn, fail)
	}
}

func TestCountResultsEmpty(t *testing.T) {
	t.Parallel()
	pass, warn, fail := CountResults(nil)
	if pass != 0 || warn != 0 || fail != 0 {
		t.Errorf("CountResults(nil) = (%d,%d,%d), want (0,0,0)", pass, warn, fail)
	}
	pass, warn, fail = CountResults([]Result{})
	if pass != 0 || warn != 0 || fail != 0 {
		t.Errorf("CountResults([]) = (%d,%d,%d), want (0,0,0)", pass, warn, fail)
	}
}

func TestCountResultsAllPass(t *testing.T) {
	t.Parallel()
	pass, warn, fail := CountResults([]Result{
		{Status: StatusPass},
		{Status: StatusPass},
		{Status: StatusPass},
	})
	if pass != 3 || warn != 0 || fail != 0 {
		t.Errorf("CountResults = (%d,%d,%d), want (3,0,0)", pass, warn, fail)
	}
}

func TestCountResultsAllFail(t *testing.T) {
	t.Parallel()
	pass, warn, fail := CountResults([]Result{
		{Status: StatusFail},
		{Status: StatusFail},
	})
	if pass != 0 || warn != 0 || fail != 2 {
		t.Errorf("CountResults = (%d,%d,%d), want (0,0,2)", pass, warn, fail)
	}
}

// ─── checkConfig ────────────────────────────────────────.

func TestCheckConfigPassiveBruteforceConflict(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{PassiveOnly: true, Bruteforce: true}
	results := checkConfig(cfg)
	found := false
	for _, r := range results {
		if r.Name == "Conflicting flags" {
			found = true
			if r.Status != StatusWarn {
				t.Errorf("expected WARN status, got %s", r.Status)
			}
		}
	}
	if !found {
		t.Error("expected Conflicting flags check")
	}
}

func TestCheckConfigDoHResolvers(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{DoH: true, Resolvers: []string{"1.1.1.1:53"}}
	results := checkConfig(cfg)
	found := false
	for _, r := range results {
		if r.Name == "Resolver conflict" {
			found = true
			if r.Status != StatusPass {
				t.Errorf("expected PASS status, got %s", r.Status)
			}
		}
	}
	if !found {
		t.Error("expected Resolver conflict check")
	}
}

func TestCheckConfigThreadsDefault(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{Threads: 0}
	results := checkConfig(cfg)
	found := false
	for _, r := range results {
		if r.Name == "Concurrency" {
			found = true
			if r.Status != StatusPass {
				t.Errorf("expected PASS, got %s", r.Status)
			}
		}
	}
	if !found {
		t.Error("expected Concurrency check")
	}
}

func TestCheckConfigThreadsExplicit(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{Threads: 100}
	results := checkConfig(cfg)
	for _, r := range results {
		if r.Name == "Concurrency" {
			if !strings.Contains(r.Message, "100") {
				t.Errorf("expected 100 in message, got %q", r.Message)
			}
		}
	}
}

func TestCheckConfigPermuteLevel(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{Permute: true, PermuteLevel: 3}
	results := checkConfig(cfg)
	found := false
	for _, r := range results {
		if r.Name == "Permutation engine" {
			found = true
			if r.Detail == "" {
				t.Error("expected detail for level 3")
			}
		}
	}
	if !found {
		t.Error("expected Permutation engine check")
	}
}

func TestCheckConfigNSECWalk(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{NSECWalk: true}
	results := checkConfig(cfg)
	found := false
	for _, r := range results {
		if r.Name == "NSEC walk" {
			found = true
			if r.Status != StatusWarn {
				t.Errorf("expected WARN, got %s", r.Status)
			}
		}
	}
	if !found {
		t.Error("expected NSEC walk check")
	}
}

func TestCheckConfigMaxWordlistSize(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{MaxWordlistSize: 500}
	results := checkConfig(cfg)
	found := false
	for _, r := range results {
		if r.Name == "Wordlist limit" {
			found = true
			if r.Status != StatusPass {
				t.Errorf("expected PASS, got %s", r.Status)
			}
		}
	}
	if !found {
		t.Error("expected Wordlist limit check")
	}
}

func TestCheckConfigEmpty(t *testing.T) {
	t.Parallel()
	cfg := &types.Config{}
	results := checkConfig(cfg)
	if len(results) == 0 {
		t.Error("expected results for empty config")
	}
}

// ─── checkWordlist ──────────────────────────────────────.

func TestCheckWordlist(t *testing.T) {
	t.Parallel()
	results := checkWordlist()
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	r := results[0]
	if r.Section != "Wordlist" {
		t.Errorf("Section = %q, want Wordlist", r.Section)
	}
	if r.Name != "Built-in" {
		t.Errorf("Name = %q, want Built-in", r.Name)
	}
	if r.Status != StatusPass {
		t.Errorf("Status = %s, want PASS", r.Status)
	}
	if !strings.Contains(r.Message, "entries") {
		t.Errorf("unexpected message: %q", r.Message)
	}
}

func TestCheckWordlistDetail(t *testing.T) {
	t.Parallel()
	results := checkWordlist()
	if len(results) == 0 {
		t.Fatal("no results")
	}
	r := results[0]
	if r.Detail == "" {
		t.Log("no detail (fewer than 3 entries)")
	} else if !strings.Contains(r.Detail, "samples:") {
		t.Errorf("expected samples in detail, got %q", r.Detail)
	}
}

// ─── FormatText ─────────────────────────────────────────.

func TestFormatTextEmpty(t *testing.T) {
	t.Parallel()
	output := FormatText([]Result{}, 0, 0, 0)
	if output == "" {
		t.Error("empty output")
	}
	if !strings.Contains(output, "Summary:") {
		t.Errorf("expected Summary in output, got %q", output)
	}
}

func TestFormatTextFull(t *testing.T) {
	t.Parallel()
	results := []Result{
		{Section: "System", Name: "Platform", Status: StatusPass, Message: "linux/amd64"},
		{Section: "System", Name: "Memory", Status: StatusWarn, Message: "low", Detail: "swap enabled"},
		{Section: "Network", Name: "Internet", Status: StatusFail, Message: "unreachable"},
	}
	output := FormatText(results, 1, 1, 1)
	if !strings.Contains(output, "[PASS]") {
		t.Error("expected PASS")
	}
	if !strings.Contains(output, "[WARN]") {
		t.Error("expected WARN")
	}
	if !strings.Contains(output, "[FAIL]") {
		t.Error("expected FAIL")
	}
	if !strings.Contains(output, "1 pass, 1 warn, 1 fail") {
		t.Errorf("unexpected summary: %q", output)
	}
}

func TestFormatTextSectionGrouping(t *testing.T) {
	t.Parallel()
	results := []Result{
		{Section: "A", Name: "A1", Status: StatusPass, Message: "ok"},
		{Section: "A", Name: "A2", Status: StatusPass, Message: "ok"},
		{Section: "B", Name: "B1", Status: StatusPass, Message: "ok"},
	}
	output := FormatText(results, 3, 0, 0)
	lines := strings.Split(output, "\n")
	sectionLines := 0
	for _, l := range lines {
		if strings.Contains(l, "A") || strings.Contains(l, "B") {
			sectionLines++
		}
	}
	if sectionLines < 2 {
		t.Errorf("expected at least 2 section lines, got %d", sectionLines)
	}
}

func TestFormatTextAllPass(t *testing.T) {
	t.Parallel()
	results := []Result{
		{Section: "Test", Name: "T1", Status: StatusPass, Message: "ok"},
	}
	output := FormatText(results, 1, 0, 0)
	if !strings.Contains(output, "All systems operational") {
		t.Errorf("expected all ok message, got %q", output)
	}
}

func TestFormatTextWithFail(t *testing.T) {
	t.Parallel()
	results := []Result{
		{Section: "Test", Name: "T1", Status: StatusFail, Message: "broken"},
	}
	output := FormatText(results, 0, 0, 1)
	if !strings.Contains(output, "Issues found") {
		t.Errorf("expected issues message, got %q", output)
	}
}

func TestFormatTextWithWarn(t *testing.T) {
	t.Parallel()
	results := []Result{
		{Section: "Test", Name: "T1", Status: StatusWarn, Message: "caution"},
	}
	output := FormatText(results, 0, 1, 0)
	if !strings.Contains(output, "Non-critical issues") {
		t.Errorf("expected warning message, got %q", output)
	}
}

// ─── FormatJSON ─────────────────────────────────────────.

func TestFormatJSONBasic(t *testing.T) {
	t.Parallel()
	results := []Result{
		{Section: "Test", Name: "T1", Status: StatusPass, Message: "ok"},
	}
	output, err := FormatJSON(results, 1, 0, 0)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	if !strings.Contains(output, "results") {
		t.Errorf("expected results in JSON, got %q", output)
	}
	if !strings.Contains(output, "summary") {
		t.Errorf("expected summary in JSON, got %q", output)
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	t.Parallel()
	output, err := FormatJSON([]Result{}, 0, 0, 0)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, output)
	}
}

func TestFormatJSONRoundTrip(t *testing.T) {
	t.Parallel()
	original := []Result{
		{Section: "S1", Name: "N1", Status: StatusPass, Message: "M1", Detail: "D1", Latency: 100 * time.Millisecond},
		{Section: "S2", Name: "N2", Status: StatusWarn, Message: "M2"},
		{Section: "S3", Name: "N3", Status: StatusFail, Message: "M3", Detail: "D3", Latency: 5 * time.Second},
	}
	output, err := FormatJSON(original, 1, 1, 1)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	var result struct {
		Results []Result `json:"results"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(result.Results) != 3 {
		t.Errorf("got %d results, want 3", len(result.Results))
	}
}

// ─── Run with nil config ────────────────────────────────.

func TestRunNilConfig(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	results := Run(t.Context(), nil, Options{Verbose: false, JSON: false})
	if len(results) == 0 {
		t.Error("expected non-empty results")
	}
}

func TestRunWithProxyConfig(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	cfg := &types.Config{
		ProxyURL: "socks5://127.0.0.1:9050",
	}
	results := Run(t.Context(), cfg, Options{Verbose: false, JSON: false})
	if len(results) == 0 {
		t.Error("expected non-empty results")
	}
}

func TestRunVerbose(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	results := Run(t.Context(), nil, Options{Verbose: true, JSON: false})
	if len(results) == 0 {
		t.Error("expected non-empty results")
	}
}

// ─── Run with tor config ────────────────────────────────.

func TestRunTorConfig(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	cfg := &types.Config{
		TorMode:  true,
		ProxyURL: "socks5://127.0.0.1:9050",
	}
	results := Run(t.Context(), cfg, Options{Verbose: false, JSON: false})
	if len(results) == 0 {
		t.Error("expected non-empty results")
	}
}

// ─── checkGoVersion ─────────────────────────────────────.

func TestCheckGoVersion(t *testing.T) {
	t.Parallel()
	results := checkGoVersion()
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	r := results[0]
	if r.Name != "Go version" {
		t.Errorf("Name = %q, want Go version", r.Name)
	}
}

// ─── checkGoMod ─────────────────────────────────────────.

func TestCheckGoMod(t *testing.T) {
	t.Parallel()
	results := checkGoMod()
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	r := results[0]
	if r.Name != "go.mod" {
		t.Errorf("Name = %q, want go.mod", r.Name)
	}
}

// ─── checkBuild ─────────────────────────────────────────.

func TestCheckBuild(t *testing.T) {
	t.Parallel()
	results := checkBuild()
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	r := results[0]
	if r.Name != "Build" {
		t.Errorf("Name = %q, want Build", r.Name)
	}
}

// ─── checkPermissions ───────────────────────────────────.

func TestCheckPermissions(t *testing.T) {
	t.Parallel()
	results := checkPermissions()
	if len(results) == 0 {
		t.Fatal("expected non-empty results")
	}
	foundWorkDir := false
	foundTempDir := false
	foundFileHandles := false
	for _, r := range results {
		switch r.Name {
		case "Working directory":
			foundWorkDir = true
		case "Temp directory":
			foundTempDir = true
		case "File handles":
			foundFileHandles = true
		}
	}
	if !foundWorkDir {
		t.Error("missing Working directory check")
	}
	if !foundTempDir {
		t.Error("missing Temp directory check")
	}
	if !foundFileHandles {
		t.Error("missing File handles check")
	}
}

// ─── checkProxyEnv ──────────────────────────────────────.

func TestCheckProxyEnv(t *testing.T) {
	t.Parallel()
	r := checkProxyEnv()
	if r.Section != "Network" {
		t.Errorf("Section = %q, want Network", r.Section)
	}
	if r.Name != "HTTP proxy env" {
		t.Errorf("Name = %q, want HTTP proxy env", r.Name)
	}
}

// ─── checkSystem ────────────────────────────────────────.

func TestCheckSystem(t *testing.T) {
	t.Parallel()
	results := checkSystem()
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	foundPlatform := false
	foundGoRuntime := false
	for _, r := range results {
		switch r.Name {
		case "Platform":
			foundPlatform = true
			if r.Status != StatusPass {
				t.Errorf("Platform status = %s, want PASS", r.Status)
			}
		case "Go runtime":
			foundGoRuntime = true
		}
	}
	if !foundPlatform {
		t.Error("missing Platform check")
	}
	if !foundGoRuntime {
		t.Error("missing Go runtime check")
	}
}

// ─── checkDependencies ──────────────────────────────────.

func TestCheckDependencies(t *testing.T) {
	t.Parallel()
	results := checkDependencies()
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
}

// ─── checkExternalTools ─────────────────────────────────.

func TestCheckExternalTools(t *testing.T) {
	t.Parallel()
	results := checkExternalTools()
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	for _, r := range results {
		if r.Section != "External Tools" {
			t.Errorf("Section = %q, want External Tools", r.Section)
		}
	}
}

// ─── Result latency ─────────────────────────────────────.

func TestResultLatencyRoundTrip(t *testing.T) {
	t.Parallel()
	r := Result{
		Latency: 100 * time.Millisecond,
	}
	if r.Latency != 100*time.Millisecond {
		t.Errorf("Latency = %v, want 100ms", r.Latency)
	}
}

// ─── Run with full config ───────────────────────────────.

func TestRunFullConfig(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping network-dependent test")
	}
	cfg := &types.Config{
		Domain:          "example.com",
		Threads:         50,
		Timeout:         30,
		Silent:          true,
		Verbose:         false,
		Bruteforce:      true,
		PassiveOnly:     false,
		Permute:         true,
		PermuteLevel:    2,
		NSECWalk:        true,
		DoH:             true,
		MaxWordlistSize: 0,
		ProxyURL:        "",
		TorMode:         false,
	}
	results := Run(t.Context(), cfg, Options{Verbose: false, JSON: false})
	if len(results) == 0 {
		t.Error("expected non-empty results")
	}
}

// ─── FormatText/JSON consistency ────────────────────────.

func TestFormatConsistency(t *testing.T) {
	t.Parallel()
	results := []Result{
		{Section: "S", Name: "N", Status: StatusPass, Message: "M"},
		{Section: "S", Name: "N2", Status: StatusWarn, Message: "M2"},
	}
	pass, warn, fail := CountResults(results)
	if pass != 1 || warn != 1 || fail != 0 {
		t.Fatalf("CountResults = (%d,%d,%d)", pass, warn, fail)
	}
	text := FormatText(results, pass, warn, fail)
	jsonOut, err := FormatJSON(results, pass, warn, fail)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	if text == "" || jsonOut == "" {
		t.Error("empty output")
	}
}

// ─── Benchmark ──────────────────────────────────────────.

func BenchmarkParseGoVersion(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		parseGoVersion("go1.26.4")
	}
}

func BenchmarkVersionAtLeast(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		versionAtLeast([3]int{1, 26, 4}, [3]int{1, 21, 0})
	}
}

func BenchmarkCountResults(b *testing.B) {
	results := []Result{
		{Status: StatusPass},
		{Status: StatusWarn},
		{Status: StatusFail},
		{Status: StatusPass},
		{Status: StatusPass},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		CountResults(results)
	}
}

func BenchmarkFormatText(b *testing.B) {
	results := []Result{
		{Section: "S1", Name: "N1", Status: StatusPass, Message: "hello world"},
		{Section: "S1", Name: "N2", Status: StatusWarn, Message: "warning"},
		{Section: "S2", Name: "N3", Status: StatusFail, Message: "failure", Detail: "some detail"},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		FormatText(results, 1, 1, 1)
	}
}

func BenchmarkFormatJSON(b *testing.B) {
	results := []Result{
		{Section: "S1", Name: "N1", Status: StatusPass, Message: "hello world"},
		{Section: "S1", Name: "N2", Status: StatusWarn, Message: "warning"},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = FormatJSON(results, 1, 1, 0)
	}
}

// ─── Example tests ──────────────────────────────────────.

func ExampleStatus() {
	fmt.Println(StatusPass.String())
	fmt.Println(StatusWarn.String())
	fmt.Println(StatusFail.String())
	// Output:
	// PASS
	// WARN
	// FAIL
}

func ExampleCountResults() {
	results := []Result{
		{Status: StatusPass},
		{Status: StatusWarn},
		{Status: StatusFail},
	}
	p, w, f := CountResults(results)
	fmt.Printf("%d %d %d", p, w, f)
	// Output: 1 1 1
}
