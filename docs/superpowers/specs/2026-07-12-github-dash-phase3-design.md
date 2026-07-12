# GitHub Dash — Phase 3 設計

herdr のペイン内 GitHub プラグイン、Phase 3 の設計。
Phase 1（読み取り専用の一覧・詳細）、Phase 2（コメント投稿）は実装済み
（[Phase 2 設計](2026-07-12-github-dash-phase2-design.md)）。
Phase 3 で PR / Issue の**状態変更（close / reopen）**を導入する（設計セッション: 2026-07-12）。

## 決定事項

| 論点 | 決定 |
| --- | --- |
| スコープ | 詳細画面から、表示中の PR / Issue を **close / reopen** する。それのみ |
| キー | 状態連動の単一キー `x`（Open→close、Closed→reopen、merged→無効） |
| 確認 | `y/n` 確認プロンプトを挟む。`composing` と同じ要領で `confirming` サブ状態を追加 |
| confirm 状態の保持 | `screenDetail` のまま `confirming bool` フラグで分岐（新規 screen は追加しない） |
| 失敗時の挙動 | 全画面エラーに落とさず、詳細画面に留まり `actionErr` をインライン表示して再試行可能にする |
| 依存追加 | なし |

Phase 1/2 で確立した「`exec.Cmd.Dir` に対象ディレクトリを設定して `gh` を実行し、
`--repo` は override 時のみ付ける」パターン、および `ghcli` の種別ごと別メソッド設計を延長する。

なぜラベル・アサインを含めないか: それらは「候補一覧取得＋複数選択ピッカー UI」という
別パターンで実装の大半を共有するため、状態変更（確認ダイアログパターン）とは分けて
Phase 4 で扱う。本 Phase は close/reopen に集中する。

## スコープ

**含む（Phase 3）**

- 詳細画面（PR / Issue、直行モード含む）から、表示中の PR / Issue を close / reopen
- 実行前の `y/n` 確認プロンプト
- 状態変更成功後に詳細を再取得し、新しい状態を反映（フッターの `x:close` ↔ `x:reopen` が反転）
- 失敗時に詳細画面へ留まり `actionErr` を表示して再試行

**含まない**

- 一覧画面からの状態変更（詳細画面のみ）
- PR の merge、comment-and-close（`gh pr close --comment`）
- ラベル付け替え・アサイン変更（Phase 4、共有ピッカー）
- コメントの編集・削除（将来 Phase）

## アーキテクチャ

Phase 1/2 の3層構成（`herdrctx` / `ghcli` / `ui`）を維持し、`ghcli` と `ui` に追加する。
`herdrctx` と `main.go` は変更しない。

### `ghcli`（状態変更メソッドを追加）

既存の種別ごと別メソッド設計（`GetPR`/`GetIssue`、`OpenPRWeb`/`OpenIssueWeb`、
`AddPRComment`/`AddIssueComment`）にそのまま倣い、種別 × アクションで別メソッドにする。
`ghcli` 側に種別／アクションの enum を持ち込まず、`ui.DataSource` を `*ghcli.Client` が
引き続き直接満たす（`main.go` の配線は無変更のまま）。

```go
// ClosePR closes a pull request.
// repo は "owner/repo"。空文字列ならワークスペースのリポジトリ（--repo を付けない）。
func (c *Client) ClosePR(repo string, number int) error     // gh pr close <n>
func (c *Client) ReopenPR(repo string, number int) error    // gh pr reopen <n>
func (c *Client) CloseIssue(repo string, number int) error  // gh issue close <n>
func (c *Client) ReopenIssue(repo string, number int) error // gh issue reopen <n>
```

- `repo` が非空なら末尾に `--repo <repo>` を付ける（既存 `appendRepo` を再利用）
- 戻り値はエラーのみ（stdout は使わない）。`AddPRComment` と同じ1行実装

### `ui`（詳細画面に confirm サブ状態を追加）

Model に以下を追加:

```go
detailState string // 表示中 item の状態 "OPEN"/"CLOSED"/"MERGED"
confirming  bool   // y/n 確認中
working     bool   // gh 実行中（スピナー表示・二重実行防止）
actionErr   string // 直近の状態変更失敗メッセージ（詳細内に表示）
```

`detailState` は `prDetailMsg` / `issueDetailMsg` 受信時に item の `State` から設定する。
（PR は `"OPEN"`/`"CLOSED"`/`"MERGED"`、Issue は `"OPEN"`/`"CLOSED"`。`gh view` の
`state` フィールドは既存の view fields に含まれるため、gh コマンドの変更は不要。）

`DataSource` インターフェースに追加（`fetchDetail` / `postComment` と同じく `ui` 側で
`target.Kind` により PR / Issue を振り分けて呼ぶ）:

```go
ClosePR(repo string, number int) error
ReopenPR(repo string, number int) error
CloseIssue(repo string, number int) error
ReopenIssue(repo string, number int) error
```

新規 msg 型:

```go
stateChangedMsg struct{}          // 状態変更成功
stateErrorMsg   struct{ err error } // 状態変更失敗（詳細に留まる）
```

