package reshape

import (
	"strings"
	"testing"

	"github.com/macintacos/herdr-reshape/internal/geometry"
)

// --- fit ------------------------------------------------------------------

// TestFitOnAnEvenTabIssuesNothing is the idempotence claim, spelled as calls:
// an already-fitted tab costs one pane.layout and no pane.resize at all.
func TestFitOnAnEvenTabIssuesNothing(t *testing.T) {
	client, f := clientOf(layoutResult(fittedAcross))

	issued, err := client.Fit("A")
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if issued != 0 {
		t.Errorf("Fit issued %d resizes on an even tab", issued)
	}
	wantMethods(t, f, "pane.layout")
}

// TestFitRereadsTheLayoutEveryPass is why a burst of splits does not overshoot:
// several fits run at once, and one planning against a snapshot two of its
// siblings have already acted on drives a divider well past its target.
func TestFitRereadsTheLayoutEveryPass(t *testing.T) {
	client, f := clientOf(
		layoutResult(threeAcross),
		resized(true),
		layoutResult(fittedAcross),
	)

	issued, err := client.Fit("A")
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if issued != 1 {
		t.Errorf("one divider is off, so one resize; got %d", issued)
	}
	wantMethods(t, f, "pane.layout", "pane.resize", "pane.layout")
	// The divider is raised by aiming at a pane on its far side and sending the
	// resize the *other* way — the rule that makes a wrong-side pane quietly
	// drive some other divider.
	wantResizeTarget(t, f, 2, "B", geometry.DirectionLeft)
}

// TestFitStopsAtMaxPasses covers a tab that never converges: the loop is
// bounded rather than spinning, and the bound is not reached by luck — there is
// no fifth pane.layout.
func TestFitStopsAtMaxPasses(t *testing.T) {
	var script []string
	for range geometry.MaxPasses {
		script = append(script, layoutResult(threeAcross), resized(true))
	}
	client, f := clientOf(script...)

	issued, err := client.Fit("A")
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if issued != geometry.MaxPasses {
		t.Errorf("one resize a pass for %d passes, got %d", geometry.MaxPasses, issued)
	}
	if got := len(f.calls); got != 2*geometry.MaxPasses {
		t.Errorf("and then it stops: %d calls, want %d", got, 2*geometry.MaxPasses)
	}
}

// TestFitReplansWhenHerdrRefusesADivider checks a refusal mid-pass abandons the
// rest of that pass. Everything after a refused call was planned against a
// layout that no longer describes the tab, so pushing on means driving dividers
// with stale numbers.
func TestFitReplansWhenHerdrRefusesADivider(t *testing.T) {
	client, f := clientOf(
		layoutResult(skewedGrid), // three dividers off, so three calls this pass
		resized(true),
		resized(false), // refused, so the third call is never sent
		layoutResult(fittedAcross),
	)

	issued, err := client.Fit("A")
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if issued != 2 {
		t.Errorf("the third call was abandoned, so two were issued; got %d", issued)
	}
	wantMethods(t, f, "pane.layout", "pane.resize", "pane.resize", "pane.layout")
}

// TestFitStopsOnAPassThatShiftedNothing is the other half of that rule: a pass
// whose every call was refused would plan the very same thing again.
func TestFitStopsOnAPassThatShiftedNothing(t *testing.T) {
	client, f := clientOf(layoutResult(skewedGrid), resized(false))

	issued, err := client.Fit("A")
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if issued != 1 {
		t.Errorf("the refused call was issued and the other two were not, got %d", issued)
	}
	wantMethods(t, f, "pane.layout", "pane.resize")
}

// --- move -----------------------------------------------------------------

// TestMoveSwapsWhenTheSiblingIsOneWholeSlot checks the cheap path: a leaf
// filling its side of the divide can simply trade places, which is one call and
// no tab-bar flicker.
func TestMoveSwapsWhenTheSiblingIsOneWholeSlot(t *testing.T) {
	client, f := clientOf(layoutResult(pair), ok)

	movedIt, err := client.Move("A", geometry.DirectionRight)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if !movedIt {
		t.Error("A and B trade places")
	}
	wantMethods(t, f, "pane.layout", "pane.swap")
	wantCall(t, f, 2, "pane.swap", map[string]any{"source_pane_id": "A", "target_pane_id": "B"})
}

// TestMoveAnnouncesWhenThereIsNowhereToGo is the case that reads exactly like a
// broken keybinding: the rightmost pane of a row sent right leaves the tab
// untouched, and there is no scrollbar here to show which end you are at.
func TestMoveAnnouncesWhenThereIsNowhereToGo(t *testing.T) {
	client, f := clientOf(layoutResult(pair), ok)

	movedIt, err := client.Move("B", geometry.DirectionRight)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if movedIt {
		t.Error("B is already the rightmost pane")
	}
	wantMethods(t, f, "pane.layout", "notification.show")
	wantCall(t, f, 2, "notification.show", map[string]any{
		"title": "No pane to move right",
		"body":  "This pane is already the rightmost here.",
		"sound": "request",
	})
}

