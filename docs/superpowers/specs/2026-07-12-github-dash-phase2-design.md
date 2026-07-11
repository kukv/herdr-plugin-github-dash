# GitHub Dash — Phase 2 設計

herdr のペイン内 GitHub プラグイン、Phase 2 の設計。
Phase 1（読み取り専用の一覧・詳細）は実装済み（[Phase 1 設計](2026-07-11-github-dash-design.md)）。
Phase 2 で初めて書き込み操作を導入する（設計セッション: 2026-07-12）。

## 決定事項

| 論点 | 決定 |
| --- | --- |
| スコープ | 詳細画面から、表示中の PR / Issue へ**コメントを投稿**する。それのみ |
| 入力方法 | インライン `textarea`（bubbles/textarea）。詳細画面内で入力する |
| 投稿フロー | 詳細画面で `c` → textarea を開く → `Ctrl+S` 送信 / `Esc` キャンセル |
| compose 状態の保持 | `screenDetail` のまま `composing bool` フラグで分岐（新規 screen は追加しない） |
| 投稿失敗時の挙動 | 全画面エラーに落とさず、compose を維持し下書きを温存して再送可能にする |
| 依存追加 | なし。bubbles/textarea は既存 bubbles モジュール内 |

`gh` CLI にコメント投稿も委譲する。Phase 1 で確立した「`exec.Cmd.Dir` に対象ディレクトリを設定して
`gh` を実行し、`--repo` は override 時のみ付ける」パターンをそのまま延長する。

## スコープ

**含む（Phase 2）**

- 詳細画面（PR / Issue、直行モード含む）から、表示中の PR / Issue へコメントを投稿
- インライン textarea による複数行入力、`Ctrl+S` 送信 / `Esc` キャンセル
- 投稿成功後に詳細を再取得し、新しいコメントを反映
- 投稿失敗時に compose を維持し下書きを残して再送

**含まない**

- 一覧画面からのコメント投稿（詳細画面のみ）
- コメントの編集・削除
- 状態変更（close / reopen）、ラベル付け替え、アサイン変更（将来 Phase）
- 下書きの永続化（キャンセル・プロセス終了で破棄）

## アーキテクチャ

Phase 1 の3層構成（`herdrctx` / `ghcli` / `ui`）を維持し、`ghcli` と `ui` に追加する。
`herdrctx` と `main.go` は変更しない。

### `ghcli`（write メソッドを追加）

既存の `ghcli` が種別ごとに別メソッドを持つ設計（`GetPR` / `GetIssue`、
`OpenPRWeb` / `OpenIssueWeb`）にそのまま倣い、PR / Issue で別メソッドにする。
こうすると `ghcli` 側に種別 enum を持ち込む必要がなく、`ui.DataSource` を
`*ghcli.Client` が引き続き**直接**満たす（`main.go` の配線は無変更のまま）。

```go
// AddPRComment posts a comment to a pull request.
// repo は "owner/repo"。空文字列ならワークスペースのリポジトリ（--repo を付けない）。
func (c *Client) AddPRComment(repo string, number int, body string) error

// AddIssueComment posts a comment to an issue.
func (c *Client) AddIssueComment(repo string, number int, body string) error
```

- PR: `gh pr comment <n> --body <body>`、Issue: `gh issue comment <n> --body <body>`
- `repo` が非空なら末尾に `--repo <repo>` を付ける（既存 `appendRepo` を再利用）
- `body` は `exec.Command` の引数として渡す。シェルを経由しないため複数行・特殊文字も安全
- 戻り値はエラーのみ（`gh comment` は投稿したコメントの URL を stdout に返すが使わない）

### `ui`（詳細画面に compose サブ状態を追加）

Model に以下を追加:

```go
textarea  textarea.Model // bubbles/textarea
composing bool           // 詳細画面で compose 中か
posting   bool           // 送信中（スピナー表示）
postErr   string         // 直近の投稿失敗メッセージ（compose 内に表示）
```

`DataSource` インターフェースに追加（`fetchDetail` / `openWeb` と同じく `ui` 側で
`target.Kind` により PR / Issue を振り分けて呼ぶ）:

```go
AddPRComment(repo string, number int, body string) error
AddIssueComment(repo string, number int, body string) error
```

新規 msg 型:

```go
commentPostedMsg struct{}          // 投稿成功
commentErrorMsg  struct{ err error } // 投稿失敗（compose を維持）
```

