# CLAUDE.md

octoscope は GitHub のプルリクエストと Issue を見渡すためのターミナル UI。
Windows / macOS / Linux で動く単一バイナリ。

## このリポジトリの現在地

Herdr プラグインからスタンドアローン CLI への刷新の途中。

- 設計: `docs/superpowers/specs/`
- 実装計画: `docs/superpowers/plans/`

**作業を始める前に、該当フェーズの spec と plan を読むこと。**
まだ実装されていない構成（`internal/gh/`、`internal/tui/` など）が
spec には書かれている。現状のコードと食い違っていても spec が誤りとは限らない。

## コマンド

```bash
make check        # CI と同じ検査を全部（tidy / lint / fmt / test）
make test         # race 検出とカバレッジつきでテスト
make lint         # golangci-lint
make fmt          # gofumpt + goimports で整形
```

`make check` が通らない状態でコミットしない。

## 規約

- @.claude/rules/architecture.md — パッケージ境界、依存の向き、interface の置き場所
- @.claude/rules/go-style.md — Go の書き方
- @.claude/rules/errors.md — エラーの設計
- @.claude/rules/testing.md — テストの書き方
- @.claude/rules/tui.md — Bubble Tea、表示幅、多言語対応

## 動かして確かめる

```bash
go run ./cmd/octoscope                      # カレントディレクトリのリポジトリ
go run ./cmd/octoscope --repo kukv/koto     # 任意のリポジトリ
go run ./cmd/octoscope --lang ja            # 日本語表示
```

TUI の変更は、テストが通っただけで完了とみなさない。実際に起動して見る。
