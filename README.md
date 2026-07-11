# herdr-plugin-github-dash
Herdr plugin for browsing and managing GitHub pull requests and issues

## Requirements

- [herdr](https://herdr.dev) >= 0.7.0
- [GitHub CLI](https://cli.github.com/) (`gh`), authenticated via `gh auth login`
- Go toolchain (used once at install time to build the pane binary)

## Install

    herdr plugin install kukv/herdr-plugin-github-dash

## Usage

- Run the **Open GitHub dashboard** action from a workspace to open the
  dashboard as an overlay pane for that workspace's repository.
- Ctrl+click a GitHub PR/issue URL in any terminal pane to open its detail
  view directly.

### Keys

| Key | List | Detail |
|---|---|---|
| `j` / `k` | move cursor | scroll |
| `enter` | open detail | — |
| `tab` | switch PRs / Issues | — |
| `r` | refresh | refresh |
| `o` | open in browser | open in browser |
| `esc` | — | back to list |
| `q` | quit | back to list |

## Development

    go test ./...
    go build -o bin/github-dash .
    herdr plugin link "$PWD"