**投稿失敗を全画面 `screenError` に落とさない理由**: `errorMsg` は `screen = screenError`
に遷移させるため、入力中のコメント下書きが失われる。`commentErrorMsg` は
`composing` を維持したまま `postErr` にメッセージを入れて compose 内に表示し、
下書きを残して再送を可能にする。

## データフロー（投稿）

1. 詳細画面で `c` → `composing = true`、textarea を `Reset()` + `Focus()`。
   ただし `loading` 中、または `detailTarget` が未確定のときは無視する
2. compose 中のキー処理:
   - `Esc` → キャンセル。`composing = false`、下書き破棄、`postErr` クリア
   - `Ctrl+S` → `strings.TrimSpace(textarea.Value())` が空なら無視して compose 継続。
     非空なら `posting = true`・`postErr` クリアして `postComment` cmd を発行
   - その他 → `textarea.Update(msg)` に委譲
3. `postComment(src, target, body)` cmd → `target.Kind` で `src.AddPRComment` /
   `src.AddIssueComment` を振り分けて呼ぶ:
   成功で `commentPostedMsg{}`、失敗で `commentErrorMsg{err}`
4. `commentPostedMsg` → `composing = false`・`posting = false`・textarea クリア・
   `loading = true` にして `fetchDetail(src, detailTarget)` を返す（新コメント反映）
5. `commentErrorMsg` → `posting = false`（`composing` は維持）・
   `postErr = err.Error()`・下書き温存

compose 中の描画は `detailView` から分岐（`composing` 時は textarea + ヘルプ + `postErr` を表示）。

## キーバインド

| キー | 詳細画面 | compose 中 |
| --- | --- | --- |
| `c` | コメント作成を開く | —（textarea へ入力） |
| `Ctrl+S` | — | 送信 |
| `Esc` | 一覧に戻る | 作成をキャンセル |
| `Ctrl+C` | 終了 | 終了 |

- 詳細画面フッターに `c:comment` を追記
- compose 中フッター: `Ctrl+S:send  Esc:cancel`（`posting` 中はスピナー + `posting...`）

## エラーハンドリング

| 状況 | 挙動 |
| --- | --- |
| 空本文（空白のみ）で `Ctrl+S` | 送信せず compose 継続 |
| `gh comment` 失敗（未認証・権限なし・リモートなし等） | `commentErrorMsg` で compose 内にエラー行を表示、下書き温存、再送可 |
| 投稿成功 | 詳細を再取得しコメント一覧を更新 |

## テスト

- **ghcli**（ホワイトボックス、既存 `fakeRun` を利用）:
  - `TestAddPRComment` / `TestAddIssueComment`: `repo` override 有無で `run` に渡る args を検証
    （例 PR override なし: `["pr","comment","12","--body","hello"]`、
    override 有: 末尾に `"--repo","octo/hello"`）
  - `TestAddCommentError`: `run` のエラーが透過することを確認
- **ui**（既存 `fakeSource` に `AddPRComment` / `AddIssueComment` を追加し呼び出しを記録）:
  - `c` で `composing` に遷移する
  - 空本文で `Ctrl+S` → 送信されず compose 継続（cmd なし / AddPRComment 未呼出）
  - 非空本文で `Ctrl+S` → `posting`、cmd 実行で AddPRComment 呼出、
    続く `commentPostedMsg` で `composing=false` かつ再取得 cmd 発行
  - `Esc` で compose キャンセル（`composing=false`）
  - 投稿失敗（`commentErrorMsg`）で `composing` 維持・`postErr` セット・下書き保持
- **README**: 詳細画面の `c` キーを追記

## 依存

bubbles/textarea は Phase 1 で導入済みの `github.com/charmbracelet/bubbles` モジュール内の
サブパッケージ（spinner / viewport と同じ）。**go.mod への新規依存追加は発生しない**
（charmbracelet 4依存 = bubbletea / bubbles / lipgloss / glamour の制約を維持）。

## 実機検証で確認する残項目

- 実機（herdr 0.7.x + gh 2.95）で `c` → 入力 → `Ctrl+S` の一連が動くこと
- ターミナル/herdr のキーグラブにより `Ctrl+S`（XOFF）が届かない可能性。
  届かない場合は代替キー（例 `Ctrl+D`）を検討する — 実装後の手動確認事項とする
