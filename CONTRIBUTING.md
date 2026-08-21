# Contributing to lachesis-ui

Thanks for your interest in improving lachesis-ui. This document covers how to
get a working setup, the layout of the code, and what has to pass before a
change can be merged.

By participating in this project you agree to abide by our
[Code of Conduct](CODE_OF_CONDUCT.md).

## What this repo is

`lachesis-ui` is the terminal front end only. It is a single Go binary built on
[Bubble Tea](https://github.com/charmbracelet/bubbletea). It does not analyze
code itself; it spawns the lachesis **engine** (Python) over MCP and renders
what the engine reports. If your change is about how a graph is built or what
the engine computes, it belongs in the
[engine repo](https://github.com/UnboundCompute/lachesis), not here. If it is
about how that information is shown or navigated, it belongs here.

## Getting set up

You need Go 1.24.2+ and, to run the UI against a real graph, the engine.

```sh
git clone https://github.com/UnboundCompute/lachesis-ui
cd lachesis-ui

# build the whole stack (engine, catalog, this UI) into ~/.lachesis
./scripts/install.sh

# or just build the binary, if you already have the engine
make build
```

Build a graph from any source tree and launch:

```sh
~/.lachesis/build-graph.sh /path/to/some/repo
./lachesis-ui
```

## Code layout

```
main.go                  flag parsing, engine discovery, program bootstrap
internal/mcp/            the MCP client: spawn the engine, call its tools
internal/ui/             the Bubble Tea app (one file per screen)
  app.go                 root model, key routing, header and status bar
  overview.go            the Overview screen
  tree.go                the Tree screen (nested source tree plus outline)
  neighborhood.go        the Neighborhood screen
  search.go              the symbol search overlay
  styles.go              the shared color palette and style helpers
  msgs.go                cross-screen messages and the engine-call commands
internal/subsystem/      subsystem grouping helpers
scripts/install.sh       bootstrap the full stack into ~/.lachesis
```

The UI follows the Elm architecture that Bubble Tea uses: each screen is a
model with `update` and `view`, engine calls run off the UI goroutine as a
`tea.Cmd` and return a `tea.Msg` that the owning screen folds back in. When you
add a screen, keep the engine call in a command in `msgs.go` and the rendering
in the screen's own file.

## Before you open a pull request

Run the local gate. CI runs the same command and will block a merge if any check fails.

```sh
make check    # gofmt check, vet, build, and tests
```

Use `make fmt` when the formatting check reports files that need rewriting.

Guidelines:

- Keep formatting gofmt-clean. CI fails on any file gofmt would rewrite.
- Match the style of the surrounding code: comment density, naming, and idiom.
- Keep commit messages short and in the imperative mood ("fix outline scroll",
  not "fixed" or "fixes").
- One logical change per pull request where you can. Small, focused diffs are
  easier to review and revert.
- If a change is visible on screen, a short description of the before and after
  (or a paste of the rendered screen) helps a lot.

## Reporting bugs and requesting features

Open an issue. For bugs, include the graph you ran against (or how to build a
similar one), the exact keys you pressed, what you expected, and what happened.
For security issues, do not open a public issue; see [SECURITY.md](SECURITY.md).

## License of contributions

This repository is licensed under the [MIT License](LICENSE). By contributing,
you agree that your contributions are licensed under the same terms.
