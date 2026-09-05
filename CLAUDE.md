# CLAUDE.md

octoscope は GitHub のプルリクエストと Issue を見渡すためのターミナル UI。
Windows / macOS / Linux で動く単一バイナリ。

## 作業を始める前に

設計と実装計画は次の場所にある。**該当する範囲のものを読んでから手を動かす。**

- 設計: `docs/superpowers/specs/`
- 実装計画: `docs/superpowers/plans/`

設計に書かれているパッケージ構成が、まだコードに存在しないことがある。
その場合は設計が誤りなのではなく、まだそこまで実装が進んでいない。
コードと設計が食い違っていたら、どちらが正しいかを判断する前に理由を確かめる。

## コマンド

```bash
make check        # CI と同じ検査を全部（tidy / lint / fmt / test）
make test         # race 検出とカバレッジつきでテスト
make lint         # golangci-lint
make fmt          # gofumpt + goimports で整形
make release-check # goreleaser の設定と 3 OS のクロスコンパイル
```

`make check` が通らない状態でコミットしない。

## 規約

- @.claude/rules/architecture.md — パッケージ境界、依存の向き、interface の置き場所
- @.claude/rules/go-style.md — Go の書き方
- @.claude/rules/errors.md — エラーの設計
- @.claude/rules/testing.md — テストの書き方
- @.claude/rules/tui.md — Bubble Tea、表示幅、多言語対応

規約に無理があると感じたら、黙って逸脱せず、規約の変更を提案する。
提案の仕方は `.claude/rules/architecture.md` の「規約そのものを変える」にある。

## 動かして確かめる

```bash
go run ./cmd/octoscope                      # カレントディレクトリのリポジトリ
go run ./cmd/octoscope --repo kukv/koto     # 任意のリポジトリ
go run ./cmd/octoscope --lang ja            # 日本語表示
```

TUI の変更は、テストが通っただけで完了とみなさない。実際に起動して見る。
日本語は全角で桁を 2 つ使うので、`--lang ja` でも確認する。
