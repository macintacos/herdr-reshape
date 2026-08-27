# Releasing

By hand, from a laptop — there is no CI. [goreleaser](https://goreleaser.com) builds
darwin and linux binaries for both architectures, publishes the GitHub release with
checksums and notes, and commits the formula to
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
create the release, `macintacos/homebrew-tap` to commit the formula. A classic PAT with
`repo` scope covers it, as does a fine-grained token scoped to the two. goreleaser reads
`GITHUB_TOKEN`, so it lives in the environment for exactly one command and is never
committed; `$(mise exec -- gh auth token)` is enough when `gh` is already authenticated.

**Rehearse first.** `mise exec -- goreleaser release --snapshot --clean --skip=publish`
builds all four targets into `dist/` and renders the formula to
`dist/homebrew/Formula/herdr-reshape.rb` — the same `Formula/` path the tap receives —
without touching GitHub, and `mise run goreleaser-check` validates the config on its own.
Use the task rather than `goreleaser check` directly: `brews` is deprecated, so `check`
alone exits non-zero on a config that is otherwise fine, and the task is what tolerates
that one deprecation and nothing else.

The rendered formula is worth a look before it is public:

```sh
brew tap-new local/rehearsal --no-git
cp dist/homebrew/Formula/herdr-reshape.rb "$(brew --repository)/Library/Taps/local/homebrew-rehearsal/Formula/"
brew style local/rehearsal/herdr-reshape        # exit 0
brew audit --strict local/rehearsal/herdr-reshape   # exit 0
brew untap local/rehearsal
```

Both must be clean. `brew style` is where the two cops that matter here land — it is what
rejects `rm_rf` in `post_install`, and a `livecheck` block in the position goreleaser puts
`custom_block`. `audit --strict` runs those same cops plus the formula-level rules, which
is why both are listed rather than only one.

**Then the install checks.** Nothing runs the formula's `test do` block on a tag — there
is no CI, and `brew test` is a separate command — so run it explicitly along with the
rest:

```sh
brew install macintacos/tap/herdr-reshape
brew test macintacos/tap/herdr-reshape   # runs the formula's own version assertion
herdr-reshape --version   # the tag without its v — "dev" means the ldflags stamp broke
"$(brew --prefix)"/share/herdr-reshape/bin/herdr-reshape --version   # the same version
herdr plugin link "$(brew --prefix)/share/herdr-reshape"
```

The two versions agreeing is the check, and here it is `post_install` that put the second
one there rather than any command you ran. A disagreement means `post_install` did not
copy what you just downloaded, and a `stat .../bin` error means the archive layout
regressed.

And once there is a previous release to come from, the upgrade rather than the install —
`brew upgrade herdr-reshape`, then the two versions agreeing again
**with no `link` run at all**. That is the whole point of the formula: the registration
made above survives every upgrade because the path it recorded never changes. Worth one
deliberate check per release rather than an assumption.

Then drive the plugin itself from that installed build: all five actions
(`user.reshape.move-left`, `move-right`, `move-up`, `move-down`, `fit`) on their
keybindings, and all three events (`pane.created`, `pane.closed`, `pane.exited`) by
splitting and closing panes. This is the check that the release is a working plugin rather
than a working download, and it needs a real herdr session — which is why it lives here
rather than in any automated gate.
