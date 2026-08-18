# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Nested, collapsible source tree on the Tree screen. Folders expand in place
  and load their children lazily, with child-count badges, so a large repo
  opens as a handful of top-level folders rather than a flat wall of files.
- Screenshots of the three screens in the README.
- Open-source project scaffolding: MIT LICENSE, contributing guide, code of
  conduct, security policy, and a continuous-integration workflow.

### Changed

- The Tree keys now match a nested tree: right expands a folder, left collapses
  it or jumps to its parent, and enter opens a folder or a symbol's
  neighborhood.

## [0.1.0]

### Added

- First navigation-only release: Overview, Tree, and Neighborhood screens over
  a lachesis code-property-graph, driven entirely by the keyboard.
