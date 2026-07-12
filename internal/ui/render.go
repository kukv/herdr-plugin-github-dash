package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

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
	case m.listLoading[m.tab]:
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
		return m.spin.View() + " loading...\n"
	}
	header := titleStyle.Render(m.detailTitle)
	footer := dimStyle.Render("j/k:scroll  r:refresh  o:browser  c:comment  " + m.stateFooterKey() + "l:labels  a:assign  esc:back")
	body := header + "\n" + m.detail.View() + "\n"
	if m.actionErr != "" {
		body += "error: " + m.actionErr + "\n"
	}
	return body + footer
}

func (m Model) pickerView() string {
	body := m.picker.listView(m.height)
	if m.applying {
		return body + "\n" + m.spin.View() + " applying...\n"
	}
	return body + "\n" + dimStyle.Render("space:toggle  enter:apply  esc:cancel")
}

// stateFooterKey returns the state-aware footer hint (with trailing spaces),
// or "" when the item cannot change state (merged / not yet loaded).
func (m Model) stateFooterKey() string {
	closing, ok := m.stateAction()
	if !ok {
		return ""
	}
	if closing {
		return "x:close  "
	}
	return "x:reopen  "
}

func (m Model) confirmView() string {
	header := titleStyle.Render(m.detailTitle)
	closing, _ := m.stateAction()
	verb := "Reopen"
	if closing {
		verb = "Close"
	}
	noun := "issue"
	if m.detailTarget.Kind == KindPR {
		noun = "PR"
	}
	var b strings.Builder
	b.WriteString(header + "\n\n")
	fmt.Fprintf(&b, "%s this %s? ", verb, noun)
	if m.working {
		b.WriteString(m.spin.View() + " working...\n")
	} else {
		b.WriteString(dimStyle.Render("(y/n)"))
	}
	return b.String()
}

func (m Model) composeView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Comment on "+m.detailTitle) + "\n\n")
	b.WriteString(m.textarea.View() + "\n\n")
	if m.postErr != "" {
		b.WriteString("error: " + m.postErr + "\n\n")
	}
	if m.posting {
		b.WriteString(m.spin.View() + " posting...\n")
	} else {
		b.WriteString(dimStyle.Render("ctrl+s:send  esc:cancel"))
	}
	return b.String()
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

func prMarkdown(pr ghcli.PR) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# #%d %s\n\n", pr.Number, pr.Title)
	fmt.Fprintf(&b, "- **author**: @%s\n", pr.Author.Login)
	state := pr.State
	if pr.IsDraft {
		state += " (draft)"
	}
	fmt.Fprintf(&b, "- **state**: %s\n", state)
	if pr.ReviewDecision != "" {
		fmt.Fprintf(&b, "- **review**: %s\n", pr.ReviewDecision)
	}
	writeCommonMeta(&b, pr.Labels, pr.UpdatedAt)
	writeBody(&b, pr.Body)
	writeComments(&b, pr.Comments)
	return b.String()
}

func issueMarkdown(issue ghcli.Issue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# #%d %s\n\n", issue.Number, issue.Title)
	fmt.Fprintf(&b, "- **author**: @%s\n", issue.Author.Login)
	fmt.Fprintf(&b, "- **state**: %s\n", issue.State)
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
		fmt.Fprintf(b, "- **labels**: %s\n", strings.Join(names, ", "))
	}
	fmt.Fprintf(b, "- **updated**: %s\n", updatedAt.Format("2006-01-02 15:04"))
}

func writeBody(b *strings.Builder, body string) {
	b.WriteString("\n---\n\n")
	if body != "" {
		b.WriteString(body)
	} else {
		b.WriteString("_no description_")
	}
}

func writeComments(b *strings.Builder, comments []ghcli.Comment) {
	for _, c := range comments {
		fmt.Fprintf(b, "\n\n---\n\n**@%s** — %s\n\n%s",
			c.Author.Login, c.CreatedAt.Format("2006-01-02 15:04"), c.Body)
	}
}
