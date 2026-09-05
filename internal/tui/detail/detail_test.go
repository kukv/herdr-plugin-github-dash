package detail

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kukv/octoscope/internal/gh"
)

// fakeSource implements Source and records calls.
type fakeSource struct {
	pr       gh.PR
	issue    gh.Issue
	err      error
	webCalls []string // "pr:<repo>:<n>" / "issue:<repo>:<n>"

	commentCalls []string // "pr:<repo>:<n>:<body>" / "issue:<repo>:<n>:<body>"
	commentErr   error

	stateCalls []string // action:kind:repo:number, e.g. "close:pr::1"
	stateErr   error

	labels    []gh.Label
	users     []string
	editCalls []string // "pr:labels::1:add=bug:remove=wip"
	labelsErr error
	usersErr  error
	editErr   error
}

func (f *fakeSource) GetPR(ctx context.Context, repo string, n int) (gh.PR, error) {
	return f.pr, f.err
}

func (f *fakeSource) GetIssue(ctx context.Context, repo string, n int) (gh.Issue, error) {
	return f.issue, f.err
}

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

func (f *fakeSource) ListLabels(ctx context.Context, repo string) ([]gh.Label, error) {
	return f.labels, f.labelsErr
}

func (f *fakeSource) ListAssignees(ctx context.Context, repo string) ([]string, error) {
	return f.users, f.usersErr
}

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

func itoa(n int) string { return string(rune('0' + n)) } // tests only use n < 10

func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace}
	default:
		return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
}

func prRef() gh.ItemRef    { return gh.ItemRef{Kind: gh.ItemPR, Number: 1} }
func issueRef() gh.ItemRef { return gh.ItemRef{Kind: gh.ItemIssue, Number: 5} }

// loaded returns a model whose item has already arrived.
func loaded(f *fakeSource, ref gh.ItemRef) Model {
	m := New(f, ref)
	m, _ = m.Update(m.Init()())
	return m
}

func TestDetailRendersBodyAndComments(t *testing.T) {
	f := &fakeSource{pr: gh.PR{
		Number: 1, Title: "first pr", Author: gh.Author{Login: "kukv"},
		Body: "the body text", Comments: []gh.Comment{
			{Author: gh.Author{Login: "bob"}, Body: "a comment"},
		},
	}}
	m := loaded(f, prRef())
	// glamour v2 styles per cell, so the body is checked with the ANSI stripped.
	view := ansi.Strip(m.View())
	for _, want := range []string{"first pr", "the body text", "a comment"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view missing %q:\n%s", want, view)
		}
	}
}

func TestEscAndQCloseTheView(t *testing.T) {
	for _, k := range []string{"esc", "q"} {
		t.Run(k, func(t *testing.T) {
			f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr"}}
			m := loaded(f, prRef())
			_, cmd := m.Update(key(k))
			if cmd == nil {
				t.Fatalf("cmd = nil after %s, want ClosedMsg cmd", k)
			}
			if _, ok := cmd().(ClosedMsg); !ok {
				t.Errorf("msg = %T, want ClosedMsg", cmd())
			}
		})
	}
}

func TestFetchFailureBecomesErrorMsg(t *testing.T) {
	f := &fakeSource{err: errors.New("gh pr: no git remotes found")}
	m := New(f, prRef())
	m, cmd := m.Update(m.Init()())
	if m.loading {
		t.Errorf("loading = true after a failed fetch, want false")
	}
	if cmd == nil {
		t.Fatal("cmd = nil after a failed fetch, want ErrorMsg cmd")
	}
	msg, ok := cmd().(ErrorMsg)
	if !ok {
		t.Fatalf("msg = %T, want ErrorMsg", cmd())
	}
	if !strings.Contains(msg.Err.Error(), "no git remotes found") {
		t.Errorf("Err = %v, want the source's error", msg.Err)
	}
}

func TestOOpensBrowser(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr"}}
	m := loaded(f, prRef())
	_, cmd := m.Update(key("o"))
	if cmd == nil {
		t.Fatal("cmd = nil, want openWeb cmd")
	}
	cmd()
	if len(f.webCalls) != 1 || f.webCalls[0] != "pr::1" {
		t.Errorf("webCalls = %v, want [pr::1]", f.webCalls)
	}
}

func TestDetailRefetchesOnR(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr"}}
	m := loaded(f, prRef())
	m, cmd := m.Update(key("r"))
	if !m.loading || cmd == nil {
		t.Errorf("loading = %v, cmd = %v; want loading with fetch cmd", m.loading, cmd)
	}
}

func TestCEntersCompose(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("c"))
	if !m.composing {
		t.Errorf("composing = false, want true after c")
	}
}

