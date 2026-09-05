# octoscope Phase 0: スタンドアローン化とリネーム 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Herdr プラグイン専用だった pane プロセスを、Windows / macOS / Linux で動く単独の CLI バイナリ `octoscope` にし、英語と日本語で表示できるようにする。

**Architecture:** 既存の UI とデータ取得の構造には手を入れない。Herdr 依存（`internal/herdrctx`、`herdr-plugin.toml`、`open.sh`、`run.sh`、リンクハンドラ経路）を取り除き、対象リポジトリの決定を `--repo` フラグとカレントディレクトリの git remote に置き換える。表示文字列は `internal/i18n` のカタログへ移し、以降のフェーズで文字列を足すときは必ず両言語へ足す。配布は GoReleaser で 3 OS のバイナリを GitHub Releases に置く。

**Tech Stack:** Go 1.27 / Bubble Tea v2 (`charm.land/bubbletea/v2`) / lipgloss v2 / `nicksnyder/go-i18n/v2` / `pelletier/go-toml/v2` / `jeandeaual/go-locale` / GoReleaser v2 / golangci-lint v2.13.2

**Spec:** `docs/superpowers/specs/2026-09-05-octoscope-standalone-design.md`

## Global Constraints

- Go モジュールパス: `github.com/kukv/octoscope`。バイナリ名: `octoscope`
- 対応プラットフォーム: linux / darwin / windows、amd64 / arm64。`CGO_ENABLED=0`
- GitHub Actions の `uses:` は **full-length commit SHA でピンする**（org ポリシー。タグ指定は CI で reject される）。行末に `# vX.Y.Z` のコメントを付ける
- charmbracelet の TUI v2 は `charm.land/*/v2` が正しい import パス。`github.com/charmbracelet/*/v2` は使わない
- 表示幅の計算は `github.com/charmbracelet/x/ansi` を使う。`len()` や `utf8.RuneCountInString()` で桁を数えない
- カバレッジ基準は 80%（`.octocov.yml`）。下回ると CI が赤くなる
- コーディング規約は `CLAUDE.md` と `.claude/rules/` にある。実装前に読むこと
- 各タスクの完了時に `make check`（tidy-check / lint / fmt-check / test）が緑であること
- **Phase 0 のスコープ外**（勝手にやらないこと）:
  - `internal/ghcli` → `internal/gh/cli` への移設、`internal/ui` のサブモデル分割（Phase 1）
  - タブ、カンバン、diff、checks、merge、装飾（Phase 1 以降）
  - 設定ファイルの読み込み（Phase 4）。そのため Phase 0 の言語決定順は
    「`--lang` → OS ロケール → en」であり、spec §6.3 の 2 番目（config の `language`）は Phase 4 で追加する
  - Nerd Font の判定（Phase 1 以降）
- Phase 0 完了時点の画面は、文字列以外は現在とまったく同じに見えること

---

## File Structure

| ファイル | 責務 |
|---|---|
| `cmd/octoscope/main.go` | フラグ解析、言語の決定、依存の組み立て、Bubble Tea の起動 |
| `internal/i18n/i18n.go` | メッセージカタログの読み込みと `T` / `Tf` / `Tn`、言語決定 |
| `internal/i18n/locales/active.en.toml` | 英語カタログ |
| `internal/i18n/locales/active.ja.toml` | 日本語カタログ |
| `internal/ghcli/ghcli.go` | gh CLI の実行（既存）。既定リポジトリを保持するよう変更 |
| `internal/ui/*.go` | 既存の UI。文字列をカタログ参照に置換 |
| `.goreleaser.yaml` | クロスコンパイルとリリース成果物の定義 |
| `.github/workflows/release.yaml` | タグ push でリリースを作る |

**削除するもの:** `main.go`、`main_test.go`、`internal/herdrctx/`、`herdr-plugin.toml`、`open.sh`、`run.sh`

---

## Task 1: Go モジュールのリネーム

**Files:**
- Modify: `go.mod:1`
- Modify: `main.go`、`main_test.go`、`internal/ui/ui.go:14`、`internal/ui/render.go:10`、`internal/ui/ui_test.go`、`internal/ui/render_test.go`、`internal/ui/picker_test.go`、`internal/ghcli/ghcli_test.go`（import パスのみ）

**Interfaces:**
- Consumes: なし
- Produces: 以降のすべてのタスクは `github.com/kukv/octoscope/...` を import する

このタスクは import パスの機械的な置換のみで、挙動は一切変えない。

- [ ] **Step 1: 現状のテストが通ることを確認する**

Run: `make test`
Expected: PASS（この時点の基準線）

- [ ] **Step 2: モジュールパスを書き換える**

```bash
go mod edit -module github.com/kukv/octoscope
```

- [ ] **Step 3: 全ファイルの import パスを置換する**

```bash
grep -rl 'github.com/kukv/herdr-plugin-github-dash' --include='*.go' . \
  | xargs sed -i 's|github.com/kukv/herdr-plugin-github-dash|github.com/kukv/octoscope|g'
```

- [ ] **Step 4: 置換漏れが無いことを確認する**

Run: `grep -rn 'herdr-plugin-github-dash' --include='*.go' --include='go.mod' . ; echo "exit=$?"`
Expected: 何も出力されない（`exit=1`）

- [ ] **Step 5: ビルドとテストが通ることを確認する**

Run: `go build ./... && make test`
Expected: PASS

`.golangci.yml` には `local-prefixes` の設定が無いため、goimports のグループ分けはモジュールパスの変更に影響されない。設定変更は不要。

- [ ] **Step 6: フォーマットと lint を確認する**

Run: `make fmt-check && make lint`
Expected: どちらも差分・指摘なし

- [ ] **Step 7: コミット**

