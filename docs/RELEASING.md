# Releasing

By hand, from a laptop — there is no CI. [goreleaser](https://goreleaser.com) builds
darwin and linux binaries for both architectures, publishes the GitHub release with
checksums and notes, and commits the cask to
[macintacos/homebrew-tap](https://github.com/macintacos/homebrew-tap). Its config is
[`.goreleaser.yaml`](../.goreleaser.yaml); goreleaser itself is pinned in
[`mise.toml`](../mise.toml).

The steps below are the by-hand path, and the reference for what each one is for. The
[`/release` skill](../.claude/skills/release/SKILL.md) runs the same sequence with the
version computed by [`svu`](https://github.com/caarlos0/svu) rather than typed, and the
notes drafted from the commit range — which is the usual way to cut one.

The repo has no tags, so `svu current` reports `v0.0.0` and the first release is
`svu minor` → `v0.1.0` — which is already what `herdr-plugin.toml` says.

```sh
$EDITOR herdr-plugin.toml                      # bump `version` to the tag you are cutting
git add herdr-plugin.toml && git commit -m "chore: 0.1.0" && git push
git status --porcelain                         # must be empty, untracked files included
git tag -a v0.1.0 -m v0.1.0 && git push origin v0.1.0
GITHUB_TOKEN=$(mise exec -- gh auth token) mise exec -- goreleaser release \
  --clean --release-notes /tmp/release-notes-v0.1.0.md
```

Stage the manifest by name rather than reaching for `commit -am`, and check the tree
before tagging: goreleaser refuses to run on a tree with **any** uncommitted change,
untracked files included, and it refuses at the last step — after the tag is public. The
notes file is written to `/tmp` for that same reason: drafted in the repo root it would be
one more untracked file, and it would fail the release at exactly that point. Dropping
`--release-notes` is fine too; goreleaser then generates the notes from the commit log
itself.

**Through `mise exec`** because the `before:` hook execs `taplo` and inherits only
goreleaser's own `PATH` — activated mise has it, `mise exec` has it either way.

The bump comes first because a `before:` hook compares `herdr-plugin.toml` against the tag
and fails the release when they disagree. herdr reads the manifest's version rather than
the tag, so a release whose manifest says something else is misreporting itself to the one
thing that looks. `mise run version-check 0.1.0` asks the same question without releasing
anything.

**The token** needs contents write on *both* repositories — `macintacos/herdr-reshape` to
create the release, `macintacos/homebrew-tap` to commit the cask. A classic PAT with
`repo` scope covers it, as does a fine-grained token scoped to the two. goreleaser reads
`GITHUB_TOKEN`, so it lives in the environment for exactly one command and is never
committed; `$(mise exec -- gh auth token)` is enough when `gh` is already authenticated.

**Rehearse first.** `goreleaser release --snapshot --clean --skip=publish` builds all four
targets into `dist/` and renders the cask to `dist/homebrew/Casks/herdr-reshape.rb`
without touching GitHub, and `goreleaser check` validates the config on its own.

**Then check by hand what a cask cannot check for itself.** A formula has `test do`; a
cask has no equivalent, so these are yours:

```sh
brew install --cask macintacos/tap/herdr-reshape
herdr-reshape --version   # the tag you just cut — "dev" means the ldflags stamp broke
herdr-reshape link        # registers the build; nothing else does
"$HOME"/.local/share/herdr-reshape/bin/herdr-reshape --version
```

The two versions agreeing is the check, and here it is `link` that put the second one
there — this cask has no post-install hook that registers for you, which is why the
command is run explicitly above. `$HOME` is spelled out rather than `${XDG_DATA_HOME:-…}`
because a shell with `XDG_DATA_HOME` set writes somewhere else entirely; use whichever
your environment actually names. A disagreement means `link` did not install what you just
downloaded, and a `stat .../bin` error in its output means the archive layout regressed.

And once there is a previous release to come from, the upgrade rather than the install —
`brew upgrade --cask herdr-reshape`, then `herdr-reshape link` again, then the two
versions agreeing. That the plugin root survived the upgrade at all is half of what is
being checked; a versioned Caskroom path would not have. Worth one deliberate check per
release rather than an assumption.

Then drive the plugin itself from that installed build: all five actions
(`user.reshape.move-left`, `move-right`, `move-up`, `move-down`, `fit`) on their
keybindings, and all three events (`pane.created`, `pane.closed`, `pane.exited`) by
splitting and closing panes. This is the check that the release is a working plugin rather
than a working download, and it needs a real herdr session — which is why it lives here
rather than in any automated gate.

Worth one look on a fresh machine as well: Homebrew quarantines what a cask downloads, and
the cask's `postflight` strips the attribute back off. If macOS refuses to run
`herdr-reshape` anyway, `xattr -p com.apple.quarantine "$(which herdr-reshape)"` says
whether the hook ran.
