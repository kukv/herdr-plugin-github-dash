# GitHub Dash Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** herdr のペイン内で、ワークスペース連動リポジトリの GitHub PR / Issue を一覧・詳細表示する TUI プラグインを作る。

**Architecture:** Go + bubbletea の TUI が `gh` CLI をサブプロセスとして呼び、対象ディレクトリは `HERDR_PLUGIN_CONTEXT_JSON` の `workspace_cwd` から解決する。マニフェストのアクション → `open.sh` シム → `herdr plugin pane open` でオーバーレイペインを開く。リンクハンドラー経由では `GITHUB_DASH_URL` 環境変数で詳細画面へ直行する。

**Tech Stack:** Go（ローカル 1.26.4）、charmbracelet/bubbletea・bubbles・lipgloss・glamour（いずれも v1 系）、gh CLI、herdr 0.7.x

**Spec:** `docs/superpowers/specs/2026-07-11-github-dash-design.md`（承認済み・実機検証結果を含む）

## Global Constraints

- Go モジュールパス: `github.com/kukv/herdr-plugin-github-dash`
- プラグイン ID: `kukv.github-dash` / `min_herdr_version = "0.7.0"`
- 外部依存は charmbracelet の4つ（bubbletea / bubbles / lipgloss / glamour、v1 系 import path）のみ。テストは標準 `testing` のみ使用
- `gh` は必ず `exec.Cmd.Dir` に対象ディレクトリを設定して実行する（ペインプロセスの cwd はプラグインルートであり、リポジトリではない）
- コミットは conventional commits（`feat:` / `docs:` / `chore:`）。SSH 署名はリポジトリローカルに設定済み。メッセージ末尾に `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>` を付ける
- Phase 2/3 の機能（コメント投稿・編集・レビュー）は入れない。設定ファイル・複数リポ対応も入れない
- 実機検証済みの事実（スペック「実機検証の結果」参照）: コンテキスト JSON はフラットで `workspace_cwd` / `focused_pane_cwd` を持つ。ペインプロセスにもネイティブに渡る

## File Structure

```
herdr-plugin.toml                # マニフェスト（Task 6）
open.sh                          # アクション → pane open シム（Task 6）
go.mod / go.sum                  # Task 1 で init、Task 3 で deps 追加
main.go                          # 配線 + URL パース（Task 5）
main_test.go
internal/herdrctx/herdrctx.go    # コンテキスト JSON → 対象ディレクトリ（Task 1）
internal/herdrctx/herdrctx_test.go
internal/ghcli/ghcli.go          # gh CLI ラッパー（Task 2）
internal/ghcli/ghcli_test.go
internal/ui/ui.go                # Model / Update / fetch cmds（Task 3, 4）
internal/ui/render.go            # View 描画・Markdown 組み立て（Task 3, 4）
internal/ui/ui_test.go
internal/ui/render_test.go
README.md                        # 使い方追記（Task 6）
```

---

### Task 1: Go モジュール初期化 + herdrctx（対象ディレクトリ解決）

**Files:**
- Create: `go.mod`
- Create: `internal/herdrctx/herdrctx.go`
- Test: `internal/herdrctx/herdrctx_test.go`

**Interfaces:**
- Consumes: なし（標準ライブラリのみ）
- Produces: `herdrctx.Resolve(contextJSON string) (string, error)` — 後続 Task 5 の main.go が `os.Getenv("HERDR_PLUGIN_CONTEXT_JSON")` を渡して呼ぶ。`herdrctx.ErrNoTargetDir` を sentinel error として公開

- [ ] **Step 1: ブランチ名を実装用にリネームし、モジュールを初期化する**

```bash
git branch -m feat/github-dash-phase1   # 未 push のため安全
go mod init github.com/kukv/herdr-plugin-github-dash
```

Expected: `go: creating new go.mod: module github.com/kukv/herdr-plugin-github-dash`

- [ ] **Step 2: 失敗するテストを書く**

`internal/herdrctx/herdrctx_test.go`:

