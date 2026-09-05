package i18n

import "regexp"

// unresolvedPattern matches what render() emits for a missing message ID:
// "!" immediately followed by a dotted identifier. Message IDs always have at
// least one dot ("work.review_requested"), which keeps ordinary prose ending
// in "!" from matching.
var unresolvedPattern = regexp.MustCompile(`!([a-z0-9_]+(?:\.[a-z0-9_]+)+)`)

// UnresolvedIDs returns the message IDs that failed to resolve in a rendered
// view, in the order they appear. A non-empty result means the code asked for
// an ID that is missing from the active catalog.
func UnresolvedIDs(rendered string) []string {
	matches := unresolvedPattern.FindAllStringSubmatch(rendered, -1)
	if len(matches) == 0 {
		return nil
	}
	ids := make([]string, len(matches))
	for i, m := range matches {
		ids[i] = m[1]
	}
	return ids
}

// TestingT is the part of *testing.T that AssertNoUnresolvedIDs needs.
// internal/i18n must not depend on other packages of ours, and taking the
// interface keeps this file to the standard library.
type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
}

// AssertNoUnresolvedIDs fails t when rendered contains a message ID that is
// missing from the active catalog.
func AssertNoUnresolvedIDs(t TestingT, rendered string) {
	t.Helper()
	for _, id := range UnresolvedIDs(rendered) {
		t.Errorf("unresolved message ID %q in the rendered view", id)
	}
}
