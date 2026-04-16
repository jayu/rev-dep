package resolve

import (
	"path/filepath"
	"testing"

	"rev-dep-go/internal/model"
	"rev-dep-go/internal/pathutil"
	"rev-dep-go/internal/testutil"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	return root
}

func toRepoRelativePath(t *testing.T, p string) string {
	t.Helper()
	if p == "" {
		return ""
	}
	denorm := pathutil.DenormalizePathForOS(p)
	if !filepath.IsAbs(denorm) {
		return filepath.ToSlash(denorm)
	}
	root := repoRoot(t)
	rel, err := filepath.Rel(root, denorm)
	if err != nil {
		return filepath.ToSlash(denorm)
	}
	return filepath.ToSlash(rel)
}

func normalizeTreeRelative(t *testing.T, tree model.MinimalDependencyTree) model.MinimalDependencyTree {
	t.Helper()
	out := make(model.MinimalDependencyTree, len(tree))
	for file, deps := range tree {
		relFile := toRepoRelativePath(t, file)
		relDeps := make([]model.MinimalDependency, len(deps))
		for i, dep := range deps {
			dep.ID = toRepoRelativePath(t, dep.ID)
			relDeps[i] = dep
		}
		out[relFile] = relDeps
	}
	return out
}

func normalizeListRelative(t *testing.T, files []string) []string {
	t.Helper()
	if len(files) == 0 {
		return files
	}
	out := make([]string, len(files))
	for i, file := range files {
		out[i] = toRepoRelativePath(t, file)
	}
	return out
}

func getMinimalDepsTreeForCwdRel(
	t *testing.T,
	cwd string,
	ignoreTypeImports bool,
	excludeFiles []string,
	upfrontFilesList []string,
	packageJson string,
	tsconfigJson string,
	conditionNames []string,
	followMonorepoPackages model.FollowMonorepoPackagesValue,
	customAssetExtensions []string,
) (model.MinimalDependencyTree, []string, *ResolverManager) {
	t.Helper()
	absCwd := cwd
	if !filepath.IsAbs(cwd) {
		absCwd = filepath.Join(repoRoot(t), cwd)
	}
	tree, sortedFiles, manager := GetMinimalDepsTreeForCwd(absCwd, ignoreTypeImports, excludeFiles, nil, upfrontFilesList, packageJson, tsconfigJson, conditionNames, followMonorepoPackages, customAssetExtensions)
	return normalizeTreeRelative(t, tree), normalizeListRelative(t, sortedFiles), manager
}
