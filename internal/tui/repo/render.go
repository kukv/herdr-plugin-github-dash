package repo

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kukv/octoscope/internal/gh"
	"github.com/kukv/octoscope/internal/i18n"
	"github.com/kukv/octoscope/internal/tui/icon"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true)
	activeTabStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	dimStyle       = lipgloss.NewStyle().Faint(true)
)

func (m Model) View() string {
	var b strings.Builder
	title := i18n.T("app.name")
	if m.repoName != "" {
		title += " — " + m.repoName
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	prTab, issueTab := i18n.T("list.tab_prs"), i18n.T("list.tab_issues")
	if m.tab == tabPRs {
		prTab = activeTabStyle.Render(prTab)
		issueTab = dimStyle.Render(issueTab)
	} else {
		issueTab = activeTabStyle.Render(issueTab)
		prTab = dimStyle.Render(prTab)
	}
	b.WriteString(prTab + "  " + issueTab + "\n\n")

	switch {
	case m.loading[m.tab]:
		b.WriteString(m.spin.View() + " " + i18n.T("common.loading") + "\n")
	case m.tab == tabPRs && len(m.prs) == 0:
		b.WriteString(dimStyle.Render(i18n.T("list.no_open_prs")) + "\n")
	case m.tab == tabIssues && len(m.issues) == 0:
		b.WriteString(dimStyle.Render(i18n.T("list.no_open_issues")) + "\n")
	case m.tab == tabPRs:
		now := time.Now()
		for i, pr := range m.prs {
			b.WriteString(cursorPrefix(i == m.cursors[tabPRs]) + prLine(pr, now) + "\n")
		}
	default:
		now := time.Now()
		for i, issue := range m.issues {
			b.WriteString(cursorPrefix(i == m.cursors[tabIssues]) + issueLine(issue, now) + "\n")
		}
	}

	b.WriteString("\n" + dimStyle.Render(i18n.T("footer.list")))
	return clipLines(b.String(), m.width)
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

func cursorPrefix(selected bool) string {
	if selected {
		return "▸ "
	}
	return "  "
}

func prLine(pr gh.PR, now time.Time) string {
	return fmt.Sprintf("#%-5d %s  @%s  %s %s",
		pr.Number, pr.Title, pr.Author.Login,
		icon.Review(gh.ParseReviewDecision(pr.ReviewDecision), pr.IsDraft),
		i18n.RelTime(now, pr.UpdatedAt))
}

func issueLine(issue gh.Issue, now time.Time) string {
	return fmt.Sprintf("#%-5d %s  @%s  %s",
		issue.Number, issue.Title, issue.Author.Login, i18n.RelTime(now, issue.UpdatedAt))
}
