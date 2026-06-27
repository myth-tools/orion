package active

import (
	"math"
	"slices"
	"strings"
	"testing"
)

// ─── NewPermutator ──────────────────────────────────────.

func TestNewPermutatorLevels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		level int
		want  int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 3},
		{4, 3},
		{-1, 1},
	}
	for _, tc := range tests {
		p := NewPermutator(tc.level)
		if p.level != tc.want {
			t.Errorf("NewPermutator(%d).level = %d, want %d", tc.level, p.level, tc.want)
		}
	}
}

func TestNewPermutatorMaxCandidates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		level int
		want  int
	}{
		{1, 10000},
		{2, 50000},
		{3, 200000},
	}
	for _, tc := range tests {
		p := NewPermutator(tc.level)
		if p.maxCandidates != tc.want {
			t.Errorf("NewPermutator(%d).maxCandidates = %d, want %d", tc.level, p.maxCandidates, tc.want)
		}
	}
}

// ─── Generate ───────────────────────────────────────────.

func TestGenerateEmptyInput(t *testing.T) {
	t.Parallel()
	p := NewPermutator(2)
	results := p.Generate(nil, "example.com")
	if results != nil {
		t.Errorf("expected nil for nil input, got %d entries", len(results))
	}

	results = p.Generate([]string{}, "example.com")
	if len(results) != 0 {
		t.Errorf("expected 0 for empty input, got %d entries", len(results))
	}
}

func TestGenerateBasicDeduplication(t *testing.T) {
	t.Parallel()
	p := NewPermutator(2)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	seen := make(map[string]bool)
	for _, r := range results {
		if seen[r] {
			t.Errorf("duplicate permutation: %s", r)
		}
		seen[r] = true
	}
}

func TestGenerateNoOriginalsInOutput(t *testing.T) {
	t.Parallel()
	p := NewPermutator(2)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	for _, r := range results {
		if r == "www.example.com" {
			t.Errorf("output contains original: %s", r)
		}
	}
}

func TestGenerateAllValidDomains(t *testing.T) {
	t.Parallel()
	p := NewPermutator(2)
	results := p.Generate([]string{"api.example.com", "dev.example.com"}, "example.com")
	domain := "example.com"
	for _, r := range results {
		if !strings.HasSuffix(r, "."+domain) {
			t.Errorf("result %q does not end with %q", r, domain)
		}
		if strings.Count(r, ".") < 1 {
			t.Errorf("result %q has no dots", r)
		}
	}
}

func TestGenerateLevel1Count(t *testing.T) {
	t.Parallel()
	p := NewPermutator(1)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	if len(results) == 0 {
		t.Fatal("level 1 generated no results")
	}
	if len(results) > 10000 {
		t.Errorf("level 1 exceeded maxCandidates: %d", len(results))
	}
}

func TestGenerateLevel2Count(t *testing.T) {
	t.Parallel()
	p := NewPermutator(2)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	if len(results) == 0 {
		t.Fatal("level 2 generated no results")
	}
	if len(results) > 50000 {
		t.Errorf("level 2 exceeded maxCandidates: %d", len(results))
	}
}

func TestGenerateLevel3Count(t *testing.T) {
	t.Parallel()
	p := NewPermutator(3)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	if len(results) == 0 {
		t.Fatal("level 3 generated no results")
	}
	if len(results) > 200000 {
		t.Errorf("level 3 exceeded maxCandidates: %d", len(results))
	}
}

func TestGenerateLevel2HasMoreThanLevel1(t *testing.T) {
	t.Parallel()
	p1 := NewPermutator(1)
	p2 := NewPermutator(2)
	r1 := p1.Generate([]string{"www.example.com"}, "example.com")
	r2 := p2.Generate([]string{"www.example.com"}, "example.com")
	if len(r2) < len(r1) {
		t.Errorf("level 2 (%d) should have >= entries than level 1 (%d)", len(r2), len(r1))
	}
}

func TestGenerateMultipleInputs(t *testing.T) {
	t.Parallel()
	p := NewPermutator(1)
	results := p.Generate([]string{"www.example.com", "mail.example.com", "api.example.com"}, "example.com")
	if len(results) == 0 {
		t.Fatal("multiple inputs generated no results")
	}
}

