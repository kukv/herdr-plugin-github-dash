package herdrctx

import (
	"errors"
	"testing"
)

// 実機検証（2026-07-12, herdr 0.7.1）で取得した実サンプル。
const realContextJSON = `{"workspace_id":"w4","workspace_label":"herdr-plugin-github-dash","workspace_cwd":"/home/tech/dev/ghq/github.com/kukv/herdr-plugin-github-dash","tab_id":"w4:t1","tab_label":"1","focused_pane_id":"w4:p2","focused_pane_cwd":"/home/tech/dev/ghq/github.com/kukv/herdr-plugin-github-dash","focused_pane_status":"unknown","invocation_source":"cli","correlation_id":"cli:plugin"}`

func TestResolveUsesWorkspaceCwd(t *testing.T) {
	dir, err := Resolve(realContextJSON)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	want := "/home/tech/dev/ghq/github.com/kukv/herdr-plugin-github-dash"
	if dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

func TestResolveFallsBackToFocusedPaneCwd(t *testing.T) {
	dir, err := Resolve(`{"focused_pane_cwd":"/some/pane/dir"}`)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if dir != "/some/pane/dir" {
		t.Errorf("dir = %q, want /some/pane/dir", dir)
	}
}

func TestResolveErrorsWhenNoCwd(t *testing.T) {
	for name, input := range map[string]string{
		"empty json":   `{}`,
		"empty value":  `{"workspace_cwd":"","focused_pane_cwd":""}`,
		"empty string": "",
	} {
		if _, err := Resolve(input); !errors.Is(err, ErrNoTargetDir) {
			t.Errorf("%s: err = %v, want ErrNoTargetDir", name, err)
		}
	}
}

func TestResolveErrorsOnInvalidJSON(t *testing.T) {
	if _, err := Resolve("not-json"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}
