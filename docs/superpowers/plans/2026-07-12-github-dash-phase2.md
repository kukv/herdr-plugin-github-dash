# GitHub Dash Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 詳細画面から、表示中の PR / Issue へインライン textarea でコメントを投稿できるようにする。

**Architecture:** `ghcli` に PR/Issue 別のコメント投稿メソッドを追加し、`ui` の詳細画面に `composing` サブ状態を足す。`c` で textarea を開き `Ctrl+S` で送信、`Esc` でキャンセル。投稿成功で詳細を再取得、失敗は全画面エラーに落とさず compose を維持して下書きを温存する。既存の種別別メソッド設計（`GetPR`/`GetIssue`）に倣うため `main.go`・`go.mod` は無変更。

**Tech Stack:** Go、charmbracelet/bubbletea・bubbles（textarea/spinner/viewport）・lipgloss・glamour、gh CLI 2.95、標準 `testing`

**Spec:** `docs/superpowers/specs/2026-07-12-github-dash-phase2-design.md`（承認済み）

## Global Constraints

- Go モジュールパス: `github.com/kukv/herdr-plugin-github-dash`
- 外部依存は charmbracelet の4つ（bubbletea / bubbles / lipgloss / glamour）のみ。**go.mod への新規依存追加は禁止**。textarea は既存 bubbles モジュール内のサブパッケージ（`github.com/charmbracelet/bubbles/textarea`）
- テストは標準 `testing` のみ
- `gh` は必ず `exec.Cmd.Dir` に対象ディレクトリを設定して実行（既存 `runGh` がこれを行うので新規メソッドは既存 `c.run` を通す）。`--repo` は override 時のみ付ける（既存 `appendRepo` を使う）
- スコープはコメント投稿のみ。状態変更・ラベル・アサイン・一覧画面からの投稿・コメント編集削除は入れない
- コミットは conventional commits（`feat:` / `docs:`）。SSH 署名はリポジトリローカルに設定済み。メッセージ末尾に `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>` を付ける
- 各タスク後に `gofmt -l .`（出力なし）と `go vet ./...`（エラーなし）を確認してからコミット

## File Structure

```
internal/ghcli/ghcli.go        # AddPRComment / AddIssueComment を追加（Task 1）
internal/ghcli/ghcli_test.go   # コメント投稿の args 検証テストを追加（Task 1）
internal/ui/ui.go              # DataSource 拡張・Model 拡張・compose キー処理・postComment・Update ケース（Task 2）
internal/ui/ui_test.go         # fakeSource 拡張・compose 挙動テスト（Task 2, 3）
internal/ui/render.go          # composeView 追加・詳細フッターに c:comment（Task 3）
README.md                      # c キーの追記（Task 3）
```

Task 1（ghcli）は Task 2/3（ui）から独立してレビュー・テストできる。Task 2 は compose の状態遷移（View に依存しない Model フィールド・呼び出し記録のみ検証）、Task 3 は描画（textarea 表示・フッター）と分け、それぞれ独立して受け入れ可能にする。

---

### Task 1: ghcli にコメント投稿メソッドを追加

**Files:**
- Modify: `internal/ghcli/ghcli.go`（末尾に2メソッド追加）
- Test: `internal/ghcli/ghcli_test.go`（テスト3件追加）

**Interfaces:**
- Consumes: 既存の `Client{dir, run}`、`appendRepo(args []string, repo string) []string`、`newTestClient(out string, err error) (*Client, *fakeRun)`、`fakeRun{args []string}`（すべて既存）
- Produces（Task 2 が使う）:
  - `(*Client) AddPRComment(repo string, number int, body string) error`
  - `(*Client) AddIssueComment(repo string, number int, body string) error`
  - 引数 `repo` は `"owner/repo"`。空文字列なら `--repo` を付けない

- [ ] **Step 1: 失敗するテストを書く（`internal/ghcli/ghcli_test.go` の末尾に追記）**

