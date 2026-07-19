package config

import (
	"fmt"
	"os"
)

// FixResult reports the outcome of `config lint --fix`.
type FixResult struct {
	RemovedCount          int // dead patterns actually removed
	ReportOnlyKept        int // dead patterns left in place (not auto-removed / could not navigate)
	TrailingCommasRemoved int // redundant trailing commas removed
	CompactedCount        int // detector declarations rewritten to compact form
}

func ruleWasRun(result *LintResult, rule LintRuleName) bool {
	for _, r := range result.RulesRun {
		if r == rule {
			return true
		}
	}
	return false
}

// ownerKey identifies the object that directly owns a set of option members (a rule, a
// detector, a module boundary, or the config root). Grouping removable dead patterns by
// owner lets us batch all whole-member deletions from one object into a single
// non-overlapping operation — computing them one-by-one produces overlapping byte
// ranges that ApplyEdits would silently drop.
type ownerKey struct {
	ruleIndex     int
	detectorType  string
	detectorIndex int
	boundaryIndex int
}

func ownerKeyOf(dp DeadPattern) ownerKey {
	return ownerKey{
		ruleIndex:     dp.RuleIndex,
		detectorType:  dp.DetectorType,
		detectorIndex: dp.DetectorIndex,
		boundaryIndex: dp.BoundaryIndex,
	}
}

// ApplyLintFix removes every removable dead pattern from the config file on disk,
// preserving comments and formatting. Report-only patterns, and any that cannot be
// navigated, are counted but left untouched. It returns the fix summary.
func ApplyLintFix(result *LintResult) (*FixResult, error) {
	fix := &FixResult{}
	if result == nil || result.ConfigFilePath == "" {
		return fix, fmt.Errorf("no config file to fix")
	}

	content, err := os.ReadFile(result.ConfigFilePath)
	if err != nil {
		return fix, err
	}
	doc, err := ParseJSONC(content)
	if err != nil {
		return fix, fmt.Errorf("failed to parse config for fixing: %w", err)
	}

	// Group removable dead patterns by owning object, then by option key within it.
	byOwner := make(map[ownerKey]map[string][]DeadPattern)
	sample := make(map[ownerKey]DeadPattern)
	var ownerOrder []ownerKey
	optionOrder := make(map[ownerKey][]string)

	for _, dp := range result.DeadPatterns {
		if !dp.Removable || dp.ElementIndex < 0 {
			fix.ReportOnlyKept++
			continue
		}
		ok := ownerKeyOf(dp)
		if _, seen := byOwner[ok]; !seen {
			byOwner[ok] = make(map[string][]DeadPattern)
			sample[ok] = dp
			ownerOrder = append(ownerOrder, ok)
		}
		if _, seen := byOwner[ok][dp.OptionKey]; !seen {
			optionOrder[ok] = append(optionOrder[ok], dp.OptionKey)
		}
		byOwner[ok][dp.OptionKey] = append(byOwner[ok][dp.OptionKey], dp)
	}

	var edits []Edit

	for _, ok := range ownerOrder {
		owner := locateOwner(doc, sample[ok])
		if owner == nil || owner.Kind != JSONObject {
			for _, deads := range byOwner[ok] {
				fix.ReportOnlyKept += len(deads)
			}
			continue
		}

		var wholeMemberKeys []string
		wholeMemberCount := 0
		for _, optKey := range optionOrder[ok] {
			deads := byOwner[ok][optKey]
			arr := owner.Get(optKey)
			if arr == nil || arr.Kind != JSONArray {
				fix.ReportOnlyKept += len(deads)
				continue
			}
			// Keep only in-range indices. A stale/out-of-range index must never inflate the
			// "every element is dead" decision below (which deletes the WHOLE member) — that
			// could drop live elements. Such indices are left in place and reported instead.
			deadIdx := make([]int, 0, len(deads))
			for _, dp := range deads {
				if dp.ElementIndex >= 0 && dp.ElementIndex < len(arr.Elems) {
					deadIdx = append(deadIdx, dp.ElementIndex)
				} else {
					fix.ReportOnlyKept++
				}
			}
			if len(deadIdx) == 0 {
				continue
			}
			// Count unique elements actually removed, not the number of findings (duplicate
			// findings can target the same element), so the summary is accurate.
			removed := uniqueCount(deadIdx)
			if removed == len(arr.Elems) {
				// Every element is dead — mark the whole member for batched removal.
				wholeMemberKeys = append(wholeMemberKeys, optKey)
				wholeMemberCount += removed
			} else {
				edits = append(edits, RemoveArrayElements(doc.Original, arr, deadIdx)...)
				fix.RemovedCount += removed
			}
		}

		// Remove all fully-dead members of this owner together, so their byte ranges are
		// computed as one non-overlapping sequence.
		if len(wholeMemberKeys) > 0 {
			memberIdx := memberIndicesFor(owner, wholeMemberKeys)
			if len(memberIdx) >= len(owner.Members) {
				// Removing every member would leave an owner with no content; deleting the
				// owner object itself is out of scope, so leave these in place.
				fix.ReportOnlyKept += wholeMemberCount
			} else {
				edits = append(edits, RemoveObjectMembers(doc.Original, owner, memberIdx)...)
				fix.RemovedCount += wholeMemberCount
			}
		}
	}

	// The lanes are applied as an ORDERED PIPELINE, not merged into one edit set: each
	// pass re-parses the previous pass's output. This matters because lanes interact —
	// e.g. dead-glob removal can empty a detector object ({ "enabled": true, "denyFiles":
	// ["dead"] } → { "enabled": true }) which the compact lane then folds to `true`.
	// Merging the edits would make them overlap on the same subtree and ApplyEdits would
	// silently drop one. Ordering: dead globs → compact → trailing commas.
	current := doc.Original

	// Pass 1: dead-pattern removals (their offsets index the original document).
	if len(edits) > 0 {
		current = ApplyEdits(doc.Original, edits)
	}

	// Pass 2: compact detector declarations (re-parse; only when the compact rule ran).
	if ruleWasRun(result, RuleCompact) {
		if cdoc, perr := ParseJSONC(current); perr == nil {
			compactE := compactEdits(cdoc)
			if len(compactE) > 0 {
				current = ApplyEdits(cdoc.Original, compactE)
				fix.CompactedCount = len(compactE)
			}
		}
	}

	// Pass 3: strip redundant trailing commas last (re-scan; only when its rule ran).
	if ruleWasRun(result, RuleTrailingCommas) {
		commaEdits := RemoveTrailingCommas(current)
		if len(commaEdits) > 0 {
			current = ApplyEdits(current, commaEdits)
			fix.TrailingCommasRemoved = len(commaEdits)
		}
	}

	if string(current) == string(doc.Original) {
		return fix, nil // nothing changed
	}

	// Safety net: every lane preserves a valid config, so a parse failure here is a bug —
	// never write output that would no longer parse.
	if _, perr := ParseConfig(current); perr != nil {
		return fix, fmt.Errorf("fix would produce an invalid config; leaving %s unchanged: %w", result.ConfigFilePath, perr)
	}

	if err := os.WriteFile(result.ConfigFilePath, current, 0644); err != nil {
		return fix, err
	}
	return fix, nil
}

