package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kukv/octoscope/internal/ghcli"
	"github.com/kukv/octoscope/internal/i18n"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true)
	activeTabStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	dimStyle       = lipgloss.NewStyle().Faint(true)
)

func (m Model) listView() string {
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
	case m.listLoading[m.tab]:
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
	return b.String()
}

func (m Model) detailView() string {
	if m.composing {
		return m.composeView()
	}
	if m.confirming {
		return m.confirmView()
	}
	if m.picking {
		return m.pickerView()
	}
	if m.detailLoading || m.pickerLoading {
		return m.spin.View() + " " + i18n.T("common.loading") + "\n"
	}
	header := titleStyle.Render(m.detailTitle)
	footer := dimStyle.Render(i18n.T("footer.detail_prefix") + m.stateFooterKey() + i18n.T("footer.detail_suffix"))
	body := header + "\n" + m.detail.View() + "\n"
	if m.actionErr != "" {
		body += i18n.T("common.error_prefix") + m.actionErr + "\n"
	}
	return body + footer
}

func (m Model) pickerView() string {
	body := m.picker.listView(m.height)
	if m.applying {
		return body + "\n" + m.spin.View() + " " + i18n.T("picker.applying") + "\n"
	}
	return body + "\n" + dimStyle.Render(i18n.T("footer.picker"))
}

// stateFooterKey returns the state-aware footer hint (with trailing spaces),
// or "" when the item cannot change state (merged / not yet loaded).
func (m Model) stateFooterKey() string {
	closing, ok := m.stateAction()
	if !ok {
		return ""
	}
	if closing {
		return i18n.T("footer.close")
	}
	return i18n.T("footer.reopen")
}

func (m Model) confirmView() string {
	header := titleStyle.Render(m.detailTitle)
	closing, _ := m.stateAction()
	var id string
	switch {
	case m.detailTarget.Kind == KindPR && closing:
		id = "confirm.close_pr"
	case m.detailTarget.Kind == KindPR:
		id = "confirm.reopen_pr"
	case closing:
		id = "confirm.close_issue"
	default:
		id = "confirm.reopen_issue"
	}
	var b strings.Builder
	b.WriteString(header + "\n\n")
	b.WriteString(i18n.T(id))
	if m.working {
		b.WriteString(m.spin.View() + " " + i18n.T("confirm.working") + "\n")
	} else {
		b.WriteString(dimStyle.Render(i18n.T("confirm.yes_no")))
	}
	return b.String()
}

func (m Model) composeView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(i18n.Tf("compose.title", map[string]any{"Title": m.detailTitle})) + "\n\n")
	b.WriteString(m.textarea.View() + "\n\n")
	if m.postErr != "" {
		b.WriteString(i18n.T("common.error_prefix") + m.postErr + "\n\n")
	}
	if m.posting {
		b.WriteString(m.spin.View() + " " + i18n.T("compose.posting") + "\n")
	} else {
		b.WriteString(dimStyle.Render(i18n.T("footer.compose")))
	}
	return b.String()
}

func errorView(text string) string {
	return titleStyle.Render(i18n.T("app.error_title")) + "\n\n" + text + "\n\n" +
		dimStyle.Render(i18n.T("footer.error"))
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
		return i18n.T("time.now")
	case d < time.Hour:
		return i18n.Tn("time.minutes_ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return i18n.Tn("time.hours_ago", int(d.Hours()))
	default:
		return i18n.Tn("time.days_ago", int(d.Hours()/24))
	}
}

func prMarkdown(pr ghcli.PR) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# #%d %s\n\n", pr.Number, pr.Title)
	fmt.Fprintf(&b, "- **%s**: @%s\n", i18n.T("md.author"), pr.Author.Login)
	state := pr.State
	if pr.IsDraft {
		state += i18n.T("md.draft_suffix")
	}
	fmt.Fprintf(&b, "- **%s**: %s\n", i18n.T("md.state"), state)
	if pr.ReviewDecision != "" {
		fmt.Fprintf(&b, "- **%s**: %s\n", i18n.T("md.review"), pr.ReviewDecision)
	}
	writeCommonMeta(&b, pr.Labels, pr.UpdatedAt)
	writeBody(&b, pr.Body)
	writeComments(&b, pr.Comments)
	return b.String()
}

func issueMarkdown(issue ghcli.Issue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# #%d %s\n\n", issue.Number, issue.Title)
	fmt.Fprintf(&b, "- **%s**: @%s\n", i18n.T("md.author"), issue.Author.Login)
	fmt.Fprintf(&b, "- **%s**: %s\n", i18n.T("md.state"), issue.State)
	writeCommonMeta(&b, issue.Labels, issue.UpdatedAt)
	writeBody(&b, issue.Body)
	writeComments(&b, issue.Comments)
	return b.String()
}

func writeCommonMeta(b *strings.Builder, labels []ghcli.Label, updatedAt time.Time) {
	if len(labels) > 0 {
		names := make([]string, len(labels))
		for i, l := range labels {
			names[i] = l.Name
		}
		fmt.Fprintf(b, "- **%s**: %s\n", i18n.T("md.labels"), strings.Join(names, ", "))
	}
	fmt.Fprintf(b, "- **%s**: %s\n", i18n.T("md.updated"), i18n.DateTime(updatedAt))
}

func writeBody(b *strings.Builder, body string) {
	b.WriteString("\n---\n\n")
	if body != "" {
		b.WriteString(body)
	} else {
		b.WriteString(i18n.T("md.no_description"))
	}
}

func writeComments(b *strings.Builder, comments []ghcli.Comment) {
	for _, c := range comments {
		fmt.Fprintf(b, "\n\n---\n\n**@%s** — %s\n\n%s",
			c.Author.Login, i18n.DateTime(c.CreatedAt), c.Body)
	}
}
