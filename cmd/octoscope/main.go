// Command octoscope is a standalone terminal dashboard for GitHub.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/kukv/octoscope/internal/ghcli"
	"github.com/kukv/octoscope/internal/ui"
)

func main() {
	repo := flag.String("repo", "",
		"target repository as owner/name; defaults to the repository of the current directory")
	flag.Parse()

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	p := tea.NewProgram(ui.New(ghcli.New(dir, *repo)))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
