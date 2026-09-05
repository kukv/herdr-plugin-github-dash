# Go の書き方

## 整形と lint

`gofumpt` と `goimports` は `golangci-lint fmt` が面倒を見る。手で整えない。
有効な linter は `.golangci.yml` にある（revive / gosec / gocritic / unparam ほか）。

`//nolint` を書くときは、必ず理由をコメントに残す。既存の例:

```go
cmd := exec.Command("gh", args...) //nolint:gosec // G204: args are internally constructed, not attacker-controlled
```

## Go のバージョン

`go.mod` は 1.27.1。新しい標準ライブラリを使ってよい。
`slices` や `maps` があるものを自前で書かない。

## 命名

- パッケージ名は短い小文字の単語 1 つ。`ghclient` ではなく `gh`
- パッケージ名を型名で繰り返さない。`gh.GHClient` ではなく `gh.Client`
- レシーバは 1〜2 文字。同じ型では常に同じ字を使う
- エクスポートするのは、パッケージの外から本当に使うものだけ

## 関数

引数を 4 つ以上並べたくなったら、構造体にまとめる。
既存の `editItems(kindCmd, repo string, number int, add, remove []string, addFlag, removeFlag string)`
は限界を超えている例で、触る機会があれば直す。

戻り値でエラーを返せる場所で `panic` しない。例外は、埋め込みリソースの
読み込み失敗のような**ビルド時のバグ**だけ（`internal/i18n` の `init` がそれ）。

## コメント

コードのコメントは英語で書く。既存コードがそうなっている。

**何をしているかではなく、なぜそうしたかを書く。**
コードを読めば分かることは書かない。

```go
// gh api substitutes {owner}/{repo} from the current directory's repo; for an
// override we build the explicit path (gh api takes no --repo).
```

これは良い例。`gh api` が `--repo` を取らないという外部の事情は、
コードを読んでも分からない。

エクスポートした識別子には doc コメントを付ける。名前から始める。

## context

現状 `context.Context` は使っていない。ネットワーク I/O を並行させる段階
（Phase 1）で導入する。導入するときは:

- 第一引数に `ctx context.Context` を取る
- 構造体のフィールドに持たせない
- `context.Background()` を作るのは `cmd/octoscope/main.go` と
  Bubble Tea の `tea.Cmd` の中だけ

## 外部プロセスの実行

`exec.Command` の引数は、内部で組み立てた値だけを渡す。
ユーザー入力（コメント本文、検索クエリ）はフラグの値として渡し、
シェルを経由させない。`sh -c` を使わない。
