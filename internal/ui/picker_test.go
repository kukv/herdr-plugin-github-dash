package ui

import (
	"strings"
	"testing"
)

func TestNewPickerPrechecksCurrent(t *testing.T) {
	p := newPicker(pickLabels, "Labels", []string{"bug", "wip"}, map[string]string{"bug": "d73a4a"}, []string{"bug"})
	if len(p.items) != 2 {
		t.Fatalf("items = %d, want 2", len(p.items))
	}
	if p.items[0].name != "bug" || !p.items[0].selected {
		t.Errorf("item0 = %+v, want bug selected", p.items[0])
	}
	if p.items[1].name != "wip" || p.items[1].selected {
		t.Errorf("item1 = %+v, want wip unselected", p.items[1])
	}
	if p.items[0].color != "d73a4a" {
		t.Errorf("color = %q, want d73a4a", p.items[0].color)
	}
}

func TestNewPickerIncludesCurrentNotInCandidates(t *testing.T) {
	// "bug" is currently applied but no longer in the candidate list; it must
	// still appear (selected) so enter does not silently remove it.
	p := newPicker(pickLabels, "Labels", []string{"wip"}, nil, []string{"bug"})
	var names []string
	for _, it := range p.items {
		names = append(names, it.name)
	}
	if len(p.items) != 2 {
		t.Fatalf("items = %v, want wip + bug", names)
	}
	found := false
	for _, it := range p.items {
		if it.name == "bug" && it.selected {
			found = true
		}
	}
	if !found {
		t.Errorf("current-but-uncandidate 'bug' missing or unselected: %v", names)
	}
}

func TestPickerToggleDiff(t *testing.T) {
	p := newPicker(pickLabels, "Labels", []string{"bug", "wip"}, nil, []string{"bug"})
	// cursor at 0 (bug, selected) -> toggle off (remove bug)
	p.toggle()
	// move to wip and toggle on (add wip)
	p.moveDown(10)
	p.toggle()
	add, remove := p.diff()
	if len(add) != 1 || add[0] != "wip" {
		t.Errorf("add = %v, want [wip]", add)
	}
	if len(remove) != 1 || remove[0] != "bug" {
		t.Errorf("remove = %v, want [bug]", remove)
	}
}

func TestPickerNoChangeEmptyDiff(t *testing.T) {
	p := newPicker(pickLabels, "Labels", []string{"bug", "wip"}, nil, []string{"bug"})
	add, remove := p.diff()
	if len(add) != 0 || len(remove) != 0 {
		t.Errorf("diff = %v/%v, want empty", add, remove)
	}
}

func TestPickerCursorAndScroll(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e"}
	p := newPicker(pickAssignees, "Assignees", names, nil, nil)
	// visible window of 2: moving down past the window advances offset
	for i := 0; i < 4; i++ {
		p.moveDown(2)
	}
	if p.cursor != 4 {
		t.Errorf("cursor = %d, want 4", p.cursor)
	}
	if p.offset != 3 { // window [3,5) shows cursor 4
		t.Errorf("offset = %d, want 3", p.offset)
	}
	p.moveDown(2) // already at last item, no-op
	if p.cursor != 4 {
		t.Errorf("cursor moved past end: %d", p.cursor)
	}
	for i := 0; i < 5; i++ {
		p.moveUp()
	}
	if p.cursor != 0 || p.offset != 0 {
		t.Errorf("cursor/offset = %d/%d, want 0/0", p.cursor, p.offset)
	}
}

func TestPickerListViewShowsItems(t *testing.T) {
	p := newPicker(pickLabels, "Labels", []string{"bug", "wip"}, nil, []string{"bug"})
	view := p.listView(20)
	for _, want := range []string{"Labels", "[x] bug", "[ ] wip"} {
		if !strings.Contains(view, want) {
			t.Errorf("listView missing %q:\n%s", want, view)
		}
	}
}
