# GitHub Dash — Phase 4 設計

herdr のペイン内 GitHub プラグイン、Phase 4 の設計。
Phase 1（読み取り専用）、Phase 2（コメント投稿）、Phase 3（close/reopen）は実装済み
（[Phase 3 設計](2026-07-12-github-dash-phase3-design.md)）。
Phase 4 で PR / Issue の**ラベル／アサインの付け替え**を、共有の複数選択ピッカーで導入する
（設計セッション: 2026-07-12）。

## 決定事項

| 論点 | 決定 |
| --- | --- |
| スコープ | 詳細画面から、表示中の PR / Issue の **ラベル** と **アサイン** を付け替える。それのみ |
| 入力方法 | 共有の複数選択ピッカー（新規 `internal/ui/picker.go` の小型コンポーネント） |
| 候補取得 | ラベル: `gh label list`。アサイン: `gh api repos/{owner}/{repo}/assignees`（アサイン可能ユーザー） |
| 適用方法 | `space` でトグル → `enter` で現在値との差分を **1 回の `gh edit`** に適用（add/remove まとめて） |
| picking 状態の保持 | `screenDetail` のまま `picking bool` フラグで分岐（新規 screen は追加しない） |
| 失敗時の挙動 | 適用失敗はピッカーを維持して選択を温存・再送可能。候補取得失敗は詳細に留まりインライン表示 |
| ラベル色 | ピッカーで lipgloss を使い色付き表示 |
| 依存追加 | なし |

Phase 1〜3 のパターン（`exec.Cmd.Dir` を設定して `gh` を実行し `--repo` は override 時のみ、
`ghcli` は種別ごと別メソッド、詳細画面のサブ状態で書き込みを扱う）を延長する。

## スコープ

**含む（Phase 4）**

- 詳細画面（PR / Issue、直行モード含む）から、表示中の PR / Issue のラベル／アサインを付け替える
- 候補一覧を取得し、現在値をプリチェックした複数選択ピッカーで add/remove をトグル
- `enter` で差分を 1 回の `gh edit` に適用し、詳細を再取得して反映
- 適用失敗時にピッカーを維持し選択を温存して再送。候補取得失敗時は詳細にインラインエラー

**含まない**

- 新規ラベルの作成、マイルストーン・プロジェクト・レビュアーの変更
- 一覧画面からの編集（詳細画面のみ）
- 状態変更（Phase 3 で実装済み）、コメント編集・削除（将来 Phase）

## アーキテクチャ

Phase 1〜3 の3層構成（`herdrctx` / `ghcli` / `ui`）を維持。`herdrctx` と `main.go` は変更しない。
ピッカーは境界の明確な小型コンポーネントとして `internal/ui/picker.go` に分離し、
`ui.go` / `render.go` の肥大化を防ぐ。

### `ghcli`

**構造体の拡張**: `PR` / `Issue` に現在のアサインを取得するフィールドを追加する
（ラベルは既に `Labels` として取得済み。アサインはプリチェックのために追加が必要）。

```go
Assignees []Author `json:"assignees"`
```

view fields に `assignees` を追加する（`prViewFields` と `issueViewFields`）。

**候補取得**:

```go
// ListLabels lists the repository's labels (for the picker).
func (c *Client) ListLabels(repo string) ([]Label, error) // gh label list --json name,color --limit 100

// ListAssignees lists users assignable on the repository.
func (c *Client) ListAssignees(repo string) ([]string, error) // gh api repos/{owner}/{repo}/assignees
```

- `ListLabels`: `gh label list --json name,color --limit 100`、`appendRepo` で override。`[]Label` にパース
- `ListAssignees`: `gh api` はプレースホルダ `{owner}` `{repo}` を cwd のリポジトリから補完する。
  `repo` が非空なら明示パス `repos/<repo>/assignees`、空なら `repos/{owner}/{repo}/assignees` を
  `c.run(c.dir, "api", path)` で実行。返る JSON 配列を `[]Author` にパースし `Login` を集めて `[]string` で返す
  （`gh api` は `--repo` を取らないため `appendRepo` は使わない）

**適用**（種別 × 次元の 4 メソッド。内部は共通 private ヘルパ `editItems` に集約して重複を避ける）:

```go
func (c *Client) EditPRLabels(repo string, number int, add, remove []string) error
func (c *Client) EditIssueLabels(repo string, number int, add, remove []string) error
func (c *Client) EditPRAssignees(repo string, number int, add, remove []string) error
func (c *Client) EditIssueAssignees(repo string, number int, add, remove []string) error
```

- 生成コマンド例: `gh pr edit 12 --add-label bug --remove-label wip`（override 時は末尾に `--repo`）
- `add` / `remove` は各値ごとにフラグを繰り返す（`--add-label a --add-label b`）。カンマ結合しないので
  名前中のカンマや特殊文字も安全
