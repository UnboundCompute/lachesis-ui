#!/usr/bin/env bash
# doctor.sh: validate a Lachesis UI stack without building or scanning a graph.
set -u

LACHESIS_HOME="${LACHESIS_HOME:-$HOME/.lachesis}"
SRC="$LACHESIS_HOME/src"
VENV="$LACHESIS_HOME/venv"
BIN="$LACHESIS_HOME/bin"
MANIFEST="$LACHESIS_HOME/stack-manifest.json"

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  cat <<'USAGE'
Usage: ./scripts/doctor.sh

Check the installed Lachesis engine, Atropos catalog, graph helper, UI binary,
and resolved stack manifest. Set LACHESIS_HOME to inspect another installation.
This command does not build a graph or modify the installation.
USAGE
  exit 0
fi
if [ "$#" -ne 0 ]; then
  echo "error: unknown argument '$1' (try --help)" >&2
  exit 2
fi

ok=0
warn=0
fail=0
pass() { printf '  [ok]   %s\n' "$*"; ok=$((ok + 1)); }
notice() { printf '  [warn] %s\n' "$*"; warn=$((warn + 1)); }
bad() { printf '  [fail] %s\n' "$*"; fail=$((fail + 1)); }

echo "Lachesis stack doctor"
echo "  root: $LACHESIS_HOME"

if [ -f "$MANIFEST" ]; then
  if python3 - "$MANIFEST" <<'PY'
import json
import sys
from pathlib import Path

data = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
required = {"schema_version", "evidence_schema_version", "product", "engine", "catalog", "ui"}
missing = sorted(required - data.keys())
if missing:
    raise SystemExit("missing fields: " + ", ".join(missing))
if data["evidence_schema_version"] != 1:
    raise SystemExit("unsupported evidence schema version: " + str(data["evidence_schema_version"]))
PY
  then
    pass "stack manifest is valid"
  else
    bad "stack manifest is not valid JSON or is missing required fields"
  fi
else
  bad "stack manifest missing: $MANIFEST (run scripts/install.sh)"
fi

if [ -x "$VENV/bin/python" ]; then pass "engine Python: $VENV/bin/python"; else bad "engine Python missing: $VENV/bin/python"; fi
if [ -x "$VENV/bin/lachesis-analyze" ]; then pass "graph builder is installed"; else bad "lachesis-analyze missing from the engine environment"; fi
if [ -x "$VENV/bin/lachesis" ]; then pass "Lachesis CLI is installed"; else bad "Lachesis CLI missing from the engine environment"; fi
if { [ -d "$SRC/atropos" ] || { [ -n "${ATROPOS_ROOT:-}" ] && [ -d "$ATROPOS_ROOT" ]; }; }; then
  pass "Atropos catalog is discoverable"
else
  bad "Atropos catalog missing (expected $SRC/atropos or ATROPOS_ROOT)"
fi
if [ -x "$LACHESIS_HOME/build-graph.sh" ]; then pass "graph helper is executable"; else bad "graph helper missing: $LACHESIS_HOME/build-graph.sh"; fi
if [ -x "$BIN/lachesis-ui" ]; then
  if "$BIN/lachesis-ui" --version >/dev/null 2>&1; then pass "UI binary responds to --version"; else bad "UI binary does not start cleanly"; fi
else
  bad "UI binary missing: $BIN/lachesis-ui"
fi

echo
printf 'Result: %d passed, %d warnings, %d failed\n' "$ok" "$warn" "$fail"
if [ "$fail" -ne 0 ]; then
  echo "Repair the failed checks, then rerun the doctor."
  exit 1
fi
echo "Stack is ready. Build a graph with: $LACHESIS_HOME/build-graph.sh /path/to/repository"
exit 0
