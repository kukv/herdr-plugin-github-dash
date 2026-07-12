// Command github-dash is the GitHub Dash pane process for Herdr.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/kukv/herdr-plugin-github-dash/internal/ghcli"
	"github.com/kukv/herdr-plugin-github-dash/internal/herdrctx"
	"github.com/kukv/herdr-plugin-github-dash/internal/ui"
)

var urlPattern = regexp.MustCompile(
	`^https://github\.com/([^/]+)/([^/]+)/(issues|pull)/([0-9]+)/?$`)

// parseTarget converts a clicked GitHub URL into a detail-view target.
func parseTarget(url string) *ui.Target {
	m := urlPattern.FindStringSubmatch(url)
	if m == nil {
		return nil
	}
	number, err := strconv.Atoi(m[4])
	if err != nil {
		return nil
	}
	kind := ui.KindIssue
	if m[3] == "pull" {
		kind = ui.KindPR
	}
	return &ui.Target{Kind: kind, Repo: m[1] + "/" + m[2], Number: number}
}

func main() {
	var model tea.Model
	dir, err := herdrctx.Resolve(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON"))
	if err != nil {
		model = ui.NewError(fmt.Sprintf(
			"could not resolve the target directory: %v\n\nRun GitHub Dash from a Herdr workspace.", err))
	} else {
		model = ui.New(ghcli.New(dir), parseTarget(os.Getenv("GITHUB_DASH_URL")))
	}
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
