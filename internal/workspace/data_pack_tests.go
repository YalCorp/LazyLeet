package workspace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/YalCorp/LazyLeet/internal/catalog"
)

type DataPackTestCases struct {
	packs map[string]catalog.DataPack
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
