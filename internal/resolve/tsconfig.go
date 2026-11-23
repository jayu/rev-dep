package resolve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/jsonc"

	"rev-dep-go/internal/diag"
)

// ParseTsConfig reads tsconfig from disk (JSON or JSONC) at tsconfigPath and
// resolves any extended configs via a string or array in the "extends" field.
// Later parents override earlier parents, and the child overrides all parents.
// Compiler options are merged by name, with explicitly supplied values replacing
// inherited values wholesale. This includes empty paths objects and types arrays.
// It returns the merged configuration as JSON bytes.
func ParseTsConfig(tsconfigPath string) ([]byte, error) {
	// read file
	content, err := os.ReadFile(tsconfigPath)
	if err != nil {
		return nil, err
	}

	// normalize to JSON
	content = jsonc.ToJSON(content)

	var raw map[string]interface{}
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tsconfig: %w", err)
	}

	baseDir := filepath.Dir(tsconfigPath)
	absPath, err := filepath.Abs(tsconfigPath)
	if err != nil {
		return nil, err
	}

	// resolve extends recursively
	merged, err := resolveExtends(raw, baseDir, map[string]bool{absPath: true})
	if err != nil {
		return nil, err
	}

	out, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func resolveExtends(cfg map[string]interface{}, baseDir string, active map[string]bool) (map[string]interface{}, error) {
	// start with a copy of cfg
	result := map[string]interface{}{}
	for k, v := range cfg {
		result[k] = v
	}

	ext, hasExt := result["extends"]
	if !hasExt {
		// nothing to extend; ensure compilerOptions exists
		ensureCompilerOptions(result)
		return result, nil
	}

	var parents []interface{}
	switch value := ext.(type) {
	case string:
		parents = []interface{}{value}
	case []interface{}:
		parents = value
	default:
		diag.Warnf("invalid tsconfig extends in %q: expected a nonempty string or an array of nonempty strings", baseDir)
		ensureCompilerOptions(result)
		return result, nil
	}

	merged := map[string]interface{}{}
	for i, parent := range parents {
		extStr, ok := parent.(string)
		if !ok || strings.TrimSpace(extStr) == "" {
			diag.Warnf("invalid tsconfig extends entry %d in %q: expected a nonempty string", i, baseDir)
			continue
		}

		resolvedBase, err := resolveExtendedTsConfig(extStr, baseDir, active)
		if err != nil {
			return nil, err
		}
		merged = mergeTsConfigs(merged, resolvedBase)
	}

	merged = mergeTsConfigs(merged, result)
	delete(merged, "extends")
	ensureCompilerOptions(merged)
	return merged, nil
}

// resolveExtendedTsConfig loads one parent and rebases its inherited paths to
// baseDir. active contains only configs on the current recursion chain, so
// sibling parents may extend the same base without being treated as a cycle.
func resolveExtendedTsConfig(extStr, baseDir string, active map[string]bool) (map[string]interface{}, error) {
	// resolve path of extends
	candidates := []string{}

	// Per TypeScript, an `extends` value is a file path only when it is absolute
	// or starts with "./" or "../". Everything else - including names that
	// contain "/", like "@docusaurus/tsconfig" or "pkg/tsconfig.json" - is a
	// bare specifier resolved Node-style from node_modules. (Don't key off the
	// presence of a slash: a scoped package name contains one.)
	if filepath.IsAbs(extStr) || strings.HasPrefix(extStr, ".") {
		// treat as file path relative to baseDir when not absolute
		p := extStr
		if !filepath.IsAbs(p) {
			p = filepath.Join(baseDir, p)
		}
		candidates = append(candidates, p)
		// try with .json suffix
		candidates = append(candidates, p+".json")
	} else {
		// Node-style resolution for a tsconfig published as a package.
		candidates = append(candidates, tsConfigPackageExtendsCandidates(baseDir, extStr)...)
	}

	var baseCfg map[string]interface{}
	var foundPath string
	for _, cand := range candidates {
		// try exact file
		fi, err := os.Stat(cand)
		if err != nil && !os.IsNotExist(err) {
			diag.Warnf("could not inspect extended tsconfig %q: %v", cand, err)
		}
		if err == nil && !fi.IsDir() {
			// read file
			bb, err := os.ReadFile(cand)
			if err != nil {
				diag.Warnf("could not read extended tsconfig %q: %v", cand, err)
				continue
			}
			bb = jsonc.ToJSON(bb)
			var parsed map[string]interface{}
			if err := json.Unmarshal(bb, &parsed); err != nil {
				diag.Warnf("could not parse extended tsconfig %q: %v", cand, err)
				continue
			}
			if parsed == nil {
				diag.Warnf("could not parse extended tsconfig %q: expected an object", cand)
				continue
			}
			foundPath = cand
			baseCfg = parsed
			break
		}
	}

	if baseCfg == nil {
		diag.Warnf("could not resolve tsconfig extends %q from %q; continuing without its settings", extStr, baseDir)
		return nil, nil
	}

	// avoid cycles
	absFound, err := filepath.Abs(foundPath)
	if err != nil {
		return nil, err
	}
	if active[absFound] {
		diag.Warnf("circular tsconfig extends at %q; continuing without its settings", foundPath)
		return nil, nil
	}
	active[absFound] = true
	defer delete(active, absFound)

	// resolve base's extends first
	baseDirNext := filepath.Dir(foundPath)
	resolvedBase, err := resolveExtends(baseCfg, baseDirNext, active)
	if err != nil {
		return nil, err
	}

	// rebase any relative paths in the resolved base so they are correct
	// relative to the current config's baseDir. Extended configs can contain
	// paths that are relative to their own location; when merged into the
	// child config we must adjust them to point correctly from the child's
	// directory.
	rebasePaths(resolvedBase, baseDirNext, baseDir)
	return resolvedBase, nil
}

