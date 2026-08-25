package geometry

import (
	"encoding/json"
	"maps"
	"math"
	"slices"
	"testing"
)

// TestTargets checks a target is its divider's grid index over the span the
// split covers — not its cell coordinates, which is what makes a fit idempotent.
func TestTargets(t *testing.T) {
	_, target := readFixture(threeAcross)
	if math.Abs(target["root"]-1.0/3.0) >= 1e-9 {
		t.Errorf("A takes the first of three columns, got %v", target["root"])
	}
	if math.Abs(target["s1"]-0.5) >= 1e-9 {
		t.Errorf("and B the first of the two that remain, got %v", target["s1"])
	}

	_, target = readFixture(leftTwoRight)
	if !maps.Equal(target, Ratios{"root": 0.5, "s1": 0.5}) {
		t.Errorf("a 2x2 grid halves both splits, got %v", target)
	}

	_, target = readFixture(threeByTwo)
	if math.Abs(target["root"]-1.0/3.0) >= 1e-9 {
		t.Errorf("A takes one column of three, got %v", target["root"])
	}
	if target["s1"] != 0.5 {
		t.Errorf("the top row is one of two, got %v", target["s1"])
	}
	if target["s2"] != 0.5 {
		t.Errorf("and B one of the two columns beside A, got %v", target["s2"])
	}
}

// TestIsEven checks that even means every divider already sits where a fit
// would put it — the auto-fit gate and the idempotence proof at once.
func TestIsEven(t *testing.T) {
	if root, target := readFixture(leftTwoRight); !IsEven(root, target) {
		t.Error("a 2x2 tab split down the middle is already even")
	}
	if root, target := readFixture(threeAcross); IsEven(root, target) {
		t.Error("three panes on halved splits are 50/25/25")
	}
	if root, target := readFixture(fittedAcross); !IsEven(root, target) {
		t.Error("the same tab after a fit reads as even")
	}
}

// TestWithout checks dropping a leaf collapses its split and re-derives the
// rects beneath. Run backwards over a pane that has just opened, this is the
// whole created-side gate.
func TestWithout(t *testing.T) {
	root := Without(Tree(threeAcross), "C")
	split, ok := root.(*Split)
	if !ok {
		t.Fatalf("two panes still need a split between them, got %#v", root)
	}
	if got := paneNames(root); !slices.Equal(got, []PaneID{"A", "B"}) {
		t.Errorf("s1 collapses into B, got %v", got)
	}
	if got := split.Kids[1].Bounds(); got != (Rect{110, 1, 74, 52}) {
		t.Errorf("B takes the whole slot, got %+v", got)
	}
	lines := NewGrid(leafRects(root), tabArea)
	if !IsEven(root, Targets(root, lines)) {
		t.Error("and the tab before the split was even")
	}

	root = Without(Tree(threeByTwo), "A")
	split, ok = root.(*Split)
	if !ok || split.ID != "s1" {
		t.Fatalf("dropping a root child promotes its sibling, got %#v", root)
	}
	if split.Rect != tabArea {
		t.Errorf("which then fills the tab, got %+v", split.Rect)
	}
	if got := split.Kids[0].Bounds(); got != (Rect{35, 1, 149, 26}) {
		t.Errorf("rects re-derive top-down, got %+v", got)
	}

	if got := Without(Tree(solo), "A"); got != nil {
		t.Errorf("nothing left, got %#v", got)
	}
}

// TestCollapsedFromEven checks the close gate reads an even ancestry off the
// survivors alone. A close leaves nothing to run backwards, so this runs the
// layout forwards instead — and the fixtures it must say no to matter as much
// as the ones it must say yes to.
func TestCollapsedFromEven(t *testing.T) {
	for _, c := range []struct {
		layout Layout
		want   bool
		why    string
	}{
		{fittedAcross, true, "an even tab needs no ancestry"},
		{solo, true, "one pane is even"},
		{evenMinusMiddle, true, "thirds minus the middle pane is where a close leaves an even tab"},
		{evenFourMinusSecond, true, "the closed pane's sibling can be a whole subtree, not just a leaf"},
		{evenRowsMinusMiddle, true, "and the pane goes back on the axis it left by, not always sideways"},
		{tunedMinusOne, false, "a tab dragged to a fifth did not come from an even one"},
		{tunedMinusLast, false, "nor did one dragged to nine tenths"},
		// A row too deep to ever be even is also too deep to have come from
		// one, and the degenerate splits it ends in are the ones Targets skips
		// — so this is the fixture that would pass vacuously if the search took
		// "no split had a target to miss" for "every split hit its target".
		{deepRow, false, "a degenerate row is unreachable"},
		// The two gates have to agree on where even stops. offsetRows sits 0.02
		// off its grid line, which is twenty times RatioEps, so the created
		// gate already leaves it alone; a close gate that fitted it would
		// square up a tab the split gate had just promised not to touch.
		{offsetRows, false, "and both gates say so"},
	} {
		t.Run(c.why, func(t *testing.T) {
			if got := CollapsedFromEven(Tree(c.layout), tabArea); got != c.want {
				t.Errorf("CollapsedFromEven = %v, want %v", got, c.want)
			}
		})
	}

	if root, target := readFixture(offsetRows); IsEven(root, target) {
		t.Error("a fiftieth off the grid is off the grid")
	}
}

// TestParsesRealJSON checks the model reads a pane.layout reply as herdr
// actually sends one — extra fields and all.
func TestParsesRealJSON(t *testing.T) {
	// Captured verbatim from herdr 0.8.2: the wire carries `focused`,
	// `focused_pane_id`, and `workspace_id`, which the model ignores.
	const reply = `{
	  "workspace_id": "w1",
	  "tab_id": "w1:tQ",
	  "zoomed": false,
	  "area": {"x": 35, "y": 1, "width": 149, "height": 52},
	  "focused_pane_id": "w1:p1X",
	  "panes": [
	    {"pane_id": "w1:p1X", "focused": true,  "rect": {"x": 35,  "y": 1, "width": 75, "height": 52}},
	    {"pane_id": "w1:p1Y", "focused": false, "rect": {"x": 110, "y": 1, "width": 37, "height": 52}},
	    {"pane_id": "w1:p1Z", "focused": false, "rect": {"x": 147, "y": 1, "width": 37, "height": 52}}
	  ],
	  "splits": [
	    {"id": "split_0_root", "direction": "right", "ratio": 0.5, "rect": {"x": 35,  "y": 1, "width": 149, "height": 52}},
	    {"id": "split_1_1",    "direction": "right", "ratio": 0.5, "rect": {"x": 110, "y": 1, "width": 74,  "height": 52}}
	  ]
	}`

	var layout Layout
	if err := json.Unmarshal([]byte(reply), &layout); err != nil {
		t.Fatalf("decode reply: %v", err)
	}
	if layout.TabID != "w1:tQ" {
		t.Errorf("the tab id survives the parse, got %q", layout.TabID)
	}

	root, target := readFixture(layout)
	want := []PaneID{"w1:p1X", "w1:p1Y", "w1:p1Z"}
	if got := paneNames(root); !slices.Equal(got, want) {
		t.Errorf("left to right, got %v", got)
	}
	if math.Abs(target["split_0_root"]-1.0/3.0) >= 1e-9 {
		t.Errorf("and it is the same three-column grid, got %v", target["split_0_root"])
	}
	if IsEven(root, target) {
		t.Error("which this reply is not yet on")
	}
}