```bash
git add -A
git commit -m "refactor: rename go module to github.com/kukv/octoscope

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: スタンドアローンのエントリポイント

**Files:**
- Create: `cmd/octoscope/main.go`
- Delete: `main.go`、`main_test.go`、`internal/herdrctx/herdrctx.go`、`internal/herdrctx/herdrctx_test.go`、`herdr-plugin.toml`、`open.sh`、`run.sh`
- Modify: `internal/ui/ui.go:145`（`New` のシグネチャと起動時の詳細画面分岐）
- Modify: `internal/ui/ui_test.go`（`ui.New` の呼び出し箇所、および `:307` 付近の起動時詳細テスト）

**Interfaces:**
- Consumes: Task 1 のモジュールパス
- Produces:
  - `ui.New(ds DataSource) Model` — 第 2 引数の `*Target` を削除した形
  - `ui.NewError(text string) Model` — 変更なし
  - `ghcli.New(dir string) *Client` — 変更なし（Task 3 で変わる）
  - `ui.Target` / `ui.Kind` / `ui.KindPR` / `ui.KindIssue` — **そのまま残る**

**背景:** `parseTarget` と `GITHUB_DASH_URL` は Herdr のリンクハンドラ専用の入口であり、リンクハンドラを廃止すると誰も使わなくなる。

一方で **`Target` 型そのものは Herdr 固有ではない**。`Model.detailTarget` が「いま開いている PR / Issue はどれか」を保持しており、`render.go:111` の `m.detailTarget.Kind == KindPR` をはじめ、詳細画面の再取得（`fetchDetail`）、ブラウザで開く（`openWeb`）、close/reopen の分岐がすべてこのフィールドを使っている。**削除してはいけない。**

このタスクで消すのは次の 3 つだけ:

1. `New` の第 2 引数 `initial *Target`
2. その引数が有効なときに起動直後から詳細画面を開く分岐（`internal/ui/ui.go:145` 付近の `m.detailTarget = *initial` と、続く `screenDetail` への遷移および `:171` の初期 `fetchDetail`）
3. `main.go` の `parseTarget` と `main_test.go`

`DataSource` の各メソッドが持つ `repo string` 引数も **残す** — Phase 1 のリポジトリ横断表示がこの引数を使う。

- [ ] **Step 1: 影響範囲を洗い出す**

Run: `grep -rn 'parseTarget\|GITHUB_DASH_URL\|herdrctx' --include='*.go' --include='*.toml' --include='*.sh' .`
Expected: `main.go`、`main_test.go`、`internal/herdrctx/`、`herdr-plugin.toml`、`run.sh` が挙がる

Run: `grep -rn 'detailTarget\|KindPR\|KindIssue' --include='*.go' internal/`
Expected: `render.go`、`ui.go`、`ui_test.go` に多数。**これらは残す対象**であることを確認する

- [ ] **Step 2: Herdr 資産を削除する**

```bash
git rm -r internal/herdrctx herdr-plugin.toml open.sh run.sh main.go main_test.go
```

- [ ] **Step 3: 新しいエントリポイントを作る**

Create `cmd/octoscope/main.go`:

```go
// Command octoscope is a standalone terminal dashboard for GitHub.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/kukv/octoscope/internal/ghcli"
	"github.com/kukv/octoscope/internal/ui"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	p := tea.NewProgram(ui.New(ghcli.New(dir)))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: `ui.New` から起動時ターゲットの引数を取り除く**

`internal/ui/ui.go` を次のように変更する。**型定義（`Kind`、`KindPR`、`KindIssue`、`Target`）には触らない。**

1. `New` のシグネチャを `func New(ds DataSource) Model` にする
2. `:145` 付近の `if initial != nil { m.detailTarget = *initial; ... }` の分岐を削除し、常に `screenList` から始まるようにする
3. `:171` 付近で、その分岐が有効なときだけ積んでいた初期 `fetchDetail` の `cmds` 追加を削除する

`m.detailTarget` フィールド自体は残す。リストから `enter` で詳細を開くときに `:481` で代入される。

- [ ] **Step 5: コンパイルが通るまで呼び出し側を直す**

Run: `go build ./... && go vet ./...`
Expected: エラーなし。`internal/ui/ui_test.go` の `ui.New(fake, nil)` は `ui.New(fake)` に直す

`internal/ui/ui_test.go:307` 付近の「起動直後に別リポジトリの PR を開く」テスト（`"external pr"` を検査しているもの）は、削除した機能を検査しているため**テストごと削除する**。リストから `enter` で詳細を開くテストは残す。

- [ ] **Step 6: テストが通ることを確認する**

Run: `make test`
Expected: PASS。`TestParseTarget` と herdrctx のテストは削除されているので実行されない

- [ ] **Step 7: 削除漏れが無いことを確認する**

Run: `grep -rn 'herdr\|Herdr\|HERDR' --include='*.go' --include='*.yml' --include='*.yaml' . ; echo "exit=$?"`
Expected: Go ファイルとワークフローには一切残っていない（`exit=1`）。`README.md` にはまだ残るが、それは Task 6 で扱う

- [ ] **Step 8: 実際に起動して確認する**

Run: `go run ./cmd/octoscope`（このリポジトリのディレクトリで実行する）
Expected: PR / Issue のリストが表示される。`q` で終了できる

- [ ] **Step 9: `make check` とコミット**

