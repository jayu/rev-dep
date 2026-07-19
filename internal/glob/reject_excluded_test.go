package globutil

import (
	"fmt"
	"slices"
	"testing"
)

// TestRejectExcludedMatchesIsExcludedByPatterns is the correctness anchor: RejectExcluded is
// an optimised, sharded rewrite of a plain filter loop, so it must agree with
// IsExcludedByPatterns path for path. The input deliberately straddles
// parallelFilterThreshold so both the inline and the sharded branch are exercised.
func TestRejectExcludedMatchesIsExcludedByPatterns(t *testing.T) {
	root := "/repo/"
	excludes := CreateGlobMatchers([]string{"**/*.test.ts", "dist/**", "node_modules"}, root)
	includes := CreateGlobMatchers([]string{"dist/keep/**"}, root)

	sizes := []int{0, 1, 10, parallelFilterThreshold - 1, parallelFilterThreshold, parallelFilterThreshold + 1, parallelFilterThreshold*3 + 7}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size-%d", size), func(t *testing.T) {
			paths := syntheticPaths(root, size)

			want := make([]string, 0, len(paths))
			for _, path := range paths {
				if !IsExcludedByPatterns(path, excludes, includes) {
					want = append(want, path)
				}
			}

			got := RejectExcluded(paths, excludes, includes)

			if !slices.Equal(want, got) {
				t.Errorf("RejectExcluded disagrees with IsExcludedByPatterns: got %d kept, want %d", len(got), len(want))
				for i := 0; i < min(len(want), len(got)); i++ {
					if want[i] != got[i] {
						t.Errorf("first divergence at %d: got %s, want %s", i, got[i], want[i])
						break
					}
				}
			}
		})
	}
}

// TestRejectExcludedIsDeterministic guards against shard interleaving leaking into the
// result. Sharding only kicks in above parallelFilterThreshold, so use an input past it.
func TestRejectExcludedIsDeterministic(t *testing.T) {
	root := "/repo/"
	excludes := CreateGlobMatchers([]string{"**/*.test.ts", "dist/**"}, root)
	paths := syntheticPaths(root, parallelFilterThreshold*2)

	first := RejectExcluded(paths, excludes, nil)
	for run := 1; run < 25; run++ {
		got := RejectExcluded(paths, excludes, nil)
		if !slices.Equal(first, got) {
			t.Fatalf("RejectExcluded output varies between runs (run %d): %d vs %d kept", run, len(got), len(first))
		}
	}
}

// TestRejectExcludedPreservesOrder checks the documented ordering guarantee explicitly,
// rather than only as a side effect of the equivalence test.
func TestRejectExcludedPreservesOrder(t *testing.T) {
	root := "/repo/"
	excludes := CreateGlobMatchers([]string{"**/*.test.ts"}, root)

	paths := []string{
		"/repo/z.ts",
		"/repo/a.test.ts",
		"/repo/m.ts",
		"/repo/b.test.ts",
		"/repo/a.ts",
	}
	want := []string{"/repo/z.ts", "/repo/m.ts", "/repo/a.ts"}

	if got := RejectExcluded(paths, excludes, nil); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v (input order must be preserved, not sorted)", got, want)
	}
}

// TestRejectExcludedNeverAliasesInput pins the contract that the result is always a fresh
// slice. Callers treat it as their own while the input may still be appended to elsewhere;
// returning the caller's backing array would let one write through the other.
func TestRejectExcludedNeverAliasesInput(t *testing.T) {
	// Each subtest builds its own input: assertNoAliasing probes the slice's spare capacity,
	// and an aliasing failure writes into it, so a shared fixture would carry the first
	// subtest's pollution into the second and report a false positive there.
	newInput := func() []string {
		paths := make([]string, 3, 16) // spare capacity: an append would land in the same array
		copy(paths, []string{"/repo/a.ts", "/repo/b.ts", "/repo/c.ts"})
		return paths
	}

	t.Run("no exclude patterns", func(t *testing.T) {
		paths := newInput()
		got := RejectExcluded(paths, nil, nil)
		if !slices.Equal(got, paths) {
			t.Fatalf("with no exclude patterns everything must be kept, got %v", got)
		}
		assertNoAliasing(t, paths, got)
	})

	t.Run("with exclude patterns matching nothing", func(t *testing.T) {
		paths := newInput()
		excludes := CreateGlobMatchers([]string{"**/*.md"}, "/repo/") // matches nothing here
		got := RejectExcluded(paths, excludes, nil)
		if !slices.Equal(got, paths) {
			t.Fatalf("non-matching patterns must keep everything, got %v", got)
		}
		assertNoAliasing(t, paths, got)
	})
}

// assertNoAliasing verifies that appending to one slice cannot be observed through the other.
func assertNoAliasing(t *testing.T, input, result []string) {
	t.Helper()
	if len(result) == 0 {
		return
	}
	grown := append(result, "/repo/appended.ts")
	if len(input) > 0 && grown[0] != input[0] {
		t.Fatalf("sanity: contents diverged unexpectedly")
	}
	// If result shared input's backing array, appending would have written into input's
	// spare capacity; re-slicing input to that length would then reveal the new element.
	if cap(input) > len(input) {
		spare := input[:len(input)+1]
		if spare[len(input)] == "/repo/appended.ts" {
			t.Errorf("RejectExcluded returned a slice aliasing its input: an append wrote through to the caller's array")
		}
	}
}

// syntheticPaths builds a deterministic path set mixing kept files, excluded files, and
// files re-included by an include pattern.
func syntheticPaths(root string, size int) []string {
	paths := make([]string, 0, size)
	for i := 0; i < size; i++ {
		switch i % 5 {
		case 0:
			paths = append(paths, fmt.Sprintf("%ssrc/mod%d.ts", root, i))
		case 1:
			paths = append(paths, fmt.Sprintf("%ssrc/mod%d.test.ts", root, i))
		case 2:
			paths = append(paths, fmt.Sprintf("%sdist/bundle%d.js", root, i))
		case 3:
			paths = append(paths, fmt.Sprintf("%sdist/keep/asset%d.ts", root, i))
		default:
			paths = append(paths, fmt.Sprintf("%slib/nested/deep/util%d.ts", root, i))
		}
	}
	return paths
}