// locateOwner navigates to the object that directly owns a dead pattern's option
// member (the config root, a rule, a module boundary, or a detector object).
func locateOwner(doc *JSONDocument, dp DeadPattern) *JSONNode {
	return locateOwnerNav(doc, dp.RuleIndex, dp.DetectorType, dp.DetectorIndex, dp.BoundaryIndex)
}

// locateOwnerNav resolves the object that owns an option member from raw navigation
// fields. Returns nil if the path cannot be resolved.
func locateOwnerNav(doc *JSONDocument, ruleIndex int, detectorType string, detectorIndex, boundaryIndex int) *JSONNode {
	root := doc.Root

	// Top-level option: owned by the root object.
	if ruleIndex < 0 {
		return root
	}

	ruleObj := root.Get("workspaces").Index(ruleIndex)
	if ruleObj == nil {
		return nil
	}

	// Rule-level option (entry points): owned by the rule object.
	if detectorType == "" {
		return ruleObj
	}

	// Module boundaries: rules[i].moduleBoundaries[b].<option>
	if detectorType == "moduleBoundaries" {
		return ruleObj.Get("moduleBoundaries").Index(boundaryIndex)
	}

	// Detector option: rules[i].<detector>(.[detIdx]).<option>
	return ruleObj.Get(detectorType).AsObjectOrElem(detectorIndex)
}

// memberIndicesFor returns the indices, in object member order, of the members whose
// key is in keys.
func memberIndicesFor(obj *JSONNode, keys []string) []int {
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}
	var idx []int
	for i, m := range obj.Members {
		if want[m.Name] {
			idx = append(idx, i)
		}
	}
	return idx
}

func uniqueCount(idx []int) int {
	seen := make(map[int]bool, len(idx))
	for _, i := range idx {
		seen[i] = true
	}
	return len(seen)
}
