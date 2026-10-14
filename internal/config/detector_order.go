package config

import (
	"bytes"
	"encoding/json"
	"slices"
)

// The fallback is the existing default-output order, used for programmatically
// constructed rules and detectors no longer present in the config.
var detectorFields = []struct{ field, detector string }{
	{"circularImportsDetection", "circular-imports"},
	{"orphanFilesDetection", "orphan-files"},
	{"duplicatedCodeDetection", "duplicated-code"},
	{"moduleBoundaries", "module-boundaries"},
	{"unusedNodeModulesDetection", "unused-node-modules"},
	{"missingNodeModulesDetection", "missing-node-modules"},
	{"unusedExportsDetection", "unused-exports"},
	{"unresolvedImportsDetection", "unresolved-imports"},
	{"devDepsUsageOnProdDetection", "dev-deps-usage-on-prod"},
	{"restrictedImportsDetection", "restricted-imports"},
	{"restrictedImportersDetection", "restricted-importers"},
	{"restrictedDirectImportersDetection", "restricted-direct-importers"},
	{"importConventions", "import-conventions"},
}

func readDetectorOrder(data []byte) ([]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if start != json.Delim('{') {
		return nil, nil
	}
	var order []string
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		for _, entry := range detectorFields {
			if key == entry.field && !slices.Contains(order, entry.detector) {
				order = append(order, entry.detector)
				break
			}
		}
	}
	return order, nil
}

// OrderDetectors follows the configured property order, then appends detectors
// absent from the config in the existing default-output order.
func OrderDetectors(detectors, configured []string) []string {
	var ordered []string
	add := func(detector string) {
		if slices.Contains(detectors, detector) && !slices.Contains(ordered, detector) {
			ordered = append(ordered, detector)
		}
	}
	for _, detector := range configured {
		add(detector)
	}
	for _, entry := range detectorFields {
		add(entry.detector)
	}
	for _, detector := range detectors {
		add(detector)
	}
	return ordered
}
