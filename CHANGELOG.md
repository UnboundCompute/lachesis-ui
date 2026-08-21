# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

- Added a machine-readable `VERSION` source used by the installer’s source-build
  default; release CI now rejects tags that do not match it.

### Added

- Tagged releases now build checksummed Linux and macOS binaries for amd64 and arm64.
- Nested, collapsible source tree on the Tree screen. Folders expand in place
  and load their children lazily, with child-count badges, so a large repo
  opens as a handful of top-level folders rather than a flat wall of files.
- Screenshots of the three screens in the README.
- Open-source project scaffolding: MIT LICENSE, contributing guide, code of
  conduct, security policy, and a continuous-integration workflow.

### Changed

- Release verification now executes the native Linux binary and checks its stamped
  version from the extracted archive before uploading cross-platform archives.

- The stack installer now disables Git terminal prompts so unattended setup fails
  clearly when a checkout cannot authenticate or reach its repository.

- The stack installer now applies Git low-speed and pip network bounds, so stalled
  repository or dependency transfers fail clearly instead of hanging unattended.

- UI MCP tool requests now have a two-minute default bound, configurable with
  `LACHESIS_UI_REQUEST_TIMEOUT`, and report recent engine diagnostics when a request
  times out instead of leaving the terminal waiting indefinitely.

- The install documentation now uses `python -m pip` so engine dependencies
  stay in the interpreter the UI discovers.

- Engine startup recovery now points to the supported source-checkout install and
  labels the PyPI command as conditional until a release is published.

- The binary-only install section now distinguishes the working source build from
  the `go install` command reserved for a published UI tag.

- The stack installer now warns when engine or catalog refs remain on mutable `main`,
  while preserving those defaults for development use.

- UI startup diagnostics now use `python -m pip` in their engine-install hint,
  avoiding interpreter mismatches during recovery.

- The installer rejects empty, option-like, or whitespace-containing engine,
  catalog, and UI refs before invoking Git.

- MCP startup and request failures now include the bounded recent engine stderr
  tail, making wrong-interpreter, missing-graph, and import failures visible in
  the UI instead of looking like a silent server exit.

- Added a single `make check` developer/CI gate covering formatting, vet, build,
  and tests.

- The generated `build-graph.sh` helper rejects path-like graph names so output
  cannot escape the configured graph directory.

- The generated graph helper now reports a clear error for a missing source
  directory before attempting to resolve it.

- The Tree keys now match a nested tree: right expands a folder, left collapses
  it or jumps to its parent, and enter opens a folder or a symbol's
  neighborhood.
- Custom `LACHESIS_HOME` installs now work end-to-end, including UI engine and graph
  discovery.
- Engine startup is bounded by a configurable timeout, and invalid graph artifacts
  are rejected with actionable diagnostics.

## [0.1.0]

### Added

- First navigation-only release: Overview, Tree, and Neighborhood screens over
  a lachesis code-property-graph, driven entirely by the keyboard.
