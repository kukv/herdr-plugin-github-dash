# octoscope

[日本語](README.ja.md)

A terminal dashboard for GitHub pull requests and issues.

octoscope = **Octo**cat + **-scope**: a telescope for looking over your GitHub work.

## Requirements

- [GitHub CLI](https://cli.github.com/) (`gh`), authenticated via `gh auth login`

## Install

Download a binary for your platform from the
[releases page](https://github.com/kukv/octoscope/releases), or build from source:

    go install github.com/kukv/octoscope/cmd/octoscope@latest

## Usage

Run it inside a git repository:

    octoscope

Or point it at any repository:

    octoscope --repo kukv/octoscope

### Flags

| Flag | Description |
|---|---|
| `--repo owner/name` | Target repository. Defaults to the repository of the current directory. |
| `--lang en\|ja` | Display language. Defaults to the operating system locale. |
| `--version` | Print the version and exit. |

### Keys

| Key | List | Detail |
|---|---|---|
| `j` / `k` | move cursor | scroll |
| `enter` | open detail | — |
| `tab` | switch PRs / Issues | — |
| `r` | refresh | refresh |
| `o` | open in browser | open in browser |
| `c` | — | comment (`Ctrl+S` send / `Esc` cancel) |
| `x` | — | close / reopen (`y` confirm / `n` cancel) |
| `l` | — | edit labels (`space` toggle / `enter` apply) |
| `a` | — | edit assignees (`space` toggle / `enter` apply) |
| `esc` | — | back to list |
| `q` | quit | back to list |

## Localization

octoscope speaks English and Japanese. The language is chosen from `--lang`
first, then the operating system locale, and falls back to English.
