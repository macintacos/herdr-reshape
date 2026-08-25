package geometry

import (
	"slices"
	"testing"
)

// TestTree checks that the flat boxes rebuild into the nesting they came from.
// Nothing else in the suite notices a lost pane: every later check reads the
// tree this one proves, so a mis-parented subtree would silently narrow what
// they all run over.
func TestTree(t *testing.T) {
	root, ok := Tree(threeByTwo).(*Split)
	if !ok || root.ID != "root" {
		t.Fatalf("the tab area holds the root split, got %#v", Tree(threeByTwo))
	}

	left, ok := root.Kids[0].(*Leaf)
	if !ok || left.Pane != "A" {
		t.Errorf("a right split's first child is its left pane, got %#v", root.Kids[0])
	}
	rest, ok := root.Kids[1].(*Split)
	if !ok || rest.ID != "s1" {
		t.Fatalf("and its second the subtree beside it, got %#v", root.Kids[1])
	}
	row, ok := rest.Kids[0].(*Split)
	if !ok || row.ID != "s2" {
		t.Fatalf("s2 is s1's top child, got %#v", rest.Kids[0])
	}
	below, ok := rest.Kids[1].(*Leaf)
	if !ok || below.Pane != "C" {
		t.Errorf("and C the pane beneath it, got %#v", rest.Kids[1])
	}
	if got := paneNames(row); !slices.Equal(got, []PaneID{"B", "D"}) {
		t.Errorf("which holds B then D, got %v", got)
	}

	if got := paneNames(root); !slices.Equal(got, []PaneID{"A", "B", "D", "C"}) {
		t.Errorf("every pane, in order, got %v", got)
	}
	if got := splitNames(root); !slices.Equal(got, []SplitID{"root", "s1", "s2"}) {
		t.Errorf("outermost split first, got %v", got)
	}

	alone, ok := Tree(solo).(*Leaf)
	if !ok || alone.Pane != "A" {
		t.Errorf("a tab with one pane is a bare leaf, got %#v", Tree(solo))
	}

	// Left-nested: the root's *first* child is a subtree, so the search for it
	// has two candidates sharing the same near edge and must take the larger.
	// Taking the smaller silently drops B, which is why this shape is a fixture.
	nested := Tree(leftNested)
	if got := paneNames(nested); !slices.Equal(got, []PaneID{"A", "B", "C"}) {
		t.Errorf("a left-nested tree keeps B, got %v", got)
	}
	nestedRoot, ok := nested.(*Split)
	if !ok {
		t.Fatalf("three panes still need two splits, got %#v", nested)
	}
	if _, ok := nestedRoot.Kids[0].(*Split); !ok {
		t.Errorf("and this one nests to the left, not the right, got %#v", nestedRoot.Kids[0])
	}
}

// TestGrid checks that leaf edges become grid lines, and near-identical ones
// become one line. The merge is what stops a rounded rect manufacturing a
// phantom column that no fit could ever satisfy.
func TestGrid(t *testing.T) {
	lines := NewGrid(paneRects(threeByTwo), tabArea)
	if !slices.Equal(lines.Cols, []int{35, 110, 147, 184}) {
		t.Errorf("three columns, from four lines, got %v", lines.Cols)
	}
	if !slices.Equal(lines.Rows, []int{1, 27, 53}) {
		t.Errorf("and two rows, got %v", lines.Rows)
	}
	if got := lines.Index(AxisRight, 147); got != 2 {
		t.Errorf("an edge lands on its own line, got %d", got)
	}

	lines = NewGrid(paneRects(offsetRows), tabArea)
	if !slices.Equal(lines.Cols, []int{35, 110, 184}) {
		t.Errorf("two columns, got %v", lines.Cols)
	}
	if !slices.Equal(lines.Rows, []int{1, 26, 53}) {
		t.Errorf("a one-cell offset collapses instead of making a third row, got %v", lines.Rows)
	}
	if got := lines.Index(AxisDown, 27); got != 1 {
		t.Errorf("and the far edge still finds the merged line, got %d", got)
	}
}
