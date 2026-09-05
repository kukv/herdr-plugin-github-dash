package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kukv/octoscope/internal/gh"
)

// workSearches pairs each board column with its GraphQL alias and the search
// query behind it (spec §4.1).
var workSearches = []struct {
	section gh.WorkSection
	alias   string
	query   string
}{
	{gh.SectionReviewRequested, "reviewRequested", "is:open is:pr review-requested:@me"},
	{gh.SectionYourPRs, "yourPRs", "is:open is:pr author:@me"},
	{gh.SectionAssigned, "assigned", "is:open assignee:@me"},
	{gh.SectionMentioned, "mentioned", "is:open mentions:@me"},
}

// workItemFields is the selection every column shares. reviewDecision and
// isDraft are GraphQL-only: REST does not expose them (spec §3.3).
const workItemFields = `
    __typename
    ... on PullRequest {
      number title url isDraft updatedAt reviewDecision
      author { login }
      repository { nameWithOwner }
      commits(last: 1) { nodes { commit { statusCheckRollup { contexts(first: 100) { nodes {
        __typename
        ... on CheckRun { status conclusion }
        ... on StatusContext { state }
      } } } } } }
    }
    ... on Issue {
      number title url updatedAt
      author { login }
      repository { nameWithOwner }
    }`

const workItemsPerColumn = 50

// workQuery builds one document whose four aliased searches fill the board in
// a single request. The search strings are compile-time constants of this
// package, so %q's Go quoting is a valid GraphQL string literal for them; a
// query that ever needs a quote or a backslash has to move to a $query
// variable passed with -F instead.
func workQuery() string {
	var b strings.Builder
	b.WriteString("query {")
	for _, s := range workSearches {
		fmt.Fprintf(&b,
			"\n  %s: search(type: ISSUE, first: %d, query: %q) { nodes {%s\n  } }",
			s.alias, workItemsPerColumn, s.query, workItemFields)
	}
	b.WriteString("\n}")
	return b.String()
}

type workResponse struct {
	Data map[string]struct {
		Nodes []searchNode `json:"nodes"`
	} `json:"data"`
}

type searchNode struct {
	Typename       string    `json:"__typename"`
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	URL            string    `json:"url"`
	IsDraft        bool      `json:"isDraft"`
	UpdatedAt      time.Time `json:"updatedAt"`
	ReviewDecision string    `json:"reviewDecision"`
	Author         struct {
		Login string `json:"login"`
	} `json:"author"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					Contexts struct {
						Nodes []checkNode `json:"nodes"`
					} `json:"contexts"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

type checkNode struct {
	Typename   string `json:"__typename"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

// ListWork fetches every column of the Work board in one GraphQL request.
func (c *Client) ListWork(ctx context.Context) (gh.Work, error) {
	// gh api graphql exits non-zero when the response body carries a top-level
	// "errors" array, so a query GitHub rejects arrives here as an error from
	// c.run rather than as a body we'd otherwise parse into empty columns.
	out, err := c.run(ctx, c.dir, "api", "graphql", "-f", "query="+workQuery())
	if err != nil {
		return gh.Work{}, err
	}
	var resp workResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return gh.Work{}, fmt.Errorf("parse work search: %w", err)
	}
	var w gh.Work
	for _, s := range workSearches {
		nodes := resp.Data[s.alias].Nodes
		items := make([]gh.WorkItem, 0, len(nodes))
		for _, n := range nodes {
			items = append(items, n.toWorkItem())
		}
		w[s.section] = items
	}
	return w, nil
}

func (n searchNode) toWorkItem() gh.WorkItem {
	item := gh.WorkItem{
		Ref: gh.ItemRef{
			Kind:   gh.ItemIssue,
			Repo:   n.Repository.NameWithOwner,
			Number: n.Number,
		},
		Title:     n.Title,
		Author:    n.Author.Login,
		UpdatedAt: n.UpdatedAt,
		URL:       n.URL,
	}
	if n.Typename != "PullRequest" {
		return item
	}
	item.Ref.Kind = gh.ItemPR
	item.IsDraft = n.IsDraft
	item.Review = gh.ParseReviewDecision(n.ReviewDecision)
	item.Checks = n.checks()
	return item
}

// checks counts every check-run context once: each context increments Total
// and exactly one of Passed, Failed, or Running, so Passed+Failed+Running
// always equals Total.
func (n searchNode) checks() gh.Checks {
	var c gh.Checks
	for _, commit := range n.Commits.Nodes {
		rollup := commit.Commit.StatusCheckRollup
		if rollup == nil {
			continue
		}
		for _, node := range rollup.Contexts.Nodes {
			c.Total++
			switch checkOutcome(node) {
			case gh.CheckSuccess:
				c.Passed++
			case gh.CheckFailure:
				c.Failed++
			default:
				c.Running++
			}
		}
	}
	switch {
	case c.Total == 0:
		c.State = gh.CheckNone
	case c.Failed > 0:
		c.State = gh.CheckFailure
	case c.Running > 0:
		c.State = gh.CheckRunning
	default:
		c.State = gh.CheckSuccess
	}
	return c
}

// checkOutcome reads one context of the rollup. CheckRun reports status and
// conclusion; the older StatusContext reports a single state, so the two
// shapes have to be read differently.
func checkOutcome(n checkNode) gh.CheckState {
	if n.Typename == "StatusContext" {
		switch n.State {
		case "SUCCESS":
			return gh.CheckSuccess
		case "FAILURE", "ERROR":
			return gh.CheckFailure
		default:
			return gh.CheckPending
		}
	}
	if n.Status != "COMPLETED" {
		return gh.CheckRunning
	}
	switch n.Conclusion {
	case "SUCCESS", "NEUTRAL", "SKIPPED":
		return gh.CheckSuccess
	case "FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE":
		return gh.CheckFailure
	default:
		return gh.CheckPending
	}
}