```go
package herdrctx

import (
	"errors"
	"testing"
)

// 実機検証（2026-07-12, herdr 0.7.1）で取得した実サンプル。
const realContextJSON = `{"workspace_id":"w4","workspace_label":"herdr-plugin-github-dash","workspace_cwd":"/home/tech/dev/ghq/github.com/kukv/herdr-plugin-github-dash","tab_id":"w4:t1","tab_label":"1","focused_pane_id":"w4:p2","focused_pane_cwd":"/home/tech/dev/ghq/github.com/kukv/herdr-plugin-github-dash","focused_pane_status":"unknown","invocation_source":"cli","correlation_id":"cli:plugin"}`

func TestResolveUsesWorkspaceCwd(t *testing.T) {
	dir, err := Resolve(realContextJSON)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	want := "/home/tech/dev/ghq/github.com/kukv/herdr-plugin-github-dash"
	if dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

func TestResolveFallsBackToFocusedPaneCwd(t *testing.T) {
	dir, err := Resolve(`{"focused_pane_cwd":"/some/pane/dir"}`)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if dir != "/some/pane/dir" {
		t.Errorf("dir = %q, want /some/pane/dir", dir)
	}
}

func TestResolveErrorsWhenNoCwd(t *testing.T) {
	for name, input := range map[string]string{
		"empty json":  `{}`,
		"empty value": `{"workspace_cwd":"","focused_pane_cwd":""}`,
		"empty string": "",
	} {
		if _, err := Resolve(input); !errors.Is(err, ErrNoTargetDir) {
			t.Errorf("%s: err = %v, want ErrNoTargetDir", name, err)
		}
	}
}

func TestResolveErrorsOnInvalidJSON(t *testing.T) {
	if _, err := Resolve("not-json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}
```

- [ ] **Step 3: テストが失敗することを確認する**

Run: `go test ./internal/herdrctx/ -v`
Expected: FAIL（`undefined: Resolve` のコンパイルエラー）

- [ ] **Step 4: 最小実装を書く**

`internal/herdrctx/herdrctx.go`:

```go
// Package herdrctx resolves the directory GitHub Dash operates in
// from the Herdr plugin invocation context.
package herdrctx

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNoTargetDir is returned when the invocation context carries no usable cwd.
var ErrNoTargetDir = errors.New("herdr context has no workspace or pane cwd")

type invocationContext struct {
	WorkspaceCwd   string `json:"workspace_cwd"`
	FocusedPaneCwd string `json:"focused_pane_cwd"`
}

// Resolve picks the target directory from the HERDR_PLUGIN_CONTEXT_JSON value.
func Resolve(contextJSON string) (string, error) {
	if contextJSON == "" {
		return "", fmt.Errorf("HERDR_PLUGIN_CONTEXT_JSON is empty: %w", ErrNoTargetDir)
	}
	var ctx invocationContext
	if err := json.Unmarshal([]byte(contextJSON), &ctx); err != nil {
		return "", fmt.Errorf("parse HERDR_PLUGIN_CONTEXT_JSON: %w", err)
	}
	if ctx.WorkspaceCwd != "" {
		return ctx.WorkspaceCwd, nil
	}
	if ctx.FocusedPaneCwd != "" {
		return ctx.FocusedPaneCwd, nil
	}
	return "", ErrNoTargetDir
}
```

- [ ] **Step 5: テストが通ることを確認する**

Run: `go test ./internal/herdrctx/ -v`
Expected: PASS（4 テストすべて）

- [ ] **Step 6: コミット**

```bash
gofmt -l . && go vet ./...   # gofmt は出力なし、vet はエラーなしを確認
git add go.mod internal/herdrctx/
git commit -m "feat: add herdrctx package to resolve target directory

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 2: ghcli（gh CLI ラッパー）

**Files:**
- Create: `internal/ghcli/ghcli.go`
- Test: `internal/ghcli/ghcli_test.go`

**Interfaces:**
- Consumes: なし
- Produces（Task 3〜5 が使う）:
  - 型: `ghcli.PR`, `ghcli.Issue`, `ghcli.Comment`, `ghcli.Label`, `ghcli.Author`（フィールドは下記実装のとおり）
  - `ghcli.New(dir string) *Client`
  - `(*Client) ListPRs() ([]PR, error)` / `(*Client) ListIssues() ([]Issue, error)`
  - `(*Client) GetPR(repo string, number int) (PR, error)` / `(*Client) GetIssue(repo string, number int) (Issue, error)` — `repo` は `"owner/repo"`。空文字列ならワークスペースのリポジトリ（`--repo` を付けない）
  - `(*Client) RepoName() (string, error)`
  - `(*Client) OpenPRWeb(repo string, number int) error` / `(*Client) OpenIssueWeb(repo string, number int) error`
  - `ghcli.ErrGhNotFound`（sentinel error）

- [ ] **Step 1: 失敗するテストを書く**

`internal/ghcli/ghcli_test.go`（同一パッケージのホワイトボックステスト。フィクスチャは実機の gh 2.95 出力形状に合わせている）:

```go
package ghcli

import (
	"errors"
	"reflect"
	"testing"
)

const prListJSON = `[{"number":12,"title":"feat: add pane view","author":{"is_bot":false,"login":"kukv"},"state":"OPEN","isDraft":false,"updatedAt":"2026-07-11T10:00:00Z","reviewDecision":"APPROVED","url":"https://github.com/kukv/demo/pull/12"}]`

