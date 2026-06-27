package active

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const testCandidate = "www.example.com"

func newTestBruteforcer(words []string) *Bruteforcer {
	return NewBruteforcer(words, nil, 0, 0, 0)
}

// ─── NewBruteforcer ─────────────────────────────────────.

func TestNewBruteforcerBasic(t *testing.T) {
	t.Parallel()
	words := []string{"www", "mail", "api"}
	b := newTestBruteforcer(words)
	if b == nil {
		t.Fatal("NewBruteforcer returned nil")
	}
	if b.WordlistSize() != 3 {
		t.Errorf("WordlistSize = %d, want 3", b.WordlistSize())
	}
}

func TestNewBruteforcerEmpty(t *testing.T) {
	t.Parallel()
	b := newTestBruteforcer([]string{})
	if b == nil {
		t.Fatal("NewBruteforcer returned nil")
	}
	if b.WordlistSize() != 0 {
		t.Errorf("WordlistSize = %d, want 0", b.WordlistSize())
	}
}

func TestNewBruteforcerNil(t *testing.T) {
	t.Parallel()
	b := newTestBruteforcer(nil)
	if b == nil {
		t.Fatal("NewBruteforcer returned nil")
	}
	if b.WordlistSize() != 0 {
		t.Errorf("WordlistSize = %d, want 0", b.WordlistSize())
	}
}

// ─── GenerateCandidates ─────────────────────────────────.

func TestGenerateCandidatesBasic(t *testing.T) {
	t.Parallel()
	words := []string{"www", "mail", "api"}
	b := newTestBruteforcer(words)
	candidates := b.GenerateCandidates("example.com")
	if len(candidates) != 3 {
		t.Fatalf("len = %d, want 3", len(candidates))
	}
	expected := []string{"www.example.com", "mail.example.com", "api.example.com"}
	for i, c := range candidates {
		if c != expected[i] {
			t.Errorf("candidates[%d] = %q, want %q", i, c, expected[i])
		}
	}
}

func TestGenerateCandidatesEmptyWordlist(t *testing.T) {
	t.Parallel()
	b := newTestBruteforcer([]string{})
	candidates := b.GenerateCandidates("example.com")
	if len(candidates) != 0 {
		t.Errorf("len = %d, want 0", len(candidates))
	}
}

func TestGenerateCandidatesEmptyDomain(t *testing.T) {
	t.Parallel()
	b := newTestBruteforcer([]string{"www", "mail"})
	candidates := b.GenerateCandidates("")
	if len(candidates) != 2 {
		t.Fatalf("len = %d, want 2", len(candidates))
	}
	if candidates[0] != "www." {
		t.Errorf("candidates[0] = %q, want %q", candidates[0], "www.")
	}
}

func TestGenerateCandidatesDomainWithDot(t *testing.T) {
	t.Parallel()
	b := newTestBruteforcer([]string{"www"})
	candidates := b.GenerateCandidates("sub.example.com")
	if len(candidates) != 1 {
		t.Fatalf("len = %d, want 1", len(candidates))
	}
	if candidates[0] != "www.sub.example.com" {
		t.Errorf("candidates[0] = %q, want %q", candidates[0], "www.sub.example.com")
	}
}

func TestGenerateCandidatesSpecialChars(t *testing.T) {
	t.Parallel()
	b := newTestBruteforcer([]string{"test-123", "test_456"})
	candidates := b.GenerateCandidates("example.com")
	if len(candidates) != 2 {
		t.Fatalf("len = %d, want 2", len(candidates))
	}
	if candidates[0] != "test-123.example.com" {
		t.Errorf("candidates[0] = %q, want %q", candidates[0], "test-123.example.com")
	}
}

func TestGenerateCandidatesDeterministic(t *testing.T) {
	t.Parallel()
	b1 := newTestBruteforcer([]string{"www", "mail", "api"})
	b2 := newTestBruteforcer([]string{"www", "mail", "api"})
	c1 := b1.GenerateCandidates("example.com")
	c2 := b2.GenerateCandidates("example.com")
	if !slices.Equal(c1, c2) {
		t.Error("candidates not deterministic")
	}
}

// ─── WordlistSize ───────────────────────────────────────.

func TestWordlistSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		words []string
		want  int
	}{
		{[]string{"a", "b", "c"}, 3},
		{[]string{}, 0},
		{nil, 0},
		{make([]string, 1000), 1000},
	}
	for _, tc := range tests {
		b := newTestBruteforcer(tc.words)
		if got := b.WordlistSize(); got != tc.want {
			t.Errorf("WordlistSize = %d, want %d", got, tc.want)
		}
	}
}

// ─── NewBruteforcerFromFile ─────────────────────────────.

func TestNewBruteforcerFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "wordlist.txt")
	content := "www\nmail\napi\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := NewBruteforcerFromFile(path, nil, 0, 0, 0)
	if err != nil {
		t.Fatalf("NewBruteforcerFromFile: %v", err)
	}
	if b.WordlistSize() != 3 {
		t.Fatalf("WordlistSize = %d, want 3", b.WordlistSize())
	}

	candidates := b.GenerateCandidates("example.com")
	if len(candidates) != 3 {
		t.Fatalf("candidates len = %d, want 3", len(candidates))
	}

	expected := []string{"www.example.com", "mail.example.com", "api.example.com"}
	for i, c := range candidates {
		if c != expected[i] {
			t.Errorf("candidates[%d] = %q, want %q", i, c, expected[i])
		}
	}
}

