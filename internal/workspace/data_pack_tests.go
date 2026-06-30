package workspace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/YalCorp/LazyLeet/internal/catalog"
)

type DataPackTestCases struct {
	packs map[string]catalog.DataPack
}

type TestCase struct {
	Input    map[string]json.RawMessage `json:"input"`
	Expected json.RawMessage            `json:"expected"`
	Comment  string                     `json:"comment"`
}

type TestRunResult struct {
	Passed   int
	Total    int
	Failures []TestFailure
	Output   string
}

type TestFailure struct {
	Index    int
	Comment  string
	Expected string
	Actual   string
}

func NewDataPackTestCases(packs []catalog.DataPack) DataPackTestCases {
	bySlug := make(map[string]catalog.DataPack, len(packs))
	for _, pack := range packs {
		bySlug[pack.Slug] = pack
	}
	return DataPackTestCases{packs: bySlug}
}

func (s DataPackTestCases) CountTestCases(problem catalog.Problem) (int, string, error) {
	path, err := s.testCasePath(problem)
	if err != nil {
		return 0, "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, path, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024*64)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, path, err
	}
	return count, path, nil
}

func (s DataPackTestCases) RunTestCases(problem catalog.Problem, language Language, solution string) (TestRunResult, error) {
	cases, _, err := s.ReadTestCases(problem)
	if err != nil {
		return TestRunResult{}, err
	}
	if len(cases) == 0 {
		return TestRunResult{}, fmt.Errorf("no test cases found for %s", problem.Slug)
	}
	switch normalizeLanguage(language).ID {
	case "java":
		return runJavaTestCases(problem, solution, cases)
	default:
		return TestRunResult{}, fmt.Errorf("running %s solutions is not implemented yet", language.Title)
	}
}

func (s DataPackTestCases) ReadTestCases(problem catalog.Problem) ([]TestCase, string, error) {
	path, err := s.testCasePath(problem)
	if err != nil {
		return nil, "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, path, err
	}
	defer file.Close()

	var cases []TestCase
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024*64)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var tc TestCase
		if err := json.Unmarshal([]byte(line), &tc); err != nil {
			return nil, path, fmt.Errorf("parse %s line %d: %w", path, lineNo, err)
		}
		cases = append(cases, tc)
	}
	if err := scanner.Err(); err != nil {
		return nil, path, err
	}
	return cases, path, nil
}

