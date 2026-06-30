package catalog

import (
	"fmt"
	"sort"
)

type Difficulty string

const (
	Easy   Difficulty = "Easy"
	Medium Difficulty = "Medium"
	Hard   Difficulty = "Hard"
)

type Problem struct {
	ID         int
	Slug       string
	Title      string
	Difficulty Difficulty
	URL        string
	Tags       []string
	TopicTags  []string
	Patterns   []string
	Tracks     []string
	Snippets   []CodeSnippet
	Method     Method
}

type CodeSnippet struct {
	Lang     string
	LangSlug string
	Code     string
}

type Method struct {
	Name       string
	Params     []MethodParam
	ReturnType string
}

type MethodParam struct {
	Name string
	Type string
}

type Track struct {
	Slug        string
	Title       string
	Description string
	Problems    []string
}

type Catalog struct {
	Problems map[string]Problem
	Tracks   []Track
}

type seed struct {
	ID         int
	Slug       string
	Title      string
	Difficulty Difficulty
	Tags       []string
}

func Load() (Catalog, error) {
	problems := make(map[string]Problem, len(problemSeeds))
	for _, item := range problemSeeds {
		if item.Slug == "" {
			return Catalog{}, fmt.Errorf("problem %q has empty slug", item.Title)
		}
		if _, ok := problems[item.Slug]; ok {
			return Catalog{}, fmt.Errorf("duplicate problem slug %q", item.Slug)
		}
		tags := append([]string(nil), item.Tags...)
		problems[item.Slug] = Problem{
			ID:         item.ID,
			Slug:       item.Slug,
			Title:      item.Title,
			Difficulty: item.Difficulty,
			URL:        "https://leetcode.com/problems/" + item.Slug + "/",
			Tags:       tags,
			TopicTags:  append([]string(nil), tags...),
			Patterns:   tags,
		}
	}

	tracks := []Track{
		{
			Slug:        "blind-75",
			Title:       "Blind 75",
			Description: "Classic interview problem set curated by topic.",
			Problems:    append([]string(nil), blind75...),
		},
		{
			Slug:        "neetcode-150",
			Title:       "NeetCode 150",
			Description: "Expanded pattern-first DSA practice set.",
			Problems:    append([]string(nil), neetcode150...),
		},
	}

	for _, track := range tracks {
		seen := map[string]struct{}{}
		for _, slug := range track.Problems {
			if _, ok := problems[slug]; !ok {
				return Catalog{}, fmt.Errorf("track %s references unknown problem %s", track.Slug, slug)
			}
			if _, ok := seen[slug]; ok {
				return Catalog{}, fmt.Errorf("track %s contains duplicate problem %s", track.Slug, slug)
			}
			seen[slug] = struct{}{}
			problem := problems[slug]
			problem.Tracks = append(problem.Tracks, track.Slug)
			sort.Strings(problem.Tracks)
			problems[slug] = problem
		}
	}

	return Catalog{Problems: problems, Tracks: tracks}, nil
}

func (c Catalog) Problem(slug string) (Problem, bool) {
	problem, ok := c.Problems[slug]
	return problem, ok
}

func (c Catalog) Track(slug string) (Track, bool) {
	for _, track := range c.Tracks {
		if track.Slug == slug {
			return track, true
		}
	}
	return Track{}, false
}

func (c Catalog) TrackProblems(track Track) []Problem {
	out := make([]Problem, 0, len(track.Problems))
	for _, slug := range track.Problems {
		if problem, ok := c.Problems[slug]; ok {
			out = append(out, problem)
		}
	}
	return out
}

