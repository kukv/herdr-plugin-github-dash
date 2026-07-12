# GitHub Dash Phase 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 詳細画面から、表示中の PR / Issue のラベル／アサインを共有の複数選択ピッカーで付け替えられるようにする。

**Architecture:** `ghcli` に候補取得（`ListLabels`/`ListAssignees`）と適用（`Edit{PR,Issue}{Labels,Assignees}`、内部は `editItems` ヘルパで DRY）を追加し、`PR`/`Issue` に `Assignees` を持たせる。`ui` は新規 `internal/ui/picker.go` に境界の明確なピッカーコンポーネント（純ロジック＋リスト描画）を切り出し、詳細画面に `picking` サブ状態を足す。`l`/`a` で候補を取得してピッカーを開き、`space` トグル・`enter` で差分を1回の `gh edit` に適用、`esc` キャンセル。適用失敗はピッカーを維持して選択を温存、候補取得失敗は詳細にインライン表示。`main.go`・`go.mod` は無変更。

**Tech Stack:** Go、charmbracelet/bubbletea・bubbles（spinner/viewport/textarea）・lipgloss・glamour、gh CLI 2.95、標準 `testing`

**Spec:** `docs/superpowers/specs/2026-07-12-github-dash-phase4-design.md`（承認済み）

## Global Constraints

- Go モジュールパス: `github.com/kukv/herdr-plugin-github-dash`
- 外部依存は charmbracelet の4つ（bubbletea / bubbles / lipgloss / glamour）のみ。**go.mod への新規依存追加は禁止**。ピッカーは自前実装（bubbles の list は使わない）
- テストは標準 `testing` のみ
- `gh` は必ず `exec.Cmd.Dir` に対象ディレクトリを設定して実行（新規メソッドは既存 `c.run` を通す）。`--repo` は override 時のみ付ける（既存 `appendRepo` を使う）。ただし `gh api` は `--repo` を取らないため、override 時は明示パスを組む
- スコープはラベル／アサインの付け替えのみ。新規ラベル作成・マイルストーン・レビュアー・一覧画面からの編集は入れない
- コミットは conventional commits（`feat:` / `docs:`）。SSH 署名はリポジトリローカルに設定済み。メッセージ末尾に `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>` を付ける
- 各タスク後に `gofmt -l .`（出力なし）と `go vet ./...`（エラーなし）を確認してからコミット

## File Structure

```
internal/ghcli/ghcli.go        # Assignees フィールド・view fields・ListLabels/ListAssignees（Task 1）、editItems と Edit系4メソッド（Task 2）
internal/ghcli/ghcli_test.go   # 候補取得・edit の args 検証テスト（Task 1, 2）
internal/ui/picker.go          # ピッカーコンポーネント（純ロジック＋listView）（Task 3）
internal/ui/picker_test.go     # ピッカーのユニットテスト（Task 3）
internal/ui/ui.go              # DataSource 拡張・Model 拡張・msg・detail 反映・enter リセット・キー処理・fetch/apply cmd・Update ケース（Task 4）
internal/ui/ui_test.go         # fakeSource 拡張・picking 挙動テスト（Task 4）、pickerView 描画テスト（Task 5）
internal/ui/render.go          # pickerView・detailView の picking/pickerLoading 分岐・フッター（Task 5）
README.md                      # l / a キーの追記（Task 5）
```

Task 1/2（ghcli）は ui から独立。Task 3（picker コンポーネント）は Model に依存しない純粋な型なので単体で受け入れ可能。Task 4 は picking の状態遷移（View 非依存の Model フィールド・fakeSource 呼び出し記録のみ検証）、Task 5 は描画（pickerView・フッター）と分ける。

---

### Task 1: ghcli に候補取得と Assignees を追加

**Files:**
- Modify: `internal/ghcli/ghcli.go`（view fields、`PR`/`Issue` に `Assignees`、末尾に `ListLabels`/`ListAssignees`）
- Test: `internal/ghcli/ghcli_test.go`（テスト追加）

**Interfaces:**
- Consumes: 既存の `Client{dir, run}`、`appendRepo`、`Label`、`Author`、`newTestClient`、`fakeRun`
- Produces（Task 3/4 が使う）:
  - `PR`/`Issue` の `Assignees []Author`
  - `(*Client) ListLabels(repo string) ([]Label, error)`
  - `(*Client) ListAssignees(repo string) ([]string, error)`

- [ ] **Step 1: 失敗するテストを書く（`internal/ghcli/ghcli_test.go` の末尾に追記）**

