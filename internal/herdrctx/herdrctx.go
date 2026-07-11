// Package herdrctx resolves the directory GitHub Dash operates in
// from the Herdr plugin invocation context.
package herdrctx

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNoTargetDir is returned when the invocation context carries no usable cwd.
var ErrNoTargetDir = errors.New("herdr context has no workspace or pane cwd")

type invocationContext struct {
	WorkspaceCwd   string `json:"workspace_cwd"`
	FocusedPaneCwd string `json:"focused_pane_cwd"`
}

// Resolve picks the target directory from the HERDR_PLUGIN_CONTEXT_JSON value.
func Resolve(contextJSON string) (string, error) {
	if contextJSON == "" {
		return "", fmt.Errorf("HERDR_PLUGIN_CONTEXT_JSON is empty: %w", ErrNoTargetDir)
	}
	var ctx invocationContext
	if err := json.Unmarshal([]byte(contextJSON), &ctx); err != nil {
		return "", fmt.Errorf("parse HERDR_PLUGIN_CONTEXT_JSON: %w", err)
	}
	if ctx.WorkspaceCwd != "" {
		return ctx.WorkspaceCwd, nil
	}
	if ctx.FocusedPaneCwd != "" {
		return ctx.FocusedPaneCwd, nil
	}
	return "", ErrNoTargetDir
}
