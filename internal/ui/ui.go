// Package ui implements the GitHub Dash terminal UI.
package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/kukv/herdr-plugin-github-dash/internal/ghcli"
)

// DataSource is what the UI needs from the GitHub layer.
// repo is "owner/repo"; empty string targets the workspace repository.
type DataSource interface {
	ListPRs() ([]ghcli.PR, error)
	ListIssues() ([]ghcli.Issue, error)
	GetPR(repo string, number int) (ghcli.PR, error)
	GetIssue(repo string, number int) (ghcli.Issue, error)
	RepoName() (string, error)
	OpenPRWeb(repo string, number int) error
	OpenIssueWeb(repo string, number int) error
}

type Kind int

const (
	KindPR Kind = iota
	KindIssue
)

// Target identifies one PR or issue, optionally in another repository.
type Target struct {
	Kind   Kind
	Repo   string
	Number int
}

type screen int

const (
	screenList screen = iota
	screenDetail
	screenError
)

type tabID int

const (
	tabPRs tabID = iota
	tabIssues
)

type (
	prListMsg      []ghcli.PR
	issueListMsg   []ghcli.Issue
	prDetailMsg    ghcli.PR
	issueDetailMsg ghcli.Issue
	repoNameMsg    string
	errorMsg       struct{ err error }
)

type Model struct {
	src DataSource

	width, height int
	repoName      string

	screen  screen
	tab     tabID
	cursors [2]int
	prs     []ghcli.PR
	issues  []ghcli.Issue
	loaded  [2]bool
	loading bool

	detail       viewport.Model
	detailTitle  string
	detailTarget Target

	spin    spinner.Model
	errText string
}

func New(src DataSource, initial *Target) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	m := Model{
		src:     src,
		spin:    s,
		screen:  screenList,
		loading: true,
		detail:  viewport.New(80, 20),
	}
	if initial != nil {
		m.screen = screenDetail
		m.detailTarget = *initial
	}
	return m
}

// NewError builds a model that only shows an error message.
func NewError(text string) Model {
	return Model{screen: screenError, errText: text}
}

func (m Model) Init() tea.Cmd {
	if m.screen == screenError {
		return nil
	}
	cmds := []tea.Cmd{m.spin.Tick, fetchRepoName(m.src)}
	if m.screen == screenDetail {
		cmds = append(cmds, fetchDetail(m.src, m.detailTarget))
	} else {
		cmds = append(cmds, fetchList(m.src, m.tab))
	}
	return tea.Batch(cmds...)
}

func fetchList(src DataSource, t tabID) tea.Cmd {
	return func() tea.Msg {
		if t == tabPRs {
			prs, err := src.ListPRs()
			if err != nil {
				return errorMsg{err}
			}
			return prListMsg(prs)
		}
		issues, err := src.ListIssues()
		if err != nil {
			return errorMsg{err}
		}
		return issueListMsg(issues)
	}
}

func fetchDetail(src DataSource, target Target) tea.Cmd {
	return func() tea.Msg {
		if target.Kind == KindPR {
			pr, err := src.GetPR(target.Repo, target.Number)
			if err != nil {
				return errorMsg{err}
			}
			return prDetailMsg(pr)
		}
		issue, err := src.GetIssue(target.Repo, target.Number)
		if err != nil {
			return errorMsg{err}
		}
		return issueDetailMsg(issue)
	}
}

func fetchRepoName(src DataSource) tea.Cmd {
	return func() tea.Msg {
		name, err := src.RepoName()
		if err != nil {
			return repoNameMsg("") // ヘッダー表示専用の情報なので失敗は無視する
		}
		return repoNameMsg(name)
	}
}

