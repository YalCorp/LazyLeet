# LazyLeet

LazyLeet is a LazyGit-inspired terminal companion for LeetCode-style DSA practice.

It is built for developers who already live in the terminal and want a fast, keyboard-first way to browse curated problem tracks, open official LeetCode links, solve locally, run tests, track progress, and keep notes without constantly switching between browser, editor, notes, and shell.

> LazyGit, but for LeetCode practice.

## What LazyLeet Is

LazyLeet is not a LeetCode replacement. It is a local-first companion for people who like LeetCode, NeetCode, curated DSA sheets, Vim, Neovim, tmux, and fast terminal workflows.

The goal is to make daily DSA practice feel native to the terminal while keeping official problem pages one shortcut away.

## Core Goals

- Fast startup with no network calls required to open the main UI.
- Lightweight terminal UI without Electron, browser engines, or GPU-heavy rendering.
- Keyboard-first navigation inspired by LazyGit.
- Local solution files, notes, tests, and progress.
- Curated tracks such as Blind 75, NeetCode 150, Striver-style sheets, and pattern-based lists.
- Additive problem lists where multiple tracks can reference the same canonical problem.
- Optional Mermaid support for diagrams in Markdown notes.

## Current Demo

The repository now contains a first runnable Go demo:

- `lazyleet` opens a pane-based terminal UI.
- `lazyleet version` prints build metadata.
- Blind 75 and NeetCode 150 are bundled as metadata-only tracks.
- Progress is stored locally in SQLite at `~/.lazyleet/db.sqlite`.
- Local Python, Go, and Java solution files are edited in an embedded terminal editor and saved under `~/.lazyleet/workspace`.
- Local statement previews are read from per-problem `statement.md` files.
- Search, pane navigation, a command palette, help, official URLs, solution editing, and progress marking are available in the TUI.

Install or run from source:

```bash
go run ./cmd/lazyleet
go run ./cmd/lazyleet version
```

Build a local binary:

```bash
go build ./cmd/lazyleet
```

## MVP Scope

Future MVP work should add:

- Local Markdown notes per problem.
- Statement editing/import workflows for user-provided problem text.
- Local test running for supported languages.
- `$EDITOR` integration for solving in tools like Neovim, Vim, Helix, or Nano.

The MVP does not aim to include:

- A full IDE-grade embedded code editor.
- Community solutions or public discussion threads.
- AI-generated explanations by default.
- Mandatory terminal graphics support.
- Mandatory online LeetCode submission.

## Example Workflow

```bash
lazyleet
```

Inside the TUI:

```text
/        search problems
Enter    open selected problem
e        edit local solution
l        cycle language
ctrl+s   save solution while editing
esc      leave editor
r        run tests
n        open notes
d        view diagrams
o        open official LeetCode URL
m        mark progress
q        go back / quit
?        help
```

Typical flow:

1. Open LazyLeet.
2. Pick a track, such as Blind 75 or NeetCode 150.
3. Select a problem.
4. Review title, difficulty, tags, URL, and local details.
5. Add your own local statement text to `statement.md` if you want the prompt visible in LazyLeet.
6. Press `l` to choose Python, Go, or Java.
7. Press `e` to solve in the embedded editor.
8. Press `r` to run tests.
9. Press `n` to edit notes.
10. Mark progress locally.
11. Press `o` to open the official LeetCode page when needed.

## Problem Metadata

Problems should use canonical LeetCode metadata where possible:

```toml
id = 1
slug = "two-sum"
title = "Two Sum"
difficulty = "Easy"
url = "https://leetcode.com/problems/two-sum/"
tags = ["Array", "Hash Table"]
```

Tracks should reference canonical problem slugs or IDs, so a problem like `Two Sum` can appear in Blind 75, NeetCode 150, Arrays, Hash Table, and custom lists while remaining one canonical local problem.

## Local-First Storage

LazyLeet should keep user data local and portable:

```text
~/.lazyleet/
  config.toml
  db.sqlite
  tracks/
    neetcode-150.toml
    blind-75.toml
    striver-sde.toml
    patterns.toml
    custom.toml
  workspace/
    two-sum/
      statement.md
      solution.py
      solution.go
      Solution.java
      notes.md
      tests.json
```

Solutions, statements, and notes are normal files, and progress is stored locally so users can back up or sync their workspace however they prefer. LazyLeet does not bundle LeetCode statements; `statement.md` is for user-provided local text.

## Progress Tracking

LazyLeet should support local progress states such as:

- Todo
- Attempted
- Solved
- Revisiting
- Skipped

Progress should be visible per problem, per track, and per pattern.

## Technology Direction

The implementation stack is:

- Go 1.26
- Bubble Tea v2
- Bubbles v2
- Lip Gloss v2
- Cobra
- SQLite via `modernc.org/sqlite`
- GoReleaser
- A single lightweight CLI binary named `lazyleet`

Rust with Ratatui is also a strong alternative. Heavy desktop or browser-based frameworks should be avoided.

## Command Shape

Primary command:

```bash
lazyleet
```

Possible future subcommands:

```bash
lazyleet open two-sum
lazyleet run
lazyleet notes two-sum
lazyleet tracks
lazyleet progress
lazyleet config
```

## Project Status

LazyLeet has a runnable interactive TUI demo with local progress persistence. It is not yet a full solving workspace.
