// Command herdr-reshape moves the focused herdr pane around its tab, and
// squares the tab up into an even grid.
//
// The distinction that shapes it: a move re-orients a pane against its sibling
// in the split tree, which is not the same as travelling to whatever pane lies
// in that direction — and herdr has no primitive for the former.
package main

import "github.com/macintacos/herdr-reshape/cmd"

// version is what --version reports. There is no release pipeline yet, so every
// build reports "dev"; a goreleaser build will stamp the tag through
// `-ldflags -X main.version=`.
var version = "dev"

func main() {
	cmd.Execute(version)
}