// TestMoveRoundTripsAndRefitsAnEvenTab is the whole reattach path. herdr
// refuses every within-tab move target, so re-orienting means going out to a
// temporary tab and back — and the fresh slot it comes back into is split at
// 0.5, which on a tab that was even would read as hand-tuned and switch
// auto-fit off for good.
func TestMoveRoundTripsAndRefitsAnEvenTab(t *testing.T) {
	client, f := clientOf(
		layoutResult(leftTwoRight),
		ok,                         // out to a new tab
		moved(true),                // and back beside B
		layoutResult(fittedAcross), // the fit that puts the grid back
	)

	movedIt, err := client.Move("A", geometry.DirectionRight)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if !movedIt {
		t.Error("A lands beside B")
	}
	wantMethods(t, f, "pane.layout", "pane.move", "pane.move", "pane.layout")
	wantCall(t, f, 2, "pane.move", map[string]any{
		"pane_id":     "A",
		"destination": map[string]any{"type": "new_tab"},
		"focus":       false,
	})
	wantCall(t, f, 3, "pane.move", map[string]any{
		"pane_id": "A",
		"destination": map[string]any{
			"type":           "tab",
			"tab_id":         "w1:tQ",
			"target_pane_id": "B",
			"split":          "right",
		},
		"focus": true,
	})
}

// TestMoveSwapsBackGoingLeft covers the extra call the round trip costs in the
// other direction: `split` only ever appends *after* its target.
//
// The tab here is one someone dragged off the grid, which is also the half of
// the previous test that has to be shown separately — no fit follows.
func TestMoveSwapsBackGoingLeft(t *testing.T) {
	client, f := clientOf(layoutResult(offsetRows), ok, moved(true), ok)

	movedIt, err := client.Move("A", geometry.DirectionLeft)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if !movedIt {
		t.Error("A lands in front of B")
	}
	wantMethods(t, f, "pane.layout", "pane.move", "pane.move", "pane.swap")
	wantCall(t, f, 4, "pane.swap", map[string]any{"source_pane_id": "A", "target_pane_id": "B"})
}

// TestMoveAnnouncesARefusedReattach is the one failure the user has to hear
// about: the pane is alive in a temporary tab, with whatever was running still
// running, but it is somewhere they were not looking and nothing else will go
// and find it.
func TestMoveAnnouncesARefusedReattach(t *testing.T) {
	client, f := clientOf(layoutResult(leftTwoRight), ok, moved(false), ok)

	movedIt, err := client.Move("A", geometry.DirectionRight)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if movedIt {
		t.Error("herdr did not apply the re-attach")
	}
	wantMethods(t, f, "pane.layout", "pane.move", "pane.move", "notification.show")
	wantCall(t, f, 4, "notification.show", map[string]any{
		"title": "Move failed",
		"body":  "A is in a temporary tab; move it back by hand.",
		"sound": "request",
	})
}

// --- created and closed ---------------------------------------------------

// TestCreatedFitsATabThatWasEvenBeforeTheSplit runs the layout backwards over
// the pane that just arrived, which is the whole gate — and the reason none of
// this has to be remembered between runs.
func TestCreatedFitsATabThatWasEvenBeforeTheSplit(t *testing.T) {
	client, f := clientOf(layoutResult(threeAcross), layoutResult(fittedAcross))

	fitted, err := client.Created("C")
	if err != nil {
		t.Fatalf("Created: %v", err)
	}
	if !fitted {
		t.Error("threeAcross minus C is an even two-column tab")
	}
	wantMethods(t, f, "pane.layout", "pane.layout")
}

// TestCreatedLeavesAHandTunedTabAlone is the gate saying no, and it stays that
// way until the tab is tuned back into an even shape.
func TestCreatedLeavesAHandTunedTabAlone(t *testing.T) {
	client, f := clientOf(layoutResult(offsetRows))

	fitted, err := client.Created("D")
	if err != nil {
		t.Fatalf("Created: %v", err)
	}
	if fitted {
		t.Error("the left column was dragged to 0.48 by hand")
	}
	wantMethods(t, f, "pane.layout")
}

// TestCreatedOnTheTabsOnlyPane covers the pane that had no tab to be even
// before it: running the layout backwards leaves nothing at all.
func TestCreatedOnTheTabsOnlyPane(t *testing.T) {
	client, f := clientOf(layoutResult(solo))

	fitted, err := client.Created("A")
	if err != nil {
		t.Fatalf("Created: %v", err)
	}
	if fitted {
		t.Error("there is no before-tab here")
	}
	wantMethods(t, f, "pane.layout")
}

