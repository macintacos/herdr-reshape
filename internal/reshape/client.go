// Package reshape is the half of this plugin that talks to herdr: one Unix
// socket, the seven calls this plugin makes over it, and the four operations
// that drive [github.com/macintacos/herdr-reshape/internal/geometry] through
// them.
//
// internal/geometry only computes; this package only talks; cmd is glue.
// What lives here is everything that
// depends on the environment rather than on arithmetic — and that is where the
// surprises are, every one of them measured against herdr 0.8.2 rather than
// inferred:
//
//   - herdr names the acting pane differently depending on how a command was
//     reached, and a plugin action inherits less than an event hook does. See
//     [ThisPane], and the two operations in ops.go that deliberately do not use
//     it.
//   - pane.move refuses every within-tab target, so re-orienting a pane inside
//     its own tab is a round trip out to a temporary tab and back. See
//     [Client.MoveToNewTab] and [Client.MoveBeside].
//   - A failure and an announcement are different channels. A socket error
//     returns an error, which cmd.Execute prints on stderr — the plugin log,
//     for an event hook. A user-visible outcome goes through
//     [Client.Notify] and exits 0, because a keypress's stderr reaches nobody.
package reshape

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"time"

	"github.com/macintacos/herdr-reshape/internal/geometry"
)

// Timeout bounds one round trip, in both directions together. These run from
// keypresses and from event hooks that fire once per pane in a burst, so a
// server that has gone quiet has to stop being this plugin's problem quickly.
const Timeout = 5 * time.Second

// ErrNoPane is returned when nothing herdr said names the pane a command is
// acting on — no environment variable, and no focused pane either.
//
// A sentinel rather than a plain failure because the caller does not report it
// as one: it is a user-visible outcome, so it is announced and the command
// exits 0.
var ErrNoPane = errors.New("herdr did not say which pane this is")

// Client is a connection to herdr's API socket.
//
// Its transport is a function field rather than an interface: there is one real
// implementation and there will only ever be one, and a fake standing in for it
// sees the actual method name and the actual params map. A param name typo is
// exactly the failure herdr surfaces months later as a chord that silently does
// nothing, and an interface fake would sail straight past it.
type Client struct {
	do func(method string, params map[string]any) (json.RawMessage, error)
}

// NewClient builds a client over the socket at path, opening nothing — a
// connection is made per call. So a command tree can be built, and its manifest
// checked, without a herdr running.
func NewClient(path string, timeout time.Duration) *Client {
	return &Client{
		do: func(method string, params map[string]any) (json.RawMessage, error) {
			return roundTrip(path, timeout, method, params)
		},
	}
}

// SocketPath resolves herdr's API socket from the environment: an explicit
// HERDR_SOCKET_PATH, else herdr.sock under the XDG config directory.
//
// getenv is a parameter so that the entry point owns the environment read and
// everything below it can be tested without one.
func SocketPath(getenv func(string) string) string {
	if path := getenv("HERDR_SOCKET_PATH"); path != "" {
		return path
	}
	config := getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(getenv("HOME"), ".config")
	}
	return filepath.Join(config, "herdr", "herdr.sock")
}

// roundTrip sends one request and returns the `result` object of the reply.
//
// One request, one connection, one line each way — the shape herdr's socket
// speaks. The envelope is decoded as far as `result` and no further: herdr's
// error shape is not documented, so no error schema is invented here.
func roundTrip(path string, timeout time.Duration, method string, params map[string]any) (json.RawMessage, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("%s: dial %s: %w", method, path, err)
	}
	defer func() { _ = conn.Close() }()
	// One deadline covering both directions, so a server that accepts and goes
	// quiet cannot hold an event hook open.
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}

	request, err := json.Marshal(map[string]any{
		"id":     "reshape:" + method,
		"method": method,
		"params": params,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: encode request: %w", method, err)
	}
	if _, err := conn.Write(append(request, '\n')); err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}

	// A server that writes a complete reply and closes returns the bytes
	// alongside io.EOF; that is a reply, not a failure. Anything else — and an
	// EOF with nothing before it — is.
	reply, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil && (!errors.Is(err, io.EOF) || len(reply) == 0) {
		return nil, fmt.Errorf("%s: read reply: %w", method, err)
	}

	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(reply, &envelope); err != nil {
		return nil, fmt.Errorf("%s: decode reply %s: %w", method, reply, err)
	}
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil, fmt.Errorf("%s: reply carries no result: %s", method, reply)
	}
	return envelope.Result, nil
}

