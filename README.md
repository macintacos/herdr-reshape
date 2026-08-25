# herdr-reshape

A [herdr](https://herdr.dev) plugin that moves the focused pane around its tab and squares
the tab up into an even grid.

herdr can resize a pane, split one, and send one to another tab. It has no word for
*re-orienting* a pane against the one beside it, and no word for "make these even" — a new
pane always lands on a halved split, so three panes come out 50/25/25 rather than in
thirds. This adds both, and runs the second one as panes come and go, on tabs you have not
resized yourself.

**Move** re-orients the focused pane against its **sibling** in the split tree, which is
not the same as travelling to whatever pane lies in that direction. With `A | (B / C)` and
B focused, moving right puts B beside C — travelling right would reach nothing, and left
would reach A. Whatever the pane is running keeps running across the move.

**Fit** works on rows and columns together, not just one axis: it projects every pane edge
onto shared grid lines and puts each divider on the line an even grid would want, so a tab
of nested splits comes out even in both directions and fitting an already-fitted tab does
nothing.

> The manifest declares all five actions and all three events, but the commands behind
> them are stubs — every one exits non-zero saying so. The grid arithmetic is being ported
> from the Python original and lands next.

## Requirements

herdr 0.8.0 or newer — that is the manifest's floor. Everything described here was
measured against 0.8.2, and the event behaviour has not been checked below it.

Everything else is pinned in `mise.toml`, so [mise](https://mise.jdx.dev) is the only
other thing you need on `PATH`.

## Build and link a local checkout

```bash
git clone https://github.com/macintacos/herdr-reshape.git
cd herdr-reshape
mise run setup   # install the pinned tools, register the git hooks
mise run build   # `herdr plugin link` does NOT build — this is the step it skips

herdr plugin link "$PWD"
herdr server reload-config
```

`link` registers the directory where it stands rather than copying it, so put the checkout
somewhere it can live. Check it took:

```bash
herdr plugin action list --plugin user.reshape   # five actions
```

A `herdr plugin install` from GitHub needs none of this: the manifest's `[[build]]` block
compiles the binary during the install, before herdr registers the plugin, so a failed
build leaves nothing registered rather than a half-working plugin.

## Working on it

| Task                | What it does                                    |
| ------------------- | ----------------------------------------------- |
| `mise run build`    | build the binary into `bin/herdr-reshape`       |
| `mise run format`   | rewrite every file into canonical form          |
| `mise run lint`     | check formatting and lint rules, read-only      |
| `mise run test`     | run the test suite                              |
| `mise run preflight`| lint + test, the gate before pushing            |

`hk` runs the formatters and linters on every commit and the tests on every push, so those
tasks are the same checks the hooks apply — just runnable on demand.
