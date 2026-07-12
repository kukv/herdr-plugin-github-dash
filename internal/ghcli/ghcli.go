// Package ghcli fetches GitHub data by running the gh CLI in a target directory.
package ghcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

const (
	prListFields    = "number,title,author,state,isDraft,updatedAt,reviewDecision,url"
	prViewFields    = prListFields + ",body,comments,labels,assignees"
	issueListFields = "number,title,author,state,updatedAt,labels,url"
	issueViewFields = issueListFields + ",body,comments,assignees"
)

// ErrGhNotFound is returned when the gh binary is not on PATH.
var ErrGhNotFound = errors.New("gh CLI not found; install it and run: gh auth login")

type Author struct {
	Login string `json:"login"`
}

type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Comment struct {
	Author    Author    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type PR struct {
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	Author         Author    `json:"author"`
	State          string    `json:"state"`
	IsDraft        bool      `json:"isDraft"`
	UpdatedAt      time.Time `json:"updatedAt"`
	ReviewDecision string    `json:"reviewDecision"`
	URL            string    `json:"url"`
	Body           string    `json:"body"`
	Comments       []Comment `json:"comments"`
	Labels         []Label   `json:"labels"`
	Assignees      []Author  `json:"assignees"`
}

type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Author    Author    `json:"author"`
	State     string    `json:"state"`
	UpdatedAt time.Time `json:"updatedAt"`
	URL       string    `json:"url"`
	Body      string    `json:"body"`
	Comments  []Comment `json:"comments"`
	Labels    []Label   `json:"labels"`
	Assignees []Author  `json:"assignees"`
}

type runFunc func(dir string, args ...string) ([]byte, error)

// Client runs gh commands in a fixed directory.
type Client struct {
	dir string
	run runFunc
}

func New(dir string) *Client {
	return &Client{dir: dir, run: runGh}
}

func runGh(dir string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, ErrGhNotFound
	}
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := bytes.TrimSpace(stderr.Bytes()); len(msg) > 0 {
			return nil, fmt.Errorf("gh %s: %s", args[0], msg)
		}
		return nil, fmt.Errorf("gh %s: %w", args[0], err)
	}
	return stdout.Bytes(), nil
}

func appendRepo(args []string, repo string) []string {
	if repo != "" {
		return append(args, "--repo", repo)
	}
	return args
}

func (c *Client) ListPRs() ([]PR, error) {
	out, err := c.run(c.dir, "pr", "list", "--json", prListFields)
	if err != nil {
		return nil, err
	}
	var prs []PR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("parse pr list: %w", err)
	}
	return prs, nil
}

func (c *Client) ListIssues() ([]Issue, error) {
	out, err := c.run(c.dir, "issue", "list", "--json", issueListFields)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parse issue list: %w", err)
	}
	return issues, nil
}

func (c *Client) GetPR(repo string, number int) (PR, error) {
	args := appendRepo([]string{"pr", "view", strconv.Itoa(number), "--json", prViewFields}, repo)
	out, err := c.run(c.dir, args...)
	if err != nil {
		return PR{}, err
	}
	var pr PR
	if err := json.Unmarshal(out, &pr); err != nil {
		return PR{}, fmt.Errorf("parse pr view: %w", err)
	}
	return pr, nil
}

func (c *Client) GetIssue(repo string, number int) (Issue, error) {
	args := appendRepo([]string{"issue", "view", strconv.Itoa(number), "--json", issueViewFields}, repo)
	out, err := c.run(c.dir, args...)
	if err != nil {
		return Issue{}, err
	}
	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return Issue{}, fmt.Errorf("parse issue view: %w", err)
	}
	return issue, nil
}

