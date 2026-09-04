package reshape

import (
	"fmt"
	"strings"
)

// Tabs measured off herdr 0.8.2 with pane.layout, on a 149x52 tab area — the
// same measurements internal/geometry's fixtures were built from, kept here as
// raw JSON rather than as struct literals so every test through one is also a
// test of the decode.

// layoutResult wraps a tab in the `result` object a pane.layout reply carries.
func layoutResult(tab string) string { return `{"layout":` + tab + `}` }

// panesResult is a pane.list reply built from "pane@tab" pairs, in the order
// herdr listed them — which is the order the close sweep walks tabs in.
func panesResult(panes ...string) string {
	entries := make([]string, 0, len(panes))
	for _, pane := range panes {
		id, tab, _ := strings.Cut(pane, "@")
		entries = append(entries, fmt.Sprintf(`{"pane_id":%q,"tab_id":%q,"focused":false}`, id, tab))
	}
	return `{"panes":[` + strings.Join(entries, ",") + `]}`
}

// zoomed is tab with its zoom flag set. A zoomed tab still reports geometry for
// every pane it holds while showing exactly one, so every operation here
// short-circuits on it rather than resizing panes nobody can see.
func zoomed(tab string) string {
	return strings.Replace(tab, `"zoomed": false`, `"zoomed": true`, 1)
}

// resized is a pane.resize reply saying whether herdr moved the divider.
func resized(changed bool) string {
	if changed {
		return `{"resize":{"changed":true}}`
	}
	return `{"resize":{"changed":false}}`
}

// moved is a pane.move reply saying whether the pane landed.
func moved(changed bool) string {
	if changed {
		return `{"move_result":{"changed":true}}`
	}
	return `{"move_result":{"changed":false}}`
}

// ok is the reply for a call whose result nothing reads.
const ok = `{}`

// solo is the one-pane tab: no dividers at all, and nothing a close or a fit
// could be about.
const solo = `{
  "tab_id": "w1:tQ",
  "zoomed": false,
  "area": {"x": 35, "y": 1, "width": 149, "height": 52},
  "panes": [{"pane_id": "A", "focused": true, "rect": {"x": 35, "y": 1, "width": 149, "height": 52}}],
  "splits": []
}`

// pair is two even columns — the tab where a move is a straight swap, because
// the sibling is one leaf filling its whole slot.
const pair = `{
  "tab_id": "w1:tQ",
  "zoomed": false,
  "area": {"x": 35, "y": 1, "width": 149, "height": 52},
  "panes": [
    {"pane_id": "A", "focused": true,  "rect": {"x": 35,  "y": 1, "width": 75, "height": 52}},
    {"pane_id": "B", "focused": false, "rect": {"x": 110, "y": 1, "width": 74, "height": 52}}
  ],
  "splits": [
    {"id": "root", "direction": "right", "ratio": 0.5, "rect": {"x": 35, "y": 1, "width": 149, "height": 52}}
  ]
}`

// threeAcross is three columns on halved splits: 75, 37, 37, which is 50/25/25
// rather than thirds. The shape a burst of splits leaves behind, and the one a
// fit exists to square up.
const threeAcross = `{
  "workspace_id": "w1",
  "tab_id": "w1:tQ",
  "zoomed": false,
  "area": {"x": 35, "y": 1, "width": 149, "height": 52},
  "focused_pane_id": "A",
  "panes": [
    {"pane_id": "A", "focused": true,  "rect": {"x": 35,  "y": 1, "width": 75, "height": 52}},
    {"pane_id": "B", "focused": false, "rect": {"x": 110, "y": 1, "width": 37, "height": 52}},
    {"pane_id": "C", "focused": false, "rect": {"x": 147, "y": 1, "width": 37, "height": 52}}
  ],
  "splits": [
    {"id": "root", "direction": "right", "ratio": 0.5, "rect": {"x": 35,  "y": 1, "width": 149, "height": 52}},
    {"id": "s1",   "direction": "right", "ratio": 0.5, "rect": {"x": 110, "y": 1, "width": 74,  "height": 52}}
  ]
}`

// fittedAcross is threeAcross after a fit: root on a third, so the columns come
// out 50, 50 and 49. A fit over this one asks for nothing.
const fittedAcross = `{
  "tab_id": "w1:tQ",
  "zoomed": false,
  "area": {"x": 35, "y": 1, "width": 149, "height": 52},
  "panes": [
    {"pane_id": "A", "focused": true,  "rect": {"x": 35,  "y": 1, "width": 50, "height": 52}},
    {"pane_id": "B", "focused": false, "rect": {"x": 85,  "y": 1, "width": 50, "height": 52}},
    {"pane_id": "C", "focused": false, "rect": {"x": 135, "y": 1, "width": 49, "height": 52}}
  ],
  "splits": [
    {"id": "root", "direction": "right", "ratio": 0.3333333333333333, "rect": {"x": 35, "y": 1, "width": 149, "height": 52}},
    {"id": "s1",   "direction": "right", "ratio": 0.5,                "rect": {"x": 85, "y": 1, "width": 99,  "height": 52}}
  ]
}`