```go
func TestListLabels(t *testing.T) {
	c, f := newTestClient(`[{"name":"bug","color":"d73a4a"},{"name":"wip","color":"ededed"}]`, nil)
	labels, err := c.ListLabels("")
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	wantArgs := []string{"label", "list", "--json", "name,color", "--limit", "100"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if len(labels) != 2 || labels[0].Name != "bug" || labels[0].Color != "d73a4a" || labels[1].Name != "wip" {
		t.Errorf("unexpected parse result: %+v", labels)
	}
}

func TestListAssignees(t *testing.T) {
	c, f := newTestClient(`[{"login":"alice"},{"login":"bob"}]`, nil)
	users, err := c.ListAssignees("")
	if err != nil {
		t.Fatalf("ListAssignees: %v", err)
	}
	wantArgs := []string{"api", "repos/{owner}/{repo}/assignees"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
	if len(users) != 2 || users[0] != "alice" || users[1] != "bob" {
		t.Errorf("unexpected parse result: %v", users)
	}
}

func TestListAssigneesWithRepoOverride(t *testing.T) {
	c, f := newTestClient(`[]`, nil)
	if _, err := c.ListAssignees("octo/hello"); err != nil {
		t.Fatalf("ListAssignees: %v", err)
	}
	wantArgs := []string{"api", "repos/octo/hello/assignees"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestGetPRParsesAssignees(t *testing.T) {
	c, _ := newTestClient(`{"number":12,"title":"t","assignees":[{"login":"alice"},{"login":"bob"}]}`, nil)
	pr, err := c.GetPR("", 12)
	if err != nil {
		t.Fatalf("GetPR: %v", err)
	}
	if len(pr.Assignees) != 2 || pr.Assignees[0].Login != "alice" || pr.Assignees[1].Login != "bob" {
		t.Errorf("unexpected assignees: %+v", pr.Assignees)
	}
}
```

（`ghcli_test.go` は既に `errors` / `reflect` / `testing` を import 済み。追加 import は不要）

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ghcli/ -run 'TestListLabels|TestListAssignees|TestGetPRParsesAssignees' -v`
Expected: FAIL（`c.ListLabels undefined`、`pr.Assignees undefined` のコンパイルエラー）

- [ ] **Step 3: view fields と Assignees フィールドを追加（`internal/ghcli/ghcli.go`）**

view fields 定数を書き換える（`assignees` を追加）:

```go
const (
	prListFields    = "number,title,author,state,isDraft,updatedAt,reviewDecision,url"
	prViewFields    = prListFields + ",body,comments,labels,assignees"
	issueListFields = "number,title,author,state,updatedAt,labels,url"
	issueViewFields = issueListFields + ",body,comments,assignees"
)
```

`PR` struct の `Labels []Label` の行の後に追加:

```go
	Assignees []Author `json:"assignees"`
```

`Issue` struct の `Labels []Label` の行の後に追加:

```go
	Assignees []Author `json:"assignees"`
```

- [ ] **Step 4: ListLabels / ListAssignees を実装（`internal/ghcli/ghcli.go` の末尾、`ReopenIssue` の後に追加）**

```go
func (c *Client) ListLabels(repo string) ([]Label, error) {
	args := appendRepo([]string{"label", "list", "--json", "name,color", "--limit", "100"}, repo)
	out, err := c.run(c.dir, args...)
	if err != nil {
		return nil, err
	}
	var labels []Label
	if err := json.Unmarshal(out, &labels); err != nil {
		return nil, fmt.Errorf("parse label list: %w", err)
	}
	return labels, nil
}

// ListAssignees returns the logins of users assignable on the repository.
// gh api substitutes {owner}/{repo} from the current directory's repo; for an
// override we build the explicit path (gh api takes no --repo).
func (c *Client) ListAssignees(repo string) ([]string, error) {
	path := "repos/{owner}/{repo}/assignees"
	if repo != "" {
		path = "repos/" + repo + "/assignees"
	}
	out, err := c.run(c.dir, "api", path)
	if err != nil {
		return nil, err
	}
	var users []Author
	if err := json.Unmarshal(out, &users); err != nil {
		return nil, fmt.Errorf("parse assignees: %w", err)
	}
	logins := make([]string, len(users))
	for i, u := range users {
		logins[i] = u.Login
	}
	return logins, nil
}
```

（`json` / `fmt` は既に import 済み。追加 import は不要）

- [ ] **Step 5: テストが通ることを確認する**

Run: `go test ./internal/ghcli/ -v`
Expected: PASS（既存 + 新規4件すべて）

- [ ] **Step 6: gofmt / vet を確認してコミット**

```bash
gofmt -l . && go vet ./...
git add internal/ghcli/
git commit -m "feat: add label/assignee candidate fetching to ghcli

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: ghcli にラベル／アサイン編集を追加

