# octoscope Phase 1: Work タブと GraphQL バックエンド 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 1 リポジトリの PR / Issue 一覧しか出せない現在の画面に、複数リポジトリを横断して「自分に関係する仕事」を 4 列のカンバンで見せる Work タブを足し、タブ切替を持つルートモデルへ再構成する。

**Architecture:** GitHub アクセス層を `internal/gh`（ドメイン型のみ）と `internal/gh/cli`（`gh` コマンドを実行する実装）へ分ける。横断取得は `gh api graphql` の `search(type: ISSUE)` を 4 つのエイリアスで 1 リクエストにまとめる。TUI は `internal/ui` の 1 枚モデルを `internal/tui/app`（ルート・タブ切替）、`internal/tui/work`（カンバン）、`internal/tui/repo`（既存の PR/Issue 一覧）、`internal/tui/detail`（既存の詳細・コメント・ピッカー）へ分割し、サブモデルは自分の状態だけを持ってメッセージ型でのみ親と話す。

**Tech Stack:** Go 1.27 / Bubble Tea v2 (`charm.land/bubbletea/v2`) / lipgloss v2 / `charm.land/bubbles/v2` / `charm.land/glamour/v2` / `nicksnyder/go-i18n/v2` / `github.com/charmbracelet/x/ansi` / golangci-lint v2.13.2

**Spec:** `docs/superpowers/specs/2026-09-05-octoscope-standalone-design.md`

**前フェーズの計画:** `docs/superpowers/plans/2026-09-06-octoscope-phase0.md`

## Global Constraints

- Go モジュールパス: `github.com/kukv/octoscope`。バイナリ名: `octoscope`
- charmbracelet の TUI v2 は `charm.land/*/v2` が正しい import パス。`github.com/charmbracelet/*/v2` は使わない。`github.com/charmbracelet/x/ansi` は `github.com/` のままで正しい
- 表示幅の計算は `github.com/charmbracelet/x/ansi` の `ansi.StringWidth` / `ansi.Truncate` を使う。`len()` や `utf8.RuneCountInString()` で桁を数えない
- 画面に出す文字列は `internal/i18n` から引く。Go コードに英語も日本語も直書きしない。**文字列を足すときは `active.en.yaml` と `active.ja.yaml` の両方に足す**（カタログの形式は YAML。Phase 0 計画の「TOML」という記述は実装時に YAML へ変わっている）
- カバレッジ基準は 80%（`.octocov.yml`）。下回ると CI が赤くなる
- ネットワークも外部プロセスも実際には叩かない。差し替えは `Client.run` フィールド（関数型）で行う
- **各タスクの完了時に `make check`（tidy-check / lint / fmt-check / test）が緑であること。** 途中でビルドが壊れるタスク分割にしない
- `//nolint` を新たに足さない。必要になったら `.golangci.yml` の `exclusions` に理由つきで書く
- **パッケージを増やしたら、その場で `.golangci.yml` の `depguard` にも足す**
- GitHub Actions の `uses:` は full-length commit SHA でピンする（org ポリシー）。ただし本 Phase でワークフローを触る予定は無い
- コーディング規約は `CLAUDE.md` と `.claude/rules/` にある。実装前に読むこと

### ルートモデルの `View()` の型

Bubble Tea v2 では `tea.Program` に渡すモデルの `View()` は **`tea.View` を返す**（既存の
`internal/ui/ui.go:697` がその形で、テストは `m.View().Content` を見ている）。

- `internal/tui/app.Model.View()` は `tea.View` を返す。テストは `.Content` を見る
- サブモデル（`work` / `repo` / `detail`）の `View()` は **`string` を返す**。親が組み立てて `tea.View` にする

### Phase 1 のスコープ外（勝手にやらないこと）

- Repos タブのサイドバー、リポジトリ一覧の追加/削除ダイアログ、Search タブ（Phase 4）
- 設定ファイルの読み込み（Phase 4）。言語決定順は「`--lang` → OS ロケール → en」のまま
- diff ビューア、レビュー提出、行コメント（Phase 2）
- checks の一覧画面、merge 操作（Phase 3）。Phase 1 が扱う checks は**カンバンのカードに出す進捗バーの集計値だけ**
- `internal/gh/api`（go-github / githubv4 バックエンド）（Phase 4）
- Nerd Font の判定。Phase 1 の装飾は既存と同じく Unicode 記号（`▰▱◌✓×•`）で行い、グリフ判定は入れない
- スパークライン、ラベルの実色バッジ（spec §4.5 の残り）

### 本計画で spec から意図的に逸れる点

**spec §3.2 は「`internal/gh` に `Client` interface を定義」と書いているが、本計画では `internal/gh` に interface を置かない。**

`.claude/rules/architecture.md` の「interface は利用側で定義する」「GitHub アクセス層は interface を export しない」に従い、
`internal/gh` はドメイン型のみ、`internal/gh/cli` は具体型 `cli.Client` のみを公開し、
必要な操作の interface は各サブモデルのファイルで宣言する。
バックエンドの差し替え（Phase 4 の `api` 実装）は、**利用側 interface を両実装が満たす**形で達成できるため、
共通 interface を下位層に置く必要はない。Task 12 で spec §3.2 をこの方針に合わせて更新する。

### 本計画で置く前提

**Phase 1 のタブは 2 つ（`1`: Work / `2`: Repos）とする。** spec §4 は Work / Repos / Search の 3 タブだが、
Search タブは Phase 4 のため存在しない。既存の「1 リポジトリの PR/Issue 一覧」は spec §4.2 の Repos タブ右ペインそのものなので、
これを Repos タブとして据え、サイドバー（リポジトリ一覧・追加ダイアログ）は Phase 4 で左に足す。
`3` キーは Phase 1 では何も起こらない（未定義キーとして無視する）。

---

## PR 分割

各 PR は単独で `make check` が緑になり、`go run ./cmd/octoscope` が動く状態で切る。

| PR | 内容 | タスク | ブランチ | ラベル |
|---|---|---|---|---|
| 1 | 本実装計画のドキュメント | — | `docs/octoscope-phase1-plan` | `Kind: Documentation` |
| 2 | 翻訳漏れ検出ガード、日時書式のカタログ化 | Task 1–2 | `feat/octoscope-phase1-i18n` | `Kind: Tests`, `Kind: Enhancement` |
| 3 | `internal/gh` への移設と利用側 interface、depguard | Task 3–4 | `refactor/octoscope-phase1-gh-layer` | `Kind: Refactoring` |
| 4 | GraphQL 横断取得、Work タブ、ルートモデル | Task 5–12 | `feat/octoscope-phase1-work-tab` | `Kind: Feature` |

PR 2 を先に出すのは、**ガードが先に無いと Phase 1 で足す大量の文字列のタイポを誰も検出できない**ため。

各 PR の作成時は `gh pr create --label` で上表のラベルを付ける。
コミットメッセージの末尾に `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`、
PR 本文の末尾に `🤖 Generated with [Claude Code](https://claude.com/claude-code)` を入れる。

### PR 4 の中のタスク順

`internal/ui` を壊さないために、**下から積む**。各タスクの末尾で `make check` が緑になる。

```
Task 5  gh のドメイン型（Work / ItemRef / enum）      internal/ui に影響なし
Task 6  cli.ListWork（GraphQL）                       既存メソッドの signature は変えない
Task 7  i18n.RelTime と internal/tui/icon             internal/ui は RelTime を使う形へ
Task 8  internal/tui/work（まだ誰も使わない独立パッケージ）
Task 9  internal/ui → internal/tui/{repo,detail}、読み取りに ctx を通す
Task 10 internal/tui/app と main の配線、internal/ui を削除
Task 11 日本語と幅の通し確認
Task 12 spec の更新
```

---

## File Structure

### 最終形（Phase 1 完了時点）

| ファイル | 責務 |
|---|---|
| `cmd/octoscope/main.go` | フラグ解析、言語決定、`cli.Client` の組み立て、ルートモデルの起動 |
| `internal/i18n/i18n.go` | カタログ読み込み、`T` / `Tf` / `Tn` / `DateTime` / `RelTime`、言語決定 |
| `internal/i18n/locales/active.{en,ja}.yaml` | メッセージカタログ |
| `internal/i18n/unresolved.go` | 未解決 ID（`!` 前置き）の検出と、それを使うテストヘルパー |
| `internal/gh/gh.go` | ドメイン型（`PR` / `Issue` / `Label` / `Comment` / `ItemRef` / `ReviewState` / `CheckState` / `WorkItem` / `Work`） |
| `internal/gh/cli/cli.go` | `gh` コマンドの実行と JSON パース（既存 `internal/ghcli` の移設） |
| `internal/gh/cli/graphql.go` | `gh api graphql` による横断取得のクエリ組み立てとレスポンス変換 |
| `internal/tui/icon/icon.go` | 状態アイコンと checks 進捗バー（spec §4.5） |
| `internal/tui/app/app.go` | ルートモデル。タブ切替、ウィンドウサイズ配布 |
| `internal/tui/app/render.go` | タブ行とエラー画面の描画 |
| `internal/tui/work/work.go` | Work タブの状態・`Update`・データ取得 |
| `internal/tui/work/render.go` | カンバンの描画（列・カード・ドロワー・幅の劣化） |
| `internal/tui/repo/repo.go` | 既存 `internal/ui/ui.go` の一覧部分 |
| `internal/tui/repo/render.go` | 既存 `internal/ui/render.go` の一覧部分 |
| `internal/tui/detail/detail.go` | 既存 `internal/ui/ui.go` の詳細・コメント・状態変更部分 |
| `internal/tui/detail/render.go` | 既存 `internal/ui/render.go` の詳細部分 |
| `internal/tui/detail/picker.go` | 既存 `internal/ui/picker.go` |

**削除するもの:** `internal/ghcli/`、`internal/ui/`

---

## Task 1: 翻訳漏れ検出ガード

spec §6.5。Phase 0 の `TestNoUnresolvedIDsInEitherCatalog` は `i18n.IDs()`（カタログ自身の ID 一覧）を回しているため、
**カタログにある ID がカタログから引けること**しか見ておらず、構造上ほぼ失敗しない。
コード側で `i18n.T("work.reveiw_requested")` とタイポしても検出できない。
ビューのレンダリング結果を走査して `!` 前置きの未解決 ID が混じっていないことを見る形に置き換える。

**Files:**
- Create: `internal/i18n/unresolved.go`
- Create: `internal/i18n/unresolved_test.go`
- Modify: `internal/ui/i18n_test.go`（`TestNoUnresolvedIDsInEitherCatalog` を置き換え）

**Interfaces:**
- Consumes: `i18n.T` / `i18n.SetLanguage`（既存）
- Produces:
  - `func i18n.UnresolvedIDs(rendered string) []string`
  - `func i18n.AssertNoUnresolvedIDs(t TestingT, rendered string)`
  - `type i18n.TestingT interface { Helper(); Errorf(string, ...any) }`

`internal/i18n` は他の internal パッケージに依存できない（depguard）ので、
ヘルパーは `testing.T` を直接取らず最小 interface を取る。
本番バイナリに載るファイルだが、依存は標準ライブラリの `regexp` だけで済む。

- [ ] **Step 1: 失敗するテストを書く**

`internal/i18n/unresolved_test.go`:

```go
package i18n_test

import (
	"testing"

	"github.com/kukv/octoscope/internal/i18n"
)

func TestUnresolvedIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rendered string
		want     []string
	}{
		{
			name:     "clean render has none",
			rendered: "Pull Requests  Issues\n  #12 fix the thing  @kukv",
			want:     nil,
		},
		{
			name:     "typo in a message ID leaks as !id",
			rendered: "!work.reveiw_requested  Issues",
			want:     []string{"work.reveiw_requested"},
		},
		{
			name:     "several on one line",
			rendered: "!a.b !c.d_e",
			want:     []string{"a.b", "c.d_e"},
		},
		{
			name:     "an exclamation mark in prose is not an ID",
			rendered: "done! and no.dots after a space",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := i18n.UnresolvedIDs(tt.rendered)
			if len(got) != len(tt.want) {
				t.Fatalf("%s: got %v, want %v", tt.name, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("%s: got[%d] = %q, want %q", tt.name, i, got[i], tt.want[i])
				}
			}
		})
	}
}
```

- [ ] **Step 2: 落ちることを確認する**

Run: `go test ./internal/i18n/ -run TestUnresolvedIDs -v`

Expected: FAIL（`undefined: i18n.UnresolvedIDs`）

- [ ] **Step 3: 実装する**

`internal/i18n/unresolved.go`:

