# herdr-reshape

A [herdr](https://herdr.dev) plugin that moves the focused pane around its tab and squares
the tab up into an even grid.

herdr can resize a pane, split one, and send one to another tab. It has no word for
*re-orienting* a pane against the one beside it, and no word for "make these even" — a new
pane always lands on a halved split, so three panes come out 50/25/25 rather than in
thirds. This plugin adds both.

## What it does

**Move** re-orients the focused pane against its **sibling** in the split tree.

That is not the same as travelling to whatever pane lies in that direction. Take
`A | (B / C)` — A beside a column of B stacked over C — with B focused. Moving right puts
B to the right of C; *travelling* right would reach nothing, and left would reach A.

Three outcomes, then. The split rotates onto the axis you asked for, as it does above. Or
the sibling is already on that axis, and the two swap places. Or the pane is already on
that side, and nothing moves — herdr says so rather than leaving you guessing. Whatever
the pane is running keeps running across any of it.

**Fit** squares a tab's panes into an even grid, rows and columns together, so nested
splits come out even in both directions.

Fit also runs on its own as panes come and go — but only on a tab that is still even, so a
tab you have sized yourself is left alone. Running `fit` there is both how you put it back
on the grid and how you re-arm the automatic one.

## Install

Needs herdr 0.8.0 or newer.

```bash
brew install --cask macintacos/tap/herdr-reshape
herdr plugin link "$(brew --prefix)/share/herdr-reshape"
herdr server reload-config
```

`herdr plugin link` is one-time. The cask refreshes that path on every install and
upgrade, so from here on `brew upgrade` is the whole procedure. Check it took —
`user.reshape` is the id herdr knows this plugin by:

```bash
herdr plugin action list --plugin user.reshape   # five actions
```

Installed 0.3.0 or 0.3.1, which shipped as a formula? Swap it for the cask once. The
registration survives — both put the plugin root at the same path, so there is no
`herdr plugin link` to re-run:

```bash
brew uninstall herdr-reshape
brew install --cask macintacos/tap/herdr-reshape
```

### On Linux

Homebrew has no cask support on Linux, so take the prebuilt tarball instead. Every
[release](https://github.com/macintacos/herdr-reshape/releases) ships `linux_amd64` and
`linux_arm64` archives that are already a plugin root — `bin/herdr-reshape` beside the
manifest — so nothing is compiled and Go is not needed.

```bash
mkdir -p ~/.local/opt/herdr-reshape
tar -xzf herdr-reshape_0.3.1_linux_arm64.tar.gz -C ~/.local/opt/herdr-reshape
herdr plugin link ~/.local/opt/herdr-reshape
```

Replace the tarball in place to upgrade. The registration points at a directory that is
yours rather than a package manager's, so nothing renumbers it and the link holds.

The event hooks' behaviour was measured on macOS and has not been re-measured on Linux —
see the note in [`herdr-plugin.toml`](herdr-plugin.toml). The actions have nothing
OS-shaped in them.

## Configure

Nothing is bound out of the box. These are the five actions and the bindings worth giving
them:

| Action                    | Binding        | Vim alternative  | What it does                    |
| ------------------------- | -------------- | ---------------- | ------------------------------- |
| `user.reshape.move-left`  | `prefix+left`  | `prefix+shift+h` | Move the pane left              |
| `user.reshape.move-down`  | `prefix+down`  | `prefix+shift+j` | Move the pane down              |
| `user.reshape.move-up`    | `prefix+up`    | `prefix+shift+k` | Move the pane up                |
| `user.reshape.move-right` | `prefix+right` | `prefix+shift+l` | Move the pane right             |
| `user.reshape.fit`        | `prefix+=`     | —                | Square this tab into a grid     |

Each binding is one `[[keys.command]]` block in `~/.config/herdr/config.toml`. The five
arrow bindings above, ready to paste:

```toml
[[keys.command]]
key         = "prefix+left"
type        = "plugin_action"
command     = "user.reshape.move-left"
description = "move the pane left"

[[keys.command]]
key         = "prefix+down"
type        = "plugin_action"
command     = "user.reshape.move-down"
description = "move the pane down"

[[keys.command]]
key         = "prefix+up"
type        = "plugin_action"
command     = "user.reshape.move-up"
description = "move the pane up"

[[keys.command]]
key         = "prefix+right"
type        = "plugin_action"
command     = "user.reshape.move-right"
description = "move the pane right"

[[keys.command]]
key         = "prefix+="
type        = "plugin_action"
command     = "user.reshape.fit"
description = "best-fit the tab"
```

Then apply them:

```bash
herdr server reload-config
```

For the vim alternative, add a second block per action — same `command`, different `key`.
Binding the moves both ways at once is harmless. For fit, `=` is the key tmux uses for
`select-layout`; write it as the literal character, as herdr rejects both `equal` and
`equals`.

## Uninstall

Three steps, because brew owns only the first. The plugin root at `share/herdr-reshape` is
written by the cask's own post-install hook rather than installed by brew, so a plain
uninstall leaves it behind — and `--zap` is not the answer, since brew removes a path
outside your home directory with `sudo` and would ask for a password to delete a directory
you already own. herdr's registration is its own to forget.

```bash
brew uninstall --cask herdr-reshape
herdr plugin unlink user.reshape
rm -rf "$(brew --prefix)/share/herdr-reshape"
```

Delete the `[[keys.command]]` blocks from `~/.config/herdr/config.toml` too — nothing else
will.

## Contributing

Building and linking a local checkout is [`CONTRIBUTING.md`](CONTRIBUTING.md); cutting a
release is [`docs/RELEASING.md`](docs/RELEASING.md).
