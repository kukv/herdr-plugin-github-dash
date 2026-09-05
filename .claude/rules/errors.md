# エラーの設計

## 3 種類を分けて扱う

| 種類 | 例 | 扱い |
|---|---|---|
| 環境の不備 | `gh` が無い、認証していない | 対処方法を添えて画面に出す |
| GitHub 側の失敗 | 404、403、レート制限 | メッセージをそのまま見せる |
| プログラムのバグ | カタログに無いメッセージ ID | 握りつぶさず目に見える形にする |

環境の不備だけがユーザーに行動を促す。ここには翻訳した案内文を付ける。
GitHub 側の失敗は原文が最も情報量が多いので、翻訳せずそのまま出す。

## センチネルエラー

利用側が種類で分岐する必要があるものだけ、パッケージ変数にする。

```go
// ErrGhNotFound is returned when the gh binary is not on PATH.
var ErrGhNotFound = errors.New("gh CLI not found; install it and run: gh auth login")
```

判定は `errors.Is` を使う。文字列比較しない。

分岐に使わないエラーをセンチネルにしない。増やす前に
「これを `errors.Is` で判定する箇所があるか」を確認する。

## ラップ

呼び出し元が原因を追えるよう、文脈を足して `%w` でラップする。

```go
return nil, fmt.Errorf("parse pr list: %w", err)
```

- 文脈は小文字で始め、末尾に句点を付けない
- `failed to` / `error while` を付けない。エラーであることは自明
- 同じ文脈を二重に足さない。`gh pr: gh pr: ...` になっていたらどこかで重複している

## ユーザーに見せる文字列

画面に出すメッセージは `internal/i18n` のカタログから引く。
エラーの技術的な内容（gh の stderr、HTTP ステータス）は翻訳せず、
翻訳した前置きに続けて出す。

```go
i18n.T("common.error_prefix") + err.Error()
```

## エラーを無視してよい場面

無視するときは `_` に入れて、なぜ無視してよいか書く。

```go
osLocale, _ := locale.GetLocale() // an error here just means "unknown"
```

理由が書けないなら、それは無視してはいけないエラー。

## Bubble Tea の中でのエラー

`tea.Cmd` の中で起きたエラーは `panic` させず、専用のメッセージ型に載せて
`Update` に返す。既存の `errorMsg`、`commentErrorMsg`、`stateErrorMsg` がその形。

エラーの種類ごとに表示場所が違う（画面全体か、フッターの 1 行か）ため、
メッセージ型を使い回さず、表示場所ごとに分ける。
