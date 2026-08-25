package reshape

// Tabs measured off herdr 0.8.2 with pane.layout, on a 149x52 tab area — the
// same measurements internal/geometry's fixtures were built from, kept here as
// raw JSON rather than as struct literals so every test through one is also a
// test of the decode.

// layoutResult wraps a tab in the `result` object a pane.layout reply carries.
func layoutResult(tab string) string { return `{"layout":` + tab + `}` }

// threeAcross is three columns on halved splits: 50/25/25, the shape a burst of
// splits leaves behind and the one a fit exists to square up.
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
