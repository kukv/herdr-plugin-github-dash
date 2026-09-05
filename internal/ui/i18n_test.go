package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/language"

	tea "charm.land/bubbletea/v2"

	"github.com/kukv/octoscope/internal/i18n"
)

// TestJapaneseListViewFitsTheWidth guards against counting runes instead of
// display columns: Japanese characters occupy two columns each.
func TestJapaneseListViewFitsTheWidth(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })
	i18n.SetLanguage(language.Japanese)

	const width = 80
	f := &fakeSource{prs: samplePRs()}
	m := loadedModel(f)
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
	m = next.(Model)

	for _, line := range strings.Split(m.View().Content, "\n") {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("line is %d columns wide, want <= %d: %q", w, width, line)
		}
	}
}
