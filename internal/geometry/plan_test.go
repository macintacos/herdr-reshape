package geometry

import (
	"math"
	"testing"
)

// TestEdgeLeaf checks the descent lands on the pane touching the edge it was
// asked for. Everything below reads its driving leaf and its move neighbour
// through this, so a flipped descent side sends both to the wrong pane.
func TestEdgeLeaf(t *testing.T) {
	root, ok := Tree(threeByTwo).(*Split)
	if !ok {
		t.Fatalf("the fixture has three panes, got %#v", Tree(threeByTwo))
	}
	rest := root.Kids[1]

	for _, c := range []struct {
		side Direction
		want PaneID
		why  string
	}{
		{DirectionLeft, "B", "B is leftmost beside A"},
		{DirectionRight, "D", "and D rightmost"},
		{DirectionDown, "C", "C is the bottom of that subtree"},
		{DirectionUp, "B", "a cross-axis split takes either child"},
	} {
		t.Run(c.why, func(t *testing.T) {
			if got := EdgeLeaf(rest, c.side).Pane; got != c.want {
				t.Errorf("EdgeLeaf(%s) = %q, want %q", c.side, got, c.want)
			}
		})
	}
}

// TestFitCalls checks a fit drives each divider from the side its direction can
// reach. Both signs, because they read from opposite children: threeAcross
// lowers a ratio and underTuned raises one.
func TestFitCalls(t *testing.T) {
	root, target := readFixture(threeAcross)
	calls := FitCalls(root, target)
	if len(calls) != 1 {
		t.Fatalf("only the root divider is off; s1 is already halved, got %v", calls)
	}
	// Measured on herdr 0.8.2: `resize B left` moves the root divider, because
	// B's own left edge *is* that divider. `resize A left` would move it too —
	// but by falling back off the tab edge, which is not a rule to build on.
	if calls[0].Split != "root" {
		t.Errorf("and it is the root that has to move, got %q", calls[0].Split)
	}
	if calls[0].Pane != "B" {
		t.Errorf("lowering a ratio drives from the far side of the divider, got %q", calls[0].Pane)
	}
	if calls[0].Direction != DirectionLeft {
		t.Errorf("and left is what lowers it, got %q", calls[0].Direction)
	}
	if math.Abs(calls[0].Amount-(0.5-1.0/3.0)) >= 1e-9 {
		t.Errorf("by exactly the distance to the target, got %v", calls[0].Amount)
	}

	root, target = readFixture(fittedAcross)
	if got := FitCalls(root, target); len(got) != 0 {
		t.Errorf("a fitted tab asks for nothing — this is idempotence, got %v", got)
	}

	root, target = readFixture(threeByTwo)
	calls = FitCalls(root, target)
	if len(calls) != 1 || calls[0].Split != "root" {
		t.Errorf("only root is off here, got %v", calls)
	}

	root, target = readFixture(handTuned)
	calls = FitCalls(root, target)
	if len(calls) != 1 {
		t.Fatalf("one divider off, got %v", calls)
	}
	if calls[0].Amount != MaxAmount {
		t.Errorf("a delta past the per-call cap is clamped to it, got %v", calls[0].Amount)
	}

	// The mirror of the threeAcross case above, and the only fixture that
	// reaches it: raising a ratio has to drive from a leaf touching the divider
	// from the *near* side, which is the first child's far edge.
	root, target = readFixture(underTuned)
	calls = FitCalls(root, target)
	if len(calls) != 1 {
		t.Fatalf("one divider off, got %v", calls)
	}
	if calls[0].Split != "root" {
		t.Errorf("A is far too narrow, so the root divider moves, got %q", calls[0].Split)
	}
	if calls[0].Pane != "A" {
		t.Errorf("raising a ratio drives from the near side of the divider, got %q", calls[0].Pane)
	}
	if calls[0].Direction != DirectionRight {
		t.Errorf("and right is what raises it, got %q", calls[0].Direction)
	}
}