const prViewJSON = `{"number":12,"title":"feat: add pane view","author":{"is_bot":false,"login":"kukv"},"state":"OPEN","isDraft":false,"updatedAt":"2026-07-11T10:00:00Z","reviewDecision":"REVIEW_REQUIRED","url":"https://github.com/kukv/demo/pull/12","body":"Adds the pane.","labels":[{"id":"LA_x","name":"Kind: Feature","description":"","color":"ededed"}],"comments":[{"author":{"login":"bob"},"body":"LGTM","createdAt":"2026-07-11T11:00:00Z"}]}`

const issueListJSON = `[{"number":3,"title":"bug: crash on empty list","author":{"is_bot":false,"login":"alice"},"state":"OPEN","updatedAt":"2026-07-10T09:00:00Z","labels":[],"url":"https://github.com/kukv/demo/issues/3"}]`

// fakeRun records invocations and returns canned output.
type fakeRun struct {
	dir  string
	args []string
	out  []byte
	err  error
}

func (f *fakeRun) run(dir string, args ...string) ([]byte, error) {
	f.dir = dir
	f.args = args
	return f.out, f.err
}

func newTestClient(out string, err error) (*Client, *fakeRun) {
	f := &fakeRun{out: []byte(out), err: err}
	return &Client{dir: "/repo", run: f.run}, f
}

func TestListPRs(t *testing.T) {
	c, f := newTestClient(prListJSON, nil)
	prs, err := c.ListPRs()
	if err != nil {
		t.Fatalf("ListPRs: %v", err)
	}
	wantArgs := []string{"pr", "list", "--json", prListFields}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if f.dir != "/repo" {
		t.Errorf("dir = %q, want /repo", f.dir)
	}
	if len(prs) != 1 || prs[0].Number != 12 || prs[0].Author.Login != "kukv" ||
		prs[0].ReviewDecision != "APPROVED" {
		t.Errorf("unexpected parse result: %+v", prs)
	}
}

func TestListPRsEmpty(t *testing.T) {
	c, _ := newTestClient(`[]`, nil)
	prs, err := c.ListPRs()
	if err != nil || len(prs) != 0 {
		t.Errorf("prs = %v, err = %v; want empty, nil", prs, err)
	}
}

func TestGetPRParsesDetailFields(t *testing.T) {
	c, f := newTestClient(prViewJSON, nil)
	pr, err := c.GetPR("", 12)
	if err != nil {
		t.Fatalf("GetPR: %v", err)
	}
	wantArgs := []string{"pr", "view", "12", "--json", prViewFields}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if pr.Body != "Adds the pane." || len(pr.Comments) != 1 ||
		pr.Comments[0].Author.Login != "bob" || len(pr.Labels) != 1 ||
		pr.Labels[0].Name != "Kind: Feature" {
		t.Errorf("unexpected parse result: %+v", pr)
	}
}