```go
package i18n

import "regexp"

// unresolvedPattern matches what render() emits for a missing message ID:
// "!" immediately followed by a dotted identifier. Message IDs always have at
// least one dot ("work.review_requested"), which keeps ordinary prose ending
// in "!" from matching.
var unresolvedPattern = regexp.MustCompile(`!([a-z0-9_]+(?:\.[a-z0-9_]+)+)`)

// UnresolvedIDs returns the message IDs that failed to resolve in a rendered
// view, in the order they appear. A non-empty result means the code asked for
// an ID that is missing from the active catalog.
func UnresolvedIDs(rendered string) []string {
	matches := unresolvedPattern.FindAllStringSubmatch(rendered, -1)
	if len(matches) == 0 {
		return nil
	}
	ids := make([]string, len(matches))
	for i, m := range matches {
		ids[i] = m[1]
	}
	return ids
}

// TestingT is the part of *testing.T that AssertNoUnresolvedIDs needs.
// internal/i18n must not depend on other packages of ours, and taking the
// interface keeps this file to the standard library.
type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
}

// AssertNoUnresolvedIDs fails t when rendered contains a message ID that is
// missing from the active catalog.
func AssertNoUnresolvedIDs(t TestingT, rendered string) {
	t.Helper()
	for _, id := range UnresolvedIDs(rendered) {
		t.Errorf("unresolved message ID %q in the rendered view", id)
	}
}
```

- [ ] **Step 4: 通ることを確認する**

Run: `go test ./internal/i18n/ -run TestUnresolvedIDs -v`

Expected: PASS

- [ ] **Step 5: 既存のカタログ一巡テストを、描画走査に置き換える**

`internal/ui/i18n_test.go` の `TestNoUnresolvedIDsInEitherCatalog` を丸ごと次で置き換える。
使うヘルパーは `internal/ui/ui_test.go` にある既存のもの:
`fakeSource`（フィールド `prs` / `issues` / `pr` / `labels`）、`samplePRs()`、
`key(s string) tea.KeyMsg`、`loadedModel(f *fakeSource) Model`、`detailModel(f *fakeSource) Model`。

```go
// TestNoUnresolvedIDsInRenderedViews guards spec §6.5. It renders each screen
// in both languages and fails when a message ID the code asked for is missing
// from that language's catalog. Walking i18n.IDs() cannot catch this: it only
// proves the catalog can resolve its own IDs, never that the IDs the code
// spells match them.
func TestNoUnresolvedIDsInRenderedViews(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })

	for _, lang := range []language.Tag{language.English, language.Japanese} {
		i18n.SetLanguage(lang)
		for name, view := range renderEveryScreen() {
			t.Run(lang.String()+"/"+name, func(t *testing.T) {
				i18n.AssertNoUnresolvedIDs(t, view)
			})
		}
	}
}

// step applies one key to m and feeds back whatever command it produced, so
// screens that need a round trip (the list tab, the label picker) settle.
func step(m Model, k string) Model {
	next, cmd := m.Update(key(k))
	m = next.(Model)
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	next, _ = m.Update(msg)
	return next.(Model)
}

// renderEveryScreen renders every screen the ui package can show, so the
// message IDs on each path are exercised at least once.
func renderEveryScreen() map[string]string {
	f := &fakeSource{
		prs:    samplePRs(),
		issues: []ghcli.Issue{{Number: 3, Title: "an issue"}},
		// State must be OPEN or the confirm screen has nothing to offer.
		pr:     ghcli.PR{Number: 1, Title: "first pr", State: "OPEN"},
		labels: []ghcli.Label{{Name: "bug", Color: "ff0000"}},
	}

	list := loadedModel(f)
	detail := detailModel(f)

	return map[string]string{
		"list_prs":    list.View().Content,
		"list_issues": step(list, "tab").View().Content,
		"detail":      detail.View().Content,
		"compose":     step(detail, "c").View().Content,
		"confirm":     step(detail, "x").View().Content,
		"picker":      step(detail, "l").View().Content,
		"error":       errorView("boom"),
	}
}
```

`ghcli` の import が `i18n_test.go` に無ければ足す（Task 3 のあとは `gh`）。

- [ ] **Step 6: ガードが空振りしていないことを確認する**

`internal/ui/render.go` の `i18n.T("footer.list")` を `i18n.T("footer.lst")` に**一時的に**書き換えて、

Run: `go test ./internal/ui/ -run TestNoUnresolvedIDsInRenderedViews`

Expected: FAIL（`unresolved message ID "footer.lst"`）。en / ja の両方で落ちること。

さらに `internal/ui/render.go` の `i18n.T("picker.applying")` を
`i18n.T("picker.applyng")` に一時的に書き換えて、`picker` の画面でも落ちることを見る
（`renderEveryScreen` がその画面に到達できている証拠になる）。

確認したら両方とも元に戻し、もう一度 PASS することを見る。

- [ ] **Step 7: `make check` を通す**

Run: `make check`

Expected: 緑

- [ ] **Step 8: コミット**

`internal/i18n/unresolved.go`、`internal/i18n/unresolved_test.go`、`internal/ui/i18n_test.go` を
add してコミットする。メッセージ:

```
test: catch mistyped message IDs by scanning rendered views

The old guard walked i18n.IDs() and asked the catalog for its own IDs, so it
could never fail on a typo in the code. Scan each rendered screen for the "!id"
marker render() emits instead.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

---

## Task 2: 日時書式のカタログ化

spec §6.1 の「日時 — ロケールに応じた書式」。
現在 `render.go` は `updatedAt.Format("2006-01-02 15:04")` と Go のレイアウトを直書きしている。
これを言語ごとのレイアウトとしてカタログへ移す。

**Files:**
- Modify: `internal/i18n/i18n.go`
- Modify: `internal/i18n/locales/active.en.yaml`、`active.ja.yaml`
- Modify: `internal/ui/render.go`（`writeCommonMeta`、`writeComments`）
- Test: `internal/i18n/i18n_test.go`

**Interfaces:**
- Consumes: `i18n.T`
- Produces: `func i18n.DateTime(t time.Time) string`

- [ ] **Step 1: 失敗するテストを書く**

`internal/i18n/i18n_test.go` に足す:

```go
func TestDateTimeUsesThePerLanguageLayout(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })

	when := time.Date(2026, 9, 6, 14, 5, 0, 0, time.UTC)

	tests := []struct {
		lang language.Tag
		want string
	}{
		{language.English, "Sep 6, 2026 14:05"},
		{language.Japanese, "2026年9月6日 14:05"},
	}

	for _, tt := range tests {
		i18n.SetLanguage(tt.lang)
		if got := i18n.DateTime(when); got != tt.want {
			t.Errorf("lang %s: got %q, want %q", tt.lang, got, tt.want)
		}
	}
}
```

言語はプロセス全体の状態なので、このテストは `t.Parallel()` にしない（`.claude/rules/testing.md`）。

- [ ] **Step 2: 落ちることを確認する**

Run: `go test ./internal/i18n/ -run TestDateTimeUsesThePerLanguageLayout -v`

Expected: FAIL（`undefined: i18n.DateTime`）

- [ ] **Step 3: カタログへレイアウトを足す**

`internal/i18n/locales/active.en.yaml` の `time:` ブロックに:

```yaml
  # A Go reference-time layout, not a display string: the field order differs
  # by language, so it belongs in the catalog rather than in the code.
  datetime_layout:
    other: "Jan 2, 2006 15:04"
```

`internal/i18n/locales/active.ja.yaml` の `time:` ブロックに:

```yaml
  # A Go reference-time layout, not a display string.
  datetime_layout:
    other: "2006年1月2日 15:04"
```

- [ ] **Step 4: `DateTime` を実装する**

`internal/i18n/i18n.go` に足す（`time` を import する）:

```go
// DateTime formats t with the active language's layout. The catalog holds a
// Go reference-time layout ("Jan 2, 2006 15:04"), not a display string,
// because the field order differs by language.
func DateTime(t time.Time) string {
	return t.Format(T("time.datetime_layout"))
}
```

- [ ] **Step 5: 通ることを確認する**

Run: `go test ./internal/i18n/ -run TestDateTimeUsesThePerLanguageLayout -v`

Expected: PASS

- [ ] **Step 6: 呼び出し側を差し替える**

`internal/ui/render.go` の `writeCommonMeta`:

```go
	fmt.Fprintf(b, "- **%s**: %s\n", i18n.T("md.updated"), i18n.DateTime(updatedAt))
```

`writeComments`:

```go
		fmt.Fprintf(b, "\n\n---\n\n**@%s** — %s\n\n%s",
			c.Author.Login, i18n.DateTime(c.CreatedAt), c.Body)
```

`render.go` の `time` の import は `relTime` がまだ使うので残る。

- [ ] **Step 7: `make check` を通す**

Run: `make check`

Expected: 緑。`internal/ui` の既存テストが `2006-01-02 15:04` 形式を期待している箇所は、
英語カタログの形式（`Sep 6, 2026 14:05`）へ直す。

- [ ] **Step 8: コミット**

```
feat: format dates with a per-language layout

Spec §6.1 lists dates among the strings this tool writes itself. Move the Go
layout into the catalog so ja renders 2026年9月6日 rather than the ISO shape.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

- [ ] **Step 9: PR 2 を出す**

`git push -u origin feat/octoscope-phase1-i18n` のあと:

```
gh pr create \
  --label "Kind: Tests" --label "Kind: Enhancement" \
  --title "feat: guard against mistyped message IDs and localize date formats"
```

本文:

