package ui

import (
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
