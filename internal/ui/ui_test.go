package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kukv/herdr-plugin-github-dash/internal/ghcli"
)

// fakeSource implements DataSource and records calls.
type fakeSource struct {
	prs      []ghcli.PR
	issues   []ghcli.Issue
	pr       ghcli.PR
	issue    ghcli.Issue
	err      error
	webCalls []string // "pr:<repo>:<n>" / "issue:<repo>:<n>"

	commentCalls []string // "pr:<repo>:<n>:<body>" / "issue:<repo>:<n>:<body>"
	commentErr   error

	stateCalls []string // "close:pr:<repo>:<n>" 等の action:kind:repo:number
	stateErr   error

	labels    []ghcli.Label
	users     []string
	editCalls []string // "pr:labels::12:add=bug:remove=wip"
	labelsErr error
	usersErr  error
	editErr   error
}

func (f *fakeSource) ListPRs() ([]ghcli.PR, error)       { return f.prs, f.err }
func (f *fakeSource) ListIssues() ([]ghcli.Issue, error) { return f.issues, f.err }
func (f *fakeSource) GetPR(repo string, n int) (ghcli.PR, error) {
	return f.pr, f.err
}

func (f *fakeSource) GetIssue(repo string, n int) (ghcli.Issue, error) {
	return f.issue, f.err
}
func (f *fakeSource) RepoName() (string, error) { return "kukv/demo", f.err }
func (f *fakeSource) OpenPRWeb(repo string, n int) error {
	f.webCalls = append(f.webCalls, "pr:"+repo+":"+itoa(n))
	return nil
}

func (f *fakeSource) OpenIssueWeb(repo string, n int) error {
	f.webCalls = append(f.webCalls, "issue:"+repo+":"+itoa(n))
	return nil
}

func (f *fakeSource) AddPRComment(repo string, n int, body string) error {
	f.commentCalls = append(f.commentCalls, "pr:"+repo+":"+itoa(n)+":"+body)
	return f.commentErr
}

func (f *fakeSource) AddIssueComment(repo string, n int, body string) error {
	f.commentCalls = append(f.commentCalls, "issue:"+repo+":"+itoa(n)+":"+body)
	return f.commentErr
}

func (f *fakeSource) ClosePR(repo string, n int) error {
	f.stateCalls = append(f.stateCalls, "close:pr:"+repo+":"+itoa(n))
	return f.stateErr
}

func (f *fakeSource) ReopenPR(repo string, n int) error {
	f.stateCalls = append(f.stateCalls, "reopen:pr:"+repo+":"+itoa(n))
	return f.stateErr
}

func (f *fakeSource) CloseIssue(repo string, n int) error {
	f.stateCalls = append(f.stateCalls, "close:issue:"+repo+":"+itoa(n))
	return f.stateErr
}

func (f *fakeSource) ReopenIssue(repo string, n int) error {
	f.stateCalls = append(f.stateCalls, "reopen:issue:"+repo+":"+itoa(n))
	return f.stateErr
}

func (f *fakeSource) ListLabels(repo string) ([]ghcli.Label, error) { return f.labels, f.labelsErr }
func (f *fakeSource) ListAssignees(repo string) ([]string, error)   { return f.users, f.usersErr }
func (f *fakeSource) EditPRLabels(repo string, n int, add, remove []string) error {
	f.editCalls = append(f.editCalls, "pr:labels:"+repo+":"+itoa(n)+editSuffix(add, remove))
	return f.editErr
}

func (f *fakeSource) EditIssueLabels(repo string, n int, add, remove []string) error {
	f.editCalls = append(f.editCalls, "issue:labels:"+repo+":"+itoa(n)+editSuffix(add, remove))
	return f.editErr
}

func (f *fakeSource) EditPRAssignees(repo string, n int, add, remove []string) error {
	f.editCalls = append(f.editCalls, "pr:assignees:"+repo+":"+itoa(n)+editSuffix(add, remove))
	return f.editErr
}

func (f *fakeSource) EditIssueAssignees(repo string, n int, add, remove []string) error {
	f.editCalls = append(f.editCalls, "issue:assignees:"+repo+":"+itoa(n)+editSuffix(add, remove))
	return f.editErr
}

func editSuffix(add, remove []string) string {
	return ":add=" + strings.Join(add, ",") + ":remove=" + strings.Join(remove, ",")
}

func itoa(n int) string { return string(rune('0' + n)) } // テスト内は n < 10 のみ