**状態変更失敗を全画面 `screenError` に落とさない理由**: `errorMsg` は `screen = screenError`
に遷移し、そのキーは終了のみ（`q`/`esc`/`ctrl+c` → Quit）なので、回復可能なアクション失敗で
アプリが終了してしまう。`stateErrorMsg` は詳細画面に留めて `actionErr` にメッセージを入れ、
コンテキストを失わず再試行できるようにする（Phase 2 の `postErr` と同じ思想）。

## データフロー（状態変更）

1. 詳細画面で `x` → `stateAction()` を評価。`detailState` が:
   - `"OPEN"` → close 対象、`"CLOSED"` → reopen 対象、それ以外（`"MERGED"`/空）→ アクション無しで無視
   - 有効時のみ `confirming = true`、`actionErr` クリア。`detailLoading` 中は無視
2. confirm 中のキー処理（`working` 中は `ctrl+c` 以外を無視）:
   - `y` → `working = true`・`actionErr` クリアして `setState` cmd を発行
   - `n` / `esc` → キャンセル。`confirming = false`、`actionErr` クリア
   - `ctrl+c` → 終了
3. `setState(src, target, closing bool)` cmd → `target.Kind` と `closing` で4メソッドを振り分け:
   成功で `stateChangedMsg{}`、失敗で `stateErrorMsg{err}`
   （`closing` は `detailState == "OPEN"` から算出）
4. `stateChangedMsg` → `confirming = false`・`working = false`・`actionErr` クリア・
   `detailLoading = true` にして `fetchDetail(src, detailTarget)` を返す（新状態反映）
5. `stateErrorMsg` → `confirming = false`・`working = false`（詳細に留まる）・
   `actionErr = err.Error()`

`enter`（一覧→詳細）と詳細再ロード時は `confirming`/`actionErr` をクリアして
状態を持ち越さない。compose の `composing` 分岐と同様、confirm 中の描画は
`detailView` から `confirmView` に分岐する。

## キーバインド

| キー | 詳細画面 | confirm 中 |
| --- | --- | --- |
| `x` | close / reopen 確認を開く（状態連動、merged 時は無効） | — |
| `y` | — | 実行 |
| `n` / `esc` | — | キャンセル |
| `esc` | 一覧に戻る | 上記（キャンセル） |
| `ctrl+c` | 終了 | 終了 |

- 詳細画面フッターに状態連動キーを挿入: OPEN→`x:close`、CLOSED→`x:reopen`、MERGED→表示なし
- confirm 中フッター/プロンプト: `Close this PR? (y/n)` 等（`working` 中はスピナー + `working...`）

## エラーハンドリング

| 状況 | 挙動 |
| --- | --- |
| merged PR で `x` | アクション無しで無視（confirm を開かない） |
| `detailLoading` 中に `x` | 無視 |
| `gh close`/`reopen` 失敗（未認証・権限なし・リモートなし等） | `stateErrorMsg` で詳細に `actionErr` 行を表示、再試行可 |
| 状態変更成功 | 詳細を再取得し状態表示（フッター）を更新 |

## テスト

- **ghcli**（ホワイトボックス、既存 `fakeRun` を利用）:
  - `TestClosePR`: override なしで `run` に渡る args を検証（`["pr","close","12"]`、`dir == /repo`）
  - `TestReopenIssueWithRepoOverride`: override 有で末尾に `["--repo","octo/hello"]`
  - `TestStateChangeError`: `run` のエラーが透過することを確認
- **ui**（既存 `fakeSource` に4メソッドを追加し `stateCalls`/`stateErr` で呼び出しを記録。
  記録書式は `"close:pr::1"` / `"reopen:issue:octo/hello:3"` の `action:kind:repo:number`）:
  - Open 表示中に `x` で `confirming` に遷移する
  - merged 表示中の `x` は無視（`confirming` false のまま）
  - confirm 中 `y` → `working`、cmd 実行で該当メソッド呼出（`stateCalls==["close:pr::1"]`）、
    続く `stateChangedMsg` で `confirming=false` かつ再取得 cmd 発行（`detailLoading=true`）
  - confirm 中 `n` / `esc` でキャンセル（`confirming=false`、状態変更未呼出）
  - 状態変更失敗（`stateErrorMsg`）で詳細に留まり `actionErr` セット・`working=false`・
    `screen==screenDetail`
- **render**:
  - `confirmView` にプロンプト（`Close`/`Reopen` と `(y/n)`）が表示される
  - 詳細フッターに OPEN→`x:close`、CLOSED→`x:reopen`
  - `actionErr` が非空のとき詳細にインライン表示される
- **README**: 詳細画面の `x` キーを Keys 表に追記

## 依存

新規依存なし。confirm プロンプトは既存の描画（`titleStyle`/`dimStyle`/`spin`）で表現でき、
bubbles/textarea のような追加サブパッケージも不要。charmbracelet 4依存
（bubbletea / bubbles / lipgloss / glamour）の制約を維持する。

## 実機検証で確認する残項目

- 実機（herdr 0.7.x + gh 2.95）で詳細画面 `x` → `y` の close/reopen が動き、
  再取得後にフッターの状態表示（`x:close` ↔ `x:reopen`）が反転すること
- `x` / `y` / `n` が herdr/ターミナルのキーグラブに奪われないこと（奪われる場合は代替キーを検討）
