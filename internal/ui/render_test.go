package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/herdr-plugin-github-dash/internal/ghcli"
)

func TestRelTime(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-3 * time.Hour), "3h ago"},
		{now.Add(-49 * time.Hour), "2d ago"},
	}
	for _, c := range cases {
		if got := relTime(now, c.t); got != c.want {
			t.Errorf("relTime(%v) = %q, want %q", c.t, got, c.want)
		}
	}
}

func TestReviewIcon(t *testing.T) {
	cases := []struct {
		pr   ghcli.PR
		want string
	}{
		{ghcli.PR{IsDraft: true}, "◌"},
		{ghcli.PR{ReviewDecision: "APPROVED"}, "✓"},
		{ghcli.PR{ReviewDecision: "CHANGES_REQUESTED"}, "×"},
		{ghcli.PR{ReviewDecision: "REVIEW_REQUIRED"}, "•"},
		{ghcli.PR{}, "•"},
	}
	for _, c := range cases {
		if got := reviewIcon(c.pr); got != c.want {
			t.Errorf("reviewIcon(%+v) = %q, want %q", c.pr, got, c.want)
		}
	}
}

func TestPRMarkdownContainsMetaBodyAndComments(t *testing.T) {
	pr := ghcli.PR{
		Number: 12, Title: "feat: pane", Author: ghcli.Author{Login: "kukv"},
		State: "OPEN", IsDraft: true, ReviewDecision: "REVIEW_REQUIRED",
		Labels: []ghcli.Label{{Name: "Kind: Feature"}},
		Body:   "body text",
		Comments: []ghcli.Comment{
			{Author: ghcli.Author{Login: "bob"}, Body: "comment text",
				CreatedAt: time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC)},
		},
	}
	md := prMarkdown(pr)
	for _, want := range []string{"#12", "feat: pane", "@kukv", "OPEN (draft)",
		"REVIEW_REQUIRED", "Kind: Feature", "body text", "@bob", "comment text"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestIssueMarkdownEmptyBody(t *testing.T) {
	md := issueMarkdown(ghcli.Issue{Number: 3, Title: "an issue"})
	if !strings.Contains(md, "_no description_") {
		t.Errorf("markdown missing empty-body placeholder:\n%s", md)
	}
}
