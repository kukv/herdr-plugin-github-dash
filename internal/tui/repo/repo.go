// Package repo lists the open pull requests and issues of one repository.
package repo

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/kukv/octoscope/internal/gh"
)

// Source is what the repository list needs from the GitHub layer.
// The list always shows the client's own repository, so only the two
// browser-opening calls name one: repo is "owner/repo", and the empty string
// targets that same repository.
type Source interface {
	ListPRs(ctx context.Context) ([]gh.PR, error)
	ListIssues(ctx context.Context) ([]gh.Issue, error)
	RepoName(ctx context.Context) (string, error)
	OpenPRWeb(repo string, number int) error
	OpenIssueWeb(repo string, number int) error
}

// OpenDetailMsg asks the parent to show the detail view for one item.
type OpenDetailMsg struct{ Ref gh.ItemRef }

// ErrorMsg carries a failure the parent shows on its error screen.
type ErrorMsg struct{ Err error }

type (
	prListMsg    []gh.PR
	issueListMsg []gh.Issue
	repoNameMsg  string
	errMsg       struct{ err error }
)

type tabID int

const (
	tabPRs tabID = iota
	tabIssues
)

type Model struct {
	src Source

	repoName string

	tab     tabID
	cursors [2]int
	prs     []gh.PR
	issues  []gh.Issue
	loaded  [2]bool
	loading [2]bool
}

func New(src Source) Model {
	m := Model{src: src}
	m.loading[m.tab] = true
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchRepoName(m.src), fetchList(m.src, m.tab))
}

func fetchList(src Source, t tabID) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if t == tabPRs {
			prs, err := src.ListPRs(ctx)
			if err != nil {
				return errMsg{err}
			}
			return prListMsg(prs)
		}
		issues, err := src.ListIssues(ctx)
		if err != nil {
			return errMsg{err}
		}
		return issueListMsg(issues)
	}
}

func fetchRepoName(src Source) tea.Cmd {
	return func() tea.Msg {
		name, err := src.RepoName(context.Background())
		if err != nil {
			return repoNameMsg("") // the name only decorates the header: a failure is not worth reporting
		}
		return repoNameMsg(name)
	}
}

func openWeb(src Source, ref gh.ItemRef) tea.Cmd {
	return func() tea.Msg {
		var err error
		if ref.Kind == gh.ItemPR {
			err = src.OpenPRWeb(ref.Repo, ref.Number)
		} else {
			err = src.OpenIssueWeb(ref.Repo, ref.Number)
		}
		if err != nil {
			return errMsg{err}
		}
		return nil
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case repoNameMsg:
		m.repoName = string(msg)
		return m, nil
	case prListMsg:
		m.prs = []gh.PR(msg)
		m.loaded[tabPRs] = true
		if m.cursors[tabPRs] >= len(m.prs) {
			m.cursors[tabPRs] = max(len(m.prs)-1, 0)
		}
		m.loading[tabPRs] = false
		return m, nil
	case issueListMsg:
		m.issues = []gh.Issue(msg)
		m.loaded[tabIssues] = true
		if m.cursors[tabIssues] >= len(m.issues) {
			m.cursors[tabIssues] = max(len(m.issues)-1, 0)
		}
		m.loading[tabIssues] = false
		return m, nil
	case errMsg:
		// The loading flags are left as they are: which tab the failed fetch
		// belonged to is not knowable from here, and the parent takes the
		// screen over anyway.
		err := msg.err
		return m, func() tea.Msg { return ErrorMsg{err} }
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		if m.tab == tabPRs {
			m.tab = tabIssues
		} else {
			m.tab = tabPRs
		}
		if !m.loaded[m.tab] {
			m.loading[m.tab] = true
			return m, fetchList(m.src, m.tab)
		}
		return m, nil
	case "j", "down":
		if n := m.itemCount(); n > 0 && m.cursors[m.tab] < n-1 {
			m.cursors[m.tab]++
		}
		return m, nil
	case "k", "up":
		if m.cursors[m.tab] > 0 {
			m.cursors[m.tab]--
		}
		return m, nil
	case "r":
		m.loading[m.tab] = true
		return m, fetchList(m.src, m.tab)
	case "enter":
		if ref, ok := m.SelectedRef(); ok {
			return m, func() tea.Msg { return OpenDetailMsg{ref} }
		}
		return m, nil
	case "o":
		if ref, ok := m.SelectedRef(); ok {
			return m, openWeb(m.src, ref)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) itemCount() int {
	if m.tab == tabPRs {
		return len(m.prs)
	}
	return len(m.issues)
}

// SelectedRef names the item under the cursor. ok is false when the tab is
// empty. Repo stays empty: the list only ever shows the client's repository.
func (m Model) SelectedRef() (gh.ItemRef, bool) {
	if m.tab == tabPRs {
		if len(m.prs) == 0 {
			return gh.ItemRef{}, false
		}
		return gh.ItemRef{Kind: gh.ItemPR, Number: m.prs[m.cursors[tabPRs]].Number}, true
	}
	if len(m.issues) == 0 {
		return gh.ItemRef{}, false
	}
	return gh.ItemRef{Kind: gh.ItemIssue, Number: m.issues[m.cursors[tabIssues]].Number}, true
}