func TestComposeEmptyBodyNotSent(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("c"))
	m, cmd := m.Update(key("ctrl+s")) // the textarea is empty
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
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("c"))
	m.textarea.SetValue("looks good")
	m, cmd := m.Update(key("ctrl+s"))
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
	m, cmd = m.Update(msg)
	if m.composing || m.posting || !m.loading || cmd == nil {
		t.Errorf("after posted: composing=%v posting=%v loading=%v cmd=%v; want false,false,true,non-nil",
			m.composing, m.posting, m.loading, cmd)
	}
}

func TestComposeEscCancels(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("c"))
	m.textarea.SetValue("draft")
	m, cmd := m.Update(key("esc"))
	if m.composing {
		t.Errorf("composing = true after esc, want false")
	}
	if cmd != nil {
		t.Errorf("cmd = non-nil after esc, want nil (esc cancels compose, it does not close the view)")
	}
}

func TestComposePostErrorKeepsDraft(t *testing.T) {
	f := &fakeSource{
		pr:         gh.PR{Number: 1, Title: "first pr"},
		commentErr: errors.New("gh pr: HTTP 403 forbidden"),
	}
	m := loaded(f, prRef())
	m, _ = m.Update(key("c"))
	m.textarea.SetValue("hello")
	m, cmd := m.Update(key("ctrl+s"))
	m, _ = m.Update(cmd()) // commentErrorMsg
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
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("c"))
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
		pr:         gh.PR{Number: 1, Title: "first pr"},
		commentErr: errors.New("gh pr: HTTP 403 forbidden"),
	}
	m := loaded(f, prRef())
	m, _ = m.Update(key("c"))
	m.textarea.SetValue("hello")
	m, cmd := m.Update(key("ctrl+s"))
	m, _ = m.Update(cmd())
	if !strings.Contains(m.View(), "403") {
		t.Errorf("compose view missing error text:\n%s", m.View())
	}
}

func TestComposeSubmitOnIssueRoutesToIssueComment(t *testing.T) {
	f := &fakeSource{issue: gh.Issue{Number: 5, Title: "an issue"}}
	m := loaded(f, issueRef())
	m, _ = m.Update(key("c"))
	m.textarea.SetValue("issue comment")
	m, cmd := m.Update(key("ctrl+s"))
	if !m.posting || cmd == nil {
		t.Fatalf("posting = %v, cmd = %v; want posting with post cmd", m.posting, cmd)
	}
	if _, ok := cmd().(commentPostedMsg); !ok {
		t.Fatalf("msg = %T, want commentPostedMsg", cmd())
	}
	if len(f.commentCalls) != 1 || f.commentCalls[0] != "issue::5:issue comment" {
		t.Errorf("commentCalls = %v, want [issue::5:issue comment]", f.commentCalls)
	}
}

func TestComposeIgnoresKeysWhilePosting(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("c"))
	m.textarea.SetValue("hello")
	m, _ = m.Update(key("ctrl+s")) // posting == true (the cmd is deliberately not run)
	if !m.posting {
		t.Fatalf("precondition: posting = false, want true")
	}
	m, cmd := m.Update(key("esc"))
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

func TestXEntersConfirmWhenOpen(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("x"))
	if !m.confirming {
		t.Errorf("confirming = false, want true after x on open item")
	}
}

func TestXIgnoredWhenMerged(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr", State: "MERGED"}}
	m := loaded(f, prRef())
	m, cmd := m.Update(key("x"))
	if m.confirming {
		t.Errorf("confirming = true, want false for merged item")
	}
	if cmd != nil {
		t.Errorf("cmd = non-nil, want nil for merged item")
	}
}

func TestConfirmYClosesAndRefetches(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("x"))
	m, cmd := m.Update(key("y"))
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
	m, cmd = m.Update(msg)
	if m.confirming || m.working || !m.loading || cmd == nil {
		t.Errorf("after changed: confirming=%v working=%v loading=%v cmd=%v; want false,false,true,non-nil",
			m.confirming, m.working, m.loading, cmd)
	}
}

func TestConfirmReopenRoutesToReopen(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr", State: "CLOSED"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("x"))
	_, cmd := m.Update(key("y"))
	if cmd == nil {
		t.Fatal("cmd = nil, want reopen cmd")
	}
	if _, ok := cmd().(stateChangedMsg); !ok {
		t.Fatalf("msg = %T, want stateChangedMsg", cmd())
	}
	if len(f.stateCalls) != 1 || f.stateCalls[0] != "reopen:pr::1" {
		t.Errorf("stateCalls = %v, want [reopen:pr::1]", f.stateCalls)
	}
}

func TestConfirmNCancels(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("x"))
	m, cmd := m.Update(key("n"))
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
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("x"))
	m, cmd := m.Update(key("esc"))
	if m.confirming {
		t.Errorf("confirming = true after esc, want false")
	}
	if cmd != nil {
		t.Errorf("cmd = non-nil after esc, want nil (esc cancels the confirmation, it does not close the view)")
	}
}

