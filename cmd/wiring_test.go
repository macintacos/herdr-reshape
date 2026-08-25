package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// This file checks the one thing manifest_test.go cannot: that each subcommand
// resolves its pane by the rule it is supposed to. herdr names the acting pane
// differently for an event hook than for a plugin action, and the three rules
// below were measured on 0.8.2 — a command reaching for the wrong one gates on,
// and fits, some other tab entirely.
//
// End to end over a real unix socket rather than through a seam: the socket and
// the environment *are* this layer's interface, and a stub client injected into
// the command tree would prove only that the glue calls itself.

// evenTab is a pane.layout reply for a tab a fit leaves alone, so a command
// under test costs exactly the calls its own wiring makes.
const evenTab = `{"result":{"layout":{
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
}}}`

// request is the slice of a request line these assertions read.
type request struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

// herdrStub stands in for a running herdr: a unix socket answering each method
// from a canned reply, recording every request it was sent.
type herdrStub struct {
	mu      sync.Mutex
	replies map[string]string
	got     []request
}

// serveHerdr starts a stub on a fresh socket and points HERDR_SOCKET_PATH at
// it. Call it before building a command tree — the client reads the path once,
// at construction.
func serveHerdr(t *testing.T, replies map[string]string) *herdrStub {
	t.Helper()
	// Not t.TempDir(): it embeds the test name, and a unix socket address is
	// capped at 104 bytes on darwin.
	dir, err := os.MkdirTemp("", "rs")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	path := filepath.Join(dir, "h.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	stub := &herdrStub{replies: replies}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			stub.answer(conn)
			_ = conn.Close()
		}
	}()

	t.Setenv("HERDR_SOCKET_PATH", path)
	return stub
}

func (s *herdrStub) answer(conn net.Conn) {
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return
	}
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return
	}
	s.mu.Lock()
	s.got = append(s.got, req)
	reply, ok := s.replies[req.Method]
	s.mu.Unlock()
	if !ok {
		reply = `{"result":{}}`
	}
	// Compacted, because the protocol is one line each way and the replies above
	// are written to be read. A real herdr sends them this way already.
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(reply)); err != nil {
		return
	}
	_, _ = conn.Write(append(compact.Bytes(), '\n'))
}

// requests is what the stub was sent, safe to read once the command returned.
func (s *herdrStub) requests() []request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]request(nil), s.got...)
}

// run executes one subcommand against the stub, with cobra's output discarded.
func run(t *testing.T, args ...string) error {
	t.Helper()
	root := newRootCmd("test")
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root.Execute()
}

// setPane sets both pane variables explicitly. Neither can be left to the
// ambient environment: these tests are as likely as not to be run from inside a
// herdr pane, which sets them.
func setPane(t *testing.T, hook, active string) {
	t.Helper()
	t.Setenv("HERDR_PANE_ID", hook)
	t.Setenv("HERDR_ACTIVE_PANE_ID", active)
}

// TestFitFallsBackThroughTheChain checks move and fit take the fallback chain
// the acceptance criteria name: a plugin action inherits neither variable, so
// the last resort is asking which pane has focus.
func TestFitFallsBackThroughTheChain(t *testing.T) {
	stub := serveHerdr(t, map[string]string{
		"pane.layout": evenTab,
		"pane.list":   `{"result":{"panes":[{"pane_id":"B","tab_id":"w1:tQ","focused":true}]}}`,
	})
	setPane(t, "", "")

	if err := run(t, "fit"); err != nil {
		t.Fatalf("fit: %v", err)
	}
	got := stub.requests()
	if len(got) != 2 || got[0].Method != "pane.list" || got[1].Method != "pane.layout" {
		t.Fatalf("want pane.list then pane.layout, got %v", methodsOf(got))
	}
	if got[1].Params["pane_id"] != "B" {
		t.Errorf("fit reads the tab of the focused pane, got %v", got[1].Params["pane_id"])
	}
}

// TestCreatedUsesOnlyTheEventsPaneID pins the deliberate absence of that chain.
// Measured on 0.8.2, the pane.created hook's HERDR_PANE_ID is the pane that was
// just created even when focus is in another tab — so falling back to the
// focused pane would gate on, and fit, some other tab.
func TestCreatedUsesOnlyTheEventsPaneID(t *testing.T) {
	stub := serveHerdr(t, map[string]string{
		"pane.layout": evenTab,
		"pane.list":   `{"result":{"panes":[{"pane_id":"B","tab_id":"w1:tQ","focused":true}]}}`,
	})
	// The variable a plugin action would have fallen back to, and no hook id.
	setPane(t, "", "B")

	if err := run(t, "created"); err != nil {
		t.Fatalf("created: %v", err)
	}
	if got := stub.requests(); len(got) != 0 {
		t.Errorf("created has no pane to act on, so it acts on none: %v", methodsOf(got))
	}
}

// TestClosedAsksWhichPaneHasFocus pins the third rule. pane.closed and
// pane.exited both fire after the split has collapsed, so the event's own pane
// is already pane_not_found, and neither event carries a tab id — wherever
// focus went is the only handle left on the right tab.
func TestClosedAsksWhichPaneHasFocus(t *testing.T) {
	stub := serveHerdr(t, map[string]string{
		"pane.layout": evenTab,
		"pane.list":   `{"result":{"panes":[{"pane_id":"B","tab_id":"w1:tQ","focused":true}]}}`,
	})
	// A hook id is set, and closed must still ignore it: it names the pane that
	// just went away.
	setPane(t, "A", "A")

	if err := run(t, "closed"); err != nil {
		t.Fatalf("closed: %v", err)
	}
	got := stub.requests()
	if len(got) < 2 || got[0].Method != "pane.list" {
		t.Fatalf("want pane.list first, got %v", methodsOf(got))
	}
	if got[1].Params["pane_id"] != "B" {
		t.Errorf("closed reads the focused pane's tab, not the event's; got %v", got[1].Params["pane_id"])
	}
}

// TestMoveAnnouncesWhenHerdrNamesNoPane checks the failure a keypress can
// actually see. stderr reaches nobody here, so this exits 0 with a
// notification rather than non-zero with a message.
func TestMoveAnnouncesWhenHerdrNamesNoPane(t *testing.T) {
	stub := serveHerdr(t, map[string]string{
		"pane.list": `{"result":{"panes":[]}}`,
	})
	setPane(t, "", "")

	if err := run(t, "move", "left"); err != nil {
		t.Fatalf("a pane herdr cannot name is not a crash: %v", err)
	}
	got := stub.requests()
	if len(got) != 2 || got[1].Method != "notification.show" {
		t.Fatalf("want pane.list then notification.show, got %v", methodsOf(got))
	}
	if got[1].Params["title"] != "Cannot move a pane" {
		t.Errorf("notification title, got %v", got[1].Params["title"])
	}
}

// TestASocketThatIsNotThereIsOneLine checks the event-hook path: a herdr that
// is not running has to fail as a plugin-log line, not a panic.
func TestASocketThatIsNotThereIsOneLine(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", filepath.Join(t.TempDir(), "absent.sock"))
	setPane(t, "A", "A")

	if err := run(t, "fit"); err == nil {
		t.Fatal("want an error")
	}
}

func methodsOf(got []request) []string {
	names := make([]string, 0, len(got))
	for _, r := range got {
		names = append(names, r.Method)
	}
	return names
}
