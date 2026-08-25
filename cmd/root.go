// Package cmd wires the herdr-reshape subcommands to the work they do.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
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
}

// Execute runs the CLI with the version it was built as, reporting a failure as
// one line on stderr rather than cobra's default usage dump — these run from
// keypresses and event hooks, where nobody is reading a usage screen.
func Execute(version string) {
	rootCmd.Version = version
	// Just the number: this gets read by scripts far more often than it gets
	// read as a sentence.
	rootCmd.SetVersionTemplate("{{.Version}}\n")
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "herdr-reshape:", err)
		os.Exit(1)
	}
}

// errNotImplemented is what every subcommand below returns. The manifest
// declares all five actions and all three events so that the plugin registers
// and binds as its finished self, but the grid arithmetic behind them is being
// ported from the Python original in EXC-1180 and is not here yet.
//
// One sentinel rather than a message per command: they all fail for the same
// reason, and there is nothing a caller could do differently for any of them.
var errNotImplemented = errors.New("not implemented yet — the reshape logic arrives in EXC-1180")

// directions are the four a move can take, and the only arguments `move`
// accepts. They name the pane's new position relative to its sibling, not a
// pane to travel to.
var directions = []string{"left", "right", "up", "down"}

var moveCmd = &cobra.Command{
	Use:       "move <left|right|up|down>",
	Short:     "Re-orient the focused pane against its sibling",
	ValidArgs: directions,
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
}

var fitCmd = &cobra.Command{
	Use:   "fit",
	Short: "Square this tab's panes up into an even grid",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
}

var createdCmd = &cobra.Command{
	Use:   "created",
	Short: "Fit the tab a pane was just split off in (pane.created hook)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
}

var closedCmd = &cobra.Command{
	Use:   "closed",
	Short: "Fit the tab a pane just left (pane.closed and pane.exited hooks)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
}

func init() {
	rootCmd.AddCommand(moveCmd, fitCmd, createdCmd, closedCmd)
}
