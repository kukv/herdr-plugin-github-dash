# GitHub Dash Phase 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 詳細画面から、表示中の PR / Issue を状態連動キー `x` ＋ `y/n` 確認で close / reopen できるようにする。

**Architecture:** `ghcli` に種別×アクション別の状態変更メソッド（`ClosePR`/`ReopenPR`/`CloseIssue`/`ReopenIssue`）を追加し、`ui` の詳細画面に `confirming` サブ状態を足す。`x` で状態に応じた確認プロンプトを開き `y` で実行、`n`/`Esc` でキャンセル。実行成功で詳細を再取得し状態表示を更新、失敗は全画面エラーに落とさず詳細に留めて `actionErr` を表示する。既存の種別別メソッド設計に倣うため `main.go`・`go.mod` は無変更。

**Tech Stack:** Go、charmbracelet/bubbletea・bubbles（spinner/viewport）・lipgloss・glamour、gh CLI 2.95、標準 `testing`

**Spec:** `docs/superpowers/specs/2026-07-12-github-dash-phase3-design.md`（承認済み）

## Global Constraints

- Go モジュールパス: `github.com/kukv/herdr-plugin-github-dash`
- 外部依存は charmbracelet の4つ（bubbletea / bubbles / lipgloss / glamour）のみ。**go.mod への新規依存追加は禁止**。本 Phase は追加サブパッケージも不要
- テストは標準 `testing` のみ
- `gh` は必ず `exec.Cmd.Dir` に対象ディレクトリを設定して実行（既存 `runGh` がこれを行うので新規メソッドは既存 `c.run` を通す）。`--repo` は override 時のみ付ける（既存 `appendRepo` を使う）
- スコープは close / reopen のみ。merge・comment-and-close・ラベル・アサイン・一覧画面からの状態変更は入れない
- コミットは conventional commits（`feat:` / `docs:`）。SSH 署名はリポジトリローカルに設定済み。メッセージ末尾に `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>` を付ける
- 各タスク後に `gofmt -l .`（出力なし）と `go vet ./...`（エラーなし）を確認してからコミット

## File Structure

```
internal/ghcli/ghcli.go        # ClosePR/ReopenPR/CloseIssue/ReopenIssue を追加（Task 1）
internal/ghcli/ghcli_test.go   # 状態変更の args 検証テストを追加（Task 1）
internal/ui/ui.go              # DataSource 拡張・Model 拡張・detailState 設定・confirm キー処理・setState・stateAction・Update ケース（Task 2）
internal/ui/ui_test.go         # fakeSource 拡張・confirm 挙動テスト（Task 2, 3）
internal/ui/render.go          # confirmView 追加・detailView の confirm 分岐と状態連動フッター・actionErr インライン（Task 3）
README.md                      # x キーの追記（Task 3）
```

Task 1（ghcli）は Task 2/3（ui）から独立してレビュー・テストできる。Task 2 は confirm の状態遷移（View に依存しない Model フィールド・呼び出し記録のみ検証）、Task 3 は描画（confirmView・フッター・actionErr）と分け、それぞれ独立して受け入れ可能にする。

---

### Task 1: ghcli に状態変更メソッドを追加

**Files:**
- Modify: `internal/ghcli/ghcli.go`（末尾に4メソッド追加）
- Test: `internal/ghcli/ghcli_test.go`（テスト3件追加）

**Interfaces:**
- Consumes: 既存の `Client{dir, run}`、`appendRepo(args []string, repo string) []string`、`newTestClient(out string, err error) (*Client, *fakeRun)`、`fakeRun{args []string, dir string}`（すべて既存）
- Produces（Task 2 が使う）:
  - `(*Client) ClosePR(repo string, number int) error`
  - `(*Client) ReopenPR(repo string, number int) error`
  - `(*Client) CloseIssue(repo string, number int) error`
  - `(*Client) ReopenIssue(repo string, number int) error`
  - 引数 `repo` は `"owner/repo"`。空文字列なら `--repo` を付けない

- [ ] **Step 1: 失敗するテストを書く（`internal/ghcli/ghcli_test.go` の末尾に追記）**