func samplePRs() []ghcli.PR {
	return []ghcli.PR{
		{
			Number: 1, Title: "first pr", Author: ghcli.Author{Login: "kukv"},
			UpdatedAt: time.Now(), ReviewDecision: "APPROVED",
		},
		{
			Number: 2, Title: "second pr", Author: ghcli.Author{Login: "bob"},
			UpdatedAt: time.Now(),
		},
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// loadedModel returns a Model with the PR list already loaded.
func loadedModel(f *fakeSource) Model {
	m := New(f, nil)
	next, _ := m.Update(prListMsg(f.prs))
	return next.(Model)
}

// detailModel returns a Model already on the loaded detail screen for PR #1.
func detailModel(f *fakeSource) Model {
	m := loadedModel(f)
	next, cmd := m.Update(key("enter"))
	m = next.(Model)
	next, _ = m.Update(cmd()) // fetchDetail の結果を流し込む
	return next.(Model)
}

func TestPRListRenders(t *testing.T) {
	f := &fakeSource{prs: samplePRs()}
	m := loadedModel(f)
	view := m.View()
	for _, want := range []string{"first pr", "second pr", "@kukv", "#1"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestEmptyPRList(t *testing.T) {
	f := &fakeSource{}
	m := loadedModel(f)
	if !strings.Contains(m.View(), "No open pull requests") {
		t.Errorf("view missing empty state:\n%s", m.View())
	}
}

func TestCursorMovesAndClamps(t *testing.T) {
	f := &fakeSource{prs: samplePRs()}
	m := loadedModel(f)
	for _, k := range []string{"j", "j", "j"} { // 2件しかないので末尾で止まる
		next, _ := m.Update(key(k))
		m = next.(Model)
	}
	if m.cursors[tabPRs] != 1 {
		t.Errorf("cursor = %d, want 1", m.cursors[tabPRs])
	}
	next, _ := m.Update(key("k"))
	m = next.(Model)
	if m.cursors[tabPRs] != 0 {
		t.Errorf("cursor = %d, want 0", m.cursors[tabPRs])
	}
}

func TestTabSwitchLoadsIssues(t *testing.T) {
	f := &fakeSource{issues: []ghcli.Issue{{Number: 3, Title: "an issue"}}}
	m := loadedModel(f)
	next, cmd := m.Update(key("tab"))
	m = next.(Model)
	if m.tab != tabIssues || cmd == nil {
		t.Fatalf("tab = %v, cmd = %v; want tabIssues with fetch cmd", m.tab, cmd)
	}
	next, _ = m.Update(cmd()) // fetch を同期実行して結果を流し込む
	m = next.(Model)
	if !strings.Contains(m.View(), "an issue") {
		t.Errorf("view missing issue:\n%s", m.View())
	}
}

func TestQQuitsOnList(t *testing.T) {
	f := &fakeSource{prs: samplePRs()}
	m := loadedModel(f)
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("cmd = nil, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("msg = %T, want tea.QuitMsg", cmd())
	}
}

func TestErrorMsgShowsErrorScreen(t *testing.T) {
	f := &fakeSource{}
	m := loadedModel(f)
	next, _ := m.Update(errorMsg{errors.New("gh pr: no git remotes found")})
	m = next.(Model)
	if !strings.Contains(m.View(), "no git remotes found") {
		t.Errorf("view missing error text:\n%s", m.View())
	}
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q on error screen should quit")
	}
}

func TestOOpensBrowserForSelection(t *testing.T) {
	f := &fakeSource{prs: samplePRs()}
	m := loadedModel(f)
	_, cmd := m.Update(key("o"))
	if cmd == nil {
		t.Fatal("cmd = nil, want openWeb cmd")
	}
	cmd()
	if len(f.webCalls) != 1 || f.webCalls[0] != "pr::1" {
		t.Errorf("webCalls = %v, want [pr::1]", f.webCalls)
	}
}

func TestEnterOpensDetailAndEscReturns(t *testing.T) {
	f := &fakeSource{
		prs: samplePRs(),
		pr: ghcli.PR{
			Number: 1, Title: "first pr", Author: ghcli.Author{Login: "kukv"},
			Body: "the body text", Comments: []ghcli.Comment{
				{Author: ghcli.Author{Login: "bob"}, Body: "a comment"},
			},
		},
	}
	m := loadedModel(f)
	next, cmd := m.Update(key("enter"))
	m = next.(Model)
	if m.screen != screenDetail || cmd == nil {
		t.Fatalf("screen = %v, cmd = %v; want screenDetail with fetch cmd", m.screen, cmd)
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	view := m.View()
	for _, want := range []string{"first pr", "the body text", "a comment"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view missing %q:\n%s", want, view)
		}
	}
	next, _ = m.Update(key("esc"))
	m = next.(Model)
	if m.screen != screenList {
		t.Errorf("screen = %v after esc, want screenList", m.screen)
	}
}

func TestDirectModeStartsOnDetailWithRepo(t *testing.T) {
	f := &fakeSource{pr: ghcli.PR{Number: 7, Title: "external pr"}}
	m := New(f, &Target{Kind: KindPR, Repo: "octo/hello", Number: 7})
	if m.screen != screenDetail {
		t.Fatalf("screen = %v, want screenDetail", m.screen)
	}
	next, _ := m.Update(fetchDetail(f, m.detailTarget)())
	m = next.(Model)
	if !strings.Contains(m.View(), "external pr") {
		t.Errorf("view missing detail:\n%s", m.View())
	}
	// o キーは Repo を引き継いでブラウザを開く
	_, cmd := m.Update(key("o"))
	if cmd == nil {
		t.Fatal("cmd = nil, want openWeb cmd")
	}
	cmd()
	if len(f.webCalls) != 1 || f.webCalls[0] != "pr:octo/hello:7" {
		t.Errorf("webCalls = %v, want [pr:octo/hello:7]", f.webCalls)
	}
}

func TestDetailRefetchesOnR(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"}}
	m := loadedModel(f)
	next, cmd := m.Update(key("enter"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, cmd = m.Update(key("r"))
	m = next.(Model)
	if !m.detailLoading || cmd == nil {
		t.Errorf("detailLoading = %v, cmd = %v; want detailLoading with fetch cmd", m.detailLoading, cmd)
	}
}

// TestRefreshThenTabSwitchClearsCorrectLoading reproduces the stuck-spinner
// bug: pressing r on the PRs tab, then tab to the already-loaded Issues
// tab before the PR fetch returns, must not leave Issues stuck on a
// spinner when the late prListMsg finally arrives.
func TestRefreshThenTabSwitchClearsCorrectLoading(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), issues: []ghcli.Issue{{Number: 3, Title: "an issue"}}}
	m := loadedModel(f)
	next, _ := m.Update(issueListMsg(f.issues)) // Issues tab already loaded once before
	m = next.(Model)

	next, refreshCmd := m.Update(key("r")) // refresh PRs; fetch is still "in flight"
	m = next.(Model)
	if refreshCmd == nil {
		t.Fatal("cmd = nil, want fetch cmd for r")
	}

	next, tabCmd := m.Update(key("tab")) // switch to Issues before the refresh returns
	m = next.(Model)
	if m.tab != tabIssues {
		t.Fatalf("tab = %v, want tabIssues", m.tab)
	}
	if tabCmd != nil {
		t.Fatalf("switching to an already-loaded tab issued cmd = %v, want nil", tabCmd)
	}
	if view := m.View(); strings.Contains(view, "loading...") || !strings.Contains(view, "an issue") {
		t.Errorf("Issues view should render items immediately, got:\n%s", view)
	}

	next, _ = m.Update(refreshCmd()) // late prListMsg arrives while Issues is visible
	m = next.(Model)
	if view := m.View(); strings.Contains(view, "loading...") || !strings.Contains(view, "an issue") {
		t.Errorf("Issues view got stuck on spinner after late prListMsg, got:\n%s", view)
	}

	next, _ = m.Update(key("tab")) // switch back to PRs
	m = next.(Model)
	if view := m.View(); strings.Contains(view, "loading...") || !strings.Contains(view, "first pr") {
		t.Errorf("PRs view stuck on spinner or missing refreshed items, got:\n%s", view)
	}
}

// TestRefreshThenEnterKeepsDetailSpinner covers the lesser variant: r on the
// list then enter before the refresh returns. The late prListMsg must not
// clear the detail screen's spinner while the detail fetch is still pending.
func TestRefreshThenEnterKeepsDetailSpinner(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"}}
	m := loadedModel(f)

	next, refreshCmd := m.Update(key("r")) // refresh PR list; fetch in flight
	m = next.(Model)
	if refreshCmd == nil {
		t.Fatal("cmd = nil, want fetch cmd for r")
	}

	next, detailCmd := m.Update(key("enter")) // open detail before the refresh returns
	m = next.(Model)
	if m.screen != screenDetail || detailCmd == nil {
		t.Fatalf("screen = %v, cmd = %v; want screenDetail with fetch cmd", m.screen, detailCmd)
	}

	next, _ = m.Update(refreshCmd()) // late prListMsg arrives while detail is loading
	m = next.(Model)
	if view := m.View(); !strings.Contains(view, "loading...") {
		t.Errorf("detail view lost its spinner after late prListMsg, got:\n%s", view)
	}

	next, _ = m.Update(detailCmd()) // detail fetch finally resolves
	m = next.(Model)
	if view := m.View(); !strings.Contains(view, "first pr") {
		t.Errorf("detail view missing content after detail fetch resolved, got:\n%s", view)
	}
}

func TestCEntersCompose(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"}}
	m := detailModel(f)
	next, _ := m.Update(key("c"))
	m = next.(Model)
	if !m.composing {
		t.Errorf("composing = false, want true after c")
	}
	if m.screen != screenDetail {
		t.Errorf("screen = %v, want screenDetail", m.screen)
	}
}

func TestComposeEmptyBodyNotSent(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"}}
	m := detailModel(f)
	next, _ := m.Update(key("c"))
	m = next.(Model)
	next, cmd := m.Update(key("ctrl+s")) // textarea は空
	m = next.(Model)
	if cmd != nil {
		t.Errorf("cmd = non-nil, want nil for empty body")
	}
	if !m.composing {
		t.Errorf("composing = false, want still composing")
	}
	if len(f.commentCalls) != 0 {
		t.Errorf("commentCalls = %v, want none", f.commentCalls)
	}
}

func TestComposeSubmitPostsAndRefetches(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"}}
	m := detailModel(f)
	next, _ := m.Update(key("c"))
	m = next.(Model)
	m.textarea.SetValue("looks good")
	next, cmd := m.Update(key("ctrl+s"))
	m = next.(Model)
	if !m.posting || cmd == nil {
		t.Fatalf("posting = %v, cmd = %v; want posting with post cmd", m.posting, cmd)
	}
	msg := cmd()
	if _, ok := msg.(commentPostedMsg); !ok {
		t.Fatalf("msg = %T, want commentPostedMsg", msg)
	}
	if len(f.commentCalls) != 1 || f.commentCalls[0] != "pr::1:looks good" {
		t.Fatalf("commentCalls = %v, want [pr::1:looks good]", f.commentCalls)
	}
	next, cmd = m.Update(msg)
	m = next.(Model)
	if m.composing || m.posting || !m.detailLoading || cmd == nil {
		t.Errorf("after posted: composing=%v posting=%v detailLoading=%v cmd=%v; want false,false,true,non-nil",
			m.composing, m.posting, m.detailLoading, cmd)
	}
}

func TestComposeEscCancels(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"}}
	m := detailModel(f)
	next, _ := m.Update(key("c"))
	m = next.(Model)
	m.textarea.SetValue("draft")
	next, _ = m.Update(key("esc"))
	m = next.(Model)
	if m.composing {
		t.Errorf("composing = true after esc, want false")
	}
	if m.screen != screenDetail {
		t.Errorf("screen = %v after esc, want screenDetail (esc cancels compose, not detail)", m.screen)
	}
}

func TestComposePostErrorKeepsDraft(t *testing.T) {
	f := &fakeSource{
		prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"},
		commentErr: errors.New("gh pr: HTTP 403 forbidden"),
	}
	m := detailModel(f)
	next, _ := m.Update(key("c"))
	m = next.(Model)
	m.textarea.SetValue("hello")
	next, cmd := m.Update(key("ctrl+s"))
	m = next.(Model)
	next, _ = m.Update(cmd()) // commentErrorMsg
	m = next.(Model)
	if !m.composing {
		t.Errorf("composing = false, want still composing after error")
	}
	if m.posting {
		t.Errorf("posting = true, want false after error")
	}
	if !strings.Contains(m.postErr, "403") {
		t.Errorf("postErr = %q, want to contain 403", m.postErr)
	}
	if m.textarea.Value() != "hello" {
		t.Errorf("draft lost: textarea = %q, want hello", m.textarea.Value())
	}
}

func TestComposeViewShowsTextareaAndHelp(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"}}
	m := detailModel(f)
	next, _ := m.Update(key("c"))
	m = next.(Model)
	m.textarea.SetValue("my comment")
	view := m.View()
	for _, want := range []string{"my comment", "ctrl+s:send", "esc:cancel"} {
		if !strings.Contains(view, want) {
			t.Errorf("compose view missing %q:\n%s", want, view)
		}
	}
}

func TestComposeViewShowsPostError(t *testing.T) {
	f := &fakeSource{
		prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"},
		commentErr: errors.New("gh pr: HTTP 403 forbidden"),
	}
	m := detailModel(f)
	next, _ := m.Update(key("c"))
	m = next.(Model)
	m.textarea.SetValue("hello")
	next, cmd := m.Update(key("ctrl+s"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !strings.Contains(m.View(), "403") {
		t.Errorf("compose view missing error text:\n%s", m.View())
	}
}

func TestComposeSubmitOnIssueRoutesToIssueComment(t *testing.T) {
	f := &fakeSource{
		issues: []ghcli.Issue{{Number: 5, Title: "an issue"}},
		issue:  ghcli.Issue{Number: 5, Title: "an issue"},
	}
	// New model starts on the PR tab with an empty PR list; switch to Issues,
	// load them, open detail for the issue, then compose.
	m := New(f, nil)
	next, cmd := m.Update(key("tab")) // -> Issues tab, triggers fetchList
	m = next.(Model)
	next, _ = m.Update(cmd()) // issueListMsg
	m = next.(Model)
	next, cmd = m.Update(key("enter")) // open issue detail
	m = next.(Model)
	next, _ = m.Update(cmd()) // issueDetailMsg
	m = next.(Model)
	next, _ = m.Update(key("c")) // enter compose
	m = next.(Model)
	m.textarea.SetValue("issue comment")
	next, cmd = m.Update(key("ctrl+s"))
	m = next.(Model)
	if !m.posting || cmd == nil {
		t.Fatalf("posting = %v, cmd = %v; want posting with post cmd", m.posting, cmd)
	}
	if _, ok := cmd().(commentPostedMsg); !ok {
		t.Fatalf("msg type wrong")
	}
	if len(f.commentCalls) != 1 || f.commentCalls[0] != "issue::5:issue comment" {
		t.Errorf("commentCalls = %v, want [issue::5:issue comment]", f.commentCalls)
	}
}

func TestComposeIgnoresKeysWhilePosting(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"}}
	m := detailModel(f)
	next, _ := m.Update(key("c"))
	m = next.(Model)
	m.textarea.SetValue("hello")
	next, _ = m.Update(key("ctrl+s")) // now posting == true (cmd not run, so no msg yet)
	m = next.(Model)
	if !m.posting {
		t.Fatalf("precondition: posting = false, want true")
	}
	// A keystroke while posting must be a no-op: no state change, no extra cmd.
	next, cmd := m.Update(key("esc"))
	m = next.(Model)
	if cmd != nil {
		t.Errorf("cmd = non-nil while posting, want nil")
	}
	if !m.posting || !m.composing {
		t.Errorf("posting/composing changed while posting: posting=%v composing=%v", m.posting, m.composing)
	}
	if m.textarea.Value() != "hello" {
		t.Errorf("draft changed while posting: %q", m.textarea.Value())
	}
}

func TestComposeCtrlCQuitsWhilePosting(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"}}
	m := detailModel(f)
	next, _ := m.Update(key("c"))
	m = next.(Model)
	m.textarea.SetValue("hello")
	next, _ = m.Update(key("ctrl+s")) // posting == true (cmd intentionally not run)
	m = next.(Model)
	if !m.posting {
		t.Fatalf("precondition: want posting=true")
	}
	_, cmd := m.Update(key("ctrl+c"))
	if cmd == nil {
		t.Fatal("cmd = nil while posting, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("msg = %T, want tea.QuitMsg", cmd())
	}
}

func TestXEntersConfirmWhenOpen(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	if !m.confirming {
		t.Errorf("confirming = false, want true after x on open item")
	}
	if m.screen != screenDetail {
		t.Errorf("screen = %v, want screenDetail", m.screen)
	}
}

func TestXIgnoredWhenMerged(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "MERGED"}}
	m := detailModel(f)
	next, cmd := m.Update(key("x"))
	m = next.(Model)
	if m.confirming {
		t.Errorf("confirming = true, want false for merged item")
	}
	if cmd != nil {
		t.Errorf("cmd = non-nil, want nil for merged item")
	}
}

func TestConfirmYClosesAndRefetches(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	next, cmd := m.Update(key("y"))
	m = next.(Model)
	if !m.working || cmd == nil {
		t.Fatalf("working = %v, cmd = %v; want working with state cmd", m.working, cmd)
	}
	msg := cmd()
	if _, ok := msg.(stateChangedMsg); !ok {
		t.Fatalf("msg = %T, want stateChangedMsg", msg)
	}
	if len(f.stateCalls) != 1 || f.stateCalls[0] != "close:pr::1" {
		t.Fatalf("stateCalls = %v, want [close:pr::1]", f.stateCalls)
	}
	next, cmd = m.Update(msg)
	m = next.(Model)
	if m.confirming || m.working || !m.detailLoading || cmd == nil {
		t.Errorf("after changed: confirming=%v working=%v detailLoading=%v cmd=%v; want false,false,true,non-nil",
			m.confirming, m.working, m.detailLoading, cmd)
	}
}

func TestConfirmReopenRoutesToReopen(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "CLOSED"}}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	next, cmd := m.Update(key("y"))
	_ = next.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want reopen cmd")
	}
	if _, ok := cmd().(stateChangedMsg); !ok {
		t.Fatalf("msg type wrong")
	}
	if len(f.stateCalls) != 1 || f.stateCalls[0] != "reopen:pr::1" {
		t.Errorf("stateCalls = %v, want [reopen:pr::1]", f.stateCalls)
	}
}

func TestConfirmNCancels(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	next, cmd := m.Update(key("n"))
	m = next.(Model)
	if m.confirming {
		t.Errorf("confirming = true after n, want false")
	}
	if cmd != nil {
		t.Errorf("cmd = non-nil after n, want nil")
	}
	if len(f.stateCalls) != 0 {
		t.Errorf("stateCalls = %v, want none", f.stateCalls)
	}
}

func TestConfirmEscCancels(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	next, _ = m.Update(key("esc"))
	m = next.(Model)
	if m.confirming {
		t.Errorf("confirming = true after esc, want false")
	}
	if m.screen != screenDetail {
		t.Errorf("screen = %v after esc, want screenDetail (esc cancels confirm, not detail)", m.screen)
	}
}

func TestStateErrorStaysOnDetail(t *testing.T) {
	f := &fakeSource{
		prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		stateErr: errors.New("gh pr: HTTP 403 forbidden"),
	}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	next, cmd := m.Update(key("y"))
	m = next.(Model)
	next, _ = m.Update(cmd()) // stateErrorMsg
	m = next.(Model)
	if m.confirming {
		t.Errorf("confirming = true, want false after error")
	}
	if m.working {
		t.Errorf("working = true, want false after error")
	}
	if m.screen != screenDetail {
		t.Errorf("screen = %v, want screenDetail (error stays on detail)", m.screen)
	}
	if !strings.Contains(m.actionErr, "403") {
		t.Errorf("actionErr = %q, want to contain 403", m.actionErr)
	}
}

func TestConfirmViewShowsPrompt(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	view := m.View()
	for _, want := range []string{"Close", "(y/n)"} {
		if !strings.Contains(view, want) {
			t.Errorf("confirm view missing %q:\n%s", want, view)
		}
	}
}

func TestConfirmViewReopenWording(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "CLOSED"}}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	if !strings.Contains(m.View(), "Reopen") {
		t.Errorf("confirm view missing Reopen wording:\n%s", m.View())
	}
}

