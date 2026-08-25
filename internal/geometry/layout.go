// Package geometry is the half of this plugin that can be wrong: the grid
// model behind a move and a fit.
//
// It computes and nothing else — no socket, no files, no clock. herdr reports a
// tab flat (an area, a box per pane, a box per divider, and no parent links at
// all); [Tree] puts the nesting back, [NewGrid] projects every leaf edge onto
// shared column and row lines, and everything above those two is arithmetic
// over grid indices rather than over cells. That is what makes a fit
// idempotent; see [Targets].
//
// Four things herdr does that its documentation does not say, every one
// measured against 0.8.2 and every one load-bearing here:
//
//   - pane.resize moves the divider on the *named side* of the pane, falling
//     back to the opposite side only when that side is the tab edge. So the
//     sign lives in the direction, never in the amount, and the pane a resize
//     aims at decides which divider moves. See [FitCalls].
//   - A ratio is clamped to 0.1-0.9, and one call moves it at most 0.5. Two
//     calls therefore reach any target from any start, which is what bounds a
//     fit at [MaxPasses].
//   - pane.move will not re-orient a pane inside its own tab: every within-tab
//     target answers same_tab. See [MovePlan] for what that costs.
//   - pane.closed and pane.exited both fire *after* the split has collapsed —
//     pane.layout answers pane_not_found for the pane named in the payload —
//     and neither carries a tab id. So a close cannot be run backwards. See
//     [CollapsedFromEven].
package geometry

import (
	"fmt"
	"slices"
)

// herdr's opaque identifiers. Distinct types because nothing here ever means
// one where it says another, and a plain string would let it.
type (
	// PaneID identifies a pane.
	PaneID string
	// SplitID identifies a divider.
	SplitID string
	// TabID identifies a tab.
	TabID string
)

const (
	// Tolerance is the number of cells two leaf edges may differ by and still
	// count as one grid line. A split's rect rounds independently of its
	// parent's, so an edge two panes share can land a cell apart. herdr clamps a
	// split's ratio at 0.1, so the narrowest pane on a 149-cell tab is about 15
	// cells — far wider than this, which is why no real column is ever swallowed.
	Tolerance = 2

	// RatioEps is how far a split's ratio may sit from its grid target and still
	// read as even. Ratios come back as f32 — 0.65 reads as 0.6499999 — so this
	// has to clear that noise, and it stays under a cell on any plausible tab
	// width, so "even on paper" is even on screen.
	RatioEps = 0.002

	// MinRatio is the floor herdr will drive a divider to. A target outside the
	// range it and [MaxRatio] bound is unreachable — and an unreachable target
	// leaves every fit reporting work still to do, re-firing on every
	// pane.created for the life of the tab.
	MinRatio = 0.1
	// MaxRatio is that range's ceiling.
	MaxRatio = 0.9

	// MaxAmount is the largest delta one pane.resize applies; anything more is
	// clamped to it.
	MaxAmount = 0.5

	// MaxPasses bounds a fit loop. A ratio spans 0.8 at most and each pass
	// closes 0.5 of it, so two always land. The rest are here so that a
	// misjudged divider stops rather than spins.
	MaxPasses = 4
)

// Axis is the line a split divides along, spelled the way herdr spells it.
type Axis string

// The two axes herdr cuts a tab along.
const (
	AxisRight Axis = "right"
	AxisDown  Axis = "down"
)

// Across is the axis a split on this one leaves both children spanning in full.
func (a Axis) Across() Axis {
	if a == AxisRight {
		return AxisDown
	}
	return AxisRight
}

// Direction is one of herdr's four screen directions, for a move or a resize.
type Direction string

// The four directions a move or a resize can take.
const (
	DirectionLeft  Direction = "left"
	DirectionRight Direction = "right"
	DirectionUp    Direction = "up"
	DirectionDown  Direction = "down"
)

// Axis is the axis this direction runs along.
func (d Direction) Axis() Axis {
	if d == DirectionLeft || d == DirectionRight {
		return AxisRight
	}
	return AxisDown
}

// Forward reports whether this is right or down: the two that raise a split's
// ratio.
func (d Direction) Forward() bool {
	return d == DirectionRight || d == DirectionDown
}

// DirectionAlong names the direction running forward (or back) along axis.
func DirectionAlong(axis Axis, forward bool) Direction {
	if axis == AxisRight {
		if forward {
			return DirectionRight
		}
		return DirectionLeft
	}
	if forward {
		return DirectionDown
	}
	return DirectionUp
}

// Rect is a pane or split's box, in terminal cells, as herdr reports it.
//
// Comparable, deliberately: [Tree] keys a map by one to recover the nesting.
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Start is the near edge along axis.
func (r Rect) Start(axis Axis) int {
	if axis == AxisRight {
		return r.X
	}
	return r.Y
}

// Size is the extent along axis.
func (r Rect) Size(axis Axis) int {
	if axis == AxisRight {
		return r.Width
	}
	return r.Height
}

// End is the far edge along axis.
func (r Rect) End(axis Axis) int {
	return r.Start(axis) + r.Size(axis)
}