```go
func TestClosePR(t *testing.T) {
	c, f := newTestClient("", nil)
	if err := c.ClosePR("", 12); err != nil {
		t.Fatalf("ClosePR: %v", err)
	}
	wantArgs := []string{"pr", "close", "12"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if f.dir != "/repo" {
		t.Errorf("dir = %q, want /repo", f.dir)
	}
}

func TestReopenIssueWithRepoOverride(t *testing.T) {
	c, f := newTestClient("", nil)
	if err := c.ReopenIssue("octo/hello", 3); err != nil {
		t.Fatalf("ReopenIssue: %v", err)
	}
	wantArgs := []string{"issue", "reopen", "3", "--repo", "octo/hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestStateChangeError(t *testing.T) {
	wantErr := errors.New("gh pr: HTTP 403 forbidden")
	c, _ := newTestClient("", wantErr)
	if err := c.ClosePR("", 12); !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}
```

（`ghcli_test.go` は既に `errors` / `reflect` / `testing` を import 済み。追加 import は不要）

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ghcli/ -run TestClosePR -v`
Expected: FAIL（`c.ClosePR undefined` のコンパイルエラー）

- [ ] **Step 3: 実装を書く（`internal/ghcli/ghcli.go` の末尾、`AddIssueComment` の後に追加）**

```go
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
```

（`strconv` は既に import 済み。追加 import は不要）

- [ ] **Step 4: テストが通ることを確認する**

Run: `go test ./internal/ghcli/ -v`
Expected: PASS（既存 + 新規3件すべて）

- [ ] **Step 5: gofmt / vet を確認してコミット**

```bash
gofmt -l . && go vet ./...   # gofmt は出力なし、vet はエラーなし
git add internal/ghcli/
git commit -m "feat: add close/reopen to ghcli

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: ui の confirm 状態遷移（描画は Task 3）

詳細画面の `confirming` サブ状態・状態変更 cmd・成功/失敗処理を追加する。このタスクのテストは
`m.View()` の中身には依存せず、Model フィールド（`confirming` / `working` / `actionErr` /
`detailLoading` / `detailState`）と `fakeSource` の呼び出し記録のみを検証する。描画は Task 3 で追加する。

**Files:**
- Modify: `internal/ui/ui.go`（DataSource 拡張・Model 拡張・msg 型・detailState 設定・enter リセット・Update ケース・stateAction・setState・confirm キー処理）
- Test: `internal/ui/ui_test.go`（fakeSource 拡張・テスト追加）

**Interfaces:**
- Consumes: 既存の `Model`、`Target{Kind, Repo, Number}`、`KindPR`/`KindIssue`、`fetchDetail`、`handleDetailKey`、`detailModel(f *fakeSource) Model`、`key(s)`、`itoa(n)`（すべて既存）。Task 1 の `ClosePR`/`ReopenPR`/`CloseIssue`/`ReopenIssue`
- Produces（Task 3 が使う）:
  - Model フィールド: `detailState string`、`confirming bool`、`working bool`、`actionErr string`
  - `(m Model) stateAction() (closing bool, ok bool)`（OPEN→`true,true` / CLOSED→`false,true` / それ以外→`false,false`）
  - `m.detailTitle`・`m.detailTarget`（既存。confirm ヘッダー・noun 判定で再利用）

- [ ] **Step 1: 失敗するテストを書く（`internal/ui/ui_test.go`）**

まず `fakeSource` に状態変更の記録を追加する。既存の struct 定義（`commentErr error` の行の下）に2フィールド足し、メソッドを4つ追加する:

```go
// fakeSource の struct に追記（commentErr の下）
	stateCalls []string // "close:pr:<repo>:<n>" 等の action:kind:repo:number
	stateErr   error

// fakeSource のメソッド群の末尾（AddIssueComment の後）に追記
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
```

テスト本体を追加する（`detailModel` は Phase 2 で追加済み。PR の `State` フィールドで状態を与える）:

```go
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
	m = next.(Model)
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
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		stateErr: errors.New("gh pr: HTTP 403 forbidden")}
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
```