```go
func TestAddPRComment(t *testing.T) {
	c, f := newTestClient("https://github.com/kukv/demo/pull/12#issuecomment-1\n", nil)
	if err := c.AddPRComment("", 12, "hello"); err != nil {
		t.Fatalf("AddPRComment: %v", err)
	}
	wantArgs := []string{"pr", "comment", "12", "--body", "hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if f.dir != "/repo" {
		t.Errorf("dir = %q, want /repo", f.dir)
	}
}

func TestAddIssueCommentWithRepoOverride(t *testing.T) {
	c, f := newTestClient("", nil)
	if err := c.AddIssueComment("octo/hello", 3, "hi there"); err != nil {
		t.Fatalf("AddIssueComment: %v", err)
	}
	wantArgs := []string{"issue", "comment", "3", "--body", "hi there", "--repo", "octo/hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestAddCommentError(t *testing.T) {
	wantErr := errors.New("gh pr: HTTP 403 forbidden")
	c, _ := newTestClient("", wantErr)
	if err := c.AddPRComment("", 12, "x"); !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}
```

（`ghcli_test.go` は既に `errors` / `reflect` / `testing` を import 済み。追加 import は不要）

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ghcli/ -run TestAddPRComment -v`
Expected: FAIL（`c.AddPRComment undefined` のコンパイルエラー）

- [ ] **Step 3: 実装を書く（`internal/ghcli/ghcli.go` の末尾、`OpenIssueWeb` の後に追加）**

```go
func (c *Client) AddPRComment(repo string, number int, body string) error {
	_, err := c.run(c.dir, appendRepo([]string{"pr", "comment", strconv.Itoa(number), "--body", body}, repo)...)
	return err
}