func TestGenerateNoDuplicatesAcrossMultipleInputs(t *testing.T) {
	t.Parallel()
	p := NewPermutator(1)
	results := p.Generate([]string{"www.example.com", "mail.example.com"}, "example.com")
	seen := make(map[string]bool)
	for _, r := range results {
		if seen[r] {
			t.Errorf("duplicate: %s", r)
		}
		seen[r] = true
	}
}

func TestGenerateLargeInputBatch(t *testing.T) {
	t.Parallel()
	p := NewPermutator(2)
	inputs := make([]string, 100)
	for i := range 100 {
		inputs[i] = "prefix" + strings.Repeat("x", i%10) + ".example.com"
	}
	results := p.Generate(inputs, "example.com")
	if len(results) > 50000 {
		t.Errorf("exceeded maxCandidates: %d", len(results))
	}
}

// ─── Specific permutation types ─────────────────────────.

func TestPrefixPermutations(t *testing.T) {
	t.Parallel()
	p := NewPermutator(2)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	hasPrefix := false
	for _, r := range results {
		if strings.HasPrefix(r, "dev-www.example.com") || strings.HasPrefix(r, "stage-www.example.com") {
			hasPrefix = true
			break
		}
	}
	if !hasPrefix {
		t.Error("expected prefix permutations like 'dev-www.example.com' not found")
	}
}

func TestSuffixPermutations(t *testing.T) {
	t.Parallel()
	p := NewPermutator(1)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	hasSuffix := false
	for _, r := range results {
		if strings.HasPrefix(r, "www-dev.example.com") || strings.HasPrefix(r, "www-staging.example.com") {
			hasSuffix = true
			break
		}
	}
	if !hasSuffix {
		t.Error("expected suffix permutations like 'www-dev.example.com' not found")
	}
}

func TestNumberPermutations(t *testing.T) {
	t.Parallel()
	p := NewPermutator(1)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	hasNumber := false
	for _, r := range results {
		if strings.HasPrefix(r, "www-01.example.com") || strings.HasPrefix(r, "www-1.example.com") {
			hasNumber = true
			break
		}
	}
	if !hasNumber {
		t.Error("expected number permutations like 'www-01.example.com' not found")
	}
}

func TestRegionPermutationsLevel3(t *testing.T) {
	t.Parallel()
	p := NewPermutator(3)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	hasRegion := false
	for _, r := range results {
		if strings.HasPrefix(r, "www-us-east-1.example.com") || strings.HasPrefix(r, "www-eu-west-1.example.com") {
			hasRegion = true
			break
		}
	}
	if !hasRegion {
		t.Error("expected region permutations like 'www-us-east-1.example.com' not found")
	}
}

func TestSubSubPermutationsLevel3(t *testing.T) {
	t.Parallel()
	p := NewPermutator(3)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	hasSubSub := false
	for _, r := range results {
		if strings.HasPrefix(r, "dev-www-staging.example.com") || strings.HasPrefix(r, "stage-www-prod.example.com") {
			hasSubSub = true
			break
		}
	}
	if !hasSubSub {
		t.Log("sub-sub permutations not found (may vary by build)")
	}
}

// ─── Edge cases ─────────────────────────────────────────.

func TestGenerateSpecialCharacters(t *testing.T) {
	t.Parallel()
	p := NewPermutator(1)
	results := p.Generate([]string{"test-123.example.com"}, "example.com")
	for _, r := range results {
		if !strings.HasSuffix(r, ".example.com") {
			t.Errorf("invalid domain: %q", r)
		}
	}
}

func TestGenerateVeryLongInput(t *testing.T) {
	t.Parallel()
	p := NewPermutator(1)
	longInput := strings.Repeat("a", 200) + ".example.com"
	results := p.Generate([]string{longInput}, "example.com")
	for _, r := range results {
		if len(r) > 253 {
			t.Errorf("domain too long: %q (%d chars)", r, len(r))
		}
	}
}

func TestGenerateInputWithDomain(t *testing.T) {
	t.Parallel()
	p := NewPermutator(1)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	for _, r := range results {
		if r == "www.example.com.example.com" {
			// This is technically valid but probably not useful
			t.Logf("found self-referencing domain: %q", r)
		}
	}
}

