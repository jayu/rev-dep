package node

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestFindNodeModuleBinaries runs the real function against a temp node_modules tree, so it
// covers the package.json read and decode too, not just parseBinField.
func TestFindNodeModuleBinaries(t *testing.T) {
	cases := []struct {
		moduleName string
		pkgJson    string
		want       []string
	}{
		// Scoped package with a string bin: npm installs .bin/biome, so the scope must go.
		{
			moduleName: "@biomejs/biome",
			pkgJson:    `{"name":"@biomejs/biome","dependencies":{"a":"1"},"bin":"./bin/biome","engines":{"node":">=18"}}`,
			want:       []string{"biome"},
		},
		{
			moduleName: "rimraf",
			pkgJson:    `{"name":"rimraf","bin":"./bin.js"}`,
			want:       []string{"rimraf"},
		},
		// Object form: the keys are the authored names, scope is irrelevant.
		{
			moduleName: "@scope/multi",
			pkgJson:    `{"name":"@scope/multi","bin":{"bb":"./b.js","aa":"./a.js"}}`,
			want:       []string{"aa", "bb"},
		},
		{
			moduleName: "lodash",
			pkgJson:    `{"name":"lodash","dependencies":{}}`,
			want:       []string{},
		},
		{
			moduleName: "nobin",
			pkgJson:    `{"name":"nobin","bin":null}`,
			want:       []string{},
		},
	}

	cwd := t.TempDir()
	nodeModules := map[string]bool{}

	for _, c := range cases {
		dir := filepath.Join(cwd, "node_modules", filepath.FromSlash(c.moduleName))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(c.pkgJson), 0o644); err != nil {
			t.Fatal(err)
		}
		nodeModules[c.moduleName] = true
	}

	result := FindNodeModuleBinaries(nodeModules, cwd)

	for _, c := range cases {
		if got := result[c.moduleName]; !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.moduleName, got, c.want)
		}
	}
}