（`ui_test.go` は既に `errors` / `strings` / `testing` / `time` / `tea` / `ghcli` を import 済み。`key` ヘルパーは `y`/`n`/`x` を default（KeyRunes）で扱うため switch 追加は不要。追加 import は不要）

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ui/ -run 'TestXEnters|TestConfirm|TestStateError|TestXIgnored' -v`
Expected: FAIL（`m.confirming undefined`、`fakeSource` が `DataSource` を満たさない等のコンパイルエラー）

- [ ] **Step 3: DataSource インターフェースを拡張（`internal/ui/ui.go`）**

`DataSource` の `AddIssueComment` の行の後に追加:

```go
	ClosePR(repo string, number int) error
	ReopenPR(repo string, number int) error
	CloseIssue(repo string, number int) error
	ReopenIssue(repo string, number int) error
```

- [ ] **Step 4: msg 型と Model フィールドを追加（`internal/ui/ui.go`）**

msg 型（`commentErrorMsg struct{ err error }` のある `type (...)` ブロック）に追加:

```go
	stateChangedMsg struct{}
	stateErrorMsg   struct{ err error }
```

`Model` struct の `postErr string` の下にフィールドを追加:

```go
	detailState string
	confirming  bool
	working     bool
	actionErr   string
```

- [ ] **Step 5: detailState を設定し、enter でリセット（`internal/ui/ui.go`）**

`Update` の `case prDetailMsg:` に `m.detailState = msg.State` を追加する（`m.detailLoading = false` の後）:

```go
	case prDetailMsg:
		m.detailLoading = false
		m.detailState = msg.State
		m.detailTitle = fmt.Sprintf("PR #%d %s", msg.Number, msg.Title)
		m.setDetailContent(prMarkdown(ghcli.PR(msg)))
		return m, nil
```

同様に `case issueDetailMsg:` にも追加:

```go
	case issueDetailMsg:
		m.detailLoading = false
		m.detailState = msg.State
		m.detailTitle = fmt.Sprintf("Issue #%d %s", msg.Number, msg.Title)
		m.setDetailContent(issueMarkdown(ghcli.Issue(msg)))
		return m, nil
```

`handleListKey` の `case "enter":` で、新しい詳細に入るとき前回の confirm 状態を持ち越さないようリセットする（`m.detailTitle = ""` の後に3行追加）:

```go
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
```

- [ ] **Step 6: Update に状態変更結果ケースを追加（`internal/ui/ui.go`、`case commentErrorMsg:` の後、`case errorMsg:` の前）**

```go
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
```

- [ ] **Step 7: stateAction と setState を追加（`internal/ui/ui.go`、`postComment` の後）**

```go
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
```

- [ ] **Step 8: confirm キー処理を追加し、handleDetailKey から分岐（`internal/ui/ui.go`）**

`handleDetailKey` の先頭 `composing` 分岐の後に `confirming` 分岐を足し、`x` ケースを追加する（既存関数を置き換え）:

```go
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
```

- [ ] **Step 9: テストが通ることを確認する**

Run: `go test ./internal/ui/ -run 'TestXEnters|TestConfirm|TestStateError|TestXIgnored' -v`
Expected: PASS（新規7件すべて）。続けて `go test ./internal/ui/ -v` で既存テストも壊れていないことを確認（PASS）

- [ ] **Step 10: gofmt / vet を確認してコミット**

```bash
gofmt -l . && go vet ./...
git add internal/ui/ui.go internal/ui/ui_test.go
git commit -m "feat: add close/reopen confirm state machine to detail screen

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: confirm の描画とドキュメント

confirm プロンプトを表示する `confirmView` を追加し、詳細フッターに状態連動の `x:close`/`x:reopen`
を、失敗時に `actionErr` をインライン表示する。README に `x` キーを追記する。

**Files:**
- Modify: `internal/ui/render.go`（`confirmView` 追加、`detailView` の confirm 分岐・状態連動フッター・actionErr、`stateFooterKey` 追加）
- Test: `internal/ui/ui_test.go`（描画アサーションを4件追加）
- Modify: `README.md`（Keys 表に `x` を追記）

**Interfaces:**
- Consumes: Task 2 の `m.confirming`、`m.working`、`m.actionErr`、`m.detailState`、`m.stateAction()`、`m.detailTitle`、`m.detailTarget`、既存 `titleStyle`/`dimStyle`/`m.spin`、既存 import（`fmt`/`strings`）

