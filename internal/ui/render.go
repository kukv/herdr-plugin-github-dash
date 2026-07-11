package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/kukv/herdr-plugin-github-dash/internal/ghcli"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true)
	activeTabStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	dimStyle       = lipgloss.NewStyle().Faint(true)
)

func (m Model) listView() string {
	var b strings.Builder
	title := "GitHub Dash"
	if m.repoName != "" {
		title += " — " + m.repoName
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	prTab, issueTab := "Pull Requests", "Issues"
	if m.tab == tabPRs {
		prTab = activeTabStyle.Render(prTab)
		issueTab = dimStyle.Render(issueTab)
	} else {
		issueTab = activeTabStyle.Render(issueTab)
		prTab = dimStyle.Render(prTab)
	}
	b.WriteString(prTab + "  " + issueTab + "\n\n")

	switch {
	case m.loading:
		b.WriteString(m.spin.View() + " loading...\n")
	case m.tab == tabPRs && len(m.prs) == 0:
		b.WriteString(dimStyle.Render("No open pull requests") + "\n")
	case m.tab == tabIssues && len(m.issues) == 0:
		b.WriteString(dimStyle.Render("No open issues") + "\n")
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

	b.WriteString("\n" + dimStyle.Render("j/k:move  enter:detail  tab:PR/Issue  r:refresh  o:browser  q:quit"))
	return b.String()
}

// detailView は Task 4 で完成させる。
func (m Model) detailView() string {
	if m.loading {
		return m.spin.View() + " loading...\n"
	}
	return m.detailTitle + "\n"
}

func errorView(text string) string {
	return titleStyle.Render("GitHub Dash — error") + "\n\n" + text + "\n\n" +
		dimStyle.Render("q:quit")
}

func cursorPrefix(selected bool) string {
	if selected {
		return "▸ "
	}
	return "  "
}

func prLine(pr ghcli.PR, now time.Time) string {
	return fmt.Sprintf("#%-5d %s  @%s  %s %s",
		pr.Number, pr.Title, pr.Author.Login, reviewIcon(pr), relTime(now, pr.UpdatedAt))
}

func issueLine(issue ghcli.Issue, now time.Time) string {
	return fmt.Sprintf("#%-5d %s  @%s  %s",
		issue.Number, issue.Title, issue.Author.Login, relTime(now, issue.UpdatedAt))
}

func reviewIcon(pr ghcli.PR) string {
	if pr.IsDraft {
		return "◌"
	}
	switch pr.ReviewDecision {
	case "APPROVED":
		return "✓"
	case "CHANGES_REQUESTED":
		return "×"
	default:
		return "•"
	}
}

func relTime(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