func TestGetPRWithRepoOverride(t *testing.T) {
	c, f := newTestClient(prViewJSON, nil)
	if _, err := c.GetPR("octo/hello", 12); err != nil {
		t.Fatalf("GetPR: %v", err)
	}
	wantArgs := []string{"pr", "view", "12", "--json", prViewFields, "--repo", "octo/hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestListIssues(t *testing.T) {
	c, f := newTestClient(issueListJSON, nil)
	issues, err := c.ListIssues()
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	wantArgs := []string{"issue", "list", "--json", issueListFields}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if len(issues) != 1 || issues[0].Number != 3 || issues[0].Author.Login != "alice" {
		t.Errorf("unexpected parse result: %+v", issues)
	}
}

func TestRepoName(t *testing.T) {
	c, f := newTestClient(`{"nameWithOwner":"kukv/demo"}`, nil)
	name, err := c.RepoName()
	if err != nil || name != "kukv/demo" {
		t.Errorf("name = %q, err = %v; want kukv/demo, nil", name, err)
	}
	wantArgs := []string{"repo", "view", "--json", "nameWithOwner"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestOpenPRWebWithRepoOverride(t *testing.T) {
	c, f := newTestClient("", nil)
	if err := c.OpenPRWeb("octo/hello", 7); err != nil {
		t.Fatalf("OpenPRWeb: %v", err)
	}
	wantArgs := []string{"pr", "view", "7", "--web", "--repo", "octo/hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestRunErrorPassesThrough(t *testing.T) {
	wantErr := errors.New("gh pr: no git remotes found")
	c, _ := newTestClient("", wantErr)
	if _, err := c.ListPRs(); !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}
```

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ghcli/ -v`
Expected: FAIL（`undefined: Client` などのコンパイルエラー）

- [ ] **Step 3: 実装を書く**

`internal/ghcli/ghcli.go`:

```go
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
```

- [ ] **Step 4: テストが通ることを確認する**

Run: `go test ./internal/ghcli/ -v`
Expected: PASS（8 テストすべて）

- [ ] **Step 5: コミット**

```bash
gofmt -l . && go vet ./...
git add internal/ghcli/
git commit -m "feat: add ghcli package wrapping the gh CLI

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: ui 一覧画面（タブ・カーソル・空/エラー/ローディング状態）

**Files:**
- Create: `internal/ui/ui.go`
- Create: `internal/ui/render.go`
- Test: `internal/ui/ui_test.go`
- Test: `internal/ui/render_test.go`

**Interfaces:**
- Consumes: `ghcli` の型（Task 2）
- Produces（Task 4, 5 が使う）:
  - `ui.DataSource` インターフェース（下記定義）
  - `ui.New(src DataSource, initial *Target) Model` / `ui.NewError(text string) Model`
  - `ui.Target{Kind Kind; Repo string; Number int}`、`ui.KindPR` / `ui.KindIssue`
  - 内部 msg 型: `prListMsg`, `issueListMsg`, `prDetailMsg`, `issueDetailMsg`, `repoNameMsg`, `errorMsg`
  - このタスクでは詳細画面は「スピナー表示のみ」のスタブ（`detailView` は Task 4 で完成）

- [ ] **Step 1: 依存を追加する**

```bash
go get github.com/charmbracelet/bubbletea github.com/charmbracelet/bubbles github.com/charmbracelet/lipgloss github.com/charmbracelet/glamour
```

Expected: go.mod に4依存（+間接依存）が追加される

- [ ] **Step 2: 失敗するテストを書く**

`internal/ui/ui_test.go`:

```go
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

func (f *fakeSource) ListPRs() ([]ghcli.PR, error)      { return f.prs, f.err }
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
```

`internal/ui/render_test.go`:

```go
package ui

import (
	"testing"
	"time"

	"github.com/kukv/herdr-plugin-github-dash/internal/ghcli"
)

func TestRelTime(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		t    time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "now"},
		{now.Add(-5 * time.Minute), "5m ago"},
		{now.Add(-3 * time.Hour), "3h ago"},
		{now.Add(-49 * time.Hour), "2d ago"},
	}
	for _, c := range cases {
		if got := relTime(now, c.t); got != c.want {
			t.Errorf("relTime(%v) = %q, want %q", c.t, got, c.want)
		}
	}
}

func TestReviewIcon(t *testing.T) {
	cases := []struct {
		pr   ghcli.PR
		want string
	}{
		{ghcli.PR{IsDraft: true}, "◌"},
		{ghcli.PR{ReviewDecision: "APPROVED"}, "✓"},
		{ghcli.PR{ReviewDecision: "CHANGES_REQUESTED"}, "×"},
		{ghcli.PR{ReviewDecision: "REVIEW_REQUIRED"}, "•"},
		{ghcli.PR{}, "•"},
	}
	for _, c := range cases {
		if got := reviewIcon(c.pr); got != c.want {
			t.Errorf("reviewIcon(%+v) = %q, want %q", c.pr, got, c.want)
		}
	}
}
```

- [ ] **Step 3: テストが失敗することを確認する**

Run: `go test ./internal/ui/ -v`
Expected: FAIL（`undefined: New` などのコンパイルエラー）

- [ ] **Step 4: 実装を書く**

`internal/ui/ui.go`:

```go
// Package ui implements the GitHub Dash terminal UI.
package ui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

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
	prListMsg     []ghcli.PR
	issueListMsg  []ghcli.Issue
	prDetailMsg   ghcli.PR
	issueDetailMsg ghcli.Issue
	repoNameMsg   string
	errorMsg      struct{ err error }
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

// handleDetailKey は Task 4 で完成させる。ここでは戻る/終了のみ。
func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return m.enterList()
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
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
```

`internal/ui/render.go`:

```go
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/kukv/herdr-plugin-github-dash/internal/ghcli"
)

var (
	titleStyle     = lipgloss.NewStyle().Bold(true)
	activeTabStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	dimStyle       = lipgloss.NewStyle().Faint(true)
)

func (m Model) listView() string {
	var b strings.Builder
	title := "GitHub Dash"
	if m.repoName != "" {
		title += " — " + m.repoName
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	prTab, issueTab := "Pull Requests", "Issues"
	if m.tab == tabPRs {
		prTab = activeTabStyle.Render(prTab)
		issueTab = dimStyle.Render(issueTab)
	} else {
		issueTab = activeTabStyle.Render(issueTab)
		prTab = dimStyle.Render(prTab)
	}
	b.WriteString(prTab + "  " + issueTab + "\n\n")

	switch {
	case m.loading:
		b.WriteString(m.spin.View() + " loading...\n")
	case m.tab == tabPRs && len(m.prs) == 0:
		b.WriteString(dimStyle.Render("No open pull requests") + "\n")
	case m.tab == tabIssues && len(m.issues) == 0:
		b.WriteString(dimStyle.Render("No open issues") + "\n")
	case m.tab == tabPRs:
		now := time.Now()
		for i, pr := range m.prs {
			b.WriteString(cursorPrefix(i == m.cursors[tabPRs]) + prLine(pr, now) + "\n")
		}
	default:
		now := time.Now()
		for i, issue := range m.issues {
			b.WriteString(cursorPrefix(i == m.cursors[tabIssues]) + issueLine(issue, now) + "\n")
		}
	}

	b.WriteString("\n" + dimStyle.Render("j/k:move  enter:detail  tab:PR/Issue  r:refresh  o:browser  q:quit"))
	return b.String()
}

// detailView は Task 4 で完成させる。
func (m Model) detailView() string {
	if m.loading {
		return m.spin.View() + " loading...\n"
	}
	return m.detailTitle + "\n"
}

func errorView(text string) string {
	return titleStyle.Render("GitHub Dash — error") + "\n\n" + text + "\n\n" +
		dimStyle.Render("q:quit")
}

func cursorPrefix(selected bool) string {
	if selected {
		return "▸ "
	}
	return "  "
}

func prLine(pr ghcli.PR, now time.Time) string {
	return fmt.Sprintf("#%-5d %s  @%s  %s %s",
		pr.Number, pr.Title, pr.Author.Login, reviewIcon(pr), relTime(now, pr.UpdatedAt))
}

func issueLine(issue ghcli.Issue, now time.Time) string {
	return fmt.Sprintf("#%-5d %s  @%s  %s",
		issue.Number, issue.Title, issue.Author.Login, relTime(now, issue.UpdatedAt))
}

func reviewIcon(pr ghcli.PR) string {
	if pr.IsDraft {
		return "◌"
	}
	switch pr.ReviewDecision {
	case "APPROVED":
		return "✓"
	case "CHANGES_REQUESTED":
		return "×"
	default:
		return "•"
	}
}

func relTime(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
```

- [ ] **Step 5: テストが通ることを確認する**

Run: `go test ./internal/ui/ -v`
Expected: PASS（ui_test 8件 + render_test 2件）

- [ ] **Step 6: コミット**

```bash
gofmt -l . && go vet ./...
git add go.mod go.sum internal/ui/
git commit -m "feat: add list screen with PR/issue tabs

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 4: ui 詳細画面（glamour + viewport、直行モード対応）

**Files:**
- Modify: `internal/ui/ui.go`（`handleDetailKey` の完成、`prDetailMsg` / `issueDetailMsg` の処理追加）
- Modify: `internal/ui/render.go`（`detailView` の完成、Markdown 組み立て関数の追加）
- Test: `internal/ui/ui_test.go`（追記）
- Test: `internal/ui/render_test.go`（追記）

**Interfaces:**
- Consumes: Task 3 の Model / msg 型
- Produces: `prMarkdown(pr ghcli.PR) string` / `issueMarkdown(issue ghcli.Issue) string`（render.go 内部関数）。`New(src, initial)` の `initial != nil` 時の詳細直行が完全動作する

- [ ] **Step 1: 失敗するテストを書く（ui_test.go に追記）**

```go
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
	if !m.loading || cmd == nil {
		t.Errorf("loading = %v, cmd = %v; want loading with fetch cmd", m.loading, cmd)
	}
}
```

`render_test.go` に追記:

```go
func TestPRMarkdownContainsMetaBodyAndComments(t *testing.T) {
	pr := ghcli.PR{
		Number: 12, Title: "feat: pane", Author: ghcli.Author{Login: "kukv"},
		State: "OPEN", IsDraft: true, ReviewDecision: "REVIEW_REQUIRED",
		Labels: []ghcli.Label{{Name: "Kind: Feature"}},
		Body:   "body text",
		Comments: []ghcli.Comment{
			{Author: ghcli.Author{Login: "bob"}, Body: "comment text",
				CreatedAt: time.Date(2026, 7, 11, 11, 0, 0, 0, time.UTC)},
		},
	}
	md := prMarkdown(pr)
	for _, want := range []string{"#12", "feat: pane", "@kukv", "OPEN (draft)",
		"REVIEW_REQUIRED", "Kind: Feature", "body text", "@bob", "comment text"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestIssueMarkdownEmptyBody(t *testing.T) {
	md := issueMarkdown(ghcli.Issue{Number: 3, Title: "an issue"})
	if !strings.Contains(md, "_no description_") {
		t.Errorf("markdown missing empty-body placeholder:\n%s", md)
	}
}
```

（render_test.go の import に `"strings"` を追加する）

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ui/ -v`
Expected: FAIL（`undefined: prMarkdown`、および detail view に本文が出ないことによる assert 失敗）

- [ ] **Step 3: 実装する**

`internal/ui/ui.go` の `Update` の switch に2ケースを追加（`errorMsg` ケースの前）:

```go
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
```

（`ui.go` の import に `"fmt"` と `"github.com/charmbracelet/glamour"` を追加する）

`handleDetailKey` を完成させる（既存の関数を置き換え）:

```go
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
```

`ui.go` に `setDetailContent` を追加:

```go
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
```

`internal/ui/render.go` の `detailView` を置き換え、Markdown 組み立てを追加:

```go
func (m Model) detailView() string {
	if m.loading {
		return m.spin.View() + " loading...\n"
	}
	header := titleStyle.Render(m.detailTitle)
	footer := dimStyle.Render("j/k:scroll  r:refresh  o:browser  esc:back")
	return header + "\n" + m.detail.View() + "\n" + footer
}

func prMarkdown(pr ghcli.PR) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# #%d %s\n\n", pr.Number, pr.Title)
	fmt.Fprintf(&b, "- **author**: @%s\n", pr.Author.Login)
	state := pr.State
	if pr.IsDraft {
		state += " (draft)"
	}
	fmt.Fprintf(&b, "- **state**: %s\n", state)
	if pr.ReviewDecision != "" {
		fmt.Fprintf(&b, "- **review**: %s\n", pr.ReviewDecision)
	}
	writeCommonMeta(&b, pr.Labels, pr.UpdatedAt)
	writeBody(&b, pr.Body)
	writeComments(&b, pr.Comments)
	return b.String()
}

func issueMarkdown(issue ghcli.Issue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# #%d %s\n\n", issue.Number, issue.Title)
	fmt.Fprintf(&b, "- **author**: @%s\n", issue.Author.Login)
	fmt.Fprintf(&b, "- **state**: %s\n", issue.State)
	writeCommonMeta(&b, issue.Labels, issue.UpdatedAt)
	writeBody(&b, issue.Body)
	writeComments(&b, issue.Comments)
	return b.String()
}

func writeCommonMeta(b *strings.Builder, labels []ghcli.Label, updatedAt time.Time) {
	if len(labels) > 0 {
		names := make([]string, len(labels))
		for i, l := range labels {
			names[i] = l.Name
		}
		fmt.Fprintf(b, "- **labels**: %s\n", strings.Join(names, ", "))
	}
	fmt.Fprintf(b, "- **updated**: %s\n", updatedAt.Format("2006-01-02 15:04"))
}

func writeBody(b *strings.Builder, body string) {
	b.WriteString("\n---\n\n")
	if body != "" {
		b.WriteString(body)
	} else {
		b.WriteString("_no description_")
	}
}

func writeComments(b *strings.Builder, comments []ghcli.Comment) {
	for _, c := range comments {
		fmt.Fprintf(b, "\n\n---\n\n**@%s** — %s\n\n%s",
			c.Author.Login, c.CreatedAt.Format("2006-01-02 15:04"), c.Body)
	}
}
```

- [ ] **Step 4: テストが通ることを確認する**

Run: `go test ./internal/ui/ -v`
Expected: PASS（全件。Task 3 の既存テストも壊れていないこと）

- [ ] **Step 5: コミット**

```bash
gofmt -l . && go vet ./...
git add internal/ui/
git commit -m "feat: add detail screen with glamour markdown rendering

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 5: main.go 配線 + リンクハンドラー URL パース

**Files:**
- Create: `main.go`
- Test: `main_test.go`

**Interfaces:**
- Consumes: `herdrctx.Resolve`（Task 1）、`ghcli.New`（Task 2）、`ui.New` / `ui.NewError` / `ui.Target`（Task 3, 4）
- Produces: `parseTarget(url string) *ui.Target`（main パッケージ内部）。環境変数 `HERDR_PLUGIN_CONTEXT_JSON` / `GITHUB_DASH_URL` を読む実行バイナリ

- [ ] **Step 1: 失敗するテストを書く**

`main_test.go`:

```go
package main

import (
	"testing"

	"github.com/kukv/herdr-plugin-github-dash/internal/ui"
)

func TestParseTarget(t *testing.T) {
	cases := []struct {
		url  string
		want *ui.Target
	}{
		{"https://github.com/octo/hello/pull/7",
			&ui.Target{Kind: ui.KindPR, Repo: "octo/hello", Number: 7}},
		{"https://github.com/octo/hello/issues/42/",
			&ui.Target{Kind: ui.KindIssue, Repo: "octo/hello", Number: 42}},
		{"https://github.com/octo/hello", nil},
		{"https://example.com/octo/hello/pull/7", nil},
		{"", nil},
	}
	for _, c := range cases {
		got := parseTarget(c.url)
		if c.want == nil {
			if got != nil {
				t.Errorf("parseTarget(%q) = %+v, want nil", c.url, got)
			}
			continue
		}
		if got == nil || *got != *c.want {
			t.Errorf("parseTarget(%q) = %+v, want %+v", c.url, got, c.want)
		}
	}
}
```

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test . -v`
Expected: FAIL（`undefined: parseTarget` のコンパイルエラー）

- [ ] **Step 3: 実装する**

`main.go`:

```go
// Command github-dash is the GitHub Dash pane process for Herdr.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kukv/herdr-plugin-github-dash/internal/ghcli"
	"github.com/kukv/herdr-plugin-github-dash/internal/herdrctx"
	"github.com/kukv/herdr-plugin-github-dash/internal/ui"
)

var urlPattern = regexp.MustCompile(
	`^https://github\.com/([^/]+)/([^/]+)/(issues|pull)/([0-9]+)/?$`)

// parseTarget converts a clicked GitHub URL into a detail-view target.
func parseTarget(url string) *ui.Target {
	m := urlPattern.FindStringSubmatch(url)
	if m == nil {
		return nil
	}
	number, err := strconv.Atoi(m[4])
	if err != nil {
		return nil
	}
	kind := ui.KindIssue
	if m[3] == "pull" {
		kind = ui.KindPR
	}
	return &ui.Target{Kind: kind, Repo: m[1] + "/" + m[2], Number: number}
}

func main() {
	var model tea.Model
	dir, err := herdrctx.Resolve(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON"))
	if err != nil {
		model = ui.NewError(fmt.Sprintf(
			"could not resolve the target directory: %v\n\nRun GitHub Dash from a Herdr workspace.", err))
	} else {
		model = ui.New(ghcli.New(dir), parseTarget(os.Getenv("GITHUB_DASH_URL")))
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: テストが通り、バイナリがビルドできることを確認する**

Run: `go test ./... && go build -o bin/github-dash .`
Expected: 全パッケージ PASS、`bin/github-dash` が生成される

- [ ] **Step 5: コミット**

```bash
gofmt -l . && go vet ./...
git add main.go main_test.go
git commit -m "feat: wire up main with context resolution and URL direct mode

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 6: マニフェスト + open.sh + README + 実機 E2E

**Files:**
- Create: `herdr-plugin.toml`
- Create: `open.sh`
- Modify: `.gitignore`（`bin/` を追記）
- Modify: `README.md`（使い方を追記）

**Interfaces:**
- Consumes: `bin/github-dash`（Task 5 のビルド成果物）
- Produces: `herdr plugin install kukv/herdr-plugin-github-dash` でインストール可能なプラグイン一式

- [ ] **Step 1: マニフェストと open.sh を書く**

`herdr-plugin.toml`:

```toml
id = "kukv.github-dash"
name = "GitHub Dash"
version = "0.1.0"
min_herdr_version = "0.7.0"
description = "Browse GitHub pull requests and issues for the current workspace."
platforms = ["linux", "macos"]

[[build]]
command = ["go", "build", "-o", "bin/github-dash", "."]

[[actions]]
id = "open"
title = "Open GitHub dashboard"
contexts = ["workspace"]
command = ["bash", "open.sh"]

[[panes]]
id = "dash"
title = "GitHub Dash"
placement = "overlay"
command = ["bin/github-dash"]

[[link_handlers]]
id = "github-pr-issue"
title = "Open in GitHub Dash"
pattern = "^https://github\\.com/[^/]+/[^/]+/(issues|pull)/[0-9]+/?$"
action = "open"
```

`open.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

herdr_bin="${HERDR_BIN_PATH:-herdr}"
url="${HERDR_PLUGIN_CLICKED_URL:-}"

args=(plugin pane open --plugin kukv.github-dash --entrypoint dash --focus)
if [[ -n "$url" ]]; then
  args+=(--env "GITHUB_DASH_URL=$url")
fi
exec "$herdr_bin" "${args[@]}"
```

```bash
chmod +x open.sh
grep -qx 'bin/' .gitignore || echo 'bin/' >> .gitignore
```

（注: `.gitignore` は fanout 配布ファイルのため、将来の配布で上書きされる可能性がある。上書きされた場合は再追記でよい）

- [ ] **Step 2: 実機 E2E — リンクして開く**

```bash
go build -o bin/github-dash .
herdr plugin link "$PWD"
herdr pane list | grep -o '"pane_id":"[^"]*"' | sort > /tmp/panes-before.txt
herdr plugin action invoke open --plugin kukv.github-dash
sleep 2
herdr pane list | grep -o '"pane_id":"[^"]*"' | sort > /tmp/panes-after.txt
comm -13 /tmp/panes-before.txt /tmp/panes-after.txt
```

Expected: `plugin_linked` → `plugin_action_invoked` の JSON が出て、`comm` が新しいペイン ID を1つ表示する（例: `"pane_id":"w4:p3"`）

- [ ] **Step 3: 実機 E2E — ペイン内容を読んで検証する**

新しいペイン ID を `<PANE>` として:

```bash
herdr pane read <PANE> --lines 40
```

Expected: `GitHub Dash — kukv/herdr-plugin-github-dash` ヘッダー、`Pull Requests  Issues` タブ行、PR 一覧（open PR がなければ `No open pull requests`）、フッターのキーヘルプが見える

```bash
herdr pane send-keys <PANE> tab
sleep 1
herdr pane read <PANE> --lines 40
```

Expected: Issues タブがアクティブになり、Issue 一覧（または `No open issues`）が見える

```bash
herdr pane send-keys <PANE> q
sleep 1
herdr pane list | grep -c '<PANE>' || echo closed
```

Expected: `closed`（q でペインが閉じる）

失敗した場合は `herdr plugin log list --plugin kukv.github-dash` でプラグインログを確認する。

- [ ] **Step 4: README に使い方を追記する**

`README.md` の既存1行の説明の下に追記:

```markdown
## Requirements

- [herdr](https://herdr.dev) >= 0.7.0
- [GitHub CLI](https://cli.github.com/) (`gh`), authenticated via `gh auth login`
- Go toolchain (used once at install time to build the pane binary)

## Install

    herdr plugin install kukv/herdr-plugin-github-dash

## Usage

- Run the **Open GitHub dashboard** action from a workspace to open the
  dashboard as an overlay pane for that workspace's repository.
- Ctrl+click a GitHub PR/issue URL in any terminal pane to open its detail
  view directly.

### Keys

| Key | List | Detail |
|---|---|---|
| `j` / `k` | move cursor | scroll |
| `enter` | open detail | — |
| `tab` | switch PRs / Issues | — |
| `r` | refresh | refresh |
| `o` | open in browser | open in browser |
| `esc` | — | back to list |
| `q` | quit | back to list |

## Development

    go test ./...
    go build -o bin/github-dash .
    herdr plugin link "$PWD"
```

- [ ] **Step 5: コミット**

```bash
git add herdr-plugin.toml open.sh .gitignore README.md
git commit -m "feat: add plugin manifest, action shim, and usage docs

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

- [ ] **Step 6: ユーザーによる手動確認を依頼する（チェックポイント）**

自動化できない2点をユーザーに依頼する:
1. herdr の UI からアクション **Open GitHub dashboard** を実行し、オーバーレイの見た目・キー操作を確認
2. ターミナルに表示された GitHub PR/Issue URL を **Ctrl+クリック**し、詳細直行モードが動くことを確認（リンクハンドラー経由のコンテキスト内容はここで初めて実機検証される。動かない場合は `herdr plugin log list --plugin kukv.github-dash` を確認）

---

### Task 7: 仕上げ — 全体検証と PR 作成

**Files:**
- なし（検証とリリース作業のみ）

**Interfaces:**
- Consumes: Task 1〜6 のすべての成果物
- Produces: レビュー可能な PR

- [ ] **Step 1: 全体検証**

```bash
go test ./... && gofmt -l . && go vet ./...
```

Expected: 全テスト PASS、gofmt 出力なし、vet エラーなし

- [ ] **Step 2: push して PR を作成する**

```bash
git push -u origin feat/github-dash-phase1
gh pr create --title "feat: GitHub Dash Phase 1 — PR/Issue list and detail panes" --body "$(cat <<'EOF'
## Summary
- herdr ペイン内でワークスペース連動リポジトリの GitHub PR / Issue を一覧・詳細表示する TUI プラグイン（Phase 1）
- Go + bubbletea、データ取得は gh CLI サブプロセス
- リンクハンドラー: GitHub PR/Issue URL の Ctrl+クリックで詳細直行
- 設計: docs/superpowers/specs/2026-07-11-github-dash-design.md

## Test plan
- [x] go test ./...（herdrctx / ghcli / ui / main）
- [x] herdr plugin link での実機 E2E（pane read / send-keys による確認）
- [ ] リンクハンドラーの実クリック確認（手動）

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Expected: PR の URL が表示される。main は保護されている（code owner review 1件 + 署名必須）ため、マージはレビュー後にユーザーが行う

- [ ] **Step 3: 後続作業のメモを残す**

PR 作成後、以下をユーザーに伝える:
- `herdr-plugin` トピックの付与はプラグインが動くようになってから `kukv/structure` 側の PR で行う（スペックのスコープ外メモ参照）
- Phase 2（コメント投稿・メタ変更）は別スペック・別計画で行う
