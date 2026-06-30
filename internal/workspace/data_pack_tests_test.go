package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YalCorp/LazyLeet/internal/catalog"
)

func TestDataPackTestCasesCount(t *testing.T) {
	root := t.TempDir()
	metadataDir := filepath.Join(root, "graph-tiers-metadata")
	testsDir := filepath.Join(root, "graph-tiers")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeMetadataFile(t, filepath.Join(metadataDir, "index.json"), `[
  {"id": 200, "title": "Number of Islands", "file": "200_Number_of_Islands.json", "link": "https://leetcode.com/problems/number-of-islands/"}
]`)
	writeMetadataFile(t, filepath.Join(testsDir, "200_Number_of_Islands"), strings.Join([]string{
		`{"input":{"grid":[["1"]]},"expected":1,"comment":"single land"}`,
		`{"input":{"grid":[["0"]]},"expected":0,"comment":"single water"}`,
		``,
	}, "\n"))

	store := NewDataPackTestCases([]catalog.DataPack{{
		Slug:        "graph-tiers",
		MetadataDir: metadataDir,
		TestsDir:    testsDir,
	}})
	count, path, err := store.CountTestCases(catalog.Problem{
		ID:     200,
		Slug:   "number-of-islands",
		Tracks: []string{"graph-tiers"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if !strings.HasSuffix(path, "200_Number_of_Islands") {
		t.Fatalf("path = %q", path)
	}
}
