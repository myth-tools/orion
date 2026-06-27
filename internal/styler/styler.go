package styler

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// ANSI color indices (work on any terminal).
	Cyan       = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	Green      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	Yellow     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	Red        = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	Magenta    = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	White      = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	Dim        = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	Bold       = lipgloss.NewStyle().Bold(true)
	BoldCyan   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	BoldGreen  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	BoldYellow = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	BoldRed    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
)

func Fprint(w io.Writer, s lipgloss.Style, args ...any) {
	fmt.Fprint(w, s.Render(fmt.Sprint(args...)))
}

func Fprintf(w io.Writer, s lipgloss.Style, format string, args ...any) {
	fmt.Fprint(w, s.Render(fmt.Sprintf(format, args...)))
}

func Fprintln(w io.Writer, s lipgloss.Style, args ...any) {
	fmt.Fprintln(w, s.Render(fmt.Sprint(args...)))
}

func Sprint(s lipgloss.Style, args ...any) string {
	return s.Render(fmt.Sprint(args...))
}

func Sprintf(s lipgloss.Style, format string, args ...any) string {
	return s.Render(fmt.Sprintf(format, args...))
}

func DimNote(msg string) string {
	return Dim.Render(msg)
}

func Bullet(text string) string {
	return fmt.Sprintf("  %s %s", Cyan.Render("•"), Dim.Render(text))
}

func Section(titleFmt string, args ...any) string {
	title := fmt.Sprintf(titleFmt, args...)
	rule := strings.Repeat("─", 50)
	return fmt.Sprintf("\n  %s  %s", Bold.Render("▸ "+title), Dim.Render(rule))
}

func BadgePass(text string) string {
	return BoldGreen.Render("[PASS]") + " " + text
}

func BadgeWarn(text string) string {
	return BoldYellow.Render("[WARN]") + " " + text
}

func BadgeFail(text string) string {
	return BoldRed.Render("[FAIL]") + " " + text
}

func Info(text string) string {
	return Green.Render("[+]") + " " + text
}

func Warn(text string) string {
	return Yellow.Render("[!]") + " " + text
}

func Error(text string) string {
	return Red.Render("Error:") + " " + text
}

func FmtInfo(w io.Writer, format string, args ...any) {
	fmt.Fprintln(w, Green.Render("[+]")+" "+fmt.Sprintf(format, args...))
}

func FmtWarn(w io.Writer, format string, args ...any) {
	fmt.Fprintln(w, Yellow.Render("[!]")+" "+fmt.Sprintf(format, args...))
}

func FmtError(w io.Writer, format string, args ...any) {
	fmt.Fprintln(w, Red.Render("Error:")+" "+fmt.Sprintf(format, args...))
}

func Separator(w io.Writer) {
	fmt.Fprintln(w, "  "+Dim.Render(strings.Repeat("─", 50)))
}

func HeaderBar(title string) string {
	w := 56
	padded := " " + title + " "
	rule := strings.Repeat("─", w)
	return Bold.Render("┌"+rule+"┐") + "\n" +
		Bold.Render("│") + Cyan.Render(padded) + Bold.Render(strings.Repeat(" ", w-len(padded))) + Bold.Render("│") + "\n" +
		Bold.Render("└"+rule+"┘")
}

func BorderedBox(body string) string {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")).
		Padding(0, 1).
		Render(body)
}

func TableHeader(columns ...string) string {
	var b strings.Builder
	for _, c := range columns {
		b.WriteString(fmt.Sprintf("  %-24s", Bold.Render(c)))
	}
	return strings.TrimRight(b.String(), " ")
}

func TableRow(columns ...string) string {
	var b strings.Builder
	for _, c := range columns {
		b.WriteString(fmt.Sprintf("  %-24s", c))
	}
	return strings.TrimRight(b.String(), " ")
}
