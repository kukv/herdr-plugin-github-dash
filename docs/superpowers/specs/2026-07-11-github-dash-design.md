# GitHub Dash — Phase 1 設計

herdr のペイン内で GitHub の PR / Issue を扱うプラグインの Phase 1 設計。
事前調査は [docs/plugin-development-notes.md](../../plugin-development-notes.md) を参照（設計セッション: 2026-07-11）。

## 決定事項

| 論点 | 決定 |
|---|---|
| 対象リポジトリ | ワークスペース連動の単一リポジトリ（横断ビューは対象外） |
| アプローチ | 自作 TUI（Go + bubbletea）+ `gh` CLI サブプロセス |
| placement | `overlay`（デフォルト。`plugin.pane.open` で上書き可能） |
| min_herdr_version | `0.7.0`（最新リリース v0.7.3、公式サンプルに合わせる） |
| リンクハンドラー | Phase 1 に含める（実装が小さく、自作 TUI 型を選んだ主目的の1つのため） |

`gh` CLI を選んだ理由: 認証・レート制限・API 追随をすべて `gh` に委譲でき、
Phase 2（`gh pr comment` / `gh pr edit`）も Phase 3（`gh pr review`）も同じパターンの
延長で実装できる。サブプロセス起動のオーバーヘッドは一覧+詳細の取得頻度では問題にならない。

## スコープ

**含む（Phase 1）**

- ワークスペース連動リポジトリの PR 一覧 / Issue 一覧（タブ切替。対象は open のもののみ＝`gh` のデフォルト）
- PR / Issue の詳細表示（本文 Markdown レンダリング + コメント）
- リンクハンドラー: ターミナル上の GitHub PR/Issue URL を Ctrl+クリック → 詳細ペインで開く
- `o` キーでブラウザ表示（`gh <pr|issue> view --web`）

**含まない**

- コメント投稿・ラベル/アサイン等の変更（Phase 2）
- レビュー・diff 表示（Phase 3）
- 複数リポジトリ横断ビュー、リポジトリ手動切り替え
- `herdr-plugin` トピックの付与（動くようになってから kukv/structure 側の PR で行う）

## アーキテクチャ

```
herdr-plugin-github-dash/
├── herdr-plugin.toml      # マニフェスト
├── open.sh                # アクション → pane open のシム（公式サンプルと同パターン）
├── go.mod / main.go
└── internal/
    ├── herdrctx/          # 起動コンテキスト解決（workspace → リポジトリの cwd）
    ├── ghcli/             # gh CLI ラッパー（一覧・詳細の取得、--json パース）
    └── ui/                # bubbletea モデル（一覧・詳細・エラー画面）
```

依存方向: `ui` → `herdrctx` / `ghcli`。逆方向の依存はなし。

- `herdrctx`: 「どのディレクトリを対象にするか」だけを返す
- `ghcli`: 「指定ディレクトリで gh を実行し構造化データを返す」だけ
- `ui`: 画面状態と遷移。データ取得は `tea.Cmd` として非同期実行

**重要な制約**: ペインのプロセスはプラグインディレクトリを cwd として起動される
（"Runtime commands run with the plugin directory as their working directory"）。
対象リポジトリは必ず明示的に解決し、`exec.Cmd.Dir` に設定して gh を実行する。

## マニフェスト

```toml
id = "kukv.github-dash"
name = "GitHub Dash"
version = "0.1.0"
min_herdr_version = "0.7.0"
description = "Browse GitHub pull requests and issues for the current workspace."
platforms = ["linux", "macos"]

[[build]]
command = ["go", "build", "-o", "bin/github-dash", "."]

[[actions]]
id = "open"
title = "Open GitHub dashboard"
contexts = ["workspace"]
command = ["bash", "open.sh"]

[[panes]]
id = "dash"
title = "GitHub Dash"
placement = "overlay"
command = ["bin/github-dash"]

[[link_handlers]]
id = "github-pr-issue"
title = "Open in GitHub Dash"
pattern = "^https://github\\.com/[^/]+/[^/]+/(issues|pull)/[0-9]+/?$"
action = "open"
```

- `open.sh` は `herdr plugin pane open --plugin kukv.github-dash --entrypoint dash` を呼ぶシム。
  リンクハンドラー経由（`HERDR_PLUGIN_CLICKED_URL` あり）の場合は
  `--env GITHUB_DASH_URL=<url>` を付けて詳細直行モードで開く
- 利用者側の要件: Go ツールチェーン（インストール時ビルド用）+ `gh` 認証済み

## UI

2画面構成。

```
┌ GitHub Dash ─ owner/repo ────────────────────────────┐
│ [Pull Requests] Issues                               │
│ ▸ #12  feat: add pane view      @kukv   ✓ 2h ago     │
│   #10  fix: manifest parsing    @bob    × 1d ago     │
│ j/k:移動 enter:詳細 tab:PR/Issue r:更新 o:ブラウザ q:閉│
└──────────────────────────────────────────────────────┘
        │ enter                    ▲ esc / q
        ▼                          │
┌ PR #12 feat: add pane view ──────────────────────────┐
│ メタ情報（作者・状態・ラベル・レビュー状況）           │
│ 本文（glamour で Markdown レンダリング）+ コメント     │
│ j/k:スクロール o:ブラウザ esc:戻る                     │
└──────────────────────────────────────────────────────┘
```

