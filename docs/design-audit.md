# Lachesis TUI design audit and capability handoff

This document records the design review and the graph capabilities the UI
needs. It is intentionally kept in the `lachesis-ui` branch so a follow-up
session can implement engine changes without losing the product decisions.

## UX verdict

The dark terminal visual system is strong: the cyan selection rail, restrained
One-Dark palette, breadcrumbs, and persistent key hints make a dense graph
readable. The original labels were too graph-centric (`REACHED BY`, `USES`,
`WHAT IT DEFINES`) for a first-time developer. The implementation uses
`CALLED FROM`, `CALLS`, `SYMBOLS IN THIS FILE`, `AREAS`, and `STARTING POINTS`,
with a `?` explanation overlay. This preserves the visual design while making
the mental model explicit.

The findings screens use the same spacing and colors, but deliberately call
candidate results *evidence* rather than vulnerabilities. The MCP server
describes candidates as pointers and does not make a safe/unsafe decision;
showing a red “bug” badge would create a dangerous false promise.

## Screen coverage

| Design artboard | UI status | Data source |
| --- | --- | --- |
| Overview | implemented | `hubs`, `open_folder` |
| Tree | implemented | `open_folder`, `open_file`, local source checkout |
| Neighborhood | implemented | `callers`, `callees`, `read_body`, `flow`, `guards`, `points_to` |
| Findings home | implemented | `candidates` |
| Finding detail | implemented | `candidate_detail` |
| Reaches witness | implemented with unavailable-evidence state | `reaches` |
| Skeleton | implemented with unavailable-evidence state | `skeleton` |
| Command palette | command entry is wired through `:`; compact command list is the next visual refinement | MCP tool names |
| Hubs artboard | represented by Overview “good places to start” | `hubs` |

## Known gaps and stubs

The interaction model is intentionally wider than the static artboards:

- Findings has a return stack, so evidence review returns to the screen that
  opened it instead of always teleporting to the overview.
- Tree and Neighborhood expose the skeleton action shown in the designs; when
  the required candidate context is absent they use a clearly marked function
  stub rather than fabricating a sink.
- The command palette runs `reaches`, `sources_of`, and `flow` into a bounded
  raw-result screen. This makes advanced graph operations discoverable without
  adding another permanent pane.
- Every tool result has an explicit error/empty state, and every screen is
  clipped to the terminal viewport so long responses cannot push the footer
  off-screen.

1. **Candidate payload shape varies by catalog constructor.** The UI keeps
   stable triage fields and a raw evidence map. If a constructor omits a field,
   the screen says “not reported” rather than inventing data.
2. **Review decisions are session-scoped.** The server now accepts `review` and
   the UI's `k` action records a neutral decision keyed by candidate id for the
   running session. Durable storage keyed by graph and candidate remains a
   future product decision; the UI does not claim that session notes survive a
   restart.
3. **Opening `$EDITOR` is intentionally local.** The graph protocol returns
   source locations, not an editor session. The current source-tree fallback
   reads the checkout; editor integration should be added as a local launcher,
   not an MCP mutation.
4. **Graph metadata (exact node/edge totals, language/frontend/dataflow tier)
   is not exposed by the navigation client.** Overview labels the missing
   per-area totals as a graph limitation instead of showing guessed numbers.
5. **A witness can be absent even when a candidate exists.** `reaches` follows
   a different edge set from the catalog. The Reaches screen therefore shows
   an explicit unavailable-evidence state and does not synthesize hops.
6. **Whole-file source now has a read-only server fallback.** The UI prefers the
   local checkout and asks the server's bounded `read_file` tool when the
   checkout is unavailable. The response still reports truncation explicitly.

## MCP score notes

The supplied `~/lachesis_glama_scores.md` and `.json` reports were treated as
documentation inputs, not copied into the product. Their main implication for
the UI is progressive disclosure: high-value comprehension tools belong in the
primary navigation, while hunting/coverage tools should be reachable from the
Findings screen and command palette. Tool descriptions that say “neutral,”
“coverage,” or “unavailable” are surfaced in the UI copy so developers do not
mistake analysis gaps for clean code.

The structured report contains 45 tools: 26 tier-A, 12 tier-B, and 7 tier-C.
The lower-scoring tools are mostly advanced analysis surfaces, so the UI keeps
them out of the default navigation and exposes their raw response only after a
developer deliberately chooses the command. This avoids presenting a sparse or
ambiguous result as a conclusion.
