package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/macintacos/herdr-reshape/internal/reshape"
)

// installed are the entries copied out of the build: the binary, and the
// manifest herdr resolves the plugin by.
var installed = []string{"bin", "herdr-plugin.toml"}

// newLinkCmd builds the subcommand that registers this build with herdr.
//
// A constructor for the same reason newRootCmd is one: cobra mutates a command
// as it runs it, so a package-level command wired by init() is shared mutable
// state the tests would have to work around.
func newLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link",
		Short: "Register this build with herdr, and install its manifest",
		Long: `A Homebrew cask install and upgrade leave this for you to run. Run it
by hand after every install and upgrade, and after any install that did not come
from the cask.

herdr records a plugin by resolving its manifest and keeping the real directory
that holds it. Point herdr straight at a package manager's prefix and it records
a directory numbered by version — Caskroom/herdr-reshape/<version> for a cask —
which the next upgrade deletes, taking the registration with it.

This builds a directory herdr can keep instead, and copies this release into it:
the binary and the manifest. Every file under it is a copy, so no upgrade can
reach it. That cuts both ways — an upgraded build reaches herdr when you run
this, and not before.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			source, err := buildRoot()
			if err != nil {
				return err
			}
			root := reshape.StableRoot(os.Getenv)

			if err := installBuild(source, root); err != nil {
				return err
			}

			register := exec.Command(herdrBin(), "plugin", "link", root)
			register.Stderr = os.Stderr
			if err := register.Run(); err != nil {
				return err
			}

			// Ignored deliberately, as everywhere else in this package: a write
			// to stdout that fails leaves nothing worth doing about it.
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "installed %s from %s\n", root, source)
			return nil
		},
	}
}

// installBuild puts this release's copy of every entry the plugin needs into
// the directory herdr records, replacing whatever the last one left there.
//
// Copies rather than symlinks into the build, because the build is where the
// package manager staged it and every package manager numbers that directory by
// version. A link into one is dead the moment the version changes; a copy is
// only as stale as the last run of this command.
func installBuild(source, root string) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	// The copy in the plugin root is itself a runnable build, so running its
	// own `link` would hand this the same directory twice — removing each entry
	// immediately before trying to copy from it. Say so rather than emptying
	// the directory herdr runs out of.
	//
	// Both sides resolved, because comparing them as spelled does not catch it:
	// buildRoot reports the source through EvalSymlinks, while root is whatever
	// XDG_DATA_HOME or the home directory spells — and on macOS the same
	// directory reached both ways differs by /var -> /private/var alone.
	sourceReal, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if sourceReal == rootReal {
		return fmt.Errorf("%s holds the copy this command installs; run the build your package manager put on PATH", root)
	}
	// Opened once so the file entry is written through it rather than through a
	// path joined by hand. The tree entries cannot be — os.CopyFS takes a path,
	// not a root — so this is not a containment guarantee over the whole loop.
	owned, openErr := os.OpenRoot(root)
	if openErr != nil {
		return openErr
	}
	defer func() { _ = owned.Close() }()

	for _, name := range installed {
		// Replace outright rather than merge into, so a file a release stopped
		// shipping does not linger here forever. os.CopyFS also refuses to
		// write over a file that is already there.
		//
		// Deliberately not atomic: between the remove and the copy there is no
		// bin/herdr-reshape, so a keypress landing just then does nothing, and a
		// copy that fails part-way leaves nothing to run. Re-running this
		// command is the repair, which is cheaper than staging every entry to
		// rename into place for a window this narrow.
		if err := owned.RemoveAll(name); err != nil {
			return err
		}
		if err := installEntry(owned, filepath.Join(source, name), name); err != nil {
			return err
		}
	}
	return nil
}

// installEntry copies one entry of the build into the directory this plugin
// owns: a file as itself, a directory as a tree.
//
// The two spellings do not agree on permissions. os.CopyFS keeps the source's
// execute bits and forces the rest to 0666 before umask — so a tree arrives as
// 0755/0644 under the usual umask and wider under a permissive one — while the
// file branch reproduces the source's mode exactly. The execute bit is the one
// that has to survive either way: the manifest's actions and events exec
// ./bin/herdr-reshape directly.
func installEntry(owned *os.Root, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.CopyFS(filepath.Join(owned.Name(), name), os.DirFS(src))
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return owned.WriteFile(name, data, info.Mode().Perm())
}

// buildRoot is the directory this build's files sit in.
//
// os.Executable reports the path the binary was invoked by, and Homebrew puts a
// symlink on PATH — the manifest sits beside the target of that link, not beside
// the link. Without EvalSymlinks the root resolves to the Homebrew prefix and
// link fails on a bare `stat .../bin: no such file or directory`.
func buildRoot() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(self)), nil // <root>/bin/herdr-reshape
}

// herdrBin is the herdr to invoke. herdr sets HERDR_BIN_PATH when it runs a
// plugin command, which names the running herdr rather than whichever one PATH
// happens to find — and Homebrew's own hooks run with a PATH the prefix is not
// on at all.
func herdrBin() string {
	if herdr := os.Getenv("HERDR_BIN_PATH"); herdr != "" {
		return herdr
	}
	return "herdr"
}
