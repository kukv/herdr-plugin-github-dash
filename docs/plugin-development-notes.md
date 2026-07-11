# herdr-plugin-github-dash 開発メモ

herdr 上で GitHub の PR / Issue を扱うプラグインを作るための事前調査メモ。
設計セッションはこのドキュメントを起点に進める（調査日: 2026-07-11）。

## ゴール

herdr のペイン内で GitHub の PR / Issue を操作できるようにする。段階的に進める:

1. **Phase 1**: PR / Issue の一覧表示と内容（詳細）表示
2. **Phase 2**: コメント投稿、メタ情報の変更（ラベル・アサインなど）
3. **Phase 3**: レビュー（diff 表示 + `gh pr review`）※急がない

## herdr プラグインシステムの要点

出典: https://herdr.dev/docs/plugins/

- herdr はターミナルワークスペース管理ツール（Rust製ターミナルマルチプレクサ）。
  herdr 側が担うのは「installation, manifest validation, keybindings, terminal panes,
  events, invocation context, and socket access」。
- プラグインの最小構成は **`herdr-plugin.toml` マニフェスト + 実行可能な argv コマンド**。
  言語は自由（Bash / JavaScript / Lua / Rust バイナリなど何でも可）。
- 拡張ポイント（マニフェストで宣言）:
  - **アクション**: ユーザーが呼び出せる処理
  - **イベントフック**: イベント発火時に実行（例: `on = "worktree.created"`。一覧は要確認）
  - **ペイン**: ターミナル領域の表示
  - **キーバインディング**
  - **リンクハンドラー**: URL クリック時の処理（modified-click は全プラットフォームで Control）

### ペイン

- 「Plugin panes are normal Herdr panes after they open」→ **対話的な TUI がそのまま動く**
- `placement` は `overlay`（デフォルト。一時的なズームオーバーレイで、閉じると元のフォーカスに戻る）/
  `split` / `tab` / `zoomed` の4種。`plugin.pane.open` リクエストでマニフェストの指定を上書き可能

### マニフェスト例（ドキュメントより）

```toml
id = "example.id"
name = "Plugin Name"
version = "0.1.0"
min_herdr_version = "0.7.0"

[[actions]]
id = "list-workspaces"
title = "List workspaces"
contexts = ["workspace"]
command = ["node", "index.js"]
```

アクションの必須フィールドは `id` / `title` / `contexts` / `command`。

### プラグインに注入される環境変数

`HERDR_SOCKET_PATH`, `HERDR_BIN_PATH`, `HERDR_ENV=1`, `HERDR_PLUGIN_ID`,
`HERDR_PLUGIN_ROOT`, `HERDR_PLUGIN_CONFIG_DIR`, `HERDR_PLUGIN_STATE_DIR`,
`HERDR_PLUGIN_CONTEXT_JSON`, および利用可能なら `HERDR_WORKSPACE_ID`,
`HERDR_TAB_ID`, `HERDR_PANE_ID`。
状況に応じて `HERDR_PLUGIN_ACTION_ID`, `HERDR_PLUGIN_EVENT`, `HERDR_PLUGIN_CLICKED_URL` など。

→ ワークスペース ID から対象リポジトリ（cwd）を解決して、PR/Issue 一覧の対象を自動で切り替えられるはず。

### API

- herdr CLI 全体がプラグイン API を兼ねる。`HERDR_BIN_PATH` 経由で herdr を呼ぶか、
  Socket API（`HERDR_SOCKET_PATH`）で直接通信する
- 関連ドキュメント:
  - CLI reference: https://herdr.dev/docs/cli-reference/
  - Socket API: https://herdr.dev/docs/socket-api/
  - Agent skill file: https://herdr.dev/docs/agent-skill/
  - Configuration: https://herdr.dev/docs/configuration/

### 配布・マーケットプレイス

- マーケットプレイス（https://herdr.dev/plugins/）は GitHub トピック **`herdr-plugin`**
  が付いた公開リポジトリの自動インデックス
