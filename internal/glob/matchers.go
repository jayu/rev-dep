package globutil

import (
	"fmt"
	"path"
	"strings"

	"github.com/gobwas/glob"

	"rev-dep-go/internal/pathutil"
)

type GlobMatcher struct {
	globPattern glob.Glob
	inputString string
	patternRoot string
	// matchesAnySegment marks a gitignore pattern with no `/` in it. Such a pattern is
	// matched against every path segment rather than the path as a whole, so `*.ts`
	// matches `src/deep/a.ts` and `log*` matches the `logs` directory in `logs/e.log`.
	matchesAnySegment bool
	// dirOnly marks a pattern written with a trailing `/`. It can only match a directory,
	// i.e. a proper ancestor of the candidate path.
	dirOnly                 bool
	isAnchoredToPatternRoot bool
	isAdditional            bool
	isNegated               bool
	// canMatchAncestor is false for patterns ending in `/**` (or a bare `**`), which already
	// match everything below any directory they match - so for a plain yes/no answer the
	// ancestor walk adds nothing. Precomputed: it is a fixed property of the pattern.
	canMatchAncestor bool
	// literalSegment is set for a no-slash pattern with no glob metacharacters at all - the
	// commonest .gitignore shape by far (`node_modules`, `dist`, `.next`). Such a pattern is
	// a plain string comparison against a path segment, so it never needs the glob engine.
	literalSegment string
	// literalSuffix is the trailing run of the pattern containing no glob metacharacters,
	// e.g. ".ts" for `**/*.ts`. A candidate that does not end with it cannot match, which
	// rejects most ancestor directories without entering the glob engine at all.
	literalSuffix string
	// ruleIndex is the position of the originating pattern in the input slice. gitignore
	// is last-match-wins, and one pattern can expand into several matchers, so ordering
	// decisions must use this rather than the matcher's own position.
	ruleIndex int
}

func rebaseRelativePattern(pattern string, patternRoot string) (string, string, bool, bool) {
	if pattern == "" || strings.HasPrefix(pattern, "/") {
		return patternRoot, pattern, false, false
	}
	if pattern != "." && pattern != ".." && !strings.HasPrefix(pattern, "./") && !strings.HasPrefix(pattern, "../") {
		return patternRoot, pattern, false, false
	}

	trailingSlash := strings.HasSuffix(pattern, "/")
	cleaned := path.Clean(pattern)
	parts := strings.Split(cleaned, "/")
	up := 0
	i := 0
	for i < len(parts) && parts[i] == ".." {
		up++
		i++
	}
	rest := strings.Join(parts[i:], "/")
	if rest == "" {
		rest = "."
	}
	if rest == "." && !trailingSlash {
		trailingSlash = true
	}

	root := strings.TrimSuffix(patternRoot, "/")
	for i := 0; i < up; i++ {
		root = path.Dir(root)
	}
	if root != "" {
		root += "/"
	}

	return root, rest, true, trailingSlash
}