```bash
make check
git add -A
git commit -m "feat: run as a standalone command instead of a Herdr pane

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: `--repo` フラグと対象リポジトリの解決

**Files:**
- Modify: `internal/ghcli/ghcli.go`
- Modify: `internal/ghcli/ghcli_test.go`
- Modify: `cmd/octoscope/main.go`

**Interfaces:**
- Consumes: `ui.New(ds DataSource) Model`（Task 2）
- Produces:
  - `ghcli.New(dir, repo string) *Client` — `repo` は `"owner/name"`、空文字ならカレントディレクトリの git remote に従う
  - `(*Client).effectiveRepo(repo string) string` — 非公開ヘルパー

**背景:** これまでは常にカレントディレクトリのリポジトリを見ていた（`gh` の暗黙の挙動）。スタンドアローンでは、リポジトリ外から起動する場合に明示指定が要る。`DataSource` の各メソッドが受け取る `repo` 引数（呼び出しごとの上書き）は残したまま、`Client` に既定値を持たせる。

`gh` のサブコマンドは `--repo owner/name` を受け取るが、`gh repo view` は位置引数、`gh api` はどちらも取らない（パスに埋め込む）。この 3 通りを取り違えないこと。

- [ ] **Step 1: 失敗するテストを書く**

`internal/ghcli/ghcli_test.go` に追加する（既存のテストが `run` を差し替えるヘルパーを持っているはずなので、同じ形に合わせること。無ければ以下のように直接フィールドを差し替える）:

```go
func TestClientUsesDefaultRepo(t *testing.T) {
	var got []string
	c := New("/tmp", "kukv/octoscope")
	c.run = func(_ string, args ...string) ([]byte, error) {
		got = args
		return []byte("[]"), nil
	}
	if _, err := c.ListPRs(); err != nil {
		t.Fatalf("ListPRs: %v", err)
	}
	want := []string{"pr", "list", "--json", prListFields, "--repo", "kukv/octoscope"}
	if !slices.Equal(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func TestPerCallRepoOverridesDefault(t *testing.T) {
	var got []string
	c := New("/tmp", "kukv/octoscope")
	c.run = func(_ string, args ...string) ([]byte, error) {
		got = args
		return []byte("{}"), nil
	}
	if _, err := c.GetPR("herdr/herdr", 7); err != nil {
		t.Fatalf("GetPR: %v", err)
	}
	if !slices.Contains(got, "herdr/herdr") || slices.Contains(got, "kukv/octoscope") {
		t.Errorf("args = %v, want the per-call repo to win", got)
	}
}

func TestRepoNameUsesPositionalArgument(t *testing.T) {
	var got []string
	c := New("/tmp", "kukv/octoscope")
	c.run = func(_ string, args ...string) ([]byte, error) {
		got = args
		return []byte(`{"nameWithOwner":"kukv/octoscope"}`), nil
	}
	if _, err := c.RepoName(); err != nil {
		t.Fatalf("RepoName: %v", err)
	}
	want := []string{"repo", "view", "kukv/octoscope", "--json", "nameWithOwner"}
	if !slices.Equal(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func TestListAssigneesBuildsAPIPathFromDefaultRepo(t *testing.T) {
	var got []string
	c := New("/tmp", "kukv/octoscope")
	c.run = func(_ string, args ...string) ([]byte, error) {
		got = args
		return []byte("[]"), nil
	}
	if _, err := c.ListAssignees(""); err != nil {
		t.Fatalf("ListAssignees: %v", err)
	}
	want := []string{"api", "repos/kukv/octoscope/assignees?per_page=100"}
	if !slices.Equal(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}
```

`slices` を import に足すこと。

- [ ] **Step 2: テストが失敗することを確認する**

Run: `go test ./internal/ghcli/ -run 'TestClientUsesDefaultRepo|TestPerCallRepo|TestRepoNameUses|TestListAssigneesBuilds' -v`
Expected: コンパイルエラー（`New` の引数が 1 つしかない）

- [ ] **Step 3: `Client` に既定リポジトリを持たせる**

`internal/ghcli/ghcli.go`:

```go
// Client runs gh commands in a fixed directory, against a fixed repository.
type Client struct {
	dir  string
	repo string
	run  runFunc
}

// New returns a client for the repository named by repo ("owner/name").
// An empty repo falls back to the repository of the git remote in dir.
func New(dir, repo string) *Client {
	return &Client{dir: dir, repo: repo, run: runGh}
}

// effectiveRepo picks the per-call repository if given, else the client's.
func (c *Client) effectiveRepo(repo string) string {
	if repo != "" {
		return repo
	}
	return c.repo
}
```

- [ ] **Step 4: すべてのメソッドを `effectiveRepo` 経由にする**

置換の指針は 3 通り。

1. **`--repo` を取るサブコマンド**（`pr`、`issue`、`label`）: `appendRepo(args, c.effectiveRepo(repo))` にする。
   `ListPRs` / `ListIssues` / `ListLabels` は引数に `repo` を取らないので `c.repo` を渡す:

```go
func (c *Client) ListPRs() ([]PR, error) {
	args := appendRepo([]string{"pr", "list", "--json", prListFields}, c.repo)
	out, err := c.run(c.dir, args...)
	// 以下は既存のまま
```

2. **位置引数を取る `gh repo view`**:

```go
func (c *Client) RepoName() (string, error) {
	args := []string{"repo", "view"}
	if c.repo != "" {
		args = append(args, c.repo)
	}
	args = append(args, "--json", "nameWithOwner")
	out, err := c.run(c.dir, args...)
	// 以下は既存のまま
```

3. **`gh api`**（`--repo` を取らない）: 既存の `ListAssignees` の分岐を `effectiveRepo` に置き換える:

```go
func (c *Client) ListAssignees(repo string) ([]string, error) {
	path := "repos/{owner}/{repo}/assignees?per_page=100"
	if r := c.effectiveRepo(repo); r != "" {
		path = "repos/" + r + "/assignees?per_page=100"
	}
	out, err := c.run(c.dir, "api", path)
	// 以下は既存のまま
```

`GetPR` / `GetIssue` / `OpenPRWeb` / `OpenIssueWeb` / `AddPRComment` / `AddIssueComment` / `ClosePR` / `ReopenPR` / `CloseIssue` / `ReopenIssue` / `editItems` は、`appendRepo(args, repo)` を `appendRepo(args, c.effectiveRepo(repo))` に置き換える。

- [ ] **Step 5: テストが通ることを確認する**

Run: `go test ./internal/ghcli/ -v`
Expected: PASS。既存のテストも `New` の引数を 2 つに直すこと（既定リポジトリを使わないケースは `New(dir, "")`）

- [ ] **Step 6: フラグを追加する**

`cmd/octoscope/main.go` を次のようにする。`--help` の文言は英語固定（spec §6.1）。

```go
// Command octoscope is a standalone terminal dashboard for GitHub.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/kukv/octoscope/internal/ghcli"
	"github.com/kukv/octoscope/internal/ui"
)

func main() {
	repo := flag.String("repo", "",
		"target repository as owner/name; defaults to the repository of the current directory")
	flag.Parse()

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	p := tea.NewProgram(ui.New(ghcli.New(dir, *repo)))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 7: リポジトリ外から起動して確認する**

Run: `cd /tmp && go run github.com/kukv/octoscope/cmd/octoscope --repo kukv/herdr-plugin-github-dash`

Expected: `/tmp` は git リポジトリではないが、指定したリポジトリの PR / Issue が表示される。

（`go run` がモジュール外で動かない場合は、先に `go build -o /tmp/octoscope ./cmd/octoscope` してから `cd /tmp && ./octoscope --repo kukv/herdr-plugin-github-dash` とする。リポジトリ名は GitHub 上のリネームが済むまで旧名のままであることに注意。）

**`--repo` も git remote も無い場合の挙動:** `gh` が返すエラー（`no git remotes found`）をそのままエラー画面に出す。既存の `internal/ui/ui_test.go:232` がこの経路を検査しているため、Phase 0 では変更しない。spec §3.4 の 3 番目（リポジトリ非依存の Work タブから開始する）は Phase 1 で実装する。

- [ ] **Step 8: `make check` とコミット**

```bash
make check
git add -A
git commit -m "feat: add --repo flag to target a repository explicitly

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: i18n の土台と文字列のカタログ移行

**Files:**
- Create: `internal/i18n/i18n.go`
- Create: `internal/i18n/i18n_test.go`
- Create: `internal/i18n/locales/active.en.toml`
- Create: `internal/i18n/locales/active.ja.toml`
- Modify: `internal/ui/render.go`（表示文字列すべて）
- Modify: `internal/ui/ui.go:134`（textarea の placeholder）、`internal/ui/ui.go:363,372`（詳細タイトル）、`internal/ui/ui.go:406,408`（ピッカーのタイトル）
- Modify: `internal/ui/picker.go`（フッターがある場合）
- Create: `internal/ui/i18n_test.go`
- Modify: `cmd/octoscope/main.go`

**Interfaces:**
- Consumes: `ghcli.New(dir, repo string)`（Task 3）
- Produces:
  - `i18n.SetLanguage(tag language.Tag)` — プロセス全体の表示言語を切り替える
  - `i18n.T(id string) string` — 引数なしのメッセージ
  - `i18n.Tf(id string, data map[string]any) string` — テンプレート引数つき
  - `i18n.Tn(id string, n int) string` — 複数形。テンプレート変数は `.Count`
  - `i18n.Resolve(flagLang, osLocale string) language.Tag` — 純粋関数。言語決定の順序を担う
  - `i18n.IDs() []string` — カタログに登録されているメッセージ ID の一覧（テスト用）

**背景:** 現在の表示文字列は `internal/ui/render.go` と `internal/ui/ui.go` に英語で直書きされている。既存テストは `strings.Contains(view, "No open pull requests")` の形で英語の文言をそのまま検査している（`internal/ui/ui_test.go` に 27 箇所）。既定言語を英語のままにし、**英語の文言を一字も変えない**ことで、既存テストを書き換えずに済ませる。唯一の例外はアプリ名で、`GitHub Dash` → `octoscope` に変わる。

- [ ] **Step 1: 依存を追加する**

```bash
go get github.com/nicksnyder/go-i18n/v2@latest
go get github.com/pelletier/go-toml/v2@latest
go get github.com/jeandeaual/go-locale@latest
go get golang.org/x/text
```

Run: `go doc github.com/nicksnyder/go-i18n/v2/i18n.Bundle | grep -i loadmessagefilefs`
Expected: `LoadMessageFileFS` が存在する。存在しなければ `ParseMessageFileBytes` に `embed.FS` から読んだバイト列を渡す形にする

- [ ] **Step 2: 失敗するテストを書く**

Create `internal/i18n/i18n_test.go`:

```go
package i18n_test

import (
	"testing"

	"golang.org/x/text/language"

	"github.com/kukv/octoscope/internal/i18n"
)

func TestTranslatesInEnglishAndJapanese(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })

	i18n.SetLanguage(language.English)
	if got := i18n.T("list.no_open_prs"); got != "No open pull requests" {
		t.Errorf("en = %q", got)
	}

	i18n.SetLanguage(language.Japanese)
	if got := i18n.T("list.no_open_prs"); got != "オープンなプルリクエストはありません" {
		t.Errorf("ja = %q", got)
	}
}

func TestPluralInEnglish(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })
	i18n.SetLanguage(language.English)

	if got := i18n.Tn("time.hours_ago", 1); got != "1h ago" {
		t.Errorf("n=1: %q", got)
	}
	if got := i18n.Tn("time.hours_ago", 3); got != "3h ago" {
		t.Errorf("n=3: %q", got)
	}
}

func TestResolveOrder(t *testing.T) {
	cases := []struct {
		name          string
		flag, osLocal string
		want          language.Tag
	}{
		{"flag wins", "ja", "en-US", language.Japanese},
		{"os locale used when no flag", "", "ja-JP", language.Japanese},
		{"english by default", "", "", language.English},
		{"unsupported falls back to english", "", "de-DE", language.English},
		{"invalid flag falls back to os locale", "zzz", "ja-JP", language.Japanese},
	}
	for _, c := range cases {
		if got := i18n.Resolve(c.flag, c.osLocal); got != c.want {
			t.Errorf("%s: Resolve(%q, %q) = %v, want %v", c.name, c.flag, c.osLocal, got, c.want)
		}
	}
}

func TestCatalogsHaveTheSameIDs(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })

	i18n.SetLanguage(language.English)
	en := i18n.IDs()
	i18n.SetLanguage(language.Japanese)
	ja := i18n.IDs()

	missing := map[string]bool{}
	for _, id := range en {
		missing[id] = true
	}
	for _, id := range ja {
		delete(missing, id)
	}
	if len(missing) > 0 {
		t.Errorf("ja catalog is missing IDs: %v", missing)
	}
}
```

- [ ] **Step 3: テストが失敗することを確認する**

Run: `go test ./internal/i18n/ -v`
Expected: コンパイルエラー（パッケージが存在しない）

- [ ] **Step 4: カタログを作る**

Create `internal/i18n/locales/active.en.toml`。ID にドットを含むため、テーブル名は必ずクォートする。

```toml
["app.name"]
other = "octoscope"

["app.error_title"]
other = "octoscope — error"

["common.loading"]
other = "loading..."

["common.error_prefix"]
other = "error: "

["list.tab_prs"]
other = "Pull Requests"

["list.tab_issues"]
other = "Issues"

["list.no_open_prs"]
other = "No open pull requests"

["list.no_open_issues"]
other = "No open issues"

["detail.pr_title"]
other = "PR #{{.Number}} {{.Title}}"

["detail.issue_title"]
other = "Issue #{{.Number}} {{.Title}}"

["compose.title"]
other = "Comment on {{.Title}}"

["compose.placeholder"]
other = "Leave a comment..."

["compose.posting"]
other = "posting..."

["picker.labels"]
other = "Labels"

["picker.assignees"]
other = "Assignees"

["picker.applying"]
other = "applying..."

["confirm.close_pr"]
other = "Close this PR? "

["confirm.reopen_pr"]
other = "Reopen this PR? "

["confirm.close_issue"]
other = "Close this issue? "

["confirm.reopen_issue"]
other = "Reopen this issue? "

["confirm.yes_no"]
other = "(y/n)"

["confirm.working"]
other = "working..."

["footer.list"]
other = "j/k:move  enter:detail  tab:PR/Issue  r:refresh  o:browser  q:quit"

["footer.detail_prefix"]
other = "j/k:scroll  r:refresh  o:browser  c:comment  "

["footer.detail_suffix"]
other = "l:labels  a:assign  esc:back"

["footer.close"]
other = "x:close  "

["footer.reopen"]
other = "x:reopen  "

["footer.picker"]
other = "space:toggle  enter:apply  esc:cancel"

["footer.compose"]
other = "ctrl+s:send  esc:cancel"

["footer.error"]
other = "q:quit"

["md.author"]
other = "author"

["md.state"]
other = "state"

["md.review"]
other = "review"

["md.labels"]
other = "labels"

["md.updated"]
other = "updated"

["md.draft_suffix"]
other = " (draft)"

["md.no_description"]
other = "_no description_"

["time.now"]
other = "now"

["time.minutes_ago"]
one = "{{.Count}}m ago"
other = "{{.Count}}m ago"

["time.hours_ago"]
one = "{{.Count}}h ago"
other = "{{.Count}}h ago"

["time.days_ago"]
one = "{{.Count}}d ago"
other = "{{.Count}}d ago"
```

Create `internal/i18n/locales/active.ja.toml`。日本語には複数形が無いため `other` のみを書く。

```toml
["app.name"]
other = "octoscope"

["app.error_title"]
other = "octoscope — エラー"

["common.loading"]
other = "読み込み中..."

["common.error_prefix"]
other = "エラー: "

["list.tab_prs"]
other = "プルリクエスト"

["list.tab_issues"]
other = "Issue"

["list.no_open_prs"]
other = "オープンなプルリクエストはありません"

["list.no_open_issues"]
other = "オープンな Issue はありません"

["detail.pr_title"]
other = "PR #{{.Number}} {{.Title}}"

["detail.issue_title"]
other = "Issue #{{.Number}} {{.Title}}"

["compose.title"]
other = "{{.Title}} にコメント"

["compose.placeholder"]
other = "コメントを入力..."

["compose.posting"]
other = "送信中..."

["picker.labels"]
other = "ラベル"

["picker.assignees"]
other = "担当者"

["picker.applying"]
other = "適用中..."

["confirm.close_pr"]
other = "この PR をクローズしますか? "

["confirm.reopen_pr"]
other = "この PR を再オープンしますか? "

["confirm.close_issue"]
other = "この Issue をクローズしますか? "

["confirm.reopen_issue"]
other = "この Issue を再オープンしますか? "

["confirm.yes_no"]
other = "(y/n)"

["confirm.working"]
other = "処理中..."

["footer.list"]
other = "j/k:移動  enter:詳細  tab:PR/Issue  r:更新  o:ブラウザ  q:終了"

["footer.detail_prefix"]
other = "j/k:スクロール  r:更新  o:ブラウザ  c:コメント  "

["footer.detail_suffix"]
other = "l:ラベル  a:担当者  esc:戻る"

["footer.close"]
other = "x:クローズ  "

["footer.reopen"]
other = "x:再オープン  "

["footer.picker"]
other = "space:選択  enter:適用  esc:中止"

["footer.compose"]
other = "ctrl+s:送信  esc:中止"

["footer.error"]
other = "q:終了"

["md.author"]
other = "作成者"

["md.state"]
other = "状態"

["md.review"]
other = "レビュー"

["md.labels"]
other = "ラベル"

["md.updated"]
other = "更新"

["md.draft_suffix"]
other = "（下書き）"

["md.no_description"]
other = "_説明はありません_"

["time.now"]
other = "たった今"

["time.minutes_ago"]
other = "{{.Count}} 分前"

["time.hours_ago"]
other = "{{.Count}} 時間前"

["time.days_ago"]
other = "{{.Count}} 日前"
```

- [ ] **Step 5: i18n パッケージを実装する**

Create `internal/i18n/i18n.go`:

```go
// Package i18n loads the message catalogs and renders localized strings.
package i18n

import (
	"embed"
	"sort"
	"sync"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var localeFS embed.FS

// supported lists the languages with a catalog, most preferred first.
var supported = []language.Tag{language.English, language.Japanese}

var (
	mu        sync.RWMutex
	bundle    *goi18n.Bundle
	localizer *goi18n.Localizer
	catalog   = map[language.Tag][]string{}
	current   = language.English
)

func init() {
	bundle = goi18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	for _, name := range []string{"locales/active.en.toml", "locales/active.ja.toml"} {
		f, err := bundle.LoadMessageFileFS(localeFS, name)
		if err != nil {
			// The catalogs are embedded, so a failure here is a build-time bug.
			panic(err)
		}
		ids := make([]string, 0, len(f.Messages))
		for _, m := range f.Messages {
			ids = append(ids, m.ID)
		}
		sort.Strings(ids)
		catalog[f.Tag] = ids
	}
	SetLanguage(language.English)
}

// SetLanguage switches the language used by T, Tf and Tn.
func SetLanguage(tag language.Tag) {
	mu.Lock()
	defer mu.Unlock()
	current = tag
	localizer = goi18n.NewLocalizer(bundle, tag.String())
}

// Resolve picks the display language: the --lang flag first, then the
// locale reported by the operating system, then English.
func Resolve(flagLang, osLocale string) language.Tag {
	matcher := language.NewMatcher(supported)
	for _, candidate := range []string{flagLang, osLocale} {
		if candidate == "" {
			continue
		}
		tag, err := language.Parse(candidate)
		if err != nil {
			continue
		}
		if _, index, conf := matcher.Match(tag); conf != language.No {
			return supported[index]
		}
	}
	return language.English
}

// IDs returns the message IDs in the current language's catalog, sorted.
func IDs() []string {
	mu.RLock()
	defer mu.RUnlock()
	return catalog[current]
}

// T renders the message with no template data.
func T(id string) string {
	return render(&goi18n.LocalizeConfig{MessageID: id})
}

// Tf renders the message with template data, e.g. {"Title": "..."}.
func Tf(id string, data map[string]any) string {
	return render(&goi18n.LocalizeConfig{MessageID: id, TemplateData: data})
}

// Tn renders a message whose wording depends on n; the template variable
// is .Count.
func Tn(id string, n int) string {
	return render(&goi18n.LocalizeConfig{
		MessageID:    id,
		PluralCount:  n,
		TemplateData: map[string]any{"Count": n},
	})
}

func render(cfg *goi18n.LocalizeConfig) string {
	mu.RLock()
	l := localizer
	mu.RUnlock()
	s, err := l.Localize(cfg)
	if err != nil {
		// A missing ID is a programming error; surface it rather than
		// rendering an empty string that is hard to trace.
		return "!" + cfg.MessageID
	}
	return s
}
```

- [ ] **Step 6: i18n のテストが通ることを確認する**

Run: `go test ./internal/i18n/ -v`
Expected: PASS

`language.Parse("zzz")` がエラーにならず未対応タグとして通る場合は、`matcher.Match` の `conf == language.No` で弾かれるため結果は変わらない。テストが落ちる場合はそのケースの期待値ではなく実装を疑うこと。

- [ ] **Step 7: `render.go` の文字列をカタログ参照に置き換える**

`internal/ui/render.go` を次の対応で書き換える。**英語の文言は一字も変えない**（アプリ名を除く）。

| 現在のコード | 置き換え後 |
|---|---|
| `title := "GitHub Dash"` | `title := i18n.T("app.name")` |
| `"Pull Requests", "Issues"` | `i18n.T("list.tab_prs")`, `i18n.T("list.tab_issues")` |
| `" loading...\n"` | `" " + i18n.T("common.loading") + "\n"` |
| `"No open pull requests"` | `i18n.T("list.no_open_prs")` |
| `"No open issues"` | `i18n.T("list.no_open_issues")` |
| リストのフッター文字列 | `i18n.T("footer.list")` |
| 詳細のフッター前半 | `i18n.T("footer.detail_prefix")` |
| 詳細のフッター後半 | `i18n.T("footer.detail_suffix")` |
| `"x:close  "` / `"x:reopen  "` | `i18n.T("footer.close")` / `i18n.T("footer.reopen")` |
| `"error: "` | `i18n.T("common.error_prefix")` |
| ピッカーのフッター | `i18n.T("footer.picker")` |
| `" applying...\n"` | `" " + i18n.T("picker.applying") + "\n"` |
| `"Comment on "+m.detailTitle` | `i18n.Tf("compose.title", map[string]any{"Title": m.detailTitle})` |
| `" posting...\n"` | `" " + i18n.T("compose.posting") + "\n"` |
| `"ctrl+s:send  esc:cancel"` | `i18n.T("footer.compose")` |
| `"GitHub Dash — error"` | `i18n.T("app.error_title")` |
| `"q:quit"`（エラー画面） | `i18n.T("footer.error")` |
| `" working...\n"` | `" " + i18n.T("confirm.working") + "\n"` |
| `"(y/n)"` | `i18n.T("confirm.yes_no")` |

`confirmView` の「動詞 + 名詞」の組み立ては、語順が言語によって変わるため文ごと 4 つの ID に分ける:

```go
func (m Model) confirmView() string {
	header := titleStyle.Render(m.detailTitle)
	closing, _ := m.stateAction()
	var id string
	switch {
	case m.detailTarget.Kind == KindPR && closing:
		id = "confirm.close_pr"
	case m.detailTarget.Kind == KindPR:
		id = "confirm.reopen_pr"
	case closing:
		id = "confirm.close_issue"
	default:
		id = "confirm.reopen_issue"
	}
	var b strings.Builder
	b.WriteString(header + "\n\n")
	b.WriteString(i18n.T(id))
	if m.working {
		b.WriteString(m.spin.View() + " " + i18n.T("confirm.working") + "\n")
	} else {
		b.WriteString(dimStyle.Render(i18n.T("confirm.yes_no")))
	}
	return b.String()
}
```

`m.detailTarget.Kind` は Task 2 の変更後も残っているため、上のコードはそのまま使える。

`relTime` は複数形を使う:

```go
func relTime(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return i18n.T("time.now")
	case d < time.Hour:
		return i18n.Tn("time.minutes_ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return i18n.Tn("time.hours_ago", int(d.Hours()))
	default:
		return i18n.Tn("time.days_ago", int(d.Hours()/24))
	}
}
```

Markdown のメタ項目名も置き換える:

```go
fmt.Fprintf(&b, "- **%s**: @%s\n", i18n.T("md.author"), pr.Author.Login)
```

`state += " (draft)"` は `state += i18n.T("md.draft_suffix")`、`"_no description_"` は `i18n.T("md.no_description")` にする。

- [ ] **Step 8: `ui.go` と `picker.go` の文字列を置き換える**

- `internal/ui/ui.go:134` の `ta.Placeholder = "Leave a comment..."` → `ta.Placeholder = i18n.T("compose.placeholder")`
- `internal/ui/ui.go:363` の `fmt.Sprintf("PR #%d %s", ...)` → `i18n.Tf("detail.pr_title", map[string]any{"Number": msg.Number, "Title": msg.Title})`
- `internal/ui/ui.go:372` の Issue 版 → `i18n.Tf("detail.issue_title", ...)`
- `internal/ui/ui.go:406,408` の `"Labels"` / `"Assignees"` → `i18n.T("picker.labels")` / `i18n.T("picker.assignees")`

`internal/ui/picker.go` に表示文字列があれば同様に置き換える（`grep -n '"' internal/ui/picker.go` で確認）。キーバインドの判定に使う文字列（`"j"`、`"ctrl+c"` など）は**翻訳しない**。

- [ ] **Step 9: 既存テストが変わらず通ることを確認する**

Run: `make test`
Expected: PASS。既定言語が英語で、英語の文言を変えていないため、`strings.Contains` の 27 箇所は無修正で通る。唯一 `GitHub Dash` を検査している箇所があれば `octoscope` に直す

Run: `grep -rn 'GitHub Dash' --include='*.go' . ; echo "exit=$?"`
Expected: 何も出ない（`exit=1`）

- [ ] **Step 10: 日本語表示のテストを足す**

Create `internal/ui/i18n_test.go`:

既存の `internal/ui/ui_test.go` は非公開フィールド（`m.postErr` など）を読むため `package ui` である。新しいテストも同じパッケージにしないとヘルパーが見えない。

```go
package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/language"

	"github.com/kukv/octoscope/internal/i18n"
)

// TestJapaneseListViewFitsTheWidth guards against counting runes instead of
// display columns: Japanese characters occupy two columns each.
func TestJapaneseListViewFitsTheWidth(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })
	i18n.SetLanguage(language.Japanese)

	const width = 80
	for _, line := range strings.Split(newTestModelWithData(t).View().Content, "\n") {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("line is %d columns wide, want <= %d: %q", w, width, line)
		}
	}
}
```

`newTestModelWithData` は `internal/ui/ui_test.go` にある既存のヘルパー（フェイクの `DataSource` でモデルを組み立てて `WindowSizeMsg` を送るもの）に合わせて名前を直すこと。既存テストがモデルを組み立てている箇所を読み、同じ手順を使う。

- [ ] **Step 11: 日本語テストが通ることを確認する**

Run: `go test ./internal/ui/ -run TestJapaneseListView -v`
Expected: PASS。落ちる場合は、桁を `len()` で数えている箇所が残っている

- [ ] **Step 12: `--lang` フラグを足す**

`cmd/octoscope/main.go` に追加する:

```go
import (
	// 既存に加えて
	"github.com/jeandeaual/go-locale"

	"github.com/kukv/octoscope/internal/i18n"
)

	lang := flag.String("lang", "",
		"display language: en or ja; defaults to the operating system locale")
	flag.Parse()

	osLocale, _ := locale.GetLocale() // an error here just means "unknown"
	i18n.SetLanguage(i18n.Resolve(*lang, osLocale))
