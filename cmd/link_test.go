package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stageBuild writes a build tree of the shape `link` copies out of, into dir:
// the binary under bin/, the manifest beside it.
//
// Every file carries the marker, so a test can tell one release's copy from the
// next without caring what is actually in them.
func stageBuild(t *testing.T, dir, marker string) {
	t.Helper()
	for path, mode := range map[string]os.FileMode{
		filepath.Join("bin", "herdr-reshape"): 0o755,
		"herdr-plugin.toml":                   0o644,
	} {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(marker), mode); err != nil {
			t.Fatal(err)
		}
	}
}

// readInstalled is the installed copy of one entry, as contents and mode.
//
// Lstat rather than Stat, and a real-file assertion before either: os.Stat
// reports the target's mode through a symlink, so a check built on it would
// pass just as happily against a link into the build this command exists to
// copy out of.
func readInstalled(t *testing.T, path string) (string, os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s is %v, want a real file rather than something that resolves to one", path, info.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), info.Mode().Perm()
}

func TestInstallBuildKeepsTheBinaryExecutable(t *testing.T) {
	// The manifest's actions and events exec ./bin/herdr-reshape directly, so a
	// copy that lost the execute bit is five bindings and three hooks that do
	// nothing.
	source := filepath.Join(t.TempDir(), "build")
	stageBuild(t, source, "0.1.0")
	root := filepath.Join(t.TempDir(), "herdr-reshape")

	if err := installBuild(source, root); err != nil {
		t.Fatal(err)
	}

	if _, mode := readInstalled(t, filepath.Join(root, "bin", "herdr-reshape")); mode&0o111 == 0 {
		t.Errorf("installed binary mode = %v, want the execute bits set", mode)
	}
}

func TestInstallBuildRefusesARootThatIsNotAbsolute(t *testing.T) {
	// StableRoot joins onto whatever HOME says, so an empty HOME hands this a
	// relative path — and MkdirAll would then write a plugin tree under whatever
	// the working directory happened to be and register that with herdr. The
	// only signal of it would be a plugin herdr silently cannot find later.
	source := filepath.Join(t.TempDir(), "build")
	stageBuild(t, source, "0.1.0")

	if err := installBuild(source, filepath.Join(".local", "share", "herdr-reshape")); err == nil {
		t.Error("installBuild with a relative root = nil, want an error rather than a tree under the cwd")
	}
}

func TestLinkRegistersTheRootRatherThanTheBuild(t *testing.T) {
	// The whole point of the command: herdr must be pointed at the copy, not at
	// the staging directory the copy came from. Passing source here would look
	// correct on install and only surface one `brew upgrade` later, as a plugin
	// herdr no longer has a directory for.
	source := filepath.Join(t.TempDir(), "build")
	stageBuild(t, source, "0.1.0")
	root := filepath.Join(t.TempDir(), "herdr-reshape")
	herdr, argv := stubHerdr(t)

	var out strings.Builder
	if err := runLink(source, root, herdr, &out); err != nil {
		t.Fatal(err)
	}

	if got := readArgv(t, argv); got != "plugin link "+root {
		t.Errorf("herdr was called with %q, want %q", got, "plugin link "+root)
	}
	if want := "installed " + root + " from " + source + "\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// stubHerdr writes an executable standing in for herdr, which records the
// arguments it was called with, and returns both paths.
func stubHerdr(t *testing.T) (herdr, argv string) {
	t.Helper()
	dir := t.TempDir()
	herdr, argv = filepath.Join(dir, "herdr"), filepath.Join(dir, "argv")
	script := "#!/bin/sh\nprintf '%s' \"$*\" > " + argv + "\n"
	if err := os.WriteFile(herdr, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return herdr, argv
}

func readArgv(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInstallBuildRefusesToInstallOverItself(t *testing.T) {
	// The installed copy is a runnable build, so `link` can be invoked from it.
	// Left unguarded that removes each entry just before copying from it, which
	// empties the directory herdr runs out of.
	root := filepath.Join(t.TempDir(), "herdr-reshape")
	stageBuild(t, root, "1.0.0")

	// Spelled two ways as well as one. buildRoot resolves the source through
	// EvalSymlinks while root keeps whatever XDG_DATA_HOME said, so a guard that
	// compares them as written misses the real case entirely — on macOS the two
	// spellings of a temp directory differ by /var -> /private/var.
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}

	for _, source := range []string{root, alias} {
		if err := installBuild(source, root); err == nil {
			t.Errorf("installBuild(%q, root) = nil, want an error rather than a wiped plugin root", source)
		}
		if got, _ := readInstalled(t, filepath.Join(root, "bin", "herdr-reshape")); got != "1.0.0" {
			t.Fatalf("the binary was disturbed by installBuild(%q, root): %q, want %q", source, got, "1.0.0")
		}
	}
}

func TestInstallBuildSurvivesTheStagingDirectoryBeingReplaced(t *testing.T) {
	// The upgrade this command exists to withstand. A package manager stages at
	// a path numbered by version — Cellar/<name>/<version> for Homebrew — and
	// deletes that directory on upgrade, so anything the plugin root still
	// points into goes with it. Owning the copy is what makes the root
	// independent of the staging path.
	cellar := filepath.Join(t.TempDir(), "Cellar", "herdr-reshape")
	root := filepath.Join(t.TempDir(), "herdr-reshape")

	staged := filepath.Join(cellar, "0.1.0")
	stageBuild(t, staged, "0.1.0")
	if err := installBuild(staged, root); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(staged); err != nil {
		t.Fatal(err)
	}
	stageBuild(t, filepath.Join(cellar, "0.2.0"), "0.2.0")

	for _, entry := range []string{filepath.Join("bin", "herdr-reshape"), "herdr-plugin.toml"} {
		if got, _ := readInstalled(t, filepath.Join(root, entry)); got != "0.1.0" {
			t.Errorf("%s = %q after the staging directory was replaced, want %q", entry, got, "0.1.0")
		}
	}

	// And the re-link picks the new release up, which is what makes the copy a
	// snapshot rather than a one-time freeze.
	if err := installBuild(filepath.Join(cellar, "0.2.0"), root); err != nil {
		t.Fatal(err)
	}
	if got, _ := readInstalled(t, filepath.Join(root, "bin", "herdr-reshape")); got != "0.2.0" {
		t.Errorf("binary = %q after re-linking, want %q", got, "0.2.0")
	}
}

func TestInstallBuildDropsWhatTheNextReleaseNoLongerShips(t *testing.T) {
	// Each entry is replaced outright rather than merged into, so a file a
	// release stopped shipping does not linger in the root forever — herdr
	// would keep finding it, and nothing would ever remove it.
	source := filepath.Join(t.TempDir(), "build")
	stageBuild(t, source, "0.1.0")
	root := filepath.Join(t.TempDir(), "herdr-reshape")
	if err := installBuild(source, root); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(root, "bin", "herdr-reshape-helper")
	if err := os.WriteFile(gone, []byte("0.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := installBuild(source, root); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Lstat(gone); !os.IsNotExist(err) {
		t.Errorf("bin/herdr-reshape-helper survived the release that dropped it: %v", err)
	}
}
