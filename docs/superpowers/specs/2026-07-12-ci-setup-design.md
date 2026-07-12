# CI 導入 設計

作成日: 2026-07-12

## ゴール

Pull Request 上で fmt/lint・test・カバレッジを自動チェックする CI を導入する。
public リポジトリのため lint と test は別ジョブに分け、並列で実行する。依存
（Go モジュール）キャッシュは設定しない。

**スコープ外**: リリース（タグ/GitHub Release）の自動化、タグと
`herdr-plugin.toml` の version 一致ガード。herdr はプラグインを default ブランチ
HEAD から git clone してインストールし（`--ref` で任意 revision に pin 可能、専用の
update 機構は無し）、`version` フィールドはメタデータのみで install/update の解決に
使われないため、タグ運用は必須ではない。今回は手動リリースで足りる。

## 成功条件

- PR を作成/更新すると `lint` と `test` の2ジョブが並列で走る。
- フォーマット崩れ・lint 違反があれば `lint` ジョブが fail する。
- テスト失敗、またはカバレッジが閾値（80%）を下回れば `test` ジョブが fail する。
- カバレッジが PR にコメントで報告される。

## 作成するファイル

### 1. `.github/workflows/ci.yaml`

- トリガー: `on: pull_request`（main への直接 push では走らせない）
- `permissions`: `contents: read` / `pull-requests: write`（octocov の PR コメント用）
- ジョブ（並列・独立）:
  - **lint**
    - `actions/checkout`
    - `actions/setup-go`（`go-version-file: go.mod`, `cache: false`）
    - `golangci/golangci-lint-action@v7`（golangci-lint v2 対応）で `run`
    - `golangci-lint fmt --diff` 相当でフォーマット崩れを検出し fail させる
  - **test**
    - `actions/checkout`
    - `actions/setup-go`（`go-version-file: go.mod`, `cache: false`）
    - gotestsum をCI内で導入（`go install gotest.tools/gotestsum@<pinned>`）
    - `gotestsum -- -coverprofile=coverage.out -covermode=atomic ./...`
    - `k1LoW/octocov-action` でカバレッジのPRコメント報告 + 閾値判定

### 2. `.golangci.yml`（golangci-lint v2, しっかりめ）

- `version: "2"`
- デフォルト有効（errcheck / govet / ineffassign / staticcheck / unused）に加え、
  revive・gosec・misspell・gocritic・bodyclose 等を追加。
- `formatters:` に gofmt + goimports。
- 誤検知が出た linter は実装フェーズで無効化/調整する。

### 3. `.octocov.yml`

- `coverage.paths: [coverage.out]`
- `comment.if: is_pull_request`（PR にカバレッジをコメント報告）
- `coverage.acceptable: 80%`（現状の合計カバレッジ 82.4% に対する floor。
  下回ると octocov が非ゼロ終了し `test` ジョブが fail）

## 現状カバレッジ（閾値の根拠）

- 合計: 82.4%
- root 47.4% / internal/ghcli 68.3% / internal/herdrctx 100% / internal/ui 86.0%

## 検証方針

- `.golangci.yml` はローカルの golangci-lint v2.12.2 で `golangci-lint run` /
  `golangci-lint fmt --diff` を実行して設定が有効かつ通ることを確認する。
- gotestsum のコマンド列はローカルで実行し、`coverage.out` が生成されることを確認する。
