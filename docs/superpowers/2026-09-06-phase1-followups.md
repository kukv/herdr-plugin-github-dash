# Phase 1 の積み残し

Phase 1（PR #46 / #47 / #49 / #51、いずれも main にマージ済み）で
**満たせていない spec 要件**と、意図的に後回しにした宿題をまとめる。
次のセッションはこのファイルから始めてよい。

spec は `docs/superpowers/specs/2026-09-05-octoscope-standalone-design.md`。
実装計画は `docs/superpowers/plans/2026-09-06-octoscope-phase1.md`。

## なぜこれが起きたか

この環境には TTY が無く、Phase 1 の実装中に**一度も画面を見ないまま 4 PR を
マージした**。テストは 246 本すべて緑、カバレッジも 89% あったが、
**色・グリフ・カードの行数を検証しているテストが 1 つも無かった**ため、
spec §4.5 の「装飾の強度は最大限」が丸ごと未達のまま通ってしまった。

TTY が無くても、テストの中で `View()` を `fmt.Println` すれば見える。
以下の 1 番を最初にやること。

---

## 1. 描画ハーネス（最優先）

各ビューの `View()` を **en / ja × 80 / 120 / 160 桁**で出力する golden test を作る。

- 置き場所: `internal/tui/<view>/golden_test.go` と `testdata/*.golden`
- 目的は 2 つ。桁ずれの検出と、**人間が差分を目で見られること**
- ANSI エスケープを落とさずに保存する。色が消えた／付いた変更が diff に出ないと意味がない
- `go test ./... -update` で golden を更新できるようにする

**これが入るまで、TUI の変更を「完了」と言わない。**

参考: 現状を手で描画したときの出力（120 桁）。装飾は bold と faint だけで、
色は 1 箇所も使われていない。

```
  Review requested              Your PRs                      Assigned                      Mentioned
────────────────────────────  ────────────────────────────  ────────────────────────────  ────────────────────────────
▸ • chore(deps): update act…    × refactor: split the UI …    #7 Nerd Font のフォールバ…    (nothing here)
  kukv/ktor-typed-routing       kukv/octoscope                kukv/octoscope
  ▰▰▰▰▰▱▱ 3h ago                ▰▰▰▰▱▱▱ 3h ago                3h ago
```

## 2. 色（spec §4.5 違反）

`lipgloss` を依存に入れておきながら `AdaptiveColor` の使用箇所がゼロ。

- `internal/tui/theme` を作り、状態色を 1 箇所に集める
  （approved / changes requested / review required / draft / check success /
  check failure / check running）
- ターミナルの背景色は前提にできないので `AdaptiveColor` を使う
  （`.claude/rules/tui.md`）
- 現状のスタイルは `internal/tui/work/render.go` の
  `headingStyle` / `dimStyle` / `cursorStyle` の 3 つだけ。ここを置き換える

## 3. Nerd Font グリフと ASCII フォールバック（spec §4.5 違反）

spec は「Nerd Font が利用できる環境ではグリフを使い、利用できない場合は
ASCII 代替へ自動的にフォールバックする」と書いている。

`internal/tui/icon/icon.go` の冒頭に
「Phase 1 は Unicode 記号のみ、Nerd Font は後」というコメントを入れて
**実装側が勝手にスコープを外した**。spec に Phase 送りの根拠は無い。

- 検出方法を決める（環境変数 / 設定ファイル `§5` / `--icons` フラグ）
- グリフ表と ASCII 表を並べ、`icon` パッケージが選ぶ
- 決めた検出方法を spec §4.5 に追記する

## 4. カードを 2 行にする（spec §4.1 違反）

spec: 「各カードは 2 行で、1 行目にタイトルと状態アイコン、2 行目に
リポジトリ名・checks の進捗バー・経過時間を置く」

実装は 3 行（タイトル / リポジトリ名 / バー + 経過時間）。
`internal/tui/work/render.go` の `cardLines`。

## 5. ラベルの塗りつぶしバッジ（spec §4.5 違反）

Work カードにラベルが出ていない。`ListWork` の GraphQL
（`internal/gh/cli/work.graphql`）がラベルを取っていないので、
クエリと `searchNode` の両方に足す必要がある。

GitHub のラベル色は 16 進の実色で返るので、それを背景色として再現する。
前景色は背景の輝度から白／黒を選ぶ。

**注意:** `work.graphql` にフィールドを足したら
`searchNode` にも対応する JSON タグを足すこと。
`TestTheQueryAsksForEveryFieldWeParse` が両方向を縛っている。

## 6. ドロワーに本文と checks 一覧（spec §4.1 違反）

spec: 「画面下部のドロワーに選択中カードの**本文**と checks **一覧**が出るため、
`enter` を押さずに中身を読める」

実装は本文なし、checks も一覧ではなく要約 1 行
（`internal/tui/work/render.go` の `drawer`）。

GraphQL に `bodyText` と各 context の `name` を足す必要がある。

## 7. マウス操作（spec に記述なし）

spec にマウスの記述が無いため実装していないが、TUI として当然期待される。
`tea.WithMouseCellMotion` すら渡していないので、ホイールスクロールもできない。

- カードのクリックで選択、ダブルクリックで詳細
- ホイールで列内スクロール、ドロワー内スクロール
- タブのクリックで切り替え
- **spec §4 にマウス操作を追記して合意を残す**こと

## 8. 実端末での目視確認

上記が入ったら、実際に起動して見る。`.claude/rules/tui.md` の「確認」。

```bash
go run ./cmd/octoscope
go run ./cmd/octoscope --lang ja   # 全角で桁がずれていないか
```

80 桁でも崩れないこと、100 桁未満でドロワーが消えること、
60 桁未満で 1 列ページングになることを確認する。

---

## その他の宿題（spec 違反ではない）

- **`hasRepo` が起動時に最大 5 秒ブロックする** — `cmd/octoscope/main.go`。
  `gh` に git remote を解決させる同期呼び出しを UI 起動前に行っている。
  Work タブから開始して非同期に埋める形にできる
- **`gh.PR.State` / `gh.PR.ReviewDecision` が GitHub の生文字列のまま** —
  `internal/gh/gh.go`。`.claude/rules/architecture.md` の
  「GitHub API 固有の値はパッケージの外に出さない」に反する。
  `WorkItem` 側は `ReviewState` / `CheckState` に変換済みなので、
  `PR` / `Issue` も同じ形に揃える（Phase 2 送りにしていた）
- **古い `prMsg` / `issueMsg` のクロストーク** — `internal/tui/detail/`。
  `errMsg` には `gh.ItemRef` を持たせて古いものを捨てるようにしたが、
  成功側の 2 つは同じ手当てをしていない。詳細を素早く開き直すと、
  前のアイテムの内容が一瞬出る可能性がある