// Cut splits r into the two boxes a divider first cells along axis leaves behind.
func (r Rect) Cut(axis Axis, first int) (Rect, Rect) {
	if axis == AxisRight {
		return Rect{X: r.X, Y: r.Y, Width: first, Height: r.Height},
			Rect{X: r.X + first, Y: r.Y, Width: r.Width - first, Height: r.Height}
	}
	return Rect{X: r.X, Y: r.Y, Width: r.Width, Height: first},
		Rect{X: r.X, Y: r.Y + first, Width: r.Width, Height: r.Height - first}
}

// PaneBox is one leaf of a layout: a pane and the box it fills.
type PaneBox struct {
	PaneID PaneID `json:"pane_id"`
	Rect   Rect   `json:"rect"`
}

// SplitBox is one divider of a layout: which way it cuts, where it sits, and
// what it cuts.
type SplitBox struct {
	ID        SplitID `json:"id"`
	Direction Axis    `json:"direction"`
	Ratio     float64 `json:"ratio"`
	Rect      Rect    `json:"rect"`
}

// Layout is a tab's whole arrangement — and the only state this plugin has.
//
// herdr reports it flat: the tab area, a box per pane, a box per split, and no
// parent links at all. [Tree] puts the nesting back.
type Layout struct {
	// TabID is what a move re-attaches by, so a herdr that stopped reporting one
	// would strand a pane in the temporary tab it was passing through. Nothing
	// here decodes a reply, so enforcing its presence belongs at the wire
	// boundary rather than in this struct.
	TabID  TabID      `json:"tab_id"`
	Zoomed bool       `json:"zoomed"`
	Area   Rect       `json:"area"`
	Panes  []PaneBox  `json:"panes"`
	Splits []SplitBox `json:"splits"`
}

// PaneEntry is the slice of a pane.list reply this plugin reads. No geometry
// below touches it — it is the wire type the socket client decodes into.
type PaneEntry struct {
	PaneID  PaneID `json:"pane_id"`
	TabID   TabID  `json:"tab_id"`
	Focused bool   `json:"focused"`
}

// Node is one node of a tab's split tree: a [*Leaf] or a [*Split], and nothing
// else. Pointers rather than values because a reinsertion search matches a
// subtree by identity, and pointer equality is what reproduces that.
type Node interface {
	// Bounds is the box this node fills — the one question both kinds answer the
	// same way, and what a search reads off a node it has not looked inside.
	Bounds() Rect

	// node seals the interface over the two types below. Every traversal here
	// type-asserts without checking, so a third implementation would panic deep
	// in a descent; unexported, it cannot be written.
	node()
}

// Leaf is a pane, and the box it fills.
type Leaf struct {
	Pane PaneID
	Rect Rect
}

// Bounds implements [Node].
func (l *Leaf) Bounds() Rect { return l.Rect }

func (*Leaf) node() {}

// Split is a divider, the box it cuts, and the two nodes either side of it.
type Split struct {
	ID        SplitID
	Direction Axis
	Ratio     float64
	Rect      Rect
	Kids      [2]Node
}

// Bounds implements [Node].
func (s *Split) Bounds() Rect { return s.Rect }

func (*Split) node() {}

// Divider is where the two children meet: the far edge of the first one.
func (s *Split) Divider() int { return s.Kids[0].Bounds().End(s.Direction) }

// Splits is every split in the tree, outermost first.
func Splits(node Node) []*Split {
	split, ok := node.(*Split)
	if !ok {
		return nil
	}
	found := []*Split{split}
	for _, kid := range split.Kids {
		found = append(found, Splits(kid)...)
	}
	return found
}

// Leaves is every pane in the tree, left to right and top to bottom.
func Leaves(node Node) []*Leaf {
	split, ok := node.(*Split)
	if !ok {
		return []*Leaf{node.(*Leaf)}
	}
	var found []*Leaf
	for _, kid := range split.Kids {
		found = append(found, Leaves(kid)...)
	}
	return found
}

// Nodes is every node in the tree, dividers and panes alike, outermost first.
//
// A closed pane's sibling could have been either, so the close gate searches
// over this rather than over [Leaves].
func Nodes(node Node) []Node {
	found := []Node{node}
	if split, ok := node.(*Split); ok {
		for _, kid := range split.Kids {
			found = append(found, Nodes(kid)...)
		}
	}
	return found
}

// layoutBox is what [Tree] keys its map by: a pane's box or a split's box, the
// two things a layout reports flat.
type layoutBox interface{ bounds() Rect }

func (b PaneBox) bounds() Rect  { return b.Rect }
func (b SplitBox) bounds() Rect { return b.Rect }

