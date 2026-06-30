# LazyLeet

LazyLeet is a local-first terminal app for DSA practice.

Think: **LazyGit for LeetCode-style problems**.

It helps you browse tracks, read local problem statements, write solutions, run local test cases, and track progress without depending on the browser for your daily practice loop.

## Why

DSA practice should be simple:

```text
open track -> pick problem -> read prompt -> solve locally -> run tests -> mark progress
```

LazyLeet keeps that loop in the terminal.

## What It Does

- Works offline once your data packs are installed.
- Shows curated tracks such as Blind 75, NeetCode 150, and custom packs.
- Reads problem metadata, statements, and test cases from external data packs.
- Stores progress locally in SQLite.
- Saves solutions under `~/.lazyleet/workspace`.
- Opens official problem URLs when you want the source page.
- Lets you resize panes and remembers the layout.

LazyLeet does **not** bundle LeetCode statements or scraped data. Problem content belongs in external data packs that you install separately.

## Run

From source:

```bash
go run ./cmd/lazyleet
```

Build:

```bash
go build ./cmd/lazyleet
```

Version:

```bash
go run ./cmd/lazyleet version
```

## Controls

```text
j/k      scroll
tab      switch pane
[        shrink active pane
]        widen active pane
0        reset pane widths
/        search
e        edit solution
l        cycle language
m        mark progress
o        open official URL
?        help
q        quit
```

## Local Files

LazyLeet keeps user state in `~/.lazyleet`:

```text
~/.lazyleet/
  config.toml
  db.sqlite
  packs/
  workspace/
```

Progress goes into `db.sqlite`.

Solutions go into `workspace/`.

Pane layout goes into `config.toml`.

Data packs go into `packs/`.

## Data Packs

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

See [docs/data-packs.md](docs/data-packs.md) for the full format.

## Stack

- Go
- Bubble Tea
- Bubbles
- Lip Gloss
- Cobra
- SQLite via `modernc.org/sqlite`

## Status

LazyLeet is early but usable: the TUI, local progress, local solutions, external data-pack loading, metadata statement rendering, test-case counting, and pane layout persistence are implemented.
