package resolve

import (
	"os"
	"path/filepath"
	"testing"

	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/model"
	"rev-dep-go/internal/pathutil"
	"rev-dep-go/internal/testutil"
)

// tempCwd returns a fresh temp directory in internal (forward-slash) form with a trailing
// slash, so tests can build file paths as cwd+"src/...". The ResolverManager now reads
// package.json / tsconfig.json from disk, so unit tests seed them there via rmFromContent.
func tempCwd(t *testing.T) string {
	t.Helper()
	return pathutil.StandardiseDirPathInternal(pathutil.NormalizePathForInternal(t.TempDir()))
}

// rmFromContent writes the given tsconfig / package.json content into cwd's directory and
// builds a ResolverManager rooted there. Either content may be empty to skip that file.
// It replaces the old pattern of injecting pre-parsed content through RootParams.
func rmFromContent(t *testing.T, conditionNames []string, tsConfigContent, pkgJsonContent []byte, sortedFiles []string, cwd string) *ResolverManager {
	t.Helper()
	dir := pathutil.DenormalizePathForOS(cwd)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if len(tsConfigContent) > 0 {
		if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), tsConfigContent, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if len(pkgJsonContent) > 0 {
		if err := os.WriteFile(filepath.Join(dir, "package.json"), pkgJsonContent, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return NewResolverManager(model.FollowMonorepoPackagesValue{}, conditionNames, ResolverManagerInput{
		Cwd:         cwd,
		SortedFiles: sortedFiles,
	}, []globutil.GlobMatcher{}, nil)
}

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
	tree, sortedFiles, manager := GetMinimalDepsTreeForCwd(absCwd, ignoreTypeImports, excludeFiles, nil, upfrontFilesList, tsconfigJson, conditionNames, followMonorepoPackages, customAssetExtensions, model.NodeModulesMatchingStrategyCwdResolver)
	return normalizeTreeRelative(t, tree), normalizeListRelative(t, sortedFiles), manager
}
