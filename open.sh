#!/usr/bin/env bash
set -euo pipefail

herdr_bin="${HERDR_BIN_PATH:-herdr}"
url="${HERDR_PLUGIN_CLICKED_URL:-}"

args=(plugin pane open --plugin kukv.github-dash --entrypoint dash --focus)
if [[ -n "$url" ]]; then
  args+=(--env "GITHUB_DASH_URL=$url")
fi
exec "$herdr_bin" "${args[@]}"
