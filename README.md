# herdr-plugin-github-dash
Herdr plugin for browsing and managing GitHub pull requests and issues

## Requirements

- [herdr](https://herdr.dev) >= 0.7.0
- [GitHub CLI](https://cli.github.com/) (`gh`), authenticated via `gh auth login`
- Go toolchain (used once at install time to build the pane binary)

## Install

    herdr plugin install kukv/herdr-plugin-github-dash

Herdr has no default keybinding for plugin actions, so bind the
**Open GitHub dashboard** action yourself. Add this to
`~/.config/herdr/config.toml` and reload (`herdr server reload-config`):

    [[keys.command]]
    key = "prefix+alt+g"
    type = "plugin_action"
    command = "kukv.github-dash.open"

The `command` is `<plugin_id>.<action_id>`. You can also trigger the action
without a keybinding via `herdr plugin action invoke open --plugin kukv.github-dash`.

## Usage

- Press your keybinding (e.g. `prefix+alt+g`) from a workspace to open the
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

## Troubleshooting

### Ctrl+click opens the browser instead of the detail view

The host terminal is intercepting the modified click before Herdr sees it.
Herdr's link handler only fires if the click reaches Herdr's mouse layer.

- **Windows Terminal**: it auto-detects URLs and grabs Ctrl+click itself. Add
  `"experimental.detectURLs": false` under `profiles.defaults` in its
  `settings.json`, then restart Windows Terminal.
- Other terminals with a "Ctrl+click to open link" feature: disable it, or use
  the Herdr desktop app, which owns the mouse directly.

The URL must match `.../pull/<n>` or `.../issues/<n>` exactly — a trailing
`/files` or `#comment-...` will not trigger the handler. To verify the direct
mode independently of clicks:

    herdr plugin pane open --plugin kukv.github-dash --entrypoint dash --focus \
      --env GITHUB_DASH_URL=https://github.com/OWNER/REPO/pull/1

## Development

    go test ./...
    go build -o bin/github-dash .
    herdr plugin link "$PWD"

Remove the development link when done:

    herdr plugin unlink kukv.github-dash