func CreateGlobMatchers(patterns []string, patternsRoot string) []GlobMatcher {
	globMatchers := []GlobMatcher{}
	// normalize pattern root to internal form and ensure trailing '/'
	patternRootNorm := pathutil.NormalizePathForInternal(patternsRoot)
	if patternRootNorm != "" && !strings.HasSuffix(patternRootNorm, "/") {
		patternRootNorm = patternRootNorm + "/"
	}

	for ruleIndex, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)

		// Negation (gitignore-style): a leading `!` marks an exception that re-includes paths a
		// positive pattern would otherwise match. A leading `\!` is an escaped literal `!`.
		isNegated := false
		if strings.HasPrefix(pattern, "!") {
			isNegated = true
			pattern = pattern[1:]
		} else if strings.HasPrefix(pattern, "\\!") {
			pattern = pattern[1:] // drop the escaping backslash, keep the literal '!'
		}
		if isNegated && pattern == "" {
			continue // a lone "!" is not a usable pattern
		}

		pattern = pathutil.NormalizeGlobPattern(pattern)

		patternRootForPattern := patternRootNorm
		isAnchoredToPatternRoot := strings.HasPrefix(pattern, "/")
		if isAnchoredToPatternRoot {
			pattern = strings.TrimPrefix(pattern, "/")
		}

		if !isAnchoredToPatternRoot {
			if newRoot, newPattern, ok, trailingSlash := rebaseRelativePattern(pattern, patternRootNorm); ok {
				patternRootForPattern = newRoot
				pattern = newPattern
				isAnchoredToPatternRoot = true
				if trailingSlash {
					if pattern == "." {
						pattern = ""
					}
					if pattern == "" {
						pattern = "**"
					} else if !strings.HasSuffix(pattern, "/") {
						pattern += "/"
					}
				}
			}
		}

		// A trailing `/` restricts the pattern to directories. The subtree below a matched
		// directory is covered by the ancestor walk in matchesOne, so no `/**` suffix is
		// appended here.
		dirOnly := strings.HasSuffix(pattern, "/")
		pattern = strings.TrimSuffix(pattern, "/")

		// gitignore: "If there is no slash in the pattern, git treats it as a shell glob
		// pattern" matched against each path segment - so it applies at any depth, and
		// matching a directory segment ignores that whole subtree. This holds for wildcard
		// patterns too (`*.ts`, `log*`, `[ab].ts`), not just plain names. A pattern anchored
		// with a leading `/` is always matched against the path as a whole instead.
		matchesAnySegment := !isAnchoredToPatternRoot && !strings.Contains(pattern, "/")

		// gitignore: "A leading `**` followed by a slash means match in all directories. For
		// example, `**/foo` matches file or directory `foo` anywhere, the same as pattern
		// `foo`." So a leading `**/` with no further `/` is exactly a no-slash pattern. Saying
		// so explicitly is worth real time: matched as a segment the subject can never contain
		// a separator, so the glob compiles without one, and `**/*.ts` drops from ~63ns to
		// ~3ns per match compared with evaluating it as a full path.
		if !isAnchoredToPatternRoot && strings.HasPrefix(pattern, "**/") {
			if rest := pattern[3:]; rest != "" && !strings.Contains(rest, "/") {
				pattern = rest
				matchesAnySegment = true
			}
		}

		// normalize pattern separators (globs and gitignore entries use forward slashes)
		patternNorm := pathutil.NormalizeGlobPattern(pattern)

		// !!! gobwas/glob treats the slashes around `**` as literal, so a `**/` segment
		// always requires at least one directory to be present. .gitignore says the
		// opposite: "A slash followed by two consecutive asterisks then a slash matches
		// zero or more directories. For example, "a/**/b" matches "a/b", "a/x/b", ...".
		// So `src/**/*.ts` wrongly misses `src/index.ts`, and `**/*.log` wrongly misses
		// `file.log`. This is not aligned with .gitignore nor with the TS rev-dep
		// implementation.
		//
		// The natural workaround - rewriting `a/**/b` to `a/{,**/}b` - is unusable
		// because of a second gobwas bug: a `**` inside a `{...}` group silently loses
		// its cross-separator power when the group is followed by a wildcard, so
		// `{,**/}*.ts` stops matching at depth 2 (`a/b/f.ts`). Related upstream report:
		// https://github.com/gobwas/glob/issues/43. The library has had no release
		// since v0.2.3 (2018), so we work around it on our side instead.
		//
		// The workaround is to compile one additional pattern per combination of
		// present/absent `**/` segments and match against all of them - see
		// expandDoubleStarSegments.
		for i, expandedPattern := range expandDoubleStarSegments(patternNorm) {
			item := GlobMatcher{
				// Compile with '/' as the path separator so a single '*' stays within one path
				// segment, as gitignore requires; '**' still crosses '/'. Segment matchers are
				// compiled without one - their subject is a single segment that cannot contain
				// a separator, and the separator-aware matchers gobwas builds are much slower.
				globPattern:             compilePattern(expandedPattern, matchesAnySegment),
				inputString:             expandedPattern,
				patternRoot:             patternRootForPattern,
				isAnchoredToPatternRoot: isAnchoredToPatternRoot,
				matchesAnySegment:       matchesAnySegment,
				dirOnly:                 dirOnly,
				canMatchAncestor:        expandedPattern != "**" && !strings.HasSuffix(expandedPattern, "/**"),
				literalSegment:          literalSegment(matchesAnySegment, expandedPattern),
				literalSuffix:           literalSuffix(expandedPattern),
				isAdditional:            i > 0,
				isNegated:               isNegated,
				ruleIndex:               ruleIndex,
			}
			globMatchers = append(globMatchers, item)
		}
	}
	return globMatchers
}

