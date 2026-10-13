package geometry

import "math"

// Ratios is a target ratio per split, which is what a fit is: the whole plan,
// as data.
type Ratios map[SplitID]float64

// ghost stands in for the pane a close took away, and for the split put back
// with it. It never reaches herdr — it exists only inside the candidate tabs
// [CollapsedFromEven] builds and throws away, so it stays off the package's
// exported surface. Untyped, because the search needs it as both a [SplitID]
// and a [PaneID].
const ghost = "__ghost__"

// EvenRatios derives the ratio each split would carry if the tab were an even
// grid, clamped into the range herdr will actually drive a divider to.
//
// A split's share is read off the tree's shape alone: how many panes each side
// lines up along the split's axis. Nothing positional enters into it, so a
// divider in one subtree is never a grid line in its sibling — two columns of
// two and three rows even out to halves and thirds — and a fitted tab computes
// exactly the ratios it already has, which is what makes a fit idempotent.
func EvenRatios(root Node) Ratios {
	target := Ratios{}
	for _, split := range Splits(root) {
		first := float64(span(split.Kids[0], split.Direction))
		second := float64(span(split.Kids[1], split.Direction))
		target[split.ID] = min(max(first/(first+second), MinRatio), MaxRatio)
	}
	return target
}

// span is how many panes node lines up along axis: both sides of a split along
// axis, or the busier side of one across it, since each of those spans the
// whole node. It is vim's rule for CTRL-W =.
func span(node Node, axis Axis) int {
	split, ok := node.(*Split)
	if !ok {
		return 1
	}
	first, second := span(split.Kids[0], axis), span(split.Kids[1], axis)
	if split.Direction == axis {
		return first + second
	}
	return max(first, second)
}

// IsEven reports whether every divider already sits on its grid line.
//
// The auto-fit gate and the idempotence proof are the same predicate: a tab is
// even exactly when a fit would do nothing to it. An uneven tab has been tuned
// by hand, and is left alone.
func IsEven(root Node, target Ratios) bool {
	for _, split := range Splits(root) {
		if math.Abs(split.Ratio-target[split.ID]) > RatioEps {
			return false
		}
	}
	return true
}

// Without collapses the tree the way closing pane would, returning the
// surviving tree with its rects re-derived — or nil if pane was the tab's only
// one.
//
// Run backwards over a pane that has just *opened*, this answers "was the tab
// even before this arrived?" — the gate auto-fit needs, and the reason none of
// it has to be remembered between runs.
func Without(root Node, pane PaneID) Node {
	pruned := prune(root, pane)
	if pruned == nil {
		return nil
	}
	return relayout(pruned, root.Bounds())
}

// prune is node without pane, any split left holding one child collapsing away.
func prune(node Node, pane PaneID) Node {
	split, ok := node.(*Split)
	if !ok {
		if node.(*Leaf).Pane == pane {
			return nil
		}
		return node
	}
	first, second := prune(split.Kids[0], pane), prune(split.Kids[1], pane)
	switch {
	case first != nil && second != nil:
		out := *split
		out.Kids = [2]Node{first, second}
		return &out
	case first != nil:
		return first
	case second != nil:
		return second
	default:
		return nil
	}
}

// relayout re-derives rects top-down the way herdr does: round half up on the
// ratio.
func relayout(node Node, rect Rect) Node {
	split, ok := node.(*Split)
	if !ok {
		out := *node.(*Leaf)
		out.Rect = rect
		return &out
	}
	// math.Floor(x + 0.5) rather than math.Round: the sizes here are positive so
	// the two agree, but this expression is the documented statement of how
	// herdr rounds, and reads as such.
	span := float64(rect.Size(split.Direction))
	near, far := rect.Cut(split.Direction, int(math.Floor(split.Ratio*span+0.5)))
	out := *split
	out.Rect = rect
	out.Kids = [2]Node{relayout(split.Kids[0], near), relayout(split.Kids[1], far)}
	return &out
}

// CollapsedFromEven reports whether an even tab is where root came from, one
// close ago.
//
// The created side asks the same question the easy way round: the pane that just
// arrived is still in the tree, so [Without] runs the layout backwards and reads
// the answer straight off. A close leaves nothing to run backwards. Measured on
// 0.8.2: pane.closed and pane.exited both fire *after* herdr has collapsed the
// split, pane.layout answers pane_not_found for the pane that went, and neither
// event carries the rects it had.
//
// So run it forwards instead — put a pane back. Whatever closed was the sibling
// of *some* node in what survives, so try every node in turn; if any of those
// tabs would be even while carrying the exact ratios this one still carries,
// then an even tab is where this one came from. The surviving ratios are the
// whole evidence, and they are good evidence: a close collapses one split and
// leaves every other ratio exactly as it was.
func CollapsedFromEven(root Node) bool {
	if IsEven(root, EvenRatios(root)) {
		return true
	}
	for _, candidate := range reinsertions(root) {
		if fitsEvenly(candidate, EvenRatios(candidate)) {
			return true
		}
	}
	return false
}

// fitsEvenly reports whether every split of candidate already sits on its even
// share.
func fitsEvenly(candidate Node, target Ratios) bool {
	// The re-inserted split is skipped because its ratio is one this search made
	// up — the real one left with the pane.
	for _, split := range Splits(candidate) {
		if split.ID != ghost && math.Abs(split.Ratio-target[split.ID]) > RatioEps {
			return false
		}
	}
	return true
}

// reinsertions is every tab that would become root if one pane closed out of it.
//
// Two per node rather than four: which *side* of its sibling the pane went back
// on cannot change the verdict. A span counts panes, not their order, and the
// re-inserted split's own ratio is never checked.
func reinsertions(root Node) []Node {
	var candidates []Node
	for _, node := range Nodes(root) {
		for _, axis := range []Axis{AxisRight, AxisDown} {
			back := &Leaf{Pane: ghost, Rect: node.Bounds()}
			putback := &Split{
				ID:        ghost,
				Direction: axis,
				Ratio:     0.5,
				Rect:      node.Bounds(),
				Kids:      [2]Node{node, back},
			}
			candidates = append(candidates, swap(root, node, putback))
		}
	}
	return candidates
}

// swap is node with the subtree old replaced by new, matched by identity —
// which is why the tree is built out of pointers.
func swap(node, old, replacement Node) Node {
	if node == old {
		return replacement
	}
	split, ok := node.(*Split)
	if !ok {
		return node
	}
	out := *split
	out.Kids = [2]Node{
		swap(split.Kids[0], old, replacement),
		swap(split.Kids[1], old, replacement),
	}
	return &out
}