```

`i18n.SetLanguage` は `ui.New` を呼ぶ**前**に実行すること。`textarea` の placeholder が構築時に決まるため、後から切り替えると英語のまま残る。

- [ ] **Step 13: 実際に日本語で起動して確認する**

Run: `go run ./cmd/octoscope --lang ja`
Expected: タブ名・フッター・相対時刻が日本語になり、桁がずれていない。`--lang en` で英語に戻る

- [ ] **Step 14: `make check` とコミット**

```bash
make check
git add -A
git commit -m "feat: localize the interface in English and Japanese

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: GoReleaser とリリースワークフロー

**Files:**
- Create: `.goreleaser.yaml`
- Create: `.github/workflows/release.yaml`
- Modify: `cmd/octoscope/main.go`（`--version`）
- Modify: `.gitignore`（`dist/`）
- Modify: `Makefile`（`release-check` ターゲット）

**Interfaces:**
- Consumes: `cmd/octoscope`（Task 2〜4）
- Produces: タグ `vX.Y.Z` を push すると 3 OS × 2 アーキテクチャのアーカイブが Releases に並ぶ

- [ ] **Step 1: バージョン変数と `--version` を足す**

`cmd/octoscope/main.go`:

```go
// version is set by GoReleaser via -ldflags at release build time.
var version = "dev"
```

