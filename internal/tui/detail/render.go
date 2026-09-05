package detail

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kukv/octoscope/internal/gh"
	"github.com/kukv/octoscope/internal/i18n"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
)

func (m Model) View() string {
	if m.composing {
		return m.composeView()
	}
	if m.confirming {
		return m.confirmView()
	}
	if m.picking {
		return m.pickerView()
	}
	if m.loading || m.pickerLoading {
		return m.spin.View() + " " + i18n.T("common.loading") + "\n"
	}
	header := titleStyle.Render(m.title)
	footer := dimStyle.Render(i18n.T("footer.detail_prefix") + m.stateFooterKey() + i18n.T("footer.detail_suffix"))
	body := header + "\n" + m.body.View() + "\n"
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
	header := titleStyle.Render(m.title)
	closing, _ := m.stateAction()
	var id string
	switch {
	case m.ref.Kind == gh.ItemPR && closing:
		id = "confirm.close_pr"
	case m.ref.Kind == gh.ItemPR:
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
	b.WriteString(titleStyle.Render(i18n.Tf("compose.title", map[string]any{"Title": m.title})) + "\n\n")
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

func cursorPrefix(selected bool) string {
	if selected {
		return "▸ "
	}
	return "  "
}

func prMarkdown(pr gh.PR) string {
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

func issueMarkdown(issue gh.Issue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# #%d %s\n\n", issue.Number, issue.Title)
	fmt.Fprintf(&b, "- **%s**: @%s\n", i18n.T("md.author"), issue.Author.Login)
	fmt.Fprintf(&b, "- **%s**: %s\n", i18n.T("md.state"), issue.State)
	writeCommonMeta(&b, issue.Labels, issue.UpdatedAt)
	writeBody(&b, issue.Body)
	writeComments(&b, issue.Comments)
	return b.String()
}

func writeCommonMeta(b *strings.Builder, labels []gh.Label, updatedAt time.Time) {
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

func writeComments(b *strings.Builder, comments []gh.Comment) {
	for _, c := range comments {
		fmt.Fprintf(b, "\n\n---\n\n**@%s** — %s\n\n%s",
			c.Author.Login, i18n.DateTime(c.CreatedAt), c.Body)
	}
}