```
Phase 1 の基盤 2 つ。

- 翻訳漏れ検出ガードを、カタログ一巡から**描画結果の走査**へ置き換えた（spec §6.5）。
  従来のテストは `i18n.IDs()` を回してカタログ自身の ID を引いていたため、
  コード側のタイポを構造上検出できなかった。
- 日時書式を言語ごとのレイアウトとしてカタログへ移した（spec §6.1）。

Plan: `docs/superpowers/plans/2026-09-06-octoscope-phase1.md` Task 1–2

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

---

## Task 3: `internal/ghcli` を `internal/gh` + `internal/gh/cli` へ移設

spec §3.1。ドメイン型を `internal/gh` に、`gh` コマンドの実行を `internal/gh/cli` に分ける。
**この時点では挙動を変えない。** 型の置き場所と import パスだけが変わる。

**Files:**
- Create: `internal/gh/gh.go`
- Create: `internal/gh/cli/cli.go`（`internal/ghcli/ghcli.go` から移設）
- Create: `internal/gh/cli/cli_test.go`（`internal/ghcli/ghcli_test.go` から移設）
- Delete: `internal/ghcli/`
- Modify: `internal/ui/*.go`、`internal/ui/*_test.go`、`cmd/octoscope/main.go`（import のみ）
- Modify: `.golangci.yml`

**Interfaces:**
- Consumes: なし
- Produces:
  - `package gh`: 型 `Author`、`Label`、`Comment`、`PR`、`Issue`。センチネル `gh.ErrGhNotFound`
  - `package cli`: `func New(dir, repo string) *Client` と既存の全メソッド。戻り値は `gh.PR` / `gh.Issue` / `gh.Label`

- [ ] **Step 1: ドメイン型を `internal/gh` へ移す**

`internal/gh/gh.go` を作る。`internal/ghcli/ghcli.go` の型宣言と `ErrGhNotFound` をそのまま移す。
JSON タグも変えない。

```go
// Package gh holds the domain types the GitHub access layer returns.
// It has no behaviour: the backends live in subpackages (cli, and api in a
// later phase) and both speak these types.
package gh

import (
	"errors"
	"time"
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
	Assignees      []Author  `json:"assignees"`
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
	Assignees []Author  `json:"assignees"`
}
```

`PR.ReviewDecision` と `PR.State` の GitHub 文字列は Task 5 と Task 9 でドメイン値に変換する。
このタスクでは触らない。

- [ ] **Step 2: `internal/gh/cli` へ実装を移す**

`mkdir -p internal/gh/cli` のあと `git mv` で 2 ファイルを移し、`internal/ghcli` を消す。

- `internal/ghcli/ghcli.go` → `internal/gh/cli/cli.go`
- `internal/ghcli/ghcli_test.go` → `internal/gh/cli/cli_test.go`

`internal/gh/cli/cli.go` を直す:

- パッケージ宣言を `package cli`、doc コメントを
  `// Package cli fetches GitHub data by running the gh CLI in a target directory.` にする
- 型宣言と `ErrGhNotFound` を削除し、`"github.com/kukv/octoscope/internal/gh"` を import する
- 戻り値の型を `gh.PR` / `gh.Issue` / `gh.Label` / `gh.Author` に書き換える
- `runGh` の中の `ErrGhNotFound` を `gh.ErrGhNotFound` にする

`internal/gh/cli/cli_test.go` も `package cli` にして、期待値の型を `gh.` 付きへ直す。

- [ ] **Step 3: 利用側の import を直す**

`internal/ui/*.go`、`internal/ui/*_test.go`、`cmd/octoscope/main.go`:

- `"github.com/kukv/octoscope/internal/ghcli"` → `"github.com/kukv/octoscope/internal/gh"`
- `main.go` はさらに `"github.com/kukv/octoscope/internal/gh/cli"` を import する
- `ghcli.PR` などの `ghcli.` をすべて `gh.` に
- `main.go` の `ghcli.New(dir, *repo)` → `cli.New(dir, *repo)`

- [ ] **Step 4: depguard を更新する**

`.golangci.yml`:

- `gh-layer` の `files` から `"**/internal/ghcli/**"` を削る（`"**/internal/gh/**"` が `internal/gh/cli` も含む）
- `i18n-layer` の `deny` から `github.com/kukv/octoscope/internal/ghcli` を削る
- `exclusions` の gosec G204 の `path` を `internal/ghcli/ghcli\.go` から `internal/gh/cli/cli\.go` へ直し、
  コメント中の `internal/ghcli` という記述も `internal/gh/cli` に直す

- [ ] **Step 5: テストが通ることを確認する**

Run: `make check`

Expected: 緑。`internal/gh/cli` のテストは移設前と同じ本数が通ること。

Run: `go run ./cmd/octoscope --version`

Expected: `octoscope dev`

- [ ] **Step 6: コミット**

```
refactor: split the GitHub layer into gh (types) and gh/cli (backend)

Spec §3.1. The domain types move to internal/gh so a second backend can speak
them in a later phase; the gh-command implementation keeps its behaviour and
only changes package. depguard follows the new paths.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

---

## Task 4: 利用側 interface の分割

現在 `internal/ui` の `DataSource` は 19 メソッドある。
`.claude/rules/architecture.md` の「テスト用のフェイクで、そのテストが呼ばないメソッドのスタブを
十個以上書かされるなら大きすぎる」に当てはまる。Task 9 のサブモデル分割に備えて、
**画面ごとに必要なメソッドだけを宣言する形へ割る。**

このタスクでは `internal/ui` のファイル構成は変えず、interface の宣言だけを分ける。
分けた interface を `DataSource` が埋め込むので、`New(src DataSource)` の呼び出し側は変わらない。

**Files:**
- Modify: `internal/ui/ui.go:18-39`

**Interfaces:**
- Consumes: `gh.PR` / `gh.Issue` / `gh.Label`（Task 3）
- Produces: `listSource` / `detailSource` / `pickerSource` と、それらを埋め込む `DataSource`

- [ ] **Step 1: interface を割る**

`internal/ui/ui.go` の `DataSource` を次で置き換える:

```go
// listSource is what the list screen needs.
// repo is "owner/repo"; the empty string targets the workspace repository.
type listSource interface {
	ListPRs() ([]gh.PR, error)
	ListIssues() ([]gh.Issue, error)
	RepoName() (string, error)
}

// detailSource is what the detail screen needs.
type detailSource interface {
	GetPR(repo string, number int) (gh.PR, error)
	GetIssue(repo string, number int) (gh.Issue, error)
	OpenPRWeb(repo string, number int) error
	OpenIssueWeb(repo string, number int) error
	AddPRComment(repo string, number int, body string) error
	AddIssueComment(repo string, number int, body string) error
	ClosePR(repo string, number int) error
	ReopenPR(repo string, number int) error
	CloseIssue(repo string, number int) error
	ReopenIssue(repo string, number int) error
}

// pickerSource is what the label / assignee picker needs.
type pickerSource interface {
	ListLabels(repo string) ([]gh.Label, error)
	ListAssignees(repo string) ([]string, error)
	EditPRLabels(repo string, number int, add, remove []string) error
	EditIssueLabels(repo string, number int, add, remove []string) error
	EditPRAssignees(repo string, number int, add, remove []string) error
	EditIssueAssignees(repo string, number int, add, remove []string) error
}

// DataSource is the union the root model is handed; each screen takes only
// the slice of it that it uses.
type DataSource interface {
	listSource
	detailSource
	pickerSource
}
```

- [ ] **Step 2: 取得関数の引数を狭める**

- `fetchList` / `fetchRepoName` → `src listSource`
- `fetchDetail` / `openWeb` / `postComment` / `setState` → `src detailSource`
- `fetchLabelPicker` / `fetchAssigneePicker` / `applyPicker` → `src pickerSource`

呼び出し側（`m.src` を渡している箇所）は変更不要。

- [ ] **Step 3: 通ることを確認する**

Run: `make check`

Expected: 緑（`fakeSource` は全メソッドを持つので変更不要）

- [ ] **Step 4: コミット**

```
refactor: declare one interface per screen instead of a 19-method DataSource

architecture.md asks each view to name only what it uses, so a fake needs
stubs for that view alone. DataSource stays as the union the root model gets.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

- [ ] **Step 5: PR 3 を出す**

`git push -u origin refactor/octoscope-phase1-gh-layer` のあと
`gh pr create --label "Kind: Refactoring"`。本文:

```
Phase 1 の下準備。挙動は変えていない。

- `internal/ghcli` を `internal/gh`（ドメイン型）と `internal/gh/cli`（gh コマンドの実行）へ分けた（spec §3.1）
- 19 メソッドの `DataSource` を画面ごとの interface へ割った（`.claude/rules/architecture.md`）
- depguard と gosec の除外パスを新しい配置へ追従させた

**spec §3.2 との差:** spec は `internal/gh` に `Client` interface を置くと書いているが、
`.claude/rules/architecture.md` の「GitHub アクセス層は interface を export しない」に従い、
`internal/gh` はドメイン型のみとした。spec 側の更新は Work タブの PR に含める。

Plan: `docs/superpowers/plans/2026-09-06-octoscope-phase1.md` Task 3–4

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

---

## Task 5: Work のドメイン型

spec §4.1 と §4.5。カンバンのカードが必要とする値を、GitHub API の文字列ではなくドメインの値で表す
（`.claude/rules/architecture.md`「GitHub API 固有の値はパッケージの外に出さない」）。

「どの PR / Issue を指すか」を表す `ItemRef` も**ここに置く**。ビューのパッケージに置くと、
`work` が `detail` を import することになり、兄弟ビュー間に依存が走る。
`ItemRef` は画面の都合ではなく GitHub 上の対象そのものなので、ドメイン型が正しい置き場所である。

**Files:**
- Modify: `internal/gh/gh.go`
- Test: `internal/gh/gh_test.go`

**Interfaces:**
- Consumes: なし
- Produces:
  - `type ItemKind int` と `ItemPR` / `ItemIssue`
  - `type ItemRef struct { Kind ItemKind; Repo string; Number int }`
  - `type ReviewState int` と `ReviewNone` / `ReviewRequired` / `ReviewApproved` / `ReviewChangesRequested`
  - `func ParseReviewDecision(decision string) ReviewState`
  - `type CheckState int` と `CheckNone` / `CheckPending` / `CheckRunning` / `CheckSuccess` / `CheckFailure`
  - `type Checks struct { Total, Passed, Failed, Running int; State CheckState }`
  - `type WorkItem struct { Ref ItemRef; Title, Author string; IsDraft bool; Review ReviewState; Checks Checks; UpdatedAt time.Time; URL string }`
  - `type WorkSection int` と 4 つの定数、`func WorkSections() []WorkSection`
  - `type Work [4][]WorkItem`

- [ ] **Step 1: 失敗するテストを書く**

`internal/gh/gh_test.go`:

```go
package gh_test

import (
	"testing"

	"github.com/kukv/octoscope/internal/gh"
)

func TestWorkSectionsCoversEveryColumn(t *testing.T) {
	t.Parallel()

	got := gh.WorkSections()
	want := []gh.WorkSection{
		gh.SectionReviewRequested,
		gh.SectionYourPRs,
		gh.SectionAssigned,
		gh.SectionMentioned,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d sections, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestWorkIndexesBySection(t *testing.T) {
	t.Parallel()

	var w gh.Work
	w[gh.SectionAssigned] = []gh.WorkItem{{Ref: gh.ItemRef{Number: 7}}}

	if n := len(w[gh.SectionAssigned]); n != 1 {
		t.Fatalf("assigned column holds %d items, want 1", n)
	}
	if got := w[gh.SectionAssigned][0].Ref.Number; got != 7 {
		t.Errorf("got #%d, want #7", got)
	}
}

func TestParseReviewDecision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		decision string
		want     gh.ReviewState
	}{
		{"APPROVED", gh.ReviewApproved},
		{"CHANGES_REQUESTED", gh.ReviewChangesRequested},
		{"REVIEW_REQUIRED", gh.ReviewRequired},
		{"", gh.ReviewNone},
		{"SOMETHING_NEW", gh.ReviewNone},
	}

	for _, tt := range tests {
		if got := gh.ParseReviewDecision(tt.decision); got != tt.want {
			t.Errorf("%q: got %v, want %v", tt.decision, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: 落ちることを確認する**

Run: `go test ./internal/gh/ -v`

Expected: FAIL（`undefined: gh.WorkSections`）

- [ ] **Step 3: 型を足す**

`internal/gh/gh.go` に足す:

```go
// ItemKind separates pull requests from issues in a mixed list.
type ItemKind int

const (
	ItemPR ItemKind = iota
	ItemIssue
)

// ItemRef names one pull request or issue. Repo is "owner/name": both the
// Work board and the Repos tab can open an item from another repository, so
// the reference carries its own.
type ItemRef struct {
	Kind   ItemKind
	Repo   string
	Number int
}

// ReviewState is the review outcome of a pull request, translated out of the
// GraphQL reviewDecision enum so the UI never switches on API spelling.
type ReviewState int

const (
	ReviewNone ReviewState = iota
	ReviewRequired
	ReviewApproved
	ReviewChangesRequested
)

// ParseReviewDecision maps the GraphQL reviewDecision enum onto the domain
// value. An empty string means the pull request needs no review at all; an
// unknown one is treated the same way rather than failing the whole fetch.
func ParseReviewDecision(decision string) ReviewState {
	switch decision {
	case "APPROVED":
		return ReviewApproved
	case "CHANGES_REQUESTED":
		return ReviewChangesRequested
	case "REVIEW_REQUIRED":
		return ReviewRequired
	default:
		return ReviewNone
	}
}

// CheckState is the rolled-up outcome of a pull request's checks.
type CheckState int

const (
	CheckNone CheckState = iota
	CheckPending
	CheckRunning
	CheckSuccess
	CheckFailure
)

// Checks counts the check runs behind CheckState so a progress bar can be
// drawn without a second request.
type Checks struct {
	Total   int
	Passed  int
	Failed  int
	Running int
	State   CheckState
}

// WorkItem is one card on the Work board.
type WorkItem struct {
	Ref       ItemRef
	Title     string
	Author    string
	IsDraft   bool
	Review    ReviewState
	Checks    Checks
	UpdatedAt time.Time
	URL       string
}

// WorkSection is one column of the Work board.
type WorkSection int

const (
	SectionReviewRequested WorkSection = iota
	SectionYourPRs
	SectionAssigned
	SectionMentioned
)

// WorkSections returns the columns in display order, left to right.
func WorkSections() []WorkSection {
	return []WorkSection{
		SectionReviewRequested,
		SectionYourPRs,
		SectionAssigned,
		SectionMentioned,
	}
}

// Work holds the items of each column, indexed by WorkSection.
type Work [4][]WorkItem
```

- [ ] **Step 4: 通ることを確認する**

Run: `make check`

Expected: 緑（`internal/ui` は無影響）

- [ ] **Step 5: コミット**

```
feat: add the domain types for the Work board

Review and check outcomes become enums here so the TUI never switches on
"APPROVED" or "SUCCESS". ItemRef lives here too: it names a GitHub object,
not a screen, and putting it in a view would make the sibling views import
each other.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

---

## Task 6: GraphQL search による横断取得

spec §3.3。4 つの列を `search(type: ISSUE)` の 4 エイリアスで **1 リクエスト**にまとめる。

**既存メソッドの signature は変えない。** `runFunc` に `ctx` を足し、既存の呼び出しは
内部で `context.Background()` を渡す。読み取りメソッドへの `ctx` の付与は Task 9 で、
利用側（分割後のビュー）と同時に行う。こうすれば各タスクの終わりで `make check` が緑を保てる。

**Files:**
- Create: `internal/gh/cli/graphql.go`、`internal/gh/cli/graphql_test.go`
- Modify: `internal/gh/cli/cli.go`（`runFunc` と `runGh` のみ）
- Modify: `internal/gh/cli/cli_test.go`（`c.run` を差し替えているテストの関数シグネチャ）

**Interfaces:**
- Consumes: `gh.Work` / `gh.WorkItem` / `gh.ItemRef` / `gh.ParseReviewDecision` / `gh.Checks`（Task 5）
- Produces: `func (c *Client) ListWork(ctx context.Context) (gh.Work, error)`

### context の導入について

`.claude/rules/go-style.md` は「並行に走らせる、あるいは途中でやめる必要が出た時点で通す」と書いている。
Work タブは 4 列を 1 リクエストで取り、利用者はその最中にタブを移れる。**ここがその時点。**
コメント投稿やクローズのような書き込みは短命で、途中でやめる意味が無いため通さない。

- [ ] **Step 1: `runFunc` に ctx を通す**

`internal/gh/cli/cli.go`:

```go
type runFunc func(ctx context.Context, dir string, args ...string) ([]byte, error)
```

```go
func runGh(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, gh.ErrGhNotFound
	}
	// gh subcommand args are built internally from typed values (subcommand,
	// numbers, flags), never from untrusted external input.
	cmd := exec.CommandContext(ctx, "gh", args...)
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
```

既存メソッドの `c.run(c.dir, args...)` はすべて `c.run(context.Background(), c.dir, args...)` にする。
**メソッドの signature は変えない**ので `internal/ui` は無影響。

`internal/gh/cli/cli_test.go` で `c.run = func(dir string, args ...string) ...` としている箇所は
`func(_ context.Context, dir string, args ...string) ...` に直す。

Run: `make check`

Expected: 緑

- [ ] **Step 2: 失敗するテストを書く**

`internal/gh/cli/graphql_test.go`:

```go
package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/kukv/octoscope/internal/gh"
)

const workJSON = `{"data":{
  "reviewRequested":{"nodes":[
    {"__typename":"PullRequest","number":12,"title":"fix the thing",
     "url":"https://github.com/kukv/octoscope/pull/12","isDraft":false,
     "updatedAt":"2026-09-06T12:00:00Z","reviewDecision":"REVIEW_REQUIRED",
     "author":{"login":"someone"},
     "repository":{"nameWithOwner":"kukv/octoscope"},
     "commits":{"nodes":[{"commit":{"statusCheckRollup":{"contexts":{"nodes":[
        {"__typename":"CheckRun","conclusion":"SUCCESS","status":"COMPLETED"},
        {"__typename":"CheckRun","conclusion":"","status":"IN_PROGRESS"},
        {"__typename":"CheckRun","conclusion":"FAILURE","status":"COMPLETED"}
     ]}}}}]}}
  ]},
  "yourPRs":{"nodes":[]},
  "assigned":{"nodes":[
    {"__typename":"Issue","number":7,"title":"an issue",
     "url":"https://github.com/kukv/octoscope/issues/7",
     "updatedAt":"2026-09-05T12:00:00Z","author":{"login":"kukv"},
     "repository":{"nameWithOwner":"kukv/octoscope"}}
  ]},
  "mentioned":{"nodes":[]}
}}`

func TestListWorkBuildsOneGraphQLRequest(t *testing.T) {
	t.Parallel()

	var got []string
	c := New("/tmp", "")
	c.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		got = args
		return []byte(workJSON), nil
	}

	if _, err := c.ListWork(context.Background()); err != nil {
		t.Fatalf("ListWork: %v", err)
	}

	if len(got) < 2 || got[0] != "api" || got[1] != "graphql" {
		t.Fatalf("got args %v, want them to start with api graphql", got)
	}
	query := flagValue(t, got, "-f")
	for _, want := range []string{
		"review-requested:@me", "author:@me", "assignee:@me", "mentions:@me",
		"reviewDecision", "statusCheckRollup",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("query is missing %q:\n%s", want, query)
		}
	}
}

func TestListWorkTranslatesToDomainValues(t *testing.T) {
	t.Parallel()

	c := New("/tmp", "")
	c.run = func(context.Context, string, ...string) ([]byte, error) {
		return []byte(workJSON), nil
	}

	w, err := c.ListWork(context.Background())
	if err != nil {
		t.Fatalf("ListWork: %v", err)
	}

	rr := w[gh.SectionReviewRequested]
	if len(rr) != 1 {
		t.Fatalf("review requested holds %d items, want 1", len(rr))
	}
	item := rr[0]
	if item.Ref.Kind != gh.ItemPR {
		t.Errorf("kind: got %v, want ItemPR", item.Ref.Kind)
	}
	if item.Ref.Repo != "kukv/octoscope" {
		t.Errorf("repo: got %q, want kukv/octoscope", item.Ref.Repo)
	}
	if item.Review != gh.ReviewRequired {
		t.Errorf("review: got %v, want ReviewRequired", item.Review)
	}
	if item.Checks.Total != 3 || item.Checks.Passed != 1 ||
		item.Checks.Failed != 1 || item.Checks.Running != 1 {
		t.Errorf("checks counts: got %+v", item.Checks)
	}
	if item.Checks.State != gh.CheckFailure {
		t.Errorf("checks state: got %v, want CheckFailure", item.Checks.State)
	}

	assigned := w[gh.SectionAssigned]
	if len(assigned) != 1 || assigned[0].Ref.Kind != gh.ItemIssue {
		t.Fatalf("assigned column: got %+v, want one issue", assigned)
	}
	if n := len(w[gh.SectionYourPRs]); n != 0 {
		t.Errorf("your PRs holds %d items, want 0", n)
	}
}

func TestListWorkReportsAFailure(t *testing.T) {
	t.Parallel()

	c := New("/tmp", "")
	c.run = func(context.Context, string, ...string) ([]byte, error) {
		return []byte("not json"), nil
	}

	if _, err := c.ListWork(context.Background()); err == nil {
		t.Error("ListWork accepted a body that is not JSON")
	}
}

// flagValue returns the argument that follows the last occurrence of flag.
func flagValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	for i := len(args) - 2; i >= 0; i-- {
		if args[i] == flag {
			return args[i+1]
		}
	}
	t.Fatalf("flag %q not found in %v", flag, args)
	return ""
}
```

- [ ] **Step 3: 落ちることを確認する**

Run: `go test ./internal/gh/cli/ -run TestListWork -v`

Expected: FAIL（`c.ListWork undefined`）

- [ ] **Step 4: 実装する**

`internal/gh/cli/graphql.go`:

```go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kukv/octoscope/internal/gh"
)

// workSearches pairs each board column with its GraphQL alias and the search
// query behind it (spec §4.1).
var workSearches = []struct {
	section gh.WorkSection
	alias   string
	query   string
}{
	{gh.SectionReviewRequested, "reviewRequested", "is:open is:pr review-requested:@me"},
	{gh.SectionYourPRs, "yourPRs", "is:open is:pr author:@me"},
	{gh.SectionAssigned, "assigned", "is:open assignee:@me"},
	{gh.SectionMentioned, "mentioned", "is:open mentions:@me"},
}

// workItemFields is the selection every column shares. reviewDecision and
// isDraft are GraphQL-only: REST does not expose them (spec §3.3).
const workItemFields = `
    __typename
    ... on PullRequest {
      number title url isDraft updatedAt reviewDecision
      author { login }
      repository { nameWithOwner }
      commits(last: 1) { nodes { commit { statusCheckRollup { contexts(first: 100) { nodes {
        __typename
        ... on CheckRun { status conclusion }
        ... on StatusContext { state }
      } } } } } }
    }
    ... on Issue {
      number title url updatedAt
      author { login }
      repository { nameWithOwner }
    }`

const workItemsPerColumn = 50

// workQuery builds one document whose four aliased searches fill the board in
// a single request. The search strings are compile-time constants of this
// package, so %q's Go quoting is a valid GraphQL string literal for them; a
// query that ever needs a quote or a backslash has to move to a $query
// variable passed with -F instead.
func workQuery() string {
	var b strings.Builder
	b.WriteString("query {")
	for _, s := range workSearches {
		fmt.Fprintf(&b,
			"\n  %s: search(type: ISSUE, first: %d, query: %q) { nodes {%s\n  } }",
			s.alias, workItemsPerColumn, s.query, workItemFields)
	}
	b.WriteString("\n}")
	return b.String()
}

type workResponse struct {
	Data map[string]struct {
		Nodes []searchNode `json:"nodes"`
	} `json:"data"`
}

type searchNode struct {
	Typename       string    `json:"__typename"`
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	URL            string    `json:"url"`
	IsDraft        bool      `json:"isDraft"`
	UpdatedAt      time.Time `json:"updatedAt"`
	ReviewDecision string    `json:"reviewDecision"`
	Author         struct {
		Login string `json:"login"`
	} `json:"author"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					Contexts struct {
						Nodes []checkNode `json:"nodes"`
					} `json:"contexts"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

type checkNode struct {
	Typename   string `json:"__typename"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

// ListWork fetches every column of the Work board in one GraphQL request.
func (c *Client) ListWork(ctx context.Context) (gh.Work, error) {
	out, err := c.run(ctx, c.dir, "api", "graphql", "-f", "query="+workQuery())
	if err != nil {
		return gh.Work{}, err
	}
	var resp workResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return gh.Work{}, fmt.Errorf("parse work search: %w", err)
	}
	var w gh.Work
	for _, s := range workSearches {
		nodes := resp.Data[s.alias].Nodes
		items := make([]gh.WorkItem, 0, len(nodes))
		for _, n := range nodes {
			items = append(items, n.toWorkItem())
		}
		w[s.section] = items
	}
	return w, nil
}

func (n searchNode) toWorkItem() gh.WorkItem {
	item := gh.WorkItem{
		Ref: gh.ItemRef{
			Kind:   gh.ItemIssue,
			Repo:   n.Repository.NameWithOwner,
			Number: n.Number,
		},
		Title:     n.Title,
		Author:    n.Author.Login,
		UpdatedAt: n.UpdatedAt,
		URL:       n.URL,
	}
	if n.Typename != "PullRequest" {
		return item
	}
	item.Ref.Kind = gh.ItemPR
	item.IsDraft = n.IsDraft
	item.Review = gh.ParseReviewDecision(n.ReviewDecision)
	item.Checks = n.checks()
	return item
}

func (n searchNode) checks() gh.Checks {
	var c gh.Checks
	for _, commit := range n.Commits.Nodes {
		rollup := commit.Commit.StatusCheckRollup
		if rollup == nil {
			continue
		}
		for _, node := range rollup.Contexts.Nodes {
			c.Total++
			switch checkOutcome(node) {
			case gh.CheckSuccess:
				c.Passed++
			case gh.CheckFailure:
				c.Failed++
			default:
				c.Running++
			}
		}
	}
	switch {
	case c.Total == 0:
		c.State = gh.CheckNone
	case c.Failed > 0:
		c.State = gh.CheckFailure
	case c.Running > 0:
		c.State = gh.CheckRunning
	default:
		c.State = gh.CheckSuccess
	}
	return c
}

// checkOutcome reads one context of the rollup. CheckRun reports status and
// conclusion; the older StatusContext reports a single state, so the two
// shapes have to be read differently.
func checkOutcome(n checkNode) gh.CheckState {
	if n.Typename == "StatusContext" {
		switch n.State {
		case "SUCCESS":
			return gh.CheckSuccess
		case "FAILURE", "ERROR":
			return gh.CheckFailure
		default:
			return gh.CheckPending
		}
	}
	if n.Status != "COMPLETED" {
		return gh.CheckRunning
	}
	switch n.Conclusion {
	case "SUCCESS", "NEUTRAL", "SKIPPED":
		return gh.CheckSuccess
	case "FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE":
		return gh.CheckFailure
	default:
		return gh.CheckPending
	}
}
```

- [ ] **Step 5: 通ることを確認する**

Run: `make check`

Expected: 緑

- [ ] **Step 6: テストが空振りしていないことを確認する**

`gh.ParseReviewDecision` の `"REVIEW_REQUIRED"` を `"REVIEW_REQUIRE"` に**一時的に**書き換えて、

Run: `go test ./internal/gh/... -run 'TestListWorkTranslates|TestParseReviewDecision'`

Expected: FAIL

`checks()` の `c.Failed > 0` を `c.Failed > 1` に**一時的に**書き換えて、

Run: `go test ./internal/gh/cli/ -run TestListWorkTranslates`

Expected: FAIL（`checks state`）

どちらも戻して PASS することを見る。

- [ ] **Step 7: コミット**

```
feat: fetch the Work board in one GraphQL request

Four aliased search(type: ISSUE) queries fill the four columns at once;
repeating gh pr list per repository would cost one request each. The GraphQL
enums are translated to domain values here so the TUI never sees them.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

---

## Task 7: 共有する表示部品（`i18n.RelTime` と `internal/tui/icon`）

`relTime` と `reviewIcon` は現在 `internal/ui/render.go` にあり、Task 8 の Work タブと
Task 9 の Repos タブの両方が使う。片方に置いてもう片方から import すると兄弟間の依存になるので、
先に共有できる場所へ出す。ゴミ箱パッケージ（`util` の類）は作らない。

- `relTime` は `time.Time` から表示文字列を作る純関数で、カタログの `time.*` を引く。
  **`internal/i18n` に `RelTime` として移す。** 依存は増えない
- `reviewIcon` は `gh.ReviewState` / `gh.CheckState` から記号を選ぶ。
  **`internal/tui/icon` パッケージを作る。** spec §4.5 の装飾はここに集める

**Files:**
- Modify: `internal/i18n/i18n.go`、`internal/i18n/i18n_test.go`
- Create: `internal/tui/icon/icon.go`、`internal/tui/icon/icon_test.go`
- Modify: `internal/ui/render.go`（`relTime` を削除し `i18n.RelTime` を呼ぶ、`reviewIcon` を `icon.Review` に）
- Modify: `.golangci.yml`

**Interfaces:**
- Consumes: `gh.ReviewState` / `gh.CheckState` / `gh.Checks`（Task 5）
- Produces:
  - `func i18n.RelTime(now, t time.Time) string`
  - `func icon.Review(s gh.ReviewState, draft bool) string`
  - `func icon.Check(s gh.CheckState) string`
  - `const icon.BarWidth = 7`、`func icon.ChecksBar(c gh.Checks) string`

- [ ] **Step 1: `RelTime` のテストを書く**

`internal/i18n/i18n_test.go` に足す:

```go
func TestRelTimePicksTheRightUnit(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })
	i18n.SetLanguage(language.English)

	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		ago  time.Duration
		want string
	}{
		{"under a minute", 59 * time.Second, "now"},
		{"exactly a minute", time.Minute, "1m ago"},
		{"under an hour", 59 * time.Minute, "59m ago"},
		{"exactly an hour", time.Hour, "1h ago"},
		{"under a day", 25 * time.Hour, "1d ago"},
		{"several days", 72 * time.Hour, "3d ago"},
	}

	for _, tt := range tests {
		if got := i18n.RelTime(now, now.Add(-tt.ago)); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
```

Run: `go test ./internal/i18n/ -run TestRelTime -v`

Expected: FAIL（`undefined: i18n.RelTime`）

- [ ] **Step 2: `i18n.RelTime` を実装し、`internal/ui` から移す**

`internal/i18n/i18n.go` に足す:

```go
// RelTime renders how long ago t was, relative to now.
func RelTime(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return T("time.now")
	case d < time.Hour:
		return Tn("time.minutes_ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return Tn("time.hours_ago", int(d.Hours()))
	default:
		return Tn("time.days_ago", int(d.Hours()/24))
	}
}
```

`internal/ui/render.go` の `relTime` を削除し、`prLine` / `issueLine` の呼び出しを
`i18n.RelTime(now, ...)` にする。

Run: `go test ./internal/i18n/ ./internal/ui/ -v`

Expected: PASS

- [ ] **Step 3: `internal/tui/icon` を作る**

`internal/tui/icon/icon.go`:

```go
// Package icon picks the glyphs the views use for review and check state.
// Phase 1 uses Unicode symbols only; the Nerd Font variants come later.
package icon

import (
	"strings"

	"github.com/kukv/octoscope/internal/gh"
)

// Review returns the one-column marker for a pull request's review state.
func Review(s gh.ReviewState, draft bool) string {
	if draft {
		return "◌"
	}
	switch s {
	case gh.ReviewApproved:
		return "✓"
	case gh.ReviewChangesRequested:
		return "×"
	default:
		return "•"
	}
}

// Check returns the one-column marker for a rolled-up check state.
func Check(s gh.CheckState) string {
	switch s {
	case gh.CheckSuccess:
		return "✓"
	case gh.CheckFailure:
		return "×"
	case gh.CheckRunning, gh.CheckPending:
		return "◍"
	default:
		return " "
	}
}

// BarWidth is how many cells a checks bar occupies. A Work card has about 30
// columns, so the bar has to stay narrow (spec §4.1, §6.4).
const BarWidth = 7

// ChecksBar draws the passed / total ratio as a fixed-width bar. It returns
// an empty string when there are no checks, so callers can leave the field
// out instead of drawing an empty bar.
func ChecksBar(c gh.Checks) string {
	if c.Total == 0 {
		return ""
	}
	filled := c.Passed * BarWidth / c.Total
	if filled == 0 && c.Passed > 0 {
		filled = 1
	}
	return strings.Repeat("▰", filled) + strings.Repeat("▱", BarWidth-filled)
}
```

`internal/tui/icon/icon_test.go` にテーブル駆動テストを書く。少なくとも次を検査する。

- `Review`: draft が他のどの状態よりも優先されること、4 状態それぞれの記号
- `Check`: 5 状態それぞれの記号
- `ChecksBar`: `Total == 0` で空文字。`Total > 0` のとき `ansi.StringWidth` が常に `BarWidth`。
  `Passed == 0` で `▰` を含まない。`0 < Passed` かつ `Passed*BarWidth/Total == 0` になる比
  （例: `Total: 100, Passed: 1`）で `▰` が 1 つだけ入る

Run: `go test ./internal/tui/icon/ -v`

Expected: PASS

- [ ] **Step 4: `internal/ui` を `icon.Review` に切り替える**

`internal/ui/render.go` の `reviewIcon` を削除し、`prLine` を次にする:

```go
func prLine(pr gh.PR, now time.Time) string {
	return fmt.Sprintf("#%-5d %s  @%s  %s %s",
		pr.Number, pr.Title, pr.Author.Login,
		icon.Review(gh.ParseReviewDecision(pr.ReviewDecision), pr.IsDraft),
		i18n.RelTime(now, pr.UpdatedAt))
}
```

- [ ] **Step 5: depguard に `internal/tui/icon` を足す**

`internal/tui/icon` は `internal/gh` に依存してよい（UI → GitHub 層は正しい向き）。
新たに禁止を書く必要は無いが、`i18n-layer` の `deny` に `internal/tui` が既にあることを確認する。
兄弟ビュー間のルールは Task 8 と Task 9 で足す。

- [ ] **Step 6: `make check` を通す**

Run: `make check`

Expected: 緑

- [ ] **Step 7: コミット**

```
refactor: move relative times to i18n and status glyphs to tui/icon

Both the Work board and the repository list need them. Keeping either in one
view would make the sibling views import each other; a relative time is a
catalog lookup and a status glyph is a domain-to-symbol map, so each has a
place that is already below both.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

---

## Task 8: Work タブ

spec §4.1、§4.5、§4.6。**このタスクの時点では誰も `work` を使わない**（配線は Task 10）。
独立したパッケージとして作り、テストだけで検証する。`make check` は緑を保つ。

**Files:**
- Create: `internal/tui/work/work.go`、`render.go`、`work_test.go`、`render_test.go`
- Modify: `internal/i18n/locales/active.en.yaml`、`active.ja.yaml`
- Modify: `.golangci.yml`

**Interfaces:**
- Consumes: `gh.Work` / `gh.WorkItem` / `gh.ItemRef` / `gh.WorkSections`（Task 5）、`icon.*`（Task 7）
- Produces:
  - `type Source interface { ListWork(ctx context.Context) (gh.Work, error) }`
  - `func New(src Source) Model`
  - `func (m Model) Refresh() (Model, tea.Cmd)`、`func (m Model) Cancel()`
  - `func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)`、`func (m Model) View() string`
  - `func (m Model) SelectedRef() (gh.ItemRef, bool)`
  - `type OpenDetailMsg struct{ Ref gh.ItemRef }`、`type ErrorMsg struct{ Err error }`

`work.Model` に `Init()` は**置かない**。最初の取得も `Refresh()` で行う
（`Init() tea.Cmd` だと `cancel` を持ち帰れず、`r` を押したときに前の取得を止められない）。
親（`app`）が `m.work, cmd = m.work.Refresh()` の形で呼ぶ。

### レイアウト

```
Review requested   Your PRs           Assigned           Mentioned
─────────────────  ─────────────────  ─────────────────  ─────────────────
▸ • fix the thing  ◌ wip: refactor    #7 an issue        • bump deps
  kukv/octoscope     kukv/koto          kukv/octoscope     kukv/koto
  ▰▰▰▰▱▱▱ 2h ago     ▱▱▱▱▱▱▱ 5m ago     3d ago             ▰▰▰▰▰▰▰ 1d ago
...
──────────────────────────────────────────────────────────────────────────
kukv/octoscope#12 fix the thing
1/3 checks passed, 1 failed, 1 running
```

- 列幅は `(width - columnGap*(列数-1)) / 列数`。`ansi.Truncate` で切り詰める
- ドロワー（下段）は選択中カードの `repo#number title` と checks の内訳
- `h`/`l` で列、`j`/`k` で行。列を移ったとき、その列の行数を超えていたら末尾へ丸める
- `enter` で `OpenDetailMsg`、`r` で再取得

### 幅の劣化（spec §4.6）

- `width < 100`: ドロワーを畳む
- `width < 60`: 1 列だけ表示し、`h`/`l` は列のページングになる。ヘッダーに `column 2/4` を添える

- [ ] **Step 1: カタログへ文字列を足す**

`active.en.yaml` に新しいトップレベル `work:` を足す:

```yaml
work:
  review_requested:
    other: "Review requested"
  your_prs:
    other: "Your PRs"
  assigned:
    other: "Assigned"
  mentioned:
    other: "Mentioned"
  empty_column:
    other: "(nothing here)"
  checks_summary:
    other: "{{.Passed}}/{{.Total}} checks passed, {{.Failed}} failed, {{.Running}} running"
  no_checks:
    other: "no checks"
  column_position:
    other: "column {{.Index}}/{{.Total}}"
```

`footer:` に足す:

```yaml
  work:
    other: "h/l:column  j/k:move  enter:detail  r:refresh  1/2:tab  q:quit"
```

`active.ja.yaml` に同じ ID で:

```yaml
work:
  review_requested:
    other: "レビュー依頼"
  your_prs:
    other: "自分の PR"
  assigned:
    other: "担当"
  mentioned:
    other: "メンション"
  empty_column:
    other: "（なし）"
  checks_summary:
    other: "checks {{.Total}} 件中 {{.Passed}} 件成功 / {{.Failed}} 件失敗 / {{.Running}} 件実行中"
  no_checks:
    other: "checks なし"
  column_position:
    other: "{{.Index}}/{{.Total}} 列目"
```

```yaml
  work:
    other: "h/l:列  j/k:移動  enter:詳細  r:再取得  1/2:タブ  q:終了"
```

**タブ名（`Work` / `Repos`）は翻訳しない。** キー `1` / `2` と対で覚える画面上の識別子であり、
GitHub の検索構文と同じく「訳すと分かりにくくなる」側に入る。
この判断は Task 12 Step 4 で spec §6.1 に追記する。

- [ ] **Step 2: 失敗するテストを書く（カーソル移動）**

`internal/tui/work/work_test.go`:

```go
package work

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kukv/octoscope/internal/gh"
)

type fakeSource struct {
	work  gh.Work
	err   error
	calls int
}

func (f *fakeSource) ListWork(context.Context) (gh.Work, error) {
	f.calls++
	return f.work, f.err
}

func sampleWork() gh.Work {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	var w gh.Work
	w[gh.SectionReviewRequested] = []gh.WorkItem{
		{
			Ref:   gh.ItemRef{Kind: gh.ItemPR, Repo: "kukv/octoscope", Number: 12},
			Title: "fix the thing", UpdatedAt: now,
			Checks: gh.Checks{Total: 3, Passed: 1, Failed: 1, Running: 1, State: gh.CheckFailure},
		},
		{
			Ref:   gh.ItemRef{Kind: gh.ItemPR, Repo: "kukv/koto", Number: 3},
			Title: "bump deps", UpdatedAt: now,
		},
	}
	w[gh.SectionAssigned] = []gh.WorkItem{
		{
			Ref:   gh.ItemRef{Kind: gh.ItemIssue, Repo: "kukv/octoscope", Number: 7},
			Title: "an issue", UpdatedAt: now,
		},
	}
	return w
}

// loaded returns a model that already received its data, sized wide enough
// for all four columns and the drawer.
func loaded() Model {
	m := New(&fakeSource{work: sampleWork()})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = m.Update(workMsg(sampleWork()))
	return m
}

// key builds the KeyPressMsg for a key name, matching the shape the app uses.
func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	default:
		return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
}

func press(m Model, k string) Model {
	m, _ = m.Update(key(k))
	return m
}

func TestCursorMovesWithinAColumn(t *testing.T) {
	m := press(loaded(), "j")
	if m.row != 1 {
		t.Errorf("row: got %d, want 1", m.row)
	}
	if m = press(m, "j"); m.row != 1 {
		t.Errorf("row past the end: got %d, want it clamped to 1", m.row)
	}
	if m = press(press(m, "k"), "k"); m.row != 0 {
		t.Errorf("row before the start: got %d, want it clamped to 0", m.row)
	}
}

func TestMovingToAShorterColumnClampsTheRow(t *testing.T) {
	m := press(loaded(), "j") // row 1 of the 2-item first column
	m = press(m, "l")         // column 1 is empty
	m = press(m, "l")         // column 2 holds one item
	if m.col != 2 {
		t.Fatalf("col: got %d, want 2", m.col)
	}
	if m.row != 0 {
		t.Errorf("row: got %d, want it clamped to 0", m.row)
	}
}

func TestColumnWrapsAtBothEnds(t *testing.T) {
	if m := press(loaded(), "h"); m.col != 3 {
		t.Errorf("h from column 0: got %d, want 3", m.col)
	}
	m := loaded()
	for range 4 {
		m = press(m, "l")
	}
	if m.col != 0 {
		t.Errorf("four l presses: got %d, want 0", m.col)
	}
}

func TestSelectedRefNamesTheItemUnderTheCursor(t *testing.T) {
	ref, ok := press(loaded(), "j").SelectedRef()
	if !ok {
		t.Fatal("SelectedRef reported no selection")
	}
	if ref.Repo != "kukv/koto" || ref.Number != 3 {
		t.Errorf("got %s#%d, want kukv/koto#3", ref.Repo, ref.Number)
	}
}

func TestEmptyColumnHasNoSelection(t *testing.T) {
	if _, ok := press(loaded(), "l").SelectedRef(); ok {
		t.Error("SelectedRef reported a selection in an empty column")
	}
}

func TestEnterAsksTheParentToOpenTheDetail(t *testing.T) {
	_, cmd := loaded().Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	msg, ok := cmd().(OpenDetailMsg)
	if !ok {
		t.Fatalf("got %T, want OpenDetailMsg", cmd())
	}
	if msg.Ref.Number != 12 {
		t.Errorf("got #%d, want #12", msg.Ref.Number)
	}
}

func TestEnterOnAnEmptyColumnDoesNothing(t *testing.T) {
	if _, cmd := press(loaded(), "l").Update(key("enter")); cmd != nil {
		t.Error("enter produced a command with nothing selected")
	}
}

func TestFetchFailureBecomesAnErrorMsg(t *testing.T) {
	m := New(&fakeSource{err: errors.New("boom")})
	_, cmd := m.Update(errMsg{errors.New("boom")})
	if cmd == nil {
		t.Fatal("no command returned for a failed fetch")
	}
	got, ok := cmd().(ErrorMsg)
	if !ok {
		t.Fatalf("got %T, want ErrorMsg", cmd())
	}
	if got.Err.Error() != "boom" {
		t.Errorf("got %q, want boom", got.Err)
	}
}

func TestRefreshCancelsThePreviousFetch(t *testing.T) {
	f := &fakeSource{work: sampleWork()}
	m := New(f)

	m, first := m.Refresh()
	m, second := m.Refresh()

	// The first fetch's context is cancelled, so its command sees a dead
	// context; the second one still runs.
	_ = first
	if second == nil {
		t.Fatal("Refresh returned no command")
	}
	if _, ok := second().(workMsg); !ok {
		t.Errorf("second fetch: got %T, want workMsg", second())
	}
	m.Cancel()
}
```

- [ ] **Step 3: 落ちることを確認する**

Run: `go test ./internal/tui/work/ -v`

Expected: FAIL（パッケージが無い）

- [ ] **Step 4: `work.go` を実装する**

```go
// Package work implements the Work board: the columns of what needs the
// user's attention, across every repository they touch.
package work

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/kukv/octoscope/internal/gh"
)

// Source is what the Work board needs from the GitHub layer.
type Source interface {
	ListWork(ctx context.Context) (gh.Work, error)
}

type (
	workMsg gh.Work
	errMsg  struct{ err error }
)

// OpenDetailMsg asks the parent to show the detail view for the selected card.
type OpenDetailMsg struct{ Ref gh.ItemRef }

// ErrorMsg carries a failure the parent shows on its error screen.
type ErrorMsg struct{ Err error }

type Model struct {
	src Source

	width, height int
	loading       bool
	work          gh.Work
	col, row      int

	// cancel stops the in-flight fetch. The board is the one place where a
	// request outlives the user's interest in it: they can switch tabs or ask
	// for a refresh while four searches are still running.
	cancel context.CancelFunc
}

func New(src Source) Model {
	return Model{src: src}
}

// Refresh cancels any in-flight fetch and starts a new one. It is also how
// the first fetch is started: a tea.Cmd-returning Init could not hand the
// cancel function back to the caller.
func (m Model) Refresh() (Model, tea.Cmd) {
	m.Cancel()
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.loading = true
	src := m.src
	return m, func() tea.Msg {
		w, err := src.ListWork(ctx)
		if err != nil {
			return errMsg{err}
		}
		return workMsg(w)
	}
}

// Cancel stops the in-flight fetch. The parent calls it when the user quits.
func (m Model) Cancel() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case workMsg:
		m.loading = false
		m.work = gh.Work(msg)
		m.clampCursor()
	case errMsg:
		m.loading = false
		err := msg.err
		return m, func() tea.Msg { return ErrorMsg{err} }
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "h", "left":
		m.col = wrapColumn(m.col-1, m.columns())
		m.clampCursor()
	case "l", "right":
		m.col = wrapColumn(m.col+1, m.columns())
		m.clampCursor()
	case "j", "down":
		if m.row+1 < len(m.work[m.section()]) {
			m.row++
		}
	case "k", "up":
		if m.row > 0 {
			m.row--
		}
	case "r":
		return m.Refresh()
	case "enter":
		if ref, ok := m.SelectedRef(); ok {
			return m, func() tea.Msg { return OpenDetailMsg{ref} }
		}
	}
	return m, nil
}

func (m Model) columns() int { return len(gh.WorkSections()) }

func (m Model) section() gh.WorkSection { return gh.WorkSections()[m.col] }

func wrapColumn(i, n int) int {
	switch {
	case i < 0:
		return n - 1
	case i >= n:
		return 0
	default:
		return i
	}
}

// clampCursor pulls the row back into range after the column changed or the
// data was replaced. It takes a pointer because it is only ever called on the
// local copy handleKey and Update are about to return.
func (m *Model) clampCursor() {
	n := len(m.work[m.section()])
	if m.row >= n {
		m.row = max(n-1, 0)
	}
}

// SelectedRef names the card under the cursor. ok is false when the column is
// empty.
func (m Model) SelectedRef() (gh.ItemRef, bool) {
	items := m.work[m.section()]
	if m.row >= len(items) {
		return gh.ItemRef{}, false
	}
	return items[m.row].Ref, true
}
```

- [ ] **Step 5: 通ることを確認する**

Run: `go test ./internal/tui/work/ -v`

Expected: PASS

- [ ] **Step 6: 描画のテストを書く**

`internal/tui/work/render_test.go`:

```go
package work

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/language"

	"github.com/kukv/octoscope/internal/i18n"
)

func TestViewShowsEveryColumnHeading(t *testing.T) {
	out := loaded().View()
	for _, want := range []string{
		i18n.T("work.review_requested"),
		i18n.T("work.your_prs"),
		i18n.T("work.assigned"),
		i18n.T("work.mentioned"),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view is missing the %q heading", want)
		}
	}
}

func TestViewShowsTheSelectedCardInTheDrawer(t *testing.T) {
	if out := press(loaded(), "j").View(); !strings.Contains(out, "kukv/koto#3") {
		t.Errorf("drawer does not name the selected card:\n%s", out)
	}
}

func TestEmptyColumnSaysSo(t *testing.T) {
	if out := loaded().View(); !strings.Contains(out, i18n.T("work.empty_column")) {
		t.Errorf("no empty-column marker for Your PRs:\n%s", out)
	}
}

func TestNarrowTerminalDropsTheDrawer(t *testing.T) {
	m := loaded()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	if strings.Contains(m.View(), "kukv/octoscope#12") {
		t.Error("the drawer is still drawn at 80 columns")
	}
}

func TestVeryNarrowTerminalShowsOneColumn(t *testing.T) {
	m := loaded()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 50, Height: 40})
	out := m.View()
	if strings.Contains(out, i18n.T("work.mentioned")) {
		t.Error("all four headings are drawn at 50 columns")
	}
	if !strings.Contains(out, i18n.T("work.review_requested")) {
		t.Error("the current column's heading is missing")
	}
}

func TestNoLineExceedsTheTerminalWidth(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })

	for _, lang := range []language.Tag{language.English, language.Japanese} {
		i18n.SetLanguage(lang)
		for _, width := range []int{50, 80, 100, 120} {
			m := loaded()
			m, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
			for _, line := range strings.Split(m.View(), "\n") {
				if w := ansi.StringWidth(line); w > width {
					t.Errorf("lang %s width %d: line is %d columns: %q",
						lang, width, w, line)
				}
			}
		}
	}
}

func TestNoUnresolvedIDsInTheWorkView(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage(language.English) })

	for _, lang := range []language.Tag{language.English, language.Japanese} {
		i18n.SetLanguage(lang)
		i18n.AssertNoUnresolvedIDs(t, loaded().View())
		i18n.AssertNoUnresolvedIDs(t, New(&fakeSource{}).View())
	}
}
```

言語を切り替えるテストは `t.Parallel()` にしない。

- [ ] **Step 7: `render.go` を実装する**

構成:

```go
package work

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kukv/octoscope/internal/gh"
	"github.com/kukv/octoscope/internal/i18n"
	"github.com/kukv/octoscope/internal/tui/icon"
)

var (
	headingStyle = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Faint(true)
	cursorStyle  = lipgloss.NewStyle().Bold(true)
)

const (
	columnGap         = 2
	drawerMinColumns  = 100
	singleColumnBelow = 60
)

// sectionTitleIDs maps a column to its heading in the catalog.
var sectionTitleIDs = map[gh.WorkSection]string{
	gh.SectionReviewRequested: "work.review_requested",
	gh.SectionYourPRs:         "work.your_prs",
	gh.SectionAssigned:        "work.assigned",
	gh.SectionMentioned:       "work.mentioned",
}
```

- `View() string` は `m.loading` のとき `i18n.T("common.loading")` の行だけを返す
  （スピナーは親が持つ。サブモデルに `tea.Tick` を持たせない）
- `visibleSections() []gh.WorkSection` が幅で分岐する。`m.width < singleColumnBelow` なら
  `m.section()` の 1 列だけを返し、ヘッダーに
  `i18n.Tf("work.column_position", map[string]any{"Index": m.col + 1, "Total": m.columns()})` を添える
- `columnWidth(n int) int` は `(m.width - columnGap*(n-1)) / n`
- `cardLines(it gh.WorkItem, w int, selected bool) []string` が 3 行を返す
  - 1 行目: カーソル記号（`▸ ` / `  `）+ PR なら `icon.Review(it.Review, it.IsDraft)`、
    Issue なら `#番号` + タイトル
  - 2 行目: `it.Ref.Repo`
  - 3 行目: `icon.ChecksBar(it.Checks)` + `i18n.RelTime(now, it.UpdatedAt)`
  - 各行は `ansi.Truncate(s, w, "…")` で切り詰め、`lipgloss.NewStyle().Width(w)` で桁を揃える
- 空の列は `dimStyle.Render(i18n.T("work.empty_column"))` を 1 行出す
- `drawer() string` は `m.width >= drawerMinColumns` かつ選択があるときだけ描く。
  `fmt.Sprintf("%s#%d %s", ref.Repo, ref.Number, it.Title)` と、
  checks があれば `i18n.Tf("work.checks_summary", map[string]any{"Passed": …, "Total": …, "Failed": …, "Running": …})`、
  無ければ `i18n.T("work.no_checks")`
- フッターは `dimStyle.Render(i18n.T("footer.work"))`

**桁の数え方は必ず `ansi.StringWidth` / `ansi.Truncate`。** `len()` と `utf8.RuneCountInString` は使わない。

- [ ] **Step 8: depguard に兄弟ビューの禁止を足す**

`.golangci.yml` の `depguard.rules` に足す:

```yaml
        # Views are siblings: a view must not reach into another view's model,
        # and no child may import its parent.
        tui-work:
          files:
            - "**/internal/tui/work/**"
          deny:
            - pkg: github.com/kukv/octoscope/internal/tui/repo
              desc: sibling views communicate through the app model, not directly
            - pkg: github.com/kukv/octoscope/internal/tui/detail
              desc: sibling views communicate through the app model, not directly
            - pkg: github.com/kukv/octoscope/internal/tui/app
              desc: a child view must not import its parent
```

- [ ] **Step 9: `make check` を通す**

Run: `make check`

Expected: 緑

- [ ] **Step 10: 幅のテストが空振りしていないことを確認する**

`cardLines` の `ansi.Truncate(s, w, "…")` を `s` に**一時的に**書き換えて、

Run: `go test ./internal/tui/work/ -run TestNoLineExceedsTheTerminalWidth`

Expected: FAIL（特に `lang ja` と `width 50` で）

戻して PASS することを見る。

- [ ] **Step 11: コミット**

```
feat: add the Work board

Four columns of what needs attention, drawn from one GraphQL request. Column
height is the point of the layout: a tall Review requested column is a backlog
you can see without reading it. Nothing wires it up yet.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

---

## Task 9: `internal/ui` を `internal/tui/{repo,detail}` へ分割

spec §3.1。1 枚のモデルに一覧・詳細・コメント入力・ピッカーが同居している状態を、
ビュー単位のサブモデルへ割る。**画面の見た目と操作は変えない。**
同時に、読み取りメソッドへ `ctx` を通す（Task 6 の続き）。

このタスクの終わりでは `internal/ui` がまだ残っている（`app` が無いため）。
`internal/ui` は削除せず、**新しい 2 パッケージを足すだけ**にする。
`internal/ui` の削除は Task 10。ビルドは緑のまま。

**Files:**
- Create: `internal/tui/detail/detail.go`、`render.go`、`picker.go`、`detail_test.go`、`picker_test.go`
- Create: `internal/tui/repo/repo.go`、`render.go`、`repo_test.go`
- Modify: `internal/gh/cli/cli.go`（読み取りメソッドに `ctx` を足す）
- Modify: `internal/ui/*.go`、`internal/ui/*_test.go`（`ctx` を渡すよう追従）
- Modify: `.golangci.yml`

**Interfaces:**
- Consumes: `gh.*`（Task 3、Task 5）、`icon.*` / `i18n.RelTime`（Task 7）
- Produces:
  - `package detail`: `type Source interface{...}`、`func New(src Source, ref gh.ItemRef) Model`、
    `func (m Model) Init() tea.Cmd`、`Update(msg) (Model, tea.Cmd)`、`View() string`、
    `type ClosedMsg struct{}`、`type ErrorMsg struct{ Err error }`
  - `package repo`: `type Source interface{...}`、`func New(src Source) Model`、
    `func (m Model) Init() tea.Cmd`、`Update(msg) (Model, tea.Cmd)`、`View() string`、
    `type OpenDetailMsg struct{ Ref gh.ItemRef }`、`type ErrorMsg struct{ Err error }`

### 分割の線

- **`repo`**: 一覧（PR / Issue のサブタブ、カーソル、再取得、リポジトリ名ヘッダー）。
  `enter` で `OpenDetailMsg` を親へ返すところまで。詳細の状態は持たない
- **`detail`**: 詳細ビュー、コメント入力、クローズ/リオープン確認、ラベル/アサイニーのピッカー。
  `esc` で `ClosedMsg` を親へ返す
- エラー画面はどちらにも置かず、`ErrorMsg` を親へ上げて `app` が出す（Task 10）
- スピナーは親が持つ。サブモデルは「読み込み中」の文字列を出すだけにする

- [ ] **Step 1: `cli` の読み取りメソッドに `ctx` を足す**

`ListPRs` / `ListIssues` / `GetPR` / `GetIssue` / `RepoName` / `ListLabels` / `ListAssignees` の
第一引数に `ctx context.Context` を足し、`c.run(ctx, c.dir, ...)` にする。
書き込みメソッドは `context.Background()` のまま。

`internal/ui` の `DataSource` とその実装（`fakeSource`）、`fetch*` の呼び出しを追従させる。
`tea.Cmd` の中で `context.Background()` を作る
（`.claude/rules/go-style.md` は `cmd/octoscope` と `tea.Cmd` の組み立て箇所を例外に挙げている）。

Run: `make check`

Expected: 緑

- [ ] **Step 2: `internal/tui/detail` を作る**

`internal/ui/picker.go` と `picker_test.go` を `internal/tui/detail/` へ `git mv` し、
`internal/ui/ui.go` の詳細まわりの状態とロジックを `internal/tui/detail/detail.go` へ、
`internal/ui/render.go` の `detailView` / `composeView` / `confirmView` / `pickerView` /
`stateFooterKey` / `prMarkdown` / `issueMarkdown` / `writeCommonMeta` / `writeBody` /
`writeComments` を `internal/tui/detail/render.go` へ**複製**する
（`internal/ui` 側は Task 10 まで残すので、この時点では複製になる）。

```go
// Source is what the detail view needs from the GitHub layer.
type Source interface {
	GetPR(ctx context.Context, repo string, number int) (gh.PR, error)
	GetIssue(ctx context.Context, repo string, number int) (gh.Issue, error)
	OpenPRWeb(repo string, number int) error
	OpenIssueWeb(repo string, number int) error
	AddPRComment(repo string, number int, body string) error
	AddIssueComment(repo string, number int, body string) error
	ClosePR(repo string, number int) error
	ReopenPR(repo string, number int) error
	CloseIssue(repo string, number int) error
	ReopenIssue(repo string, number int) error
	ListLabels(ctx context.Context, repo string) ([]gh.Label, error)
	ListAssignees(ctx context.Context, repo string) ([]string, error)
	EditPRLabels(repo string, number int, add, remove []string) error
	EditIssueLabels(repo string, number int, add, remove []string) error
	EditPRAssignees(repo string, number int, add, remove []string) error
	EditIssueAssignees(repo string, number int, add, remove []string) error
}

// ClosedMsg tells the parent the user left the detail view.
type ClosedMsg struct{}

// ErrorMsg carries a failure the parent shows on its error screen.
type ErrorMsg struct{ Err error }
```

`Model` に `context.Context` のフィールドを持たせない。

`internal/ui/ui_test.go` のうち詳細・コメント・確認・ピッカーのテストを
`internal/tui/detail/detail_test.go` へ**移す**（`internal/ui` 側からは消す。
`internal/ui` は Task 10 で消えるので、二重に持つ必要は無い）。
`fakeSource` を `detail.Source` のメソッドだけに削る。

Run: `go test ./internal/tui/detail/ -v`

Expected: PASS

- [ ] **Step 3: `internal/tui/repo` を作る**

`internal/ui/ui.go` の一覧部分と `internal/ui/render.go` の `listView` / `prLine` /
`issueLine` / `cursorPrefix` を `internal/tui/repo/` へ移す。

```go
// Source is what the repository list needs from the GitHub layer.
type Source interface {
	ListPRs(ctx context.Context) ([]gh.PR, error)
	ListIssues(ctx context.Context) ([]gh.Issue, error)
	RepoName(ctx context.Context) (string, error)
}

// OpenDetailMsg asks the parent to show the detail view for one item.
type OpenDetailMsg struct{ Ref gh.ItemRef }

// ErrorMsg carries a failure the parent shows on its error screen.
type ErrorMsg struct{ Err error }
```

一覧のテストも `internal/ui/ui_test.go` から `internal/tui/repo/repo_test.go` へ移す。
`View()` は `string` を返すので、テストは `.Content` を付けない。

エラー経路のテストは、「`Source` がエラーを返したとき `Update` が `repo.ErrorMsg` を運ぶ
`tea.Cmd` を返す」ことを見る形に変える（旧 `internal/ui/ui_test.go:232` の意図の移し替え）。
エラー画面そのものの検証は Task 10 の `app` 側で行う。

Run: `go test ./internal/tui/repo/ -v`

Expected: PASS

- [ ] **Step 4: depguard に `repo` と `detail` を足す**

```yaml
        tui-repo:
          files:
            - "**/internal/tui/repo/**"
          deny:
            - pkg: github.com/kukv/octoscope/internal/tui/work
              desc: sibling views communicate through the app model, not directly
            - pkg: github.com/kukv/octoscope/internal/tui/detail
              desc: sibling views communicate through the app model, not directly
            - pkg: github.com/kukv/octoscope/internal/tui/app
              desc: a child view must not import its parent
        tui-detail:
          files:
            - "**/internal/tui/detail/**"
          deny:
            - pkg: github.com/kukv/octoscope/internal/tui/work
              desc: sibling views communicate through the app model, not directly
            - pkg: github.com/kukv/octoscope/internal/tui/repo
              desc: sibling views communicate through the app model, not directly
            - pkg: github.com/kukv/octoscope/internal/tui/app
              desc: a child view must not import its parent
```

`gh.ItemRef` を使うことで、`repo` も `work` も `detail` を import せずに済む。

- [ ] **Step 5: `make check` を通す**

Run: `make check`

Expected: 緑。`internal/ui` はまだ残っており、コードの一部が `internal/tui/*` と重複する。
これは Task 10 で解消する（`unused` の指摘が出たら、`internal/ui` 側の
未使用になった非公開関数だけを消す）。

- [ ] **Step 6: コミット**

```
refactor: split the UI into repo and detail submodels

The single ui.Model held the list, the detail, the compose box and the picker
at once; adding tabs and a board on top of that would not fit. Each view now
owns its own state and talks to its parent through message types. Read methods
take a context so a fetch can be abandoned when the user moves on.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

---

## Task 10: ルートモデルと配線

spec §4（タブ切替）、§3.4-3（`--repo` も git remote も無い場合は Work タブから開始）。

**Files:**
- Create: `internal/tui/app/app.go`、`internal/tui/app/render.go`、`internal/tui/app/app_test.go`
- Modify: `cmd/octoscope/main.go`
- Delete: `internal/ui/`
- Modify: `.golangci.yml`
- Modify: `internal/i18n/locales/active.en.yaml`、`active.ja.yaml`

**Interfaces:**
- Consumes: `work.Model` / `repo.Model` / `detail.Model` とそれぞれの `Source`、`OpenDetailMsg`、`ErrorMsg`
- Produces:
  - `type Source interface { work.Source; repo.Source; detail.Source }`
  - `type Options struct { HasRepo bool }`
  - `func New(src Source, opts Options) Model`
  - `func (m Model) Init() tea.Cmd` / `Update(tea.Msg) (tea.Model, tea.Cmd)` / `View() tea.View`

`app.Model` は `tea.Program` に渡すルートなので、`Update` は `(tea.Model, tea.Cmd)` を、
`View` は `tea.View` を返す（既存の `internal/ui/ui.go:697` と同じ形）。

- [ ] **Step 1: カタログへタブ名を足す**

Go にタブ名を直書きしないため、両カタログに同じ値で置く。

`active.en.yaml` / `active.ja.yaml` の両方に:

```yaml
tab:
  # Tab names double as the labels for the 1 / 2 keys, so they stay the same
  # in both languages (see spec §6.1).
  work:
    other: "Work"
  repos:
    other: "Repos"
```

- [ ] **Step 2: 失敗するテストを書く**

`internal/tui/app/app_test.go`:

```go
package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kukv/octoscope/internal/gh"
	"github.com/kukv/octoscope/internal/i18n"
	"github.com/kukv/octoscope/internal/tui/repo"
	"github.com/kukv/octoscope/internal/tui/work"
)

// fakeSource satisfies Source with just enough to build the model. The child
// views have their own tests; here we only exercise the root's routing.
type fakeSource struct{}

func (fakeSource) ListWork(context.Context) (gh.Work, error) { return gh.Work{}, nil }

func (fakeSource) ListPRs(context.Context) ([]gh.PR, error)       { return nil, nil }
func (fakeSource) ListIssues(context.Context) ([]gh.Issue, error) { return nil, nil }
func (fakeSource) RepoName(context.Context) (string, error)       { return "kukv/demo", nil }

// ... the remaining detail.Source methods return zero values.

func newTestModel(opts Options) Model {
	m := New(fakeSource{}, opts)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return next.(Model)
}

func press(m Model, k string) Model {
	next, _ := m.Update(tea.KeyPressMsg{Code: []rune(k)[0], Text: k})
	return next.(Model)
}

func TestStartsOnTheWorkTab(t *testing.T) {
	if m := newTestModel(Options{HasRepo: true}); m.tab != tabWork {
		t.Errorf("tab: got %v, want tabWork", m.tab)
	}
}

func TestTabKeysSwitchTabs(t *testing.T) {
	m := press(newTestModel(Options{HasRepo: true}), "2")
	if m.tab != tabRepos {
		t.Errorf("after 2: got %v, want tabRepos", m.tab)
	}
	if m = press(m, "1"); m.tab != tabWork {
		t.Errorf("after 1: got %v, want tabWork", m.tab)
	}
}

// TestReposTabIsUnreachableWithoutARepository guards spec §3.4: with neither
// --repo nor a git remote there is nothing for the Repos tab to show, so the
// app stays on Work rather than surfacing gh's "no git remotes found".
func TestReposTabIsUnreachableWithoutARepository(t *testing.T) {
	m := press(newTestModel(Options{HasRepo: false}), "2")
	if m.tab != tabWork {
		t.Errorf("tab: got %v, want it to stay on tabWork", m.tab)
	}
	if strings.Contains(m.View().Content, i18n.T("tab.repos")) {
		t.Error("the Repos tab is offered even though no repository is known")
	}
}

func TestOpenDetailMsgShowsTheDetailView(t *testing.T) {
	m := newTestModel(Options{HasRepo: true})
	next, _ := m.Update(work.OpenDetailMsg{
		Ref: gh.ItemRef{Kind: gh.ItemPR, Repo: "kukv/koto", Number: 3},
	})
	m = next.(Model)
	if !m.showingDetail {
		t.Fatal("the detail view did not open")
	}
	next, _ = m.Update(detail.ClosedMsg{})
	if next.(Model).showingDetail {
		t.Error("the detail view did not close on ClosedMsg")
	}
}

func TestErrorMsgShowsTheErrorScreen(t *testing.T) {
	for name, msg := range map[string]tea.Msg{
		"work": work.ErrorMsg{Err: errors.New("boom")},
		"repo": repo.ErrorMsg{Err: errors.New("boom")},
	} {
		t.Run(name, func(t *testing.T) {
			next, _ := newTestModel(Options{HasRepo: true}).Update(msg)
			view := next.(Model).View().Content
			if !strings.Contains(view, "boom") {
				t.Errorf("the error screen does not show the message:\n%s", view)
			}
			if !strings.Contains(view, i18n.T("app.error_title")) {
				t.Errorf("the error screen has no title:\n%s", view)
			}
		})
	}
}

func TestGhNotFoundIsTranslated(t *testing.T) {
	next, _ := newTestModel(Options{HasRepo: true}).
		Update(work.ErrorMsg{Err: gh.ErrGhNotFound})
	view := next.(Model).View().Content
	if !strings.Contains(view, i18n.T("error.gh_not_found")) {
		t.Errorf("gh_not_found was not translated:\n%s", view)
	}
}
```

環境の不備（`gh.ErrGhNotFound`）を案内へ差し替える `errors.Is` の判定は、
`internal/ui/ui.go` の既存実装から `internal/tui/app` へ移す（`.claude/rules/errors.md`）。

- [ ] **Step 3: 落ちることを確認する**

Run: `go test ./internal/tui/app/ -v`

Expected: FAIL（パッケージが無い）

- [ ] **Step 4: `app.go` を実装する**

```go
// Package app is the root model: it owns the tabs, hands each child its size,
// and shows the error screen.
package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kukv/octoscope/internal/tui/detail"
	"github.com/kukv/octoscope/internal/tui/repo"
	"github.com/kukv/octoscope/internal/tui/work"
)

// Source is the union of what the child views need. Each view takes only its
// own slice of it.
type Source interface {
	work.Source
	repo.Source
	detail.Source
}

// Options carries what main determined before the UI started.
type Options struct {
	// HasRepo reports whether a target repository is known, from --repo or
	// from the git remote of the working directory. Without one the Repos tab
	// has nothing to show and is not offered (spec §3.4).
	HasRepo bool
}

type tabID int

const (
	tabWork tabID = iota
	tabRepos
)

type Model struct {
	src  Source
	opts Options

	width, height int
	tab           tabID

	spin   spinner.Model
	work   work.Model
	repo   repo.Model
	detail detail.Model

	showingDetail bool
	errText       string
}

func New(src Source, opts Options) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return Model{
		src:  src,
		opts: opts,
		spin: s,
		work: work.New(src),
		repo: repo.New(src),
	}
}

func (m Model) Init() (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.spin.Tick}
	var cmd tea.Cmd
	m.work, cmd = m.work.Refresh()
	cmds = append(cmds, cmd)
	if m.opts.HasRepo {
		cmds = append(cmds, m.repo.Init())
	}
	return m, tea.Batch(cmds...)
}
```

`Init` の署名は Bubble Tea v2 の `tea.Model` に合わせる
（既存の `internal/ui/ui.go` の `Init` がどちらの形かを見て揃える。
`func (m Model) Init() tea.Cmd` ならば、`work` の最初の取得は
`Update` が最初の `tea.WindowSizeMsg` を受けたときに `Refresh()` して行う）。

`Update` の骨子:

- `tea.WindowSizeMsg`: サイズを覚えて全サブモデルへ配る
- `tea.KeyPressMsg`:
  - `ctrl+c` はいつでも `m.work.Cancel()` してから `tea.Quit`
  - `m.showingDetail` のときは `detail` にだけ渡す（`q` も渡す。コメント入力中の `q` を
    終了に取られないため）
  - それ以外で `q` なら `m.work.Cancel()` して `tea.Quit`
  - `"1"` → `tabWork`、`"2"` → `m.opts.HasRepo` のときだけ `tabRepos`
  - 残りは現在のタブのサブモデルへ渡す
- `work.OpenDetailMsg` / `repo.OpenDetailMsg`:
  `m.detail = detail.New(m.src, msg.Ref)`、`m.showingDetail = true`、`m.detail.Init()` を返す
- `detail.ClosedMsg`: `m.showingDetail = false`
- `work.ErrorMsg` / `repo.ErrorMsg` / `detail.ErrorMsg`: `m.errText = errorText(err)` にして
  エラー画面へ。`errorText` は `errors.Is(err, gh.ErrGhNotFound)` のとき
  `i18n.T("error.gh_not_found")` を返し、それ以外は `err.Error()` をそのまま返す
- `spinner.TickMsg`: `m.spin` を進める

`render.go` には `errorView`（`internal/ui/render.go` から移設）とタブ行の描画を置く。
タブ行は `HasRepo` が false のとき `i18n.T("tab.work")` だけを出す。

- [ ] **Step 5: 通ることを確認する**

Run: `go test ./internal/tui/app/ -v`

Expected: PASS

- [ ] **Step 6: `main.go` を配線する**

```go
	client := cli.New(dir, *repoFlag)

	p := tea.NewProgram(app.New(client, app.Options{
		HasRepo: hasRepo(client, *repoFlag),
	}))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
```

```go
// repoLookupTimeout bounds the one gh call main makes before the UI starts.
const repoLookupTimeout = 5 * time.Second

// hasRepo reports whether a target repository is known. An explicit --repo
// settles it; otherwise we ask gh, which resolves the git remote of the
// working directory and fails when there is none.
func hasRepo(c *cli.Client, flagRepo string) bool {
	if flagRepo != "" {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), repoLookupTimeout)
	defer cancel()
	_, err := c.RepoName(ctx)
	return err == nil
}
```

`repo` というローカル変数名がパッケージ名と衝突しうるので、フラグの変数名を `repoFlag` に変える。

- [ ] **Step 7: `internal/ui` を消す**

`git rm -r internal/ui` し、`.golangci.yml` から `internal/ui` への言及をすべて消す。
`internal/tui/app` 用の depguard ルールは不要（親は全ての子を import してよい）。

- [ ] **Step 8: `make check` を通す**

Run: `make check`

Expected: 緑。カバレッジが 80% を下回る場合は、
`app` の `Update` 分岐（タブ切替、詳細の開閉、エラー、`q` の扱い）にテストを足す。
数字を埋めるためのテストは書かない — 落ちたときはまず「そのコードは要るのか」を疑う。

- [ ] **Step 9: 実際に動かす**

```bash
go run ./cmd/octoscope
go run ./cmd/octoscope --lang ja
go run ./cmd/octoscope --repo kukv/koto
```

確認すること:

- 起動直後が Work タブで、4 列が出ている
- `h`/`l`/`j`/`k` でカーソルが動き、ドロワーが選択に追従する
- `enter` で詳細が開き、`esc` で戻る
- `2` で Repos タブに移り、既存の PR/Issue 一覧が Phase 0 と同じに見える
- `--lang ja` で桁がずれない
- 80 桁の端末でドロワーが畳まれ、60 桁未満で 1 列表示になる
- リポジトリでないディレクトリで起動して、Work タブが出て Repos タブが提示されないこと

  ```bash
  go build -o /tmp/octoscope ./cmd/octoscope
  cd /tmp && ./octoscope
  ```

- [ ] **Step 10: コミット**

```
feat: add the root model with Work and Repos tabs

The tabs live here and the children talk to it through message types; the old
internal/ui is gone. Without a target repository the Repos tab has nothing to
show, so the app starts and stays on Work instead of surfacing gh's "no git
remotes found" (spec §3.4).

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

---

## Task 11: 日本語と幅の通し確認

**Files:**
- Modify: 必要に応じて `internal/tui/*/render.go`、`internal/i18n/locales/active.ja.yaml`

- [ ] **Step 1: 全ビューの幅テストが揃っていることを確認する**

`internal/tui/work`、`internal/tui/repo`、`internal/tui/detail`、`internal/tui/app` の各テストに、
en / ja の両方で端末幅を超える行が無いことを見るテストがあること。
無いパッケージには Task 8 Step 6 の `TestNoLineExceedsTheTerminalWidth` と同じ形で足す
（`app` は `View().Content` を分割する）。

- [ ] **Step 2: 未解決 ID のテストが全ビューに揃っていることを確認する**

同じく各パッケージに `i18n.AssertNoUnresolvedIDs` を使うテストがあること。
Task 1 で `internal/ui` に置いた `renderEveryScreen` の役目は、分割後は各パッケージのテストに散る。
`detail` は詳細 / コメント入力 / 確認 / ピッカーの 4 画面すべてを走査すること。

- [ ] **Step 3: 実際に狭い端末で動かす**

```bash
go run ./cmd/octoscope --lang ja
```

50 桁 / 80 桁 / 100 桁 / 120 桁で、カンバンのカードと Repos タブの一覧が崩れないことを目で見る。

- [ ] **Step 4: コミット（差分があれば）**

```
fix: keep the Japanese views inside the terminal width

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

