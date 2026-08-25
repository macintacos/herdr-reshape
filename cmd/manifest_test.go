package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// binary is how every manifest command line spells this plugin's own
// executable — relative to the plugin root, which is the working directory
// herdr runs actions and events in.
const binary = "./bin/herdr-reshape"

// manifest is the slice of herdr-plugin.toml this file cares about: every place
// it names a command for herdr to run.
type manifest struct {
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

func readManifest(t *testing.T) manifest {
	t.Helper()
	data, err := os.ReadFile("../herdr-plugin.toml")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

// TestManifestCommandsResolve is why this test exists at all: nothing else here
// catches a typo in a manifest command line, and herdr would surface it as a
// keypress that silently does nothing, months from now.
func TestManifestCommandsResolve(t *testing.T) {
	m := readManifest(t)
	if len(m.Actions) == 0 || len(m.Events) == 0 {
		t.Fatal("manifest declares no actions or no events")
	}

	for _, a := range m.Actions {
		t.Run("action/"+a.ID, func(t *testing.T) { assertResolves(t, a.Command) })
	}
	for _, e := range m.Events {
		t.Run("event/"+e.On, func(t *testing.T) { assertResolves(t, e.Command) })
	}
}

// assertResolves checks one manifest command line: it must invoke this plugin's
// own binary, and the rest of it must be a subcommand path cobra can dispatch.
func assertResolves(t *testing.T, argv []string) {
	t.Helper()
	if len(argv) < 2 {
		t.Fatalf("command %q names no subcommand", argv)
	}
	if argv[0] != binary {
		t.Errorf("command %q runs %q, want %q", argv, argv[0], binary)
	}

	found, rest, err := rootCmd.Find(argv[1:])
	if err != nil {
		t.Fatalf("command %q: %v", argv, err)
	}
	if found == rootCmd {
		t.Fatalf("command %q: %q is not a subcommand", argv, argv[1])
	}
	if err := found.ValidateArgs(rest); err != nil {
		t.Errorf("command %q: %v", argv, err)
	}
}

// TestBuildCommandMatchesTask keeps the manifest's [[build]] and the build task
// from drifting apart. herdr runs the first at install time and a linked
// checkout runs the second by hand, so a difference between them means the two
// ways of getting a binary produce different binaries.
func TestBuildCommandMatchesTask(t *testing.T) {
	m := readManifest(t)
	if len(m.Build) != 1 {
		t.Fatalf("manifest declares %d build commands, want 1", len(m.Build))
	}

	build := strings.Join(m.Build[0].Command, " ")
	task, err := os.ReadFile("../.mise/tasks/build")
	if err != nil {
		t.Fatalf("read build task: %v", err)
	}
	if !strings.Contains(string(task), build) {
		t.Errorf(".mise/tasks/build does not run %q", build)
	}
}
