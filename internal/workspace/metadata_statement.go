package workspace

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/YalCorp/LazyLeet/internal/catalog"
)

type MetadataStatements struct {
	packs map[string]catalog.DataPack
}

func NewMetadataStatements(packs []catalog.DataPack) MetadataStatements {
	bySlug := make(map[string]catalog.DataPack, len(packs))
	for _, pack := range packs {
		bySlug[pack.Slug] = pack
	}
	return MetadataStatements{packs: bySlug}
}

func (s MetadataStatements) ReadStatement(problem catalog.Problem) (content string, path string, err error) {
	path, err = s.metadataPath(problem)
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", path, err
	}

	var meta metadataStatement
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", path, fmt.Errorf("parse %s: %w", path, err)
	}

	text := renderHTMLStatement(meta.Question.ContentHTML)
	if text == "" {
		text = normalizeStatementText(meta.Question.ContentText)
	}
	if text == "" {
		text = "No local statement text found in metadata."
	}

	return text, path, nil
}

func (s MetadataStatements) metadataPath(problem catalog.Problem) (string, error) {
	pack, ok := s.packForProblem(problem)
	if !ok {
		return "", fmt.Errorf("data pack for %s not found", problem.Slug)
	}
	indexPath := filepath.Join(pack.MetadataDir, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return "", err
	}
	var index []metadataIndexItem
	if err := json.Unmarshal(data, &index); err != nil {
		return "", fmt.Errorf("parse %s: %w", indexPath, err)
	}
	for _, item := range index {
		if item.ID == problem.ID || slugFromURL(item.Link) == problem.Slug {
			return filepath.Join(pack.MetadataDir, item.File), nil
		}
	}
	return "", fmt.Errorf("metadata for %s not found in %s", problem.Slug, indexPath)
}

func (s MetadataStatements) packForProblem(problem catalog.Problem) (catalog.DataPack, bool) {
	for _, track := range problem.Tracks {
		if pack, ok := s.packs[track]; ok {
			return pack, true
		}
	}
	return catalog.DataPack{}, false
}

func slugFromURL(url string) string {
	const marker = "/problems/"
	if idx := strings.Index(url, marker); idx >= 0 {
		rest := strings.Trim(url[idx+len(marker):], "/")
		if slash := strings.Index(rest, "/"); slash >= 0 {
			rest = rest[:slash]
		}
		return rest
	}
	return ""
}

func renderHTMLStatement(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	replacements := []struct {
		pattern string
		value   string
	}{
		{`(?i)<\s*br\s*/?\s*>`, "\n"},
		{`(?i)<\s*/\s*(p|div|pre|ul|ol)\s*>`, "\n"},
		{`(?i)<\s*(p|div|pre|ul|ol)(\s+[^>]*)?>`, "\n"},
		{`(?i)<\s*li(\s+[^>]*)?>`, "\n- "},
		{`(?i)<\s*/\s*li\s*>`, ""},
		{`(?i)<\s*(strong|b)(\s+[^>]*)?>`, "\x1b[1m"},
		{`(?i)<\s*/\s*(strong|b)\s*>`, "\x1b[0m"},
		{`(?i)<\s*(em|i)(\s+[^>]*)?>`, "\x1b[3m"},
		{`(?i)<\s*/\s*(em|i)\s*>`, "\x1b[0m"},
		{`(?i)<\s*code(\s+[^>]*)?>`, "\x1b[36m"},
		{`(?i)<\s*/\s*code\s*>`, "\x1b[0m"},
		{`(?i)<\s*/?\s*span(\s+[^>]*)?>`, ""},
	}
	for _, replacement := range replacements {
		value = regexp.MustCompile(replacement.pattern).ReplaceAllString(value, replacement.value)
	}
	value = regexp.MustCompile(`(?s)<[^>]*>`).ReplaceAllString(value, "")
	return styleStatementSections(addStatementSectionSpacing(normalizeStatementText(html.UnescapeString(value))))
}

func normalizeStatementText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = regexp.MustCompile(`[ \t]+`).ReplaceAllString(line, " ")
		if line == "" {
			if !blank && len(out) > 0 {
				out = append(out, "")
				blank = true
			}
			continue
		}
		out = append(out, line)
		blank = false
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

func addStatementSectionSpacing(value string) string {
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines)+2)
	inConstraints := false
	for i, line := range lines {
		plain := stripANSI(line)
		if isConstraintsTitle(plain) {
			inConstraints = true
		} else if strings.HasPrefix(strings.TrimSpace(plain), "Example ") {
			inConstraints = false
		}
		if strings.TrimSpace(plain) == "" && inConstraints && nextNonBlankLineIsConstraintItem(lines, i+1) {
			continue
		}
		if strings.TrimSpace(plain) == "" && len(out) > 0 && isConstraintsTitle(stripANSI(out[len(out)-1])) {
			continue
		}
		if isSpacedStatementSectionStart(plain) && len(out) > 0 && out[len(out)-1] != "" {
			out = append(out, "")
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func nextNonBlankLineIsConstraintItem(lines []string, start int) bool {
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(stripANSI(lines[i]))
		if line == "" {
			continue
		}
		return strings.HasPrefix(line, "- ")
	}
	return false
}

func styleStatementSections(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		plain := strings.TrimSpace(stripANSI(line))
		switch {
		case isStatementTitle(plain):
			lines[i] = sectionTitleANSI + plain + resetANSI
		case isExampleFieldTitle(plain):
			lines[i] = accentANSI + line + resetANSI
		}
	}
	return strings.Join(lines, "\n")
}

func isSpacedStatementSectionStart(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "Example ") || isConstraintsTitle(line)
}

func isStatementTitle(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "Example ") || isConstraintsTitle(line)
}

func isConstraintsTitle(line string) bool {
	line = strings.TrimSpace(line)
	return line == "Constraints:" || strings.HasPrefix(line, "Constraints")
}

func isExampleFieldTitle(line string) bool {
	return strings.HasPrefix(line, "Input:") || strings.HasPrefix(line, "Output:") || strings.HasPrefix(line, "Explanation:")
}

const (
	boldANSI         = "\x1b[1m"
	sectionTitleANSI = "\x1b[1;38;2;235;203;139m"
	accentANSI       = "\x1b[36m"
	resetANSI        = "\x1b[0m"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

type metadataIndexItem struct {
	ID   int    `json:"id"`
	File string `json:"file"`
	Link string `json:"link"`
}

type metadataStatement struct {
	Title       string `json:"title"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	Question    struct {
		ContentHTML string `json:"content_html"`
		ContentText string `json:"content_text"`
	} `json:"question"`
}
