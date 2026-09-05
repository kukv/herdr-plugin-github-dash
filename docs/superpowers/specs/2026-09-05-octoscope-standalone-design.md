# octoscope: スタンドアローン GitHub TUI への刷新 設計書

- 日付: 2026-09-05
- 対象リポジトリ: `kukv/herdr-plugin-github-dash` → `kukv/octoscope`

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

3 つのタブを並列に持つ。起動直後は Work タブ。

| タブ | 内容 |
|---|---|
| **Work**（既定） | セクション別リスト: レビュー依頼中 / 自分の PR / assign された Issue / メンション。各行にリポジトリ名と CI 状態バッジを表示 |
| **Repos** | 自分のリポジトリ + 所属 Org のリポジトリの一覧。所有者を切り替えることで他ユーザー・他 Org・starred リポジトリも閲覧できる。選択するとそのリポジトリの PR/Issue リストへ |
| **Search** | 任意の GitHub 検索クエリを入力して結果を表示。設定ファイルに保存クエリを定義できる。Work タブの各セクションもこの仕組みの上に実装する |

詳細ビュー（PR / Issue）からは、diff 閲覧、レビュー提出、checks 一覧、
merge 操作へ遷移する。

## 5. 設定ファイル

`os.UserConfigDir()` 配下の `octoscope/config.toml` を読む。
これにより Windows（`%AppData%`）、macOS（`~/Library/Application Support`）、
Linux（`~/.config`）で自然な場所に収まる。

保持する内容は保存クエリ、既定のタブ、テーマ程度に留める。
設定が無くても全機能が動くことを前提とする。

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
- Work タブ（4 セクション）の実装、タブ切替を持つルートモデル

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

- 所有者切替つきリポジトリブラウザ
- 任意クエリ検索と保存クエリ
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
