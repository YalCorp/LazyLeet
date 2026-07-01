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

func LanguageByID(id string) (Language, bool) {
	for _, language := range supportedLanguages {
		if language.ID == id {
			return language, true
		}
	}
	return Language{}, false
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
		return sanitizeSolutionMetadata(string(data)), path, nil
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
	return filepath.Join(s.root, problemWorkspaceDir(problem), language.Filename)
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
	return filepath.Join(s.root, problemWorkspaceDir(problem), "statement.md")
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
	tags := solutionTags(problem)
	sort.Strings(tags)
	metadata := fmt.Sprintf("Problem: %s\nTags: %s", problem.Title, strings.Join(tags, ", "))
	if snippet, ok := problemSnippet(problem, language); ok {
		return fmt.Sprintf("%s\n\n%s\n", languageCommentBlock(metadata, language), snippet)
	}
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

func sanitizeSolutionMetadata(content string) string {
	lines := strings.Split(content, "\n")
	searchLimit := len(lines)
	if searchLimit > 30 {
		searchLimit = 30
	}
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if i < searchLimit && isLegacySolutionMetadataLine(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func isLegacySolutionMetadataLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
	return strings.HasPrefix(trimmed, "ID:") || strings.HasPrefix(trimmed, "URL:")
}

func problemWorkspaceDir(problem catalog.Problem) string {
	if problem.ID > 0 {
		return fmt.Sprintf("%d_%s", problem.ID, problem.Slug)
	}
	return problem.Slug
}

func solutionTags(problem catalog.Problem) []string {
	if len(problem.TopicTags) > 0 {
		return append([]string(nil), problem.TopicTags...)
	}
	return append([]string(nil), problem.Tags...)
}

func problemSnippet(problem catalog.Problem, language Language) (string, bool) {
	langSlugs := snippetLangSlugs(normalizeLanguage(language))
	for _, langSlug := range langSlugs {
		for _, snippet := range problem.Snippets {
			if strings.EqualFold(snippet.LangSlug, langSlug) && strings.TrimSpace(snippet.Code) != "" {
				return strings.TrimRight(snippet.Code, " \t\n\r"), true
			}
		}
	}
	return "", false
}

func snippetLangSlugs(language Language) []string {
	switch language.ID {
	case "python":
		return []string{"python3", "python"}
	case "go":
		return []string{"golang", "go"}
	case "java":
		return []string{"java"}
	default:
		return []string{language.ID}
	}
}

func languageCommentBlock(metadata string, language Language) string {
	switch normalizeLanguage(language).ID {
	case "python":
		return fmt.Sprintf(`"""
%s
"""`, metadata)
	default:
		return "// " + strings.ReplaceAll(metadata, "\n", "\n// ")
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
