package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type DataPack struct {
	Slug        string
	Title       string
	Version     string
	Description string
	RootDir     string
	MetadataDir string
	TestsDir    string
}

type dataPackManifest struct {
	Slug        string `toml:"slug"`
	Title       string `toml:"title"`
	Version     string `toml:"version"`
	Description string `toml:"description"`
	MetadataDir string `toml:"metadata_dir"`
	TestsDir    string `toml:"tests_dir"`
}

type dataPackIndexItem struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	File  string `json:"file"`
	Link  string `json:"link"`
}

type dataPackMetadata struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Link        string `json:"link"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	Leetcode    struct {
		TitleSlug  string `json:"title_slug"`
		Difficulty string `json:"difficulty"`
		TopicTags  []struct {
			Name string `json:"name"`
		} `json:"topic_tags"`
	} `json:"leetcode"`
}

func DiscoverDataPacks(roots ...string) ([]DataPack, error) {
	seen := map[string]struct{}{}
	var packs []DataPack
	for _, root := range roots {
		rootPacks, err := discoverDataPacksInRoot(root)
		if err != nil {
			return nil, err
		}
		for _, pack := range rootPacks {
			if _, ok := seen[pack.Slug]; ok {
				continue
			}
			seen[pack.Slug] = struct{}{}
			packs = append(packs, pack)
		}
	}
	sort.Slice(packs, func(i, j int) bool {
		return packs[i].Slug < packs[j].Slug
	})
	return packs, nil
}

func discoverDataPacksInRoot(root string) ([]DataPack, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var packs []DataPack
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if pack, ok, err := readManifestDataPack(dir); err != nil {
			return nil, err
		} else if ok {
			packs = append(packs, pack)
			continue
		}
		if !strings.HasSuffix(entry.Name(), "-metadata") {
			continue
		}
		slug := strings.TrimSuffix(entry.Name(), "-metadata")
		metadataDir := dir
		if _, err := os.Stat(filepath.Join(metadataDir, "index.json")); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		packs = append(packs, DataPack{
			Slug:        slug,
			Title:       titleFromSlug(slug),
			RootDir:     root,
			MetadataDir: metadataDir,
			TestsDir:    filepath.Join(root, slug),
		})
	}
	sort.Slice(packs, func(i, j int) bool {
		return packs[i].Slug < packs[j].Slug
	})
	return packs, nil
}

func readManifestDataPack(dir string) (DataPack, bool, error) {
	path := filepath.Join(dir, "lazyleet-pack.toml")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return DataPack{}, false, nil
		}
		return DataPack{}, false, err
	}

	var manifest dataPackManifest
	if _, err := toml.DecodeFile(path, &manifest); err != nil {
		return DataPack{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	if manifest.Slug == "" {
		return DataPack{}, false, fmt.Errorf("%s has empty slug", path)
	}
	metadataDir := manifest.MetadataDir
	if metadataDir == "" {
		metadataDir = "metadata"
	}
	testsDir := manifest.TestsDir
	if testsDir == "" {
		testsDir = "tests"
	}

	pack := DataPack{
		Slug:        manifest.Slug,
		Title:       firstNonEmpty(manifest.Title, titleFromSlug(manifest.Slug)),
		Version:     manifest.Version,
		Description: manifest.Description,
		RootDir:     dir,
		MetadataDir: filepath.Join(dir, metadataDir),
		TestsDir:    filepath.Join(dir, testsDir),
	}
	if _, err := os.Stat(filepath.Join(pack.MetadataDir, "index.json")); err != nil {
		return DataPack{}, false, err
	}
	return pack, true, nil
}

func LoadDataPacks(packs []DataPack) (Catalog, error) {
	problems := map[string]Problem{}
	tracks := make([]Track, 0, len(packs))

	for _, pack := range packs {
		track, packProblems, err := loadDataPack(pack)
		if err != nil {
			return Catalog{}, err
		}
		for _, problem := range packProblems {
			existing, ok := problems[problem.Slug]
			if ok {
				existing.Tracks = appendUnique(existing.Tracks, track.Slug)
				sort.Strings(existing.Tracks)
				problems[problem.Slug] = existing
				continue
			}
			problem.Tracks = []string{track.Slug}
			problems[problem.Slug] = problem
		}
		tracks = append(tracks, track)
	}

	return Catalog{Problems: problems, Tracks: tracks}, nil
}

func loadDataPack(pack DataPack) (Track, []Problem, error) {
	indexPath := filepath.Join(pack.MetadataDir, "index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return Track{}, nil, err
	}

	var index []dataPackIndexItem
	if err := json.Unmarshal(data, &index); err != nil {
		return Track{}, nil, fmt.Errorf("parse %s: %w", indexPath, err)
	}
	if len(index) == 0 {
		return Track{}, nil, fmt.Errorf("%s does not contain any problems", indexPath)
	}

	track := Track{
		Slug:        pack.Slug,
		Title:       firstNonEmpty(pack.Title, titleFromSlug(pack.Slug)),
		Description: firstNonEmpty(pack.Description, fmt.Sprintf("Local data pack from %s.", pack.MetadataDir)),
		Problems:    make([]string, 0, len(index)),
	}
	problems := make([]Problem, 0, len(index))
	seen := map[string]struct{}{}

	for _, item := range index {
		meta, err := readDataPackMetadata(pack.MetadataDir, item)
		if err != nil {
			return Track{}, nil, err
		}
		problem := dataPackProblem(item, meta)
		if problem.Slug == "" {
			return Track{}, nil, fmt.Errorf("problem %q has empty slug", problem.Title)
		}
		if _, ok := seen[problem.Slug]; ok {
			return Track{}, nil, fmt.Errorf("data pack %s contains duplicate problem %s", pack.Slug, problem.Slug)
		}
		seen[problem.Slug] = struct{}{}
		track.Problems = append(track.Problems, problem.Slug)
		problems = append(problems, problem)
	}

	return track, problems, nil
}

func readDataPackMetadata(metadataDir string, item dataPackIndexItem) (dataPackMetadata, error) {
	if item.File == "" {
		return dataPackMetadata{}, fmt.Errorf("data-pack index item %q has empty file", item.Title)
	}
	path := filepath.Join(metadataDir, item.File)
	data, err := os.ReadFile(path)
	if err != nil {
		return dataPackMetadata{}, err
	}
	var meta dataPackMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return dataPackMetadata{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return meta, nil
}

func dataPackProblem(item dataPackIndexItem, meta dataPackMetadata) Problem {
	title := firstNonEmpty(meta.Title, item.Title)
	url := firstNonEmpty(meta.Link, item.Link)
	tags := dataPackTags(meta)
	return Problem{
		ID:         firstNonZero(meta.ID, item.ID),
		Slug:       dataPackSlug(meta, title, url),
		Title:      title,
		Difficulty: parseDifficulty(meta.Leetcode.Difficulty),
		URL:        url,
		Tags:       tags,
		Patterns:   append([]string(nil), tags...),
	}
}

func dataPackTags(meta dataPackMetadata) []string {
	seen := map[string]struct{}{}
	var tags []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		tags = append(tags, value)
	}
	add(meta.Category)
	add(meta.Subcategory)
	for _, tag := range meta.Leetcode.TopicTags {
		add(tag.Name)
	}
	return tags
}

func dataPackSlug(meta dataPackMetadata, title, url string) string {
	if meta.Leetcode.TitleSlug != "" {
		return meta.Leetcode.TitleSlug
	}
	const marker = "/problems/"
	if idx := strings.Index(url, marker); idx >= 0 {
		rest := strings.Trim(url[idx+len(marker):], "/")
		if slash := strings.Index(rest, "/"); slash >= 0 {
			rest = rest[:slash]
		}
		if rest != "" {
			return rest
		}
	}
	return slugify(title)
}

func parseDifficulty(value string) Difficulty {
	switch Difficulty(strings.TrimSpace(value)) {
	case Easy:
		return Easy
	case Hard:
		return Hard
	default:
		return Medium
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func titleFromSlug(slug string) string {
	parts := strings.Fields(strings.ReplaceAll(slug, "-", " "))
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

var nonSlugRunes = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonSlugRunes.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}
