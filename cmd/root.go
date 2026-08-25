// Package cmd wires the herdr-reshape subcommands to the work they do.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/macintacos/herdr-reshape/internal/geometry"
	"github.com/macintacos/herdr-reshape/internal/reshape"
)

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

	// Built once, and opening nothing — a connection is made per call. So the
	// manifest test can build trees without a herdr running.
	client := reshape.NewClient(reshape.SocketPath(os.Getenv), reshape.Timeout)

	move := &cobra.Command{
		Use:   "move <left|right|up|down>",
		Short: "Re-orient the focused pane against its sibling",
		// The four a move can take. They name the pane's new position relative
		// to its sibling, not a pane to travel to.
		ValidArgs: []string{"left", "right", "up", "down"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(_ *cobra.Command, args []string) error {
			pane, ok, err := actingPane(client, "Cannot move a pane")
			if !ok {
				return err
			}
			_, err = client.Move(pane, geometry.Direction(args[0]))
			return err
		},
	}
	fit := &cobra.Command{
		Use:   "fit",
		Short: "Square this tab's panes up into an even grid",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			pane, ok, err := actingPane(client, "Cannot fit this tab")
			if !ok {
				return err
			}
			_, err = client.Fit(pane)
			return err
		},
	}
	created := &cobra.Command{
		Use:   "created",
		Short: "Fit the tab a pane was just split off in (pane.created hook)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			// No fallback chain here, deliberately. Measured on 0.8.2: the
			// pane.created hook's HERDR_PANE_ID is the pane that was just
			// created, even when focus is in another tab entirely. Falling back
			// to the focused pane would gate on — and fit — some other tab.
			pane := os.Getenv("HERDR_PANE_ID")
			if pane == "" {
				return nil
			}
			_, err := client.Created(geometry.PaneID(pane))
			return err
		},
	}
	closed := &cobra.Command{
		Use:   "closed",
		Short: "Fit the tab a pane just left (pane.closed and pane.exited hooks)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			// The pane named by the event is no use and is deliberately not
			// read: measured on 0.8.2, pane.closed and pane.exited both fire
			// after the split has collapsed, so pane.layout answers
			// pane_not_found for it. Neither event carries a tab id either, so
			// the only handle left on the right tab is wherever focus went —
			// which, measured, is a surviving pane of the same tab whenever the
			// pane that closed was in the focused one.
			//
			// A pane closing in a *background* tab therefore looks at the
			// focused tab instead. That is left alone rather than worked
			// around, because the gate already makes it harmless: a focused tab
			// that is even is one a fit does nothing to, and one that is off the
			// grid by hand is one the gate declines.
			pane, err := client.FocusedPane()
			if errors.Is(err, reshape.ErrNoPane) {
				return nil
			}
			if err != nil {
				return err
			}
			_, err = client.Closed(pane)
			return err
		},
	}

	root.AddCommand(move, fit, created, closed)
	return root
}

// actingPane resolves the pane a keybinding is acting on, reporting whether
// there is one to go on with.
//
// herdr naming no pane is a user-visible outcome rather than a failure: move
// and fit run from keypresses, where stderr reaches nobody. So it is announced
// under title and reported as not-ok, and the command exits 0 having said so.
func actingPane(client *reshape.Client, title string) (geometry.PaneID, bool, error) {
	pane, err := reshape.ThisPane(os.Getenv, client)
	if errors.Is(err, reshape.ErrNoPane) {
		return "", false, client.Notify(title, "herdr did not say which pane this is.")
	}
	if err != nil {
		return "", false, err
	}
	return pane, true, nil
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