func TestNewBruteforcerFromFileNotExist(t *testing.T) {
	t.Parallel()
	_, err := NewBruteforcerFromFile("/nonexistent/path/words.txt", nil, 0, 0, 0)
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestNewBruteforcerFromFileEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := NewBruteforcerFromFile(path, nil, 0, 0, 0)
	if err != nil {
		t.Fatalf("NewBruteforcerFromFile: %v", err)
	}
	if b.WordlistSize() != 0 {
		t.Errorf("WordlistSize = %d, want 0", b.WordlistSize())
	}
}

func TestNewBruteforcerFromFileCommentsOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "comments.txt")
	content := "# comment 1\n# comment 2\n\n  \n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := NewBruteforcerFromFile(path, nil, 0, 0, 0)
	if err != nil {
		t.Fatalf("NewBruteforcerFromFile: %v", err)
	}
	if b.WordlistSize() != 4 {
		t.Errorf("WordlistSize = %d, want 4 (lines are read as-is)", b.WordlistSize())
	}
}

func TestNewBruteforcerFromFileWhitespaceTrim(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "trim.txt")
	content := "  www  \n\tmail\t\n  api\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := NewBruteforcerFromFile(path, nil, 0, 0, 0)
	if err != nil {
		t.Fatalf("NewBruteforcerFromFile: %v", err)
	}
	if b.WordlistSize() != 3 {
		t.Fatalf("WordlistSize = %d, want 3", b.WordlistSize())
	}

	candidates := b.GenerateCandidates("example.com")
	if candidates[0] != testCandidate {
		t.Errorf("untrimmed: %q", candidates[0])
	}
}

// ─── Run ────────────────────────────────────────────────.

func TestBruteforcerRunEmptyWordlist(t *testing.T) {
	t.Parallel()
	b := newTestBruteforcer([]string{})
	results := make(chan string, 10)
	b.Run(t.Context(), Config{
		Domain:  "example.com",
		Threads: 5,
		Results: results,
	})
	close(results)

	count := 0
	for range results {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 results for empty wordlist, got %d", count)
	}
}

// ─── Large wordlist ─────────────────────────────────────.

func TestBruteforcerLargeWordlist(t *testing.T) {
	t.Parallel()
	words := make([]string, 10000)
	for i := range 10000 {
		words[i] = "w" + strings.Repeat("0", i%10)
	}
	b := newTestBruteforcer(words)
	if b.WordlistSize() != 10000 {
		t.Errorf("WordlistSize = %d, want 10000", b.WordlistSize())
	}
}

// ─── Duplicate words ────────────────────────────────────.

func TestBruteforcerDuplicateWords(t *testing.T) {
	t.Parallel()
	words := []string{"www", "www", "www"}
	b := newTestBruteforcer(words)
	candidates := b.GenerateCandidates("example.com")
	if len(candidates) != 1 {
		t.Errorf("len = %d, want 1 (deduplicated)", len(candidates))
	}
	if candidates[0] != testCandidate {
		t.Errorf("candidate = %q, want www.example.com", candidates[0])
	}
}

// ─── Concurrency ────────────────────────────────────────.

func TestBruteforcerConcurrentGenerate(t *testing.T) {
	t.Parallel()
	b := newTestBruteforcer([]string{"www", "mail", "api"})
	done := make(chan bool, 20)
	for range 20 {
		go func() {
			c := b.GenerateCandidates("example.com")
			_ = c
			done <- true
		}()
	}
	for range 20 {
		<-done
	}
}

// ─── Benchmarks ─────────────────────────────────────────.

func BenchmarkBruteforcerGenerate100(b *testing.B) {
	words := make([]string, 100)
	for i := range 100 {
		words[i] = "word" + strings.Repeat("x", i%10)
	}
	bencher := newTestBruteforcer(words)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		bencher.GenerateCandidates("example.com")
	}
}

func BenchmarkBruteforcerGenerate1000(b *testing.B) {
	words := make([]string, 1000)
	for i := range 1000 {
		words[i] = "word" + strings.Repeat("x", i%10)
	}
	bencher := newTestBruteforcer(words)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		bencher.GenerateCandidates("example.com")
	}
}

func BenchmarkBruteforcerRun100(b *testing.B) {
	words := make([]string, 100)
	for i := range 100 {
		words[i] = "word" + strings.Repeat("x", i%10)
	}
	bencher := newTestBruteforcer(words)
	results := make(chan string, 100)
	b.ResetTimer()
	for range b.N {
		bencher.Run(b.Context(), Config{
			Domain:  "example.com",
			Threads: 5,
			Results: results,
		})
	}
	close(results)
}

// ─── loadWordlist edge cases ────────────────────────────.

func TestLoadWordlistPathological(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		wantOk  bool
	}{
		{"normal", "www\nmail\n", true},
		{"large lines", strings.Repeat("a\n", 5000), true},
		{"unicode", "café\nüber\n", true},
		{"dots", "www.test\nv2.api\n", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(dir, tc.name+".txt")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			words, err := loadWordlist(path)
			if err != nil {
				t.Fatalf("loadWordlist: %v", err)
			}
			if len(words) == 0 {
				t.Error("empty wordlist")
			}
		})
	}
}

// ─── Example tests ──────────────────────────────────────.

func ExampleNewBruteforcer() {
	b := newTestBruteforcer([]string{"www", "mail"})
	candidates := b.GenerateCandidates("example.com")
	slices.Sort(candidates)
	for _, c := range candidates {
		fmt.Println(c)
	}
	// Output:
	// mail.example.com
	// www.example.com
}
