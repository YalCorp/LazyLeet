package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YalCorp/LazyLeet/internal/catalog"
)

func TestMetadataStatementsReadStatement(t *testing.T) {
	dir := t.TempDir()
	writeMetadataFile(t, filepath.Join(dir, "index.json"), `[
  {"id": 200, "title": "Number of Islands", "file": "200_Number_of_Islands.json", "link": "https://leetcode.com/problems/number-of-islands/"}
]`)
	writeMetadataFile(t, filepath.Join(dir, "200_Number_of_Islands.json"), `{
  "title": "Number of Islands",
  "category": "Tier 1: DFS/BFS Foundations",
  "subcategory": "Grid components",
  "question": {
    "content_html": "<p>Count <strong>connected</strong> and <em>separate</em> land components using <code>grid</code>.</p>\n\n<p><strong class=\"example\">Example 1:</strong></p><pre>Input: grid = [[\"1\"]]\nOutput: 1\nExplanation: one island exists.</pre><p><strong>Constraints:</strong></p>\n\n<ul>\n\t<li>Grid contains water or land.</li>\n\t<li>Grid is non-empty.</li>\n</ul>",
    "content_text": "Count connected land components in a binary grid."
  }
}`)

	store := NewMetadataStatements([]catalog.DataPack{{
		Slug:        "graph-tiers",
		MetadataDir: dir,
	}})
	content, path, err := store.ReadStatement(catalog.Problem{
		ID:     200,
		Slug:   "number-of-islands",
		Title:  "Number of Islands",
		URL:    "https://leetcode.com/problems/number-of-islands/",
		Tracks: []string{"graph-tiers"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "200_Number_of_Islands.json") {
		t.Fatalf("path = %q", path)
	}
	for _, want := range []string{
		"Count \x1b[1mconnected\x1b[0m and \x1b[3mseparate\x1b[0m land components using \x1b[36mgrid\x1b[0m.",
		"\n\n\x1b[1;38;2;235;203;139mExample 1:\x1b[0m",
		"\x1b[36mInput: grid = [[\"1\"]]\x1b[0m",
		"\x1b[36mOutput: 1\x1b[0m",
		"\x1b[36mExplanation: one island exists.\x1b[0m",
		"\n\n\x1b[1;38;2;235;203;139mConstraints:\x1b[0m\n- Grid contains water or land.\n- Grid is non-empty.",
		"- Grid contains water or land.",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("statement missing %q:\n%s", want, content)
		}
	}
	for _, unwanted := range []string{"# Number of Islands", "Tier 1: DFS/BFS Foundations", "\n\n\n"} {
		if strings.Contains(content, unwanted) {
			t.Fatalf("statement contains unwanted %q:\n%s", unwanted, content)
		}
	}
	if strings.Contains(content, "- Grid contains water or land.\n\n- Grid is non-empty.") {
		t.Fatalf("constraints should not have blank lines between items:\n%s", content)
	}
}

func writeMetadataFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