`flag.Parse()` の直後に:

```go
	if *showVersion {
		fmt.Println("octoscope " + version)
		return
	}
```

フラグの宣言:

```go
	showVersion := flag.Bool("version", false, "print the version and exit")
```

- [ ] **Step 2: 動作を確認する**

Run: `go run ./cmd/octoscope --version`
Expected: `octoscope dev`

Run: `go run -ldflags "-X main.version=1.2.3" ./cmd/octoscope --version`
Expected: `octoscope 1.2.3`

- [ ] **Step 3: GoReleaser 設定を書く**

Create `.goreleaser.yaml`:

```yaml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - id: octoscope
    main: ./cmd/octoscope
    binary: octoscope
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    files:
      - README.md
      - LICENSE*

checksum:
  name_template: checksums.txt

changelog:
  use: github
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore\\(deps\\):"
```

- [ ] **Step 4: LICENSE があることを確認する**

Run: `head -3 LICENSE`
Expected: `MIT License` と `Copyright (c) 2026 kukv`

MIT ライセンスは計画作成時に追加済み。`.goreleaser.yaml` の `files:` の `LICENSE*` はこれを同梱する。無ければ `gh api /licenses/mit --jq .body > LICENSE` で作り、`[year]` と `[fullname]` を埋める。

- [ ] **Step 5: 設定とクロスコンパイルを検証する**

