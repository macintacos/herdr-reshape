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

// threeAcross is A | B | C, split twice to the right. Lopsided: 75 cells, then
// 37 and 37.
var threeAcross = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 75, 52}},
		{"B", Rect{110, 1, 37, 52}},
		{"C", Rect{147, 1, 37, 52}},
	},
	[]splitSpec{
		{"root", AxisRight, 0.5, Rect{35, 1, 149, 52}},
		{"s1", AxisRight, 0.5, Rect{110, 1, 74, 52}},
	},
)

// leftTwoRight is A full-height on the left, B over C on the right. Already an
// even 2x2 grid.
var leftTwoRight = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 75, 52}},
		{"B", Rect{110, 1, 74, 26}},
		{"C", Rect{110, 27, 74, 26}},
	},
	[]splitSpec{
		{"root", AxisRight, 0.5, Rect{35, 1, 149, 52}},
		{"s1", AxisDown, 0.5, Rect{110, 1, 74, 52}},
	},
)

// fittedAcross is threeAcross after a fit: root on 1/3, so the columns are 50,
// 50 and 49.
var fittedAcross = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 50, 52}},
		{"B", Rect{85, 1, 50, 52}},
		{"C", Rect{135, 1, 49, 52}},
	},
	[]splitSpec{
		{"root", AxisRight, 1.0 / 3.0, Rect{35, 1, 149, 52}},
		{"s1", AxisRight, 0.5, Rect{85, 1, 99, 52}},
	},
)

// deepRow is what splitting the rightmost pane seven times actually produces:
// every split halves, so the row runs 75, 37, 19, 9, 5, 2, 1, 1 cells. The
// degenerate end is the point. Panes narrower than [Tolerance] collapse into
// one grid line, which leaves the innermost split spanning no column at all and
// the one outside it wanting a ratio of 1.0 — so this is the fixture that
// reaches both the divide-by-zero guard and the clamp, and the only one that
// cannot be made even.
var deepRow = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 75, 52}},
		{"B", Rect{110, 1, 37, 52}},
		{"C", Rect{147, 1, 19, 52}},
		{"D", Rect{166, 1, 9, 52}},
		{"E", Rect{175, 1, 5, 52}},
		{"F", Rect{180, 1, 2, 52}},
		{"G", Rect{182, 1, 1, 52}},
		{"H", Rect{183, 1, 1, 52}},
	},
	[]splitSpec{
		{"s0", AxisRight, 0.5, Rect{35, 1, 149, 52}},
		{"s1", AxisRight, 0.5, Rect{110, 1, 74, 52}},
		{"s2", AxisRight, 0.5, Rect{147, 1, 37, 52}},
		{"s3", AxisRight, 0.5, Rect{166, 1, 18, 52}},
		{"s4", AxisRight, 0.5, Rect{175, 1, 9, 52}},
		{"s5", AxisRight, 0.5, Rect{180, 1, 4, 52}},
		{"s6", AxisRight, 0.5, Rect{182, 1, 2, 52}},
	},
)

// evenMinusMiddle is fittedAcross after the *middle* pane closes: s1 collapses
// into C, which takes the whole right two thirds, and root keeps the 1/3 it
// had. The shape a close gate has to recognise — uneven now, even one pane ago.
var evenMinusMiddle = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 50, 52}},
		{"C", Rect{85, 1, 99, 52}},
	},
	[]splitSpec{{"root", AxisRight, 1.0 / 3.0, Rect{35, 1, 149, 52}}},
)

// evenFourMinusSecond is an even four-column row after its *second* pane
// closes: A on the quarter it had, then C and D halving what is left. The
// closed pane's sibling was the s1 split rather than a leaf, which is the case
// a leaves-only search misses.
var evenFourMinusSecond = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 37, 52}},
		{"C", Rect{72, 1, 56, 52}},
		{"D", Rect{128, 1, 56, 52}},
	},
	[]splitSpec{
		{"root", AxisRight, 0.25, Rect{35, 1, 149, 52}},
		{"s1", AxisRight, 0.5, Rect{72, 1, 112, 52}},
	},
)

// evenRowsMinusMiddle is evenMinusMiddle stood on its side: three even *rows*
// whose middle pane closed, so C inherits the bottom two thirds. The mirror
// matters because the pane put back has to be put back on the same axis it left
// — a search that only ever splits sideways adds a column, leaves the row grid
// at two, and reads this tab as hand-tuned.
var evenRowsMinusMiddle = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 149, 17}},
		{"C", Rect{35, 18, 149, 35}},
	},
	[]splitSpec{{"root", AxisDown, 1.0 / 3.0, Rect{35, 1, 149, 52}}},
)

// tunedMinusOne is two panes left over from a tab dragged to a fifth. No pane
// put back anywhere makes 0.2 an even share, which is what keeps the close gate
// off a tuned tab.
var tunedMinusOne = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 30, 52}},
		{"B", Rect{65, 1, 119, 52}},
	},
	[]splitSpec{{"root", AxisRight, 0.2, Rect{35, 1, 149, 52}}},
)

// tunedMinusLast is the same story at the other end of the range: handTuned
// after C closes.
var tunedMinusLast = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 134, 52}},
		{"B", Rect{169, 1, 15, 52}},
	},
	[]splitSpec{{"root", AxisRight, 0.9, Rect{35, 1, 149, 52}}},
)

// handTuned is threeAcross dragged well off the grid: A takes nine tenths of
// the tab. One pane.resize cannot close that, so a fit has to take two.
var handTuned = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 134, 52}},
		{"B", Rect{169, 1, 8, 52}},
		{"C", Rect{177, 1, 7, 52}},
	},
	[]splitSpec{
		{"root", AxisRight, 0.9, Rect{35, 1, 149, 52}},
		{"s1", AxisRight, 0.5, Rect{169, 1, 15, 52}},
	},
)

// underTuned is threeAcross dragged the other way: A on a tenth, so a fit has
// to *raise* the root ratio. Every other fixture lowers one, which leaves the
// raising branch of the driving-leaf choice — the half that reads from the
// first child — untested.
var underTuned = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 15, 52}},
		{"B", Rect{50, 1, 67, 52}},
		{"C", Rect{117, 1, 67, 52}},
	},
	[]splitSpec{
		{"root", AxisRight, 0.1, Rect{35, 1, 149, 52}},
		{"s1", AxisRight, 0.5, Rect{50, 1, 134, 52}},
	},
)

// stackedPair is A over B, the issue's worked example for a move. Already an
// even 1x2 grid.
var stackedPair = layoutOf(
	[]paneSpec{
		{"A", Rect{35, 1, 149, 26}},
		{"B", Rect{35, 27, 149, 26}},
	},
	[]splitSpec{{"root", AxisDown, 0.5, Rect{35, 1, 149, 52}}},
)

// readFixture reads a fixture the way every entry point does: its tree and its
// targets.
func readFixture(layout Layout) (Node, Ratios) {
	root := Tree(layout)
	return root, EvenRatios(root, layout.Area)
}
