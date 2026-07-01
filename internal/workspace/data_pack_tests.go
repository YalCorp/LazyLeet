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

type TestRunMode string

const (
	TestRunExamples TestRunMode = "examples"
	TestRunAll      TestRunMode = "all"
	TestRunCustom   TestRunMode = "custom"
)

type TestRunRequest struct {
	Mode  TestRunMode
	Cases []TestCase
}

type TestRunResult struct {
	Passed    int
	Total     int
	Mode      TestRunMode
	Failures  []TestFailure
	Output    string
	TimedOut  bool
	TimeLimit time.Duration
}

type TestFailure struct {
	Index    int
	Comment  string
	Input    string
	Expected string
	Actual   string
	Case     TestCase
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

func (s DataPackTestCases) RunTestCases(problem catalog.Problem, language Language, solution string, request TestRunRequest) (TestRunResult, error) {
	cases, _, err := s.ReadTestCases(problem)
	if err != nil {
		return TestRunResult{}, err
	}
	cases, err = s.selectTestCases(problem, cases, request)
	if err != nil {
		return TestRunResult{}, err
	}
	if len(cases) == 0 {
		return TestRunResult{}, fmt.Errorf("no test cases found for %s", problem.Slug)
	}
	switch normalizeLanguage(language).ID {
	case "java":
		result, err := runJavaTestCases(problem, solution, cases)
		result.Mode = normalizeTestRunMode(request.Mode)
		return result, err
	default:
		return TestRunResult{}, fmt.Errorf("running %s solutions is not implemented yet", language.Title)
	}
}

func (s DataPackTestCases) selectTestCases(problem catalog.Problem, cases []TestCase, request TestRunRequest) ([]TestCase, error) {
	switch normalizeTestRunMode(request.Mode) {
	case TestRunAll:
		return cases, nil
	case TestRunCustom:
		if len(request.Cases) == 0 {
			return nil, fmt.Errorf("no selected testcase to run")
		}
		return append([]TestCase(nil), request.Cases...), nil
	default:
		examples, err := s.exampleTestCases(problem, cases)
		if err != nil {
			return nil, err
		}
		return examples, nil
	}
}

func normalizeTestRunMode(mode TestRunMode) TestRunMode {
	switch mode {
	case TestRunAll, TestRunCustom:
		return mode
	default:
		return TestRunExamples
	}
}

func (s DataPackTestCases) exampleTestCases(problem catalog.Problem, cases []TestCase) ([]TestCase, error) {
	statement, err := s.statementText(problem)
	if err != nil {
		return firstTestCases(cases, 2), nil
	}
	statement = normalizeExampleMatchText(statement)
	examples := make([]TestCase, 0, min(2, len(cases)))
	for _, tc := range cases {
		summary := normalizeExampleMatchText(javaInputSummary(problem.Method.Params, tc.Input))
		if summary != "" && strings.Contains(statement, summary) {
			examples = append(examples, tc)
		}
	}
	if len(examples) == 0 {
		return firstTestCases(cases, 2), nil
	}
	return examples, nil
}

func firstTestCases(cases []TestCase, count int) []TestCase {
	count = min(count, len(cases))
	return append([]TestCase(nil), cases[:count]...)
}

func (s DataPackTestCases) statementText(problem catalog.Problem) (string, error) {
	path, err := MetadataStatements{packs: s.packs}.metadataPath(problem)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var meta metadataStatement
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", err
	}
	return renderHTMLStatement(meta.Question.ContentHTML), nil
}

