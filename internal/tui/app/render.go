package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
		content = errorView(m.errText)
	case m.showingDetail:
		content = m.detail.View()
	default:
		content = m.tabRow() + "\n\n" + m.activeTab()
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

func errorView(text string) string {
	return titleStyle.Render(i18n.T("app.error_title")) + "\n\n" + text + "\n\n" +
		dimStyle.Render(i18n.T("footer.error"))
}
