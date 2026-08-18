#!/usr/bin/env bash
#
# install.sh: bootstrap the whole lachesis stack for lachesis-ui.
#
# Pulls and builds the three pieces the UI needs and wires them together:
#
#   1. the engine  (UnboundCompute/lachesis): Python; the code-graph builder,
#                  nav layer, and MCP server the UI talks to
#   2. the catalog (UnboundCompute/atropos): the sink/taint models the engine
#                  reads; cloned as a sibling so the engine auto-discovers it
#   3. the UI      (this repo): the Go terminal binary
#
# It creates the standard layout the UI already knows how to discover:
#
#   ~/.lachesis/src/lachesis   the engine checkout
#   ~/.lachesis/src/atropos    the catalog checkout (sibling → auto-found)
#   ~/.lachesis/venv           the engine's virtualenv (UI's default interpreter)
#   ~/.lachesis/graphs         where built graphs land (UI's default search dir)
#   ~/.lachesis/bin/lachesis-ui   the built UI binary
#   ~/.lachesis/build-graph.sh    helper: build a graph from any source tree
#
# Re-running is safe: it updates existing checkouts instead of recloning.
#
# Env overrides:
#   LACHESIS_HOME   install root            (default: ~/.lachesis)
#   PYTHON          python to build the venv (default: python3)
#
set -euo pipefail

LACHESIS_HOME="${LACHESIS_HOME:-$HOME/.lachesis}"
SRC="$LACHESIS_HOME/src"
VENV="$LACHESIS_HOME/venv"
BIN="$LACHESIS_HOME/bin"
GRAPHS="$LACHESIS_HOME/graphs"
PYTHON="${PYTHON:-python3}"

LACHESIS_REPO="https://github.com/UnboundCompute/lachesis.git"
ATROPOS_REPO="https://github.com/UnboundCompute/atropos.git"

info() { printf '\033[36m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33m warn:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"; }

# ---- preflight ------------------------------------------------------------
need git
need "$PYTHON"
command -v go >/dev/null 2>&1 || warn "go not found. Install Go 1.24+ to build the UI (a 'go install' fallback is tried only if you have go)"

mkdir -p "$SRC" "$BIN" "$GRAPHS"

# ---- 1 + 2. clone/update engine and catalog (side by side) ----------------
clone_or_update() {
  local url="$1" dir="$2" name="$3"
  if [ -d "$dir/.git" ]; then
    info "updating $name"
    git -C "$dir" pull --ff-only --quiet || warn "$name: could not fast-forward (local changes?), leaving as is"
  else
    info "cloning $name"
    git clone --depth 1 --quiet "$url" "$dir"
  fi
}
clone_or_update "$LACHESIS_REPO" "$SRC/lachesis" "engine (lachesis)"
clone_or_update "$ATROPOS_REPO"  "$SRC/atropos"  "catalog (atropos)"

# ---- 3. engine virtualenv -------------------------------------------------
if [ ! -x "$VENV/bin/python" ]; then
  info "creating virtualenv at $VENV"
  "$PYTHON" -m venv "$VENV"
fi
info "installing the engine into the virtualenv"
"$VENV/bin/python" -m pip install --quiet --upgrade pip
"$VENV/bin/python" -m pip install --quiet -e "$SRC/lachesis"

# ---- 4. graph-build helper ------------------------------------------------
info "writing $LACHESIS_HOME/build-graph.sh"
cat > "$LACHESIS_HOME/build-graph.sh" <<'HELPER'
#!/usr/bin/env bash
# build-graph.sh <source_dir> [name]
# Builds a lachesis graph and prints the store path (~/.lachesis/graphs/<name>.kuzu).
set -euo pipefail
SRC="${1:?usage: build-graph.sh <source_dir> [name]}"
SRC="$(cd "$SRC" && pwd)"
NAME="${2:-$(basename "$SRC")}"
OUT="$HOME/.lachesis/graphs/$NAME.kuzu"
export ATROPOS_ROOT="${ATROPOS_ROOT:-$HOME/.lachesis/src/atropos}"
"$HOME/.lachesis/venv/bin/lachesis-analyze" "$SRC" "$OUT" --prune --timeout 3600
echo "$OUT"
HELPER
chmod +x "$LACHESIS_HOME/build-graph.sh"

# ---- 5. build the UI ------------------------------------------------------
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if command -v go >/dev/null 2>&1; then
  if [ -f "$HERE/main.go" ]; then
    info "building lachesis-ui from source checkout"
    (cd "$HERE" && go build -o "$BIN/lachesis-ui" .)
  else
    info "installing lachesis-ui via go install"
    GOBIN="$BIN" go install github.com/UnboundCompute/lachesis-ui@latest
  fi
else
  warn "skipped building the UI (no go). Install Go and run: go build -o $BIN/lachesis-ui ."
fi

# ---- done -----------------------------------------------------------------
cat <<DONE

$(info 'stack ready')

  engine   $SRC/lachesis   (venv: $VENV)
  catalog  $SRC/atropos
  UI       $BIN/lachesis-ui

Add the UI to your PATH:

  export PATH="$BIN:\$PATH"

Build a graph from any Python / TypeScript / JavaScript / C tree:

  $LACHESIS_HOME/build-graph.sh /path/to/some/repo

Then launch the UI (it picks the newest graph automatically):

  lachesis-ui

DONE