func mergeTsConfigs(base, child map[string]interface{}) map[string]interface{} {
	merged := map[string]interface{}{}
	// copy base first
	for k, v := range base {
		merged[k] = v
	}
	// then overlay child
	for k, v := range child {
		if k == "compilerOptions" {
			baseCO, _ := merged["compilerOptions"].(map[string]interface{})
			childCO, _ := v.(map[string]interface{})
			merged["compilerOptions"] = mergeCompilerOptions(baseCO, childCO)
		} else {
			merged[k] = v
		}
	}

	return merged
}

// tsConfigPackageExtendsCandidates returns candidate file paths for a bare
// `extends` specifier (e.g. "@docusaurus/tsconfig" or "pkg/tsconfig.json"),
// resolved Node-style by walking node_modules directories up from baseDir. For a
// package directory it honors the package.json "main" entry and falls back to a
// root "tsconfig.json".
func tsConfigPackageExtendsCandidates(baseDir, extStr string) []string {
	candidates := []string{}
	dir := baseDir
	for {
		target := filepath.Join(dir, "node_modules", extStr)
		// Direct file (e.g. "pkg/base.json" or "@scope/pkg/tsconfig.json").
		candidates = append(candidates, target, target+".json")
		// Package directory: honor package.json "main", then default tsconfig.json.
		if info, err := os.Stat(target); err == nil && info.IsDir() {
			if main := readPackageJSONMain(target); main != "" {
				candidates = append(candidates, filepath.Join(target, main))
			}
			candidates = append(candidates, filepath.Join(target, "tsconfig.json"))
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return candidates
}

// readPackageJSONMain returns the "main" field of pkgDir/package.json, or "" if
// the file is missing or unparsable.
func readPackageJSONMain(pkgDir string) string {
	filename := filepath.Join(pkgDir, "package.json")
	content, err := os.ReadFile(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			diag.Warnf("could not read tsconfig package.json %q: %v", filename, err)
		}
		return ""
	}
	var pkg struct {
		Main string `json:"main"`
	}
	if err := json.Unmarshal(jsonc.ToJSON(content), &pkg); err != nil {
		diag.Warnf("could not parse tsconfig package.json %q: %v", filename, err)
		return ""
	}
	return pkg.Main
}

func ensureCompilerOptions(cfg map[string]interface{}) {
	if _, ok := cfg["compilerOptions"]; !ok {
		cfg["compilerOptions"] = map[string]interface{}{}
	}
}

func mergeCompilerOptions(base, child map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range base {
		out[k] = v
	}

	// TypeScript replaces each supplied option as a whole, including paths and
	// types. Missing keys inherit; explicit empty objects or arrays clear them.
	for k, v := range child {
		out[k] = v
	}

	return out
}

// rebasePaths rewrites any relative entries in cfg.compilerOptions.paths so
// that they point correctly from toDir instead of fromDir. fromDir is the
// directory where the base (extended) tsconfig was located; toDir is the
// directory of the child config that is merging the base into itself.
func rebasePaths(cfg map[string]interface{}, fromDir, toDir string) {
	co, ok := cfg["compilerOptions"].(map[string]interface{})
	if !ok {
		return
	}

	pathsRaw, ok := co["paths"].(map[string]interface{})
	if !ok {
		return
	}

	newPaths := map[string]interface{}{}
	for key, val := range pathsRaw {
		switch arr := val.(type) {
		case []interface{}:
			newArr := make([]interface{}, 0, len(arr))
			for _, e := range arr {
				str, ok := e.(string)
				if !ok {
					newArr = append(newArr, e)
					continue
				}

				// If absolute, keep as-is (normalized). Otherwise resolve from fromDir
				if filepath.IsAbs(str) {
					newArr = append(newArr, filepath.ToSlash(str))
					continue
				}

				abs := filepath.Clean(filepath.Join(fromDir, str))
				rel, err := filepath.Rel(toDir, abs)
				if err != nil {
					// fallback to absolute path if relative conversion fails
					newArr = append(newArr, filepath.ToSlash(abs))
				} else {
					// Use forward slashes for TS paths
					newArr = append(newArr, filepath.ToSlash(rel))
				}
			}
			newPaths[key] = newArr
		default:
			newPaths[key] = val
		}
	}

	co["paths"] = newPaths
	cfg["compilerOptions"] = co
}