func TestStateErrorStaysOnDetail(t *testing.T) {
	f := &fakeSource{
		pr:       gh.PR{Number: 1, Title: "first pr", State: "OPEN"},
		stateErr: errors.New("gh pr: HTTP 403 forbidden"),
	}
	m := loaded(f, prRef())
	m, _ = m.Update(key("x"))
	m, cmd := m.Update(key("y"))
	m, cmd = m.Update(cmd()) // stateErrorMsg
	if m.confirming {
		t.Errorf("confirming = true, want false after error")
	}
	if m.working {
		t.Errorf("working = true, want false after error")
	}
	if cmd != nil {
		t.Errorf("cmd = non-nil, want nil (a state failure stays inline, it is not the parent's error screen)")
	}
	if !strings.Contains(m.actionErr, "403") {
		t.Errorf("actionErr = %q, want to contain 403", m.actionErr)
	}
	if !strings.Contains(m.View(), "403") {
		t.Errorf("detail view missing inline error:\n%s", m.View())
	}
}

func TestConfirmViewShowsPrompt(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("x"))
	view := m.View()
	for _, want := range []string{"Close", "(y/n)"} {
		if !strings.Contains(view, want) {
			t.Errorf("confirm view missing %q:\n%s", want, view)
		}
	}
}

func TestConfirmViewReopenWording(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr", State: "CLOSED"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("x"))
	if !strings.Contains(m.View(), "Reopen") {
		t.Errorf("confirm view missing Reopen wording:\n%s", m.View())
	}
}

func TestDetailFooterShowsStateAndPickerKeys(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := loaded(f, prRef())
	view := m.View()
	for _, want := range []string{"x:close", "l:labels", "a:assign"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail footer missing %q:\n%s", want, view)
		}
	}
}

// TestActionErrClearedOnReload guards against a stale actionErr surviving a
// successful reload: a failed close leaves actionErr set, and a subsequent
// r-triggered refresh must clear it once the new detail arrives.
func TestActionErrClearedOnReload(t *testing.T) {
	f := &fakeSource{
		pr:       gh.PR{Number: 1, Title: "first pr", State: "OPEN"},
		stateErr: errors.New("gh pr: HTTP 403 forbidden"),
	}
	m := loaded(f, prRef())
	m, _ = m.Update(key("x"))
	m, cmd := m.Update(key("y"))
	m, _ = m.Update(cmd()) // stateErrorMsg
	if !strings.Contains(m.actionErr, "403") {
		t.Fatalf("precondition: actionErr = %q, want to contain 403", m.actionErr)
	}

	m, cmd = m.Update(key("r"))
	if !m.loading || cmd == nil {
		t.Fatalf("loading = %v, cmd = %v; want loading with fetch cmd", m.loading, cmd)
	}
	m, _ = m.Update(cmd()) // prMsg
	if m.actionErr != "" {
		t.Errorf("actionErr = %q after reload, want empty", m.actionErr)
	}
	if strings.Contains(m.View(), "403") {
		t.Errorf("view still shows stale error after reload:\n%s", m.View())
	}
}

func TestConfirmSubmitOnIssueRoutesToClose(t *testing.T) {
	f := &fakeSource{issue: gh.Issue{Number: 5, Title: "an issue", State: "OPEN"}}
	m := loaded(f, issueRef())
	m, _ = m.Update(key("x"))
	_, cmd := m.Update(key("y"))
	if cmd == nil {
		t.Fatal("cmd = nil, want state cmd")
	}
	if _, ok := cmd().(stateChangedMsg); !ok {
		t.Fatalf("msg = %T, want stateChangedMsg", cmd())
	}
	if len(f.stateCalls) != 1 || f.stateCalls[0] != "close:issue::5" {
		t.Errorf("stateCalls = %v, want [close:issue::5]", f.stateCalls)
	}
}

func TestConfirmIgnoresKeysWhileWorking(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr", State: "OPEN"}}
	m := loaded(f, prRef())
	m, _ = m.Update(key("x"))
	m, _ = m.Update(key("y")) // working == true (the cmd is deliberately not run)
	if !m.working {
		t.Fatalf("precondition: working = false, want true")
	}
	m, cmd := m.Update(key("esc"))
	if cmd != nil {
		t.Errorf("cmd = non-nil while working, want nil")
	}
	if !m.working || !m.confirming {
		t.Errorf("working/confirming changed while working: working=%v confirming=%v", m.working, m.confirming)
	}
}

func TestLoadingShowsLoadingText(t *testing.T) {
	f := &fakeSource{pr: gh.PR{Number: 1, Title: "first pr"}}
	m := New(f, prRef())
	if !strings.Contains(m.View(), "loading...") {
		t.Errorf("view missing the loading text before the fetch resolves:\n%s", m.View())
	}
}
