#!/usr/bin/env bash
# install-ui-binary.sh: download and verify a published Lachesis UI archive.
set -euo pipefail

UI_VERSION="${LACHESIS_UI_VERSION:-v0.1.1}"
LACHESIS_HOME="${LACHESIS_HOME:-$HOME/.lachesis}"
BIN="$LACHESIS_HOME/bin"
REPO="UnboundCompute/lachesis-ui"

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
  cat <<'USAGE'
Usage: ./scripts/install-ui-binary.sh

Download the pinned Lachesis UI release for this machine, verify its SHA-256
checksum from the matching SHA256SUMS file, and install it under
$LACHESIS_HOME/bin. Set LACHESIS_UI_VERSION to a reviewed tag such as v0.1.1.
USAGE
  exit 0
fi
if [ "$#" -ne 0 ]; then
  echo "error: unknown argument '$1' (try --help)" >&2
  exit 2
fi

case "$UI_VERSION" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "error: LACHESIS_UI_VERSION must be a release tag such as v0.1.1" >&2; exit 2 ;;
esac

need() { command -v "$1" >/dev/null 2>&1 || { echo "error: missing required tool: $1" >&2; exit 1; }; }
need curl
need tar
if command -v sha256sum >/dev/null 2>&1; then
  CHECKSUM_CMD=(sha256sum --check -)
elif command -v shasum >/dev/null 2>&1; then
  CHECKSUM_CMD=(shasum -a 256 --check -)
else
  echo "error: missing SHA-256 verifier (sha256sum or shasum)" >&2
  exit 1
fi

case "$(uname -s)" in
  Darwin) GOOS=darwin ;;
  Linux) GOOS=linux ;;
  *) echo "error: unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) GOARCH=amd64 ;;
  arm64|aarch64) GOARCH=arm64 ;;
  *) echo "error: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

ASSET="lachesis-ui-${UI_VERSION}-${GOOS}-${GOARCH}.tar.gz"
BASE="https://github.com/${REPO}/releases/download/${UI_VERSION}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Downloading $ASSET"
curl --fail --silent --show-error --location "$BASE/$ASSET" --output "$TMP/$ASSET"
curl --fail --silent --show-error --location "$BASE/SHA256SUMS" --output "$TMP/SHA256SUMS"

(cd "$TMP" && grep -F "  $ASSET" SHA256SUMS | "${CHECKSUM_CMD[@]}")

mkdir -p "$BIN"
tar -xzf "$TMP/$ASSET" -C "$TMP"
EXTRACTED="$TMP/lachesis-ui-${UI_VERSION}-${GOOS}-${GOARCH}/lachesis-ui"
[ -x "$EXTRACTED" ] || { echo "error: release archive did not contain an executable UI" >&2; exit 1; }
cp "$EXTRACTED" "$BIN/lachesis-ui"
chmod +x "$BIN/lachesis-ui"

EXPECTED="lachesis-ui ${UI_VERSION#v}"
ACTUAL="$($BIN/lachesis-ui --version)"
[ "$ACTUAL" = "$EXPECTED" ] || { echo "error: installed UI reports '$ACTUAL', expected '$EXPECTED'" >&2; exit 1; }

echo "Installed $ACTUAL at $BIN/lachesis-ui"
echo "Add it to your PATH: export PATH=\"$BIN:\$PATH\""
