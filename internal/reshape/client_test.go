package reshape

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/macintacos/herdr-reshape/internal/geometry"
)

// --- the fake transport ---------------------------------------------------

// call is one request a fake recorded: the method name and the params map as
// the client really built them.
type call struct {
	method string
	params map[string]any
}

// fake is a transport that records every call and answers from a script of raw
// `result` objects, one per call in order.
//
// Running off the end of the script is an error rather than a silent empty
// reply: a change that adds a call would otherwise pass on a script written for
// the old sequence, and the sequence is the whole observable here.
type fake struct {
	replies []string
	calls   []call
}

func (f *fake) do(method string, params map[string]any) (json.RawMessage, error) {
	f.calls = append(f.calls, call{method: method, params: params})
	if len(f.calls) > len(f.replies) {
		return nil, fmt.Errorf("fake: call %d (%s) ran off a %d-reply script", len(f.calls), method, len(f.replies))
	}
	return json.RawMessage(f.replies[len(f.calls)-1]), nil
}

// methods is the sequence of method names recorded, which is what most
// assertions here are about.
func (f *fake) methods() []string {
	names := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		names = append(names, c.method)
	}
	return names
}

// clientOf builds a client over a scripted fake, returning both so a test can
// drive the one and read the other.
func clientOf(replies ...string) (*Client, *fake) {
	f := &fake{replies: replies}
	return &Client{do: f.do}, f
}

// wantCall asserts the nth (1-based) recorded call was method with params.
func wantCall(t *testing.T, f *fake, n int, method string, params map[string]any) {
	t.Helper()
	if len(f.calls) < n {
		t.Fatalf("wanted %d calls, got %v", n, f.methods())
	}
	got := f.calls[n-1]
	if got.method != method {
		t.Errorf("call %d is %s, want %s", n, got.method, method)
	}
	if !reflect.DeepEqual(got.params, params) {
		t.Errorf("call %d params:\n got %#v\nwant %#v", n, got.params, params)
	}
}

// envOf is a getenv reading from a map rather than from the process.
func envOf(pairs map[string]string) func(string) string {
	return func(key string) string { return pairs[key] }
}

// --- the transport --------------------------------------------------------

// sockPath is where a test's socket goes.
//
// Not t.TempDir(): that embeds the test's name in the path, and a unix socket
// address is capped at 104 bytes on darwin — so the longer test names below
// fail to bind, on a limit that has nothing to do with what they check.
func sockPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "rs")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "h.sock")
}

// listen starts a unix socket, handing every connection to handle, and returns
// the path to dial.
func listen(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	path := sockPath(t)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			handle(conn)
			_ = conn.Close()
		}
	}()
	return path
}

// replying is a handler that reads one request line, records it, and answers
// with reply followed by a newline — the shape herdr's socket speaks.
func replying(requests chan<- string, reply string) func(net.Conn) {
	return func(conn net.Conn) {
		line, _ := bufio.NewReader(conn).ReadString('\n')
		requests <- line
		_, _ = conn.Write([]byte(reply + "\n"))
	}
}

// TestRoundTripSpeaksOneLineEachWay pins the bytes on the wire. The id prefix
// and the trailing newline are both load-bearing — herdr reads a line at a time
// — and neither shows up in any other assertion here.
func TestRoundTripSpeaksOneLineEachWay(t *testing.T) {
	requests := make(chan string, 1)
	path := listen(t, replying(requests, `{"result":{"ok":true}}`))

	result, err := NewClient(path, Timeout).do("pane.resize", map[string]any{"pane_id": "A", "amount": 0.25})
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	want := `{"id":"reshape:pane.resize","method":"pane.resize","params":{"amount":0.25,"pane_id":"A"}}` + "\n"
	if got := <-requests; got != want {
		t.Errorf("request line:\n got %q\nwant %q", got, want)
	}
	if string(result) != `{"ok":true}` {
		t.Errorf("the envelope's result and nothing else, got %s", result)
	}
}

// TestRoundTripAcceptsAReplyWithNoTrailingNewline covers a server that writes a
// complete reply and closes: the read returns the bytes alongside io.EOF, which
// is data rather than a failure. The Python's `while b"\n" not in reply` loop
// broke on the same empty chunk.
func TestRoundTripAcceptsAReplyWithNoTrailingNewline(t *testing.T) {
	path := listen(t, func(conn net.Conn) {
		_, _ = bufio.NewReader(conn).ReadString('\n')
		_, _ = conn.Write([]byte(`{"result":{"ok":true}}`))
	})

	result, err := NewClient(path, Timeout).do("pane.list", map[string]any{})
	if err != nil {
		t.Fatalf("a complete reply is a complete reply: %v", err)
	}
	if string(result) != `{"ok":true}` {
		t.Errorf("got %s", result)
	}
}

