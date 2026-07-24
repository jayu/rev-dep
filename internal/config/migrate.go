package config

import (
	"fmt"
	"strings"
)

// migrate.go upgrades a v2 config to the 2.0 (v3) schema. It edits the RAW bytes via jsonedit
// (comments/formatting preserved) because ParseConfig rejects a v1 config outright.

type MigrateResult struct {
	Migrated       []byte
	Changed        bool
	AppliedChanges []string        // safe auto-edits made
	PatternReviews []PatternReview // globs to re-check by hand
	ResultNotes    []string        // behavior shifts, no edit possible
	CINotes        []string        // removed CLI flags to drop from CI
}

type PatternReview struct {
	Workspace string   // the workspace's `path`
	Field     string   // grouping key within the workspace, e.g. devEntryPoints, restrictedImportsDetection[0]
	Item      string   // location within Field, e.g. [3], entryPoints[1], pattern
	Pattern   string   //
	Reasons   []string // short codes: "semantics", "sibling"
}

// Path-glob option keys per detector. Module/import/export patterns are excluded — they are
// never workspace-relative and are unaffected by the glob changes.
var pathGlobDetectorFields = map[string][]string{
	"orphanFilesDetection":               {"validEntryPoints", "graphExclude"},
	"unusedExportsDetection":             {"validEntryPoints", "graphExclude", "ignoreFiles"},
	"unresolvedImportsDetection":         {"ignoreFiles"},
	"devDepsUsageOnProdDetection":        {"prodEntryPoints"},
	"restrictedImportsDetection":         {"entryPoints", "graphExclude", "denyFiles", "ignoreMatches"},
	"restrictedImportersDetection":       {"files", "allowedEntryPoints", "graphExclude", "ignoreMatches"},
	"restrictedDirectImportersDetection": {"files", "allowImporters", "denyImporters", "ignoreMatches"},
}

// Detectors whose `ignore` object uses file globs as its keys.
var detectorsWithIgnoreMap = map[string]bool{"unusedExportsDetection": true, "unresolvedImportsDetection": true}

var workspaceArrayFields = []string{"prodEntryPoints", "devEntryPoints", "ignoreEntryPoints"}

func MigrateConfig(content []byte) (*MigrateResult, error) {
	doc, err := ParseJSONC(content)
	if err != nil {
		return nil, fmt.Errorf("config is not valid JSON/JSONC: %w", err)
	}
	root := doc.Root
	if root == nil || root.Kind != JSONObject {
		return nil, fmt.Errorf("config root must be a JSON object")
	}

	res := &MigrateResult{Migrated: content}
	workspaces := root.Get("workspaces")
	if workspaces == nil {
		workspaces = root.Get("rules")
	}

	var edits []Edit

	if m := root.GetMember("rules"); m != nil { // A1: rename rules -> workspaces
		edits = append(edits, Edit{Start: m.KeyStart, End: m.KeyStart + len(`"rules"`), Text: `"workspaces"`})
		res.AppliedChanges = append(res.AppliedChanges, "renamed top-level `rules` to `workspaces`")
	}
	if v := root.Get("configVersion"); v != nil && v.Kind == JSONString { // A2: bump version
		if cur, _ := doc.StringValue(v); cur != CurrentConfigVersion {
			edits = append(edits, ReplaceNode(v, `"`+CurrentConfigVersion+`"`))
			res.AppliedChanges = append(res.AppliedChanges, fmt.Sprintf("set configVersion to %q (was %q)", CurrentConfigVersion, cur))
		}
	} else {
		res.ResultNotes = append(res.ResultNotes, `configVersion is missing — add "configVersion": "2.0" manually`)
	}
	removed := 0 // A3: drop the discontinued `algorithm` option
	forEachWorkspace(workspaces, func(ws *JSONNode, _ int) {
		for _, d := range detectorNodes(ws.Get("circularImportsDetection")) {
			if e, ok := RemoveMember(res.Migrated, d.node, "algorithm"); ok {
				edits, removed = append(edits, e...), removed+1
			}
		}
	})
	if removed > 0 {
		res.AppliedChanges = append(res.AppliedChanges, fmt.Sprintf("removed the `algorithm` field from %d circularImportsDetection block(s)", removed))
	}

	if len(edits) > 0 {
		res.Migrated, res.Changed = ApplyEdits(content, edits), true
	}

	res.PatternReviews = collectPatternReviews(doc, workspaces)
	res.ResultNotes = append(res.ResultNotes, resultNotes(doc, workspaces)...)
	res.CINotes = []string{
		"`config run` and `config lint` no longer accept --condition-names, --follow-monorepo-packages, --package-json, or --tsconfig-json — remove them from CI/scripts.",
		"--package-json was removed from every command — remove it from CI/scripts.",
	}
	return res, nil
}