func (c *Client) AddIssueComment(repo string, number int, body string) error {
	_, err := c.run(c.dir, appendRepo([]string{"issue", "comment", strconv.Itoa(number), "--body", body}, repo)...)
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
git commit -m "feat: add comment posting to ghcli

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: ui の compose 状態遷移（描画は Task 3）

詳細画面の `composing` サブ状態・投稿 cmd・成功/失敗処理を追加する。このタスクのテストは
`m.View()` の中身には依存せず、Model フィールド（`composing` / `posting` / `postErr` /
`detailLoading`）と `fakeSource` の呼び出し記録のみを検証する。描画は Task 3 で追加する。

**Files:**
- Modify: `internal/ui/ui.go`（DataSource 拡張・Model 拡張・New 初期化・WindowSizeMsg・msg 型・Update ケース・キー処理・postComment）
- Test: `internal/ui/ui_test.go`（fakeSource 拡張・key ヘルパー拡張・テスト追加）

**Interfaces:**
- Consumes: 既存の `Model`、`Target{Kind, Repo, Number}`、`KindPR`/`KindIssue`、`fetchDetail`、`handleDetailKey`、`New(src, initial)`、`loadedModel(f)`、`key(s)`（すべて既存）。Task 1 の `AddPRComment`/`AddIssueComment`
- Produces（Task 3 が使う）:
  - Model フィールド: `textarea textarea.Model`、`composing bool`、`posting bool`、`postErr string`
  - `m.detailTitle`（既存。compose ヘッダーで再利用）
  - テストヘルパー `detailModel(f *fakeSource) Model`（詳細ロード済みの Model を返す）

- [ ] **Step 1: 失敗するテストを書く（`internal/ui/ui_test.go`）**

まず `fakeSource` にコメント投稿の記録を追加する。既存の struct 定義（`webCalls []string` の行）に2フィールド足し、メソッドを2つ追加する:

```go
// fakeSource の struct に追記（webCalls の下に）
	commentCalls []string // "pr:<repo>:<n>:<body>" / "issue:<repo>:<n>:<body>"
	commentErr   error

// fakeSource のメソッド群の末尾に追記
func (f *fakeSource) AddPRComment(repo string, n int, body string) error {
	f.commentCalls = append(f.commentCalls, "pr:"+repo+":"+itoa(n)+":"+body)
	return f.commentErr
}
func (f *fakeSource) AddIssueComment(repo string, n int, body string) error {
	f.commentCalls = append(f.commentCalls, "issue:"+repo+":"+itoa(n)+":"+body)
	return f.commentErr
}
```

`key` ヘルパーの switch に `ctrl+s` を追加する（`case "esc":` の後）:

```go
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
```

詳細ロード済み Model を作るヘルパーと、テスト本体を追加する:

```go
// detailModel returns a Model already on the loaded detail screen for PR #1.
func detailModel(f *fakeSource) Model {
	m := loadedModel(f)
	next, cmd := m.Update(key("enter"))
	m = next.(Model)
	next, _ = m.Update(cmd()) // fetchDetail の結果を流し込む
	return next.(Model)
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
```

（`ui_test.go` は既に `errors` / `strings` / `testing` / `time` / `tea` / `ghcli` を import 済み。追加 import は不要）

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ui/ -run TestCompose -v`
Expected: FAIL（`m.composing undefined`、`fakeSource` が `DataSource` を満たさない等のコンパイルエラー）

- [ ] **Step 3: DataSource インターフェースを拡張（`internal/ui/ui.go`）**

`DataSource` の `OpenIssueWeb` の行の後に追加:

```go
	AddPRComment(repo string, number int, body string) error
	AddIssueComment(repo string, number int, body string) error
```

- [ ] **Step 4: import と Model フィールドと msg 型を追加（`internal/ui/ui.go`）**

import ブロックに2行追加する（`"fmt"` の下、bubbles 群の中）:

```go
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
```

（`strings` は標準ライブラリ側、`textarea` は charmbracelet 側のグループに置く。gofmt が並べ替える）

msg 型（`errorMsg struct{ err error }` のある `type (...)` ブロック）に追加:

```go
	commentPostedMsg struct{}
	commentErrorMsg  struct{ err error }
```

`Model` struct の `spin` / `errText` の近くにフィールドを追加:

```go
	textarea  textarea.Model
	composing bool
	posting   bool
	postErr   string
```

- [ ] **Step 5: New で textarea を初期化（`internal/ui/ui.go` の `New`）**

`New` の `s := spinner.New()` の後、`m := Model{...}` の前に追加:

```go
	ta := textarea.New()
	ta.Placeholder = "Leave a comment..."
	ta.ShowLineNumbers = false
```

`m := Model{...}` の複合リテラルに `textarea: ta,` を追加する（`detail: viewport.New(80, 20),` の後）:

```go
	m := Model{
		src:      src,
		spin:     s,
		screen:   screenList,
		detail:   viewport.New(80, 20),
		textarea: ta,
	}
```

- [ ] **Step 6: WindowSizeMsg で textarea をサイズ調整（`internal/ui/ui.go` の `Update` の `tea.WindowSizeMsg` ケース）**

既存の `m.detail.Height = max(msg.Height-4, 5)` の後に追加:

```go
		m.textarea.SetWidth(msg.Width)
		m.textarea.SetHeight(max(msg.Height-6, 3))
```

- [ ] **Step 7: Update に投稿結果ケースを追加（`internal/ui/ui.go` の `Update`、`case errorMsg:` の前）**

```go
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
```

- [ ] **Step 8: postComment cmd を追加（`internal/ui/ui.go`、`openWeb` の後）**

```go
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
```

- [ ] **Step 9: compose キー処理を追加し、handleDetailKey から分岐（`internal/ui/ui.go`）**

既存の `handleDetailKey` の先頭に compose 分岐を足し、`c` ケースを追加する（既存関数を置き換え）:

```go
func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.composing {
		return m.handleComposeKey(msg)
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
	case "ctrl+c":
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg) // j/k などのスクロールは viewport に委譲
	return m, cmd
}

