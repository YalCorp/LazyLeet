package workspace

import (
	"encoding/json"
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

func TestDataPackTestCasesReadNDJSON(t *testing.T) {
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
  {"id": 1971, "title": "Find if Path Exists in Graph", "file": "1971_Find_if_Path_Exists_in_Graph.json", "link": "https://leetcode.com/problems/find-if-path-exists-in-graph/"}
]`)
	writeMetadataFile(t, filepath.Join(testsDir, "1971_Find_if_Path_Exists_in_Graph"), strings.Join([]string{
		`{"input":{"n":3,"edges":[[0,1]],"source":0,"destination":1},"expected":true,"comment":"connected"}`,
		`{"input":{"n":3,"edges":[],"source":0,"destination":2},"expected":false,"comment":"disconnected"}`,
	}, "\n"))

	store := NewDataPackTestCases([]catalog.DataPack{{
		Slug:        "graph-tiers",
		MetadataDir: metadataDir,
		TestsDir:    testsDir,
	}})
	cases, path, err := store.ReadTestCases(catalog.Problem{
		ID:     1971,
		Slug:   "find-if-path-exists-in-graph",
		Tracks: []string{"graph-tiers"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "1971_Find_if_Path_Exists_in_Graph") {
		t.Fatalf("path = %q", path)
	}
	if len(cases) != 2 {
		t.Fatalf("cases = %d, want 2", len(cases))
	}
	if cases[0].Comment != "connected" || string(cases[0].Expected) != "true" {
		t.Fatalf("first case = %#v", cases[0])
	}
}

func TestJavaHarnessUsesMethodMetadataAndCaseInputs(t *testing.T) {
	problem := catalog.Problem{
		Slug: "find-if-path-exists-in-graph",
		Method: catalog.Method{
			Name:       "validPath",
			ReturnType: "boolean",
			Params: []catalog.MethodParam{
				{Name: "n", Type: "integer"},
				{Name: "edges", Type: "integer[][]"},
				{Name: "source", Type: "integer"},
				{Name: "destination", Type: "integer"},
			},
		},
	}
	cases := []TestCase{{
		Input: map[string]json.RawMessage{
			"n":           json.RawMessage(`3`),
			"edges":       json.RawMessage(`[[0,1],[1,2]]`),
			"source":      json.RawMessage(`0`),
			"destination": json.RawMessage(`2`),
		},
		Expected: json.RawMessage(`true`),
		Comment:  "connected",
	}}
	harness, err := javaHarness(problem, cases)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"s.validPath(3, new int[][]{new int[]{0, 1}, new int[]{1, 2}}, 0, 2)",
		"check(1, \"connected\"",
		"LAZYLEET_RESULT",
	} {
		if !strings.Contains(harness, want) {
			t.Fatalf("harness missing %q:\n%s", want, harness)
		}
	}
}
