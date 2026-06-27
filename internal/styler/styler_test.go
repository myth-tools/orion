package styler

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// ─── Style Rendering ──────────────────────────────────.

func hasAnsi(s string) bool {
	return strings.Contains(s, "\x1b[")
}

func TestStylesRenderColored(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		style lipgloss.Style
	}{
		{"Cyan", Cyan},
		{"Green", Green},
		{"Yellow", Yellow},
		{"Red", Red},
		{"Magenta", Magenta},
		{"White", White},
		{"Dim", Dim},
		{"Bold", Bold},
		{"BoldCyan", BoldCyan},
		{"BoldGreen", BoldGreen},
		{"BoldYellow", BoldYellow},
		{"BoldRed", BoldRed},
	}

	for _, tc := range tests {
		got := tc.style.Render("test")
		if !hasAnsi(got) && !strings.Contains(got, "test") {
			t.Errorf("%s.Render(\"test\") = %q, should contain 'test'", tc.name, got)
		}
	}
}

// ─── Fprint / Fprintf / Fprintln ──────────────────────.

func TestFprint(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	Fprint(&buf, Bold, "hello", " ", "world")
	got := buf.String()
	if !strings.Contains(got, "hello world") {
		t.Errorf("Fprint output = %q, want 'hello world'", got)
	}
}

func TestFprintf(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	Fprintf(&buf, Green, "count=%d", 42)
	got := buf.String()
	if !strings.Contains(got, "count=42") {
		t.Errorf("Fprintf output = %q, want 'count=42'", got)
	}
}

