package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/octoscope/internal/gh"
)

func TestPRLineShowsReviewIconAndRelTime(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		pr   gh.PR
		want []string
	}{
		{"draft", gh.PR{IsDraft: true, UpdatedAt: now.Add(-30 * time.Second)}, []string{"◌", "now"}},
		{"approved", gh.PR{ReviewDecision: "APPROVED", UpdatedAt: now.Add(-5 * time.Minute)}, []string{"✓", "5m ago"}},
		{"changes requested", gh.PR{ReviewDecision: "CHANGES_REQUESTED", UpdatedAt: now.Add(-3 * time.Hour)}, []string{"×", "3h ago"}},
		{"review required", gh.PR{ReviewDecision: "REVIEW_REQUIRED", UpdatedAt: now.Add(-49 * time.Hour)}, []string{"•", "2d ago"}},
		{"none", gh.PR{UpdatedAt: now}, []string{"•", "now"}},
	}
	for _, c := range cases {
		got := prLine(c.pr, now)
		for _, want := range c.want {
			if !strings.Contains(got, want) {
				t.Errorf("%s: prLine = %q, want to contain %q", c.name, got, want)
			}
		}
	}
}

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
