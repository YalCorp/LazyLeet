# LazyLeet

<p align="center">
  <img src="docs/assets/lazyleet-demo.gif" alt="LazyLeet demo" width="900">
</p>

LazyLeet is a local-first terminal app for practicing DSA problems.

It keeps the core loop in one place: pick a problem, read the prompt, write code, run examples, submit against local tests, and move on. No browser hop. No network dependency after setup. Just the practice flow, in your terminal.

Think of it as a small, focused workspace for LeetCode-style practice. Not a replacement for LeetCode, not a content mirror, and not a giant platform. Just a fast local tool for people who like their workflow clean.

## Why

DSA practice has a way of turning into tab management:

```text
problem page -> editor -> terminal -> browser -> notes -> terminal -> repeat
```

LazyLeet tries to make that loop quieter:

```text
choose -> read -> solve -> run -> submit -> track
```

The project is opinionated about one thing: practice should be easy to resume. Your problems, solutions, progress, layout, and test runs should feel close at hand.

## What You Can Do

LazyLeet currently supports the main local practice workflow:

- browse curated or custom problem tracks
- read problem statements from local metadata
- edit solutions beside the prompt
- run public/example cases
- submit against all available local test cases
- reuse a failed submit case as a focused debug case
- track progress locally
- keep pane sizes across restarts
- store solutions in a predictable local workspace

Problem data is intentionally external. LazyLeet does not ship scraped statements, examples, editorials, or test cases. Bring a data pack, install it, and LazyLeet will pick it up.

## Quick Start

Run from source:

```bash
go run ./cmd/lazyleet
```

Build a local binary:

```bash
go build -o lazyleet ./cmd/lazyleet
```

Check the version:

```bash
go run ./cmd/lazyleet version
```

## The Editor Flow

Open the selected problem with `e`.

Inside the editor:

```text
ctrl+r   run examples
ctrl+t   submit all local tests
ctrl+y   use the first failed submit case
ctrl+s   format and save
ctrl+w   switch solution/output/problem panes
j/k      scroll the focused non-editor pane
```

For Java, LazyLeet adds common LeetCode-style imports while running code. On save, Java formatting only happens when `javac` accepts the file. If there is a syntax error, the buffer stays unchanged.

## Browse Controls

```text
j/k                 scroll or move
tab                 switch pane
ctrl+left/right     resize panes
ctrl+0              reset pane widths
/                   search
e                   edit solution
r                   run saved solution
l                   cycle language
m                   mark progress
o                   open official URL
?                   help
q / esc             quit or close
```

## Local Files

LazyLeet stores its state in `~/.lazyleet`:

```text
~/.lazyleet/
  config.toml
  db.sqlite
  packs/
  workspace/
```

What lives there:

- `db.sqlite` stores progress
- `config.toml` stores layout preferences
- `packs/` contains installed data packs
- `workspace/` contains your solutions

Example solution path:

```text
~/.lazyleet/workspace/1971_find-if-path-exists-in-graph/Solution.java
```

## Data Packs

LazyLeet is data-pack driven. A pack provides metadata, statements, and tests; the app provides the local workflow around them.

Installed packs use this shape:

```text
~/.lazyleet/packs/<pack-slug>/
  lazyleet-pack.toml
  metadata/
    index.json
    <problem>.json
  tests/
    <problem-test-file>
```

During development, LazyLeet also reads local packs from:

```text
.local/<pack-slug>-metadata/
.local/<pack-slug>/
```

See [docs/data-packs.md](docs/data-packs.md) for the pack format.

## Status

LazyLeet is early, but usable. The core loop is in place: browse, read, edit, run, submit, debug failed cases, and track progress locally.

Expect rough edges. The goal is to keep improving the local-first workflow without turning the app into something heavier than it needs to be.

## Stack

- Go
- Bubble Tea
- Bubbles
- Lip Gloss
- Cobra
- SQLite via `modernc.org/sqlite`