func (m Model) handleComposeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.posting {
		return m, nil // 送信中の入力は無視する
	}
	switch msg.String() {
	case "esc":
		m.composing = false
		m.postErr = ""
		m.textarea.Reset()
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
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
```

- [ ] **Step 10: テストが通ることを確認する**

Run: `go test ./internal/ui/ -run 'TestCompose|TestCEntersCompose' -v`
Expected: PASS（新規5件すべて）。続けて `go test ./internal/ui/ -v` で既存テストも壊れていないことを確認（PASS）

- [ ] **Step 11: gofmt / vet を確認してコミット**

```bash
gofmt -l . && go vet ./...
git add internal/ui/ui.go internal/ui/ui_test.go
git commit -m "feat: add comment compose state machine to detail screen

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: compose の描画とドキュメント

textarea を表示する `composeView` を追加し、詳細フッターに `c:comment` を足す。README に `c` キーを追記する。

**Files:**
- Modify: `internal/ui/render.go`（`composeView` 追加、`detailView` の compose 分岐、フッター文言）
- Test: `internal/ui/ui_test.go`（描画アサーションを1件追加）
- Modify: `README.md`（キーバインド表 or 説明に `c` を追記）

**Interfaces:**
- Consumes: Task 2 の `m.composing`、`m.posting`、`m.postErr`、`m.textarea`、`m.detailTitle`、既存 `titleStyle`/`dimStyle`/`m.spin`

- [ ] **Step 1: 失敗するテストを書く（`internal/ui/ui_test.go` の末尾に追記）**

```go
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
```

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ui/ -run TestComposeView -v`
Expected: FAIL（compose 中でも通常の詳細ビューが描画され、"ctrl+s:send" 等が出ないため assert 失敗）

- [ ] **Step 3: detailView に compose 分岐を追加し composeView を実装（`internal/ui/render.go`）**

`detailView` を置き換える（先頭に compose 分岐、フッターに `c:comment` を追加）:

```go
func (m Model) detailView() string {
	if m.composing {
		return m.composeView()
	}
	if m.detailLoading {
		return m.spin.View() + " loading...\n"
	}
	header := titleStyle.Render(m.detailTitle)
	footer := dimStyle.Render("j/k:scroll  r:refresh  o:browser  c:comment  esc:back")
	return header + "\n" + m.detail.View() + "\n" + footer
}

func (m Model) composeView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Comment on "+m.detailTitle) + "\n\n")
	b.WriteString(m.textarea.View() + "\n\n")
	if m.postErr != "" {
		b.WriteString("error: " + m.postErr + "\n\n")
	}
	if m.posting {
		b.WriteString(m.spin.View() + " posting...\n")
	} else {
		b.WriteString(dimStyle.Render("ctrl+s:send  esc:cancel"))
	}
	return b.String()
}
```

（`render.go` は既に `strings` を import 済み。追加 import は不要）

- [ ] **Step 4: テストが通ることを確認する**

Run: `go test ./internal/ui/ -v`
Expected: PASS（Task 2 の5件 + 本タスクの2件 + 既存すべて）

- [ ] **Step 5: README の Keys 表に c 行を追記（`README.md`）**

`README.md` の `### Keys` 表（`| Key | List | Detail |`）の `o` の行の直後に、`c`（詳細のみ）の行を1行だけ追加する。`c` は詳細画面専用なので List 列は `—`:

```
| `o` | open in browser | open in browser |
| `c` | — | comment (`Ctrl+S` send / `Esc` cancel) |
| `esc` | — | back to list |
```

（追加するのは `c` の行のみ。表の他の行や無関係な箇所は編集しない）

- [ ] **Step 6: gofmt / vet / 全テストを確認してコミット**

```bash
gofmt -l . && go vet ./... && go test ./...
git add internal/ui/render.go internal/ui/ui_test.go README.md
git commit -m "feat: render comment compose view and document c key

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## 実装後の手動 E2E（自動テスト対象外・スペック「実機検証で確認する残項目」）

- `go build -o bin/github-dash .` → `herdr plugin link` で実機に載せ、詳細画面で `c` → 入力 → `Ctrl+S` が動くこと
- `Ctrl+S`（端末の XOFF）が herdr/ターミナルに奪われて届かない場合は、送信キーを `Ctrl+D` 等に変更する（その場合 Task 2 Step 9 の `ctrl+s` と Task 3 の文言、README を合わせて更新）
