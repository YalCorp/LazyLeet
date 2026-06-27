package catalog

import "testing"

func TestLoadTracks(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	blind, ok := c.Track("blind-75")
	if !ok {
		t.Fatal("Blind 75 track missing")
	}
	if len(blind.Problems) != 75 {
		t.Fatalf("Blind 75 has %d problems, want 75", len(blind.Problems))
	}

	neetcode, ok := c.Track("neetcode-150")
	if !ok {
		t.Fatal("NeetCode 150 track missing")
	}
	if len(neetcode.Problems) != 150 {
		t.Fatalf("NeetCode 150 has %d problems, want 150", len(neetcode.Problems))
	}
}

func TestCanonicalProblemMembership(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	problem, ok := c.Problem("two-sum")
	if !ok {
		t.Fatal("two-sum missing")
	}
	if len(problem.Tracks) != 2 {
		t.Fatalf("two-sum track count = %d, want 2", len(problem.Tracks))
	}
	if len(c.Problems) != 150 {
		t.Fatalf("canonical problems = %d, want 150 because Blind 75 is resolved into shared NeetCode entries", len(c.Problems))
	}
}
