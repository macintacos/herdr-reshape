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

There are no tags yet, so the first release is `svu minor` → `v0.1.0`, which is already
what `herdr-plugin.toml` says. The numbers below are that first release spelled out; after
it, they are illustrative — the version is whatever `svu` reports at the time, never one
typed from here.

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

**Through `mise exec`** because the `before:` hook's `version-check` execs `taplo` and
inherits only goreleaser's own `PATH` — activated mise has it, `mise exec` has it either
way.

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

**Rehearse first.** `mise exec -- goreleaser release --snapshot --clean --skip=publish`
builds all four targets into `dist/` and renders the cask to
`dist/homebrew/Casks/herdr-reshape.rb` — the same `Casks/` path the tap receives — without
touching GitHub, and `goreleaser check` validates the config on its own.

The rendered cask is worth a look before it is public:

```sh
brew tap-new local/rehearsal --no-git
mkdir -p "$(brew --repository)/Library/Taps/local/homebrew-rehearsal/Casks"
cp dist/homebrew/Casks/herdr-reshape.rb "$(brew --repository)/Library/Taps/local/homebrew-rehearsal/Casks/"
brew audit --strict --cask local/rehearsal/herdr-reshape   # exit 0
brew style local/rehearsal/herdr-reshape                   # see below — not exit 0
brew untap local/rehearsal
```

`audit --strict --cask` is the gate: it must exit 0 and print nothing.

`brew style` is **not** a gate here, and this is the one place the cask is worse than a
formula was. goreleaser's own rendering cannot come out clean, so read it against a known
profile rather than against zero: **9 offences across four cops** — `Cask/StanzaOrder`
(4), `Cask/StanzaGrouping` (2), `Layout/FirstArrayElementIndentation` (2) and
`Style/NumericPredicate` (1). The sibling `Casks/herdr-scratch.rb` in the tap fails with
the identical nine, which is what identifies them as goreleaser's rather than ours. A
**tenth** offence, or one outside those four cops, came from
[`.goreleaser.yaml`](../.goreleaser.yaml) and is yours to fix.

**Then the install checks.** A cask has no `test do` block, so the version assertion a
formula could make on its own is made here by hand instead:

```sh
brew install --cask macintacos/tap/herdr-reshape
herdr-reshape --version   # the tag without its v — "dev" means the ldflags stamp broke
"$(brew --prefix)"/share/herdr-reshape/bin/herdr-reshape --version   # the same version
herdr plugin link "$(brew --prefix)/share/herdr-reshape"
```

Two things can stop that first line, both new with the cask and neither of them a bug in
the release. Homebrew 6 refuses to load a cask from a non-official tap it does not trust —
`brew trust macintacos/tap`, once per machine, is the whole fix. And until step 3 of the
cutover below deletes `Formula/herdr-reshape.rb`, the tap answers to that name twice, so
every `brew` command here needs its `--cask` or `--formula` or it resolves to the wrong
one. An unqualified `brew uninstall herdr-reshape` in that window is the likeliest way to
meet both at once.

The two versions agreeing is the check, and here it is the cask's `postflight` hook that
put the second one there rather than any command you ran. A disagreement means the hook
did not copy what you just downloaded, and a `stat .../bin` error means the archive layout
regressed. That the first one runs at all is the other half: it is the staged binary
through the PATH symlink, so a launch failure means the quarantine strip did not fire.

And once there is a previous release to come from, the upgrade rather than the install —
`brew upgrade --cask herdr-reshape`, then the two versions agreeing again
**with no `link` run at all**. That is the whole point of the `postflight` copy: the
registration made above survives every upgrade because the path it recorded never changes.
Worth one deliberate check per release rather than an assumption.

Then drive the plugin itself from that installed build: all five actions
(`user.reshape.move-left`, `move-right`, `move-up`, `move-down`, `fit`) on their
keybindings, and all three events (`pane.created`, `pane.closed`, `pane.exited`) by
splitting and closing panes. This is the check that the release is a working plugin rather
than a working download, and it needs a real herdr session — which is why it lives here
rather than in any automated gate.

## The one-time formula cutover

Not yet done, and it cannot be until a cask release exists. `brew upgrade` does not
convert a formula install into a cask install, and the tap must not lose the formula
before there is something to replace it with — so the order is fixed:

1. Cut the first release from this config. goreleaser commits `Casks/herdr-reshape.rb` to
   the tap; `Formula/herdr-reshape.rb` is still sitting there, now pointing at an older
   release.
2. Tell existing users to remove the formula before installing the cask. The two cannot
   coexist: both put `herdr-reshape` on `PATH`, and while both are in the tap an
   unqualified `brew install macintacos/tap/herdr-reshape` resolves to the formula — so
   `--cask` is not optional until step 3.

   ```sh
   brew uninstall herdr-reshape
   brew install --cask macintacos/tap/herdr-reshape
   ```

   No `herdr plugin link` and no `unlink`: both packages maintain the same plugin root at
   `$(brew --prefix)/share/herdr-reshape`, so the registration made against the formula
   still points at what the cask's `postflight` now refreshes. This is the whole reason
   that path was kept rather than moved.
3. Delete `Formula/herdr-reshape.rb` from `macintacos/homebrew-tap`. This is one of the
   two deliberate hand-edits of the tap the
   [`/release` skill](../.claude/skills/release/SKILL.md)'s "When NOT to Use" allows for.

Announcing before deleting is the point of that order: between the delete and the
migration, a formula user running `brew upgrade` hits a formula the tap no longer has.
Leaving it in place until people have moved costs nothing.

Delete this section once the cutover has happened.
