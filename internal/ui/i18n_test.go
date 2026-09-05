package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/language"

	"github.com/kukv/octoscope/internal/i18n"
)

// TestJapaneseListViewFitsTheWidth guards that the Japanese catalog strings
// fit within 80 display columns; listView renders m.width verbatim and does
// no rune-counting or truncation of its own.
func TestJapaneseListViewFitsTheWidth(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })
	i18n.SetLanguage(language.Japanese)

	const width = 80
	f := &fakeSource{prs: samplePRs()}
	m := loadedModel(f)

	for _, line := range strings.Split(m.View().Content, "\n") {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("line is %d columns wide, want <= %d: %q", w, width, line)
		}
	}
}

// TestNoUnresolvedIDsInEitherCatalog guards spec §6.5: a message ID missing
// from a catalog must be caught by tests, not discovered as a "!id" string
// leaking into the rendered UI.
func TestNoUnresolvedIDsInEitherCatalog(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })

	for _, lang := range []language.Tag{language.English, language.Japanese} {
		i18n.SetLanguage(lang)
		for _, id := range i18n.IDs() {
			if got := i18n.T(id); strings.HasPrefix(got, "!") {
				t.Errorf("lang %s: id %q rendered as unresolved: %q", lang, id, got)
			}
		}
	}
}
