package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kukv/octoscope/internal/i18n"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true)
	activeTabStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	dimStyle       = lipgloss.NewStyle().Faint(true)
)

func (m Model) View() tea.View {
	var content string
	switch {
	case m.errText != "":
		content = m.errorView()
	case m.showingDetail:
		content = m.detail.View()
	default:
		content = clipLines(m.tabRow(), m.width) + "\n\n" + m.activeTab()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) activeTab() string {
	if m.tab == tabRepos {
		return m.repo.View()
	}
	return m.work.View()
}

// tabRow labels each tab with the key that reaches it. Without a target
// repository the Repos tab is not offered at all (spec 3.4).
func (m Model) tabRow() string {
	labels := []string{"1 " + i18n.T("tab.work")}
	if m.opts.HasRepo {
		labels = append(labels, "2 "+i18n.T("tab.repos"))
	}

	for i, label := range labels {
		if tabID(i) == m.tab {
			labels[i] = activeTabStyle.Render(label)
		} else {
			labels[i] = dimStyle.Render(label)
		}
	}
	return strings.Join(labels, "  ")
}

// errorView shows the failure that stopped the run. The heading and the key
// hint are ours and are cut to the terminal width; the message itself came
// from gh or GitHub and is all the user has to go on, so it is wrapped rather
// than cut short (.claude/rules/errors.md).
func (m Model) errorView() string {
	return clipLines(titleStyle.Render(i18n.T("app.error_title")), m.width) + "\n\n" +
		wrap(m.errText, m.width) + "\n\n" +
		clipLines(dimStyle.Render(i18n.T("footer.error")), m.width)
}

// clipLines cuts every line of s to w display columns. Japanese takes two
// columns per character, so the count is never a byte or a rune count. Before
// the first tea.WindowSizeMsg there is no width to clip to.
func clipLines(s string, w int) string {
	if w <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, w, "…")
	}
	return strings.Join(lines, "\n")
}

func wrap(s string, w int) string {
	if w <= 0 {
		return s
	}
	return ansi.Wrap(s, w, "")
}