var problemSeeds = []seed{
	{217, "contains-duplicate", "Contains Duplicate", Easy, []string{"Arrays & Hashing"}},
	{242, "valid-anagram", "Valid Anagram", Easy, []string{"Arrays & Hashing"}},
	{1, "two-sum", "Two Sum", Easy, []string{"Arrays & Hashing"}},
	{49, "group-anagrams", "Group Anagrams", Medium, []string{"Arrays & Hashing"}},
	{347, "top-k-frequent-elements", "Top K Frequent Elements", Medium, []string{"Arrays & Hashing", "Heap"}},
	{271, "encode-and-decode-strings", "Encode and Decode Strings", Medium, []string{"Arrays & Hashing", "Design"}},
	{238, "product-of-array-except-self", "Product of Array Except Self", Medium, []string{"Arrays & Hashing"}},
	{36, "valid-sudoku", "Valid Sudoku", Medium, []string{"Arrays & Hashing"}},
	{128, "longest-consecutive-sequence", "Longest Consecutive Sequence", Medium, []string{"Arrays & Hashing"}},
	{125, "valid-palindrome", "Valid Palindrome", Easy, []string{"Two Pointers"}},
	{167, "two-sum-ii-input-array-is-sorted", "Two Sum II - Input Array Is Sorted", Medium, []string{"Two Pointers"}},
	{15, "3sum", "3Sum", Medium, []string{"Two Pointers"}},
	{11, "container-with-most-water", "Container With Most Water", Medium, []string{"Two Pointers"}},
	{42, "trapping-rain-water", "Trapping Rain Water", Hard, []string{"Two Pointers"}},
	{121, "best-time-to-buy-and-sell-stock", "Best Time to Buy and Sell Stock", Easy, []string{"Sliding Window"}},
	{3, "longest-substring-without-repeating-characters", "Longest Substring Without Repeating Characters", Medium, []string{"Sliding Window"}},
	{424, "longest-repeating-character-replacement", "Longest Repeating Character Replacement", Medium, []string{"Sliding Window"}},
	{567, "permutation-in-string", "Permutation in String", Medium, []string{"Sliding Window"}},
	{76, "minimum-window-substring", "Minimum Window Substring", Hard, []string{"Sliding Window"}},
	{239, "sliding-window-maximum", "Sliding Window Maximum", Hard, []string{"Sliding Window", "Heap"}},
	{20, "valid-parentheses", "Valid Parentheses", Easy, []string{"Stack"}},
	{155, "min-stack", "Min Stack", Medium, []string{"Stack", "Design"}},
	{150, "evaluate-reverse-polish-notation", "Evaluate Reverse Polish Notation", Medium, []string{"Stack"}},
	{22, "generate-parentheses", "Generate Parentheses", Medium, []string{"Stack", "Backtracking"}},
	{739, "daily-temperatures", "Daily Temperatures", Medium, []string{"Stack"}},
	{853, "car-fleet", "Car Fleet", Medium, []string{"Stack"}},
	{84, "largest-rectangle-in-histogram", "Largest Rectangle In Histogram", Hard, []string{"Stack"}},
	{704, "binary-search", "Binary Search", Easy, []string{"Binary Search"}},
	{74, "search-a-2d-matrix", "Search a 2D Matrix", Medium, []string{"Binary Search"}},
	{875, "koko-eating-bananas", "Koko Eating Bananas", Medium, []string{"Binary Search"}},
	{153, "find-minimum-in-rotated-sorted-array", "Find Minimum In Rotated Sorted Array", Medium, []string{"Binary Search"}},
	{33, "search-in-rotated-sorted-array", "Search In Rotated Sorted Array", Medium, []string{"Binary Search"}},
	{981, "time-based-key-value-store", "Time Based Key-Value Store", Medium, []string{"Binary Search", "Design"}},
	{4, "median-of-two-sorted-arrays", "Median of Two Sorted Arrays", Hard, []string{"Binary Search"}},
	{206, "reverse-linked-list", "Reverse Linked List", Easy, []string{"Linked List"}},
	{21, "merge-two-sorted-lists", "Merge Two Sorted Lists", Easy, []string{"Linked List"}},
	{143, "reorder-list", "Reorder List", Medium, []string{"Linked List"}},
	{19, "remove-nth-node-from-end-of-list", "Remove Nth Node From End of List", Medium, []string{"Linked List"}},
	{138, "copy-list-with-random-pointer", "Copy List With Random Pointer", Medium, []string{"Linked List"}},
	{2, "add-two-numbers", "Add Two Numbers", Medium, []string{"Linked List"}},
	{141, "linked-list-cycle", "Linked List Cycle", Easy, []string{"Linked List"}},
	{287, "find-the-duplicate-number", "Find the Duplicate Number", Medium, []string{"Linked List", "Two Pointers"}},
	{146, "lru-cache", "LRU Cache", Medium, []string{"Linked List", "Design"}},
	{23, "merge-k-sorted-lists", "Merge K Sorted Lists", Hard, []string{"Linked List", "Heap"}},
	{25, "reverse-nodes-in-k-group", "Reverse Nodes In K-Group", Hard, []string{"Linked List"}},
	{226, "invert-binary-tree", "Invert Binary Tree", Easy, []string{"Trees"}},
	{104, "maximum-depth-of-binary-tree", "Maximum Depth of Binary Tree", Easy, []string{"Trees"}},
	{543, "diameter-of-binary-tree", "Diameter of Binary Tree", Easy, []string{"Trees"}},
	{110, "balanced-binary-tree", "Balanced Binary Tree", Easy, []string{"Trees"}},
	{100, "same-tree", "Same Tree", Easy, []string{"Trees"}},
	{572, "subtree-of-another-tree", "Subtree of Another Tree", Easy, []string{"Trees"}},
	{235, "lowest-common-ancestor-of-a-binary-search-tree", "Lowest Common Ancestor of a Binary Search Tree", Medium, []string{"Trees"}},
	{102, "binary-tree-level-order-traversal", "Binary Tree Level Order Traversal", Medium, []string{"Trees"}},
	{199, "binary-tree-right-side-view", "Binary Tree Right Side View", Medium, []string{"Trees"}},
	{1448, "count-good-nodes-in-binary-tree", "Count Good Nodes In Binary Tree", Medium, []string{"Trees"}},
	{98, "validate-binary-search-tree", "Validate Binary Search Tree", Medium, []string{"Trees"}},
	{230, "kth-smallest-element-in-a-bst", "Kth Smallest Element In a BST", Medium, []string{"Trees"}},
	{105, "construct-binary-tree-from-preorder-and-inorder-traversal", "Construct Binary Tree From Preorder and Inorder Traversal", Medium, []string{"Trees"}},
	{124, "binary-tree-maximum-path-sum", "Binary Tree Maximum Path Sum", Hard, []string{"Trees"}},
	{297, "serialize-and-deserialize-binary-tree", "Serialize and Deserialize Binary Tree", Hard, []string{"Trees", "Design"}},
	{208, "implement-trie-prefix-tree", "Implement Trie Prefix Tree", Medium, []string{"Tries", "Design"}},
	{211, "design-add-and-search-words-data-structure", "Design Add and Search Words Data Structure", Medium, []string{"Tries", "Design"}},
	{212, "word-search-ii", "Word Search II", Hard, []string{"Tries", "Backtracking"}},
	{703, "kth-largest-element-in-a-stream", "Kth Largest Element In a Stream", Easy, []string{"Heap"}},
	{1046, "last-stone-weight", "Last Stone Weight", Easy, []string{"Heap"}},
	{973, "k-closest-points-to-origin", "K Closest Points to Origin", Medium, []string{"Heap"}},
	{215, "kth-largest-element-in-an-array", "Kth Largest Element In an Array", Medium, []string{"Heap"}},
	{621, "task-scheduler", "Task Scheduler", Medium, []string{"Heap", "Greedy"}},
	{355, "design-twitter", "Design Twitter", Medium, []string{"Heap", "Design"}},
	{295, "find-median-from-data-stream", "Find Median From Data Stream", Hard, []string{"Heap", "Design"}},
	{78, "subsets", "Subsets", Medium, []string{"Backtracking"}},
	{39, "combination-sum", "Combination Sum", Medium, []string{"Backtracking"}},
	{46, "permutations", "Permutations", Medium, []string{"Backtracking"}},
	{90, "subsets-ii", "Subsets II", Medium, []string{"Backtracking"}},
	{40, "combination-sum-ii", "Combination Sum II", Medium, []string{"Backtracking"}},
	{79, "word-search", "Word Search", Medium, []string{"Backtracking"}},
	{131, "palindrome-partitioning", "Palindrome Partitioning", Medium, []string{"Backtracking"}},
	{17, "letter-combinations-of-a-phone-number", "Letter Combinations of a Phone Number", Medium, []string{"Backtracking"}},
	{51, "n-queens", "N-Queens", Hard, []string{"Backtracking"}},
	{200, "number-of-islands", "Number of Islands", Medium, []string{"Graphs"}},
	{133, "clone-graph", "Clone Graph", Medium, []string{"Graphs"}},
	{695, "max-area-of-island", "Max Area of Island", Medium, []string{"Graphs"}},
	{417, "pacific-atlantic-water-flow", "Pacific Atlantic Water Flow", Medium, []string{"Graphs"}},
	{130, "surrounded-regions", "Surrounded Regions", Medium, []string{"Graphs"}},
	{994, "rotting-oranges", "Rotting Oranges", Medium, []string{"Graphs"}},
	{286, "walls-and-gates", "Walls and Gates", Medium, []string{"Graphs"}},
	{684, "redundant-connection", "Redundant Connection", Medium, []string{"Graphs"}},
	{207, "course-schedule", "Course Schedule", Medium, []string{"Graphs"}},
	{210, "course-schedule-ii", "Course Schedule II", Medium, []string{"Graphs"}},
	{261, "graph-valid-tree", "Graph Valid Tree", Medium, []string{"Graphs"}},
	{323, "number-of-connected-components-in-an-undirected-graph", "Number of Connected Components In an Undirected Graph", Medium, []string{"Graphs"}},
	{332, "reconstruct-itinerary", "Reconstruct Itinerary", Hard, []string{"Advanced Graphs"}},
	{1584, "min-cost-to-connect-all-points", "Min Cost to Connect All Points", Medium, []string{"Advanced Graphs"}},
	{743, "network-delay-time", "Network Delay Time", Medium, []string{"Advanced Graphs"}},
	{778, "swim-in-rising-water", "Swim In Rising Water", Hard, []string{"Advanced Graphs"}},
	{269, "alien-dictionary", "Alien Dictionary", Hard, []string{"Advanced Graphs"}},
	{787, "cheapest-flights-within-k-stops", "Cheapest Flights Within K Stops", Medium, []string{"Advanced Graphs"}},
	{127, "word-ladder", "Word Ladder", Hard, []string{"Advanced Graphs"}},
	{70, "climbing-stairs", "Climbing Stairs", Easy, []string{"1-D Dynamic Programming"}},
	{746, "min-cost-climbing-stairs", "Min Cost Climbing Stairs", Easy, []string{"1-D Dynamic Programming"}},
	{198, "house-robber", "House Robber", Medium, []string{"1-D Dynamic Programming"}},
	{213, "house-robber-ii", "House Robber II", Medium, []string{"1-D Dynamic Programming"}},
	{5, "longest-palindromic-substring", "Longest Palindromic Substring", Medium, []string{"1-D Dynamic Programming"}},
	{647, "palindromic-substrings", "Palindromic Substrings", Medium, []string{"1-D Dynamic Programming"}},
	{91, "decode-ways", "Decode Ways", Medium, []string{"1-D Dynamic Programming"}},
	{322, "coin-change", "Coin Change", Medium, []string{"1-D Dynamic Programming"}},
	{152, "maximum-product-subarray", "Maximum Product Subarray", Medium, []string{"1-D Dynamic Programming"}},
	{139, "word-break", "Word Break", Medium, []string{"1-D Dynamic Programming"}},
	{300, "longest-increasing-subsequence", "Longest Increasing Subsequence", Medium, []string{"1-D Dynamic Programming"}},
	{416, "partition-equal-subset-sum", "Partition Equal Subset Sum", Medium, []string{"1-D Dynamic Programming"}},
	{62, "unique-paths", "Unique Paths", Medium, []string{"2-D Dynamic Programming"}},
	{1143, "longest-common-subsequence", "Longest Common Subsequence", Medium, []string{"2-D Dynamic Programming"}},
	{309, "best-time-to-buy-and-sell-stock-with-cooldown", "Best Time to Buy and Sell Stock With Cooldown", Medium, []string{"2-D Dynamic Programming"}},
	{518, "coin-change-ii", "Coin Change II", Medium, []string{"2-D Dynamic Programming"}},
	{494, "target-sum", "Target Sum", Medium, []string{"2-D Dynamic Programming"}},
	{97, "interleaving-string", "Interleaving String", Medium, []string{"2-D Dynamic Programming"}},
	{329, "longest-increasing-path-in-a-matrix", "Longest Increasing Path In a Matrix", Hard, []string{"2-D Dynamic Programming"}},
	{115, "distinct-subsequences", "Distinct Subsequences", Hard, []string{"2-D Dynamic Programming"}},
	{72, "edit-distance", "Edit Distance", Medium, []string{"2-D Dynamic Programming"}},
	{312, "burst-balloons", "Burst Balloons", Hard, []string{"2-D Dynamic Programming"}},
	{10, "regular-expression-matching", "Regular Expression Matching", Hard, []string{"2-D Dynamic Programming"}},
	{53, "maximum-subarray", "Maximum Subarray", Medium, []string{"Greedy"}},
	{55, "jump-game", "Jump Game", Medium, []string{"Greedy"}},
	{45, "jump-game-ii", "Jump Game II", Medium, []string{"Greedy"}},
	{134, "gas-station", "Gas Station", Medium, []string{"Greedy"}},
	{846, "hand-of-straights", "Hand of Straights", Medium, []string{"Greedy"}},
	{1899, "merge-triplets-to-form-target-triplet", "Merge Triplets to Form Target Triplet", Medium, []string{"Greedy"}},
	{763, "partition-labels", "Partition Labels", Medium, []string{"Greedy"}},
	{678, "valid-parenthesis-string", "Valid Parenthesis String", Medium, []string{"Greedy"}},
	{57, "insert-interval", "Insert Interval", Medium, []string{"Intervals"}},
	{56, "merge-intervals", "Merge Intervals", Medium, []string{"Intervals"}},
	{435, "non-overlapping-intervals", "Non-overlapping Intervals", Medium, []string{"Intervals"}},
	{252, "meeting-rooms", "Meeting Rooms", Easy, []string{"Intervals"}},
	{253, "meeting-rooms-ii", "Meeting Rooms II", Medium, []string{"Intervals"}},
	{1851, "minimum-interval-to-include-each-query", "Minimum Interval to Include Each Query", Hard, []string{"Intervals"}},
	{48, "rotate-image", "Rotate Image", Medium, []string{"Math & Geometry"}},
	{54, "spiral-matrix", "Spiral Matrix", Medium, []string{"Math & Geometry"}},
	{73, "set-matrix-zeroes", "Set Matrix Zeroes", Medium, []string{"Math & Geometry"}},
	{202, "happy-number", "Happy Number", Easy, []string{"Math & Geometry"}},
	{66, "plus-one", "Plus One", Easy, []string{"Math & Geometry"}},
	{50, "powx-n", "Pow(x, n)", Medium, []string{"Math & Geometry"}},
	{43, "multiply-strings", "Multiply Strings", Medium, []string{"Math & Geometry"}},
	{2013, "detect-squares", "Detect Squares", Medium, []string{"Math & Geometry", "Design"}},
	{136, "single-number", "Single Number", Easy, []string{"Bit Manipulation"}},
	{191, "number-of-1-bits", "Number of 1 Bits", Easy, []string{"Bit Manipulation"}},
	{338, "counting-bits", "Counting Bits", Easy, []string{"Bit Manipulation"}},
	{190, "reverse-bits", "Reverse Bits", Easy, []string{"Bit Manipulation"}},
	{268, "missing-number", "Missing Number", Easy, []string{"Bit Manipulation"}},
	{371, "sum-of-two-integers", "Sum of Two Integers", Medium, []string{"Bit Manipulation"}},
	{7, "reverse-integer", "Reverse Integer", Medium, []string{"Bit Manipulation"}},
}