- [ ] **Step 1: 失敗するテストを書く（`internal/ui/ui_test.go` の末尾に追記）**

```go
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
	f := &fakeSource{prs: samplePRs(), pr: ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		stateErr: errors.New("gh pr: HTTP 403 forbidden")}
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
```

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ui/ -run 'TestConfirmView|TestDetailFooterShowsStateKey|TestStateErrorShownInline' -v`
Expected: FAIL（confirm 中でも通常の詳細ビューが描画され、"Close"/"(y/n)"/"x:close" 等が出ないため assert 失敗）

- [ ] **Step 3: detailView に confirm 分岐・状態連動フッター・actionErr を追加し confirmView / stateFooterKey を実装（`internal/ui/render.go`）**

`detailView` を置き換える（composing の後に confirming 分岐、フッターに状態連動キー、本文末に actionErr）:

```go
func (m Model) detailView() string {
	if m.composing {
		return m.composeView()
	}
	if m.confirming {
		return m.confirmView()
	}
	if m.detailLoading {
		return m.spin.View() + " loading...\n"
	}
	header := titleStyle.Render(m.detailTitle)
	footer := dimStyle.Render("j/k:scroll  r:refresh  o:browser  c:comment  " + m.stateFooterKey() + "esc:back")
	body := header + "\n" + m.detail.View() + "\n"
	if m.actionErr != "" {
		body += "error: " + m.actionErr + "\n"
	}
	return body + footer
}

// stateFooterKey returns the state-aware footer hint (with trailing spaces),
// or "" when the item cannot change state (merged / not yet loaded).
func (m Model) stateFooterKey() string {
	closing, ok := m.stateAction()
	if !ok {
		return ""
	}
	if closing {
		return "x:close  "
	}
	return "x:reopen  "
}

func (m Model) confirmView() string {
	header := titleStyle.Render(m.detailTitle)
	closing, _ := m.stateAction()
	verb := "Reopen"
	if closing {
		verb = "Close"
	}
	noun := "issue"
	if m.detailTarget.Kind == KindPR {
		noun = "PR"
	}
	var b strings.Builder
	b.WriteString(header + "\n\n")
	b.WriteString(fmt.Sprintf("%s this %s? ", verb, noun))
	if m.working {
		b.WriteString(m.spin.View() + " working...\n")
	} else {
		b.WriteString(dimStyle.Render("(y/n)"))
	}
	return b.String()
}
```

（`render.go` は既に `fmt` / `strings` を import 済み。追加 import は不要）

- [ ] **Step 4: テストが通ることを確認する**

Run: `go test ./internal/ui/ -v`
Expected: PASS（Task 2 の7件 + 本タスクの4件 + 既存すべて）

- [ ] **Step 5: README の Keys 表に x 行を追記（`README.md`）**

`README.md` の `### Keys` 表（`| Key | List | Detail |`）の `c` の行の直後に、`x`（詳細のみ）の行を1行だけ追加する。`x` は詳細画面専用なので List 列は `—`:

```
| `c` | — | comment (`Ctrl+S` send / `Esc` cancel) |
| `x` | — | close / reopen (`y` confirm / `n` cancel) |
| `esc` | — | back to list |
```

（追加するのは `x` の行のみ。表の他の行や無関係な箇所は編集しない）

- [ ] **Step 6: gofmt / vet / 全テストを確認してコミット**

```bash
gofmt -l . && go vet ./... && go test ./...
git add internal/ui/render.go internal/ui/ui_test.go README.md
git commit -m "feat: render close/reopen confirm view and document x key

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## 実装後の手動 E2E（自動テスト対象外・スペック「実機検証で確認する残項目」）

- `go build -o bin/github-dash .` → `herdr plugin link` で実機に載せ、詳細画面で `x` → `y` の close/reopen が動くこと
- 再取得後にフッターの状態表示（`x:close` ↔ `x:reopen`）が反転すること
- `x` / `y` / `n` が herdr/ターミナルのキーグラブに奪われて届かない場合は、キーを変更する（その場合 Task 2 Step 8 の該当キーと Task 3 の文言、README を合わせて更新）