// TestRoundTripRejectsAReplyCarryingNoResult checks that herdr saying anything
// other than a result is a failure the caller hears about. The Python raised
// KeyError here; nothing documents herdr's error shape, so none is invented.
func TestRoundTripRejectsAReplyCarryingNoResult(t *testing.T) {
	for _, reply := range []string{`{"error":{"code":"pane_not_found"}}`, `{"result":null}`, `not json`} {
		t.Run(reply, func(t *testing.T) {
			requests := make(chan string, 1)
			path := listen(t, replying(requests, reply))

			if _, err := NewClient(path, Timeout).do("pane.layout", map[string]any{}); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

// TestRoundTripTimesOut checks the deadline is the client's own, so an event
// hook cannot hang on a server that accepted the connection and went quiet.
func TestRoundTripTimesOut(t *testing.T) {
	ctx := t.Context()
	path := listen(t, func(net.Conn) { <-ctx.Done() })

	start := time.Now()
	if _, err := NewClient(path, 50*time.Millisecond).do("pane.list", map[string]any{}); err == nil {
		t.Fatal("a server that never replies is an error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("the deadline did not fire, took %v", elapsed)
	}
}

// TestRoundTripReportsADeadSocket checks the connect failure surfaces rather
// than becoming a nil reply — this is the path a herdr that is not running
// takes, and cmd.Execute prints it as one line.
func TestRoundTripReportsADeadSocket(t *testing.T) {
	if _, err := NewClient(sockPath(t), Timeout).do("pane.list", nil); err == nil {
		t.Fatal("want an error dialling a socket that is not there")
	}
}

// TestSocketPath checks the three-step resolution, and that it reads the
// environment through its parameter rather than around it.
func TestSocketPath(t *testing.T) {
	for _, c := range []struct {
		why  string
		env  map[string]string
		want string
	}{
		{"an explicit socket wins", map[string]string{
			"HERDR_SOCKET_PATH": "/run/herdr.sock",
			"XDG_CONFIG_HOME":   "/xdg",
			"HOME":              "/home/j",
		}, "/run/herdr.sock"},
		{"then the XDG config dir", map[string]string{
			"XDG_CONFIG_HOME": "/xdg",
			"HOME":            "/home/j",
		}, "/xdg/herdr/herdr.sock"},
		{"and .config under HOME last", map[string]string{
			"HOME": "/home/j",
		}, "/home/j/.config/herdr/herdr.sock"},
	} {
		t.Run(c.why, func(t *testing.T) {
			if got := SocketPath(envOf(c.env)); got != c.want {
				t.Errorf("SocketPath = %q, want %q", got, c.want)
			}
		})
	}
}

// --- the typed calls ------------------------------------------------------

func TestLayoutSendsThePaneAndDecodesTheTab(t *testing.T) {
	client, f := clientOf(layoutResult(threeAcross))

	layout, err := client.Layout("A")
	if err != nil {
		t.Fatalf("Layout: %v", err)
	}
	wantCall(t, f, 1, "pane.layout", map[string]any{"pane_id": "A"})
	if layout.TabID != "w1:tQ" {
		t.Errorf("tab id, got %q", layout.TabID)
	}
	if got := len(layout.Panes); got != 3 {
		t.Errorf("three panes, got %d", got)
	}
	if layout.Area != (geometry.Rect{X: 35, Y: 1, Width: 149, Height: 52}) {
		t.Errorf("the tab area, got %+v", layout.Area)
	}
}

// TestLayoutRejectsAReplyWithNoTabID is the guarantee internal/geometry
// deliberately does not carry: a move re-attaches by tab id, so a layout
// missing one would strand a pane in the temporary tab it was passing through.
// This is the only place in the program holding the bytes.
func TestLayoutRejectsAReplyWithNoTabID(t *testing.T) {
	client, _ := clientOf(`{"layout":{"zoomed":false,"area":{"x":0,"y":0,"width":10,"height":10},"panes":[],"splits":[]}}`)

	_, err := client.Layout("A")
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "tab_id") {
		t.Errorf("the error should name what is missing, got %q", err)
	}
}

func TestResizeSendsTheDividerCallAndReadsChanged(t *testing.T) {
	client, f := clientOf(`{"resize":{"changed":true}}`)

	changed, err := client.Resize("A", geometry.DirectionRight, 0.25)
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	wantCall(t, f, 1, "pane.resize", map[string]any{
		"pane_id": "A", "direction": "right", "amount": 0.25,
	})
	if !changed {
		t.Error("herdr said it changed")
	}
}

// TestResizeRejectsAReplyWithNoResizeObject diverges from the Python, which
// stopped the fit silently. The tab is unchanged either way, so the only
// difference is a line in the plugin log where there was silence.
func TestResizeRejectsAReplyWithNoResizeObject(t *testing.T) {
	client, _ := clientOf(`{}`)

	if _, err := client.Resize("A", geometry.DirectionRight, 0.25); err == nil {
		t.Fatal("want an error")
	}
}

func TestSwapSendsBothPanes(t *testing.T) {
	client, f := clientOf(`{}`)

	if err := client.Swap("A", "B"); err != nil {
		t.Fatalf("Swap: %v", err)
	}
	wantCall(t, f, 1, "pane.swap", map[string]any{"source_pane_id": "A", "target_pane_id": "B"})
}

func TestMoveToNewTabLeavesFocusBehind(t *testing.T) {
	client, f := clientOf(`{}`)

	if err := client.MoveToNewTab("A"); err != nil {
		t.Fatalf("MoveToNewTab: %v", err)
	}
	wantCall(t, f, 1, "pane.move", map[string]any{
		"pane_id":     "A",
		"destination": map[string]any{"type": "new_tab"},
		"focus":       false,
	})
}

func TestMoveBesideSendsTheTabDestinationAndReadsChanged(t *testing.T) {
	client, f := clientOf(`{"move_result":{"changed":true}}`)

	changed, err := client.MoveBeside("A", "w1:tQ", "B", geometry.AxisRight)
	if err != nil {
		t.Fatalf("MoveBeside: %v", err)
	}
	wantCall(t, f, 1, "pane.move", map[string]any{
		"pane_id": "A",
		"destination": map[string]any{
			"type":           "tab",
			"tab_id":         "w1:tQ",
			"target_pane_id": "B",
			"split":          "right",
		},
		"focus": true,
	})
	if !changed {
		t.Error("herdr said it landed")
	}
}

// TestMoveBesideReadsAMissingResultAsRefused keeps the Python's reading: the
// caller announces on false, and a reply that does not say it landed is not a
// reply that says it did.
func TestMoveBesideReadsAMissingResultAsRefused(t *testing.T) {
	client, _ := clientOf(`{}`)

	changed, err := client.MoveBeside("A", "w1:tQ", "B", geometry.AxisRight)
	if err != nil {
		t.Fatalf("MoveBeside: %v", err)
	}
	if changed {
		t.Error("nothing in that reply says the pane landed")
	}
}

func TestNotifySendsTitleBodyAndSound(t *testing.T) {
	client, f := clientOf(`{}`)

	if err := client.Notify("No pane to move left", "This pane is already the leftmost here."); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	wantCall(t, f, 1, "notification.show", map[string]any{
		"title": "No pane to move left",
		"body":  "This pane is already the leftmost here.",
		"sound": "request",
	})
}

func TestFocusedPaneFindsTheFocusedEntry(t *testing.T) {
	client, f := clientOf(`{"panes":[
	  {"pane_id":"A","tab_id":"w1:tQ","focused":false},
	  {"pane_id":"B","tab_id":"w1:tQ","focused":true}
	]}`)

	pane, err := client.FocusedPane()
	if err != nil {
		t.Fatalf("FocusedPane: %v", err)
	}
	wantCall(t, f, 1, "pane.list", map[string]any{})
	if pane != "B" {
		t.Errorf("FocusedPane = %q, want B", pane)
	}
}

// TestFocusedPaneWithNothingFocused checks the no-pane case is a sentinel
// rather than a bare failure: the caller announces on it and exits 0, because
// these run from keypresses where stderr reaches nobody.
func TestFocusedPaneWithNothingFocused(t *testing.T) {
	client, _ := clientOf(`{"panes":[{"pane_id":"A","tab_id":"w1:tQ","focused":false}]}`)

	if _, err := client.FocusedPane(); !errors.Is(err, ErrNoPane) {
		t.Errorf("want ErrNoPane, got %v", err)
	}
}

// TestThisPane walks the three rules in order. herdr names the acting pane
// differently for an event hook than for a plugin action, and a plugin action
// inherits less than either — so the last resort is asking which pane has focus.
func TestThisPane(t *testing.T) {
	t.Run("HERDR_PANE_ID first", func(t *testing.T) {
		client, f := clientOf()
		pane, err := ThisPane(envOf(map[string]string{
			"HERDR_PANE_ID":        "A",
			"HERDR_ACTIVE_PANE_ID": "B",
		}), client)
		if err != nil || pane != "A" {
			t.Errorf("ThisPane = %q, %v; want A", pane, err)
		}
		if len(f.calls) != 0 {
			t.Errorf("and no socket call, got %v", f.methods())
		}
	})

	t.Run("then HERDR_ACTIVE_PANE_ID", func(t *testing.T) {
		client, f := clientOf()
		pane, err := ThisPane(envOf(map[string]string{"HERDR_ACTIVE_PANE_ID": "B"}), client)
		if err != nil || pane != "B" {
			t.Errorf("ThisPane = %q, %v; want B", pane, err)
		}
		if len(f.calls) != 0 {
			t.Errorf("and no socket call, got %v", f.methods())
		}
	})

	t.Run("then whichever pane has focus", func(t *testing.T) {
		client, f := clientOf(`{"panes":[{"pane_id":"C","tab_id":"w1:tQ","focused":true}]}`)
		pane, err := ThisPane(envOf(nil), client)
		if err != nil || pane != "C" {
			t.Errorf("ThisPane = %q, %v; want C", pane, err)
		}
		wantCall(t, f, 1, "pane.list", map[string]any{})
	})

	t.Run("and ErrNoPane when nothing answers", func(t *testing.T) {
		client, _ := clientOf(`{"panes":[]}`)
		if _, err := ThisPane(envOf(nil), client); !errors.Is(err, ErrNoPane) {
			t.Errorf("want ErrNoPane, got %v", err)
		}
	})
}