// expandDoubleStarSegments returns the pattern itself plus one variant for every
// combination of `**/` segments removed, so that a `**/` can match zero directories
// as .gitignore requires. `src/**/*.ts` yields [src/**/*.ts, src/*.ts]; the caller
// matches a path against all of them and treats any hit as a match.
//
// Only a whole `**` segment is expanded. Per .gitignore, "other consecutive asterisks
// are considered regular asterisks", so `a**/b` is left untouched. Literal segments are
// never dropped either, which is why `a/**/b/**/c.ts` still does not match `a/c.ts`.
//
// Growth is 2^n in the number of `**/` segments, but variants are deduplicated and real
// patterns carry one or two, so this stays at 2-4 matchers in practice.
func expandDoubleStarSegments(pattern string) []string {
	seen := map[string]bool{pattern: true}
	variants := []string{pattern}
	// queue-driven: every variant is expanded once, so this always terminates
	for i := 0; i < len(variants); i++ {
		variant := variants[i]
		for j := 0; j+3 <= len(variant); j++ {
			if variant[j:j+3] != "**/" {
				continue
			}
			if j > 0 && variant[j-1] != '/' {
				continue // not a whole segment, e.g. the `a**/` in `a**/b`
			}
			withoutSegment := variant[:j] + variant[j+3:]
			if !seen[withoutSegment] {
				seen[withoutSegment] = true
				variants = append(variants, withoutSegment)
			}
		}
	}
	return variants
}

type pathCandidate struct {
	path  string
	isDir bool
}

// ancestorsAndSelf yields every directory prefix of an internal-form path, shallowest
// first, followed by the path itself. gitignore tests a pattern against each of these:
// matching an ancestor directory ignores everything below it.
func ancestorsAndSelf(fileInternal string) []pathCandidate {
	out := make([]pathCandidate, 0, strings.Count(fileInternal, "/")+1)
	for i := 0; i < len(fileInternal); i++ {
		if fileInternal[i] == '/' && i > 0 {
			out = append(out, pathCandidate{path: fileInternal[:i], isDir: true})
		}
	}
	return append(out, pathCandidate{path: fileInternal, isDir: false})
}

// relativize re-expresses an absolute candidate against the matcher's pattern root, and
// reports false when the candidate lies outside that root. A pattern is interpreted
// relative to its root - the workspace, or the directory an explicit "../" climbed to -
// so anything above it is out of scope. An empty root means "no scoping", which is what
// callers matching non-path strings (e.g. node module specifiers) pass.
func (matcher GlobMatcher) relativize(candidate string) (string, bool) {
	if matcher.patternRoot == "" {
		return candidate, true
	}
	if strings.HasPrefix(candidate, matcher.patternRoot) {
		return strings.TrimPrefix(candidate, matcher.patternRoot), true
	}
	// The scoping guard applies only when BOTH the root and the candidate are absolute
	// paths. Callers also match values that are not absolute paths against an absolute
	// root - relative file IDs like "src/index.ts", and node module specifiers like
	// "react-dom/client" - and those must not be scoped out.
	if pathutil.IsAbsoluteInternalPath(matcher.patternRoot) && pathutil.IsAbsoluteInternalPath(candidate) {
		return "", false
	}
	return candidate, true
}