// TestClosedFitsWhenACloseIsWhyTheTabIsOff checks the forwards-run gate: a
// close collapses its split into the sibling, and the sibling inherits the
// whole slot rather than an even share.
func TestClosedFitsWhenACloseIsWhyTheTabIsOff(t *testing.T) {
	client, f := clientOf(layoutResult(evenMinusMiddle), layoutResult(fittedAcross))

	fitted, err := client.Closed("A")
	if err != nil {
		t.Fatalf("Closed: %v", err)
	}
	if !fitted {
		t.Error("thirds minus the middle pane is where a close leaves an even tab")
	}
	wantMethods(t, f, "pane.layout", "pane.layout")
}

// TestClosedLeavesAHandTunedTabAlone checks both gates agree on where even
// stops: a close gate that fitted this would square up a tab the split gate had
// just promised not to touch.
func TestClosedLeavesAHandTunedTabAlone(t *testing.T) {
	client, f := clientOf(layoutResult(offsetRows))

	fitted, err := client.Closed("A")
	if err != nil {
		t.Fatalf("Closed: %v", err)
	}
	if fitted {
		t.Error("a tab a fiftieth off its grid line did not come from an even one")
	}
	wantMethods(t, f, "pane.layout")
}

// --- the two short-circuits -----------------------------------------------

// TestZoomedShortCircuitsEveryOperation checks a zoomed tab is never resized.
// It reports geometry for every pane it holds while showing one, so a fit over
// it would drive dividers nobody can see.
func TestZoomedShortCircuitsEveryOperation(t *testing.T) {
	t.Run("fit", func(t *testing.T) {
		client, f := clientOf(layoutResult(zoomed(threeAcross)))
		issued, err := client.Fit("A")
		if err != nil || issued != 0 {
			t.Errorf("Fit = %d, %v", issued, err)
		}
		wantMethods(t, f, "pane.layout")
	})

	t.Run("created", func(t *testing.T) {
		client, f := clientOf(layoutResult(zoomed(threeAcross)))
		if fitted, err := client.Created("C"); fitted || err != nil {
			t.Errorf("Created = %v, %v", fitted, err)
		}
		wantMethods(t, f, "pane.layout")
	})

	t.Run("closed", func(t *testing.T) {
		client, f := clientOf(layoutResult(zoomed(evenMinusMiddle)))
		if fitted, err := client.Closed("A"); fitted || err != nil {
			t.Errorf("Closed = %v, %v", fitted, err)
		}
		wantMethods(t, f, "pane.layout")
	})

	// move is the one that says so, because it ran from a keypress.
	t.Run("move", func(t *testing.T) {
		client, f := clientOf(layoutResult(zoomed(pair)), ok)
		if movedIt, err := client.Move("A", geometry.DirectionRight); movedIt || err != nil {
			t.Errorf("Move = %v, %v", movedIt, err)
		}
		wantMethods(t, f, "pane.layout", "notification.show")
		wantCall(t, f, 2, "notification.show", map[string]any{
			"title": "Cannot move a zoomed pane",
			"body":  "Unzoom the tab first.",
			"sound": "request",
		})
	})
}

// TestALayoutNoTreeCanBeBuiltFromIsAnError covers both replies
// geometry.Tree documents itself as panicking on. They reach herdr-reshape as
// one line on stderr, not as a goroutine dump — and internal/geometry is not
// reopened to get that.
func TestALayoutNoTreeCanBeBuiltFromIsAnError(t *testing.T) {
	for _, c := range []struct{ why, tab, wants string }{
		{"an area naming no box", areaNamesNoBox, "names no pane or split"},
		{"a split whose children the reply omits", splitMissingAChild, "no child on one side"},
	} {
		t.Run(c.why, func(t *testing.T) {
			client, _ := clientOf(layoutResult(c.tab))

			_, err := client.Fit("A")
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), c.wants) {
				t.Errorf("the error should say what is wrong with the reply, got %q", err)
			}
		})
	}
}

// wantResizeTarget asserts the nth (1-based) call aimed a resize at pane in
// direction, leaving the amount alone — client_test.go pins the params shape,
// and what matters here is which divider a call drives.
func wantResizeTarget(t *testing.T, f *fake, n int, pane geometry.PaneID, direction geometry.Direction) {
	t.Helper()
	if len(f.calls) < n {
		t.Fatalf("wanted %d calls, got %v", n, f.methods())
	}
	got := f.calls[n-1]
	if got.params["pane_id"] != string(pane) || got.params["direction"] != string(direction) {
		t.Errorf("call %d aims at %v %v, want %s %s",
			n, got.params["pane_id"], got.params["direction"], pane, direction)
	}
}
