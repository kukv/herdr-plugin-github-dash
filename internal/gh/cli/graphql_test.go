package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/kukv/octoscope/internal/gh"
)

const workJSON = `{"data":{
  "reviewRequested":{"nodes":[
    {"__typename":"PullRequest","number":12,"title":"fix the thing",
     "url":"https://github.com/kukv/octoscope/pull/12","isDraft":false,
     "updatedAt":"2026-09-06T12:00:00Z","reviewDecision":"REVIEW_REQUIRED",
     "author":{"login":"someone"},
     "repository":{"nameWithOwner":"kukv/octoscope"},
     "commits":{"nodes":[{"commit":{"statusCheckRollup":{"contexts":{"nodes":[
        {"__typename":"CheckRun","conclusion":"SUCCESS","status":"COMPLETED"},
        {"__typename":"CheckRun","conclusion":"","status":"IN_PROGRESS"},
        {"__typename":"CheckRun","conclusion":"FAILURE","status":"COMPLETED"}
     ]}}}}]}}
  ]},
  "yourPRs":{"nodes":[]},
  "assigned":{"nodes":[
    {"__typename":"Issue","number":7,"title":"an issue",
     "url":"https://github.com/kukv/octoscope/issues/7",
     "updatedAt":"2026-09-05T12:00:00Z","author":{"login":"kukv"},
     "repository":{"nameWithOwner":"kukv/octoscope"}}
  ]},
  "mentioned":{"nodes":[]}
}}`

func TestListWorkBuildsOneGraphQLRequest(t *testing.T) {
	t.Parallel()

	var got []string
	c := New("/tmp", "")
	c.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		got = args
		return []byte(workJSON), nil
	}

	if _, err := c.ListWork(context.Background()); err != nil {
		t.Fatalf("ListWork: %v", err)
	}

	if len(got) < 2 || got[0] != "api" || got[1] != "graphql" {
		t.Fatalf("got args %v, want them to start with api graphql", got)
	}
	query := flagValue(t, got, "-f")
	for _, want := range []string{
		"review-requested:@me", "author:@me", "assignee:@me", "mentions:@me",
		"reviewDecision", "statusCheckRollup",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("query is missing %q:\n%s", want, query)
		}
	}
}

func TestListWorkTranslatesToDomainValues(t *testing.T) {
	t.Parallel()

	c := New("/tmp", "")
	c.run = func(context.Context, string, ...string) ([]byte, error) {
		return []byte(workJSON), nil
	}

	w, err := c.ListWork(context.Background())
	if err != nil {
		t.Fatalf("ListWork: %v", err)
	}

	rr := w[gh.SectionReviewRequested]
	if len(rr) != 1 {
		t.Fatalf("review requested holds %d items, want 1", len(rr))
	}
	item := rr[0]
	if item.Ref.Kind != gh.ItemPR {
		t.Errorf("kind: got %v, want ItemPR", item.Ref.Kind)
	}
	if item.Ref.Repo != "kukv/octoscope" {
		t.Errorf("repo: got %q, want kukv/octoscope", item.Ref.Repo)
	}
	if item.Review != gh.ReviewRequired {
		t.Errorf("review: got %v, want ReviewRequired", item.Review)
	}
	if item.Checks.Total != 3 || item.Checks.Passed != 1 ||
		item.Checks.Failed != 1 || item.Checks.Running != 1 {
		t.Errorf("checks counts: got %+v", item.Checks)
	}
	if item.Checks.State != gh.CheckFailure {
		t.Errorf("checks state: got %v, want CheckFailure", item.Checks.State)
	}

	assigned := w[gh.SectionAssigned]
	if len(assigned) != 1 || assigned[0].Ref.Kind != gh.ItemIssue {
		t.Fatalf("assigned column: got %+v, want one issue", assigned)
	}
	if n := len(w[gh.SectionYourPRs]); n != 0 {
		t.Errorf("your PRs holds %d items, want 0", n)
	}
}

func TestListWorkReportsAFailure(t *testing.T) {
	t.Parallel()

	c := New("/tmp", "")
	c.run = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("not json"), nil
	}

	if _, err := c.ListWork(context.Background()); err == nil {
		t.Error("ListWork accepted a body that is not JSON")
	}
}

// flagValue returns the argument that follows the last occurrence of flag.
func flagValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	for i := len(args) - 2; i >= 0; i-- {
		if args[i] == flag {
			return args[i+1]
		}
	}
	t.Fatalf("flag %q not found in %v", flag, args)
	return ""
}
