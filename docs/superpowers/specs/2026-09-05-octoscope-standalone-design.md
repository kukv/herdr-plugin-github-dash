# octoscope: スタンドアローン GitHub TUI への刷新 設計書

- 日付: 2026-09-05
- 対象リポジトリ: `kukv/herdr-plugin-github-dash` → `kukv/octoscope`
- UI モックアップ: https://claude.ai/code/artifact/96b1dad5-ed75-4176-b110-20923b1b565e

## 1. 背景と目的

現在のこのリポジトリは Herdr プラグイン専用の pane プロセスとして作られている。
起動経路が Herdr に固定されており（`HERDR_PLUGIN_CONTEXT_JSON` からの cwd 解決、
`open.sh` / `run.sh` という bash スクリプト、`herdr-plugin.toml` のアクション定義）、
そのため次の制約を抱えている。

- **Windows で使えない。** プラグインの起動経路が bash スクリプトであり、
  `herdr-plugin.toml` の `platforms` も `["linux", "macos"]` に限定されている。
- **単体で起動できない。** Herdr のワークスペース外からは何もできない。
- **機能が少ない。** 単一リポジトリの PR/Issue 一覧・詳細・コメント・
  ラベル/アサイニー編集・close/reopen のみ。レビュー、diff、CI、マージ、
  リポジトリ横断の閲覧はいずれもできない。

本設計はこれを、**Windows / macOS / Linux で動くスタンドアローンの
GitHub ダッシュボード TUI** として作り直すためのものである。
Herdr との統合は完全に廃止する（Herdr からは通常のコマンドとして起動すればよい）。

## 2. 名前の由来

**octoscope** = **Octo**cat + **-scope**。

- **Octo-**: GitHub のマスコットである Octocat から。このツールが見るものが
  GitHub であることを一目で示す。
- **-scope**: telescope（望遠鏡）、periscope（潜望鏡）、microscope（顕微鏡）と
  同じ接尾辞で、ギリシャ語 *skopein*（見る・観察する）に由来する。
  「道具を通して、肉眼では見えないところまで見渡す」という意味を持つ。

つまり **「GitHub を見渡すための望遠鏡」**。
このツールの中心的な価値は「操作」よりも先に「散らばった自分の仕事を
1 画面で見渡せること」にある、という設計思想をそのまま名前にしている。

命名候補として `ghd`（短いが一般名詞的で検索性が低い）と
`gh-dash`（用途は明快だが著名な同名 OSS `dlvhdr/gh-dash` と衝突する）も
検討したが、固有性と意味の両立を理由に `octoscope` を採用した。

- リポジトリ名: `octoscope`
- Go モジュールパス: `github.com/kukv/octoscope`
- バイナリ名: `octoscope`

## 3. 全体アーキテクチャ

### 3.1 パッケージ構成

```
cmd/octoscope/        エントリポイント、フラグ解析
internal/gh/          GitHub アクセスの抽象（interface とドメイン型）
  ├ cli/              gh CLI バックエンド（既存 internal/ghcli を移設）
  └ api/              go-github + githubv4 バックエンド（Phase 4）
internal/config/      設定ファイルの読み込み
internal/tui/         Bubble Tea モデル群
  ├ app/              ルートモデル、タブ切替、共通キーマップ
  ├ work/             「自分に関係する仕事」ビュー
  ├ repos/            リポジトリブラウザ
  ├ search/           クエリビルダーと結果
  ├ dialog/           ポップアップ（リポジトリ追加、保存クエリ呼び出し）
  ├ detail/           PR/Issue 詳細
  └ diff/             diff ビューア
```

現在の `internal/ui/ui.go` は 728 行に一覧・詳細・コメント入力・ピッカーの
状態がすべて同居している。ここにタブ・diff・checks を足すと確実に破綻するため、
刷新の一環としてビュー単位のサブモデルへ分割する。各サブモデルは
「何を表示するか」「どの操作を受け付けるか」を自身の `Update`/`View` に閉じ込め、
親モデルとはメッセージ型のみで通信する。

### 3.2 バックエンド抽象

`internal/gh` に `Client` interface を定義し、2 つの実装を差し替え可能にする。

| 実装 | 手段 | 認証 |
|---|---|---|
| `cli`（既定） | `gh` コマンドの実行（`gh api graphql` を含む） | `gh auth login` に委ねる |
| `api`（フォールバック） | `go-github` / `githubv4` による HTTP 直叩き | `GH_TOKEN` / `GITHUB_TOKEN` |

起動時に `exec.LookPath("gh")` を試み、見つかれば `cli`、見つからなければ
環境変数のトークンで `api` を構築する。どちらも利用できない場合は、
`gh auth login` の実行かトークンの設定を促すエラー画面を表示する。

既存の `ghcli.Client` は `runFunc` を差し替え可能なフィールドとして持っており、
この抽象化はその延長線上にある。

### 3.3 データ取得