func TestDetailFooterShowsStateKey(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := detailModel(f)
	if !strings.Contains(m.View(), "x:close") {
		t.Errorf("detail footer missing x:close for open item:\n%s", m.View())
	}
}

func TestStateErrorShownInline(t *testing.T) {
	f := &fakeSource{
		prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		stateErr: errors.New("gh pr: HTTP 403 forbidden"),
	}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	next, cmd := m.Update(key("y"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !strings.Contains(m.View(), "403") {
		t.Errorf("detail view missing inline error:\n%s", m.View())
	}
}

// TestActionErrClearedOnReload guards against a stale actionErr surviving a
// successful detail reload: a failed close leaves actionErr set, and a
// subsequent r-triggered refresh must clear it once the new detail arrives,
// even though it does not go through the list->detail enter path.
func TestActionErrClearedOnReload(t *testing.T) {
	f := &fakeSource{
		prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		stateErr: errors.New("gh pr: HTTP 403 forbidden"),
	}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	next, cmd := m.Update(key("y"))
	m = next.(Model)
	next, _ = m.Update(cmd()) // stateErrorMsg
	m = next.(Model)
	if !strings.Contains(m.actionErr, "403") {
		t.Fatalf("precondition: actionErr = %q, want to contain 403", m.actionErr)
	}
	if m.screen != screenDetail {
		t.Fatalf("precondition: screen = %v, want screenDetail", m.screen)
	}

	next, cmd = m.Update(key("r"))
	m = next.(Model)
	if !m.detailLoading || cmd == nil {
		t.Fatalf("detailLoading = %v, cmd = %v; want detailLoading with fetch cmd", m.detailLoading, cmd)
	}
	next, _ = m.Update(cmd()) // prDetailMsg
	m = next.(Model)
	if m.actionErr != "" {
		t.Errorf("actionErr = %q after reload, want empty", m.actionErr)
	}
	if strings.Contains(m.View(), "403") {
		t.Errorf("view still shows stale error after reload:\n%s", m.View())
	}
}

func TestConfirmSubmitOnIssueRoutesToClose(t *testing.T) {
	f := &fakeSource{
		issues: []ghcli.Issue{{Number: 5, Title: "an issue"}},
		issue:  ghcli.Issue{Number: 5, Title: "an issue", State: "OPEN"},
	}
	// New model starts on the PR tab with an empty PR list; switch to Issues,
	// load them, open detail for the issue, then confirm a close.
	m := New(f, nil)
	next, cmd := m.Update(key("tab")) // -> Issues tab, triggers fetchList
	m = next.(Model)
	next, _ = m.Update(cmd()) // issueListMsg
	m = next.(Model)
	next, cmd = m.Update(key("enter")) // open issue detail
	m = next.(Model)
	next, _ = m.Update(cmd()) // issueDetailMsg
	m = next.(Model)
	next, _ = m.Update(key("x"))
	m = next.(Model)
	next, cmd = m.Update(key("y"))
	_ = next.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want state cmd")
	}
	if _, ok := cmd().(stateChangedMsg); !ok {
		t.Fatalf("msg type wrong")
	}
	if len(f.stateCalls) != 1 || f.stateCalls[0] != "close:issue::5" {
		t.Errorf("stateCalls = %v, want [close:issue::5]", f.stateCalls)
	}
}

func TestConfirmIgnoresKeysWhileWorking(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	next, _ = m.Update(key("y")) // now working == true (cmd not run, so no msg yet)
	m = next.(Model)
	if !m.working {
		t.Fatalf("precondition: working = false, want true")
	}
	// A keystroke while working must be a no-op: no state change, no extra cmd.
	next, cmd := m.Update(key("esc"))
	m = next.(Model)
	if cmd != nil {
		t.Errorf("cmd = non-nil while working, want nil")
	}
	if !m.working || !m.confirming {
		t.Errorf("working/confirming changed while working: working=%v confirming=%v", m.working, m.confirming)
	}
}

func TestConfirmCtrlCQuitsWhileWorking(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := detailModel(f)
	next, _ := m.Update(key("x"))
	m = next.(Model)
	next, _ = m.Update(key("y")) // working == true (cmd intentionally not run)
	m = next.(Model)
	if !m.working {
		t.Fatalf("precondition: want working=true")
	}
	_, cmd := m.Update(key("ctrl+c"))
	if cmd == nil {
		t.Fatal("cmd = nil while working, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("msg = %T, want tea.QuitMsg", cmd())
	}
}

func TestLOpensLabelPickerPrechecked(t *testing.T) {
	f := &fakeSource{
		prs:    samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}},
	}
	m := detailModel(f)
	next, cmd := m.Update(key("l"))
	m = next.(Model)
	if !m.pickerLoading || cmd == nil {
		t.Fatalf("pickerLoading = %v, cmd = %v; want loading with fetch cmd", m.pickerLoading, cmd)
	}
	next, _ = m.Update(cmd()) // pickerCandidatesMsg
	m = next.(Model)
	if !m.picking || m.pickerLoading {
		t.Fatalf("picking = %v, pickerLoading = %v; want picking", m.picking, m.pickerLoading)
	}
	if m.picker.kind != pickLabels || len(m.picker.items) != 2 {
		t.Fatalf("picker = %+v, want 2 label items", m.picker)
	}
	if !m.picker.items[0].selected { // "bug" precheck
		t.Errorf("current label not prechecked: %+v", m.picker.items)
	}
}

