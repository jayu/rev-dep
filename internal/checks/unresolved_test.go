package checks

import (
	"testing"

	globutil "rev-dep-go/internal/glob"
)

// unresolvedTree builds a tree with three unresolved imports across two files:
//   - src/a.ts imports "missing-pkg-one" and "./local-missing"
//   - src/b.ts imports "missing-pkg-one"
//
// "resolved-dep" is a resolved node module and must never be reported.
func unresolvedTree() MinimalDependencyTree {
	return MinimalDependencyTree{
		"/repo/src/a.ts": {
			{Request: "missing-pkg-one", ResolvedType: NotResolvedModule},
			{Request: "./local-missing", ResolvedType: NotResolvedModule},
			{Request: "resolved-dep", ResolvedType: NodeModule},
		},
		"/repo/src/b.ts": {
			{Request: "missing-pkg-one", ResolvedType: NotResolvedModule},
			{Request: "someVar", ResolvedType: NonLiteralModule},
		},
	}
}

func hasUnresolved(list []UnresolvedImport, filePath, request string) bool {
	for _, u := range list {
		if u.FilePath == filePath && u.Request == request {
			return true
		}
	}
	return false
}

func TestDetectUnresolvedImports_ReportsAllUnresolved(t *testing.T) {
	got := DetectUnresolvedImports(unresolvedTree(), nil, false)
	if len(got) != 3 {
		t.Fatalf("expected 3 unresolved imports, got %d: %+v", len(got), got)
	}
	if !hasUnresolved(got, "/repo/src/a.ts", "missing-pkg-one") ||
		!hasUnresolved(got, "/repo/src/a.ts", "./local-missing") ||
		!hasUnresolved(got, "/repo/src/b.ts", "missing-pkg-one") {
		t.Fatalf("missing expected unresolved imports: %+v", got)
	}
	if hasUnresolved(got, "/repo/src/a.ts", "resolved-dep") {
		t.Fatalf("resolved node module should not be reported: %+v", got)
	}
}

// A non-literal import is reported only when the option asks for it, so a project that passes
// today does not start failing.
func TestDetectUnresolvedImports_NonLiteralIsOptIn(t *testing.T) {
	off := DetectUnresolvedImports(unresolvedTree(), nil, false)
	if hasUnresolved(off, "/repo/src/b.ts", "someVar") {
		t.Fatalf("non-literal import reported with the option off: %+v", off)
	}
	if len(off) != 3 {
		t.Fatalf("expected 3 unresolved imports with the option off, got %+v", off)
	}

	on := DetectUnresolvedImports(unresolvedTree(), nil, true)
	if !hasUnresolved(on, "/repo/src/b.ts", "someVar") {
		t.Fatalf("non-literal import not reported with the option on: %+v", on)
	}
	if len(on) != 4 {
		t.Fatalf("expected 4 unresolved imports with the option on, got %+v", on)
	}
	for _, u := range on {
		if u.Request == "someVar" && !u.IsNonLiteral {
			t.Fatalf("non-literal import is not marked as one: %+v", u)
		}
	}
}

// ignoreImports matches the import request globally, regardless of the importing file.
func TestFilterUnresolvedImports_IgnoreImports(t *testing.T) {
	unresolved := DetectUnresolvedImports(unresolvedTree(), nil, false)

	opts := &UnresolvedFilterOptions{
		IgnoreImports: []string{"missing-pkg-*"},
	}
	got := FilterUnresolvedImports(unresolved, opts, "/repo")

	if len(got) != 1 {
		t.Fatalf("expected 1 unresolved import after ignoreImports, got %d: %+v", len(got), got)
	}
	if !hasUnresolved(got, "/repo/src/a.ts", "./local-missing") {
		t.Fatalf("expected only ./local-missing to survive, got %+v", got)
	}
}

// ignoreFiles suppresses every unresolved import originating from a matching file.
func TestFilterUnresolvedImports_IgnoreFiles(t *testing.T) {
	unresolved := DetectUnresolvedImports(unresolvedTree(), nil, false)

	opts := &UnresolvedFilterOptions{
		IgnoreFiles: []string{"src/a.ts"},
	}
	got := FilterUnresolvedImports(unresolved, opts, "/repo")

	if len(got) != 1 {
		t.Fatalf("expected 1 unresolved import after ignoreFiles, got %d: %+v", len(got), got)
	}
	if !hasUnresolved(got, "/repo/src/b.ts", "missing-pkg-one") {
		t.Fatalf("expected only src/b.ts import to survive, got %+v", got)
	}
}

// The ignore map suppresses an import only when BOTH the file glob and the value
// glob match. The same request from a non-matching file must NOT be suppressed.
func TestFilterUnresolvedImports_IgnoreFileValueMap(t *testing.T) {
	unresolved := DetectUnresolvedImports(unresolvedTree(), nil, false)

	opts := &UnresolvedFilterOptions{
		Ignore: globutil.FileValueIgnoreMap{
			"src/a.ts": []string{"missing-pkg-one"},
		},
	}
	got := FilterUnresolvedImports(unresolved, opts, "/repo")

	if len(got) != 2 {
		t.Fatalf("expected 2 unresolved imports after ignore map, got %d: %+v", len(got), got)
	}
	// a.ts + missing-pkg-one is the only pair that matches both file and value.
	if hasUnresolved(got, "/repo/src/a.ts", "missing-pkg-one") {
		t.Fatalf("expected src/a.ts + missing-pkg-one to be ignored, got %+v", got)
	}
	// The same request from b.ts must survive - the file glob does not match it.
	if !hasUnresolved(got, "/repo/src/b.ts", "missing-pkg-one") {
		t.Fatalf("ignore map over-filtered: src/b.ts + missing-pkg-one should survive, got %+v", got)
	}
	if !hasUnresolved(got, "/repo/src/a.ts", "./local-missing") {
		t.Fatalf("ignore map over-filtered: src/a.ts + ./local-missing should survive, got %+v", got)
	}
}