主軸は GraphQL（`gh api graphql` もしくは `githubv4`）。
リポジトリ横断の取得では `search(type: ISSUE, query: ...)` を使い、
`review-requested:@me`、`author:@me`、`assignee:@me`、`mentions:@me` といった
検索クエリで各セクションを構成する。リポジトリごとに `gh pr list` を
繰り返し呼ぶ方式に比べ、リクエスト数が桁違いに少ない。

また `reviewDecision` や `isDraft` といった既存コードが依存しているフィールドは
REST では取得できず GraphQL 専用である。この点でも GraphQL を主軸に据える。

### 3.4 対象リポジトリの決定

Herdr のコンテキストに代わり、次の優先順で決定する。

1. `--repo owner/name` フラグ（明示指定）
2. カレントディレクトリの git remote（`gh` と同じ暗黙の挙動）
3. どちらも無い場合はリポジトリ非依存の Work タブから開始

## 4. 画面構成

3 つのタブを並列に持ち、`1` / `2` / `3` で切り替える。起動直後は Work タブ。
タブごとに役割が違うため、共通のレイアウトを被せず、それぞれに合った形を採る。

### 4.1 Work タブ — カンバン

セクションを**列**として横に並べる。`h` / `l` で列、`j` / `k` で行を移動する。

| 列 | 検索クエリ |
|---|---|
| Review requested | `is:open is:pr review-requested:@me` |
| Your PRs | `is:open is:pr author:@me` |
| Assigned | `is:open assignee:@me` |
| Mentioned | `is:open mentions:@me` |

各カードは 2 行で、1 行目にタイトルと状態アイコン、2 行目にリポジトリ名・
checks の進捗バー・経過時間を置く。画面下部のドロワーに選択中カードの
本文と checks 一覧が出るため、`enter` を押さずに中身を読める。

列の高さがそのまま滞留量になるのがこのレイアウトの要点であり、
「レビューが溜まっている」「CI が落ちたまま放置されている」が形として見える。

### 4.2 Repos タブ — サイドバー + サブタブ

左ペインにリポジトリ一覧、右ペインに選択中リポジトリの中身を置く 2 ペイン構成。

- サイドバーの各行には PR 件数 / Issue 件数のバッジを出し、開く前に状況が分かるようにする
- 右ペインは `tab` で **Pull Requests** / **Issues** のサブタブを切り替える。両者を混ぜない
- 右ペインのリストは桁を揃えた表として描く（状態・番号・タイトル・checks・経過時間）
- リストの下に選択中アイテムの要約（ブランチ、変更量、checks）を出す

リポジトリ一覧は「所有者を切り替える」のではなく、**利用者が育てる一覧**とする。
サイドバー末尾の「＋ リポジトリを追加」ボタン、またはどこからでも `a` で
追加ダイアログをポップアップし、`owner/name` を入力して一覧に足す。
入力中は GitHub 検索で候補を引いて提示する。削除は `x`。
一覧は設定ファイルに永続化する。

初回起動時は一覧が空になるため、自分のリポジトリと所属 Org のリポジトリを
初期投入する導線を用意する。

### 4.3 Search タブ — クエリビルダー

左ペインでフィルタ項目（type / state / org / repo / author / label / review / sort）を
組み立て、右ペインに結果を出す。組み立て結果の生クエリは上段に常時表示し、
`e` で直接編集もできる。label や author は候補をチップとして提示する。

GitHub の検索構文を覚えていなくても絞り込めることがこの形の目的であり、
構文を網羅することは目的としない。網羅が必要な場合は生クエリの直接編集に落とす。

組み立てたクエリは `s` で保存し、保存済みクエリは `Ctrl+O` のポップアップから
呼び出す。Repos の追加ダイアログと同じ「ポップアップで選ぶ」操作に揃える。

### 4.4 詳細ビュー

PR / Issue の詳細からは、diff 閲覧、レビュー提出、checks 一覧、merge 操作へ遷移する。

### 4.5 装飾

装飾の強度は**最大限**とする。具体的には次を含む。

- 行内の checks 進捗バー（`▰▰▰▰▰▱▱`）
- 変更量の推移スパークライン（`▁▃▅▇▅▂`）
- GitHub ラベルの実色を再現した塗りつぶしバッジ
- 状態アイコン（成功 / 失敗 / 実行中 / draft / approved）

アイコンは Nerd Font が利用できる環境ではグリフを使い、
利用できない場合は ASCII 代替へ自動的にフォールバックする。

### 4.6 幅への対応

端末幅が足りない場合は、次の順に段階的に劣化させる。

1. サイドバーを畳む（Repos / Search）
2. ドロワー・詳細ペインを畳む（Work / Repos）
3. Work タブのカンバンを 1 列ずつのページングに切り替える

## 5. 設定ファイル