**Files:**
- Modify: `internal/ghcli/ghcli.go`（末尾に private `editItems` と 4メソッド）
- Test: `internal/ghcli/ghcli_test.go`（テスト追加）

**Interfaces:**
- Consumes: 既存の `Client{dir, run}`、`appendRepo`、`strconv`
- Produces（Task 4 が使う）:
  - `(*Client) EditPRLabels(repo string, number int, add, remove []string) error`
  - `(*Client) EditIssueLabels(repo string, number int, add, remove []string) error`
  - `(*Client) EditPRAssignees(repo string, number int, add, remove []string) error`
  - `(*Client) EditIssueAssignees(repo string, number int, add, remove []string) error`

- [ ] **Step 1: 失敗するテストを書く（`internal/ghcli/ghcli_test.go` の末尾に追記）**

```go
func TestEditPRLabels(t *testing.T) {
	c, f := newTestClient("", nil)
	if err := c.EditPRLabels("", 12, []string{"bug"}, []string{"wip"}); err != nil {
		t.Fatalf("EditPRLabels: %v", err)
	}
	wantArgs := []string{"pr", "edit", "12", "--add-label", "bug", "--remove-label", "wip"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestEditPRLabelsAddOnly(t *testing.T) {
	c, f := newTestClient("", nil)
	if err := c.EditPRLabels("", 12, []string{"a", "b"}, nil); err != nil {
		t.Fatalf("EditPRLabels: %v", err)
	}
	wantArgs := []string{"pr", "edit", "12", "--add-label", "a", "--add-label", "b"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestEditIssueAssigneesWithRepoOverride(t *testing.T) {
	c, f := newTestClient("", nil)
	if err := c.EditIssueAssignees("octo/hello", 3, []string{"alice"}, []string{"bob"}); err != nil {
		t.Fatalf("EditIssueAssignees: %v", err)
	}
	wantArgs := []string{"issue", "edit", "3", "--add-assignee", "alice", "--remove-assignee", "bob", "--repo", "octo/hello"}
	if !reflect.DeepEqual(f.args, wantArgs) {
		t.Errorf("args = %v, want %v", f.args, wantArgs)
	}
}

func TestEditItemsError(t *testing.T) {
	wantErr := errors.New("gh pr: HTTP 403 forbidden")
	c, _ := newTestClient("", wantErr)
	if err := c.EditPRLabels("", 12, []string{"bug"}, nil); !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}
```

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ghcli/ -run TestEdit -v`
Expected: FAIL（`c.EditPRLabels undefined` のコンパイルエラー）

- [ ] **Step 3: editItems ヘルパと4メソッドを実装（`internal/ghcli/ghcli.go` の末尾、`ListAssignees` の後に追加）**

```go
func (c *Client) editItems(kindCmd, repo string, number int, add, remove []string, addFlag, removeFlag string) error {
	args := []string{kindCmd, "edit", strconv.Itoa(number)}
	for _, v := range add {
		args = append(args, addFlag, v)
	}
	for _, v := range remove {
		args = append(args, removeFlag, v)
	}
	_, err := c.run(c.dir, appendRepo(args, repo)...)
	return err
}

func (c *Client) EditPRLabels(repo string, number int, add, remove []string) error {
	return c.editItems("pr", repo, number, add, remove, "--add-label", "--remove-label")
}

func (c *Client) EditIssueLabels(repo string, number int, add, remove []string) error {
	return c.editItems("issue", repo, number, add, remove, "--add-label", "--remove-label")
}

func (c *Client) EditPRAssignees(repo string, number int, add, remove []string) error {
	return c.editItems("pr", repo, number, add, remove, "--add-assignee", "--remove-assignee")
}

func (c *Client) EditIssueAssignees(repo string, number int, add, remove []string) error {
	return c.editItems("issue", repo, number, add, remove, "--add-assignee", "--remove-assignee")
}
```

（`strconv` は既に import 済み。追加 import は不要）

- [ ] **Step 4: テストが通ることを確認する**

Run: `go test ./internal/ghcli/ -v`
Expected: PASS（既存 + 新規4件すべて）

- [ ] **Step 5: gofmt / vet を確認してコミット**

```bash
gofmt -l . && go vet ./...
git add internal/ghcli/
git commit -m "feat: add label/assignee editing to ghcli

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: ピッカーコンポーネント（純ロジック＋リスト描画）

