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

// Targets derives the ratio each split would carry if the tab were an even
// grid, clamped into the range herdr will actually drive a divider to.
//
// Reading a target off *grid indices* rather than off cell coordinates is what
// makes a fit idempotent: fitting an already-fitted tab computes the ratios it
// already has, where cell arithmetic would drift by whatever the last rounding
// cost.
func Targets(root Node, lines Grid) Ratios {
	target := Ratios{}
	for _, split := range Splits(root) {
		axis := split.Direction
		near := lines.Index(axis, split.Rect.Start(axis))
		far := lines.Index(axis, split.Rect.End(axis))
		mid := lines.Index(axis, split.Divider())
		if far == near {
			// The split's whole extent projected onto one line, so it spans no
			// grid cell and has no even share to aim at. herdr's 0.1 ratio floor
			// should keep every split wider than [Tolerance], but this runs on
			// the pane.created path, where being wrong is silent: the division
			// yields NaN, which the clamp propagates, [IsEven] reads as even
			// because every comparison against NaN is false, and [FitCalls]
			// cannot skip for the same reason.
			continue
		}
		target[split.ID] = min(max(float64(mid-near)/float64(far-near), MinRatio), MaxRatio)
	}
	return target
}

// IsEven reports whether every divider already sits on its grid line.
//
// The auto-fit gate and the idempotence proof are the same predicate: a tab is
// even exactly when a fit would do nothing to it. An uneven tab has been tuned
// by hand, and is left alone.
func IsEven(root Node, target Ratios) bool {
	for _, split := range Splits(root) {
		// A split [Targets] skipped has no even share to be off, so it cannot
		// make a tab uneven — and a fit will leave it alone for the same reason.
		if want, aimed := target[split.ID]; aimed && math.Abs(split.Ratio-want) > RatioEps {
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

// EvenRatios reads the ratio every split would carry if root filled its own box
// evenly.
//
// The box is read off root rather than taken as an argument: every tree this
// package hands back already fills the tab area — [Tree] returns the node whose
// rect *is* the area, and [Without] and reinsertions both re-derive from it — so
// an area parameter could only ever be told the same thing or told a lie.
//
// Gridded over the tree's own leaves rather than over a layout's panes: the two
// differ for anything herdr reports as a pane but does not put in the split tree
// — a plugin-owned popup — and a phantom column from one of those would bend
// every target on that axis.
func EvenRatios(root Node) Ratios {
	return Targets(root, NewGrid(leafRects(root), root.Bounds()))
}

// leafRects is what a grid is read off: one box per leaf pane.
func leafRects(root Node) []Rect {
	leaves := Leaves(root)
	rects := make([]Rect, 0, len(leaves))
	for _, leaf := range leaves {
		rects = append(rects, leaf.Rect)
	}
	return rects
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

// fitsEvenly reports whether every split of candidate that has an even share
// already sits on it.
func fitsEvenly(candidate Node, target Ratios) bool {
	// The re-inserted split is skipped because its ratio is one this search made
	// up — the real one left with the pane. Splits [Targets] skipped are skipped
	// for the reason [IsEven] skips them: no even share to be off.
	aimed := false
	for _, split := range Splits(candidate) {
		if split.ID == ghost {
			continue
		}
		want, ok := target[split.ID]
		if !ok {
			continue
		}
		aimed = true
		if math.Abs(split.Ratio-want) > RatioEps {
			return false
		}
	}
	// `aimed` so this fails closed. A candidate squeezed until nothing left in
	// it has an even share is not evidence of an even ancestry, and a vacuous
	// "every one of no splits is on target" would read as though it were.
	return aimed
}

// reinsertions is every tab that would become root if one pane closed out of it.
//
// Two per node rather than four: which *side* of its sibling the pane went back
// on cannot change the verdict. The re-inserted line always lands inside the
// sibling's own box, so it falls on the same side of every other divider either
// way, and a target is read from (mid - near) / (far - near) — which shifts with
// the line and is unchanged by it.
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
			// Re-derived from the tab area down, so the candidate's rects are
			// consistent with its own ratios — which is what makes the grid read
			// off them the grid this tab would really have had.
			candidates = append(candidates, relayout(swap(root, node, putback), root.Bounds()))
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