var neetcode150 = []string{
	"contains-duplicate", "valid-anagram", "two-sum", "group-anagrams", "top-k-frequent-elements", "encode-and-decode-strings", "product-of-array-except-self", "valid-sudoku", "longest-consecutive-sequence",
	"valid-palindrome", "two-sum-ii-input-array-is-sorted", "3sum", "container-with-most-water", "trapping-rain-water",
	"best-time-to-buy-and-sell-stock", "longest-substring-without-repeating-characters", "longest-repeating-character-replacement", "permutation-in-string", "minimum-window-substring", "sliding-window-maximum",
	"valid-parentheses", "min-stack", "evaluate-reverse-polish-notation", "generate-parentheses", "daily-temperatures", "car-fleet", "largest-rectangle-in-histogram",
	"binary-search", "search-a-2d-matrix", "koko-eating-bananas", "find-minimum-in-rotated-sorted-array", "search-in-rotated-sorted-array", "time-based-key-value-store", "median-of-two-sorted-arrays",
	"reverse-linked-list", "merge-two-sorted-lists", "reorder-list", "remove-nth-node-from-end-of-list", "copy-list-with-random-pointer", "add-two-numbers", "linked-list-cycle", "find-the-duplicate-number", "lru-cache", "merge-k-sorted-lists", "reverse-nodes-in-k-group",
	"invert-binary-tree", "maximum-depth-of-binary-tree", "diameter-of-binary-tree", "balanced-binary-tree", "same-tree", "subtree-of-another-tree", "lowest-common-ancestor-of-a-binary-search-tree", "binary-tree-level-order-traversal", "binary-tree-right-side-view", "count-good-nodes-in-binary-tree", "validate-binary-search-tree", "kth-smallest-element-in-a-bst", "construct-binary-tree-from-preorder-and-inorder-traversal", "binary-tree-maximum-path-sum", "serialize-and-deserialize-binary-tree",
	"implement-trie-prefix-tree", "design-add-and-search-words-data-structure", "word-search-ii",
	"kth-largest-element-in-a-stream", "last-stone-weight", "k-closest-points-to-origin", "kth-largest-element-in-an-array", "task-scheduler", "design-twitter", "find-median-from-data-stream",
	"subsets", "combination-sum", "permutations", "subsets-ii", "combination-sum-ii", "word-search", "palindrome-partitioning", "letter-combinations-of-a-phone-number", "n-queens",
	"number-of-islands", "clone-graph", "max-area-of-island", "pacific-atlantic-water-flow", "surrounded-regions", "rotting-oranges", "walls-and-gates", "redundant-connection", "course-schedule", "course-schedule-ii", "graph-valid-tree", "number-of-connected-components-in-an-undirected-graph",
	"reconstruct-itinerary", "min-cost-to-connect-all-points", "network-delay-time", "swim-in-rising-water", "alien-dictionary", "cheapest-flights-within-k-stops", "word-ladder",
	"climbing-stairs", "min-cost-climbing-stairs", "house-robber", "house-robber-ii", "longest-palindromic-substring", "palindromic-substrings", "decode-ways", "coin-change", "maximum-product-subarray", "word-break", "longest-increasing-subsequence", "partition-equal-subset-sum",
	"unique-paths", "longest-common-subsequence", "best-time-to-buy-and-sell-stock-with-cooldown", "coin-change-ii", "target-sum", "interleaving-string", "longest-increasing-path-in-a-matrix", "distinct-subsequences", "edit-distance", "burst-balloons", "regular-expression-matching",
	"maximum-subarray", "jump-game", "jump-game-ii", "gas-station", "hand-of-straights", "merge-triplets-to-form-target-triplet", "partition-labels", "valid-parenthesis-string",
	"insert-interval", "merge-intervals", "non-overlapping-intervals", "meeting-rooms", "meeting-rooms-ii", "minimum-interval-to-include-each-query",
	"rotate-image", "spiral-matrix", "set-matrix-zeroes", "happy-number", "plus-one", "powx-n", "multiply-strings", "detect-squares",
	"single-number", "number-of-1-bits", "counting-bits", "reverse-bits", "missing-number", "sum-of-two-integers", "reverse-integer",
}