Model に依存しない自前の複数選択ピッカーを新規ファイルに実装する。このタスクは `internal/ui/picker.go` と
`internal/ui/picker_test.go` のみを追加し、ui.go/render.go には触れない。ピッカーは選択状態・カーソル・
スクロール窓・差分算出を持ち、リスト部分の描画（`listView`）まで担う。適用スピナーや Model 状態には依存しない。

**Files:**
- Create: `internal/ui/picker.go`
- Test: `internal/ui/picker_test.go`

**Interfaces:**
- Consumes: 既存の `titleStyle` / `dimStyle` / `cursorPrefix`（`render.go`。同一パッケージ）、lipgloss
- Produces（Task 4/5 が使う）:
  - 型 `pickerKind`（`pickLabels` / `pickAssignees`）、`pickItem`、`picker`
  - `newPicker(kind pickerKind, title string, candidates []string, colors map[string]string, current []string) picker`
  - `(*picker) toggle()`、`(*picker) moveUp(visible int)`、`(*picker) moveDown(visible int)`
  - `(picker) diff() (add, remove []string)`、`(picker) listView(height int) string`
  - `visibleRows(height int) int`

- [ ] **Step 1: 失敗するテストを書く（`internal/ui/picker_test.go` を新規作成）**

```go
package ui

import (
	"strings"
	"testing"
)

func TestNewPickerPrechecksCurrent(t *testing.T) {
	p := newPicker(pickLabels, "Labels", []string{"bug", "wip"}, map[string]string{"bug": "d73a4a"}, []string{"bug"})
	if len(p.items) != 2 {
		t.Fatalf("items = %d, want 2", len(p.items))
	}
	if p.items[0].name != "bug" || !p.items[0].selected {
		t.Errorf("item0 = %+v, want bug selected", p.items[0])
	}
	if p.items[1].name != "wip" || p.items[1].selected {
		t.Errorf("item1 = %+v, want wip unselected", p.items[1])
	}
	if p.items[0].color != "d73a4a" {
		t.Errorf("color = %q, want d73a4a", p.items[0].color)
	}
}

func TestNewPickerIncludesCurrentNotInCandidates(t *testing.T) {
	// "bug" is currently applied but no longer in the candidate list; it must
	// still appear (selected) so enter does not silently remove it.
	p := newPicker(pickLabels, "Labels", []string{"wip"}, nil, []string{"bug"})
	var names []string
	for _, it := range p.items {
		names = append(names, it.name)
	}
	if len(p.items) != 2 {
		t.Fatalf("items = %v, want wip + bug", names)
	}
	found := false
	for _, it := range p.items {
		if it.name == "bug" && it.selected {
			found = true
		}
	}
	if !found {
		t.Errorf("current-but-uncandidate 'bug' missing or unselected: %v", names)
	}
}

func TestPickerToggleDiff(t *testing.T) {
	p := newPicker(pickLabels, "Labels", []string{"bug", "wip"}, nil, []string{"bug"})
	// cursor at 0 (bug, selected) -> toggle off (remove bug)
	p.toggle()
	// move to wip and toggle on (add wip)
	p.moveDown(10)
	p.toggle()
	add, remove := p.diff()
	if len(add) != 1 || add[0] != "wip" {
		t.Errorf("add = %v, want [wip]", add)
	}
	if len(remove) != 1 || remove[0] != "bug" {
		t.Errorf("remove = %v, want [bug]", remove)
	}
}

func TestPickerNoChangeEmptyDiff(t *testing.T) {
	p := newPicker(pickLabels, "Labels", []string{"bug", "wip"}, nil, []string{"bug"})
	add, remove := p.diff()
	if len(add) != 0 || len(remove) != 0 {
		t.Errorf("diff = %v/%v, want empty", add, remove)
	}
}

func TestPickerCursorAndScroll(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e"}
	p := newPicker(pickAssignees, "Assignees", names, nil, nil)
	// visible window of 2: moving down past the window advances offset
	for i := 0; i < 4; i++ {
		p.moveDown(2)
	}
	if p.cursor != 4 {
		t.Errorf("cursor = %d, want 4", p.cursor)
	}
	if p.offset != 3 { // window [3,5) shows cursor 4
		t.Errorf("offset = %d, want 3", p.offset)
	}
	p.moveDown(2) // already at last item, no-op
	if p.cursor != 4 {
		t.Errorf("cursor moved past end: %d", p.cursor)
	}
	for i := 0; i < 5; i++ {
		p.moveUp(2)
	}
	if p.cursor != 0 || p.offset != 0 {
		t.Errorf("cursor/offset = %d/%d, want 0/0", p.cursor, p.offset)
	}
}

func TestPickerListViewShowsItems(t *testing.T) {
	p := newPicker(pickLabels, "Labels", []string{"bug", "wip"}, nil, []string{"bug"})
	view := p.listView(20)
	for _, want := range []string{"Labels", "[x] bug", "[ ] wip"} {
		if !strings.Contains(view, want) {
			t.Errorf("listView missing %q:\n%s", want, view)
		}
	}
}
```

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ui/ -run TestPicker -v`
Expected: FAIL（`newPicker undefined` 等のコンパイルエラー）

- [ ] **Step 3: ピッカーコンポーネントを実装（`internal/ui/picker.go` を新規作成）**

```go
package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type pickerKind int

