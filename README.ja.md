# octoscope

[English](README.md)

GitHub のプルリクエストと Issue を見渡すためのターミナルダッシュボード。

octoscope = **Octo**cat + **-scope**: 自分の GitHub の仕事を見渡す望遠鏡。

## 必要なもの

- [GitHub CLI](https://cli.github.com/)（`gh`）。`gh auth login` で認証済みであること

## インストール

[リリースページ](https://github.com/kukv/octoscope/releases)からプラットフォーム向けのバイナリを
ダウンロードするか、ソースからビルドする。

    go install github.com/kukv/octoscope/cmd/octoscope@latest

## 使い方

git リポジトリの中で実行する。

    octoscope

または任意のリポジトリを指定する。

    octoscope --repo kukv/octoscope

### フラグ

| フラグ | 説明 |
|---|---|
| `--repo owner/name` | 対象リポジトリ。デフォルトはカレントディレクトリのリポジトリ |
| `--lang en\|ja` | 表示言語。デフォルトはオペレーティングシステムのロケール |
| `--version` | バージョンを表示して終了する |

### キー

| キー | 一覧 | 詳細 |
|---|---|---|
| `j` / `k` | カーソル移動 | スクロール |
| `enter` | 詳細を開く | — |
| `tab` | PR / Issue を切り替え | — |
| `r` | 更新 | 更新 |
| `o` | ブラウザで開く | ブラウザで開く |
| `c` | — | コメント（`Ctrl+S` 送信 / `Esc` 中止） |
| `x` | — | クローズ / 再オープン（`y` 確定 / `n` 中止） |
| `l` | — | ラベルを編集（`space` 選択 / `enter` 適用） |
| `a` | — | 担当者を編集（`space` 選択 / `enter` 適用） |
| `esc` | — | 一覧に戻る |
| `q` | 終了 | 一覧に戻る |

## 多言語対応

octoscope は英語と日本語に対応している。表示言語はまず `--lang`、次にオペレーティング
システムのロケールから選ばれ、どちらもなければ英語にフォールバックする。
