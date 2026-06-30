package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverAndLoadDataPacks(t *testing.T) {
	root := t.TempDir()
	metadataDir := filepath.Join(root, "graph-tiers-metadata")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(metadataDir, "index.json"), `[
  {"id": 200, "title": "Number of Islands", "file": "200_Number_of_Islands.json", "link": "https://leetcode.com/problems/number-of-islands/"}
]`)
	writeFile(t, filepath.Join(metadataDir, "200_Number_of_Islands.json"), `{
  "id": 200,
  "title": "Number of Islands",
  "link": "https://leetcode.com/problems/number-of-islands/",
  "category": "Tier 1: DFS/BFS Foundations",
  "subcategory": "Grid components",
  "leetcode": {
    "title_slug": "number-of-islands",
    "difficulty": "Medium",
    "topic_tags": [{"name": "Depth-First Search"}, {"name": "Matrix"}]
  },
  "method": {
    "all_code_snippets": [
      {
        "lang": "Java",
        "langSlug": "java",
        "code": "class Solution {\n    public int numIslands(char[][] grid) {\n        \n    }\n}"
      }
    ]
  }
}`)

	packs, err := DiscoverDataPacks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packs) != 1 || packs[0].Slug != "graph-tiers" {
		t.Fatalf("packs = %#v", packs)
	}

	c, err := LoadDataPacks(packs)
	if err != nil {
		t.Fatal(err)
	}
	track, ok := c.Track("graph-tiers")
	if !ok {
		t.Fatal("graph-tiers track missing")
	}
	if len(track.Problems) != 1 || track.Problems[0] != "number-of-islands" {
		t.Fatalf("track problems = %#v", track.Problems)
	}
	problem, ok := c.Problem("number-of-islands")
	if !ok {
		t.Fatal("number-of-islands missing")
	}
	if problem.ID != 200 || problem.Difficulty != Medium {
		t.Fatalf("problem = %#v", problem)
	}
	if len(problem.Tags) != 4 {
		t.Fatalf("tags = %#v, want category, subcategory, and topic tags", problem.Tags)
	}
	if len(problem.TopicTags) != 2 {
		t.Fatalf("topic tags = %#v, want only LeetCode topic tags", problem.TopicTags)
	}
	for _, unwanted := range []string{"Tier 1: DFS/BFS Foundations", "Grid components"} {
		for _, tag := range problem.TopicTags {
			if tag == unwanted {
				t.Fatalf("topic tags include non-topic tag %q: %#v", unwanted, problem.TopicTags)
			}
		}
	}
	if len(problem.Snippets) != 1 || problem.Snippets[0].LangSlug != "java" {
		t.Fatalf("snippets = %#v", problem.Snippets)
	}
	if !strings.Contains(problem.Snippets[0].Code, "numIslands") {
		t.Fatalf("snippet code = %q", problem.Snippets[0].Code)
	}
}

func TestDiscoverManifestDataPack(t *testing.T) {
	root := t.TempDir()
	packDir := filepath.Join(root, "graph-tiers")
	metadataDir := filepath.Join(packDir, "metadata")
	testsDir := filepath.Join(packDir, "tests")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(packDir, "lazyleet-pack.toml"), `slug = "graph-tiers"
title = "Graph Tiers"
version = "0.1.0"
description = "Graph practice pack."
metadata_dir = "metadata"
tests_dir = "tests"
`)
	writeFile(t, filepath.Join(metadataDir, "index.json"), `[
  {"id": 1971, "title": "Find if Path Exists in Graph", "file": "1971_Find_if_Path_Exists_in_Graph.json", "link": "https://leetcode.com/problems/find-if-path-exists-in-graph/"}
]`)
	writeFile(t, filepath.Join(metadataDir, "1971_Find_if_Path_Exists_in_Graph.json"), `{
  "id": 1971,
  "title": "Find if Path Exists in Graph",
  "link": "https://leetcode.com/problems/find-if-path-exists-in-graph/",
  "leetcode": {
    "title_slug": "find-if-path-exists-in-graph",
    "difficulty": "Easy",
    "topic_tags": [{"name": "Graph"}]
  }
}`)

	packs, err := DiscoverDataPacks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(packs) != 1 {
		t.Fatalf("packs = %#v", packs)
	}
	pack := packs[0]
	if pack.Slug != "graph-tiers" || pack.Title != "Graph Tiers" || pack.Version != "0.1.0" {
		t.Fatalf("pack = %#v", pack)
	}
	if pack.MetadataDir != metadataDir || pack.TestsDir != testsDir {
		t.Fatalf("pack dirs = %#v", pack)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