var blind75 = []string{
	"two-sum", "best-time-to-buy-and-sell-stock", "contains-duplicate", "product-of-array-except-self", "maximum-subarray", "maximum-product-subarray", "find-minimum-in-rotated-sorted-array", "search-in-rotated-sorted-array", "3sum", "container-with-most-water",
	"sum-of-two-integers", "number-of-1-bits", "counting-bits", "missing-number", "reverse-bits",
	"climbing-stairs", "coin-change", "longest-increasing-subsequence", "longest-common-subsequence", "word-break", "combination-sum", "house-robber", "house-robber-ii", "decode-ways", "unique-paths", "jump-game",
	"clone-graph", "course-schedule", "pacific-atlantic-water-flow", "number-of-islands", "longest-consecutive-sequence", "alien-dictionary", "graph-valid-tree", "number-of-connected-components-in-an-undirected-graph",
	"insert-interval", "merge-intervals", "non-overlapping-intervals", "meeting-rooms", "meeting-rooms-ii",
	"reverse-linked-list", "linked-list-cycle", "merge-two-sorted-lists", "merge-k-sorted-lists", "remove-nth-node-from-end-of-list", "reorder-list",
	"set-matrix-zeroes", "spiral-matrix", "rotate-image", "word-search",
	"longest-substring-without-repeating-characters", "longest-repeating-character-replacement", "minimum-window-substring", "valid-anagram", "group-anagrams", "valid-parentheses", "valid-palindrome", "longest-palindromic-substring", "palindromic-substrings",
	"maximum-depth-of-binary-tree", "same-tree", "invert-binary-tree", "binary-tree-maximum-path-sum", "binary-tree-level-order-traversal", "serialize-and-deserialize-binary-tree", "subtree-of-another-tree", "construct-binary-tree-from-preorder-and-inorder-traversal", "validate-binary-search-tree", "kth-smallest-element-in-a-bst", "lowest-common-ancestor-of-a-binary-search-tree", "implement-trie-prefix-tree", "design-add-and-search-words-data-structure", "word-search-ii",
	"top-k-frequent-elements", "find-median-from-data-stream", "kth-largest-element-in-an-array",
}
