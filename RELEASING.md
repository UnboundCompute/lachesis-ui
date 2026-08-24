# Releasing lachesis-ui

The UI version is recorded in [`VERSION`](VERSION). The UI is a Go binary released independently from the Lachesis engine and Atropos
catalog. Keep the UI tag and the engine/catalog refs it discovers separate.

Before tagging:

```bash
gofmt -l .                 # must print nothing
go vet ./...
go test ./...
go build ./...
```

Update `VERSION` and the matching changelog heading, then create an annotated
`vMAJOR.MINOR.PATCH` tag from a clean commit. The tag workflow
builds Linux and macOS binaries for amd64 and arm64 with CGO disabled, packages the
README and license, and writes `SHA256SUMS`. Downloaded archives must pass:

```bash
sha256sum -c SHA256SUMS --ignore-missing
```

On macOS, the equivalent verification command is:

```bash
shasum -a 256 -c SHA256SUMS
```

The workflow stamps each binary with the tag version; `go install ...@vTAG` also
reports the module tag through Go build metadata. Verify a downloaded or installed
binary with `./lachesis-ui --version` before promoting it.

The release workflow extracts the native Linux amd64 archive and performs that version
smoke test against the extracted binary; the other targets are cross-compiled and
verified by their reproducible archive hashes.

The UI startup contract also requires the engine to report MCP protocol
`2024-11-05` and a non-empty engine version. Keep this check in the clean-machine
TUI smoke test when changing the engine/client boundary.

Archive ordering, timestamps, ownership metadata, and the gzip header are normalized
from the tagged commit, so rebuilding the same tag produces identical archive bytes.

The workflow publishes a GitHub Release with all four archives and one combined
`SHA256SUMS` file. It does not modify a package registry. Retain the prior version
for rollback and never overwrite a published tag; cut a new patch release.
