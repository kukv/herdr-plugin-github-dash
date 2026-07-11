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
