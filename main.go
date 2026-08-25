// Command herdr-reshape moves the focused herdr pane around its tab, and
// squares the tab up into an even grid.
//
// The distinction that shapes it: a move re-orients a pane against its sibling
// in the split tree, which is not the same as travelling to whatever pane lies
// in that direction — and herdr has no primitive for the former.
package main

import "github.com/macintacos/herdr-reshape/cmd"

// version is what --version reports. A goreleaser build stamps the release over
// it through `-ldflags -X main.version=`, so "dev" is what a local `go build`
// reports and what a stamp that broke would leave behind.
var version = "dev"

func main() {
	cmd.Execute(version)
}