func TestAOpensAssigneePicker(t *testing.T) {
	f := &fakeSource{
		prs:   samplePRs(),
		pr:    ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		users: []string{"alice", "bob"},
	}
	m := detailModel(f)
	next, cmd := m.Update(key("a"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !m.picking || m.picker.kind != pickAssignees || len(m.picker.items) != 2 {
		t.Fatalf("picker = %+v, want 2 assignee items", m.picker)
	}
}

func TestPickerApplyComputesDiffAndRefetches(t *testing.T) {
	f := &fakeSource{
		prs:    samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}},
	}
	m := detailModel(f)
	next, cmd := m.Update(key("l"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	// toggle bug off (cursor 0), move to wip, toggle on
	next, _ = m.Update(key("space"))
	m = next.(Model)
	next, _ = m.Update(key("j"))
	m = next.(Model)
	next, _ = m.Update(key("space"))
	m = next.(Model)
	next, cmd = m.Update(key("enter"))
	m = next.(Model)
	if !m.applying || cmd == nil {
		t.Fatalf("applying = %v, cmd = %v; want applying with edit cmd", m.applying, cmd)
	}
	msg := cmd()
	if _, ok := msg.(pickerAppliedMsg); !ok {
		t.Fatalf("msg = %T, want pickerAppliedMsg", msg)
	}
	if len(f.editCalls) != 1 || f.editCalls[0] != "pr:labels::1:add=wip:remove=bug" {
		t.Fatalf("editCalls = %v, want [pr:labels::1:add=wip:remove=bug]", f.editCalls)
	}
	next, cmd = m.Update(msg)
	m = next.(Model)
	if m.picking || m.applying || !m.detailLoading || cmd == nil {
		t.Errorf("after applied: picking=%v applying=%v detailLoading=%v cmd=%v; want false,false,true,non-nil",
			m.picking, m.applying, m.detailLoading, cmd)
	}
}

func TestPickerNoChangeClosesWithoutEdit(t *testing.T) {
	f := &fakeSource{
		prs:    samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}},
	}
	m := detailModel(f)
	next, cmd := m.Update(key("l"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, cmd = m.Update(key("enter")) // no toggle
	m = next.(Model)
	if m.picking {
		t.Errorf("picking = true, want closed after empty-diff enter")
	}
	if cmd != nil {
		t.Errorf("cmd = non-nil, want nil for empty diff")
	}
	if len(f.editCalls) != 0 {
		t.Errorf("editCalls = %v, want none", f.editCalls)
	}
}

func TestPickerEscCancels(t *testing.T) {
	f := &fakeSource{
		prs:    samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}},
	}
	m := detailModel(f)
	next, cmd := m.Update(key("l"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, _ = m.Update(key("space")) // change something
	m = next.(Model)
	next, _ = m.Update(key("esc"))
	m = next.(Model)
	if m.picking {
		t.Errorf("picking = true after esc, want false")
	}
	if len(f.editCalls) != 0 {
		t.Errorf("editCalls = %v, want none after esc", f.editCalls)
	}
}

func TestPickerApplyErrorKeepsPicker(t *testing.T) {
	f := &fakeSource{
		prs:     samplePRs(),
		pr:      ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels:  []ghcli.Label{{Name: "bug"}, {Name: "wip"}},
		editErr: errors.New("gh pr: HTTP 403 forbidden"),
	}
	m := detailModel(f)
	next, cmd := m.Update(key("l"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, _ = m.Update(key("space")) // toggle bug off -> a diff
	m = next.(Model)
	next, cmd = m.Update(key("enter"))
	m = next.(Model)
	next, _ = m.Update(cmd()) // pickErrorMsg
	m = next.(Model)
	if !m.picking {
		t.Errorf("picking = false, want still picking after apply error")
	}
	if m.applying {
		t.Errorf("applying = true, want false after error")
	}
	if !strings.Contains(m.picker.err, "403") {
		t.Errorf("picker.err = %q, want to contain 403", m.picker.err)
	}
}

func TestPickerFetchErrorInlineOnDetail(t *testing.T) {
	f := &fakeSource{
		prs:       samplePRs(),
		pr:        ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		labelsErr: errors.New("gh label: HTTP 403 forbidden"),
	}
	m := detailModel(f)
	next, cmd := m.Update(key("l"))
	m = next.(Model)
	next, _ = m.Update(cmd()) // pickErrorMsg (picking was never set)
	m = next.(Model)
	if m.picking || m.pickerLoading {
		t.Errorf("picking/pickerLoading = %v/%v, want false", m.picking, m.pickerLoading)
	}
	if m.screen != screenDetail {
		t.Errorf("screen = %v, want screenDetail", m.screen)
	}
	if !strings.Contains(m.actionErr, "403") {
		t.Errorf("actionErr = %q, want to contain 403", m.actionErr)
	}
}

func TestPickerApplyOnIssueRoutesToIssue(t *testing.T) {
	f := &fakeSource{
		issues: []ghcli.Issue{{Number: 5, Title: "an issue"}},
		issue:  ghcli.Issue{Number: 5, Title: "an issue", State: "OPEN"},
		labels: []ghcli.Label{{Name: "bug"}},
	}
	m := New(f, nil)
	next, cmd := m.Update(key("tab")) // -> Issues
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, cmd = m.Update(key("enter"))
	m = next.(Model)
	next, _ = m.Update(cmd()) // issueDetailMsg
	m = next.(Model)
	next, cmd = m.Update(key("l"))
	m = next.(Model)
	next, _ = m.Update(cmd()) // pickerCandidatesMsg
	m = next.(Model)
	next, _ = m.Update(key("space")) // add bug
	m = next.(Model)
	next, cmd = m.Update(key("enter"))
	_ = next.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want edit cmd")
	}
	cmd()
	if len(f.editCalls) != 1 || f.editCalls[0] != "issue:labels::5:add=bug:remove=" {
		t.Errorf("editCalls = %v, want [issue:labels::5:add=bug:remove=]", f.editCalls)
	}
}

func TestPickerLoadingIgnoresKeys(t *testing.T) {
	f := &fakeSource{
		prs:    samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}},
	}
	m := detailModel(f)
	next, _ := m.Update(key("l"))
	m = next.(Model)
	if !m.pickerLoading {
		t.Fatalf("pickerLoading = false, want true right after pressing l")
	}
	next, cmd := m.Update(key("x")) // fetch still in flight; must be a no-op
	m = next.(Model)
	if cmd != nil {
		t.Errorf("cmd = %v, want nil while pickerLoading", cmd)
	}
	if m.confirming || m.composing || m.picking {
		t.Errorf("confirming=%v composing=%v picking=%v, want all false while pickerLoading",
			m.confirming, m.composing, m.picking)
	}
	if m.screen != screenDetail {
		t.Errorf("screen = %v, want screenDetail", m.screen)
	}
}

func TestPickerLoadingCtrlCQuits(t *testing.T) {
	f := &fakeSource{
		prs:    samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}},
	}
	m := detailModel(f)
	next, _ := m.Update(key("l"))
	m = next.(Model)
	next, cmd := m.Update(key("ctrl+c"))
	_ = next.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want tea.Quit while pickerLoading")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}

