# Releasing lachesis-ui

The UI is a Go binary released independently from the Lachesis engine and Atropos
catalog. Keep the UI tag and the engine/catalog refs it discovers separate.

Before tagging:

```bash
gofmt -l .                 # must print nothing
go vet ./...
go test ./...
go build ./...
```

Create an annotated `vMAJOR.MINOR.PATCH` tag from a clean commit. The tag workflow
builds Linux and macOS binaries for amd64 and arm64 with CGO disabled, packages the
README and license, and writes `SHA256SUMS`. Downloaded archives must pass:

```bash
sha256sum -c SHA256SUMS --ignore-missing
```

On macOS, the equivalent verification command is:

```bash
shasum -a 256 -c SHA256SUMS
```

The workflow stamps each binary with the tag version; verify a downloaded binary with
`./lachesis-ui --version` before promoting it.

The release workflow performs that version smoke test automatically for the native
Linux amd64 artifact; the other targets are cross-compiled and verified by their
reproducible archive hashes.

Archive ordering, timestamps, ownership metadata, and the gzip header are normalized
from the tagged commit, so rebuilding the same tag produces identical archive bytes.

The workflow uploads artifacts but does not publish a GitHub release or modify a
package registry. Promote the reviewed artifacts explicitly and retain the prior
version for rollback. Never overwrite a published tag; cut a new patch release.