const (
	pickLabels pickerKind = iota
	pickAssignees
)

type pickItem struct {
	name     string
	color    string // ラベルの hex 色（アサインは空）
	selected bool
}

type picker struct {
	kind     pickerKind
	title    string
	items    []pickItem
	original map[string]bool // 初期選択（差分算出の基準）
	cursor   int
	offset   int    // スクロール窓の先頭 index
	err      string // 直近の適用失敗メッセージ
}

// newPicker builds a picker whose items are the union of candidates and the
// currently-applied values, with current values pre-selected. Including
// current-but-uncandidate values prevents a hidden item from being removed on
// apply.
func newPicker(kind pickerKind, title string, candidates []string, colors map[string]string, current []string) picker {
	currentSet := make(map[string]bool, len(current))
	for _, c := range current {
		currentSet[c] = true
	}
	seen := make(map[string]bool)
	var items []pickItem
	add := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		items = append(items, pickItem{name: name, color: colors[name], selected: currentSet[name]})
	}
	for _, c := range candidates {
		add(c)
	}
	for _, c := range current {
		add(c)
	}
	return picker{kind: kind, title: title, items: items, original: currentSet}
}

func (p *picker) toggle() {
	if len(p.items) == 0 {
		return
	}
	p.items[p.cursor].selected = !p.items[p.cursor].selected
}

func (p *picker) moveDown(visible int) {
	if p.cursor < len(p.items)-1 {
		p.cursor++
		if p.cursor >= p.offset+visible {
			p.offset = p.cursor - visible + 1
		}
	}
}

func (p *picker) moveUp(visible int) {
	if p.cursor > 0 {
		p.cursor--
		if p.cursor < p.offset {
			p.offset = p.cursor
		}
	}
}

func (p picker) diff() (add, remove []string) {
	for _, it := range p.items {
		switch {
		case it.selected && !p.original[it.name]:
			add = append(add, it.name)
		case !it.selected && p.original[it.name]:
			remove = append(remove, it.name)
		}
	}
	return add, remove
}

func (p picker) listView(height int) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(p.title) + "\n\n")
	if len(p.items) == 0 {
		b.WriteString(dimStyle.Render("(no candidates)") + "\n")
	}
	visible := visibleRows(height)
	end := p.offset + visible
	if end > len(p.items) {
		end = len(p.items)
	}
	for i := p.offset; i < end; i++ {
		it := p.items[i]
		box := "[ ]"
		if it.selected {
			box = "[x]"
		}
		name := it.name
		if it.color != "" {
			name = lipgloss.NewStyle().Foreground(lipgloss.Color("#" + it.color)).Render(name)
		}
		b.WriteString(cursorPrefix(i == p.cursor) + box + " " + name + "\n")
	}
	if p.err != "" {
		b.WriteString("\nerror: " + p.err + "\n")
	}
	return b.String()
}

// visibleRows is how many candidate rows fit given the terminal height.
func visibleRows(height int) int {
	if height <= 0 {
		return 10
	}
	return max(height-6, 3)
}
```

（`max` は Go 1.21 の組み込み。`titleStyle`/`dimStyle`/`cursorPrefix` は同一パッケージの `render.go` で定義済み）

- [ ] **Step 4: テストが通ることを確認する**

Run: `go test ./internal/ui/ -run TestPicker -v`
Expected: PASS（新規6件）。続けて `go test ./internal/ui/ -v` で既存も PASS

- [ ] **Step 5: gofmt / vet を確認してコミット**

```bash
gofmt -l . && go vet ./...
git add internal/ui/picker.go internal/ui/picker_test.go
git commit -m "feat: add multi-select picker component

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: ui の picking 状態遷移（描画は Task 5）