GoReleaser が入っているか確認する。

Run: `goreleaser --version || echo "install goreleaser first"`

入っていない場合は `mise` か `go install github.com/goreleaser/goreleaser/v2@latest` で入れる。

Run: `goreleaser check`
Expected: `1 configuration file(s) validated`

Run: `GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/octoscope && GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/octoscope && GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/octoscope`
Expected: 3 つともエラーなし

- [ ] **Step 6: `dist/` を無視する**

`.gitignore` の末尾に `dist/` を足す。

- [ ] **Step 7: `Makefile` に検証ターゲットを足す**

```makefile
# Validate the release configuration and cross-compilation (mirrors release CI).
release-check:
	goreleaser check
	GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/octoscope
	GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/octoscope
	GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/octoscope
```

`.PHONY` の行にも `release-check` を足すこと。

- [ ] **Step 8: リリースワークフローの action SHA を取得する**

action はタグではなく full-length commit SHA でピンする必要がある（Global Constraints）。SHA を記憶で書かず、必ず取得すること。

```bash
for ref in actions/checkout:v7.0.1 actions/setup-go:v7.0.0 goreleaser/goreleaser-action:v6.4.0; do
  repo="${ref%%:*}"; tag="${ref##*:}"
  obj=$(gh api "repos/$repo/git/ref/tags/$tag" --jq '.object.sha + " " + .object.type')
  sha="${obj%% *}"; type="${obj##* }"
  if [ "$type" = "tag" ]; then sha=$(gh api "repos/$repo/git/tags/$sha" --jq '.object.sha'); fi
  echo "$repo@$sha # $tag"
done
```

