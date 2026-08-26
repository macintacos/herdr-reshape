package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// testManifest is the slice of herdr-plugin.toml this file cares about: every
// place it names a command for herdr to run. Prefixed so the name stays out of
// the way of a real manifest type, if one ever lands here.
type testManifest struct {
	Build []struct {
		Command []string `toml:"command"`
	} `toml:"build"`
	Events []struct {
		On      string   `toml:"on"`
		Command []string `toml:"command"`
	} `toml:"events"`
	Actions []struct {
		ID      string   `toml:"id"`
		Command []string `toml:"command"`
	} `toml:"actions"`
}

func readTestManifest(t *testing.T) testManifest {
	t.Helper()
	data, err := os.ReadFile("../herdr-plugin.toml")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m testManifest
	if err := toml.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

// builtBinary is the path [[build]] writes the executable to, spelled the way an
// action or event command line has to invoke it — relative to the plugin root,
// which is the working directory herdr runs them in.
//
// Derived rather than hardcoded: a literal here would be a third copy of the
// path, and renaming the build output would leave every action pointing at a
// binary that no longer exists while this test still passed.
func builtBinary(t *testing.T, build []string) string {
	t.Helper()
	for i, arg := range build {
		if arg == "-o" && i+1 < len(build) {
			return "./" + strings.TrimPrefix(build[i+1], "./")
		}
	}
	t.Fatalf("build command %q writes no -o output", build)
	return ""
}

// TestManifestCommandsResolve is why this test exists at all: nothing else here
// catches a typo in a manifest command line, and herdr would surface it as a
// keypress that silently does nothing, months from now.
func TestManifestCommandsResolve(t *testing.T) {
	m := readTestManifest(t)
	if len(m.Build) != 1 {
		t.Fatalf("manifest declares %d build commands, want 1", len(m.Build))
	}
	binary := builtBinary(t, m.Build[0].Command)
	root := newRootCmd("test")

	for _, a := range m.Actions {
		t.Run("action/"+a.ID, func(t *testing.T) {
			assertResolves(t, newRootCmd("test"), binary, a.Command)
		})
	}
	for _, e := range m.Events {
		t.Run("event/"+e.On, func(t *testing.T) {
			assertResolves(t, newRootCmd("test"), binary, e.Command)
		})
	}

	assertEveryCommandDeclared(t, root, m)
}

// assertResolves checks one manifest command line: it must invoke this plugin's
// own binary, and the rest of it must be a subcommand path cobra can dispatch.
func assertResolves(t *testing.T, root *cobra.Command, binary string, argv []string) {
	t.Helper()
	if len(argv) < 2 {
		t.Fatalf("command %q names no subcommand", argv)
	}
	if argv[0] != binary {
		t.Errorf("command %q runs %q, want %q", argv, argv[0], binary)
	}

	found, rest, err := root.Find(argv[1:])
	if err != nil {
		t.Fatalf("command %q: %v", argv, err)
	}
	if found == root {
		t.Fatalf("command %q: %q is not a subcommand", argv, argv[1])
	}
	if err := found.ValidateArgs(rest); err != nil {
		t.Errorf("command %q: %v", argv, err)
	}
}

// assertEveryCommandDeclared is the other direction, and it is the one a
// deleted manifest entry hides in: every command the binary registers must be
// reachable from the manifest, or it is a capability nothing can invoke.
func assertEveryCommandDeclared(t *testing.T, root *cobra.Command, m testManifest) {
	t.Helper()
	declared := map[string]bool{}
	declaredArgs := map[string]bool{}
	note := func(argv []string) {
		if len(argv) > 1 {
			declared[argv[1]] = true
		}
		if len(argv) > 2 {
			declaredArgs[argv[1]+" "+argv[2]] = true
		}
	}
	for _, a := range m.Actions {
		note(a.Command)
	}
	for _, e := range m.Events {
		note(e.Command)
	}

	for _, c := range root.Commands() {
		// cobra adds help and completion itself; the manifest has no reason to
		// name them. link is different in kind from the other four: the manifest
		// describes what herdr can invoke, and link is what installs the
		// manifest — a human runs it after every install and upgrade, herdr
		// never does, and declaring it as an action would offer a keybinding for
		// installing the plugin.
		if c.Name() == "help" || c.Name() == "completion" || c.Name() == "link" {
			continue
		}
		if !declared[c.Name()] {
			t.Errorf("command %q is registered but no manifest action or event invokes it", c.Name())
			continue
		}
		// Naming the command is not enough for one that takes a constrained
		// argument: `move` stays declared when a single direction is dropped,
		// and a dropped direction is a whole binding the user silently loses.
		for _, valid := range c.ValidArgs {
			if !declaredArgs[c.Name()+" "+valid] {
				t.Errorf("%q accepts %q but no manifest action or event invokes it", c.Name(), valid)
			}
		}
	}
}

// TestBuildCommandMatchesTask keeps the manifest's [[build]] and the build task
// from drifting apart. herdr runs the first at install time and a linked
// checkout runs the second by hand, so a difference between them means the two
// ways of getting a binary produce different binaries.
func TestBuildCommandMatchesTask(t *testing.T) {
	m := readTestManifest(t)
	if len(m.Build) != 1 {
		t.Fatalf("manifest declares %d build commands, want 1", len(m.Build))
	}

	build := strings.Join(m.Build[0].Command, " ")
	task, err := os.ReadFile("../.mise/tasks/build")
	if err != nil {
		t.Fatalf("read build task: %v", err)
	}

	// Whole lines, not a substring of the file: the task's own comment talks
	// about this command, so a Contains check passes on the prose alone.
	for _, line := range strings.Split(string(task), "\n") {
		if strings.TrimSpace(line) == build {
			return
		}
	}
	t.Errorf(".mise/tasks/build does not run %q", build)
}