// TestMovePlan checks a move re-orients against the sibling, not against travel
// direction — the distinction the whole command exists for.
func TestMovePlan(t *testing.T) {
	pair, row, twoRight := Tree(stackedPair), Tree(threeAcross), Tree(leftTwoRight)
	nested, gridded, alone := Tree(leftNested), Tree(threeByTwo), Tree(solo)

	for _, c := range []struct {
		root      Node
		pane      PaneID
		direction Direction
		want      Move
		why       string
	}{
		// The issue's own worked example, stated as it states it: two panes
		// stacked vertically, the bottom one moved right, ends up beside the top.
		{pair, "B", DirectionRight, Move{Kind: ReorientReattach, Target: "A"}, "the lower of two stacked panes moved right lands to the right of the upper"},
		{pair, "A", DirectionDown, Move{Kind: ReorientSwap, Target: "B"}, "and the upper moved down is a pure reorder within the same split"},

		// The ticket's own discriminator. B sits above C, both to the right of
		// A. Travelling right from B reaches nothing and travelling left reaches
		// A, but re-orienting B rightwards puts it beside its sibling C.
		{twoRight, "B", DirectionRight, Move{Kind: ReorientReattach, Target: "C"}, "B re-orients beside C, not towards A"},
		{twoRight, "B", DirectionLeft, Move{Kind: ReorientReattach, Target: "C", SwapBack: true}, "and leftwards lands it in front of C, which takes the extra swap"},

		{row, "C", DirectionLeft, Move{Kind: ReorientSwap, Target: "B"}, "two leaves in one split reorder with a swap, with no round trip"},
		{row, "C", DirectionRight, Move{Kind: ReorientNothing}, "the last child of a right split has no sibling further right"},
		{row, "B", DirectionLeft, Move{Kind: ReorientNothing}, "and the first child none further left — a move acts inside its own split"},
		{row, "A", DirectionRight, Move{Kind: ReorientSwap, Target: "B"}, "A's sibling is a subtree, but B fills its whole slot, so one swap does it"},
		{row, "A", DirectionUp, Move{Kind: ReorientReattach, Target: "B", SwapBack: true}, "a cross-axis move has to re-split, whatever the neighbour fills"},

		// A pane at index 1 whose sibling is a subtree: the neighbour is that
		// subtree's *far* edge, not its near one. Pinning this is what stops the
		// descent side being flipped without a test noticing. And because B
		// fills the sibling's whole slot, this is a swap, not a round trip.
		{nested, "C", DirectionLeft, Move{Kind: ReorientSwap, Target: "B"}, "the neighbour is the sibling subtree's far edge, and B fills its slot"},
		{nested, "A", DirectionRight, Move{Kind: ReorientSwap, Target: "B"}, "and within that subtree the two leaves simply trade places"},

		// A sibling subtree the neighbour does NOT fill: B is half-height, so
		// trading places with it would leave C where it was. That still needs
		// the round trip.
		{gridded, "A", DirectionRight, Move{Kind: ReorientReattach, Target: "B"}, "a neighbour narrower than its sibling's slot cannot just be swapped with"},

		// The descent runs along the PARENT's axis, not the requested one, and
		// this is the case that tells them apart: C's sibling is the top row,
		// and the leaf on the divider C sat under is B. Descending on `right`
		// instead would pick D, the far end of that row, and re-orient C
		// against the wrong neighbour.
		{gridded, "C", DirectionRight, Move{Kind: ReorientReattach, Target: "B"}, "the neighbour is the leaf across the divider, not the far end of the row"},

		{alone, "A", DirectionLeft, Move{Kind: ReorientNothing}, "no parent, no move"},
		{row, "Z", DirectionLeft, Move{Kind: ReorientNothing}, "no such pane"},
	} {
		t.Run(c.why, func(t *testing.T) {
			if got := MovePlan(c.root, c.pane, c.direction); got != c.want {
				t.Errorf("MovePlan(%q, %s) = %+v, want %+v", c.pane, c.direction, got, c.want)
			}
		})
	}
}
