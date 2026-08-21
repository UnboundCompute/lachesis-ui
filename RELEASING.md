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

Archive ordering, timestamps, and ownership metadata are normalized from the tagged
commit, so rebuilding the same tag produces identical archive bytes.

The workflow uploads artifacts but does not publish a GitHub release or modify a
package registry. Promote the reviewed artifacts explicitly and retain the prior
version for rollback. Never overwrite a published tag; cut a new patch release.
