# Contributing

## Build and link a local checkout

The development path, and the one to use for a change you have not released.

```bash
git clone https://github.com/macintacos/herdr-reshape.git
cd herdr-reshape
mise trust       # a fresh clone's mise.toml is untrusted; setup exits 1 without this
mise run setup   # install the pinned tools, register the git hooks
mise run build   # `herdr plugin link` does NOT build — this is the step it skips

herdr plugin link "$PWD"
herdr server reload-config
```

`herdr plugin link` is herdr's own subcommand: it registers the directory where it stands
rather than copying it, so put the checkout somewhere it can live. Check it took:

```bash
herdr plugin action list --plugin user.reshape   # five actions
```

A `herdr plugin install` from GitHub needs none of this: the manifest's `[[build]]` block
compiles the binary during the install, before herdr registers the plugin, so a failed
build leaves nothing registered rather than a half-working plugin.

Everything the build needs is pinned in `mise.toml`, so [mise](https://mise.jdx.dev) is
the only thing you need on `PATH`. herdr itself must be 0.8.0 or newer — that is the
manifest's floor. Everything here was measured against 0.8.2, and the event behaviour has
not been checked below it, nor re-measured on Linux — see the note in
[`herdr-plugin.toml`](herdr-plugin.toml). The actions have nothing OS-shaped in them.

## Tasks

| Task                 | What it does                               |
| -------------------- | ------------------------------------------ |
| `mise run build`     | build the binary into `bin/herdr-reshape`  |
| `mise run format`    | rewrite every file into canonical form     |
| `mise run lint`      | check formatting and lint rules, read-only |
| `mise run test`      | run the test suite                         |
| `mise run preflight` | lint + test, the gate before pushing       |

`hk` runs the formatters and linters on every commit and the tests on every push, so those
tasks are the same checks the hooks apply — just runnable on demand.

Cutting a release is [`docs/RELEASING.md`](docs/RELEASING.md).