func openWeb(src DataSource, target Target) tea.Cmd {
	return func() tea.Msg {
		var err error
		if target.Kind == KindPR {
			err = src.OpenPRWeb(target.Repo, target.Number)
		} else {
			err = src.OpenIssueWeb(target.Repo, target.Number)
		}
		if err != nil {
			return errorMsg{err}
		}
		return nil
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.detail.Width = msg.Width
		m.detail.Height = max(msg.Height-4, 5)
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case repoNameMsg:
		m.repoName = string(msg)
		return m, nil
	case prListMsg:
		m.prs = []ghcli.PR(msg)
		m.loaded[tabPRs] = true
		if m.cursors[tabPRs] >= len(m.prs) {
			m.cursors[tabPRs] = max(len(m.prs)-1, 0)
		}
		if m.tab == tabPRs {
			m.loading = false
		}
		return m, nil
	case issueListMsg:
		m.issues = []ghcli.Issue(msg)
		m.loaded[tabIssues] = true
		if m.cursors[tabIssues] >= len(m.issues) {
			m.cursors[tabIssues] = max(len(m.issues)-1, 0)
		}
		if m.tab == tabIssues {
			m.loading = false
		}
		return m, nil
	case prDetailMsg:
		m.loading = false
		m.detailTitle = fmt.Sprintf("PR #%d %s", msg.Number, msg.Title)
		m.setDetailContent(prMarkdown(ghcli.PR(msg)))
		return m, nil
	case issueDetailMsg:
		m.loading = false
		m.detailTitle = fmt.Sprintf("Issue #%d %s", msg.Number, msg.Title)
		m.setDetailContent(issueMarkdown(ghcli.Issue(msg)))
		return m, nil
	case errorMsg:
		m.screen = screenError
		m.loading = false
		m.errText = msg.err.Error()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenError:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	case screenDetail:
		return m.handleDetailKey(msg)
	default:
		return m.handleListKey(msg)
	}
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		if m.tab == tabPRs {
			m.tab = tabIssues
		} else {
			m.tab = tabPRs
		}
		if !m.loaded[m.tab] {
			m.loading = true
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
		m.loading = true
		return m, fetchList(m.src, m.tab)
	case "enter":
		if t, ok := m.selectedTarget(); ok {
			m.detailTarget = t
			m.detailTitle = ""
			m.screen = screenDetail
			m.loading = true
			return m, fetchDetail(m.src, t)
		}
		return m, nil
	case "o":
		if t, ok := m.selectedTarget(); ok {
			return m, openWeb(m.src, t)
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

func (m Model) selectedTarget() (Target, bool) {
	if m.tab == tabPRs {
		if len(m.prs) == 0 {
			return Target{}, false
		}
		return Target{Kind: KindPR, Number: m.prs[m.cursors[tabPRs]].Number}, true
	}
	if len(m.issues) == 0 {
		return Target{}, false
	}
	return Target{Kind: KindIssue, Number: m.issues[m.cursors[tabIssues]].Number}, true
}

func (m Model) enterList() (tea.Model, tea.Cmd) {
	m.screen = screenList
	if !m.loaded[m.tab] {
		m.loading = true
		return m, fetchList(m.src, m.tab)
	}
	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return m.enterList()
	case "o":
		return m, openWeb(m.src, m.detailTarget)
	case "r":
		m.loading = true
		return m, fetchDetail(m.src, m.detailTarget)
	case "ctrl+c":
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg) // j/k などのスクロールは viewport に委譲
	return m, cmd
}

// setDetailContent renders markdown through glamour into the viewport.
// glamour が失敗した場合は生の Markdown をそのまま表示する。
func (m *Model) setDetailContent(md string) {
	width := m.width
	if width <= 0 {
		width = 80
	}
	content := md
	if r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-2)); err == nil {
		if out, err := r.Render(md); err == nil {
			content = out
		}
	}
	m.detail.SetContent(content)
	m.detail.GotoTop()
}

func (m Model) View() string {
	switch m.screen {
	case screenError:
		return errorView(m.errText)
	case screenDetail:
		return m.detailView()
	default:
		return m.listView()
	}
}
