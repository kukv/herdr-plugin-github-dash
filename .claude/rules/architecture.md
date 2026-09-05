# アーキテクチャ

## 依存の向き

```
cmd/octoscope
     ↓
internal/tui  ──→  internal/gh    （GitHub へのアクセス）
     ↓        ──→  internal/config （設定ファイル）
internal/i18n                      （誰にも依存しない）
```

**下の層は上の層を知らない。** `internal/gh` が `internal/tui` を import したら
設計が壊れている。`internal/i18n` は他の internal パッケージを import しない。

この向きは目視ではなく lint で守る。`.golangci.yml` の `depguard` に
禁止 import を書き、CI で落とす。ルールを増やしたら、そこにも足す。

## interface は利用側で定義する

**`internal/gh` は interface を export しない。** 具体型（`*Client`）と
ドメイン型（`PR`、`Issue`、`Label`）だけを公開する。

必要な操作の interface は、**それを使う側**が宣言する。
現在の `ui.DataSource` がその形になっている。

```go
// internal/tui/detail/detail.go
type source interface {
    GetPR(repo string, number int) (gh.PR, error)
    AddPRComment(repo string, number int, body string) error
}
```

こうすると、そのビューが何を必要としているかがビューのファイルを読むだけで分かり、
テスト用のフェイクも必要なメソッドだけ書けば済む。

## interface は小さく保つ

**画面ごとに、その画面が使う分だけ宣言する。**
全メソッドを 1 つの巨大な interface にまとめない。

現在の `ui.DataSource` は 19 メソッドあり、これは 1 つのモデルに全画面が
同居している結果である。ビューを分割するときに、この interface も
ビュー単位に割る。テスト用フェイクが 19 メソッド分のスタブを要求するようなら、
それは interface が大きすぎる合図。

## gh 固有の値はパッケージの外に出さない

`"APPROVED"`、`"CHANGES_REQUESTED"`、`"OPEN"` のような GitHub API の
文字列を TUI 側で `switch` しない。`internal/gh` でドメインの値に変換して返す。

```go
// internal/gh
type ReviewState int

const (
    ReviewPending ReviewState = iota
    ReviewApproved
    ReviewChangesRequested
)
```

理由は 2 つ。GraphQL と REST で綴りが違う場合にバックエンドの差が UI に漏れないこと、
そして UI 側が「知らない文字列」を握りつぶす分岐を持たずに済むこと。

## パッケージを増やす基準

責務が 1 つに言えるなら 1 パッケージ。「〜と〜をやる」と説明したくなったら分ける。

ファイルが 300 行を超えたら、責務が増えていないか疑う。
既存の `internal/ui/ui.go` は 728 行あり、これは分割対象として
spec に明記されている（Phase 1）。同じことを新しいパッケージで繰り返さない。

## 今はやらないこと

このプロジェクトは TUI の単一バイナリであり、Web サービスではない。
次のものは**入れない**。

- UseCase 層 / Service 層。ビューがリポジトリ層を直接呼んで構わない
- DI コンテナ。依存は `cmd/octoscope/main.go` で手で組み立てる
- ドメインモデルとインフラモデルの二重定義。`gh.PR` をそのまま画面に渡す

層を足したくなったら、まず「この層が無いと何が壊れるか」を書けるか確かめる。
書けないなら足さない。
