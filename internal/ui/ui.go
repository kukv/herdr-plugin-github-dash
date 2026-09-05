// Package ui implements the octoscope terminal UI.
package ui

import (
	"errors"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/kukv/octoscope/internal/gh"
	"github.com/kukv/octoscope/internal/i18n"
)

// prSource is what the screens need for pull requests.
// repo is "owner/repo"; the empty string targets the workspace repository.
type prSource interface {
	ListPRs() ([]gh.PR, error)
	GetPR(repo string, number int) (gh.PR, error)
	OpenPRWeb(repo string, number int) error
	AddPRComment(repo string, number int, body string) error
	ClosePR(repo string, number int) error
	ReopenPR(repo string, number int) error
	EditPRLabels(repo string, number int, add, remove []string) error
	EditPRAssignees(repo string, number int, add, remove []string) error
}

// issueSource mirrors prSource for issues. The two are kept apart rather
// than merged so that adding a PR-only call cannot silently imply an issue
// equivalent that GitHub does not offer.
type issueSource interface {
	ListIssues() ([]gh.Issue, error)
	GetIssue(repo string, number int) (gh.Issue, error)
	OpenIssueWeb(repo string, number int) error
	AddIssueComment(repo string, number int, body string) error
	CloseIssue(repo string, number int) error
	ReopenIssue(repo string, number int) error
	EditIssueLabels(repo string, number int, add, remove []string) error
	EditIssueAssignees(repo string, number int, add, remove []string) error
}

// repoNamer names the workspace repository. It stands alone because it is
// the one call that belongs to neither kind.
type repoNamer interface {
	RepoName() (string, error)
}

// candidateSource lists what a picker offers. Labels and assignees belong
// to the repository, not to a PR or an issue, so this too is kind-free.
type candidateSource interface {
	ListLabels(repo string) ([]gh.Label, error)
	ListAssignees(repo string) ([]string, error)
}

// DataSource is the union the root model is handed. A command that acts on
// one kind takes the matching half; a command that picks the kind at run
// time takes the whole.
type DataSource interface {
	prSource
	issueSource
	repoNamer
	candidateSource
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
	prListMsg           []gh.PR
	issueListMsg        []gh.Issue
	prDetailMsg         gh.PR
	issueDetailMsg      gh.Issue
	repoNameMsg         string
	errorMsg            struct{ err error }
	commentPostedMsg    struct{}
	commentErrorMsg     struct{ err error }
	stateChangedMsg     struct{}
	stateErrorMsg       struct{ err error }
	pickerCandidatesMsg struct {
		kind   pickerKind
		labels []gh.Label
		users  []string
	}
	pickerAppliedMsg struct{}
	pickErrorMsg     struct{ err error }
)

type Model struct {
	src DataSource

	width, height int
	repoName      string

	screen        screen
	tab           tabID
	cursors       [2]int
	prs           []gh.PR
	issues        []gh.Issue
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

	picking         bool
	pickerLoading   bool
	applying        bool
	picker          picker
	detailLabels    []string
	detailAssignees []string
}

func New(src DataSource) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	ta := textarea.New()
	ta.Placeholder = i18n.T("compose.placeholder")
	ta.ShowLineNumbers = false
	m := Model{
		src:      src,
		spin:     s,
		screen:   screenList,
		detail:   viewport.New(viewport.WithWidth(80), viewport.WithHeight(20)),
		textarea: ta,
	}
	m.listLoading[m.tab] = true
	return m
}

