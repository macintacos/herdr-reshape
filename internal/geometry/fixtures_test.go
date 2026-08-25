package geometry

// Fixtures measured from herdr 0.8.2 with `pane.layout`, on a 149x52 tab area.
//
// They are the oracle this port is checked against: every number here came off
// a running server, so a Go translation that disagrees with them disagrees with
// herdr, not merely with the Python it was ported from.

// tabArea is the tab every fixture below fills. Prefixed because `area` is a
// parameter name throughout the package, and a shadowed fixture reads as a typo.
var tabArea = Rect{X: 35, Y: 1, Width: 149, Height: 52}

// paneSpec is one row of a fixture's pane table: the pane, and the box it fills.
type paneSpec struct {
	pane PaneID
	box  Rect
}

// splitSpec is one row of a fixture's divider table: the divider, the way it
// cuts, where it sits, and the box it cuts.
type splitSpec struct {
	id    SplitID
	axis  Axis
	ratio float64
	box   Rect
}

// layoutOf builds a trimmed `pane.layout`, written the way the measurements read.
func layoutOf(panes []paneSpec, dividers []splitSpec) Layout {
	layout := Layout{TabID: "w1:tQ", Area: tabArea}
	for _, p := range panes {
		layout.Panes = append(layout.Panes, PaneBox{PaneID: p.pane, Rect: p.box})
	}
	for _, d := range dividers {
		layout.Splits = append(layout.Splits, SplitBox{
			ID:        d.id,
			Direction: d.axis,
			Ratio:     d.ratio,
			Rect:      d.box,
		})
	}
	return layout
}

// solo is the one-pane tab: no dividers at all, which is the case every
// traversal has to survive without a split to descend through.
var solo = layoutOf([]paneSpec{{"A", Rect{35, 1, 149, 52}}}, nil)

// threeByTwo is A full-height on the left, B and D top right, C spanning
// beneath them.
var threeByTwo = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 75, 52}},
		{"B", Rect{110, 1, 37, 26}},
		{"D", Rect{147, 1, 37, 26}},
		{"C", Rect{110, 27, 74, 26}},
	},
	[]splitSpec{
		{"root", AxisRight, 0.5, Rect{35, 1, 149, 52}},
		{"s1", AxisDown, 0.5, Rect{110, 1, 74, 52}},
		{"s2", AxisRight, 0.5, Rect{110, 1, 74, 26}},
	},
)

// leftNested is A | B | C built LEFT-nested — root(s(A, B), C) — which is the
// shape herdr reports after a move re-parents a pane. Every other fixture nests
// to the right, so without this one a first/second mix-up in [Tree] loses a pane
// and nothing notices.
var leftNested = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 50, 52}},
		{"B", Rect{85, 1, 49, 52}},
		{"C", Rect{134, 1, 50, 52}},
	},
	[]splitSpec{
		{"root", AxisRight, 0.667, Rect{35, 1, 149, 52}},
		{"s", AxisRight, 0.5, Rect{35, 1, 99, 52}},
	},
)

// offsetRows is two columns whose horizontal dividers land a cell apart — 26 on
// the left, 27 on the right. One grid of two rows, not three; the offset is
// rounding.
var offsetRows = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 75, 25}},
		{"B", Rect{35, 26, 75, 27}},
		{"C", Rect{110, 1, 74, 26}},
		{"D", Rect{110, 27, 74, 26}},
	},
	[]splitSpec{
		{"root", AxisRight, 0.5, Rect{35, 1, 149, 52}},
		{"s1", AxisDown, 0.48, Rect{35, 1, 75, 52}},
		{"s2", AxisDown, 0.5, Rect{110, 1, 74, 52}},
	},
)

// paneNames is the leaf order a traversal produced, in the form the assertions
// read: a plain list of pane ids.
func paneNames(root Node) []PaneID {
	names := make([]PaneID, 0, 4)
	for _, leaf := range Leaves(root) {
		names = append(names, leaf.Pane)
	}
	return names
}

// splitNames is the divider order a traversal produced, likewise.
func splitNames(root Node) []SplitID {
	names := make([]SplitID, 0, 4)
	for _, split := range Splits(root) {
		names = append(names, split.ID)
	}
	return names
}

// paneRects is a layout's reported pane boxes — what a grid is read off when
// the tree has not been built yet.
func paneRects(layout Layout) []Rect {
	rects := make([]Rect, 0, len(layout.Panes))
	for _, pane := range layout.Panes {
		rects = append(rects, pane.Rect)
	}
	return rects
}
