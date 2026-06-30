package workspace

import (
	"os"
	"strings"
	"testing"

	"github.com/YalCorp/LazyLeet/internal/catalog"
)

func TestReadSolutionReturnsPythonTemplateByDefault(t *testing.T) {
	store := New(t.TempDir())
	problem := testProblem()

	content, path, err := store.ReadSolution(problem, DefaultLanguage())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "1_two-sum/solution.py") {
		t.Fatalf("path = %q", path)
	}
	for _, want := range []string{"Problem: Two Sum", "ID: 1", "URL: https://leetcode.com/problems/two-sum/", "Tags: Array, Hash Table"} {
		if !strings.Contains(content, want) {
			t.Fatalf("template missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "Difficulty:") {
		t.Fatalf("solution template should not include difficulty:\n%s", content)
	}
}

func TestSupportedLanguageTemplates(t *testing.T) {
	store := New(t.TempDir())
	problem := testProblem()

	tests := map[string]string{
		"python": "class Solution:",
		"go":     "package main",
		"java":   "class Solution",
	}
	for _, language := range SupportedLanguages() {
		content, path, err := store.ReadSolution(problem, language)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(path, language.Filename) {
			t.Fatalf("%s path = %q, want suffix %q", language.ID, path, language.Filename)
		}
		if !strings.Contains(content, tests[language.ID]) {
			t.Fatalf("%s template missing %q:\n%s", language.ID, tests[language.ID], content)
		}
	}
}

func TestReadSolutionUsesProblemCodeSnippet(t *testing.T) {
	store := New(t.TempDir())
	problem := testProblem()
	problem.Tags = []string{"Tier 1: DFS/BFS Foundations", "Grid components", "Array", "Matrix"}
	problem.TopicTags = []string{"Array", "Matrix"}
	problem.Snippets = []catalog.CodeSnippet{
		{
			Lang:     "Java",
			LangSlug: "java",
			Code:     "class Solution {\n    public int[] twoSum(int[] nums, int target) {\n        \n    }\n}",
		},
		{
			Lang:     "Python3",
			LangSlug: "python3",
			Code:     "class Solution:\n    def twoSum(self, nums: List[int], target: int) -> List[int]:\n        ",
		},
	}

	javaContent, _, err := store.ReadSolution(problem, Language{ID: "java"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(javaContent, "public int[] twoSum(int[] nums, int target)") {
		t.Fatalf("java snippet missing method:\n%s", javaContent)
	}
	if strings.Contains(javaContent, "Write your solution here") {
		t.Fatalf("java snippet template still contains fallback text:\n%s", javaContent)
	}
	if strings.Contains(javaContent, "Tier 1") || strings.Contains(javaContent, "Grid components") {
		t.Fatalf("java snippet template includes non-topic tags:\n%s", javaContent)
	}
	if !strings.Contains(javaContent, "Tags: Array, Matrix") {
		t.Fatalf("java snippet template missing topic tags:\n%s", javaContent)
	}
	if strings.Contains(javaContent, "Difficulty:") {
		t.Fatalf("java snippet template should not include difficulty:\n%s", javaContent)
	}

	pythonContent, _, err := store.ReadSolution(problem, Language{ID: "python"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pythonContent, "def twoSum(self, nums: List[int], target: int) -> List[int]:") {
		t.Fatalf("python snippet missing method:\n%s", pythonContent)
	}
}

func TestSaveAndReadSolution(t *testing.T) {
	store := New(t.TempDir())
	problem := testProblem()
	language := Language{ID: "java"}
	want := "class Solution {}\n"

	path, err := store.SaveSolution(problem, language, want)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	got, gotPath, err := store.ReadSolution(problem, language)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestReadStatementReturnsLocalScaffold(t *testing.T) {
	store := New(t.TempDir())
	problem := testProblem()

	content, path, err := store.ReadStatement(problem)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "1_two-sum/statement.md") {
		t.Fatalf("path = %q", path)
	}
	for _, want := range []string{"# Two Sum", "LazyLeet does not bundle LeetCode problem statements", problem.URL} {
		if !strings.Contains(content, want) {
			t.Fatalf("statement scaffold missing %q:\n%s", want, content)
		}
	}
}

func TestSaveAndReadStatement(t *testing.T) {
	store := New(t.TempDir())
	problem := testProblem()
	want := "# Local prompt\n\nUser-authored statement."

	path, err := store.SaveStatement(problem, want)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	got, gotPath, err := store.ReadStatement(problem)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func testProblem() catalog.Problem {
	return catalog.Problem{
		ID:         1,
		Slug:       "two-sum",
		Title:      "Two Sum",
		Difficulty: catalog.Easy,
		URL:        "https://leetcode.com/problems/two-sum/",
		Tags:       []string{"Hash Table", "Array"},
		TopicTags:  []string{"Hash Table", "Array"},
	}
}
