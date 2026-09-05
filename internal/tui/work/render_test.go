package work

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/language"

	"github.com/kukv/octoscope/internal/gh"
	"github.com/kukv/octoscope/internal/i18n"
)

// overlongWork is sampleWork plus a card whose title is wider than any
// terminal the width test uses, in both scripts. Without it the fixture's
// longest line is 17 columns and every regime has room to spare, so the width
// test would pass even with the truncation removed.
func overlongWork() gh.Work {
	w := sampleWork()
	long := w[gh.SectionReviewRequested][0]
	long.Ref = gh.ItemRef{Kind: gh.ItemPR, Repo: "kukv/a-repository-with-a-name-nobody-would-choose", Number: 999}
	long.Title = "レンダリングのパイプラインをまるごと置き換える refactor that nobody asked for"
	w[gh.SectionReviewRequested] = append(w[gh.SectionReviewRequested], long)
	return w
}

// overlong returns a loaded model whose cursor sits on the overlong card, so
// the drawer renders it too.
func overlong() Model {
	m := New(&fakeSource{work: overlongWork()})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = m.Update(workMsg(overlongWork()))
	return press(press(m, "j"), "j")
}

func TestViewShowsEveryColumnHeading(t *testing.T) {
	out := loaded().View()
	for _, want := range []string{
		i18n.T("work.review_requested"),
		i18n.T("work.your_prs"),
		i18n.T("work.assigned"),
		i18n.T("work.mentioned"),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view is missing the %q heading", want)
		}
	}
}

func TestViewShowsTheSelectedCardInTheDrawer(t *testing.T) {
	if out := press(loaded(), "j").View(); !strings.Contains(out, "kukv/koto#3") {
		t.Errorf("drawer does not name the selected card:\n%s", out)
	}
}

func TestEmptyColumnSaysSo(t *testing.T) {
	if out := loaded().View(); !strings.Contains(out, i18n.T("work.empty_column")) {
		t.Errorf("no empty-column marker for Your PRs:\n%s", out)
	}
}

func TestNarrowTerminalDropsTheDrawer(t *testing.T) {
	m := loaded()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	if strings.Contains(m.View(), "kukv/octoscope#12") {
		t.Error("the drawer is still drawn at 80 columns")
	}
}

func TestVeryNarrowTerminalShowsOneColumn(t *testing.T) {
	m := loaded()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 50, Height: 40})
	out := m.View()
	if strings.Contains(out, i18n.T("work.mentioned")) {
		t.Error("all four headings are drawn at 50 columns")
	}
	if !strings.Contains(out, i18n.T("work.review_requested")) {
		t.Error("the current column's heading is missing")
	}
	if !strings.Contains(out, i18n.Tf("work.column_position", map[string]any{"Index": 1, "Total": 4})) {
		t.Errorf("the single column does not say which column it is:\n%s", out)
	}
}

func TestLoadingBoardSaysSo(t *testing.T) {
	m := New(&fakeSource{work: sampleWork()})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = m.Refresh()
	t.Cleanup(m.Cancel)
	if out := m.View(); !strings.Contains(out, i18n.T("common.loading")) {
		t.Errorf("a loading board does not say so:\n%s", out)
	}
}

func TestUnsizedBoardRendersNothing(t *testing.T) {
	if out := New(&fakeSource{}).View(); out != "" {
		t.Errorf("got %q, want an empty string before the first WindowSizeMsg", out)
	}
}

func TestNoLineExceedsTheTerminalWidth(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })

	for _, lang := range []language.Tag{language.English, language.Japanese} {
		i18n.SetLanguage(lang)
		for _, width := range []int{50, 80, 100, 120} {
			for name, base := range map[string]func() Model{"sample": loaded, "overlong": overlong} {
				m := base()
				m, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
				for _, line := range strings.Split(m.View(), "\n") {
					if w := ansi.StringWidth(line); w > width {
						t.Errorf("%s lang %s width %d: line is %d columns: %q",
							name, lang, width, w, line)
					}
				}
			}
		}
	}
}

func TestNoUnresolvedIDsInTheWorkView(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })

	for _, lang := range []language.Tag{language.English, language.Japanese} {
		i18n.SetLanguage(lang)
		i18n.AssertNoUnresolvedIDs(t, loaded().View())

		empty := New(&fakeSource{})
		empty, _ = empty.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		i18n.AssertNoUnresolvedIDs(t, empty.View())

		loading, _ := empty.Refresh()
		t.Cleanup(loading.Cancel)
		i18n.AssertNoUnresolvedIDs(t, loading.View())
	}
}
