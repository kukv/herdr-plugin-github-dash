# テスト

## 先にテストを書く

機能もバグ修正も、失敗するテストから始める。
「バリデーションを足す」ではなく「不正な入力で落ちるテストを書いて、それを通す」。

## 外部に触らない

**ネットワークも `gh` プロセスも実際には叩かない。** 例外なし。

`ghcli.Client` は `run` フィールドで外部プロセス実行を差し替えられる。
テストではここにフェイクを入れ、渡された引数を検証する。

```go
c := New("/tmp", "kukv/octoscope")
c.run = func(_ string, args ...string) ([]byte, error) {
    got = args
    return []byte("[]"), nil
}
```

新しく外部と話す型を作るときは、同じように差し替え点を 1 つ用意する。
差し替えのために interface を増やすのではなく、関数型のフィールドで足りるか先に考える。

## テーブル駆動

同じ検証を複数の入力で繰り返すならテーブルにする。
ケースには名前を付け、失敗メッセージにその名前を含める。

```go
cases := []struct {
    name string
    flag string
    want language.Tag
}{
    {"flag wins", "ja", language.Japanese},
}
```

## 何を検証するか

- **`internal/gh`**: 組み立てた `gh` の引数と、JSON のパース結果
- **`internal/tui`**: `Update` に `tea.Msg` を渡した後の状態と、`View()` の出力
- **`internal/i18n`**: 両言語での描画結果、言語の決定順、カタログの ID の一致

`View()` の検証は `strings.Contains` で必要な部分だけを見る。
画面全体を丸ごと比較すると、無関係な変更で壊れて誰も直さなくなる。

## 多言語

UI の文字列を扱うテストは、既定の英語で書く。
**日本語は「桁がずれていないこと」の検証に使う。** 文言そのものは検証しない。

```go
i18n.SetLanguage(language.Japanese)
t.Cleanup(func() { i18n.SetLanguage(language.English) })
```

言語はプロセス全体の状態なので、`t.Cleanup` で必ず戻す。
言語を切り替えるテストは `t.Parallel()` にしない。

## テストのパッケージ

非公開フィールドを読むテストは `package ui`（内部テスト）。
公開 API だけを使うテストは `package i18n_test`（外部テスト）。
既存ファイルに合わせる。混在させるとヘルパーが見えなくなる。

## カバレッジ

基準は 80%（`.octocov.yml`）。下回ると CI が赤くなる。

数字を埋めるためのテストは書かない。カバレッジが落ちたときは
「テストを足す」より先に「そのコードは要るのか」を疑う。

## 実行

```bash
make test                                    # CI と同じ（race + カバレッジ）
go test ./internal/i18n/ -run TestResolve -v # 1 つだけ
```