- インストールは `herdr plugin install owner/repo[/subdir]`（GitHub shorthand のみ。
  git clone → プレビュー表示 → ビルドコマンド実行 → 登録、という流れ）
- **注意**: このリポジトリの topics は Terraform 管理（後述）。`herdr-plugin` トピックは
  プラグインが動くようになってから structure 側の PR で追加する（空リポジトリが
  マーケットプレイスに載るのを避けるため、意図的にまだ付けていない）

## 機能と実現手段のマッピング

GitHub 操作は `gh` CLI に乗るのが最短。認証も `gh auth login` 済み環境を前提にでき、
トークン管理をプラグイン側で持たなくてよい。

| 機能 | 実現方法 |
|---|---|
| PR / Issue 一覧・詳細 | `gh pr list` / `gh issue list` / `gh pr view` など（または GraphQL API） |
| コメント投稿 | `gh pr comment` / `gh issue comment` |
| メタ情報変更 | `gh pr edit` / `gh issue edit`（ラベル・アサイン・マイルストーン等） |
| レビュー | `gh pr review` + `gh pr diff` |

## 実装アプローチ候補（未決定 → 設計セッションで決める）

1. **ラップ型（最短）**: 既存 TUI の [gh-dash](https://github.com/dlvhdr/gh-dash) を
   アクションから overlay ペインで起動するだけのマニフェストを書く。
   数十行で一覧+詳細+一部操作が手に入るが、herdr コンテキスト連携（ワークスペース連動、
   リンクハンドラー）は薄くなる
2. **自作 TUI 型（本命）**: Node (Ink) / Go (bubbletea) / Rust (ratatui) などで専用 TUI を書く。
   `HERDR_WORKSPACE_ID` から対象リポジトリを解決し、リンクハンドラーで
   「ターミナル上の PR URL を Ctrl+クリック → その PR の詳細ペインが開く」統合まで狙える

## 参考リポジトリ

- 公式サンプル集: https://github.com/ogulcancelik/herdr-plugin-examples
  （`agent-telegram-notify`, `github-link-preview`, `dev-layout-bootstrap`。
  特に **github-link-preview** はリンクハンドラーの実装例として要参照）
- https://github.com/ogulcancelik/herdr-plugin-github-start
  — GitHub の Issue/PR/Discussion から Codex/Claude を起動するプラグイン。マニフェストの実例として参考になる
- https://github.com/persiyanov/herdr-reviewr
  — エージェントの変更をレビューするサイドバー（GitHub PR ではない）。TUI ペインの実例
- herdr 本体: https://github.com/ogulcancelik/herdr
- 完全な PR/Issue ダッシュボード型のプラグインは調査時点で存在しない（= 作る価値のあるニッチ）

## このリポジトリの運用メモ

- リポジトリ自体は Terraform 管理: `kukv/structure` の
  `terraform/repository_herdr-plugin-github-dash.tf`（作成 PR: kukv/structure#77）。
  **description / topics / 設定変更は structure 側で行う**（手動変更すると drift になる）
- fanout 配布対象（`fanout = {}` = base のみ）。共通ファイルは fanout ワーカーが配布してくる
- main ブランチは保護されている: PR 必須（code owner review + approve 1件）、
  コミット署名必須、タグ保護あり

## 設計セッションで決める・確認すること

- [ ] アプローチの選択: ラップ型 or 自作 TUI 型（自作なら言語・TUI フレームワーク）
- [ ] Phase 1 の UI 設計（一覧⇔詳細の画面遷移、キーバインド、placement の選択）
- [ ] `gh` CLI 依存で行くか GitHub API 直叩きか（レート制限・認証・依存の重さの比較）
- [ ] イベントフックの種類一覧（ドキュメントに明示なし。Socket API / CLI reference で要確認）
- [ ] リンクハンドラーの正確なマニフェスト構文（github-link-preview のソースで確認）
- [ ] ワークスペース → リポジトリ（cwd）解決の具体的な方法（CLI or Socket API のどの呼び出しか）
- [ ] `min_herdr_version` をいくつにするか（現行の herdr リリースを確認）
