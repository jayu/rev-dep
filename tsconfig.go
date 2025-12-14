package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/jsonc"
)

// ParseTsConfig reads tsconfig from disk (JSON or JSONC) at tsconfigPath and
// resolves any extended configs via the "extends" field. It returns a
// merged JSON bytes containing at least compilerOptions.paths,
// compilerOptions.baseUrl and compilerOptions.types. Merging rules:
// - child overrides base for baseUrl
// - paths are merged with child keys overriding base keys
// - types arrays are combined with child entries first and de-duplicated
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

	// resolve extends recursively
	merged, err := resolveExtends(raw, baseDir, map[string]bool{})
	if err != nil {
		return nil, err
	}

	out, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func resolveExtends(cfg map[string]interface{}, baseDir string, seen map[string]bool) (map[string]interface{}, error) {
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

	extStr, ok := ext.(string)
	if !ok || strings.TrimSpace(extStr) == "" {
		ensureCompilerOptions(result)
		return result, nil
	}

	// resolve path of extends
	candidates := []string{}

	if filepath.IsAbs(extStr) || strings.HasPrefix(extStr, ".") || strings.Contains(extStr, string(filepath.Separator)) {
		// treat as file path relative to baseDir when not absolute
		p := extStr
		if !filepath.IsAbs(p) {
			p = filepath.Join(baseDir, p)
		}
		candidates = append(candidates, p)
		// try with .json suffix
		candidates = append(candidates, p+".json")
	} else {
		// try package-style resolution inside node_modules for tsconfigs published as packages
		// try <baseDir>/node_modules/<extStr>
		candidates = append(candidates, filepath.Join(baseDir, "node_modules", extStr))
		candidates = append(candidates, filepath.Join(baseDir, "node_modules", extStr, "tsconfig.json"))
		candidates = append(candidates, filepath.Join(baseDir, "node_modules", extStr+".json"))
	}

	var baseCfg map[string]interface{}
	var foundPath string
	for _, cand := range candidates {
		// try exact file
		fi, err := os.Stat(cand)
		if err == nil && !fi.IsDir() {
			// read file
			bb, err := os.ReadFile(cand)
			if err != nil {
				continue
			}
			bb = jsonc.ToJSON(bb)
			var parsed map[string]interface{}
			if err := json.Unmarshal(bb, &parsed); err != nil {
				continue
			}
			foundPath = cand
			baseCfg = parsed
			break
		}
	}

	if baseCfg == nil {
		// Not found — nothing to merge
		ensureCompilerOptions(result)
		return result, nil
	}

	// avoid cycles
	absFound, _ := filepath.Abs(foundPath)
	if seen[absFound] {
		ensureCompilerOptions(result)
		return result, nil
	}
	seen[absFound] = true

	// resolve base's extends first
	baseDirNext := filepath.Dir(foundPath)
	resolvedBase, err := resolveExtends(baseCfg, baseDirNext, seen)
	if err != nil {
		return nil, err
	}

	// rebase any relative paths in the resolved base so they are correct
	// relative to the current config's baseDir. Extended configs can contain
	// paths that are relative to their own location; when merged into the
	// child config we must adjust them to point correctly from the child's
	// directory.
	rebasePaths(resolvedBase, baseDirNext, baseDir)

	// merge resolvedBase into result: child (result) overrides base
	merged := map[string]interface{}{}
	// copy base first
	for k, v := range resolvedBase {
		merged[k] = v
	}
	// then overlay child
	for k, v := range result {
		if k == "compilerOptions" {
			// special merge for compilerOptions
			baseCO := map[string]interface{}{}
			if bo, ok := merged["compilerOptions"].(map[string]interface{}); ok {
				for kk, vv := range bo {
					baseCO[kk] = vv
				}
			}
			childCO := map[string]interface{}{}
			if co, ok := v.(map[string]interface{}); ok {
				for kk, vv := range co {
					childCO[kk] = vv
				}
			}
			mergedCO := mergeCompilerOptions(baseCO, childCO)
			merged["compilerOptions"] = mergedCO
		} else {
			merged[k] = v
		}
	}

	// remove extends from final result
	delete(merged, "extends")
	ensureCompilerOptions(merged)
	return merged, nil
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

	// handle paths specially (merge keys, child overrides)
	if basePaths, ok := base["paths"].(map[string]interface{}); ok {
		// copy base paths
		newPaths := map[string]interface{}{}
		for k, v := range basePaths {
			newPaths[k] = v
		}
		out["paths"] = newPaths
	}

	// copy child keys, overriding
	for k, v := range child {
		if k == "paths" {
			childPaths, ok := v.(map[string]interface{})
			if !ok {
				out["paths"] = v
				continue
			}
			basePaths := map[string]interface{}{}
			if bp, ok := out["paths"].(map[string]interface{}); ok {
				for kk, vv := range bp {
					basePaths[kk] = vv
				}
			}
			for kk, vv := range childPaths {
				basePaths[kk] = vv
			}
			out["paths"] = basePaths
			continue
		}

		if k == "types" {
			// combine arrays, child first, dedup
			combined := []interface{}{}
			seen := map[string]bool{}
			if chArr, ok := v.([]interface{}); ok {
				for _, e := range chArr {
					if s, ok := e.(string); ok {
						if !seen[s] {
							combined = append(combined, s)
							seen[s] = true
						}
					}
				}
			}
			if bArr, ok := base["types"].([]interface{}); ok {
				for _, e := range bArr {
					if s, ok := e.(string); ok {
						if !seen[s] {
							combined = append(combined, s)
							seen[s] = true
						}
					}
				}
			}
			out["types"] = combined
			continue
		}

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