func normalizeExampleMatchText(value string) string {
	value = stripANSI(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.ReplaceAll(value, " ", "")
	return value
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

	if err := os.WriteFile(filepath.Join(dir, "Solution.java"), []byte(javaSolutionSource(solution)), 0o644); err != nil {
		return TestRunResult{}, err
	}
	caseData, err := javaCasesJSONLines(problem, cases)
	if err != nil {
		return TestRunResult{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "cases.jsonl"), caseData, 0o644); err != nil {
		return TestRunResult{}, err
	}
	harness, err := javaHarness(problem)
	if err != nil {
		return TestRunResult{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "Main.java"), []byte(harness), 0o644); err != nil {
		return TestRunResult{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), javaCompileTimeout)
	defer cancel()
	compile := exec.CommandContext(ctx, "javac", "Solution.java", "Main.java")
	compile.Dir = dir
	compileOut, err := compile.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return TestRunResult{
				Total:     len(cases),
				Output:    strings.TrimSpace(string(compileOut)),
				TimedOut:  true,
				TimeLimit: javaCompileTimeout,
			}, nil
		}
		return TestRunResult{}, commandOutputError("javac failed", err, compileOut)
	}

	runCtx, runCancel := context.WithTimeout(context.Background(), javaRunTimeout)
	defer runCancel()
	run := exec.CommandContext(runCtx, "java", "-cp", dir, "Main")
	run.Dir = dir
	output, err := run.CombinedOutput()
	result := parseJavaRunOutput(string(output), len(cases))
	for i := range result.Failures {
		index := result.Failures[i].Index - 1
		if index >= 0 && index < len(cases) {
			result.Failures[i].Case = cases[index]
		}
	}
	result.Output = strings.TrimSpace(string(output))
	if runCtx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.TimeLimit = javaRunTimeout
		return result, nil
	}
	if err != nil {
		if result.Total > 0 {
			return result, nil
		}
		return TestRunResult{}, commandOutputError("java failed", err, output)
	}
	return result, nil
}

var (
	javaCompileTimeout = 10 * time.Second
	javaRunTimeout     = 10 * time.Second
)

func javaSolutionSource(solution string) string {
	trimmed := strings.TrimLeft(solution, "\ufeff \t\r\n")
	if strings.HasPrefix(trimmed, "package ") {
		return solution
	}
	return "import java.util.*;\nimport java.io.*;\nimport java.math.*;\n\n" + solution
}

func commandOutputError(label string, err error, output []byte) error {
	details := strings.TrimSpace(string(output))
	if details == "" {
		return fmt.Errorf("%s: %w", label, err)
	}
	return fmt.Errorf("%s: %w\n%s", label, err, details)
}

