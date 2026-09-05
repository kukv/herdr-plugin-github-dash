// Package work implements the Work board: the columns of what needs the
// user's attention, across every repository they touch.
package work

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kukv/octoscope/internal/gh"
)

// Source is what the Work board needs from the GitHub layer.
type Source interface {
	ListWork(ctx context.Context) (gh.Work, error)
}

type (
	workMsg gh.Work
	errMsg  struct{ err error }
)

// OpenDetailMsg asks the parent to show the detail view for the selected card.
type OpenDetailMsg struct{ Ref gh.ItemRef }

// ErrorMsg carries a failure the parent shows on its error screen.
type ErrorMsg struct{ Err error }

type Model struct {
	src Source

	width, height int
	loading       bool
	work          gh.Work
	col, row      int

	// fetchedAt is when the board's data arrived. The cards show relative
	// times, and View must render the same string from the same state, so the
	// clock is read once in Update rather than on every draw.
	fetchedAt time.Time

	// cancel stops the in-flight fetch. The board is the one place where a
	// request outlives the user's interest in it: they can switch tabs or ask
	// for a refresh while four searches are still running.
	cancel context.CancelFunc
}

func New(src Source) Model {
	return Model{src: src}
}

// Refresh cancels any in-flight fetch and starts a new one. It is also how
// the first fetch is started: a tea.Cmd-returning Init could not hand the
// cancel function back to the caller.
func (m Model) Refresh() (Model, tea.Cmd) {
	m.Cancel()
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.loading = true
	src := m.src
	return m, func() tea.Msg {
		w, err := src.ListWork(ctx)
		if err != nil {
			return errMsg{err}
		}
		return workMsg(w)
	}
}

// Cancel stops the in-flight fetch. The parent calls it when the user quits.
func (m Model) Cancel() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case workMsg:
		m.loading = false
		m.work = gh.Work(msg)
		m.fetchedAt = time.Now()
		m.clampCursor()
	case errMsg:
		m.loading = false
		err := msg.err
		return m, func() tea.Msg { return ErrorMsg{err} }
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "h", "left":
		m.col = wrapColumn(m.col-1, m.columns())
		m.clampCursor()
	case "l", "right":
		m.col = wrapColumn(m.col+1, m.columns())
		m.clampCursor()
	case "j", "down":
		if m.row+1 < len(m.work[m.section()]) {
			m.row++
		}
	case "k", "up":
		if m.row > 0 {
			m.row--
		}
	case "r":
		return m.Refresh()
	case "enter":
		if ref, ok := m.SelectedRef(); ok {
			return m, func() tea.Msg { return OpenDetailMsg{ref} }
		}
	}
	return m, nil
}

func (m Model) columns() int { return len(gh.WorkSections()) }

func (m Model) section() gh.WorkSection { return gh.WorkSections()[m.col] }

func wrapColumn(i, n int) int {
	switch {
	case i < 0:
		return n - 1
	case i >= n:
		return 0
	default:
		return i
	}
}

// clampCursor pulls the row back into range after the column changed or the
// data was replaced. It takes a pointer because it is only ever called on the
// local copy handleKey and Update are about to return.
func (m *Model) clampCursor() {
	n := len(m.work[m.section()])
	if m.row >= n {
		m.row = max(n-1, 0)
	}
}

// SelectedRef names the card under the cursor. ok is false when the column is
// empty.
func (m Model) SelectedRef() (gh.ItemRef, bool) {
	items := m.work[m.section()]
	if m.row >= len(items) {
		return gh.ItemRef{}, false
	}
	return items[m.row].Ref, true
}
