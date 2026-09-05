package i18n_test

import (
	"testing"

	"github.com/kukv/octoscope/internal/i18n"
)

func TestUnresolvedIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rendered string
		want     []string
	}{
		{
			name:     "clean render has none",
			rendered: "Pull Requests  Issues\n  #12 fix the thing  @kukv",
			want:     nil,
		},
		{
			name:     "typo in a message ID leaks as !id",
			rendered: "!work.reveiw_requested  Issues",
			want:     []string{"work.reveiw_requested"},
		},
		{
			name:     "several on one line",
			rendered: "!a.b !c.d_e",
			want:     []string{"a.b", "c.d_e"},
		},
		{
			name:     "an exclamation mark in prose is not an ID",
			rendered: "done! and no.dots after a space",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := i18n.UnresolvedIDs(tt.rendered)
			if len(got) != len(tt.want) {
				t.Fatalf("%s: got %v, want %v", tt.name, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("%s: got[%d] = %q, want %q", tt.name, i, got[i], tt.want[i])
				}
			}
		})
	}
}