---

## Task 12: spec の更新

**Files:**
- Modify: `docs/superpowers/specs/2026-09-05-octoscope-standalone-design.md`

- [ ] **Step 1: §3.2 を実装に合わせる**

「`internal/gh` に `Client` interface を定義し、2 つの実装を差し替え可能にする。」を
次の趣旨へ書き換える。

> `internal/gh` はドメイン型だけを公開し、interface は置かない。
> バックエンド（`cli` / `api`）は同じドメイン型を返す具体型として実装し、
> 差し替えは各ビューが宣言する利用側 interface を両実装が満たすことで達成する
> （`.claude/rules/architecture.md`「interface は利用側で定義する」）。

§3.1 の表の `internal/gh/` の説明も
「GitHub アクセスの抽象（interface とドメイン型）」から
「GitHub アクセス層が返すドメイン型」へ直す。

- [ ] **Step 2: §3.1 のパッケージ構成に実際の配置を反映する**

`internal/tui/` の下に `icon/` を足す。Phase 1 で作った `app` / `work` / `repo` / `detail` と、
まだ無い `repos`（サイドバー込みの Repos タブ）/ `search` / `dialog` / `diff` の別を書く。

- [ ] **Step 3: §4 にタブの段階的な追加を書く**

「3 つのタブを並列に持ち、`1` / `2` / `3` で切り替える」に次を足す。

