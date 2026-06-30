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

func TestDataPackTestCasesSelectsExamplesFromStatement(t *testing.T) {
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
	writeMetadataFile(t, filepath.Join(metadataDir, "1971_Find_if_Path_Exists_in_Graph.json"), `{
  "question": {
    "content_html": "<p>Example 1:</p><pre>Input: n = 3, edges = [[0,1]], source = 0, destination = 1\nOutput: true</pre><p>Example 2:</p><pre>Input: n = 3, edges = [], source = 0, destination = 2\nOutput: false</pre>"
  }
}`)
	writeMetadataFile(t, filepath.Join(testsDir, "1971_Find_if_Path_Exists_in_Graph"), strings.Join([]string{
		`{"input":{"n":3,"edges":[[0,1]],"source":0,"destination":1},"expected":true,"comment":"example 1"}`,
		`{"input":{"n":3,"edges":[],"source":0,"destination":2},"expected":false,"comment":"example 2"}`,
		`{"input":{"n":4,"edges":[[0,1],[2,3]],"source":0,"destination":3},"expected":false,"comment":"hidden"}`,
	}, "\n"))

	store := NewDataPackTestCases([]catalog.DataPack{{
		Slug:        "graph-tiers",
		MetadataDir: metadataDir,
		TestsDir:    testsDir,
	}})
	cases, _, err := store.ReadTestCases(catalog.Problem{
		ID:     1971,
		Slug:   "find-if-path-exists-in-graph",
		Tracks: []string{"graph-tiers"},
		Method: catalog.Method{
			Params: []catalog.MethodParam{
				{Name: "n", Type: "integer"},
				{Name: "edges", Type: "integer[][]"},
				{Name: "source", Type: "integer"},
				{Name: "destination", Type: "integer"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	examples, err := store.selectTestCases(catalog.Problem{
		ID:     1971,
		Slug:   "find-if-path-exists-in-graph",
		Tracks: []string{"graph-tiers"},
		Method: catalog.Method{
			Params: []catalog.MethodParam{
				{Name: "n", Type: "integer"},
				{Name: "edges", Type: "integer[][]"},
				{Name: "source", Type: "integer"},
				{Name: "destination", Type: "integer"},
			},
		},
	}, cases, TestRunRequest{Mode: TestRunExamples})
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) != 2 {
		t.Fatalf("examples = %d, want 2", len(examples))
	}
	if examples[0].Comment != "example 1" || examples[1].Comment != "example 2" {
		t.Fatalf("examples = %#v", examples)
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
	caseData, err := javaCasesJSONLines(problem, cases)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(caseData), `"summary":"n = 3, edges = [[0,1],[1,2]], source = 0, destination = 2"`) {
		t.Fatalf("case data missing input summary:\n%s", caseData)
	}
	harness, err := javaHarness(problem)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Files.readAllLines(Paths.get(\"cases.jsonl\"), StandardCharsets.UTF_8)",
		"return s.validPath(((Integer) c.input.get(\"n\")), ((int[][]) c.input.get(\"edges\")), ((Integer) c.input.get(\"source\")), ((Integer) c.input.get(\"destination\")))",
		"c.input.put(\"edges\", readValue(\"integer[][]\", field(inputJson, \"edges\"), false))",
		"LAZYLEET_RESULT",
	} {
		if !strings.Contains(harness, want) {
			t.Fatalf("harness missing %q:\n%s", want, harness)
		}
	}
}

func TestParseJavaFailureIncludesInputOutputAndExpected(t *testing.T) {
	failure := parseJavaFailure("LAZYLEET_FAIL\t2\tdisconnected\tn = 3, edges = []\tfalse\ttrue")
	if failure.Index != 2 {
		t.Fatalf("index = %d, want 2", failure.Index)
	}
	if failure.Comment != "disconnected" {
		t.Fatalf("comment = %q", failure.Comment)
	}
	if failure.Input != "n = 3, edges = []" || failure.Expected != "false" || failure.Actual != "true" {
		t.Fatalf("failure = %#v", failure)
	}
}

func TestRunJavaTestCasesProvidesLeetcodeImports(t *testing.T) {
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
	cases := []TestCase{
		{
			Input: map[string]json.RawMessage{
				"n":           json.RawMessage(`3`),
				"edges":       json.RawMessage(`[[0,1],[1,2]]`),
				"source":      json.RawMessage(`0`),
				"destination": json.RawMessage(`2`),
			},
			Expected: json.RawMessage(`true`),
			Comment:  "connected",
		},
		{
			Input: map[string]json.RawMessage{
				"n":           json.RawMessage(`3`),
				"edges":       json.RawMessage(`[[0,1]]`),
				"source":      json.RawMessage(`0`),
				"destination": json.RawMessage(`2`),
			},
			Expected: json.RawMessage(`false`),
			Comment:  "disconnected",
		},
	}
	solution := `class Solution {
    public boolean validPath(int n, int[][] edges, int source, int destination) {
        Map<Integer, List<Integer>> graph = new HashMap<>();
        for (int[] edge : edges) {
            graph.computeIfAbsent(edge[0], key -> new ArrayList<>()).add(edge[1]);
            graph.computeIfAbsent(edge[1], key -> new ArrayList<>()).add(edge[0]);
        }
        boolean[] seen = new boolean[n];
        Deque<Integer> queue = new ArrayDeque<>();
        queue.add(source);
        seen[source] = true;
        while (!queue.isEmpty()) {
            int node = queue.removeFirst();
            if (node == destination) {
                return true;
            }
            for (int next : graph.getOrDefault(node, Collections.emptyList())) {
                if (!seen[next]) {
                    seen[next] = true;
                    queue.addLast(next);
                }
            }
        }
        return false;
    }
}`

	result, err := runJavaTestCases(problem, solution, cases)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed != 2 || result.Total != 2 {
		t.Fatalf("result = %#v, want 2/2", result)
	}
}
