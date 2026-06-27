package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/YalCorp/LazyLeet/internal/catalog"
)

type Language struct {
	ID       string
	Title    string
	Filename string
}

var supportedLanguages = []Language{
	{ID: "python", Title: "Python", Filename: "solution.py"},
	{ID: "go", Title: "Go", Filename: "solution.go"},
	{ID: "java", Title: "Java", Filename: "Solution.java"},
}

func DefaultLanguage() Language {
	return supportedLanguages[0]
}

func SupportedLanguages() []Language {
	return append([]Language(nil), supportedLanguages...)
}

func NextLanguage(current Language) Language {
	languages := SupportedLanguages()
	for i, language := range languages {
		if language.ID == current.ID {
			return languages[(i+1)%len(languages)]
		}
	}
	return DefaultLanguage()
}

type Store struct {
	root string
}

func New(root string) Store {
	return Store{root: root}
}

func (s Store) ReadSolution(problem catalog.Problem, language Language) (content string, path string, err error) {
	language = normalizeLanguage(language)
	path = s.SolutionPath(problem, language)
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), path, nil
	}
	if os.IsNotExist(err) {
		return solutionTemplate(problem, language), path, nil
	}
	return "", path, err
}

func (s Store) SaveSolution(problem catalog.Problem, language Language, content string) (string, error) {
	language = normalizeLanguage(language)
	path := s.SolutionPath(problem, language)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, err
	}
	return path, os.WriteFile(path, []byte(content), 0o644)
}

func (s Store) SolutionPath(problem catalog.Problem, language Language) string {
	language = normalizeLanguage(language)
	return filepath.Join(s.root, problem.Slug, language.Filename)
}

func (s Store) ReadStatement(problem catalog.Problem) (content string, path string, err error) {
	path = s.StatementPath(problem)
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), path, nil
	}
	if os.IsNotExist(err) {
		return statementTemplate(problem), path, nil
	}
	return "", path, err
}

func (s Store) SaveStatement(problem catalog.Problem, content string) (string, error) {
	path := s.StatementPath(problem)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, err
	}
	return path, os.WriteFile(path, []byte(content), 0o644)
}

func (s Store) StatementPath(problem catalog.Problem) string {
	return filepath.Join(s.root, problem.Slug, "statement.md")
}

func normalizeLanguage(language Language) Language {
	for _, supported := range supportedLanguages {
		if supported.ID == language.ID {
			return supported
		}
	}
	return DefaultLanguage()
}

func solutionTemplate(problem catalog.Problem, language Language) string {
	tags := append([]string(nil), problem.Tags...)
	sort.Strings(tags)
	metadata := fmt.Sprintf("Problem: %s\nURL: %s\nDifficulty: %s\nTags: %s", problem.Title, problem.URL, problem.Difficulty, strings.Join(tags, ", "))
	switch normalizeLanguage(language).ID {
	case "go":
		return fmt.Sprintf(`package main

// %s
//
// Write your solution here. LazyLeet stores this file locally and does not
// bundle LeetCode statements, examples, or editorials.

func main() {
}
`, strings.ReplaceAll(metadata, "\n", "\n// "))
	case "java":
		return fmt.Sprintf(`// %s
//
// Write your solution here. LazyLeet stores this file locally and does not
// bundle LeetCode statements, examples, or editorials.

class Solution {
}
`, strings.ReplaceAll(metadata, "\n", "\n// "))
	default:
		return fmt.Sprintf(`"""
%s

Write your solution here. LazyLeet stores this file locally and does not
bundle LeetCode statements, examples, or editorials.
"""


class Solution:
    pass
`, metadata)
	}
}

func statementTemplate(problem catalog.Problem) string {
	tags := append([]string(nil), problem.Tags...)
	sort.Strings(tags)
	return fmt.Sprintf(`# %s

- URL: %s
- Difficulty: %s
- Tags: %s

LazyLeet does not bundle LeetCode problem statements.

Paste or write your own local statement, constraints, and examples here if you want them available in the TUI.
`, problem.Title, problem.URL, problem.Difficulty, strings.Join(tags, ", "))
}
