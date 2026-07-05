package cli

import (
	"testing"

	"rev-dep-go/internal/config"
)

func TestDetectorPresets(t *testing.T) {
	// state returns "absent" when a detector field is unset, else "on"/"off" by its Enabled flag.
	state := func(present bool, enabled bool) string {
		if !present {
			return "absent"
		}
		if enabled {
			return "on"
		}
		return "off"
	}
	ruleState := func(r config.Rule) map[string]string {
		return map[string]string{
			"circular":          state(len(r.CircularImportsDetections) > 0, len(r.CircularImportsDetections) > 0 && r.CircularImportsDetections[0].Enabled),
			"unresolved":        state(len(r.UnresolvedImportsDetections) > 0, len(r.UnresolvedImportsDetections) > 0 && r.UnresolvedImportsDetections[0].Enabled),
			"orphan":            state(len(r.OrphanFilesDetections) > 0, len(r.OrphanFilesDetections) > 0 && r.OrphanFilesDetections[0].Enabled),
			"unusedNodeModules": state(len(r.UnusedNodeModulesDetections) > 0, len(r.UnusedNodeModulesDetections) > 0 && r.UnusedNodeModulesDetections[0].Enabled),
			"missingModules":    state(len(r.MissingNodeModulesDetections) > 0, len(r.MissingNodeModulesDetections) > 0 && r.MissingNodeModulesDetections[0].Enabled),
			"unusedExports":     state(len(r.UnusedExportsDetections) > 0, len(r.UnusedExportsDetections) > 0 && r.UnusedExportsDetections[0].Enabled),
			"devDeps":           state(len(r.DevDepsUsageOnProdDetections) > 0, len(r.DevDepsUsageOnProdDetections) > 0 && r.DevDepsUsageOnProdDetections[0].Enabled),
			"restrictedImports": state(len(r.RestrictedImportsDetections) > 0, len(r.RestrictedImportsDetections) > 0 && r.RestrictedImportsDetections[0].Enabled),
		}
	}

	cases := []struct {
		preset detectorPreset
		want   map[string]string
	}{
		{detectorsNone, map[string]string{
			"circular": "absent", "unresolved": "absent", "orphan": "absent", "unusedNodeModules": "absent",
			"missingModules": "absent", "unusedExports": "absent", "devDeps": "absent", "restrictedImports": "absent",
		}},
		{detectorsUnresolvedOnly, map[string]string{
			"circular": "absent", "unresolved": "on", "orphan": "absent", "unusedNodeModules": "absent",
			"missingModules": "absent", "unusedExports": "absent", "devDeps": "absent", "restrictedImports": "absent",
		}},
		{detectorsUnresolvedCircular, map[string]string{
			"circular": "on", "unresolved": "on", "orphan": "absent", "unusedNodeModules": "absent",
			"missingModules": "absent", "unusedExports": "absent", "devDeps": "absent", "restrictedImports": "absent",
		}},
		{detectorsScaffold, map[string]string{
			"circular": "on", "unresolved": "on", "orphan": "off", "unusedNodeModules": "off",
			"missingModules": "off", "unusedExports": "off", "devDeps": "off", "restrictedImports": "absent",
		}},
		{detectorsAll, map[string]string{
			"circular": "on", "unresolved": "on", "orphan": "on", "unusedNodeModules": "on",
			"missingModules": "on", "unusedExports": "on", "devDeps": "on", "restrictedImports": "absent",
		}},
	}

	for _, tc := range cases {
		rule := makePackageRule("pkg", tc.preset)
		got := ruleState(rule)
		for name, want := range tc.want {
			if got[name] != want {
				t.Errorf("preset %d: detector %q = %q, want %q", tc.preset, name, got[name], want)
			}
		}
	}
}