- Search タブは Phase 4 で追加される
- 対象リポジトリが決まらないときは Repos タブを出さず、Work タブだけで動く

- [ ] **Step 4: §6.1 にタブ名の扱いを足す**

翻訳しないものの表に「タブ名（`Work` / `Repos` / `Search`）。キー `1` / `2` / `3` と対で
覚える画面上の識別子であるため」を足す。

- [ ] **Step 5: コミット**

```
docs: align the spec with where the interfaces actually live

architecture.md asks the consumer to declare the interface, so internal/gh
holds domain types only. Record that, the packages Phase 1 added, and that
tabs arrive one phase at a time.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

- [ ] **Step 6: PR 4 を出す**

`git push -u origin feat/octoscope-phase1-work-tab` のあと
`gh pr create --label "Kind: Feature"`。本文:

```
Phase 1 の本体。複数リポジトリを横断して「自分に関係する仕事」を 4 列で見せる Work タブを足した。

- `search(type: ISSUE)` の 4 エイリアスを 1 リクエストにまとめて取得（spec §3.3）。
  `reviewDecision` と `statusCheckRollup` は GraphQL でしか取れない
- GraphQL の enum は `internal/gh` でドメイン値へ変換し、TUI に文字列を漏らさない
- `internal/ui` の 1 枚モデルを `internal/tui/{app,work,repo,detail}` へ分割（spec §3.1）
- 取得系のメソッドに `context.Context` を通し、再取得と終了でキャンセルする
- `--repo` も git remote も無いときは Repos タブを出さず Work タブから始める（spec §3.4）
- 幅に応じてドロワーを畳み、60 桁未満では 1 列表示に切り替える（spec §4.6）