キーバインド:

| キー | 一覧 | 詳細 |
|---|---|---|
| `j` / `k` | カーソル移動 | スクロール |
| `enter` | 詳細を開く | — |
| `tab` | PR ⇔ Issue 切替 | — |
| `r` | 再取得 | 再取得 |
| `o` | ブラウザで開く | ブラウザで開く |
| `esc` | — | 一覧に戻る |
| `q` | 終了（オーバーレイを閉じる） | 一覧に戻る |

ライブラリ: bubbletea（TUI）、bubbles（リスト・スピナー）、lipgloss（スタイル）、
glamour（Markdown レンダリング）。すべて charmbracelet 製。

## データフロー

1. ペイン起動 → `herdrctx` が対象ディレクトリを解決:
   1. `HERDR_PLUGIN_CONTEXT_JSON` のトップレベルキー `workspace_cwd`
   2. フォールバック: 同 JSON の `focused_pane_cwd`
   3. どちらも失敗 → エラー画面
   （`herdr workspace get` は cwd を含まないことを実機検証済みのため、フォールバックには使わない）
2. `GITHUB_DASH_URL` が設定されていれば（リンクハンドラー経由）、起動直後にその PR/Issue の詳細画面へ直行。
   URL のリポジトリがワークスペースのリポジトリと異なる場合があるため、直行モードの取得は
   常に URL から抽出した `owner/repo` を `gh --repo` で明示する
3. 一覧: 対象ディレクトリを `exec.Cmd.Dir` にして
   `gh pr list --json number,title,author,state,isDraft,updatedAt,reviewDecision` /
   `gh issue list --json number,title,author,state,updatedAt,labels` を実行。
   リポジトリ解決は gh が git remote から行うため自前実装しない
4. 詳細: `gh pr view <n> --json ...` / `gh issue view <n> --json ...`（本文・コメント含む）
5. 取得はすべて `tea.Cmd` として非同期実行し、取得中はスピナー表示

## エラーハンドリング

| 状況 | 挙動 |
|---|---|
| `gh` が見つからない / 未認証 | 導入手順（`gh auth login`）を示すエラー画面 |
| 対象が git リポジトリでない / GitHub リモートなし | gh の stderr をそのまま表示 |
| ワークスペース解決失敗 | フォールバック後も失敗なら状況を表示するエラー画面 |
| 一覧が空 | 「No open pull requests」等の空状態表示 |

## テスト

- `ghcli`: gh の `--json` 出力フィクスチャに対するパースのユニットテスト
- `herdrctx`: コンテキスト JSON パースとフォールバック順序のユニットテスト
- `ui`: `Update` へのキー入力→状態遷移のユニットテスト（一覧⇔詳細、タブ切替、終了）
- E2E: `herdr plugin link <path>` でローカルリンクし、実機の herdr で手動確認

## 実機検証の結果（2026-07-12、herdr 0.7.1）

デバッグプラグイン（env ダンプ）を `herdr plugin link` で実機に載せて確認した。

- `HERDR_PLUGIN_CONTEXT_JSON` はフラットな JSON。実サンプル:
  ```json
  {"workspace_id":"w4","workspace_label":"herdr-plugin-github-dash",
   "workspace_cwd":"/home/tech/dev/ghq/github.com/kukv/herdr-plugin-github-dash",
   "tab_id":"w4:t1","tab_label":"1","focused_pane_id":"w4:p2",
   "focused_pane_cwd":"/home/tech/dev/ghq/github.com/kukv/herdr-plugin-github-dash",
   "focused_pane_status":"unknown","invocation_source":"cli","correlation_id":"cli:plugin"}
  ```
- アクション経由で `herdr plugin pane open` した**ペインプロセスにも同じコンテキスト JSON が
  ネイティブに渡る**（`workspace_cwd` 入り、`invocation_source` は `api` になる）。
  open.sh からの転送は不要
- ペインプロセスの cwd はプラグインルート（ドキュメント記載どおり）。
  `HERDR_PLUGIN_ENTRYPOINT_ID` も設定される
- `herdr workspace get` / `workspace list` の応答に cwd は**含まれない**。
  cwd を持つのはペインレコード（`herdr pane get` の `cwd` / `foreground_cwd`）のみ
- `gh pr list` / `gh issue list` / `gh pr view` の `--json` フィールド名と出力形状も実物で確認済み
  （author は `{login, is_bot}`、labels は `{id,name,description,color}` など）

**E2E テストで確認する残項目**: リンクハンドラーの実クリック経由で発火した際の
コンテキスト内容（`contexts = ["workspace"]` の適用可否を含む）。クリック操作が必要なため
実装後の手動確認とする。