func TestPickerApplyAssigneesRoutesToPR(t *testing.T) {
	f := &fakeSource{
		prs:   samplePRs(),
		pr:    ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		users: []string{"alice"},
	}
	m := detailModel(f)
	next, cmd := m.Update(key("a"))
	m = next.(Model)
	next, _ = m.Update(cmd()) // pickerCandidatesMsg
	m = next.(Model)
	next, _ = m.Update(key("space")) // select alice -> add
	m = next.(Model)
	next, cmd = m.Update(key("enter"))
	_ = next.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want edit cmd")
	}
	cmd()
	if len(f.editCalls) != 1 || f.editCalls[0] != "pr:assignees::1:add=alice:remove=" {
		t.Errorf("editCalls = %v, want [pr:assignees::1:add=alice:remove=]", f.editCalls)
	}
}

func TestPickerViewShowsItemsAndHelp(t *testing.T) {
	f := &fakeSource{
		prs:    samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}},
	}
	m := detailModel(f)
	next, cmd := m.Update(key("l"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	view := m.View()
	for _, want := range []string{"Labels", "[x] bug", "[ ] wip", "space:toggle", "enter:apply", "esc:cancel"} {
		if !strings.Contains(view, want) {
			t.Errorf("picker view missing %q:\n%s", want, view)
		}
	}
}

func TestPickerViewShowsApplyError(t *testing.T) {
	f := &fakeSource{
		prs:     samplePRs(),
		pr:      ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels:  []ghcli.Label{{Name: "bug"}, {Name: "wip"}},
		editErr: errors.New("gh pr: HTTP 403 forbidden"),
	}
	m := detailModel(f)
	next, cmd := m.Update(key("l"))
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	next, _ = m.Update(key("space"))
	m = next.(Model)
	next, cmd = m.Update(key("enter"))
	m = next.(Model)
	next, _ = m.Update(cmd()) // pickErrorMsg
	m = next.(Model)
	if !strings.Contains(m.View(), "403") {
		t.Errorf("picker view missing error text:\n%s", m.View())
	}
}

func TestDetailFooterShowsLabelAssignKeys(t *testing.T) {
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := detailModel(f)
	view := m.View()
	for _, want := range []string{"l:labels", "a:assign"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail footer missing %q:\n%s", want, view)
		}
	}
}