func runJavaTestCases(problem catalog.Problem, solution string, cases []TestCase) (TestRunResult, error) {
	if problem.Method.Name == "" || len(problem.Method.Params) == 0 {
		return TestRunResult{}, fmt.Errorf("method metadata is missing for %s", problem.Slug)
	}
	dir, err := os.MkdirTemp("", "lazyleet-java-*")
	if err != nil {
		return TestRunResult{}, err
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "Solution.java"), []byte(solution), 0o644); err != nil {
		return TestRunResult{}, err
	}
	harness, err := javaHarness(problem, cases)
	if err != nil {
		return TestRunResult{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "Main.java"), []byte(harness), 0o644); err != nil {
		return TestRunResult{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	compile := exec.CommandContext(ctx, "javac", "Solution.java", "Main.java")
	compile.Dir = dir
	compileOut, err := compile.CombinedOutput()
	if err != nil {
		return TestRunResult{}, fmt.Errorf("javac failed:\n%s", strings.TrimSpace(string(compileOut)))
	}

	runCtx, runCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer runCancel()
	run := exec.CommandContext(runCtx, "java", "-cp", dir, "Main")
	output, err := run.CombinedOutput()
	result := parseJavaRunOutput(string(output), len(cases))
	result.Output = strings.TrimSpace(string(output))
	if err != nil {
		if result.Total > 0 {
			return result, nil
		}
		return TestRunResult{}, fmt.Errorf("java failed:\n%s", strings.TrimSpace(string(output)))
	}
	return result, nil
}

func javaHarness(problem catalog.Problem, cases []TestCase) (string, error) {
	var b strings.Builder
	b.WriteString("import java.util.*;\n\n")
	b.WriteString("class Main {\n")
	b.WriteString("  static int passed = 0;\n")
	b.WriteString("  static int total = 0;\n")
	b.WriteString("  public static void main(String[] args) {\n")
	b.WriteString("    Solution s = new Solution();\n")
	for i, tc := range cases {
		args, err := javaCallArgs(problem.Method.Params, tc.Input)
		if err != nil {
			return "", fmt.Errorf("case %d: %w", i+1, err)
		}
		expected, err := javaExpectedLiteral(problem.Method.ReturnType, tc.Expected)
		if err != nil {
			return "", fmt.Errorf("case %d expected: %w", i+1, err)
		}
		comment := javaStringLiteral(tc.Comment)
		b.WriteString(fmt.Sprintf("    check(%d, %s, s.%s(%s), %s);\n", i+1, comment, problem.Method.Name, strings.Join(args, ", "), expected))
	}
	b.WriteString("    System.out.println(\"LAZYLEET_RESULT \" + passed + \" \" + total);\n")
	b.WriteString("  }\n")
	b.WriteString("  static void check(int index, String comment, Object actual, Object expected) {\n")
	b.WriteString("    total++;\n")
	b.WriteString("    if (Objects.deepEquals(actual, expected)) { passed++; return; }\n")
	b.WriteString("    System.out.println(\"LAZYLEET_FAIL \" + index + \" \" + comment + \" expected=\" + render(expected) + \" actual=\" + render(actual));\n")
	b.WriteString("  }\n")
	b.WriteString("  static String render(Object value) {\n")
	b.WriteString("    if (value == null) return \"null\";\n")
	b.WriteString("    Class<?> c = value.getClass();\n")
	b.WriteString("    if (!c.isArray()) return String.valueOf(value);\n")
	b.WriteString("    if (value instanceof int[]) return Arrays.toString((int[]) value);\n")
	b.WriteString("    if (value instanceof boolean[]) return Arrays.toString((boolean[]) value);\n")
	b.WriteString("    if (value instanceof char[]) return Arrays.toString((char[]) value);\n")
	b.WriteString("    return Arrays.deepToString((Object[]) value);\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String(), nil
}

func javaCallArgs(params []catalog.MethodParam, input map[string]json.RawMessage) ([]string, error) {
	args := make([]string, 0, len(params))
	for _, param := range params {
		raw, ok := input[param.Name]
		if !ok {
			return nil, fmt.Errorf("input parameter %q missing", param.Name)
		}
		value, err := javaValueLiteral(param.Type, raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", param.Name, err)
		}
		args = append(args, value)
	}
	return args, nil
}

func javaValueLiteral(kind string, raw json.RawMessage) (string, error) {
	switch strings.TrimSpace(kind) {
	case "integer":
		var value int
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		return fmt.Sprintf("%d", value), nil
	case "boolean":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		if value {
			return "true", nil
		}
		return "false", nil
	case "integer[][]":
		var value [][]int
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		return javaIntMatrixLiteral(value), nil
	case "character[][]":
		var value [][]string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		return javaCharMatrixLiteral(value), nil
	default:
		return "", fmt.Errorf("unsupported type %q", kind)
	}
}

func javaExpectedLiteral(kind string, raw json.RawMessage) (string, error) {
	if strings.TrimSpace(kind) == "integer[][]" {
		var value [][]int
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		return javaIntegerListMatrixLiteral(value), nil
	}
	return javaValueLiteral(kind, raw)
}

func javaIntMatrixLiteral(value [][]int) string {
	rows := make([]string, 0, len(value))
	for _, row := range value {
		items := make([]string, 0, len(row))
		for _, item := range row {
			items = append(items, fmt.Sprintf("%d", item))
		}
		rows = append(rows, "new int[]{"+strings.Join(items, ", ")+"}")
	}
	return "new int[][]{" + strings.Join(rows, ", ") + "}"
}

func javaCharMatrixLiteral(value [][]string) string {
	rows := make([]string, 0, len(value))
	for _, row := range value {
		items := make([]string, 0, len(row))
		for _, item := range row {
			runes := []rune(item)
			if len(runes) != 1 {
				items = append(items, "'\\0'")
				continue
			}
			items = append(items, fmt.Sprintf("'%s'", escapeJavaChar(runes[0])))
		}
		rows = append(rows, "new char[]{"+strings.Join(items, ", ")+"}")
	}
	return "new char[][]{" + strings.Join(rows, ", ") + "}"
}

func javaIntegerListMatrixLiteral(value [][]int) string {
	rows := make([]string, 0, len(value))
	for _, row := range value {
		items := make([]string, 0, len(row))
		for _, item := range row {
			items = append(items, fmt.Sprintf("%d", item))
		}
		rows = append(rows, "Arrays.asList("+strings.Join(items, ", ")+")")
	}
	return "Arrays.asList(" + strings.Join(rows, ", ") + ")"
}

func escapeJavaChar(r rune) string {
	switch r {
	case '\'':
		return "\\'"
	case '\\':
		return "\\\\"
	case '\n':
		return "\\n"
	case '\r':
		return "\\r"
	case '\t':
		return "\\t"
	default:
		return string(r)
	}
}

func javaStringLiteral(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func parseJavaRunOutput(output string, total int) TestRunResult {
	result := TestRunResult{Total: total}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "LAZYLEET_RESULT ") {
			fmt.Sscanf(line, "LAZYLEET_RESULT %d %d", &result.Passed, &result.Total)
			continue
		}
		if strings.HasPrefix(line, "LAZYLEET_FAIL ") {
			result.Failures = append(result.Failures, parseJavaFailure(line))
		}
	}
	if result.Total == 0 {
		result.Total = total
	}
	if result.Passed == 0 && len(result.Failures) == 0 && bytes.Contains([]byte(output), []byte("LAZYLEET_RESULT")) {
		return result
	}
	if result.Passed == 0 && len(result.Failures) > 0 {
		result.Passed = result.Total - len(result.Failures)
		if result.Passed < 0 {
			result.Passed = 0
		}
	}
	return result
}

func parseJavaFailure(line string) TestFailure {
	rest := strings.TrimPrefix(line, "LAZYLEET_FAIL ")
	parts := strings.SplitN(rest, " ", 3)
	failure := TestFailure{}
	if len(parts) > 0 {
		fmt.Sscanf(parts[0], "%d", &failure.Index)
	}
	if len(parts) > 1 {
		failure.Comment = parts[1]
	}
	if len(parts) > 2 {
		details := parts[2]
		if expected, actual, ok := strings.Cut(details, " actual="); ok {
			failure.Expected = strings.TrimPrefix(expected, "expected=")
			failure.Actual = actual
		}
	}
	return failure
}

func (s DataPackTestCases) testCasePath(problem catalog.Problem) (string, error) {
	pack, ok := s.packForProblem(problem)
	if !ok {
		return "", fmt.Errorf("data pack for %s not found", problem.Slug)
	}
	item, err := dataPackIndexItemForProblem(pack.MetadataDir, problem)
	if err != nil {
		return "", err
	}
	file := strings.TrimSuffix(item.File, filepath.Ext(item.File))
	return filepath.Join(pack.TestsDir, file), nil
}

func (s DataPackTestCases) packForProblem(problem catalog.Problem) (catalog.DataPack, bool) {
	for _, track := range problem.Tracks {
		if pack, ok := s.packs[track]; ok {
			return pack, true
		}
	}
	return catalog.DataPack{}, false
}

func dataPackIndexItemForProblem(metadataDir string, problem catalog.Problem) (metadataIndexItem, error) {
	indexPath := filepath.Join(metadataDir, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return metadataIndexItem{}, err
	}
	var index []metadataIndexItem
	if err := json.Unmarshal(data, &index); err != nil {
		return metadataIndexItem{}, fmt.Errorf("parse %s: %w", indexPath, err)
	}
	for _, item := range index {
		if item.ID == problem.ID || slugFromURL(item.Link) == problem.Slug {
			return item, nil
		}
	}
	return metadataIndexItem{}, fmt.Errorf("test case metadata for %s not found in %s", problem.Slug, indexPath)
}