// matchesCandidate reports whether the matcher matches one concrete path - either the file
// itself or one of its ancestor directories. isDir says which, so a `dirOnly` pattern
// (written with a trailing `/`) can refuse to match a plain file.
func (matcher GlobMatcher) matchesCandidate(rel string, isDir bool, debug bool) bool {
	return matcher.matchesCandidateSeg(rel, rel[strings.LastIndex(rel, "/")+1:], isDir, debug)
}

// matchesCandidateSeg is matchesCandidate with the candidate's last path segment already
// extracted. The segment is identical for every matcher, so hot callers compute it once
// per candidate instead of once per matcher.
func (matcher GlobMatcher) matchesCandidateSeg(rel string, segment string, isDir bool, debug bool) bool {
	if matcher.dirOnly && !isDir {
		return false
	}
	subject := rel
	if matcher.matchesAnySegment {
		// a pattern with no `/` is matched against the segment name, at any depth
		subject = segment
		if matcher.literalSegment != "" {
			// plain name: a string comparison, no glob engine involved
			return subject == matcher.literalSegment
		}
	}
	if matcher.literalSuffix != "" && !strings.HasSuffix(subject, matcher.literalSuffix) {
		return false // cheap reject before entering the glob engine
	}
	matched := matcher.globPattern.Match(subject)
	if debug {
		fmt.Println("Matcher", matcher.inputString, "root", matcher.patternRoot,
			"anySegment", matcher.matchesAnySegment, "dirOnly", matcher.dirOnly,
			"additional", matcher.isAdditional, "negated", matcher.isNegated)
		fmt.Println("  candidate", rel, "subject", subject, "isDir", isDir, "->", matched)
	}
	return matched
}

// matchesOne reports whether a single matcher matches the path, ignoring negation and
// ordering. Kept for callers that reason about one pattern at a time.
//
// The file itself is tested before its ancestor directories: ordering is irrelevant to a
// plain yes/no answer, and the file is far and away the likelier hit, so the ancestor walk
// is usually skipped entirely.
func (matcher GlobMatcher) matchesOne(fileInternal string, debug bool) bool {
	if rel, ok := matcher.relativize(fileInternal); ok {
		if matcher.matchesCandidate(rel, false, debug) {
			return true
		}
	}
	if !matcher.canMatchAncestor {
		return false
	}
	for i := 0; i < len(fileInternal); i++ {
		if fileInternal[i] != '/' || i == 0 {
			continue
		}
		rel, ok := matcher.relativize(fileInternal[:i])
		if !ok {
			continue
		}
		if matcher.matchesCandidate(rel, true, debug) {
			return true
		}
	}
	return false
}

// compilePattern builds the gobwas matcher for one pattern. A segment matcher is always
// applied to a single path segment, which by construction contains no '/', so it is compiled
// without a separator - that avoids gobwas's separator-aware node types, which are several
// times slower to evaluate.
func compilePattern(pattern string, matchesAnySegment bool) glob.Glob {
	if matchesAnySegment {
		return glob.MustCompile(pattern)
	}
	return glob.MustCompile(pattern, '/')
}

// literalSegment returns the pattern itself when it is a no-slash, wildcard-free name that
// can be compared to a path segment directly, and "" otherwise.
func literalSegment(matchesAnySegment bool, pattern string) string {
	if !matchesAnySegment || pattern == "" {
		return ""
	}
	if strings.ContainsAny(pattern, "*?[]{}\\") {
		return ""
	}
	return pattern
}

// literalSuffix returns the trailing part of a pattern that contains no glob
// metacharacters, so a candidate can be rejected with a plain suffix comparison before the
// glob engine is involved. `**/*.ts` yields ".ts"; `src/**` yields "" (no pruning possible).
func literalSuffix(pattern string) string {
	for i := len(pattern) - 1; i >= 0; i-- {
		switch pattern[i] {
		case '*', '?', '[', ']', '{', '}', '\\', '/':
			return pattern[i+1:]
		}
	}
	return pattern
}

