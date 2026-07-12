// Package ui implements the GitHub Dash terminal UI.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
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
	AddPRComment(repo string, number int, body string) error
	AddIssueComment(repo string, number int, body string) error
	ClosePR(repo string, number int) error
	ReopenPR(repo string, number int) error
	CloseIssue(repo string, number int) error
	ReopenIssue(repo string, number int) error
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
	prListMsg        []ghcli.PR
	issueListMsg     []ghcli.Issue
	prDetailMsg      ghcli.PR
	issueDetailMsg   ghcli.Issue
	repoNameMsg      string
	errorMsg         struct{ err error }
	commentPostedMsg struct{}
	commentErrorMsg  struct{ err error }
	stateChangedMsg  struct{}
	stateErrorMsg    struct{ err error }
)

type Model struct {
	src DataSource

	width, height int
	repoName      string

	screen        screen
	tab           tabID
	cursors       [2]int
	prs           []ghcli.PR
	issues        []ghcli.Issue
	loaded        [2]bool
	listLoading   [2]bool
	detailLoading bool

	detail       viewport.Model
	detailTitle  string
	detailTarget Target

	spin    spinner.Model
	errText string

	textarea  textarea.Model
	composing bool
	posting   bool
	postErr   string

	detailState string
	confirming  bool
	working     bool
	actionErr   string
}

func New(src DataSource, initial *Target) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	ta := textarea.New()
	ta.Placeholder = "Leave a comment..."
	ta.ShowLineNumbers = false
	m := Model{
		src:      src,
		spin:     s,
		screen:   screenList,
		detail:   viewport.New(80, 20),
		textarea: ta,
	}
	if initial != nil {
		m.screen = screenDetail
		m.detailTarget = *initial
		m.detailLoading = true
	} else {
		m.listLoading[m.tab] = true
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

func postComment(src DataSource, target Target, body string) tea.Cmd {
	return func() tea.Msg {
		var err error
		if target.Kind == KindPR {
			err = src.AddPRComment(target.Repo, target.Number, body)
		} else {
			err = src.AddIssueComment(target.Repo, target.Number, body)
		}
		if err != nil {
			return commentErrorMsg{err}
		}
		return commentPostedMsg{}
	}
}

// stateAction reports whether the shown item can change state, and if so
// whether the action is a close (true) or a reopen (false).
func (m Model) stateAction() (closing bool, ok bool) {
	switch m.detailState {
	case "OPEN":
		return true, true
	case "CLOSED":
		return false, true
	default:
		return false, false // MERGED や未取得はアクション無し
	}
}

func setState(src DataSource, target Target, closing bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch {
		case target.Kind == KindPR && closing:
			err = src.ClosePR(target.Repo, target.Number)
		case target.Kind == KindPR:
			err = src.ReopenPR(target.Repo, target.Number)
		case closing:
			err = src.CloseIssue(target.Repo, target.Number)
		default:
			err = src.ReopenIssue(target.Repo, target.Number)
		}
		if err != nil {
			return stateErrorMsg{err}
		}
		return stateChangedMsg{}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.detail.Width = msg.Width
		m.detail.Height = max(msg.Height-4, 5)
		m.textarea.SetWidth(msg.Width)
		m.textarea.SetHeight(max(msg.Height-6, 3))
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
		m.listLoading[tabPRs] = false
		return m, nil
	case issueListMsg:
		m.issues = []ghcli.Issue(msg)
		m.loaded[tabIssues] = true
		if m.cursors[tabIssues] >= len(m.issues) {
			m.cursors[tabIssues] = max(len(m.issues)-1, 0)
		}
		m.listLoading[tabIssues] = false
		return m, nil
	case prDetailMsg:
		m.detailLoading = false
		m.detailState = msg.State
		m.actionErr = ""
		m.detailTitle = fmt.Sprintf("PR #%d %s", msg.Number, msg.Title)
		m.setDetailContent(prMarkdown(ghcli.PR(msg)))
		return m, nil
	case issueDetailMsg:
		m.detailLoading = false
		m.detailState = msg.State
		m.actionErr = ""
		m.detailTitle = fmt.Sprintf("Issue #%d %s", msg.Number, msg.Title)
		m.setDetailContent(issueMarkdown(ghcli.Issue(msg)))
		return m, nil
	case commentPostedMsg:
		m.composing = false
		m.posting = false
		m.postErr = ""
		m.textarea.Reset()
		m.detailLoading = true
		return m, fetchDetail(m.src, m.detailTarget)
	case commentErrorMsg:
		m.posting = false
		m.postErr = msg.err.Error()
		return m, nil
	case stateChangedMsg:
		m.confirming = false
		m.working = false
		m.actionErr = ""
		m.detailLoading = true
		return m, fetchDetail(m.src, m.detailTarget)
	case stateErrorMsg:
		m.confirming = false
		m.working = false
		m.actionErr = msg.err.Error()
		return m, nil
	case errorMsg:
		m.screen = screenError
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
			m.listLoading[m.tab] = true
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
		m.listLoading[m.tab] = true
		return m, fetchList(m.src, m.tab)
	case "enter":
		if t, ok := m.selectedTarget(); ok {
			m.detailTarget = t
			m.detailTitle = ""
			m.detailState = ""
			m.confirming = false
			m.actionErr = ""
			m.screen = screenDetail
			m.detailLoading = true
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
		m.listLoading[m.tab] = true
		return m, fetchList(m.src, m.tab)
	}
	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.composing {
		return m.handleComposeKey(msg)
	}
	if m.confirming {
		return m.handleConfirmKey(msg)
	}
	switch msg.String() {
	case "q", "esc":
		return m.enterList()
	case "o":
		return m, openWeb(m.src, m.detailTarget)
	case "r":
		m.detailLoading = true
		return m, fetchDetail(m.src, m.detailTarget)
	case "c":
		if m.detailLoading {
			return m, nil
		}
		m.composing = true
		m.postErr = ""
		m.textarea.Reset()
		m.textarea.Focus()
		return m, textarea.Blink
	case "x":
		if m.detailLoading {
			return m, nil
		}
		if _, ok := m.stateAction(); !ok {
			return m, nil // merged など: アクション無し
		}
		m.confirming = true
		m.actionErr = ""
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg) // j/k などのスクロールは viewport に委譲
	return m, cmd
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.working {
		return m, nil // 実行中はそれ以外の入力を無視する
	}
	switch msg.String() {
	case "y":
		closing, ok := m.stateAction()
		if !ok {
			m.confirming = false
			return m, nil
		}
		m.working = true
		m.actionErr = ""
		return m, setState(m.src, m.detailTarget, closing)
	case "n", "esc":
		m.confirming = false
		m.actionErr = ""
		return m, nil
	}
	return m, nil
}

func (m Model) handleComposeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.posting {
		return m, nil // 送信中はそれ以外の入力を無視する
	}
	switch msg.String() {
	case "esc":
		m.composing = false
		m.postErr = ""
		m.textarea.Reset()
		return m, nil
	case "ctrl+s":
		if strings.TrimSpace(m.textarea.Value()) == "" {
			return m, nil // 空本文は送信しない
		}
		m.posting = true
		m.postErr = ""
		return m, postComment(m.src, m.detailTarget, m.textarea.Value())
	}
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
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