func collectPatternReviews(doc *JSONDocument, workspaces *JSONNode) []PatternReview {
	var out []PatternReview
	forEachWorkspace(workspaces, func(ws *JSONNode, wi int) {
		wsPath, _ := doc.StringValue(ws.Get("path"))
		if wsPath == "" {
			wsPath = fmt.Sprintf("workspaces[%d]", wi)
		}
		cross := workspaceEligibleForCrossLeak(doc, ws)

		add := func(field, item, pattern string) {
			var reasons []string
			if globMayHaveChangedUnderGitignore(pattern) {
				reasons = append(reasons, "matching")
			}
			if cross && globMayLeakAcrossWorkspaces(pattern) {
				reasons = append(reasons, "sibling")
			}
			if reasons != nil {
				out = append(out, PatternReview{wsPath, field, item, pattern, reasons})
			}
		}
		// addArr flags each string in arr. itemField is "" for a workspace-level array (items
		// are bare [i]) or the sub-field name for a nested array (items are itemField[i]).
		addArr := func(field, itemField string, arr *JSONNode) {
			for i, e := range arrayElems(arr) {
				s, ok := doc.StringValue(e)
				if !ok {
					continue
				}
				if itemField == "" {
					add(field, fmt.Sprintf("[%d]", i), s)
				} else {
					add(field, fmt.Sprintf("%s[%d]", itemField, i), s)
				}
			}
		}

		for _, f := range workspaceArrayFields {
			addArr(f, "", ws.Get(f))
		}

		for bi, b := range arrayElems(ws.Get("moduleBoundaries")) {
			field := fmt.Sprintf("moduleBoundaries[%d]", bi)
			if s, ok := doc.StringValue(b.Get("pattern")); ok {
				add(field, "pattern", s)
			}
			for _, f := range []string{"allow", "deny", "denyIgnore", "mutuallyExclusive"} {
				addArr(field, f, b.Get(f))
			}
		}

		for det, fields := range pathGlobDetectorFields {
			for _, d := range detectorNodes(ws.Get(det)) {
				field := det + d.loc
				for _, f := range fields {
					addArr(field, f, d.node.Get(f))
				}
				if ign := d.node.Get("ignore"); detectorsWithIgnoreMap[det] && ign != nil && ign.Kind == JSONObject {
					for _, m := range ign.Members {
						add(field, fmt.Sprintf("ignore[%q]", m.Name), m.Name)
					}
				}
			}
		}
	})
	return out
}

func resultNotes(doc *JSONDocument, workspaces *JSONNode) []string {
	var notes []string
	circular, unused := false, false
	forEachWorkspace(workspaces, func(ws *JSONNode, _ int) {
		circular = circular || detectorEnabled(doc, ws.Get("circularImportsDetection"))
		unused = unused || detectorEnabled(doc, ws.Get("unusedNodeModulesDetection"))
	})
	if circular {
		notes = append(notes, "Circular imports now use the SCC algorithm — expect the same or FEWER reported cycles. Lower any CI cycle-count baseline.")
	}
	if unused {
		notes = append(notes, "Unused-dependency binary names now match whole words only — the set of reported unused node modules may change.")
	}
	return notes
}

// globMayHaveChangedUnderGitignore mirrors the doc's "How to check": a single `*`/`?`, a
// `**/`, or a leading `!` may match differently under v3's gitignore-aligned rules.
func globMayHaveChangedUnderGitignore(p string) bool {
	if strings.HasPrefix(p, "!") || strings.Contains(p, "**/") || strings.Contains(p, "?") {
		return true
	}
	for i := 0; i < len(p); i++ {
		if p[i] == '*' && !(i > 0 && p[i-1] == '*') && !(i+1 < len(p) && p[i+1] == '*') {
			return true // a lone `*`
		}
	}
	return false
}

// globMayLeakAcrossWorkspaces flags an unanchored pattern (leading `**/`, leading wildcard
// segment, or bare name) that in v2 could reach a sibling workspace. Root-anchored (`/…`) and
// relative (`../…`, `./…`) patterns were always scoped.
func globMayLeakAcrossWorkspaces(p string) bool {
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "./") {
		return false
	}
	if strings.HasPrefix(p, "**/") {
		return true
	}
	slash := strings.IndexByte(p, '/')
	return slash == -1 || strings.ContainsAny(p[:slash], "*?")
}

func workspaceEligibleForCrossLeak(doc *JSONDocument, ws *JSONNode) bool {
	path, _ := doc.StringValue(ws.Get("path"))
	path = strings.TrimSuffix(strings.TrimPrefix(path, "./"), "/")
	if path == "" || path == "." {
		return false // root workspace legitimately spans the repo
	}
	follow := ws.Get("followMonorepoPackages")
	return !(follow != nil && follow.Kind == JSONPrimitive && doc.RawText(follow) == "false")
}

// ---- tree helpers ----

func arrayElems(n *JSONNode) []*JSONNode {
	if n == nil || n.Kind != JSONArray {
		return nil
	}
	return n.Elems
}

func forEachWorkspace(workspaces *JSONNode, fn func(ws *JSONNode, i int)) {
	for i, ws := range arrayElems(workspaces) {
		if ws.Kind == JSONObject {
			fn(ws, i)
		}
	}
}

// detNode is a concrete detector object plus its location suffix ("" for the single-object
// form, "[i]" for the array-of-objects form).
type detNode struct {
	node *JSONNode
	loc  string
}

func detectorNodes(n *JSONNode) []detNode {
	if n == nil {
		return nil
	}
	if n.Kind == JSONObject {
		return []detNode{{n, ""}}
	}
	var out []detNode
	for i, e := range arrayElems(n) {
		if e.Kind == JSONObject {
			out = append(out, detNode{e, fmt.Sprintf("[%d]", i)})
		}
	}
	return out
}

// detectorEnabled reports whether a detector value turns the check on: bare `true`, or an
// object/array-of-objects where `enabled` is absent or true.
func detectorEnabled(doc *JSONDocument, n *JSONNode) bool {
	if n == nil {
		return false
	}
	if n.Kind == JSONPrimitive {
		return doc.RawText(n) == "true"
	}
	for _, d := range detectorNodes(n) {
		if e := d.node.Get("enabled"); e == nil || doc.RawText(e) == "true" {
			return true
		}
	}
	return false
}