func javaCasesJSONLines(problem catalog.Problem, cases []TestCase) ([]byte, error) {
	var b bytes.Buffer
	for _, tc := range cases {
		record := struct {
			Input    map[string]json.RawMessage `json:"input"`
			Expected json.RawMessage            `json:"expected"`
			Comment  string                     `json:"comment,omitempty"`
			Summary  string                     `json:"summary"`
		}{
			Input:    tc.Input,
			Expected: tc.Expected,
			Comment:  tc.Comment,
			Summary:  javaInputSummary(problem.Method.Params, tc.Input),
		}
		line, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.Bytes(), nil
}

func javaHarness(problem catalog.Problem) (string, error) {
	var b strings.Builder
	b.WriteString("import java.nio.charset.StandardCharsets;\n")
	b.WriteString("import java.nio.file.Files;\n")
	b.WriteString("import java.nio.file.Paths;\n")
	b.WriteString("import java.util.*;\n\n")
	b.WriteString("class Main {\n")
	b.WriteString("  static int passed = 0;\n")
	b.WriteString("  static int total = 0;\n")
	b.WriteString("  public static void main(String[] args) throws Exception {\n")
	b.WriteString("    Solution s = new Solution();\n")
	b.WriteString("    List<String> lines = Files.readAllLines(Paths.get(\"cases.jsonl\"), StandardCharsets.UTF_8);\n")
	b.WriteString("    int index = 0;\n")
	b.WriteString("    for (String line : lines) {\n")
	b.WriteString("      if (line.trim().isEmpty()) continue;\n")
	b.WriteString("      index++;\n")
	b.WriteString("      CaseData c = parseCase(line);\n")
	b.WriteString("      check(index, c.comment, c.summary, invoke(s, c), c.expected);\n")
	b.WriteString("    }\n")
	b.WriteString("    System.out.println(\"LAZYLEET_RESULT \" + passed + \" \" + total);\n")
	b.WriteString("  }\n")
	b.WriteString("  static Object invoke(Solution s, CaseData c) {\n")
	args := make([]string, 0, len(problem.Method.Params))
	for _, param := range problem.Method.Params {
		arg, err := javaParamAccess(param)
		if err != nil {
			return "", err
		}
		args = append(args, arg)
	}
	b.WriteString(fmt.Sprintf("    return s.%s(%s);\n", problem.Method.Name, strings.Join(args, ", ")))
	b.WriteString("  }\n")
	b.WriteString("  static CaseData parseCase(String line) {\n")
	b.WriteString("    CaseData c = new CaseData();\n")
	b.WriteString("    String inputJson = field(line, \"input\");\n")
	for _, param := range problem.Method.Params {
		if _, err := javaReadValueKind(param.Type); err != nil {
			return "", err
		}
		b.WriteString(fmt.Sprintf("    c.input.put(%s, readValue(%s, field(inputJson, %s), false));\n", javaStringLiteral(param.Name), javaStringLiteral(param.Type), javaStringLiteral(param.Name)))
	}
	if _, err := javaReadValueKind(problem.Method.ReturnType); err != nil {
		return "", err
	}
	b.WriteString(fmt.Sprintf("    c.expected = readValue(%s, field(line, \"expected\"), true);\n", javaStringLiteral(problem.Method.ReturnType)))
	b.WriteString("    String comment = fieldOrNull(line, \"comment\");\n")
	b.WriteString("    c.comment = comment == null ? \"\" : parseString(comment);\n")
	b.WriteString("    String summary = fieldOrNull(line, \"summary\");\n")
	b.WriteString("    c.summary = summary == null ? \"\" : parseString(summary);\n")
	b.WriteString("    return c;\n")
	b.WriteString("  }\n")
	b.WriteString(javaHarnessHelpers())
	b.WriteString("  static class CaseData {\n")
	b.WriteString("    Map<String, Object> input = new HashMap<>();\n")
	b.WriteString("    Object expected;\n")
	b.WriteString("    String comment = \"\";\n")
	b.WriteString("    String summary = \"\";\n")
	b.WriteString("  }\n")
	b.WriteString("  static void check(int index, String comment, String input, Object actual, Object expected) {\n")
	b.WriteString("    total++;\n")
	b.WriteString("    if (Objects.deepEquals(actual, expected)) { passed++; return; }\n")
	b.WriteString("    System.out.println(\"LAZYLEET_FAIL\\t\" + index + \"\\t\" + clean(comment) + \"\\t\" + clean(input) + \"\\t\" + clean(render(expected)) + \"\\t\" + clean(render(actual)));\n")
	b.WriteString("  }\n")
	b.WriteString("  static String clean(String value) {\n")
	b.WriteString("    if (value == null) return \"\";\n")
	b.WriteString("    return value.replace('\\t', ' ').replace('\\n', ' ').replace('\\r', ' ');\n")
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

func javaParamAccess(param catalog.MethodParam) (string, error) {
	name := javaStringLiteral(param.Name)
	switch strings.TrimSpace(param.Type) {
	case "integer":
		return fmt.Sprintf("((Integer) c.input.get(%s))", name), nil
	case "boolean":
		return fmt.Sprintf("((Boolean) c.input.get(%s))", name), nil
	case "integer[][]":
		return fmt.Sprintf("((int[][]) c.input.get(%s))", name), nil
	case "character[][]":
		return fmt.Sprintf("((char[][]) c.input.get(%s))", name), nil
	default:
		return "", fmt.Errorf("unsupported type %q", param.Type)
	}
}

func javaReadValueKind(kind string) (string, error) {
	switch strings.TrimSpace(kind) {
	case "integer", "boolean", "integer[][]", "character[][]":
		return strings.TrimSpace(kind), nil
	default:
		return "", fmt.Errorf("unsupported type %q", kind)
	}
}

func javaHarnessHelpers() string {
	return `  static Object readValue(String kind, String raw, boolean expected) {
    switch (kind.trim()) {
      case "integer": return Integer.parseInt(raw.trim());
      case "boolean": return Boolean.parseBoolean(raw.trim());
      case "integer[][]": return expected ? parseIntegerListMatrix(raw) : parseIntMatrix(raw);
      case "character[][]": return parseCharMatrix(raw);
      default: throw new IllegalArgumentException("unsupported type " + kind);
    }
  }
  static String field(String json, String key) {
    String value = fieldOrNull(json, key);
    if (value == null) throw new IllegalArgumentException("missing field " + key);
    return value;
  }
  static String fieldOrNull(String json, String key) {
    String needle = "\"" + key + "\"";
    int keyPos = json.indexOf(needle);
    if (keyPos < 0) return null;
    int colon = json.indexOf(':', keyPos + needle.length());
    if (colon < 0) return null;
    int start = skipWhitespace(json, colon + 1);
    int end = valueEnd(json, start);
    return json.substring(start, end);
  }
  static int valueEnd(String json, int start) {
    char first = json.charAt(start);
    if (first == '"') return stringEnd(json, start) + 1;
    if (first == '[' || first == '{') {
      int depth = 0;
      for (int i = start; i < json.length(); i++) {
        char ch = json.charAt(i);
        if (ch == '"') {
          i = stringEnd(json, i);
          continue;
        }
        if (ch == '[' || ch == '{') depth++;
        if (ch == ']' || ch == '}') {
          depth--;
          if (depth == 0) return i + 1;
        }
      }
      return json.length();
    }
    int end = start;
    while (end < json.length() && json.charAt(end) != ',' && json.charAt(end) != '}') end++;
    return end;
  }
  static int stringEnd(String json, int start) {
    for (int i = start + 1; i < json.length(); i++) {
      char ch = json.charAt(i);
      if (ch == '\\') {
        i++;
        continue;
      }
      if (ch == '"') return i;
    }
    return json.length() - 1;
  }
  static int skipWhitespace(String s, int i) {
    while (i < s.length() && Character.isWhitespace(s.charAt(i))) i++;
    return i;
  }
  static int parseIntAt(String s, int[] pos) {
    int i = skipWhitespace(s, pos[0]);
    int sign = 1;
    if (s.charAt(i) == '-') {
      sign = -1;
      i++;
    }
    int value = 0;
    while (i < s.length() && Character.isDigit(s.charAt(i))) {
      value = value * 10 + (s.charAt(i) - '0');
      i++;
    }
    pos[0] = i;
    return value * sign;
  }
  static int[][] parseIntMatrix(String raw) {
    List<int[]> rows = new ArrayList<>();
    int[] pos = new int[]{skipWhitespace(raw, 0) + 1};
    while (true) {
      pos[0] = skipWhitespace(raw, pos[0]);
      if (pos[0] >= raw.length() || raw.charAt(pos[0]) == ']') break;
      if (raw.charAt(pos[0]) == ',') {
        pos[0]++;
        continue;
      }
      rows.add(parseIntRow(raw, pos));
    }
    return rows.toArray(new int[rows.size()][]);
  }
  static int[] parseIntRow(String raw, int[] pos) {
    List<Integer> values = new ArrayList<>();
    pos[0] = skipWhitespace(raw, pos[0]) + 1;
    while (true) {
      pos[0] = skipWhitespace(raw, pos[0]);
      if (raw.charAt(pos[0]) == ']') {
        pos[0]++;
        break;
      }
      if (raw.charAt(pos[0]) == ',') {
        pos[0]++;
        continue;
      }
      values.add(parseIntAt(raw, pos));
    }
    int[] row = new int[values.size()];
    for (int i = 0; i < values.size(); i++) row[i] = values.get(i);
    return row;
  }
  static List<List<Integer>> parseIntegerListMatrix(String raw) {
    List<List<Integer>> rows = new ArrayList<>();
    int[] pos = new int[]{skipWhitespace(raw, 0) + 1};
    while (true) {
      pos[0] = skipWhitespace(raw, pos[0]);
      if (pos[0] >= raw.length() || raw.charAt(pos[0]) == ']') break;
      if (raw.charAt(pos[0]) == ',') {
        pos[0]++;
        continue;
      }
      int[] row = parseIntRow(raw, pos);
      List<Integer> list = new ArrayList<>();
      for (int value : row) list.add(value);
      rows.add(list);
    }
    return rows;
  }
  static char[][] parseCharMatrix(String raw) {
    List<char[]> rows = new ArrayList<>();
    int[] pos = new int[]{skipWhitespace(raw, 0) + 1};
    while (true) {
      pos[0] = skipWhitespace(raw, pos[0]);
      if (pos[0] >= raw.length() || raw.charAt(pos[0]) == ']') break;
      if (raw.charAt(pos[0]) == ',') {
        pos[0]++;
        continue;
      }
      rows.add(parseCharRow(raw, pos));
    }
    return rows.toArray(new char[rows.size()][]);
  }
  static char[] parseCharRow(String raw, int[] pos) {
    List<Character> values = new ArrayList<>();
    pos[0] = skipWhitespace(raw, pos[0]) + 1;
    while (true) {
      pos[0] = skipWhitespace(raw, pos[0]);
      if (raw.charAt(pos[0]) == ']') {
        pos[0]++;
        break;
      }
      if (raw.charAt(pos[0]) == ',') {
        pos[0]++;
        continue;
      }
      String value = parseStringAt(raw, pos);
      values.add(value.isEmpty() ? '\0' : value.charAt(0));
    }
    char[] row = new char[values.size()];
    for (int i = 0; i < values.size(); i++) row[i] = values.get(i);
    return row;
  }
  static String parseString(String raw) {
    return parseStringAt(raw, new int[]{skipWhitespace(raw, 0)});
  }
  static String parseStringAt(String raw, int[] pos) {
    StringBuilder out = new StringBuilder();
    int i = skipWhitespace(raw, pos[0]) + 1;
    while (i < raw.length()) {
      char ch = raw.charAt(i++);
      if (ch == '"') break;
      if (ch != '\\') {
        out.append(ch);
        continue;
      }
      char esc = raw.charAt(i++);
      switch (esc) {
        case '"': out.append('"'); break;
        case '\\': out.append('\\'); break;
        case '/': out.append('/'); break;
        case 'b': out.append('\b'); break;
        case 'f': out.append('\f'); break;
        case 'n': out.append('\n'); break;
        case 'r': out.append('\r'); break;
        case 't': out.append('\t'); break;
        case 'u':
          out.append((char) Integer.parseInt(raw.substring(i, i + 4), 16));
          i += 4;
          break;
        default: out.append(esc);
      }
    }
    pos[0] = i;
    return out.toString();
  }
`
}

func javaInputSummary(params []catalog.MethodParam, input map[string]json.RawMessage) string {
	parts := make([]string, 0, len(params))
	for _, param := range params {
		raw, ok := input[param.Name]
		if !ok {
			continue
		}
		parts = append(parts, param.Name+" = "+compactJSON(raw))
	}
	return strings.Join(parts, ", ")
}

func compactJSON(raw json.RawMessage) string {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return strings.TrimSpace(string(raw))
	}
	data, err := json.Marshal(value)
	if err != nil {
		return strings.TrimSpace(string(raw))
	}
	return string(data)
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
		if strings.HasPrefix(line, "LAZYLEET_FAIL\t") {
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
	rest := strings.TrimPrefix(line, "LAZYLEET_FAIL\t")
	parts := strings.SplitN(rest, "\t", 6)
	failure := TestFailure{}
	if len(parts) > 0 {
		fmt.Sscanf(parts[0], "%d", &failure.Index)
	}
	if len(parts) > 1 {
		failure.Comment = parts[1]
	}
	if len(parts) > 2 {
		failure.Input = parts[2]
	}
	if len(parts) > 3 {
		failure.Expected = parts[3]
	}
	if len(parts) > 4 {
		failure.Actual = parts[4]
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