func (m Model) Init() tea.Cmd {
	if m.screen == screenError {
		return nil
	}
	cmds := []tea.Cmd{m.spin.Tick, fetchRepoName(m.src), fetchList(m.src, m.tab)}
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

func fetchRepoName(src repoNamer) tea.Cmd {
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

func fetchLabelPicker(src candidateSource, target Target) tea.Cmd {
	return func() tea.Msg {
		labels, err := src.ListLabels(target.Repo)
		if err != nil {
			return pickErrorMsg{err}
		}
		return pickerCandidatesMsg{kind: pickLabels, labels: labels}
	}
}

func fetchAssigneePicker(src candidateSource, target Target) tea.Cmd {
	return func() tea.Msg {
		users, err := src.ListAssignees(target.Repo)
		if err != nil {
			return pickErrorMsg{err}
		}
		return pickerCandidatesMsg{kind: pickAssignees, users: users}
	}
}

func applyPicker(src DataSource, target Target, kind pickerKind, add, remove []string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch {
		case kind == pickLabels && target.Kind == KindPR:
			err = src.EditPRLabels(target.Repo, target.Number, add, remove)
		case kind == pickLabels:
			err = src.EditIssueLabels(target.Repo, target.Number, add, remove)
		case target.Kind == KindPR:
			err = src.EditPRAssignees(target.Repo, target.Number, add, remove)
		default:
			err = src.EditIssueAssignees(target.Repo, target.Number, add, remove)
		}
		if err != nil {
			return pickErrorMsg{err}
		}
		return pickerAppliedMsg{}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.detail.SetWidth(msg.Width)
		m.detail.SetHeight(max(msg.Height-4, 5))
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
		m.prs = []gh.PR(msg)
		m.loaded[tabPRs] = true
		if m.cursors[tabPRs] >= len(m.prs) {
			m.cursors[tabPRs] = max(len(m.prs)-1, 0)
		}
		m.listLoading[tabPRs] = false
		return m, nil
	case issueListMsg:
		m.issues = []gh.Issue(msg)
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
		m.detailLabels = labelNames(msg.Labels)
		m.detailAssignees = authorLogins(msg.Assignees)
		m.detailTitle = i18n.Tf("detail.pr_title", map[string]any{"Number": msg.Number, "Title": msg.Title})
		m.setDetailContent(prMarkdown(gh.PR(msg)))
		return m, nil
	case issueDetailMsg:
		m.detailLoading = false
		m.detailState = msg.State
		m.actionErr = ""
		m.detailLabels = labelNames(msg.Labels)
		m.detailAssignees = authorLogins(msg.Assignees)
		m.detailTitle = i18n.Tf("detail.issue_title", map[string]any{"Number": msg.Number, "Title": msg.Title})
		m.setDetailContent(issueMarkdown(gh.Issue(msg)))
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
	case pickerCandidatesMsg:
		m.pickerLoading = false
		if msg.kind == pickLabels {
			names := make([]string, len(msg.labels))
			colors := make(map[string]string, len(msg.labels))
			for i, l := range msg.labels {
				names[i] = l.Name
				colors[l.Name] = l.Color
			}
			m.picker = newPicker(pickLabels, i18n.T("picker.labels"), names, colors, m.detailLabels)
		} else {
			m.picker = newPicker(pickAssignees, i18n.T("picker.assignees"), msg.users, nil, m.detailAssignees)
		}
		m.picking = true
		return m, nil
	case pickerAppliedMsg:
		m.picking = false
		m.applying = false
		m.detailLoading = true
		return m, fetchDetail(m.src, m.detailTarget)
	case pickErrorMsg:
		if m.picking {
			m.applying = false
			m.picker.err = msg.err.Error()
		} else {
			m.pickerLoading = false
			m.actionErr = msg.err.Error()
		}
		return m, nil
	case errorMsg:
		m.screen = screenError
		if errors.Is(msg.err, gh.ErrGhNotFound) {
			m.errText = i18n.T("error.gh_not_found")
		} else {
			m.errText = msg.err.Error()
		}
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
			m.picking = false
			m.pickerLoading = false
			m.applying = false
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

func labelNames(labels []gh.Label) []string {
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = l.Name
	}
	return names
}

func authorLogins(authors []gh.Author) []string {
	logins := make([]string, len(authors))
	for i, a := range authors {
		logins[i] = a.Login
	}
	return logins
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
	if m.picking {
		return m.handlePickerKey(msg)
	}
	if m.pickerLoading {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil // 候補取得中は他のキーを無視する
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
	case "l":
		if m.detailLoading || m.pickerLoading {
			return m, nil
		}
		m.pickerLoading = true
		m.actionErr = ""
		return m, fetchLabelPicker(m.src, m.detailTarget)
	case "a":
		if m.detailLoading || m.pickerLoading {
			return m, nil
		}
		m.pickerLoading = true
		m.actionErr = ""
		return m, fetchAssigneePicker(m.src, m.detailTarget)
	case "ctrl+c":
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg) // j/k などのスクロールは viewport に委譲
	return m, cmd
}

func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.applying {
		return m, nil // 適用中はそれ以外の入力を無視する
	}
	switch msg.String() {
	case "esc":
		m.picking = false
		return m, nil
	case "j", "down":
		m.picker.moveDown(visibleRows(m.height))
		return m, nil
	case "k", "up":
		m.picker.moveUp()
		return m, nil
	case " ", "space":
		m.picker.toggle()
		return m, nil
	case "enter":
		add, remove := m.picker.diff()
		if len(add) == 0 && len(remove) == 0 {
			m.picking = false // 変更なしは閉じるだけ
			return m, nil
		}
		m.applying = true
		m.picker.err = ""
		return m, applyPicker(m.src, m.detailTarget, m.picker.kind, add, remove)
	}
	return m, nil
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
	if r, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width-2)); err == nil {
		if out, err := r.Render(md); err == nil {
			content = out
		}
	}
	m.detail.SetContent(content)
	m.detail.GotoTop()
}

func (m Model) View() tea.View {
	var content string
	switch m.screen {
	case screenError:
		content = errorView(m.errText)
	case screenDetail:
		content = m.detailView()
	default:
		content = m.listView()
	}
	v := tea.NewView(content)
	v.AltScreen = true // v1 の tea.WithAltScreen() 相当
	return v
}