`os.UserConfigDir()` 配下の `octoscope/config.toml` を読む。
これにより Windows（`%AppData%`）、macOS（`~/Library/Application Support`）、
Linux（`~/.config`）で自然な場所に収まる。

保持する内容は次に留める。設定が無くても全機能が動くことを前提とする。

| 項目 | 内容 |
|---|---|
| `repositories` | Repos タブに表示する `owner/name` の一覧 |
| `saved_queries` | Search タブで保存したクエリ（名前とクエリ文字列） |
| `default_tab` | 起動時に開くタブ |
| `nerd_font` | Nerd Font グリフの使用可否の明示指定（既定は自動判定） |

## 6. フェーズ分割

各フェーズが単独で「動いて使える」状態になるように切る。
フェーズごとに spec → 実装計画 → 実装のサイクルを回す。

### Phase 0: スタンドアローン化とリネーム

- GitHub 上でリポジトリ名を `octoscope` に変更（旧 URL はリダイレクトされる）
- `structure` リポジトリ側の Terraform を追従させる（別 PR、詳細は §8）
- `go.mod` を `github.com/kukv/octoscope` に変更し、全 import を更新
- Herdr 資産の削除: `internal/herdrctx`、`herdr-plugin.toml`、`open.sh`、`run.sh`、
  および `GITHUB_DASH_URL` によるリンクハンドラ経路
- `main.go` を `cmd/octoscope/main.go` へ移動
- `--repo` フラグと cwd の git remote による対象リポジトリ解決を実装
- GoReleaser 設定とリリースワークフローを追加（GitHub Actions は
  full-length commit SHA でピンする org ポリシーに従う）
- README を全面的に書き換える

**検証**: `GOOS=windows/darwin/linux` のクロスコンパイルが通ること。
`octoscope --repo kukv/octoscope` で既存の PR/Issue ビューが従来どおり動くこと。

### Phase 1: Work タブと GraphQL バックエンド

- `internal/gh` の interface 化、`ghcli` の移設
- GraphQL search によるリポジトリ横断取得
- Work タブのカンバン（4 列 + ドロワー）の実装、タブ切替を持つルートモデル

**検証**: 複数リポジトリにまたがる自分関連の PR/Issue が 1 画面に表示される。

### Phase 2: PR レビュー

- diff ビューア（シンタックスハイライト、ファイル間移動、ハンク単位の移動）
- レビュー提出（approve / request changes / comment）
- 行コメントの追加

**検証**: 実際の PR に対して TUI からレビューを提出できる。

### Phase 3: checks / merge

- checks 一覧、失敗ジョブのログ閲覧、ワークフローの再実行
- merge（squash / merge / rebase）、auto-merge の有効化

**検証**: 実際の PR を TUI からマージできる。

### Phase 4: Repos タブ / Search タブ / API フォールバック

- Repos タブ（サイドバー + サブタブ + 追加ダイアログ）
- Search タブ（クエリビルダー、保存クエリのポップアップ）
- `internal/gh/api` の実装（go-github + githubv4 + トークン管理）

**検証**: `gh` を PATH から外した状態で、`GH_TOKEN` のみで全機能が動作する。

API フォールバックを最後に置いたのは、バックエンドを 2 系統とも最初から
並走させると Phase 0〜3 のすべてで二重実装の負担が生じるためである。
Phase 0〜3 は `gh` を前提に進め、interface の境界だけを先に確定させておく。

## 7. テスト方針

既存の `runFunc` 差し替えによるテストパターンを踏襲する。

- `internal/gh` はテスト用の fake 実装を持ち、上位層はそれに対してテストする
- TUI は `tea.Model` の `Update` / `View` を golden test で検証する
  （既存の `internal/ui/ui_test.go` と同じ形）
- ネットワークを実際に叩くテストは書かない

## 8. リネーム作業の注意点

`structure` リポジトリの `terraform/repository_herdr-plugin-github-dash.tf` は
モジュールラベル `repository_herdr_plugin_github_dash` でリソースを管理している。
これをリネームする際、`moved {}` ブロックを伴わずにラベルだけ変更すると
Terraform はリポジトリの destroy / recreate を計画してしまう。

手順:

1. GitHub 上でリポジトリ名を先に変更する（旧 URL は自動でリダイレクトされる）
2. `structure` 側でファイル名とモジュールラベルを変更し、`moved {}` を追加する
3. `terraform plan` の結果が `~ update in-place` であること（`-/+` でないこと）を
   確認してから適用する

この作業は `structure` リポジトリ側の独立した PR として行う。

## 9. 廃止するもの

| 対象 | 理由 |
|---|---|
| `internal/herdrctx` | Herdr のコンテキスト解決が不要になる |
| `herdr-plugin.toml` | プラグイン定義が不要になる |
| `open.sh` / `run.sh` | bash 依存の起動経路が不要になる（Windows 対応の妨げでもある） |
| `GITHUB_DASH_URL` 経路 | Herdr のリンクハンドラ専用の入口 |
