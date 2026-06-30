# LazyLeet Data Packs

LazyLeet data packs are local, user-provided problem sets. The open-source app ships the loader and schema, not copyrighted problem data or proprietary test cases.

Data packs are meant to work like installable packages: once a pack directory is placed under `~/.lazyleet/packs/`, LazyLeet discovers it automatically at startup.

## Folder Layout

Installed pack layout:

```text
~/.lazyleet/
  packs/
    graph-tiers/
      lazyleet-pack.toml
      metadata/
        index.json
        200_Number_of_Islands.json
      tests/
        200_Number_of_Islands
```

Development shortcut layout:

```text
.local/
  graph-tiers-metadata/
    index.json
    200_Number_of_Islands.json
  graph-tiers/
    200_Number_of_Islands
```

LazyLeet scans both locations:

- `~/.lazyleet/packs/*/lazyleet-pack.toml`
- `.local/*-metadata/index.json`

Installed packs are the stable format. The `.local` layout is only for quick local development and migration of existing data.

## Pack Manifest

Each installed pack has a `lazyleet-pack.toml` file:

```toml
slug = "graph-tiers"
title = "Graph Tiers"
version = "0.1.0"
description = "Graph practice pack with metadata and local test cases."
metadata_dir = "metadata"
tests_dir = "tests"
```

Required fields:

- `slug`: stable pack identifier; also used as the TUI track slug
- `title`: display name

Optional fields:

- `version`: pack version
- `description`: shown as track metadata
- `metadata_dir`: defaults to `metadata`
- `tests_dir`: defaults to `tests`

The Go CLI does not import or execute pack code. A pack is data only.

## Metadata Index

`metadata/index.json` is an ordered list:

```json
[
  {
    "id": 200,
    "title": "Number of Islands",
    "file": "200_Number_of_Islands.json",
    "link": "https://leetcode.com/problems/number-of-islands/"
  }
]
```

Required fields:

- `id`: canonical problem id when known
- `title`: display title
- `file`: metadata JSON filename relative to the metadata directory
- `link`: source URL

The index order is the track order.

## Problem Metadata

Each metadata file can include problem details, tags, method info, examples, and statement text:

```json
{
  "id": 200,
  "title": "Number of Islands",
  "link": "https://leetcode.com/problems/number-of-islands/",
  "category": "Tier 1: DFS/BFS Foundations",
  "subcategory": "Grid components",
  "leetcode": {
    "title_slug": "number-of-islands",
    "difficulty": "Medium",
    "topic_tags": [
      { "name": "Depth-First Search", "slug": "depth-first-search" },
      { "name": "Matrix", "slug": "matrix" }
    ]
  },
  "question": {
    "content_text": "Local statement text..."
  }
}
```

LazyLeet currently reads:

- `id`
- `title`
- `link`
- `category`
- `subcategory`
- `leetcode.title_slug`
- `leetcode.difficulty`
- `leetcode.topic_tags[].name`
- `question.content_text`

Extra fields are allowed and ignored by the catalog loader.

## Test Cases

Test cases live in `tests/`. The testcase filename is the metadata filename without its extension:

```text
metadata/200_Number_of_Islands.json
tests/200_Number_of_Islands
```

The testcase file is newline-delimited JSON. Each non-empty line is one testcase:

```jsonl
{"input":{"grid":[["1"]]},"expected":1,"comment":"single land"}
{"input":{"grid":[["0"]]},"expected":0,"comment":"single water"}
```

Required fields:

- `input`: object keyed by method parameter name
- `expected`: expected return value

Optional fields:

- `comment`: human-readable testcase name or note

This format is intentionally simple so future runners can stream large case files without loading them fully into memory.