// resolveCandidate applies last-match-wins to a single path and reports whether the winning
// pattern excluded it. matched is false when no pattern applies at all.
//
// Matchers are stored in pattern order, so the winner is simply the last one that matches:
// scanning backwards lets us stop at the first hit instead of evaluating every pattern.
func resolveCandidate(c pathCandidate, matchers []GlobMatcher, debug bool) (excluded bool, matched bool) {
	segment := c.path[strings.LastIndex(c.path, "/")+1:]
	cachedRoot, cachedRel, cachedOK, haveCached := "", "", false, false
	for i := len(matchers) - 1; i >= 0; i-- {
		matcher := &matchers[i]
		// NOTE: canMatchAncestor must NOT be applied here. It is valid only for a plain
		// yes/no answer, where a `src/**` that matches a directory also matches the file
		// below it. Here an ancestor match carries extra meaning - it blocks re-inclusion -
		// so `src/**` matching the directory `src/vendor` has to be seen.
		if !haveCached || matcher.patternRoot != cachedRoot {
			cachedRoot = matcher.patternRoot
			cachedRel, cachedOK = matcher.relativize(c.path)
			haveCached = true
		}
		if !cachedOK {
			continue
		}
		if matcher.matchesCandidateSeg(cachedRel, segment, c.isDir, debug) {
			return !matcher.isNegated, true
		}
	}
	return false, false
}

// MatchesAnyGlobMatcher reports whether the path is matched by the pattern set, following
// .gitignore resolution rules:
//
//   - patterns are last-match-wins, so ["!src/a.ts", "*.ts"] matches src/a.ts while the
//     reverse order does not;
//   - a path under an excluded *directory* can never be re-included, because git never
//     descends into it. ["src/**", "!src/vendor/**"] therefore still matches
//     src/vendor/lib.ts: `src/**` matches the directory `src/vendor`, and the exception
//     below it cannot undo that.
func MatchesAnyGlobMatcher(filePath string, matchers []GlobMatcher, debug bool) bool {
	fileInternal := pathutil.NormalizePathForInternal(filePath)

	// Fast path: with no exceptions in the set there is no ordering to resolve and no
	// re-inclusion to block, so the first matcher to match settles it. This is the common
	// shape for graphExclude/entryPoints style configs.
	hasNegated := false
	for i := range matchers {
		if matchers[i].isNegated {
			hasNegated = true
			break
		}
	}
	if !hasNegated {
		return anyMatches(fileInternal, matchers, debug)
	}

	// A negation can only ever re-include a path it actually matches. If none of them
	// matches this path (or a directory above it), the exceptions cannot change the answer,
	// so the ordering machinery is skipped entirely and the plain any-match path is used.
	// Negations are a small minority of a pattern set and match a small minority of files,
	// so this keeps the ordered path proportional to the files it can actually affect.
	if !anyNegatedMatches(fileInternal, matchers, debug) {
		return anyMatches(fileInternal, matchers, debug)
	}

	// The answer is "excluded" if the file itself resolves to excluded, OR if any ancestor
	// directory does - and those are independent, so the order they are tested in does not
	// affect the result. Resolving the file first decides the common case in one pass and
	// leaves the ancestor walk as a fallback.
	if excluded, matched := resolveCandidate(pathCandidate{path: fileInternal}, matchers, debug); matched && excluded {
		return true
	}
	for i := 0; i < len(fileInternal); i++ {
		if fileInternal[i] != '/' || i == 0 {
			continue
		}
		ancestor := pathCandidate{path: fileInternal[:i], isDir: true}
		if excluded, matched := resolveCandidate(ancestor, matchers, debug); matched && excluded {
			return true
		}
	}
	return false
}