詳細画面の `picking` サブ状態・候補取得 cmd・差分適用 cmd・成功/失敗処理を追加する。このタスクのテストは
`m.View()` に依存せず、Model フィールドと `fakeSource` の呼び出し記録のみを検証する。

**Files:**
- Modify: `internal/ui/ui.go`（DataSource 拡張・Model 拡張・msg・detail 反映・enter リセット・キー処理・cmd・Update ケース・小ヘルパ）
- Test: `internal/ui/ui_test.go`（fakeSource 拡張・テスト追加）

**Interfaces:**
- Consumes: Task 1/2 の6メソッド、Task 3 の `pickerKind`/`pickLabels`/`pickAssignees`/`picker`/`newPicker`/`visibleRows` と `(*picker)` メソッド群。既存の `Model`、`Target`、`fetchDetail`、`handleDetailKey`、`detailModel`、`key`、`itoa`
- Produces（Task 5 が使う）:
  - Model フィールド: `picking`/`pickerLoading`/`applying bool`、`picker picker`、`detailLabels`/`detailAssignees []string`
  - `m.pickerView()` が読む値（Task 5 で `pickerView` 自体を実装）

- [ ] **Step 1: 失敗するテストを書く（`internal/ui/ui_test.go`）**

まず `fakeSource` に候補・edit の記録を追加する。既存 struct 定義（`stateErr error` の下）に追記し、メソッドを追加する:

```go
// fakeSource の struct に追記（stateErr の下）
	labels    []ghcli.Label
	users     []string
	editCalls []string // "pr:labels::12:add=bug:remove=wip"
	labelsErr error
	usersErr  error
	editErr   error

// fakeSource のメソッド群の末尾に追記
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
```

`key` ヘルパの switch に space を追加する（`case "ctrl+c":` の後）:

```go
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
```

テスト本体を追加する:

```go
func TestLOpensLabelPickerPrechecked(t *testing.T) {
	f := &fakeSource{prs: samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}}}
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
	f := &fakeSource{prs: samplePRs(),
		pr:    ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		users: []string{"alice", "bob"}}
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
	f := &fakeSource{prs: samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}}}
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
	f := &fakeSource{prs: samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}}}
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
	f := &fakeSource{prs: samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}}}
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
	f := &fakeSource{prs: samplePRs(),
		pr:      ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels:  []ghcli.Label{{Name: "bug"}, {Name: "wip"}},
		editErr: errors.New("gh pr: HTTP 403 forbidden")}
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
	f := &fakeSource{prs: samplePRs(),
		pr:        ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		labelsErr: errors.New("gh label: HTTP 403 forbidden")}
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
	m = next.(Model)
	if cmd == nil {
		t.Fatal("cmd = nil, want edit cmd")
	}
	cmd()
	if len(f.editCalls) != 1 || f.editCalls[0] != "issue:labels::5:add=bug:remove=" {
		t.Errorf("editCalls = %v, want [issue:labels::5:add=bug:remove=]", f.editCalls)
	}
}
```

（`ui_test.go` は既に `errors`/`strings`/`testing`/`time`/`tea`/`ghcli` を import 済み。追加 import は不要。上のテストはすべて既存の `detailModel` を使う）

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ui/ -run 'TestLOpens|TestAOpens|TestPickerApply|TestPickerNoChange|TestPickerEsc|TestPickerFetch' -v`
Expected: FAIL（`m.picking undefined`、`fakeSource` が `DataSource` を満たさない等のコンパイルエラー）

- [ ] **Step 3: DataSource インターフェースを拡張（`internal/ui/ui.go`）**

`DataSource` の `ReopenIssue` の行の後に追加する（DataSource は他メソッドと同様 `ghcli.Label` で表記する）:

```go
	ListLabels(repo string) ([]ghcli.Label, error)
	ListAssignees(repo string) ([]string, error)
	EditPRLabels(repo string, number int, add, remove []string) error
	EditIssueLabels(repo string, number int, add, remove []string) error
	EditPRAssignees(repo string, number int, add, remove []string) error
	EditIssueAssignees(repo string, number int, add, remove []string) error
```

- [ ] **Step 4: msg 型と Model フィールドを追加（`internal/ui/ui.go`）**

msg 型（`stateErrorMsg struct{ err error }` のある `type (...)` ブロック）に追加:

```go
	pickerCandidatesMsg struct {
		kind   pickerKind
		labels []ghcli.Label
		users  []string
	}
	pickerAppliedMsg struct{}
	pickErrorMsg     struct{ err error }
