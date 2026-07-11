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

func itoa(n int) string { return string(rune('0' + n)) } // テスト内は n < 10 のみ

func samplePRs() []ghcli.PR {
	return []ghcli.PR{
		{Number: 1, Title: "first pr", Author: ghcli.Author{Login: "kukv"},
			UpdatedAt: time.Now(), ReviewDecision: "APPROVED"},
		{Number: 2, Title: "second pr", Author: ghcli.Author{Login: "bob"},
			UpdatedAt: time.Now()},
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
		pr: ghcli.PR{Number: 1, Title: "first pr", Author: ghcli.Author{Login: "kukv"},
			Body: "the body text", Comments: []ghcli.Comment{
				{Author: ghcli.Author{Login: "bob"}, Body: "a comment"}}},
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
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"},
		commentErr: errors.New("gh pr: HTTP 403 forbidden")}
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
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr"},
		commentErr: errors.New("gh pr: HTTP 403 forbidden")}
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
