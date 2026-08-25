package geometry

import "math"

// Resize is one pane.resize: the divider it drives, the pane it aims at, and
// how far.
type Resize struct {
	Split     SplitID
	Pane      PaneID
	Direction Direction
	Amount    float64
}

// Reorient is what moving a pane against its neighbour comes down to, given the
// layout.
type Reorient string

const (
	// ReorientNothing means there is nothing to do: no sibling that way, or the
	// pane is already on that side.
	ReorientNothing Reorient = "nothing"
	// ReorientSwap is a pure reorder inside one split — no round trip, so no
	// tab-bar flicker.
	ReorientSwap Reorient = "swap"
	// ReorientReattach is the round trip out to a temporary tab and back beside
	// the neighbour, split along the direction asked for.
	ReorientReattach Reorient = "reattach"
)

// Move is how one pane re-orients against its neighbour: the primitive calls it
// comes down to, read off the layout.
type Move struct {
	Kind Reorient
	// Target is the pane to swap with, or the one to land beside. Empty on
	// [ReorientNothing], which is the only case that names no pane.
	Target PaneID
	// SwapBack is set because re-attaching only ever appends *after* Target, so
	// landing in front of it takes one more swap.
	SwapBack bool
}

// EdgeLeaf finds the leaf on node's side.
//
// A split across side's axis leaves both its children touching that edge, so
// either will do; a split along it puts only one of them there.
func EdgeLeaf(node Node, side Direction) *Leaf {
	for {
		split, ok := node.(*Split)
		if !ok {
			return node.(*Leaf)
		}
		if side.Forward() && split.Direction == side.Axis() {
			node = split.Kids[1]
		} else {
			node = split.Kids[0]
		}
	}
}

// FitCalls works out the resizes that would put every divider on its grid line.
//
// pane.resize takes the divider on the *named side* of the pane it is aimed at
// — falling back to the opposite side only at a tab edge — and moves it in that
// direction. So the sign lives in the direction, and the pane decides which
// divider moves: to raise a split's ratio, aim at a leaf on the divider's near
// side; to lower it, aim at one on the far side. A leaf picked from the wrong
// side quietly drives some other divider, and the tab never converges.
//
// Splits already on target are skipped, so a fitted tab yields nothing at all —
// which is the same statement as [IsEven].
func FitCalls(root Node, target Ratios) []Resize {
	var calls []Resize
	for _, split := range Splits(root) {
		want, aimed := target[split.ID]
		if !aimed {
			continue
		}
		delta := want - split.Ratio
		if math.Abs(delta) <= RatioEps {
			continue
		}
		raising := delta > 0
		// One Direction carries both halves of the choice: which way the divider
		// travels, and therefore which child holds a leaf touching it.
		side := DirectionAlong(split.Direction, raising)
		driving := split.Kids[1]
		if raising {
			driving = split.Kids[0]
		}
		calls = append(calls, Resize{
			Split:     split.ID,
			Pane:      EdgeLeaf(driving, side).Pane,
			Direction: side,
			Amount:    min(math.Abs(delta), MaxAmount),
		})
	}
	return calls
}

// MovePlan works out how to re-orient pane against the neighbour on direction.
//
// A move acts on the pane's *parent split*, which is what "put me beside the
// thing next to me" means — and why it is not the same as travelling to the
// pane in a given direction. Where the sibling is a subtree, the pane
// re-orients against that subtree's nearest leaf.
func MovePlan(root Node, pane PaneID, direction Direction) Move {
	held, index := parent(root, pane)
	if held == nil {
		return Move{Kind: ReorientNothing}
	}
	sibling := held.Kids[1-index]
	neighbour := EdgeLeaf(sibling, DirectionAlong(held.Direction, index == 1))

	if held.Direction == direction.Axis() {
		// Already on the side asked for: the leftmost pane of a row moved left
		// has no sibling that way.
		if (index == 1) == direction.Forward() {
			return Move{Kind: ReorientNothing}
		}
		// A leaf filling the sibling's whole slot across the divide *is* that
		// slot, so trading places with it is exactly what the round trip would
		// have produced — for one call, and without resizing anything.
		across := held.Direction.Across()
		if neighbour.Rect.Size(across) == sibling.Bounds().Size(across) {
			return Move{Kind: ReorientSwap, Target: neighbour.Pane}
		}
	}

	// pane.move refuses every within-tab target (same_tab), so re-orienting
	// means the round trip out to a temporary tab. `split` only ever appends
	// *after* its target, so going left or up takes one more swap to get in
	// front.
	return Move{Kind: ReorientReattach, Target: neighbour.Pane, SwapBack: !direction.Forward()}
}

// parent finds the split holding pane, and which of its two children it sits
// in. The split is nil when no split holds pane — a lone pane, or one that is
// not in this tab at all.
func parent(root Node, pane PaneID) (*Split, int) {
	for _, split := range Splits(root) {
		for index, kid := range split.Kids {
			if leaf, ok := kid.(*Leaf); ok && leaf.Pane == pane {
				return split, index
			}
		}
	}
	return nil, 0
}