// Tree rebuilds the split tree from a layout's flat boxes, returning the node
// filling the tab area — a [*Split] for any tab holding more than one pane, and
// a bare [*Leaf] for one that does not.
//
// The nesting is recovered from the rects rather than from the path-shaped
// split ids (split_2_10), because the rects are documented output and that id
// syntax is not.
//
// Tree takes a layout herdr really reported, and panics on one no tree can be
// built from: an area naming no box at all, or a split whose children the reply
// omits. Both describe a reply that is not a tab, so there is no tree to fall
// back to — but the caller decoding that reply is the one holding the bytes, and
// is where the rejection belongs.
func Tree(layout Layout) Node {
	// A box is the slot its node fills and children are strictly smaller, so no
	// two boxes collide: the root split and a lone pane both claim the whole
	// area, but only one of the two ever exists. Panes go in first anyway, so
	// that a malformed reply carrying both is read as a tree rather than as one
	// pane.
	boxes := make(map[Rect]layoutBox, len(layout.Panes)+len(layout.Splits))
	for _, pane := range layout.Panes {
		boxes[pane.Rect] = pane
	}
	for _, split := range layout.Splits {
		boxes[split.Rect] = split
	}
	root, ok := boxes[layout.Area]
	if !ok {
		panic(fmt.Sprintf("layout area %+v names no pane or split", layout.Area))
	}
	return nodeFrom(root, boxes)
}

// nodeFrom turns one box into a node, finding a split's children by the edges
// they share.
//
// A split's children fill its full extent *across* the divide, and share one
// edge with it each — the first its near edge, the second its far edge. Their
// sizes along the divide sum exactly, because a divider consumes no cell of its
// own, so the match is exact rather than approximate. Deeper descendants share
// those same edges, so the child is the larger of the candidates; anything
// larger still is an ancestor, and anything in a sibling subtree sits somewhere
// else entirely.
func nodeFrom(box layoutBox, boxes map[Rect]layoutBox) Node {
	if pane, ok := box.(PaneBox); ok {
		return &Leaf{Pane: pane.PaneID, Rect: pane.Rect}
	}
	split := box.(SplitBox)
	axis := split.Direction
	across := axis.Across()

	// Widest candidate wins on each side. Two candidates can only tie by having
	// the same rect, which the map above already deduped, so map order — random
	// in Go, insertion-ordered in the Python this came from — cannot change the
	// answer.
	var first, second layoutBox
	for _, other := range boxes {
		rect := other.bounds()
		inside := rect.Start(across) == split.Rect.Start(across) &&
			rect.Size(across) == split.Rect.Size(across) &&
			rect.Size(axis) < split.Rect.Size(axis)
		if !inside {
			continue
		}
		if rect.Start(axis) == split.Rect.Start(axis) &&
			(first == nil || rect.Size(axis) > first.bounds().Size(axis)) {
			first = other
		}
		if rect.End(axis) == split.Rect.End(axis) &&
			(second == nil || rect.Size(axis) > second.bounds().Size(axis)) {
			second = other
		}
	}

	if first == nil || second == nil {
		panic(fmt.Sprintf("split %s at %+v has no child on one side", split.ID, split.Rect))
	}

	return &Split{
		ID:        split.ID,
		Direction: axis,
		Ratio:     split.Ratio,
		Rect:      split.Rect,
		Kids:      [2]Node{nodeFrom(first, boxes), nodeFrom(second, boxes)},
	}
}

// Grid is the column and row lines every leaf edge was projected onto.
//
// N columns means N + 1 lines, the outermost two being the tab's own edges.
type Grid struct {
	Cols []int
	Rows []int
}

// Lines are the lines running across axis.
func (g Grid) Lines(axis Axis) []int {
	if axis == AxisRight {
		return g.Cols
	}
	return g.Rows
}

// Index is which line coord was projected onto.
func (g Grid) Index(axis Axis, coord int) int {
	lines := g.Lines(axis)
	best, nearest := 0, distance(lines[0], coord)
	for i := 1; i < len(lines); i++ {
		// Strictly nearer, so a coordinate exactly between two lines takes the
		// first — which is the behaviour the fixtures were measured against.
		if d := distance(lines[i], coord); d < nearest {
			best, nearest = i, d
		}
	}
	return best
}

// NewGrid projects every leaf edge onto shared column and row lines.
//
// This is what turns a tab into an N x M grid, and the merge is what keeps it
// honest: two panes meeting on the same line can report edges a cell apart,
// each having rounded off its own parent's ratio, and an unmerged pair would
// manufacture a one-cell column no fit could ever satisfy.
func NewGrid(rects []Rect, area Rect) Grid {
	return Grid{
		Cols: mergeEdges(edges(rects, area, AxisRight)),
		Rows: mergeEdges(edges(rects, area, AxisDown)),
	}
}

// edges is every edge along axis that could be a grid line, the area's included.
func edges(rects []Rect, area Rect, axis Axis) []int {
	found := make([]int, 0, 2*(len(rects)+1))
	found = append(found, area.Start(axis), area.End(axis))
	for _, rect := range rects {
		found = append(found, rect.Start(axis), rect.End(axis))
	}
	return found
}

// mergeEdges is sorted edges, keeping the first of each run that falls within
// [Tolerance].
func mergeEdges(found []int) []int {
	slices.Sort(found)
	var kept []int
	for _, edge := range found {
		if len(kept) == 0 || edge-kept[len(kept)-1] > Tolerance {
			kept = append(kept, edge)
		}
	}
	return kept
}

// distance is |a - b| for two coordinates, which Go has no builtin for.
func distance(a, b int) int { return max(a-b, b-a) }
