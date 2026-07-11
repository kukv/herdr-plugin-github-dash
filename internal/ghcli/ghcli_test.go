package ghcli

import (
	"errors"
	"reflect"
	"testing"
)

const prListJSON = `[{"number":12,"title":"feat: add pane view","author":{"is_bot":false,"login":"kukv"},"state":"OPEN","isDraft":false,"updatedAt":"2026-07-11T10:00:00Z","reviewDecision":"APPROVED","url":"https://github.com/kukv/demo/pull/12"}]`

const prViewJSON = `{"number":12,"title":"feat: add pane view","author":{"is_bot":false,"login":"kukv"},"state":"OPEN","isDraft":false,"updatedAt":"2026-07-11T10:00:00Z","reviewDecision":"REVIEW_REQUIRED","url":"https://github.com/kukv/demo/pull/12","body":"Adds the pane.","labels":[{"id":"LA_x","name":"Kind: Feature","description":"","color":"ededed"}],"comments":[{"author":{"login":"bob"},"body":"LGTM","createdAt":"2026-07-11T11:00:00Z"}]}`

const issueListJSON = `[{"number":3,"title":"bug: crash on empty list","author":{"is_bot":false,"login":"alice"},"state":"OPEN","updatedAt":"2026-07-10T09:00:00Z","labels":[],"url":"https://github.com/kukv/demo/issues/3"}]`

const issueViewJSON = `{"number":3,"title":"bug: crash on empty list","author":{"is_bot":false,"login":"alice"},"state":"OPEN","updatedAt":"2026-07-10T09:00:00Z","labels":[],"url":"https://github.com/kukv/demo/issues/3","body":"Steps to reproduce.","comments":[{"author":{"login":"carol"},"body":"Confirmed","createdAt":"2026-07-10T10:00:00Z"}]}`

// fakeRun records invocations and returns canned output.
type fakeRun struct {
	dir  string
	args []string
	out  []byte
	err  error
}

func (f *fakeRun) run(dir string, args ...string) ([]byte, error) {
	f.dir = dir
	f.args = args
	return f.out, f.err
}

func newTestClient(out string, err error) (*Client, *fakeRun) {
	f := &fakeRun{out: []byte(out), err: err}
	return &Client{dir: "/repo", run: f.run}, f
}

func TestListPRs(t *testing.T) {
	c, f := newTestClient(prListJSON, nil)
	prs, err := c.ListPRs()
	if err != nil {
		t.Fatalf("ListPRs: %v", err)
	}
	wantArgs := []string{"pr", "list", "--json", prListFields}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if f.dir != "/repo" {
		t.Errorf("dir = %q, want /repo", f.dir)
	}
	if len(prs) != 1 || prs[0].Number != 12 || prs[0].Author.Login != "kukv" ||
		prs[0].ReviewDecision != "APPROVED" {
		t.Errorf("unexpected parse result: %+v", prs)
	}
}

func TestListPRsEmpty(t *testing.T) {
	c, _ := newTestClient(`[]`, nil)
	prs, err := c.ListPRs()
	if err != nil || len(prs) != 0 {
		t.Errorf("prs = %v, err = %v; want empty, nil", prs, err)
	}
}

func TestGetPRParsesDetailFields(t *testing.T) {
	c, f := newTestClient(prViewJSON, nil)
	pr, err := c.GetPR("", 12)
	if err != nil {
		t.Fatalf("GetPR: %v", err)
	}
	wantArgs := []string{"pr", "view", "12", "--json", prViewFields}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if pr.Body != "Adds the pane." || len(pr.Comments) != 1 ||
		pr.Comments[0].Author.Login != "bob" || len(pr.Labels) != 1 ||
		pr.Labels[0].Name != "Kind: Feature" {
		t.Errorf("unexpected parse result: %+v", pr)
	}
}

func TestGetPRWithRepoOverride(t *testing.T) {
	c, f := newTestClient(prViewJSON, nil)
	if _, err := c.GetPR("octo/hello", 12); err != nil {
		t.Fatalf("GetPR: %v", err)
	}
	wantArgs := []string{"pr", "view", "12", "--json", prViewFields, "--repo", "octo/hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestListIssues(t *testing.T) {
	c, f := newTestClient(issueListJSON, nil)
	issues, err := c.ListIssues()
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	wantArgs := []string{"issue", "list", "--json", issueListFields}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if len(issues) != 1 || issues[0].Number != 3 || issues[0].Author.Login != "alice" {
		t.Errorf("unexpected parse result: %+v", issues)
	}
}

func TestGetIssueWithRepoOverride(t *testing.T) {
	c, f := newTestClient(issueViewJSON, nil)
	issue, err := c.GetIssue("octo/hello", 3)
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	wantArgs := []string{"issue", "view", "3", "--json", issueViewFields, "--repo", "octo/hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if issue.Body != "Steps to reproduce." || len(issue.Comments) != 1 ||
		issue.Comments[0].Author.Login != "carol" {
		t.Errorf("unexpected parse result: %+v", issue)
	}
}

func TestRepoName(t *testing.T) {
	c, f := newTestClient(`{"nameWithOwner":"kukv/demo"}`, nil)
	name, err := c.RepoName()
	if err != nil || name != "kukv/demo" {
		t.Errorf("name = %q, err = %v; want kukv/demo, nil", name, err)
	}
	wantArgs := []string{"repo", "view", "--json", "nameWithOwner"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestOpenPRWebWithRepoOverride(t *testing.T) {
	c, f := newTestClient("", nil)
	if err := c.OpenPRWeb("octo/hello", 7); err != nil {
		t.Fatalf("OpenPRWeb: %v", err)
	}
	wantArgs := []string{"pr", "view", "7", "--web", "--repo", "octo/hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestOpenIssueWebWithRepoOverride(t *testing.T) {
	c, f := newTestClient("", nil)
	if err := c.OpenIssueWeb("octo/hello", 3); err != nil {
		t.Fatalf("OpenIssueWeb: %v", err)
	}
	wantArgs := []string{"issue", "view", "3", "--web", "--repo", "octo/hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestRunErrorPassesThrough(t *testing.T) {
	wantErr := errors.New("gh pr: no git remotes found")
	c, _ := newTestClient("", wantErr)
	if _, err := c.ListPRs(); !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}

func TestAddPRComment(t *testing.T) {
	c, f := newTestClient("https://github.com/kukv/demo/pull/12#issuecomment-1\n", nil)
	if err := c.AddPRComment("", 12, "hello"); err != nil {
		t.Fatalf("AddPRComment: %v", err)
	}
	wantArgs := []string{"pr", "comment", "12", "--body", "hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if f.dir != "/repo" {
		t.Errorf("dir = %q, want /repo", f.dir)
	}
}

func TestAddIssueCommentWithRepoOverride(t *testing.T) {
	c, f := newTestClient("", nil)
	if err := c.AddIssueComment("octo/hello", 3, "hi there"); err != nil {
		t.Fatalf("AddIssueComment: %v", err)
	}
	wantArgs := []string{"issue", "comment", "3", "--body", "hi there", "--repo", "octo/hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestAddCommentError(t *testing.T) {
	wantErr := errors.New("gh pr: HTTP 403 forbidden")
	c, _ := newTestClient("", wantErr)
	if err := c.AddPRComment("", 12, "x"); !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}