```

`Model` struct の `actionErr string` の下にフィールドを追加:

```go
	picking         bool
	pickerLoading   bool
	applying        bool
	picker          picker
	detailLabels    []string
	detailAssignees []string
```

- [ ] **Step 5: detail 反映・enter リセット・小ヘルパを追加（`internal/ui/ui.go`）**

`case prDetailMsg:` に現在ラベル／アサインの保持を追加する（`m.actionErr = ""` の後）:

```go
	case prDetailMsg:
		m.detailLoading = false
		m.detailState = msg.State
		m.actionErr = ""
		m.detailLabels = labelNames(msg.Labels)
		m.detailAssignees = authorLogins(msg.Assignees)
		m.detailTitle = fmt.Sprintf("PR #%d %s", msg.Number, msg.Title)
		m.setDetailContent(prMarkdown(ghcli.PR(msg)))
		return m, nil
```

同様に `case issueDetailMsg:`:

```go
	case issueDetailMsg:
		m.detailLoading = false
		m.detailState = msg.State
		m.actionErr = ""
		m.detailLabels = labelNames(msg.Labels)
		m.detailAssignees = authorLogins(msg.Assignees)
		m.detailTitle = fmt.Sprintf("Issue #%d %s", msg.Number, msg.Title)
		m.setDetailContent(issueMarkdown(ghcli.Issue(msg)))
		return m, nil
```

`handleListKey` の `case "enter":` に picking 系のリセットを追加する（`m.confirming = false` の後）:

```go
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
```

小ヘルパを追加する（`ui.go` の `selectedTarget` の後あたり、任意の位置でよい）:

```go
func labelNames(labels []ghcli.Label) []string {
	names := make([]string, len(labels))
	for i, l := range labels {
		names[i] = l.Name
	}
	return names
}

