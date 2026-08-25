// Package cmd wires the herdr-reshape subcommands to the work they do.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// errNotImplemented is what every subcommand below returns. The manifest
// declares all five actions and all three events so that the plugin registers
// and binds as its finished self, and internal/geometry holds the arithmetic
// behind them — but nothing yet talks to herdr's socket, so there is no layout
// to run it over.
//
// One sentinel rather than a message per command: they all fail for the same
// reason, and there is nothing a caller could do differently for any of them.
var errNotImplemented = errors.New("not implemented yet — no socket client wires the geometry up")

// newRootCmd builds a command tree for one run, stamped with the version it was
// built as.
//
// A constructor rather than package-level vars wired by init(): cobra mutates a
// command as it runs it (parsed flags, output writers, the parent link
// AddCommand sets), so a process-wide tree is shared mutable state. The tests
// below build their own, which is what keeps them independent of each other.
func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "herdr-reshape",
		Short: "Move the focused pane, and best-fit a tab into an even grid",
		Long: `herdr-reshape adds two things herdr has no word for: re-orienting a pane
against its sibling in the split tree, and squaring a tab's panes up into an
even grid.

Each subcommand is invoked by a different part of herdr: move and fit by
keybindings, created and closed by pane events.`,
		// Both, not just SilenceUsage: cobra prints its own `Error: …` line
		// independently of the usage dump, and Execute below already reports the
		// failure in this command's own voice.
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	// Just the number: this gets read by scripts far more often than it gets
	// read as a sentence.
	root.SetVersionTemplate("{{.Version}}\n")

	stub := func(cmd *cobra.Command, args []string) error { return errNotImplemented }

	move := &cobra.Command{
		Use:   "move <left|right|up|down>",
		Short: "Re-orient the focused pane against its sibling",
		// The four a move can take. They name the pane's new position relative
		// to its sibling, not a pane to travel to.
		ValidArgs: []string{"left", "right", "up", "down"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE:      stub,
	}
	fit := &cobra.Command{
		Use:   "fit",
		Short: "Square this tab's panes up into an even grid",
		Args:  cobra.NoArgs,
		RunE:  stub,
	}
	created := &cobra.Command{
		Use:   "created",
		Short: "Fit the tab a pane was just split off in (pane.created hook)",
		Args:  cobra.NoArgs,
		RunE:  stub,
	}
	closed := &cobra.Command{
		Use:   "closed",
		Short: "Fit the tab a pane just left (pane.closed and pane.exited hooks)",
		Args:  cobra.NoArgs,
		RunE:  stub,
	}

	root.AddCommand(move, fit, created, closed)
	return root
}

// Execute runs the CLI with the version it was built as, reporting a failure as
// one line on stderr rather than cobra's default usage dump — these run from
// keypresses and event hooks, where nobody is reading a usage screen.
func Execute(version string) {
	if err := newRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "herdr-reshape:", err)
		os.Exit(1)
	}
}