// Layout asks herdr for the arrangement of the tab pane sits in.
//
// The tab id is checked here rather than in [geometry.Layout] because this is
// the only place in the program holding the bytes, and a layout without one
// would strand a pane in the temporary tab a move passes it through.
func (c *Client) Layout(pane geometry.PaneID) (geometry.Layout, error) {
	result, err := c.do("pane.layout", map[string]any{"pane_id": string(pane)})
	if err != nil {
		return geometry.Layout{}, err
	}
	var payload struct {
		Layout geometry.Layout `json:"layout"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return geometry.Layout{}, fmt.Errorf("pane.layout: decode %s: %w", result, err)
	}
	if payload.Layout.TabID == "" {
		return geometry.Layout{}, fmt.Errorf("pane.layout: reply carries no layout, or none with a tab_id: %s", result)
	}
	return payload.Layout, nil
}

// Resize drives the divider on the named side of pane, reporting whether herdr
// moved it.
//
// The sign lives in direction, never in amount — see [geometry.FitCalls] for
// why, and for which pane a given divider has to be aimed at.
func (c *Client) Resize(pane geometry.PaneID, direction geometry.Direction, amount float64) (bool, error) {
	result, err := c.do("pane.resize", map[string]any{
		"pane_id":   string(pane),
		"direction": string(direction),
		"amount":    amount,
	})
	if err != nil {
		return false, err
	}
	// A pointer so a reply with no resize object at all is distinguishable from
	// one reporting no change. The former is an error rather than a silent stop
	// to the fit: the tab is unchanged either way, so the only difference is a
	// line in the plugin log where there would have been silence.
	var payload struct {
		Resize *struct {
			Changed bool `json:"changed"`
		} `json:"resize"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return false, fmt.Errorf("pane.resize: decode %s: %w", result, err)
	}
	if payload.Resize == nil {
		return false, fmt.Errorf("pane.resize: reply carries no resize: %s", result)
	}
	return payload.Resize.Changed, nil
}

// Swap trades two panes' places inside their tab — a pure reorder, with no
// round trip and so no tab-bar flicker.
func (c *Client) Swap(source, target geometry.PaneID) error {
	_, err := c.do("pane.swap", map[string]any{
		"source_pane_id": string(source),
		"target_pane_id": string(target),
	})
	return err
}

// MoveToNewTab sends pane out to a tab of its own, unfocused.
//
// The outward half of the round trip [geometry.MovePlan] calls for: herdr
// refuses every within-tab move target, so leaving is the only way back in on a
// different split. Whatever is running in the pane survives both hops, and the
// temporary tab removes itself the moment it empties.
func (c *Client) MoveToNewTab(pane geometry.PaneID) error {
	_, err := c.do("pane.move", map[string]any{
		"pane_id":     string(pane),
		"destination": map[string]any{"type": "new_tab"},
		"focus":       false,
	})
	return err
}

// MoveBeside brings pane back into tab beside target, split along split,
// reporting whether herdr applied it.
//
// A false here is the case worth reporting to the user: the pane is alive in a
// temporary tab, with whatever was running still running, but it is somewhere
// they were not looking and nothing else will go and find it.
func (c *Client) MoveBeside(pane geometry.PaneID, tab geometry.TabID, target geometry.PaneID, split geometry.Axis) (bool, error) {
	result, err := c.do("pane.move", map[string]any{
		"pane_id": string(pane),
		"destination": map[string]any{
			"type":           "tab",
			"tab_id":         string(tab),
			"target_pane_id": string(target),
			"split":          string(split),
		},
		"focus": true,
	})
	if err != nil {
		return false, err
	}
	// Absent reads as false rather than being rejected the way [Client.Resize]
	// rejects a missing resize object, because false here is announced and
	// false there is not: a reply that does not say the pane landed is not a
	// reply that says it did.
	var payload struct {
		MoveResult struct {
			Changed bool `json:"changed"`
		} `json:"move_result"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return false, fmt.Errorf("pane.move: decode %s: %w", result, err)
	}
	return payload.MoveResult.Changed, nil
}

// Notify says what happened, for a keypress whose effect is otherwise
// invisible.
//
// A pane already on the side it was sent leaves the tab untouched, which reads
// exactly like a broken keybinding. There is no scrollbar here to show which
// end of a row you are at, so saying so beats a key that silently does nothing.
func (c *Client) Notify(title, body string) error {
	_, err := c.do("notification.show", map[string]any{
		"title": title,
		"body":  body,
		"sound": "request",
	})
	return err
}

// FocusedPane asks herdr which pane has focus, for an entry point handed no id.
//
// A reply where nothing is focused returns [ErrNoPane] rather than a failure:
// callers announce or fall silent on it, they do not report it.
func (c *Client) FocusedPane() (geometry.PaneID, error) {
	result, err := c.do("pane.list", map[string]any{})
	if err != nil {
		return "", err
	}
	var payload struct {
		Panes []geometry.PaneEntry `json:"panes"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return "", fmt.Errorf("pane.list: decode %s: %w", result, err)
	}
	for _, entry := range payload.Panes {
		if entry.Focused {
			return entry.PaneID, nil
		}
	}
	return "", ErrNoPane
}

// ThisPane identifies the pane a command is acting on, returning [ErrNoPane]
// when nothing does.
//
// herdr names it differently depending on how the command was reached — an
// event hook, a keybinding — and a plugin action inherits less than either, so
// the last resort is simply asking which pane has focus.
//
// This chain is for move and fit only. The two event hooks in ops.go each
// resolve their pane a different way, for reasons measured on 0.8.2 and written
// down at their call sites.
func ThisPane(getenv func(string) string, c *Client) (geometry.PaneID, error) {
	for _, key := range []string{"HERDR_PANE_ID", "HERDR_ACTIVE_PANE_ID"} {
		if pane := getenv(key); pane != "" {
			return geometry.PaneID(pane), nil
		}
	}
	return c.FocusedPane()
}
