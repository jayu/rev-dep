package checks

import (
	"testing"
)

// Config-execution-level coverage: the per-rule checks compile their glob
// properties against the rule's workspace path (fullRulePath) as the root. An
// unanchored entry-point pattern like "**/main.ts" must therefore only match
// files inside that workspace - a followed file from a sibling workspace must
// not be picked up as an entry point.
//
// This isolates entry-point glob scoping inside the real FindOrphanFiles check:
// the foreign, unreferenced file should be reported as an orphan precisely
// because the entry-point pattern must NOT match it. On the current (buggy)
// implementation "**/main.ts" matches the foreign absolute path, the foreign
// file is treated as an entry point, and it is wrongly excluded from orphans.
func TestFindOrphanFiles_EntryPointsScopedToWorkspace(t *testing.T) {
	cwd := "/repo/apps/web/" // the rule's workspace (fullRulePath)

	webEntry := "/repo/apps/web/src/main.ts"
	webDead := "/repo/apps/web/src/dead.ts"
	foreignMain := "/repo/apps/mobile/src/main.ts" // followed in from a sibling workspace

	tree := MinimalDependencyTree{
		webEntry:    {},
		webDead:     {},
		foreignMain: {},
	}

	orphans := FindOrphanFiles(tree, []string{"**/main.ts"}, nil, false, cwd, nil)

	has := func(p string) bool {
		for _, o := range orphans {
			if o == p {
				return true
			}
		}
		return false
	}

	// In-workspace entry point is correctly NOT an orphan.
	if has(webEntry) {
		t.Errorf("in-workspace entry %q should not be an orphan", webEntry)
	}
	// In-workspace dead file is an orphan (sanity).
	if !has(webDead) {
		t.Errorf("in-workspace dead file %q should be an orphan", webDead)
	}
	// Foreign file must NOT be treated as an entry point by this workspace's
	// "**/main.ts" pattern, so being unreferenced it should surface as an orphan.
	if !has(foreignMain) {
		t.Errorf("foreign-workspace file %q was matched by '**/main.ts' as an entry point (cross-workspace glob leak)", foreignMain)
	}
}
