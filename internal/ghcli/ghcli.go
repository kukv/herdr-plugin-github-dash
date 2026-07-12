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
	prViewFields    = prListFields + ",body,comments,labels"
	issueListFields = "number,title,author,state,updatedAt,labels,url"
	issueViewFields = issueListFields + ",body,comments"
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
