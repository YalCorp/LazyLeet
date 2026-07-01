# Changelog

All notable changes to LazyLeet will be documented in this file.

This project follows semantic versioning for tagged releases.

## [0.1.0] - 2026-07-01

### Added

- Product-ready terminal workflow for browsing tracks, selecting problems, reading statements, editing solutions, and tracking progress locally.
- Local solution workspace under `~/.lazyleet/workspace`.
- SQLite-backed local progress and preference storage.
- Persistent pane layout and preferred language selection.
- Editor view with problem, solution, and output panes.
- Java local test runner support for examples, all local cases, and focused failed-case replay.
- Time Limit Exceeded reporting for local runner timeouts.
- Vertical resizing for the editor solution and output panes.
- Data-pack support for external problem metadata, statements, examples, and tests.
- Release pipeline for Linux, macOS, and Windows executable packages.

### Changed

- Simplified TUI footer and metadata presentation across browse and editor views.

### Notes

- LazyLeet does not bundle scraped LeetCode content. Problem data is expected to come from external data packs.