- private ヘルパ:
  `editItems(kindCmd, repo string, number int, add, remove []string, addFlag, removeFlag string) error`
  が `[]string{kindCmd, "edit", strconv.Itoa(number)}` に add/remove フラグを積み、`appendRepo` を通して実行する

### `ui`（詳細画面に picking サブ状態＋ピッカーコンポーネント）

Model に追加:

```go
picking       bool     // ピッカー表示中
pickerLoading bool     // 候補取得中
applying      bool     // 差分適用中（スピナー表示・二重適用防止）
picker        picker   // ピッカーコンポーネント（picker.go）
detailLabels    []string // 表示中 item の現在ラベル名（プリチェック用）
detailAssignees []string // 表示中 item の現在アサイン login（プリチェック用）
```

`detailLabels` / `detailAssignees` は `prDetailMsg` / `issueDetailMsg` 受信時に
`msg.Labels` の名前・`msg.Assignees` の login から設定する。

`DataSource` インターフェースに追加（`ui` 側で `target.Kind` により振り分けて呼ぶ）:

```go
ListLabels(repo string) ([]Label, error)
ListAssignees(repo string) ([]string, error)
EditPRLabels(repo string, number int, add, remove []string) error
EditIssueLabels(repo string, number int, add, remove []string) error
EditPRAssignees(repo string, number int, add, remove []string) error
EditIssueAssignees(repo string, number int, add, remove []string) error
```

新規 msg 型:

```go
pickerCandidatesMsg struct {           // 候補取得成功
	kind   pickerKind
	labels []Label  // kind==pickLabels のとき
	users  []string // kind==pickAssignees のとき
}
pickerAppliedMsg struct{}              // 適用成功
pickErrorMsg     struct{ err error }   // 候補取得 or 適用の失敗
```

**適用失敗を全画面 `screenError` に落とさない理由**: Phase 2/3 と同じ。入力（選択）を失わないよう、
`pickErrorMsg` は `picking` を維持したままエラーを表示し再送可能にする。

### ピッカーコンポーネント（`internal/ui/picker.go`）

```go
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
	title    string          // "Labels" / "Assignees"
	items    []pickItem
	original map[string]bool // 初期選択（name -> true）。差分算出の基準
	cursor   int
	offset   int             // スクロール窓の先頭 index
	err      string          // 直近の適用失敗メッセージ
}
```

- `newPicker(kind, candidates, colors, current)`:
  **items は「候補 ∪ 現在値」の和集合**で構築する。現在値が候補一覧に無い場合でも表示・選択済みにし、
  表示漏れによる意図しない除去を防ぐ。`current` に含まれる name を `selected = true`、`original` に登録
- `moveUp` / `moveDown`: `cursor` を境界内で移動し、可視窓（`offset`〜`offset+visible`）を追従させる
- `toggle`: `items[cursor].selected` を反転
- `diff() (add, remove []string)`: 現在の `selected` 集合と `original` を比較。
  `add` = selected かつ original に無い、`remove` = original かつ selected でない
- `listView(height int) string`: タイトル＋可視窓のリスト（`[x]`/`[ ]` ＋ `▸` カーソル ＋ ラベルは色付き名）＋
  `err` 行を返す純描画ヘルパ。スピナーやスピナー依存はここに持ち込まない

描画の責務分離: `picker` は純ロジック＋リスト描画（`listView`）に閉じ、スピナー（`m.spin`）や
`applying` 状態に依存しない。適用中スピナー・フッターの出し分けは Model 側の `pickerView()` が
`m.picker.listView(...)` を包んで行う（`m.applying` なら `m.spin.View()+" applying..."`、
通常は `space:toggle  enter:apply  esc:cancel`）。`pickerView()` も picker.go に置く。

## データフロー（付け替え）

1. 詳細画面で `l`（ラベル）/ `a`（アサイン）→ `pickerLoading = true`、候補取得 cmd を発行。
   `detailLoading` 中は無視
2. 候補取得 cmd（`fetchLabelPicker` / `fetchAssigneePicker`）→ `src.ListLabels` / `src.ListAssignees` を
   `target.Repo` で呼ぶ。成功で `pickerCandidatesMsg{kind, ...}`、失敗で `pickErrorMsg{err}`
3. `pickerCandidatesMsg` → `newPicker` で kind・候補・現在値（`detailLabels` / `detailAssignees`）から
   ピッカーを構築し、`picking = true`・`pickerLoading = false`
4. picking 中のキー処理（`applying` 中は `ctrl+c` 以外を無視）:
   - `j`/`k` → カーソル移動、`space` → トグル
   - `esc` → キャンセル。`picking = false`
   - `enter` → `picker.diff()`。差分が空なら gh を呼ばず `picking = false` で閉じる。
     非空なら `applying = true` にして `applyPicker` cmd を発行
   - `ctrl+c` → 終了
5. `applyPicker(src, target, kind, add, remove)` cmd → `kind` × `target.Kind` で
   `Edit{PR,Issue}{Labels,Assignees}` に振り分け。成功で `pickerAppliedMsg{}`、失敗で `pickErrorMsg{err}`