func authorLogins(authors []ghcli.Author) []string {
	logins := make([]string, len(authors))
	for i, a := range authors {
		logins[i] = a.Login
	}
	return logins
}
```

- [ ] **Step 6: Update にピッカー結果ケースを追加（`internal/ui/ui.go`、`case stateErrorMsg:` の後、`case errorMsg:` の前）**

```go
	case pickerCandidatesMsg:
		m.pickerLoading = false
		if msg.kind == pickLabels {
			names := make([]string, len(msg.labels))
			colors := make(map[string]string, len(msg.labels))
			for i, l := range msg.labels {
				names[i] = l.Name
				colors[l.Name] = l.Color
			}
			m.picker = newPicker(pickLabels, "Labels", names, colors, m.detailLabels)
		} else {
			m.picker = newPicker(pickAssignees, "Assignees", msg.users, nil, m.detailAssignees)
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
```

- [ ] **Step 7: fetch / apply cmd を追加（`internal/ui/ui.go`、`postComment` や `setState` の近く）**

```go
func fetchLabelPicker(src DataSource, target Target) tea.Cmd {
	return func() tea.Msg {
		labels, err := src.ListLabels(target.Repo)
		if err != nil {
			return pickErrorMsg{err}
		}
		return pickerCandidatesMsg{kind: pickLabels, labels: labels}
	}
}

func fetchAssigneePicker(src DataSource, target Target) tea.Cmd {
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
```

- [ ] **Step 8: キー処理を追加し、handleDetailKey から分岐（`internal/ui/ui.go`）**

`handleDetailKey` の先頭 `confirming` 分岐の後に `picking` 分岐を足し、`l`/`a` ケースを追加する（既存関数を置き換え）:

```go
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
		m.picker.moveUp(visibleRows(m.height))
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
```

（`space` は環境により `KeyMsg.String()` が `" "` か `"space"` になりうるため両方を case に置く。`key` テストヘルパは `tea.KeySpace` を返す）

- [ ] **Step 9: テストが通ることを確認する**

Run: `go test ./internal/ui/ -run 'TestLOpens|TestAOpens|TestPicker' -v`
Expected: PASS（新規9件すべて + Task 3 の picker ユニット）。続けて `go test ./internal/ui/ -v` で既存も PASS

- [ ] **Step 10: gofmt / vet を確認してコミット**

```bash
gofmt -l . && go vet ./...
git add internal/ui/ui.go internal/ui/ui_test.go
git commit -m "feat: add label/assignee picker state machine to detail screen

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: ピッカーの描画とドキュメント

`picking` 中に `pickerView` を表示し、`pickerLoading` 中はスピナー、詳細フッターに `l:labels  a:assign`
を追加する。README に `l`/`a` を追記する。

**Files:**
- Modify: `internal/ui/render.go`（`pickerView` 追加、`detailView` の picking/pickerLoading 分岐、フッター文言）
- Test: `internal/ui/ui_test.go`（描画アサーションを追加）
- Modify: `README.md`

**Interfaces:**
- Consumes: Task 4 の `m.picking`/`m.pickerLoading`/`m.applying`/`m.picker`、Task 3 の `m.picker.listView`、既存 `dimStyle`/`m.spin`

- [ ] **Step 1: 失敗するテストを書く（`internal/ui/ui_test.go` の末尾に追記）**

```go
func TestPickerViewShowsItemsAndHelp(t *testing.T) {
	f := &fakeSource{prs: samplePRs(),
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels: []ghcli.Label{{Name: "bug"}, {Name: "wip"}}}
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
	f := &fakeSource{prs: samplePRs(),
		pr:      ghcli.PR{Number: 1, Title: "first pr", State: "OPEN", Labels: []ghcli.Label{{Name: "bug"}}},
		labels:  []ghcli.Label{{Name: "bug"}, {Name: "wip"}},
		editErr: errors.New("gh pr: HTTP 403 forbidden")}
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
```

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ui/ -run 'TestPickerView|TestDetailFooterShowsLabelAssign' -v`
Expected: FAIL（picking 中でも通常の詳細ビューが描画され、"space:toggle" 等が出ないため assert 失敗）

- [ ] **Step 3: detailView に分岐を追加し pickerView を実装（`internal/ui/render.go`）**

`detailView` を置き換える（`confirming` の後に picking/pickerLoading 分岐、フッターに `l:labels  a:assign`）:

```go
func (m Model) detailView() string {
	if m.composing {
		return m.composeView()
	}
	if m.confirming {
		return m.confirmView()
	}
	if m.picking {
		return m.pickerView()
	}
	if m.detailLoading || m.pickerLoading {
		return m.spin.View() + " loading...\n"
	}
	header := titleStyle.Render(m.detailTitle)
	footer := dimStyle.Render("j/k:scroll  r:refresh  o:browser  c:comment  " + m.stateFooterKey() + "l:labels  a:assign  esc:back")
	body := header + "\n" + m.detail.View() + "\n"
	if m.actionErr != "" {
		body += "error: " + m.actionErr + "\n"
	}
	return body + footer
}

func (m Model) pickerView() string {
	body := m.picker.listView(m.height)
	if m.applying {
		return body + "\n" + m.spin.View() + " applying...\n"
	}
	return body + "\n" + dimStyle.Render("space:toggle  enter:apply  esc:cancel")
}
```

（`render.go` は既に `fmt`/`strings`/`lipgloss` を import 済み。追加 import は不要）

- [ ] **Step 4: テストが通ることを確認する**

Run: `go test ./internal/ui/ -v`
Expected: PASS（Task 3/4 の全件 + 本タスクの3件 + 既存すべて）

- [ ] **Step 5: README の Keys 表に l / a 行を追記（`README.md`）**

`README.md` の `### Keys` 表の `x` の行の直後に、`l` と `a`（詳細のみ）の行を追加する:

```
| `x` | — | close / reopen (`y` confirm / `n` cancel) |
| `l` | — | edit labels (`space` toggle / `enter` apply) |
| `a` | — | edit assignees (`space` toggle / `enter` apply) |
| `esc` | — | back to list |
```

（追加するのは `l` と `a` の行のみ。表の他の行や無関係な箇所は編集しない）

- [ ] **Step 6: gofmt / vet / 全テストを確認してコミット**

```bash
gofmt -l . && go vet ./... && go test ./...
git add internal/ui/render.go internal/ui/ui_test.go README.md
git commit -m "feat: render label/assignee picker and document l/a keys

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## 実装後の手動 E2E（自動テスト対象外・スペック「実機検証で確認する残項目」）

- `go build -o bin/github-dash .` → `herdr plugin link` で実機に載せ、詳細画面で `l` → トグル → `enter` のラベル付け替え、`a` のアサイン変更が動くこと
- 再取得後に詳細のラベル／アサイン表示が更新されること
- `gh api repos/{owner}/{repo}/assignees` が対象リポジトリのアサイン可能ユーザーを返すこと
- ラベル多数リポジトリでスクロール窓が破綻しないこと
- `space` / `enter` が herdr/ターミナルのキーグラブに奪われて届かない場合はキーを変更する（その場合 Task 4 Step 8 と Task 5 の文言、README を合わせて更新）
