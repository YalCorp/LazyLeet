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
	if !strings.HasSuffix(path, "two-sum/solution.py") {
		t.Fatalf("path = %q", path)
	}
	for _, want := range []string{"Problem: Two Sum", "URL: https://leetcode.com/problems/two-sum/", "Difficulty: Easy"} {
		if !strings.Contains(content, want) {
			t.Fatalf("template missing %q:\n%s", want, content)
		}
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
	if !strings.HasSuffix(path, "two-sum/statement.md") {
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
	}
}
