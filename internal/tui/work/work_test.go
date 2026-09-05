package work

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kukv/octoscope/internal/gh"
)

type fakeSource struct {
	work  gh.Work
	err   error
	calls int
}

func (f *fakeSource) ListWork(ctx context.Context) (gh.Work, error) {
	f.calls++
	if err := ctx.Err(); err != nil {
		return gh.Work{}, err
	}
	return f.work, f.err
}

func sampleWork() gh.Work {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	var w gh.Work
	w[gh.SectionReviewRequested] = []gh.WorkItem{
		{
			Ref:   gh.ItemRef{Kind: gh.ItemPR, Repo: "kukv/octoscope", Number: 12},
			Title: "fix the thing", UpdatedAt: now,
			Checks: gh.Checks{Total: 3, Passed: 1, Failed: 1, Running: 1, State: gh.CheckFailure},
		},
		{
			Ref:   gh.ItemRef{Kind: gh.ItemPR, Repo: "kukv/koto", Number: 3},
			Title: "bump deps", UpdatedAt: now,
		},
	}
	w[gh.SectionAssigned] = []gh.WorkItem{
		{
			Ref:   gh.ItemRef{Kind: gh.ItemIssue, Repo: "kukv/octoscope", Number: 7},
			Title: "an issue", UpdatedAt: now,
		},
	}
	return w
}

// loaded returns a model that already received its data, sized wide enough
// for all four columns and the drawer.
func loaded() Model {
	m := New(&fakeSource{work: sampleWork()})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = m.Update(workMsg(sampleWork()))
	return m
}

// key builds the KeyPressMsg for a key name, matching the shape the app uses.
func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	default:
		return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
}

func press(m Model, k string) Model {
	m, _ = m.Update(key(k))
	return m
}

func TestCursorMovesWithinAColumn(t *testing.T) {
	m := press(loaded(), "j")
	if m.row != 1 {
		t.Errorf("row: got %d, want 1", m.row)
	}
	if m = press(m, "j"); m.row != 1 {
		t.Errorf("row past the end: got %d, want it clamped to 1", m.row)
	}
	if m = press(press(m, "k"), "k"); m.row != 0 {
		t.Errorf("row before the start: got %d, want it clamped to 0", m.row)
	}
}

func TestMovingToAShorterColumnClampsTheRow(t *testing.T) {
	m := press(loaded(), "j") // row 1 of the 2-item first column
	m = press(m, "l")         // column 1 is empty
	m = press(m, "l")         // column 2 holds one item
	if m.col != 2 {
		t.Fatalf("col: got %d, want 2", m.col)
	}
	if m.row != 0 {
		t.Errorf("row: got %d, want it clamped to 0", m.row)
	}
}

func TestColumnWrapsAtBothEnds(t *testing.T) {
	if m := press(loaded(), "h"); m.col != 3 {
		t.Errorf("h from column 0: got %d, want 3", m.col)
	}
	m := loaded()
	for range 4 {
		m = press(m, "l")
	}
	if m.col != 0 {
		t.Errorf("four l presses: got %d, want 0", m.col)
	}
}

func TestSelectedRefNamesTheItemUnderTheCursor(t *testing.T) {
	ref, ok := press(loaded(), "j").SelectedRef()
	if !ok {
		t.Fatal("SelectedRef reported no selection")
	}
	if ref.Repo != "kukv/koto" || ref.Number != 3 {
		t.Errorf("got %s#%d, want kukv/koto#3", ref.Repo, ref.Number)
	}
}

func TestEmptyColumnHasNoSelection(t *testing.T) {
	if _, ok := press(loaded(), "l").SelectedRef(); ok {
		t.Error("SelectedRef reported a selection in an empty column")
	}
}

func TestEnterAsksTheParentToOpenTheDetail(t *testing.T) {
	_, cmd := loaded().Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter produced no command")
	}
	msg, ok := cmd().(OpenDetailMsg)
	if !ok {
		t.Fatalf("got %T, want OpenDetailMsg", cmd())
	}
	if msg.Ref.Number != 12 {
		t.Errorf("got #%d, want #12", msg.Ref.Number)
	}
}

func TestEnterOnAnEmptyColumnDoesNothing(t *testing.T) {
	if _, cmd := press(loaded(), "l").Update(key("enter")); cmd != nil {
		t.Error("enter produced a command with nothing selected")
	}
}

func TestFetchFailureBecomesAnErrorMsg(t *testing.T) {
	m := New(&fakeSource{err: errors.New("boom")})
	_, cmd := m.Update(errMsg{errors.New("boom")})
	if cmd == nil {
		t.Fatal("no command returned for a failed fetch")
	}
	got, ok := cmd().(ErrorMsg)
	if !ok {
		t.Fatalf("got %T, want ErrorMsg", cmd())
	}
	if got.Err.Error() != "boom" {
		t.Errorf("got %q, want boom", got.Err)
	}
}

func TestRefreshCancelsThePreviousFetch(t *testing.T) {
	f := &fakeSource{work: sampleWork()}
	m := New(f)

	m, first := m.Refresh()
	m, second := m.Refresh()
	t.Cleanup(m.Cancel)

	if first == nil || second == nil {
		t.Fatal("Refresh returned no command")
	}
	// The second Refresh cancelled the first one's context, so the first
	// command reports a dead context instead of the fetched board.
	if _, ok := first().(errMsg); !ok {
		t.Errorf("first fetch: got %T, want errMsg", first())
	}
	if _, ok := second().(workMsg); !ok {
		t.Errorf("second fetch: got %T, want workMsg", second())
	}
	if f.calls != 2 {
		t.Errorf("ListWork calls: got %d, want 2", f.calls)
	}
}

func TestRefreshMarksTheBoardLoading(t *testing.T) {
	m := New(&fakeSource{work: sampleWork()})
	if m.loading {
		t.Error("New returned a loading model; the parent starts the first fetch")
	}
	m, _ = m.Refresh()
	t.Cleanup(m.Cancel)
	if !m.loading {
		t.Error("Refresh did not mark the board loading")
	}
	if m, _ = m.Update(workMsg(sampleWork())); m.loading {
		t.Error("the board is still loading after its data arrived")
	}
}

func TestRKeyRefetchesTheBoard(t *testing.T) {
	f := &fakeSource{work: sampleWork()}
	m := New(f)
	m, _ = m.Update(workMsg(sampleWork()))

	m, cmd := m.Update(key("r"))
	t.Cleanup(m.Cancel)
	if cmd == nil {
		t.Fatal("r produced no command")
	}
	if !m.loading {
		t.Error("r did not mark the board loading")
	}
	if _, ok := cmd().(workMsg); !ok {
		t.Errorf("got %T, want workMsg", cmd())
	}
	if f.calls != 1 {
		t.Errorf("ListWork calls: got %d, want 1", f.calls)
	}
}

func TestNewDataClampsTheCursor(t *testing.T) {
	m := press(loaded(), "j")
	m, _ = m.Update(workMsg(gh.Work{}))
	if m.row != 0 {
		t.Errorf("row after the data was replaced: got %d, want 0", m.row)
	}
	if _, ok := m.SelectedRef(); ok {
		t.Error("SelectedRef reported a selection in an emptied column")
	}
}