func (c *Client) RepoName() (string, error) {
	out, err := c.run(c.dir, "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		return "", err
	}
	var v struct {
		NameWithOwner string `json:"nameWithOwner"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return "", fmt.Errorf("parse repo view: %w", err)
	}
	return v.NameWithOwner, nil
}

func (c *Client) OpenPRWeb(repo string, number int) error {
	_, err := c.run(c.dir, appendRepo([]string{"pr", "view", strconv.Itoa(number), "--web"}, repo)...)
	return err
}

func (c *Client) OpenIssueWeb(repo string, number int) error {
	_, err := c.run(c.dir, appendRepo([]string{"issue", "view", strconv.Itoa(number), "--web"}, repo)...)
	return err
}

func (c *Client) AddPRComment(repo string, number int, body string) error {
	_, err := c.run(c.dir, appendRepo([]string{"pr", "comment", strconv.Itoa(number), "--body", body}, repo)...)
	return err
}

func (c *Client) AddIssueComment(repo string, number int, body string) error {
	_, err := c.run(c.dir, appendRepo([]string{"issue", "comment", strconv.Itoa(number), "--body", body}, repo)...)
	return err
}

func (c *Client) ClosePR(repo string, number int) error {
	_, err := c.run(c.dir, appendRepo([]string{"pr", "close", strconv.Itoa(number)}, repo)...)
	return err
}

func (c *Client) ReopenPR(repo string, number int) error {
	_, err := c.run(c.dir, appendRepo([]string{"pr", "reopen", strconv.Itoa(number)}, repo)...)
	return err
}

func (c *Client) CloseIssue(repo string, number int) error {
	_, err := c.run(c.dir, appendRepo([]string{"issue", "close", strconv.Itoa(number)}, repo)...)
	return err
}

func (c *Client) ReopenIssue(repo string, number int) error {
	_, err := c.run(c.dir, appendRepo([]string{"issue", "reopen", strconv.Itoa(number)}, repo)...)
	return err
}

func (c *Client) ListLabels(repo string) ([]Label, error) {
	args := appendRepo([]string{"label", "list", "--json", "name,color", "--limit", "100"}, repo)
	out, err := c.run(c.dir, args...)
	if err != nil {
		return nil, err
	}
	var labels []Label
	if err := json.Unmarshal(out, &labels); err != nil {
		return nil, fmt.Errorf("parse label list: %w", err)
	}
	return labels, nil
}

// ListAssignees returns the logins of users assignable on the repository.
// gh api substitutes {owner}/{repo} from the current directory's repo; for an
// override we build the explicit path (gh api takes no --repo).
func (c *Client) ListAssignees(repo string) ([]string, error) {
	path := "repos/{owner}/{repo}/assignees"
	if repo != "" {
		path = "repos/" + repo + "/assignees"
	}
	out, err := c.run(c.dir, "api", path)
	if err != nil {
		return nil, err
	}
	var users []Author
	if err := json.Unmarshal(out, &users); err != nil {
		return nil, fmt.Errorf("parse assignees: %w", err)
	}
	logins := make([]string, len(users))
	for i, u := range users {
		logins[i] = u.Login
	}
	return logins, nil
}

func (c *Client) editItems(kindCmd, repo string, number int, add, remove []string, addFlag, removeFlag string) error {
	args := []string{kindCmd, "edit", strconv.Itoa(number)}
	for _, v := range add {
		args = append(args, addFlag, v)
	}
	for _, v := range remove {
		args = append(args, removeFlag, v)
	}
	_, err := c.run(c.dir, appendRepo(args, repo)...)
	return err
}

func (c *Client) EditPRLabels(repo string, number int, add, remove []string) error {
	return c.editItems("pr", repo, number, add, remove, "--add-label", "--remove-label")
}

func (c *Client) EditIssueLabels(repo string, number int, add, remove []string) error {
	return c.editItems("issue", repo, number, add, remove, "--add-label", "--remove-label")
}

func (c *Client) EditPRAssignees(repo string, number int, add, remove []string) error {
	return c.editItems("pr", repo, number, add, remove, "--add-assignee", "--remove-assignee")
}

func (c *Client) EditIssueAssignees(repo string, number int, add, remove []string) error {
	return c.editItems("issue", repo, number, add, remove, "--add-assignee", "--remove-assignee")
}
