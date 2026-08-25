package geometry

import "math"

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

// --- the herdr simulator --------------------------------------------------
//
// A model of the *server*, not of the plugin, which is why it lives in the test
// file: it reimplements herdr 0.8.2's measured resize rule independently of
// [FitCalls], so a fit's own claim about which divider it drives can be checked
// against what herdr would really move.

// landing is the number of cells a fitted pane may end up from an exactly even
// share. Targets are exact in ratio space, so this is purely what rendering
// costs: each nesting level rounds its rect to whole cells once, and the
// deepest layout worth fitting is a handful deep. It bounds what lands on
// screen; [Tolerance] bounds what counts as one grid line, which is a different
// question — and it bounds what the tests accept rather than what the package
// does, which is why it is not a package constant.
const landing = 2

// fixtures is every fixture a fit is checked against, so a new one joins all of
// them at once.
var fixtures = map[string]Layout{
	"three across":          threeAcross,
	"a 2x2 grid":            leftTwoRight,
	"the 3x2 example":       threeByTwo,
	"a hand-tuned tab":      handTuned,
	"an under-tuned tab":    underTuned,
	"a left-nested tab":     leftNested,
	"an already-fitted tab": fittedAcross,
	"two stacked panes":     stackedPair,
	"a deeply nested row":   deepRow,
}

// landable is fixtures minus deepRow, the one excluded from the landing bound:
// it holds panes a single cell wide, so no arrangement of it is within landing
// of an even share. Everything else can actually land, and has to.
var landable = func() map[string]Layout {
	out := make(map[string]Layout, len(fixtures)-1)
	for name, layout := range fixtures {
		if name != "a deeply nested row" {
			out[name] = layout
		}
	}
	return out
}()

// chain traces the path from node down to pane, or nil if it is not below it.
func chain(node Node, pane PaneID) []Node {
	split, ok := node.(*Split)
	if !ok {
		if node.(*Leaf).Pane == pane {
			return []Node{node}
		}
		return nil
	}
	for _, kid := range split.Kids {
		if found := chain(kid, pane); found != nil {
			return append([]Node{node}, found...)
		}
	}
	return nil
}

// driven works out which divider `pane.resize(pane, direction)` actually moves,
// returning "" when none does.
//
// This is herdr 0.8.2's measured rule, and the whole reason it is written out
// here: the divider on the **named side** of the pane, innermost first — and
// where that side is the tab edge, the one on the opposite side instead. It was
// read off all sixteen pane/direction cells of the 3x2 fixture, and it is what
// makes a fit's choice of driving leaf checkable without a running server.
func driven(root Node, pane PaneID, direction Direction) SplitID {
	path := chain(root, pane)
	if path == nil {
		return ""
	}
	leaf, axis := path[len(path)-1].(*Leaf), direction.Axis()

	var onAxis []*Split
	for _, node := range path[:len(path)-1] {
		if split, ok := node.(*Split); ok && split.Direction == axis {
			onAxis = append(onAxis, split)
		}
	}

	asked, other := leaf.Rect.Start(axis), leaf.Rect.End(axis)
	if direction.Forward() {
		asked, other = other, asked
	}
	for _, edge := range []int{asked, other} {
		for i := len(onAxis) - 1; i >= 0; i-- {
			if onAxis[i].Divider() == edge {
				return onAxis[i].ID
			}
		}
	}
	return ""
}

// drive applies resizes the way herdr would, resolving each to the divider it
// moves.
//
// Deliberately keyed off [driven] rather than off the split id the call records:
// that is what makes the harness catch a driving leaf picked from the wrong side
// of its divider, instead of assuming the call lands where it meant to.
func drive(root Node, calls []Resize) Node {
	moved := map[SplitID]float64{}
	for _, call := range calls {
		id := driven(root, call.Pane, call.Direction)
		if id == "" {
			continue
		}
		delta := call.Amount
		if !call.Direction.Forward() {
			delta = -delta
		}
		moved[id] += delta
	}

	var step func(Node) Node
	step = func(node Node) Node {
		split, ok := node.(*Split)
		if !ok {
			return node
		}
		out := *split
		out.Ratio = min(max(split.Ratio+moved[split.ID], MinRatio), MaxRatio)
		out.Kids = [2]Node{step(split.Kids[0]), step(split.Kids[1])}
		return &out
	}
	return relayout(step(root), root.Bounds())
}

// passes runs the real fit loop against simulated resizes, and counts the
// rounds. A layout still asking for work after [MaxPasses] reports one more
// than the bound, which is what a convergence check fails on.
func passes(layout Layout) int {
	root := Tree(layout)
	target := EvenRatios(root, layout.Area)
	for done := 0; done <= MaxPasses; done++ {
		calls := FitCalls(root, target)
		if len(calls) == 0 {
			return done
		}
		root = drive(root, calls)
		target = Targets(root, NewGrid(leafRects(root), layout.Area))
	}
	return MaxPasses + 1
}

// drift fits a layout, then reports the worst gap between a pane and its even
// share.
//
// Convergence is proved in ratio space, where the arithmetic is exact. This is
// the other half: what an exact ratio actually renders as once each nesting
// level has rounded its rect to whole cells.
func drift(layout Layout) int {
	root := Tree(layout)
	lines := NewGrid(leafRects(root), layout.Area)
	target := Targets(root, lines)
	for range MaxPasses {
		calls := FitCalls(root, target)
		if len(calls) == 0 {
			break
		}
		root = drive(root, calls)
		lines = NewGrid(leafRects(root), layout.Area)
		target = Targets(root, lines)
	}

	worst := 0
	for _, leaf := range Leaves(root) {
		for _, axis := range []Axis{AxisRight, AxisDown} {
			cells := len(lines.Lines(axis)) - 1
			spanned := lines.Index(axis, leaf.Rect.End(axis)) - lines.Index(axis, leaf.Rect.Start(axis))
			even := float64(spanned) * float64(layout.Area.Size(axis)) / float64(cells)
			// RoundToEven, not math.Round: Python's round() breaks a half to
			// even, and this number is compared against a bound of 2.
			worst = max(worst, int(math.RoundToEven(math.Abs(float64(leaf.Rect.Size(axis))-even))))
		}
	}
	return worst
}