`actions/checkout` と `actions/setup-go` は `.github/workflows/ci.yaml` に既にピン済みの SHA があるため、そちらと一致することを確認する。`goreleaser-action` の最新タグは上のコマンドで拒否されたら `gh api repos/goreleaser/goreleaser-action/releases/latest --jq .tag_name` で調べ直す。

- [ ] **Step 9: リリースワークフローを書く**

Create `.github/workflows/release.yaml`。`<SHA>` は Step 7 で取得した値に置き換える。

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: read

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          fetch-depth: 0
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
          cache: false
      - uses: goreleaser/goreleaser-action@<SHA> # <TAG>
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

`fetch-depth: 0` は GoReleaser が changelog を作るために必要。`persist-credentials: false` は既存ワークフローと揃える（GoReleaser は `GITHUB_TOKEN` を env から読む）。

- [ ] **Step 10: 検証**

Run: `make release-check`
Expected: 通る

Run: `gh workflow view release.yaml 2>/dev/null || echo "not pushed yet, fine"`

ワークフロー本体の実行はタグを push したときのみ。Phase 0 の完了時点では push しない（リポジトリのリネームが先）。

- [ ] **Step 11: `make check` とコミット**

```bash
make check
git add -A
git commit -m "build: release cross-platform binaries with GoReleaser

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: ドキュメントの書き換え

**Files:**
- Modify: `README.md`（全面書き換え）

**Interfaces:**
- Consumes: Task 2〜5 で確定した CLI のフラグとインストール手順
- Produces: なし

- [ ] **Step 1: 旧 docs が削除済みであることを確認する**

Run: `ls -R docs`
Expected: `docs/superpowers/specs/2026-09-05-octoscope-standalone-design.md` と `docs/superpowers/plans/2026-09-06-octoscope-phase0.md` の 2 つだけ

`docs/plugin-development-notes.md` と Herdr 時代の spec / plan は計画作成時に削除済み。残っていたら削除する。

- [ ] **Step 2: README を書き換える**

`README.md` を次の構成にする。Herdr への言及、`herdr plugin install`、キーバインド設定、Ctrl+click のトラブルシューティングはすべて削除する。

```markdown
# octoscope

A terminal dashboard for GitHub pull requests and issues.

