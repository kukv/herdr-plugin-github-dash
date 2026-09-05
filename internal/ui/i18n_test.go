package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/language"

	"github.com/kukv/octoscope/internal/gh"
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

// TestNoUnresolvedIDsInRenderedViews guards spec §6.5. It renders each screen
// in both languages and fails when a message ID the code asked for is missing
// from that language's catalog. Walking i18n.IDs() cannot catch this: it only
// proves the catalog can resolve its own IDs, never that the IDs the code
// spells match them.
func TestNoUnresolvedIDsInRenderedViews(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })

	for _, lang := range []language.Tag{language.English, language.Japanese} {
		i18n.SetLanguage(lang)
		for name, view := range renderEveryScreen() {
			t.Run(lang.String()+"/"+name, func(t *testing.T) {
				i18n.AssertNoUnresolvedIDs(t, view)
			})
		}
	}
}

// step applies one key to m and feeds back whatever command it produced, so
// screens that need a round trip (the list tab, the label picker) settle.
func step(m Model, k string) Model {
	next, cmd := m.Update(key(k))
	m = next.(Model)
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	next, _ = m.Update(msg)
	return next.(Model)
}

// renderEveryScreen renders every screen the ui package can show, so the
// message IDs on each path are exercised at least once.
func renderEveryScreen() map[string]string {
	f := &fakeSource{
		prs:    samplePRs(),
		issues: []gh.Issue{{Number: 3, Title: "an issue"}},
		// State must be OPEN or the confirm screen has nothing to offer.
		pr:     gh.PR{Number: 1, Title: "first pr", State: "OPEN"},
		labels: []gh.Label{{Name: "bug", Color: "ff0000"}},
	}

	list := loadedModel(f)
	detail := detailModel(f)

	// Toggle a label and press enter without resolving the resulting cmd, so
	// the picker is caught mid "applying" render rather than already settled.
	picker := step(detail, "l")
	next, _ := picker.Update(key("space"))
	picker = next.(Model)
	next, _ = picker.Update(key("enter"))
	picker = next.(Model)

	return map[string]string{
		"list_prs":    list.View().Content,
		"list_issues": step(list, "tab").View().Content,
		"detail":      detail.View().Content,
		"compose":     step(detail, "c").View().Content,
		"confirm":     step(detail, "x").View().Content,
		"picker":      picker.View().Content,
		"error":       errorView("boom"),
	}
}
