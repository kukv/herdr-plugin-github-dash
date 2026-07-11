package main

import (
	"testing"

	"github.com/kukv/herdr-plugin-github-dash/internal/ui"
)

func TestParseTarget(t *testing.T) {
	cases := []struct {
		url  string
		want *ui.Target
	}{
		{"https://github.com/kukv/test-repo/pull/7",
			&ui.Target{Kind: ui.KindPR, Repo: "kukv/test-repo", Number: 7}},
		{"https://github.com/kukv/test-repo/issues/42/",
			&ui.Target{Kind: ui.KindIssue, Repo: "kukv/test-repo", Number: 42}},
		{"https://github.com/kukv/test-repo", nil},
		{"https://example.com/kukv/test-repo/pull/7", nil},
		{"", nil},
	}
	for _, c := range cases {
		got := parseTarget(c.url)
		if c.want == nil {
			if got != nil {
				t.Errorf("parseTarget(%q) = %+v, want nil", c.url, got)
			}
			continue
		}
		if got == nil || *got != *c.want {
			t.Errorf("parseTarget(%q) = %+v, want %+v", c.url, got, c.want)
		}
	}
}
