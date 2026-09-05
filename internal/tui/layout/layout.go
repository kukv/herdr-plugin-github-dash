// Package layout fits rendered text to the terminal.
package layout

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// ClipLines cuts every line of s to w display columns, marking a cut line with
// an ellipsis. Japanese takes two columns per character, so the count is never
// a byte or a rune count. A w of zero or less means the caller has no width
// yet — before the first tea.WindowSizeMsg — and s is returned unchanged.
func ClipLines(s string, w int) string {
	if w <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, w, "…")
	}
	return strings.Join(lines, "\n")
}
