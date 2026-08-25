package reshape

import (
	"fmt"

	"github.com/macintacos/herdr-reshape/internal/geometry"
)

// Fit squares the tab pane sits in up into an even grid, returning how many
// resizes were issued — zero when the tab was already even.
//
// Every pass re-reads the layout rather than carrying the last reply forward.
// That costs one call a pass and buys the thing that matters: a burst of splits
// fires pane.created once per pane, so several fits run at once, and one
// planning against a snapshot two of its siblings have already acted on will
// drive a divider far past its target. Re-reading makes each pass correct
// against what the tab *is*, which is also what lets a later pass undo an
// earlier overshoot. A pass that asks for nothing ends the loop, which on an
// already-even tab is the very first one.
func (c *Client) Fit(pane geometry.PaneID) (int, error) {
	issued := 0
	for range geometry.MaxPasses {
		root, layout, err := c.tabTree(pane)
		if err != nil {
			return issued, err
		}
		if layout.Zoomed {
			break
		}
		calls := geometry.FitCalls(root, geometry.EvenRatios(root))
		if len(calls) == 0 {
			break
		}
		moved := false
		for _, call := range calls {
			changed, err := c.Resize(call.Pane, call.Direction, call.Amount)
			issued++
			if err != nil {
				return issued, err
			}
			if !changed {
				// A refusal is evidence some other fit in the burst already
				// moved the tab out from under this snapshot, so the rest of
				// this pass was planned against a layout that no longer
				// describes it. Re-plan instead of pushing on with stale
				// numbers. Refused on the *first* call there is nothing to
				// re-plan from, and the pass-shifted-nothing break below ends
				// the loop.
				break
			}
			moved = true
		}
		if !moved {
			// A whole pass that shifted nothing will plan the same thing again.
			break
		}
	}
	return issued, nil
}

// Move re-orients pane against the neighbour on direction, reporting whether it
// moved.
//
// A pane already on that side of its split, one filling a zoomed tab, and a
// round trip herdr refused all return false — and all three say so through
// [Client.Notify], because this runs from a keypress whose effect is otherwise
// invisible.
func (c *Client) Move(pane geometry.PaneID, direction geometry.Direction) (bool, error) {
	root, layout, err := c.tabTree(pane)
	if err != nil {
		return false, err
	}
	if layout.Zoomed {
		return false, c.Notify("Cannot move a zoomed pane", "Unzoom the tab first.")
	}

	plan := geometry.MovePlan(root, pane, direction)
	if plan.Kind == geometry.ReorientNothing {
		return false, c.Notify(
			fmt.Sprintf("No pane to move %s", direction),
			fmt.Sprintf("This pane is already the %smost here.", direction),
		)
	}
	if plan.Kind == geometry.ReorientSwap {
		if err := c.Swap(pane, plan.Target); err != nil {
			return false, err
		}
		return true, nil
	}

	// A round trip re-splits a fresh slot at 0.5, so it resizes the tab as a
	// side effect of reordering it. On a tab that was even, that alone would
	// read as hand-tuned and switch auto-fit off for good — so read the answer
	// now, while the tab is still the one that was asked about, and put the
	// grid back afterwards.
	wasEven := geometry.IsEven(root, geometry.EvenRatios(root))

	moved, err := c.reattach(pane, layout.TabID, plan, direction)
	if err != nil || !moved {
		return moved, err
	}
	if wasEven {
		// The pane is where it was asked to go, so a failure here is reported
		// but does not change the answer.
		_, err = c.Fit(pane)
	}
	return true, err
}

