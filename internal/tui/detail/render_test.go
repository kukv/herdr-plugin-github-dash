package detail

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/text/language"

	"github.com/kukv/octoscope/internal/gh"
	"github.com/kukv/octoscope/internal/i18n"
)

func TestPRMarkdownContainsMetaBodyAndComments(t *testing.T) {
	pr := gh.PR{
		Number: 12, Title: "feat: pane", Author: gh.Author{Login: "kukv"},
		State: "OPEN", IsDraft: true, ReviewDecision: "REVIEW_REQUIRED",
		Labels: []gh.Label{{Name: "Kind: Feature"}},
		Body:   "body text",
		Comments: []gh.Comment{
			{
				Author: gh.Author{Login: "bob"}, Body: "comment text",
				CreatedAt: time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC),
			},
		},
	}
	md := prMarkdown(pr)
	for _, want := range []string{
		"#12", "feat: pane", "@kukv", "OPEN (draft)",
		"REVIEW_REQUIRED", "Kind: Feature", "body text", "@bob", "comment text",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestIssueMarkdownEmptyBody(t *testing.T) {
	md := issueMarkdown(gh.Issue{Number: 3, Title: "an issue"})
	if !strings.Contains(md, "_no description_") {
		t.Errorf("markdown missing empty-body placeholder:\n%s", md)
	}
}

// TestNoUnresolvedIDsInRenderedViews guards spec §6.5. It renders each of the
// view's screens in both languages and fails when a message ID the code asked
// for is missing from that language's catalog. Walking i18n.IDs() cannot catch
// this: it only proves the catalog can resolve its own IDs, never that the IDs
// the code spells match them.
func TestNoUnresolvedIDsInRenderedViews(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })

	for _, lang := range []language.Tag{language.English, language.Japanese} {
		i18n.SetLanguage(lang)
		for name, view := range renderEveryScreen(t) {
			t.Run(lang.String()+"/"+name, func(t *testing.T) {
				i18n.AssertNoUnresolvedIDs(t, view)
			})
		}
	}
}

// renderEveryScreen renders every screen the detail view can show, so the
// message IDs on each path are exercised at least once.
func renderEveryScreen(t *testing.T) map[string]string {
	t.Helper()
	f := &fakeSource{
		// State must be OPEN or the confirm screen has nothing to offer.
		pr:     gh.PR{Number: 1, Title: "first pr", State: "OPEN"},
		labels: []gh.Label{{Name: "bug", Color: "ff0000"}},
	}
	detail := loaded(f, prRef())

	compose, _ := detail.Update(key("c"))
	confirm, _ := detail.Update(key("x"))

	// Toggle a label and press enter without resolving the resulting cmd, so
	// the picker is caught mid "applying" render rather than already settled.
	picker := openPicker(t, f, prRef(), "l")
	picker, _ = picker.Update(key("space"))
	picker, _ = picker.Update(key("enter"))

	return map[string]string{
		"loading": New(f, prRef()).View(),
		"detail":  detail.View(),
		"compose": compose.View(),
		"confirm": confirm.View(),
		"picker":  picker.View(),
	}
}
