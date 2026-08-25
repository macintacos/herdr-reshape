// Command herdr-reshape moves the focused herdr pane around its tab, and
// squares the tab up into an even grid.
//
// The distinction that shapes it: a move re-orients a pane against its sibling
// in the split tree, which is not the same as travelling to whatever pane lies
// in that direction — and herdr has no primitive for the former.
package main

import "github.com/macintacos/herdr-reshape/cmd"

// version is stamped in at build time from the tag it was built from. A build
// that nobody stamped is not a release, and says so.
var version = "dev"

func main() {
	cmd.Execute(version)
}