func TestFprintln(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	Fprintln(&buf, Red, "error msg")
	got := buf.String()
	if !strings.Contains(got, "error msg") {
		t.Errorf("Fprintln output = %q, want 'error msg'", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Fprintln output = %q, want trailing newline", got)
	}
}

// ─── Sprint / Sprintf ─────────────────────────────────.

func TestSprint(t *testing.T) {
	t.Parallel()
	got := Sprint(Bold, "bold", " ", "text")
	if !strings.Contains(got, "bold text") {
		t.Errorf("Sprint output = %q, want 'bold text'", got)
	}
}

func TestSprintf(t *testing.T) {
	t.Parallel()
	got := Sprintf(Dim, "value=%d (%s)", 99, "ok")
	if !strings.Contains(got, "value=99 (ok)") {
		t.Errorf("Sprintf output = %q, want 'value=99 (ok)'", got)
	}
}

// ─── DimNote ──────────────────────────────────────────.

func TestDimNote(t *testing.T) {
	t.Parallel()
	got := DimNote("some dim text")
	if !strings.Contains(got, "some dim text") {
		t.Errorf("DimNote = %q, want 'some dim text'", got)
	}
}

// ─── Bullet ───────────────────────────────────────────.

func TestBullet(t *testing.T) {
	t.Parallel()
	got := Bullet("a bullet point")
	if !strings.Contains(got, "•") {
		t.Errorf("Bullet = %q, want bullet character", got)
	}
	if !strings.Contains(got, "a bullet point") {
		t.Errorf("Bullet = %q, want 'a bullet point'", got)
	}
}

// ─── Section ──────────────────────────────────────────.

func TestSection(t *testing.T) {
	t.Parallel()
	got := Section("Usage")
	if !strings.Contains(got, "Usage") {
		t.Errorf("Section = %q, want 'Usage'", got)
	}
	if !strings.Contains(got, "─") {
		t.Errorf("Section = %q, want ruler dashes", got)
	}
}

func TestSectionWithArgs(t *testing.T) {
	t.Parallel()
	got := Section("Results %d", 42)
	if !strings.Contains(got, "Results 42") {
		t.Errorf("Section with args = %q, want 'Results 42'", got)
	}
}

// ─── Badges ───────────────────────────────────────────.

func TestBadgePass(t *testing.T) {
	t.Parallel()
	got := BadgePass("all good")
	if !strings.Contains(got, "[PASS]") {
		t.Errorf("BadgePass = %q, want '[PASS]'", got)
	}
	if !strings.Contains(got, "all good") {
		t.Errorf("BadgePass = %q, want 'all good'", got)
	}
}

func TestBadgeWarn(t *testing.T) {
	t.Parallel()
	got := BadgeWarn("caution")
	if !strings.Contains(got, "[WARN]") {
		t.Errorf("BadgeWarn = %q, want '[WARN]'", got)
	}
	if !strings.Contains(got, "caution") {
		t.Errorf("BadgeWarn = %q, want 'caution'", got)
	}
}

func TestBadgeFail(t *testing.T) {
	t.Parallel()
	got := BadgeFail("broken")
	if !strings.Contains(got, "[FAIL]") {
		t.Errorf("BadgeFail = %q, want '[FAIL]'", got)
	}
	if !strings.Contains(got, "broken") {
		t.Errorf("BadgeFail = %q, want 'broken'", got)
	}
}

// ─── Info / Warn / Error ──────────────────────────────.

func TestInfo(t *testing.T) {
	t.Parallel()
	got := Info("scanning")
	if !strings.Contains(got, "[+]") {
		t.Errorf("Info = %q, want '[+]'", got)
	}
	if !strings.Contains(got, "scanning") {
		t.Errorf("Info = %q, want 'scanning'", got)
	}
}

func TestWarn(t *testing.T) {
	t.Parallel()
	got := Warn("rate limit")
	if !strings.Contains(got, "[!]") {
		t.Errorf("Warn = %q, want '[!]'", got)
	}
	if !strings.Contains(got, "rate limit") {
		t.Errorf("Warn = %q, want 'rate limit'", got)
	}
}

func TestError(t *testing.T) {
	t.Parallel()
	got := Error("not found")
	if !strings.Contains(got, "Error:") {
		t.Errorf("Error = %q, want 'Error:'", got)
	}
	if !strings.Contains(got, "not found") {
		t.Errorf("Error = %q, want 'not found'", got)
	}
}

// ─── FmtInfo / FmtWarn / FmtError ─────────────────────.

func TestFmtInfo(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	FmtInfo(&buf, "started %s", "scan")
	got := buf.String()
	if !strings.Contains(got, "[+]") {
		t.Errorf("FmtInfo = %q, want '[+]'", got)
	}
	if !strings.Contains(got, "started scan") {
		t.Errorf("FmtInfo = %q, want 'started scan'", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("FmtInfo = %q, want trailing newline", got)
	}
}

func TestFmtWarn(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	FmtWarn(&buf, "timeout %ds", 30)
	got := buf.String()
	if !strings.Contains(got, "[!]") {
		t.Errorf("FmtWarn = %q, want '[!]'", got)
	}
	if !strings.Contains(got, "timeout 30s") {
		t.Errorf("FmtWarn = %q, want 'timeout 30s'", got)
	}
}

func TestFmtError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	FmtError(&buf, "failed: %v", "err")
	got := buf.String()
	if !strings.Contains(got, "Error:") {
		t.Errorf("FmtError = %q, want 'Error:'", got)
	}
	if !strings.Contains(got, "failed: err") {
		t.Errorf("FmtError = %q, want 'failed: err'", got)
	}
}

// ─── Separator ─────────────────────────────────────────.

func TestSeparator(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	Separator(&buf)
	got := buf.String()
	if !strings.Contains(got, "─") {
		t.Errorf("Separator = %q, want dashes", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Separator = %q, want trailing newline", got)
	}
}

// ─── BorderedBox ───────────────────────────────────────.

func TestBorderedBox(t *testing.T) {
	t.Parallel()
	got := BorderedBox("hello\nworld")
	if !strings.Contains(got, "hello") {
		t.Errorf("BorderedBox = %q, want 'hello'", got)
	}
	if !strings.Contains(got, "world") {
		t.Errorf("BorderedBox = %q, want 'world'", got)
	}
	if !strings.Contains(got, "╭") || !strings.Contains(got, "╰") {
		t.Errorf("BorderedBox = %q, want rounded border corners", got)
	}
}

// ─── TableHeader / TableRow ────────────────────────────.

func TestTableHeader(t *testing.T) {
	t.Parallel()
	got := TableHeader("Source", "Results")
	if !strings.Contains(got, "Source") {
		t.Errorf("TableHeader = %q, want 'Source'", got)
	}
	if !strings.Contains(got, "Results") {
		t.Errorf("TableHeader = %q, want 'Results'", got)
	}
}

func TestTableRow(t *testing.T) {
	t.Parallel()
	got := TableRow("alienvault", "42")
	if !strings.Contains(got, "alienvault") {
		t.Errorf("TableRow = %q, want 'alienvault'", got)
	}
	if !strings.Contains(got, "42") {
		t.Errorf("TableRow = %q, want '42'", got)
	}
}

// ─── Integration: composed output ──────────────────────.

func TestComposedOutput(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	Fprintf(&buf, Bold, "  Stats")
	Separator(&buf)

	line := Info("found 10 results")
	buf.WriteString(line + "\n")

	got := buf.String()
	if !strings.Contains(got, "Stats") {
		t.Errorf("composed output = %q, want 'Stats'", got)
	}
	if !strings.Contains(got, "[+]") {
		t.Errorf("composed output = %q, want '[+]'", got)
	}
	if !strings.Contains(got, "found 10 results") {
		t.Errorf("composed output = %q, want 'found 10 results'", got)
	}
}

// ─── Empty / edge cases ───────────────────────────────.

func TestEmptyStrings(t *testing.T) {
	t.Parallel()

	if got := Info(""); !strings.Contains(got, "[+]") {
		t.Errorf("Info(\"\") = %q, want '[+]' prefix", got)
	}
	if got := Section(""); got == "" {
		t.Error("Section(\"\") should not be empty")
	}

	var buf bytes.Buffer
	Fprintln(&buf, Bold)
	if buf.Len() == 0 {
		t.Error("Fprintln with empty args should output at least a newline")
	}
}

func TestLargeTextInBox(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 200)
	got := BorderedBox(long)
	if !strings.Contains(got, long) {
		t.Errorf("BorderedBox should contain the full text")
	}
}

// ─── HeaderBar ─────────────────────────────────────────.

func TestHeaderBar(t *testing.T) {
	t.Parallel()
	got := HeaderBar("Summary")
	if !strings.Contains(got, "Summary") {
		t.Errorf("HeaderBar = %q, want 'Summary'", got)
	}
	if !strings.Contains(got, "┌") || !strings.Contains(got, "└") {
		t.Errorf("HeaderBar = %q, want box corners", got)
	}
}