6. `pickerAppliedMsg` → `picking = false`・`applying = false`・`detailLoading = true` にして
   `fetchDetail` 再取得（新しいラベル／アサインを反映）
7. `pickErrorMsg` → `m.picking` で分岐:
   - picking 中（適用失敗）→ `applying = false`、`picker.err` にメッセージ、ピッカー維持で再送可
   - picking でない（候補取得失敗）→ `pickerLoading = false`、`actionErr` に入れて詳細にインライン表示

`enter`（一覧→詳細）と詳細再ロード時は `picking` / `pickerLoading` / `applying` / `actionErr` を
クリアして状態を持ち越さない（Phase 3 の reset に倣う）。picking 中の描画は `detailView` から
`pickerView` に分岐する。

## キーバインド

| キー | 詳細画面 | picking 中 |
| --- | --- | --- |
| `l` | ラベルピッカーを開く | — |
| `a` | アサインピッカーを開く | — |
| `j` / `k` | スクロール | カーソル移動 |
| `space` | — | 選択トグル |
| `enter` | — | 差分を適用 |
| `esc` | 一覧に戻る | ピッカーをキャンセル |
| `ctrl+c` | 終了 | 終了 |

- 詳細画面フッターに `l:labels  a:assign` を追記
- picking 中フッター: `space:toggle  enter:apply  esc:cancel`（`applying` 中はスピナー + `applying...`）

## エラーハンドリング

| 状況 | 挙動 |
| --- | --- |
| `detailLoading` 中に `l`/`a` | 無視 |
| 候補取得失敗（未認証・権限・リモートなし等） | `pickErrorMsg` で詳細に `actionErr` を表示、ピッカーは開かない |
| 差分空で `enter` | gh を呼ばずピッカーを閉じる |
| `gh edit` 失敗 | `pickErrorMsg` でピッカー内にエラー表示、選択温存、再送可 |
| 適用成功 | 詳細を再取得しラベル／アサイン表示を更新 |

## テスト

- **ghcli**（ホワイトボックス、既存 `fakeRun` を利用）:
  - `TestListLabels`: args `["label","list","--json","name,color","--limit","100"]`、パース
  - `TestListAssignees`: 空 repo で `["api","repos/{owner}/{repo}/assignees"]`、override で
    `["api","repos/octo/hello/assignees"]`、`[]Author` → login 抽出
  - `TestEditPRLabels`: args `["pr","edit","12","--add-label","bug","--remove-label","wip"]`
  - `TestEditIssueAssigneesWithRepoOverride`: 末尾に `--repo`、add/remove フラグ順
  - `TestEditItemsError`: `run` のエラー透過
- **ui**（`fakeSource` に `labels []Label` / `users []string` / `editCalls []string` / 各種 err を追加。
  記録書式は `"pr:labels::12:add=bug,wip:remove=old"` 等）:
  - `l` で候補取得→ラベルピッカー遷移、現在ラベルがプリチェックされている
  - `a` でアサインピッカー遷移
  - `space` で選択トグル、差分空 `enter` は gh 未呼出でピッカーを閉じる
  - 変更ありの `enter` で正しい add/remove を算出して `Edit*` 呼出＋続く `pickerAppliedMsg` で再取得
  - `esc` でキャンセル（`picking=false`）
  - 適用失敗（`pickErrorMsg`・picking 中）でピッカー維持・`picker.err` セット・選択温存
  - 候補取得失敗（`pickErrorMsg`・picking 前）で詳細に留まり `actionErr`、ピッカー未表示
- **picker.go**（純ロジック・描画のユニット）:
  - `newPicker` が候補 ∪ 現在値を作り現在値をプリチェックする（候補外の現在値も表示・選択される）
  - `toggle` / `moveUp` / `moveDown` の境界とスクロール窓追従
  - `diff` が add/remove を正しく算出する
  - `view` が `[x]`/`[ ]` と候補名を含む
- **README**: 詳細画面の `l` / `a` キーを Keys 表に追記

## 依存

新規依存なし。ピッカーは既存の lipgloss（色付け）・spinner で描画でき、bubbles の list 等は使わない
（複数選択が標準にないため自前の小型コンポーネントの方が制御が容易）。charmbracelet 4依存の制約を維持。

## 実機検証で確認する残項目

- 実機（herdr 0.7.x + gh 2.95）で `l` → トグル → `enter` のラベル付け替え、`a` のアサイン変更が動き、
  再取得後に詳細のラベル／アサイン表示が更新されること
- `gh api repos/{owner}/{repo}/assignees` が対象リポジトリのアサイン可能ユーザーを返すこと
  （権限・可視性により空や不足がありうる）
- ラベル多数リポジトリでスクロール窓が破綻しないこと
- `space` / `enter` が herdr/ターミナルのキーグラブに奪われないこと