// reattach is the round trip [geometry.ReorientReattach] calls for: out to a
// temporary tab and back beside the neighbour, split along direction's axis. It
// reports whether the pane ended up somewhere the user can see.
//
// The terminal survives both hops, so whatever is running never notices, and
// the temporary tab removes itself the moment it empties.
func (c *Client) reattach(
	pane geometry.PaneID, tab geometry.TabID,
	plan geometry.Move, direction geometry.Direction,
) (bool, error) {
	if err := c.MoveToNewTab(pane); err != nil {
		return false, err
	}

	landed, err := c.MoveBeside(pane, tab, plan.Target, direction.Axis())
	if err != nil || !landed {
		// The pane is in a temporary tab until that call lands. If it did not,
		// say so — the pane is still alive, with whatever was running still
		// running, but it is somewhere the user was not looking and nothing
		// else will go and find it.
		//
		// A socket error strands it exactly the way a refusal does and reaches
		// only the plugin log, so both announce. The error still wins the
		// return, so the log keeps the detail.
		body := fmt.Sprintf("%s is in a temporary tab; move it back by hand.", pane)
		if notifyErr := c.Notify("Move failed", body); err == nil {
			err = notifyErr
		}
		return false, err
	}

	// Visible from here on, so a failure below is one the user can see and put
	// right — it does not get the announcement above.
	if plan.SwapBack {
		return true, c.Swap(pane, plan.Target)
	}
	return true, nil
}

// Created fits the tab a new pane landed in, if it was even before that pane
// arrived, and reports whether it did.
//
// A hand-tuned tab is left alone, and stays that way until it is tuned back
// into an even shape.
func (c *Client) Created(pane geometry.PaneID) (bool, error) {
	root, layout, err := c.tabTree(pane)
	if err != nil {
		return false, err
	}
	if layout.Zoomed {
		return false, nil
	}
	// The easy way round: the pane that just arrived is still in the tree, so
	// the layout runs backwards and the gate reads straight off the result.
	before := geometry.Without(root, pane)
	if before == nil || !geometry.IsEven(before, geometry.EvenRatios(before)) {
		return false, nil
	}
	if _, err := c.Fit(pane); err != nil {
		return false, err
	}
	return true, nil
}

// Closed fits the tab pane survives in, if a close is what knocked it off the
// grid, and reports whether it did.
//
// Closing a pane collapses its split into its sibling, and the sibling inherits
// the *whole* slot rather than an even share of the tab: thirds minus the
// middle pane is a third beside two thirds, not two halves. So there is
// something to correct here, and [geometry.CollapsedFromEven] is what decides
// whether correcting it would be trampling a tab someone tuned by hand.
func (c *Client) Closed(pane geometry.PaneID) (bool, error) {
	root, layout, err := c.tabTree(pane)
	if err != nil {
		return false, err
	}
	if layout.Zoomed {
		return false, nil
	}
	if !geometry.CollapsedFromEven(root) {
		return false, nil
	}
	if _, err := c.Fit(pane); err != nil {
		return false, err
	}
	return true, nil
}

// tabTree reads the tab pane sits in and rebuilds its split tree — the two
// lines every operation above opens with.
func (c *Client) tabTree(pane geometry.PaneID) (geometry.Node, geometry.Layout, error) {
	layout, err := c.Layout(pane)
	if err != nil {
		return nil, geometry.Layout{}, err
	}
	root, err := treeFrom(layout)
	if err != nil {
		return nil, geometry.Layout{}, err
	}
	return root, layout, nil
}

// treeFrom is [geometry.Tree] with a panic turned into an error.
//
// Recovered rather than pre-empted: validating first means re-deriving
// nodeFrom's child-matching search outside the package that owns it, which is a
// second copy of the one part of this that can be wrong — and a copy that
// drifts silently. A recover cannot disagree with Tree's real preconditions,
// and it turns the two Tree documents — an area naming no box, and a split
// whose children the reply omits — into the one line on stderr cmd.Execute
// already prints.
//
// It catches every panic, not only those two, so a bug inside geometry arrives
// here wearing a bad reply's clothing. That is the deliberate trade: this runs
// on every pane.created, and a goroutine dump per split is the outcome the
// one-line report exists to prevent. The message is the panic's own, so it
// still says which it was.
func treeFrom(layout geometry.Layout) (root geometry.Node, err error) {
	defer func() {
		if panicked := recover(); panicked != nil {
			root, err = nil, fmt.Errorf("pane.layout: %v", panicked)
		}
	}()
	return geometry.Tree(layout), nil
}