octoscope = **Octo**cat + **-scope**: a telescope for looking over your GitHub work.

## Requirements

- [GitHub CLI](https://cli.github.com/) (`gh`), authenticated via `gh auth login`

## Install

Download a binary for your platform from the
[releases page](https://github.com/kukv/octoscope/releases), or build from source:

    go install github.com/kukv/octoscope/cmd/octoscope@latest

## Usage

Run it inside a git repository:

    octoscope

Or point it at any repository:

    octoscope --repo kukv/octoscope

### Flags

| Flag | Description |
|---|---|
| `--repo owner/name` | Target repository. Defaults to the repository of the current directory. |
| `--lang en\|ja` | Display language. Defaults to the operating system locale. |
| `--version` | Print the version and exit. |

### Keys

（既存の README のキー表をそのまま使う）

## Localization

octoscope speaks English and Japanese. The language is chosen from `--lang`
first, then the operating system locale, and falls back to English.
```

`## Troubleshooting` の節はすべて Herdr 固有のため削除する。デモ動画の埋め込みリンクは、現在の画面と乖離するため Phase 1 で撮り直すまでコメントアウトするか削除する（どちらでもよい。判断してコミットメッセージに書く）。

- [ ] **Step 3: リンク切れが無いことを確認する**

Run: `grep -n 'herdr\|Herdr\|HERDR' README.md ; echo "exit=$?"`
Expected: 何も出ない（`exit=1`）

- [ ] **Step 4: `make check` とコミット**

```bash
make check
git add README.md
git commit -m "docs: rewrite README for the standalone octoscope CLI

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: リポジトリのリネーム（ユーザーが実施する手動作業）

**このタスクは自動実行しない。** ワークトリー隔離のため、このセッションからは `structure` リポジトリの git 操作も、ghq のディレクトリ移動もできない。Task 1〜6 の PR がマージできる状態になったら、以下をユーザーに提示して実施してもらう。

**実施順序が重要**: GitHub 上のリネームを先に行う。Go のモジュールプロキシは `github.com/kukv/octoscope` が存在するまで 404 を返す。旧 URL は GitHub 側で自動的にリダイレクトされるため、先にリネームしても既存のクローンや PR は壊れない。

- [ ] **Step 1: GitHub 上でリポジトリ名を変更する**

```bash
gh repo rename octoscope --repo kukv/herdr-plugin-github-dash
```

- [ ] **Step 2: `structure` リポジトリの Terraform を追従させる**

`~/dev/ghq/github.com/kukv/structure` で作業する（このワークトリーからは実行できない）。

1. `terraform/repository_herdr-plugin-github-dash.tf` を `terraform/repository_octoscope.tf` にリネームする
2. 中身を次のようにする:

```hcl
moved {
  from = module.repository_herdr_plugin_github_dash
  to   = module.repository_octoscope
}

module "repository_octoscope" {
  source = "./modules/repository"

  name        = "octoscope"
  visibility  = "public"
  description = "A terminal dashboard for GitHub pull requests and issues"
  topics      = ["github", "cli", "tui", "golang"]
}
```

3. `terraform plan` を実行し、**`~ update in-place` であること**を確認する。`-/+ destroy and then create` になっていたら適用しない。モジュールに `prevent_destroy` は無いが `archive_on_destroy = true` が設定されているため、誤って適用するとリポジトリがアーカイブされる
4. PR を出してマージする

- [ ] **Step 3: ローカルの ghq ディレクトリを整える**

`~/dev/ghq/github.com/kukv/herdr-plugin-github-dash` はディレクトリ名が実態と合わなくなる。次のどちらかを行う。

```bash
# A: 取り直す
ghq get github.com/kukv/octoscope

# B: 移動して remote を張り替える
mv ~/dev/ghq/github.com/kukv/herdr-plugin-github-dash ~/dev/ghq/github.com/kukv/octoscope
cd ~/dev/ghq/github.com/kukv/octoscope
git remote set-url origin git@github.com:kukv/octoscope.git
```

B を選ぶ場合、このワークトリー（`.claude/worktrees/`）を含むため、作業中の Claude セッションを終えてから行うこと。

- [ ] **Step 4: 最初のリリースを出す**

```bash
git tag v0.1.0
git push origin v0.1.0
```

Expected: Release ワークフローが走り、6 つのアーカイブと `checksums.txt` が Releases に並ぶ。

- [ ] **Step 5: 記憶の更新**

`herdr-plugin-testing` のメモリは Herdr プラグインの起動経路を前提にしており、Phase 0 完了時点で内容が古くなる。削除するか、「octoscope は通常のコマンドとして起動する」旨に更新する。

---

## Self-Review

**Spec coverage（Phase 0 の項目、spec §7）**

| spec の項目 | 対応タスク |
|---|---|
| GitHub 上でリポジトリ名を変更 | Task 7 Step 1 |
| `structure` の Terraform を追従（§9） | Task 7 Step 2 |
| `go.mod` と全 import の更新 | Task 1 |
| Herdr 資産の削除 | Task 2 |
| `main.go` を `cmd/octoscope/` へ | Task 2 |
| `--repo` と git remote による解決 | Task 3 |
| `internal/i18n` の土台と全文字列の移行（§6） | Task 4 |
| GoReleaser とリリースワークフロー | Task 5 |
| README の全面書き換え | Task 6 |
| 検証: 3 OS のクロスコンパイル | Task 5 Step 5 |
| 検証: 既存ビューが従来どおり動く | Task 2 Step 8、Task 3 Step 7 |
| 検証: `--lang ja` / `--lang en` の両方でテストが通る | Task 4 Step 11 |

**型の整合**

- `ghcli.New(dir, repo string) *Client` — Task 3 で定義し、Task 3 Step 6 と Task 4 Step 12 の `main.go` で同じ形で呼んでいる
- `ui.New(ds DataSource) Model` — Task 2 で定義し、Task 3・Task 4 の `main.go` で同じ形
- `i18n.T` / `Tf` / `Tn` / `Resolve` / `SetLanguage` / `IDs` — Task 4 の Interfaces、実装（Step 5）、テスト（Step 2）、利用箇所（Step 7〜8、12）ですべて同じ名前・同じ引数
- `i18n.Tf` の第 2 引数は一貫して `map[string]any`
- `Tn` のテンプレート変数は `.Count`。カタログ（Step 4）と実装（Step 5）で一致

**未確定として残している点**（実装時に判断が要る）

- Task 4 Step 10 の `newTestModelWithData`: 既存テストヘルパーの実名に合わせる
- Task 5 Step 8 の goreleaser-action の SHA: 取得コマンドを実行して埋める
