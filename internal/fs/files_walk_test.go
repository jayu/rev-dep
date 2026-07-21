package fs

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/pathutil"
)

// referenceWalk is the sequential depth-first walk that getFiles replaced, kept verbatim as
// a test oracle. getFiles is now a concurrent breadth-first traversal that reconstructs this
// walk's emission order by sorting; that reconstruction is the thing most likely to break
// silently, and `list-cwd-files` prints the order straight to users. Diffing against this
// reference pins the behaviour without anyone having to hand-maintain a golden list.
//
// The one intentional divergence is .git, which getFiles skips - see
// TestWalkSkipsGitDirectory.
func referenceWalk(directory string, existingFiles []string, parentGlobMatchers []globutil.GlobMatcher, includeMatchers []globutil.GlobMatcher, includePrefixes []string) []string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return existingFiles
	}

	for _, entry := range entries {
		entryName := entry.Name()
		entryFilePath := filepath.Join(directory, entryName)

		if entry.IsDir() {
			if globutil.ShouldTraverseDir(entryFilePath, parentGlobMatchers, includeMatchers, includePrefixes) {
				gitignoreFile, gitignoreError := os.ReadFile(filepath.Join(entryFilePath, ".gitignore"))

				ignoreGlobs := []globutil.GlobMatcher{}
				if gitignoreError == nil {
					ignoreGlobs = ParseGitIgnore(string(gitignoreFile), entryFilePath)
				}
				if len(ignoreGlobs) > 0 {
					ignoreGlobs = append(slices.Clone(parentGlobMatchers), ignoreGlobs...)
				} else {
					ignoreGlobs = parentGlobMatchers
				}

				existingFiles = referenceWalk(entryFilePath, existingFiles, ignoreGlobs, includeMatchers, includePrefixes)
			}
			continue
		}

		if !hasCorrectExtension(entryName) {
			continue
		}
		if globutil.IsExcludedByPatterns(entryFilePath, parentGlobMatchers, includeMatchers) {
			continue
		}
		existingFiles = append(existingFiles, pathutil.NormalizePathForInternal(entryFilePath))
	}

	return existingFiles
}

// writeTree materialises a map of relative path -> contents under a fresh temp dir.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return root
}

// orderingFixture exercises the cases where a plain byte sort of full paths diverges from
// depth-first emission order, plus sibling directories carrying their own .gitignore.
func orderingFixture() map[string]string {
	files := map[string]string{
		// '/' must rank below '.' and '-' so a directory's contents precede siblings that
		// sort after the bare directory name: a/b.ts < a-b/y.ts < a.ts < ab.ts.
		"a/b.ts":   "x",
		"a-b/y.ts": "x",
		"a.ts":     "x",
		"ab.ts":    "x",
		// deep nesting under a name that is also a file prefix
		"z/deep/deeper/leaf.ts": "x",
		"z/deep/leaf.ts":        "x",
		"z.ts":                  "x",
		// a gitignore with a negation
		"neg/.gitignore":   "*.ts\n!important.ts\n",
		"neg/other.ts":     "x",
		"neg/important.ts": "x",
		// non-source extensions must be ignored entirely
		"src/readme.md":  "x",
		"src/styles.css": "x",
	}
	// Sibling directories each ignoring only their own file. If the inherited matcher slice
	// were appended into in place, siblings walked concurrently would clobber each other's
	// patterns and the wrong files would survive.
	for _, name := range []string{"alpha", "beta", "gamma", "delta"} {
		files[name+"/.gitignore"] = "ignored-" + name + ".ts\n"
		files[name+"/keep.ts"] = "x"
		files[name+"/ignored-"+name+".ts"] = "x"
		for _, other := range []string{"alpha", "beta", "gamma", "delta"} {
			files[name+"/sub/ignored-"+other+".ts"] = "x"
		}
	}
	return files
}

// TestWalkMatchesReferenceOrder is the core regression guard: the concurrent walk must
// reproduce the sequential walk's output exactly, contents and order.
func TestWalkMatchesReferenceOrder(t *testing.T) {
	root := writeTree(t, orderingFixture())

	want := referenceWalk(root, nil, nil, nil, globutil.BuildIncludePrefixes(nil))
	got := GetFiles(root, nil, nil, nil)

	if !slices.Equal(want, got) {
		t.Errorf("walk output differs from the sequential reference\n want (%d): %v\n  got (%d): %v", len(want), want, len(got), got)
	}
}

// TestWalkMatchesReferenceOrderOnRepo runs the same comparison over this repository, which
// has real nesting, real .gitignore files and enough directories to force fan-out across
// multiple workers.
func TestWalkMatchesReferenceOrderOnRepo(t *testing.T) {
	root := "../.."
	gitIgnoreMatchers := FindAndProcessGitIgnoreFilesUpToRepoRoot(root)

	want := referenceWalk(root, nil, gitIgnoreMatchers, nil, globutil.BuildIncludePrefixes(nil))
	// The reference walks .git; getFiles skips it. Drop those to compare like for like.
	want = slices.DeleteFunc(want, func(path string) bool {
		return strings.Contains(path, "/.git/")
	})
	got := GetFiles(root, nil, gitIgnoreMatchers, nil)

	if !slices.Equal(want, got) {
		t.Errorf("walk output differs from the sequential reference on the repo tree\n want %d files, got %d", len(want), len(got))
		for i := 0; i < min(len(want), len(got)); i++ {
			if want[i] != got[i] {
				t.Errorf("first divergence at %d:\n want %s\n  got %s", i, want[i], got[i])
				break
			}
		}
	}
}