// skewedGrid is two columns of two rows with all *three* dividers off their
// grid line, so one pass over it plans three resizes: root wants 0.5 and has
// 0.4, s1 wants a third and has 0.3, s2 wants two thirds and has 0.7.
//
// Three is the number that matters. A refusal abandons the rest of its pass, and
// with only two calls the second one is the last one either way — so a fixture
// planning two cannot tell abandoning the pass apart from carrying on through
// it.
const skewedGrid = `{
  "tab_id": "w1:tQ",
  "zoomed": false,
  "area": {"x": 35, "y": 1, "width": 149, "height": 52},
  "panes": [
    {"pane_id": "A", "focused": true,  "rect": {"x": 35, "y": 1,  "width": 60, "height": 16}},
    {"pane_id": "B", "focused": false, "rect": {"x": 35, "y": 17, "width": 60, "height": 36}},
    {"pane_id": "C", "focused": false, "rect": {"x": 95, "y": 1,  "width": 89, "height": 36}},
    {"pane_id": "D", "focused": false, "rect": {"x": 95, "y": 37, "width": 89, "height": 16}}
  ],
  "splits": [
    {"id": "root", "direction": "right", "ratio": 0.4, "rect": {"x": 35, "y": 1, "width": 149, "height": 52}},
    {"id": "s1",   "direction": "down",  "ratio": 0.3, "rect": {"x": 35, "y": 1, "width": 60,  "height": 52}},
    {"id": "s2",   "direction": "down",  "ratio": 0.7, "rect": {"x": 95, "y": 1, "width": 89,  "height": 52}}
  ]
}`

// leftTwoRight is A full-height on the left, B over C on the right: an even 2x2
// grid. Moving A right cannot be a swap — the sibling is a split rather than a
// leaf filling its slot — so this is the round-trip fixture, and it is even, so
// the round trip has to be followed by a fit.
const leftTwoRight = `{
  "tab_id": "w1:tQ",
  "zoomed": false,
  "area": {"x": 35, "y": 1, "width": 149, "height": 52},
  "panes": [
    {"pane_id": "A", "focused": true,  "rect": {"x": 35,  "y": 1,  "width": 75, "height": 52}},
    {"pane_id": "B", "focused": false, "rect": {"x": 110, "y": 1,  "width": 74, "height": 26}},
    {"pane_id": "C", "focused": false, "rect": {"x": 110, "y": 27, "width": 74, "height": 26}}
  ],
  "splits": [
    {"id": "root", "direction": "right", "ratio": 0.5, "rect": {"x": 35,  "y": 1, "width": 149, "height": 52}},
    {"id": "s1",   "direction": "down",  "ratio": 0.5, "rect": {"x": 110, "y": 1, "width": 74,  "height": 52}}
  ]
}`

// offsetRows is two columns whose horizontal dividers land a cell apart, the
// left one dragged to 0.48 by hand. Twenty times RatioEps off its grid line, so
// both event gates decline it and a move over it is not followed by a fit.
const offsetRows = `{
  "tab_id": "w1:tQ",
  "zoomed": false,
  "area": {"x": 35, "y": 1, "width": 149, "height": 52},
  "panes": [
    {"pane_id": "A", "focused": true,  "rect": {"x": 35,  "y": 1,  "width": 75, "height": 25}},
    {"pane_id": "B", "focused": false, "rect": {"x": 35,  "y": 26, "width": 75, "height": 27}},
    {"pane_id": "C", "focused": false, "rect": {"x": 110, "y": 1,  "width": 74, "height": 26}},
    {"pane_id": "D", "focused": false, "rect": {"x": 110, "y": 27, "width": 74, "height": 26}}
  ],
  "splits": [
    {"id": "root", "direction": "right", "ratio": 0.5,  "rect": {"x": 35,  "y": 1, "width": 149, "height": 52}},
    {"id": "s1",   "direction": "down",  "ratio": 0.48, "rect": {"x": 35,  "y": 1, "width": 75,  "height": 52}},
    {"id": "s2",   "direction": "down",  "ratio": 0.5,  "rect": {"x": 110, "y": 1, "width": 74,  "height": 52}}
  ]
}`

// evenMinusMiddle is fittedAcross after the middle pane closed: s1 collapsed
// into C, which took the whole right two thirds, and root kept its third. The
// shape the close gate exists to recognise — uneven now, even one pane ago.
const evenMinusMiddle = `{
  "tab_id": "w1:tQ",
  "zoomed": false,
  "area": {"x": 35, "y": 1, "width": 149, "height": 52},
  "panes": [
    {"pane_id": "A", "focused": true,  "rect": {"x": 35, "y": 1, "width": 50, "height": 52}},
    {"pane_id": "C", "focused": false, "rect": {"x": 85, "y": 1, "width": 99, "height": 52}}
  ],
  "splits": [
    {"id": "root", "direction": "right", "ratio": 0.3333333333333333, "rect": {"x": 35, "y": 1, "width": 149, "height": 52}}
  ]
}`

// The two replies geometry.Tree documents itself as panicking on. Both describe
// something that is not a tab, so there is no tree to fall back to — but the
// caller holding the bytes is where the rejection belongs, and one line on
// stderr is what that looks like.

// areaNamesNoBox is a reply whose tab area matches neither a pane nor a split.
const areaNamesNoBox = `{
  "tab_id": "w1:tQ",
  "zoomed": false,
  "area": {"x": 35, "y": 1, "width": 149, "height": 52},
  "panes": [{"pane_id": "A", "focused": true, "rect": {"x": 0, "y": 0, "width": 10, "height": 10}}],
  "splits": []
}`

// splitMissingAChild is a reply carrying a divider whose right-hand pane it
// never lists.
const splitMissingAChild = `{
  "tab_id": "w1:tQ",
  "zoomed": false,
  "area": {"x": 35, "y": 1, "width": 149, "height": 52},
  "panes": [{"pane_id": "A", "focused": true, "rect": {"x": 35, "y": 1, "width": 75, "height": 52}}],
  "splits": [
    {"id": "root", "direction": "right", "ratio": 0.5, "rect": {"x": 35, "y": 1, "width": 149, "height": 52}}
  ]
}`