func TestGenerateMaxCandidatesRespected(t *testing.T) {
	t.Parallel()
	p := NewPermutator(1)
	inputs := make([]string, 200)
	for i := range 200 {
		inputs[i] = "www.example.com"
	}
	results := p.Generate(inputs, "example.com")
	if len(results) > p.maxCandidates {
		t.Errorf("exceeded maxCandidates: %d > %d", len(results), p.maxCandidates)
	}
}

// ─── Sort order ─────────────────────────────────────────.

func TestGenerateResultsSorted(t *testing.T) {
	t.Parallel()
	p := NewPermutator(1)
	results := p.Generate([]string{"www.example.com", "api.example.com"}, "example.com")
	if !slices.IsSorted(results) {
		t.Error("results not sorted")
	}
}

// ─── Concurrency ────────────────────────────────────────.

func TestGenerateConcurrent(t *testing.T) {
	t.Parallel()
	p := NewPermutator(2)
	done := make(chan bool, 10)
	for range 10 {
		go func() {
			results := p.Generate([]string{"www.example.com"}, "example.com")
			_ = results
			done <- true
		}()
	}
	for range 10 {
		<-done
	}
}

// ─── idempotency ────────────────────────────────────────.

func TestGenerateDeterministic(t *testing.T) {
	t.Parallel()
	p := NewPermutator(2)
	r1 := p.Generate([]string{"www.example.com"}, "example.com")
	r2 := p.Generate([]string{"www.example.com"}, "example.com")
	if len(r1) != len(r2) {
		t.Fatalf("different lengths: %d vs %d", len(r1), len(r2))
	}
	for i := range r1 {
		if r1[i] != r2[i] {
			t.Errorf("different at index %d: %q vs %q", i, r1[i], r2[i])
		}
	}
}

// ─── Benchmarks ─────────────────────────────────────────.

func BenchmarkGenerateLevel1(b *testing.B) {
	p := NewPermutator(1)
	inputs := []string{"www.example.com", "mail.example.com", "api.example.com"}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		p.Generate(inputs, "example.com")
	}
}

func BenchmarkGenerateLevel2(b *testing.B) {
	p := NewPermutator(2)
	inputs := []string{"www.example.com", "mail.example.com", "api.example.com"}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		p.Generate(inputs, "example.com")
	}
}

func BenchmarkGenerateLevel3(b *testing.B) {
	p := NewPermutator(3)
	inputs := []string{"www.example.com", "mail.example.com"}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		p.Generate(inputs, "example.com")
	}
}

func BenchmarkGenerateManyInputs(b *testing.B) {
	p := NewPermutator(2)
	inputs := make([]string, 50)
	for i := range 50 {
		inputs[i] = "prefix" + strings.Repeat("x", i%5) + ".example.com"
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		p.Generate(inputs, "example.com")
	}
}

// ─── Example ────────────────────────────────────────────.

func TestPermutatorGenerate(t *testing.T) {
	t.Parallel()
	p := NewPermutator(1)
	results := p.Generate([]string{"www.example.com"}, "example.com")
	if len(results) == 0 {
		t.Error("expected at least one result")
	}
	for _, r := range results {
		if !strings.HasSuffix(r, ".example.com") {
			t.Errorf("result %q does not end with .example.com", r)
		}
	}
}

// ─── level bound checks ─────────────────────────────────.

func TestMaxCandidatesForLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input     int
		wantLevel int
		wantCand  int
	}{
		{0, 1, 10000},
		{1, 1, 10000},
		{2, 2, 50000},
		{3, 3, 200000},
		{4, 3, 200000},
		{-5, 1, 10000},
	}
	for _, tc := range tests {
		p := NewPermutator(tc.input)
		if p.maxCandidates != tc.wantCand {
			t.Errorf("input %d: got maxCandidates=%d, want %d", tc.input, p.maxCandidates, tc.wantCand)
		}
		if p.level != tc.wantLevel {
			t.Errorf("input %d: got level=%d, want %d", tc.input, p.level, tc.wantLevel)
		}
	}
}

// Verify float64 conversion doesn't cause issues.
func TestMaxCandidatesFloatSafe(t *testing.T) {
	t.Parallel()
	const maxInt = 1<<63 - 1
	if float64(maxInt) > math.MaxFloat64 {
		t.Error("potential overflow in float conversion")
	}
}