// TestWalkIsDeterministic guards the concurrent walk against order leaking from whichever
// worker happens to finish first.
func TestWalkIsDeterministic(t *testing.T) {
	root := writeTree(t, orderingFixture())

	first := GetFiles(root, nil, nil, nil)
	for run := 1; run < 50; run++ {
		got := GetFiles(root, nil, nil, nil)
		if !slices.Equal(first, got) {
			t.Fatalf("walk output varies between runs (run %d)\n first: %v\n   got: %v", run, first, got)
		}
	}
}

// TestWalkSkipsGitDirectory pins the one deliberate divergence from the sequential walk.
func TestWalkSkipsGitDirectory(t *testing.T) {
	root := writeTree(t, map[string]string{
		".git/config.ts":      "x",
		".git/hooks/thing.ts": "x",
		"src/index.ts":        "x",
	})

	got := GetFiles(root, nil, nil, nil)

	want := []string{pathutil.NormalizePathForInternal(filepath.Join(root, "src/index.ts"))}
	if !slices.Equal(got, want) {
		t.Errorf("expected .git to be skipped entirely, got %v", got)
	}
}

// TestWalkAppendsToExistingFiles checks that discovered paths are appended to the caller's
// slice rather than replacing it, and that only the discovered portion is sorted.
func TestWalkAppendsToExistingFiles(t *testing.T) {
	root := writeTree(t, map[string]string{"b.ts": "x", "a.ts": "x"})

	got := GetFiles(root, []string{"zzz-preexisting.ts"}, nil, nil)

	if len(got) != 3 || got[0] != "zzz-preexisting.ts" {
		t.Fatalf("existing files must be preserved at the front, got %v", got)
	}
	if !slices.IsSortedFunc(got[1:], comparePathsDepthFirst) {
		t.Errorf("discovered portion is not sorted: %v", got[1:])
	}
}

// TestComparePathsDepthFirst covers the comparator directly, including the separator
// ranking that a plain byte comparison gets wrong.
func TestComparePathsDepthFirst(t *testing.T) {
	cases := []struct {
		name  string
		left  string
		right string
		want  int // -1 left first, 1 right first, 0 equal
	}{
		{"separator outranks dot", "a/b.ts", "a.ts", -1},
		{"separator outranks letter", "a/c.ts", "ab.ts", -1},
		{"separator outranks dash", "a/x.ts", "a-b/y.ts", -1},
		{"dash before dot", "a-b/y.ts", "a.ts", -1},
		{"plain sibling order", "a.ts", "b.ts", -1},
		{"deeper nesting first", "z/deep/deeper/leaf.ts", "z/deep/leaf.ts", -1},
		{"subdir before sibling file", "z/deep/leaf.ts", "z.ts", -1},
		{"identical", "same.ts", "same.ts", 0},
		{"prefix is shorter", "a.ts", "a.tsx", -1},
		{"case is byte order", "A.ts", "a.ts", -1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := comparePathsDepthFirst(c.left, c.right)
			if sign(got) != c.want {
				t.Errorf("comparePathsDepthFirst(%q, %q) = %d, want sign %d", c.left, c.right, got, c.want)
			}
			// Antisymmetry: reversing the arguments must reverse the sign.
			if reversed := comparePathsDepthFirst(c.right, c.left); sign(reversed) != -c.want {
				t.Errorf("comparePathsDepthFirst(%q, %q) = %d, want sign %d (antisymmetry)", c.right, c.left, reversed, -c.want)
			}
		})
	}
}

// TestComparePathsDepthFirstSortsLikeReference sorts a shuffled path set and checks it lands
// in the order the depth-first walk would have emitted it.
func TestComparePathsDepthFirstSortsLikeReference(t *testing.T) {
	want := []string{
		"a/b.ts",
		"a/c/d.ts",
		"a-b/y.ts",
		"a.ts",
		"ab.ts",
		"z/deep/deeper/leaf.ts",
		"z/deep/leaf.ts",
		"z.ts",
	}

	shuffled := []string{"z.ts", "ab.ts", "a/c/d.ts", "a.ts", "z/deep/leaf.ts", "a-b/y.ts", "a/b.ts", "z/deep/deeper/leaf.ts"}
	slices.SortFunc(shuffled, comparePathsDepthFirst)

	if !slices.Equal(shuffled, want) {
		t.Errorf("sorted order\n got: %v\nwant: %v", shuffled, want)
	}
}

// TestAllowedExtsDerivedFromOrderedExts pins the two extension lists together. They used to
// be maintained separately, where adding an extension to only one would either discover
// files that never resolve or resolve files that are never discovered - both silent.
func TestAllowedExtsDerivedFromOrderedExts(t *testing.T) {
	if len(allowedExts) != len(orderedExts) {
		t.Fatalf("allowedExts has %d entries, orderedExts has %d", len(allowedExts), len(orderedExts))
	}
	for _, ext := range orderedExts {
		if _, ok := allowedExts[ext]; !ok {
			t.Errorf("%s is in orderedExts but not allowedExts", ext)
		}
	}
	if len(slices.Compact(slices.Sorted(slices.Values(orderedExts)))) != len(orderedExts) {
		t.Errorf("orderedExts contains duplicates: %v", orderedExts)
	}
	for _, ext := range orderedExts {
		if !hasCorrectExtension("file" + ext) {
			t.Errorf("hasCorrectExtension rejects %s, which orderedExts declares supported", ext)
		}
	}
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}
