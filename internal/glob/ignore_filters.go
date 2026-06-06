package globutil

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"

	"rev-dep-go/internal/pathutil"
)

type fileValueIgnorePairMatcher struct {
	fileMatcher   GlobMatcher
	valueMatchers []glob.Glob
}

// FileValueIgnoreMap maps a file glob to one or more value globs.
// JSON input supports either:
// - "file/glob": "value-glob"
// - "file/glob": ["value-glob-a", "value-glob-b"]
type FileValueIgnoreMap map[string][]string

func (m *FileValueIgnoreMap) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*m = nil
		return nil
	}

	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	parsed := make(FileValueIgnoreMap, len(raw))
	for filePath, rawValue := range raw {
		var single string
		if err := json.Unmarshal(rawValue, &single); err == nil {
			parsed[filePath] = []string{single}
			continue
		}

		var many []string
		if err := json.Unmarshal(rawValue, &many); err == nil {
			parsed[filePath] = many
			continue
		}

		return fmt.Errorf("ignore[%q] must be a string or array of strings", filePath)
	}

	*m = parsed
	return nil
}

type FileValueIgnoreMatcher struct {
	ignoreFilesMatcher []GlobMatcher
	ignoreValueMatcher []glob.Glob
	ignorePairMatcher  []fileValueIgnorePairMatcher
}

func normalizeIgnoreFilePath(path string) string {
	cleaned := filepath.Clean(pathutil.DenormalizePathForOS(strings.TrimSpace(path)))
	normalized := pathutil.NormalizePathForInternal(cleaned)
	normalized = strings.TrimPrefix(normalized, "./")
	return normalized
}

func NormalizeIgnoreFilePath(path string) string {
	return normalizeIgnoreFilePath(path)
}

func NewFileValueIgnoreMatcher(ignore FileValueIgnoreMap, ignoreFiles []string, ignoreValues []string, cwd string) *FileValueIgnoreMatcher {
	matcher := &FileValueIgnoreMatcher{
		ignoreFilesMatcher: CreateGlobMatchers(ignoreFiles, cwd),
		ignoreValueMatcher: []glob.Glob{},
		ignorePairMatcher:  []fileValueIgnorePairMatcher{},
	}

	for _, valuePattern := range ignoreValues {
		compiledValuePattern, err := glob.Compile(strings.TrimSpace(valuePattern))
		if err != nil {
			continue
		}
		matcher.ignoreValueMatcher = append(matcher.ignoreValueMatcher, compiledValuePattern)
	}

	for filePattern, valuePatterns := range ignore {
		normalizedFilePattern := normalizeIgnoreFilePath(filePattern)
		fileMatchers := CreateGlobMatchers([]string{normalizedFilePattern}, cwd)
		compiledValuePatterns := make([]glob.Glob, 0, len(valuePatterns))

		for _, valuePattern := range valuePatterns {
			compiledValuePattern, err := glob.Compile(strings.TrimSpace(valuePattern))
			if err != nil {
				continue
			}
			compiledValuePatterns = append(compiledValuePatterns, compiledValuePattern)
		}

		if len(compiledValuePatterns) == 0 {
			continue
		}

		for _, fileMatcher := range fileMatchers {
			matcher.ignorePairMatcher = append(matcher.ignorePairMatcher, fileValueIgnorePairMatcher{
				fileMatcher:   fileMatcher,
				valueMatchers: compiledValuePatterns,
			})
		}
	}

	return matcher
}

func (m *FileValueIgnoreMatcher) ShouldIgnore(filePath string, value string) bool {
	for _, valueMatcher := range m.ignoreValueMatcher {
		if valueMatcher.Match(value) {
			return true
		}
	}

	if MatchesAnyGlobMatcher(filePath, m.ignoreFilesMatcher, false) {
		return true
	}

	for _, pairMatcher := range m.ignorePairMatcher {
		// Match via MatchesAnyGlobMatcher (patternRoot-aware) so the file-key glob
		// behaves exactly like ignoreFiles - including relative patterns such as
		// "../../apps/mobile/**" that point at a sibling workspace.
		if !MatchesAnyGlobMatcher(filePath, []GlobMatcher{pairMatcher.fileMatcher}, false) {
			continue
		}
		for _, valueMatcher := range pairMatcher.valueMatchers {
			if valueMatcher.Match(value) {
				return true
			}
		}
	}

	return false
}
