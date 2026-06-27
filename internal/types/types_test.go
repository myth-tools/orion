package types

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	t.Parallel()
	c := Config{}
	if c.Threads != 0 {
		t.Errorf("default Threads = %d, want 0", c.Threads)
	}
	if c.Timeout != 0 {
		t.Errorf("default Timeout = %d, want 0", c.Timeout)
	}
	if c.PermuteLevel != 0 {
		t.Errorf("default PermuteLevel = %d, want 0", c.PermuteLevel)
	}
	if c.MaxWordlistSize != 0 {
		t.Errorf("default MaxWordlistSize = %d, want 0", c.MaxWordlistSize)
	}
}

func TestConfigFullRoundTrip(t *testing.T) {
	t.Parallel()
	original := Config{
		Domain:          "example.com",
		DomainsFile:     "/tmp/domains.txt",
		Threads:         42,
		Timeout:         15,
		OutputFile:      "/tmp/output.txt",
		OutputDirectory: "/tmp/outputs",
		Silent:          true,
		Verbose:         false,
		JSON:            true,
		HostIP:          true,
		CaptureSources:  true,
		RemoveWildcard:  true,
		Statistics:      true,
		ListSources:     true,
		Bruteforce:      true,
		Wordlist:        "/tmp/words.txt",
		Resolvers:       []string{"1.1.1.1:53", "8.8.8.8:53"},
		PassiveOnly:     false,
		Permute:         true,
		PermuteLevel:    2,
		NSECWalk:        true,
		DoH:             false,
		MaxWordlistSize: 500,
		ProxyURL:        "socks5://127.0.0.1:9050",
		TorMode:         true,
		Sources:         []string{"crtsh", "alienvault"},
		ExcludeSources:  []string{"google"},
		Match:           []string{"*.example.com"},
		Filter:          []string{"admin.example.com"},
		RateLimit:       10,
		RateLimits:      map[string]RateLimitEntry{"crtsh": {MaxCount: 10}},
	}

	checkConfigRoundTrip(t, original)
}

func checkConfigRoundTrip(t *testing.T, original Config) {
	t.Helper()
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(decoded, original) {
		t.Errorf("round-trip mismatch:\n decoded=%+v\n original=%+v", decoded, original)
	}
}

func TestConfigZeroValues(t *testing.T) {
	t.Parallel()
	c := Config{
		Domain: "test.com",
	}
	if c.Domain != "test.com" {
		t.Errorf("Domain = %q, want %q", c.Domain, "test.com")
	}
}

func TestConfigWithResolvers(t *testing.T) {
	t.Parallel()
	c := Config{
		Resolvers: []string{"1.1.1.1:53"},
	}
	if len(c.Resolvers) != 1 {
		t.Errorf("expected 1 resolver, got %d", len(c.Resolvers))
	}
	if c.Resolvers[0] != "1.1.1.1:53" {
		t.Errorf("Resolver[0] = %q, want %q", c.Resolvers[0], "1.1.1.1:53")
	}
}

func TestConfigEmptyResolvers(t *testing.T) {
	t.Parallel()
	c := Config{}
	if c.Resolvers != nil {
		t.Error("expected nil Resolvers by default")
	}
}

// ─── Benchmarks ──────────────────────────────────────────.

func BenchmarkConfigJSONRoundTrip(b *testing.B) {
	c := Config{
		Domain:          "example.com",
		Threads:         50,
		Timeout:         30,
		OutputFile:      "/tmp/out.txt",
		Silent:          true,
		Verbose:         false,
		JSON:            false,
		HostIP:          false,
		CaptureSources:  false,
		RemoveWildcard:  false,
		Statistics:      false,
		Bruteforce:      true,
		Wordlist:        "/tmp/words.txt",
		Resolvers:       []string{"1.1.1.1:53", "8.8.8.8:53", "9.9.9.9:53"},
		PassiveOnly:     false,
		Permute:         true,
		PermuteLevel:    2,
		NSECWalk:        false,
		DoH:             true,
		MaxWordlistSize: 1000,
		ProxyURL:        "",
		TorMode:         false,
		Sources:         []string{"crtsh"},
		ExcludeSources:  []string{},
		Match:           nil,
		Filter:          nil,
		RateLimit:       0,
	}

	b.ReportAllocs()
	for range b.N {
		data, err := json.Marshal(c)
		if err != nil {
			b.Fatal(err)
		}
		var c2 Config
		_ = json.Unmarshal(data, &c2)
	}
}

func ExampleConfig_usage() {
	cfg := Config{
		Domain:         "example.com",
		Threads:        10,
		Bruteforce:     true,
		JSON:           true,
		RemoveWildcard: true,
		CaptureSources: true,
	}
	fmt.Println(cfg.Domain)
	fmt.Println(cfg.Threads)
	fmt.Println(cfg.Bruteforce)
	fmt.Println(cfg.JSON)
	fmt.Println(cfg.RemoveWildcard)
	fmt.Println(cfg.CaptureSources)
	// Output:
	// example.com
	// 10
	// true
	// true
	// true
	// true
}

// ─── Fuzzing helpers ────────────────────────────────────.

func FuzzConfigJSON(f *testing.F) {
	seeds := []string{
		`{"domain":"test.com"}`,
		`{"threads":100,"timeout":30,"silent":true}`,
		`{"resolvers":["1.1.1.1:53","8.8.8.8:53"]}`,
		`{"anything":"garbage","values":1}`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(_ *testing.T, data string) {
		var c Config
		_ = json.Unmarshal([]byte(data), &c)
		// Must not panic on any input
		_ = c.Domain
		_ = c.Threads
		_ = c.Resolvers
	})
}