// anyMatches reports whether any matcher matches the path or one of its ancestor
// directories. Used for pattern sets without exceptions, where there is no ordering to
// resolve and the first hit settles the answer.
func anyMatches(fileInternal string, matchers []GlobMatcher, debug bool) bool {
	// relativize depends only on the pattern root, and in practice every matcher in a set
	// shares one, so the result is cached across matchers rather than recomputed per matcher.
	// Kept as plain locals - a closure here showed up as ~13% of samples in profiling.
	cachedRoot, cachedRel := "", ""
	cachedOK, haveCached := false, false

	// Test the file itself against every matcher first: it is much the likeliest hit, and
	// the ancestor walk below is then skipped entirely for anything that matches.
	segment := fileInternal[strings.LastIndex(fileInternal, "/")+1:]
	anyAncestorMatcher := false
	for i := range matchers {
		matcher := &matchers[i]
		anyAncestorMatcher = anyAncestorMatcher || matcher.canMatchAncestor
		if ls := matcher.literalSegment; ls != "" {
			// length and first byte reject nearly every mismatch before the full compare;
			// the scoping guard is only consulted once a name actually matches
			if len(ls) == len(segment) && ls[0] == segment[0] && ls == segment {
				if _, inScope := matcher.relativize(fileInternal); inScope {
					return true
				}
			}
			continue
		}
		if !haveCached || matcher.patternRoot != cachedRoot {
			cachedRoot = matcher.patternRoot
			cachedRel, cachedOK = matcher.relativize(fileInternal)
			haveCached = true
		}
		if !cachedOK {
			continue
		}
		if matcher.matchesCandidateSeg(cachedRel, segment, false, debug) {
			return true
		}
	}
	if !anyAncestorMatcher {
		return false
	}

	// Scan for segment boundaries once here rather than once per matcher, so a path of
	// depth d costs one pass instead of one per pattern.
	segStart := 0
	for j := 1; j < len(fileInternal); j++ {
		if fileInternal[j] != '/' {
			continue
		}
		ancestor := fileInternal[:j]
		segment := ancestor[segStart:]
		segStart = j + 1
		haveCached = false // candidate changed, the cached relativize no longer applies
		for i := range matchers {
			matcher := &matchers[i]
			if !matcher.canMatchAncestor {
				continue
			}
			// the candidate is a directory, so a dirOnly pattern is not disqualified here.
			// The segment comparison runs first and the scoping guard only on a hit: a bare
			// name like `api` rooted at /repo/apps/web must not match
			// /repo/apps/mobile/api/f.ts, but names collide rarely, so paying for the prefix
			// test only when they do keeps the common reject at two byte comparisons.
			if ls := matcher.literalSegment; ls != "" {
				if len(ls) == len(segment) && ls[0] == segment[0] && ls == segment {
					if _, inScope := matcher.relativize(ancestor); inScope {
						return true
					}
				}
				continue
			}
			if !haveCached || matcher.patternRoot != cachedRoot {
				cachedRoot = matcher.patternRoot
				cachedRel, cachedOK = matcher.relativize(ancestor)
				haveCached = true
			}
			if !cachedOK {
				continue
			}
			if matcher.matchesCandidateSeg(cachedRel, segment, true, debug) {
				return true
			}
		}
	}
	return false
}

// anyNegatedMatches reports whether any negated matcher matches the path or a directory
// above it. Ancestors are included because a negation naming a directory can re-include it.
func anyNegatedMatches(fileInternal string, matchers []GlobMatcher, debug bool) bool {
	segment := fileInternal[strings.LastIndex(fileInternal, "/")+1:]
	for i := range matchers {
		matcher := &matchers[i]
		if !matcher.isNegated {
			continue
		}
		if rel, ok := matcher.relativize(fileInternal); ok && matcher.matchesCandidateSeg(rel, segment, false, debug) {
			return true
		}
		if !matcher.canMatchAncestor {
			continue
		}
		segStart := 0
		for j := 1; j < len(fileInternal); j++ {
			if fileInternal[j] != '/' {
				continue
			}
			ancestor := fileInternal[:j]
			seg := ancestor[segStart:]
			segStart = j + 1
			if rel, ok := matcher.relativize(ancestor); ok && matcher.matchesCandidateSeg(rel, seg, true, debug) {
				return true
			}
		}
	}
	return false
}