**spec の更新を同梱している。** §3.2 の「`internal/gh` に `Client` interface」を、
`.claude/rules/architecture.md` の「interface は利用側で定義する」に合わせて書き換えた。

Plan: `docs/superpowers/plans/2026-09-06-octoscope-phase1.md` Task 5–12

## 確認したこと

- `make check` が緑
- `go run ./cmd/octoscope` / `--lang ja` / `--repo kukv/koto` を実際に起動
- 50 / 80 / 100 / 120 桁で桁ずれが無い
- リポジトリでないディレクトリで起動して Work タブから始まる

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

---

## Spec coverage（Phase 1 の項目、spec §7）

| spec の項目 | 対応するタスク |
|---|---|
| 翻訳漏れ検出ガード（§6.5） | Task 1、Task 8 Step 6、Task 11 Step 2 |
| 日時書式のカタログ化（§6.1） | Task 2 |
| `internal/gh` の interface 化、`ghcli` の移設（§3.1、§3.2） | Task 3、Task 4、Task 12 Step 1 |
| GraphQL search によるリポジトリ横断取得（§3.3） | Task 5、Task 6 |
| Work タブのカンバン（4 列 + ドロワー）（§4.1） | Task 8 |
| タブ切替を持つルートモデル（§4） | Task 10 |
| リポジトリ非依存の Work タブから開始（§3.4-3） | Task 10 Step 2、Step 6 |
| サブモデルへの分割（§3.1） | Task 9、Task 10 |
| 装飾: checks 進捗バー、状態アイコン（§4.5 の一部） | Task 7 |
| 幅への対応（§4.6 の 2 番目と 3 番目） | Task 8 Step 7 |
| 表示幅の検証（§6.4、§6.5） | Task 8 Step 6、Task 11 |

**Phase 1 で扱わない spec の項目:** §4.2 のサイドバーと追加ダイアログ、§4.3 Search タブ、
§4.5 のスパークラインとラベル実色バッジと Nerd Font、§4.6 の 1 番目（サイドバーを畳む）、
§5 設定ファイル、§6.3-2（設定ファイルの `language`）。いずれも Phase 2 以降。
