package monorepo

import "strings"

type NodeType string

const (
	LeafNode NodeType = "leaf"
	MapNode  NodeType = "map"
)

type ImportTargetTreeNode struct {
	NodeType      NodeType                         // "leaf" | "map"
	Value         string                           // target value or empty string
	ConditionsMap map[string]*ImportTargetTreeNode // conditional targets
}

type WildcardPattern struct {
	Key    string
	Prefix string
	Suffix string
}

type PackageJsonExports struct {
	Exports          map[string]interface{}
	WildcardPatterns []WildcardPattern
	ParsedTargets    map[string]*ImportTargetTreeNode
	HasDotPrefix     bool
}

// ParseImportTarget parses package.json import/export target into a tree structure.
// Returns nil for invalid targets.
func ParseImportTarget(target interface{}, conditionNames []string) *ImportTargetTreeNode {
	if targetStr, ok := target.(string); ok {
		// Check if string contains more than one wildcard
		if strings.Count(targetStr, "*") > 1 {
			return nil // Invalid target - too many wildcards
		}
		// Simple string target - leaf node
		return &ImportTargetTreeNode{
			NodeType:      LeafNode,
			Value:         targetStr,
			ConditionsMap: nil,
		}
	}

	if targetMap, ok := target.(map[string]interface{}); ok {
		// Check if this is a conditional map or nested conditions
		hasConditions := false
		conditionsMap := make(map[string]*ImportTargetTreeNode)

		// First, check for condition names (conditional exports)
		for _, condition := range conditionNames {
			if val, ok := targetMap[condition]; ok {
				hasConditions = true
				parsedChild := ParseImportTarget(val, conditionNames)
				if parsedChild != nil {
					conditionsMap[condition] = parsedChild
				}
			}
		}

		// Check for default condition
		if val, ok := targetMap["default"]; ok {
			hasConditions = true
			parsedChild := ParseImportTarget(val, conditionNames)
			if parsedChild != nil {
				conditionsMap["default"] = parsedChild
			}
		}

		if hasConditions {
			// This is a conditional map node
			return &ImportTargetTreeNode{
				NodeType:      MapNode,
				Value:         "",
				ConditionsMap: conditionsMap,
			}
		}

		// This might be a nested condition (like import/require within node)
		// Treat all keys as conditions
		for key, val := range targetMap {
			parsedChild := ParseImportTarget(val, conditionNames)
			if parsedChild != nil {
				conditionsMap[key] = parsedChild
			}
		}
		return &ImportTargetTreeNode{
			NodeType:      MapNode,
			Value:         "",
			ConditionsMap: conditionsMap,
		}
	}

	return nil // Invalid target
}
