package dupcode

import (
	"strings"
	"testing"
)

// The snapshot is the record of how its own baseline was produced, so it is the authority on
// how a run compared against it is configured. These cover the two halves of that: the
// settings survive a round trip, and a run that wants different ones is told rather than
// quietly compared against a baseline that means something else.

func settingsOpts() Options {
	return Options{
		Blinding:            Blinding{Identifiers: true, Numbers: true},
		MinTokens:           40,
		MinLines:            5,
		MinDepth:            2,
		MinStatements:       3,
		MinDuplicates:       3,
		SkipObjects:         true,
		IgnoreFiles:         []string{"**/*.test.ts"},
		ProcessIgnoredFiles: []string{"dist/**"},
	}
}

// TestSnapshotParametersSurviveApply is the round trip the whole scheme rests on: what was
// recorded has to come back as the same run.
func TestSnapshotParametersSurviveApply(t *testing.T) {
	original := settingsOpts()
	restored := SnapshotParametersFrom(original).Apply(Options{Cwd: "/elsewhere"})

	if diffs := SnapshotParametersFrom(original).DiffSettings(
		SnapshotParametersFrom(restored), nil); len(diffs) > 0 {
		t.Errorf("settings did not survive the round trip: %+v", diffs)
	}
	// Applying settings must not disturb what steers the run rather than the findings.
	if restored.Cwd != "/elsewhere" {
		t.Errorf("Apply overwrote Cwd: %q", restored.Cwd)
	}
}

// TestSnapshotParametersRecordEverySettingThatChangesTheResult is the guard that keeps this
// honest: a setting can be added to Options and honoured by the detector while never
// reaching the snapshot, and then the snapshot silently stops being the authority it claims
// to be for that one setting.
//
// Every setting the detector acts on is now listed below. There used to be one exception, a
// file-size ceiling that no flag and no config key could set, and its being a build constant
// rather than a recorded setting is what let the two entry points disagree about which files
// they analysed. It is gone.
func TestSnapshotParametersRecordEverySettingThatChangesTheResult(t *testing.T) {
	// Each of these makes the run find something different, so each must round trip.
	cases := []struct {
		name  string
		apply func(*Options)
	}{
		{"blindIdentifiers", func(o *Options) { o.Blinding.Identifiers = !o.Blinding.Identifiers }},
		{"blindStrings", func(o *Options) { o.Blinding.Strings = !o.Blinding.Strings }},
		{"blindNumbers", func(o *Options) { o.Blinding.Numbers = !o.Blinding.Numbers }},
		{"minTokens", func(o *Options) { o.MinTokens = 99 }},
		{"minLines", func(o *Options) { o.MinLines = 9 }},
		{"minDepth", func(o *Options) { o.MinDepth = 9 }},
		{"minStatements", func(o *Options) { o.MinStatements = 9 }},
		{"minDuplicates", func(o *Options) { o.MinDuplicates = 9 }},
		{"skipObjects", func(o *Options) { o.SkipObjects = !o.SkipObjects }},
		{"ignoreFiles", func(o *Options) { o.IgnoreFiles = []string{"other/**"} }},
		{"processIgnoredFiles", func(o *Options) { o.ProcessIgnoredFiles = []string{"other/**"} }},
	}
	for _, tc := range cases {
		base := settingsOpts()
		changed := settingsOpts()
		tc.apply(&changed)

		diffs := SnapshotParametersFrom(base).DiffSettings(SnapshotParametersFrom(changed), nil)
		if len(diffs) != 1 || diffs[0].Setting != tc.name {
			t.Errorf("%s is not recorded in the snapshot, so a run could change it without "+
				"the comparison noticing; got %+v", tc.name, diffs)
		}
	}
}

func snapshotWith(opts Options) *Snapshot {
	return &Snapshot{
		SchemaVersion:        SnapshotSchemaVersion,
		CanonicalFormVersion: CanonicalFormVersion,
		Parameters:           SnapshotParametersFrom(opts),
	}
}

// A command line with no flags on it has stated nothing, so it takes the snapshot's settings
// rather than its own defaults - otherwise every snapshot taken at anything but the defaults
// would be unusable without retyping the flags that produced it.
func TestFlagsOmittedTakeTheSnapshotSettings(t *testing.T) {
	snap := snapshotWith(settingsOpts())

	// A bare run: defaults everywhere, nothing asked for.
	got, err := ReconcileFlagsWithSnapshot(Options{Cwd: "/p"}, snap, "snap.json", map[string]bool{})
	if err != nil {
		t.Fatalf("a run with no flags should not conflict with anything: %v", err)
	}
	if diffs := snap.Parameters.DiffSettings(SnapshotParametersFrom(got), nil); len(diffs) > 0 {
		t.Errorf("the snapshot's settings were not applied: %+v", diffs)
	}
	if got.Cwd != "/p" {
		t.Errorf("Cwd was overwritten: %q", got.Cwd)
	}
}

// A flag someone typed is a belief about what is about to happen. Overriding it silently
// would leave them holding it.
func TestFlagThatContradictsTheSnapshotIsAnError(t *testing.T) {
	snap := snapshotWith(settingsOpts())

	wanted := Options{Cwd: "/p", MinTokens: 12}
	_, err := ReconcileFlagsWithSnapshot(wanted, snap, "snap.json", map[string]bool{"minTokens": true})
	if err == nil {
		t.Fatal("a flag that contradicts the snapshot must be reported")
	}
	var conflict *SettingsConflictError
	if !asSettingsConflict(err, &conflict) {
		t.Fatalf("wrong error type: %v", err)
	}
	if len(conflict.Differences) != 1 || conflict.Differences[0].Setting != "minTokens" {
		t.Errorf("the conflicting setting is not named: %+v", conflict.Differences)
	}
	for _, want := range []string{"minTokens", "snapshot has 40", "asks for 12", "--update-snapshot"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message is missing %q:\n%s", want, err)
		}
	}
}

// Only what was typed is checked. A run that says --min-tokens 40 against a snapshot taken
// at 40 is not in conflict just because it left every other flag alone.
func TestOnlyTheFlagsActuallyTypedAreChecked(t *testing.T) {
	snap := snapshotWith(settingsOpts())

	wanted := Options{Cwd: "/p", MinTokens: 40}
	got, err := ReconcileFlagsWithSnapshot(wanted, snap, "snap.json", map[string]bool{"minTokens": true})
	if err != nil {
		t.Fatalf("a matching flag is not a conflict: %v", err)
	}
	// The rest still comes from the snapshot.
	if got.MinDuplicates != 3 || !got.SkipObjects {
		t.Errorf("settings the caller did not state were not taken from the snapshot: %+v", got)
	}
}

// A config file states every value - by writing it down or by leaving it to a documented
// default - so there is no "did not say" to fall back on and everything is checked.
func TestConfigIsCheckedAgainstEverySetting(t *testing.T) {
	snap := snapshotWith(settingsOpts())

	// Identical but for one threshold left at its default.
	wanted := settingsOpts()
	wanted.MinLines = 0 // unset, so it normalises to the default rather than to 5

	if _, err := ReconcileConfigWithSnapshot(wanted, snap, "snap.json"); err == nil {
		t.Fatal("a config that does not match the snapshot must be reported")
	}

	if _, err := ReconcileConfigWithSnapshot(settingsOpts(), snap, "snap.json"); err != nil {
		t.Errorf("a config that matches the snapshot must run: %v", err)
	}
}

// The two sides say different things about how to resolve the conflict, because the fix is
// different: a flag can be dropped, a config file has to be edited.
func TestConflictMessageNamesTheRightFix(t *testing.T) {
	snap := snapshotWith(settingsOpts())
	wanted := Options{MinTokens: 12}

	_, flagErr := ReconcileFlagsWithSnapshot(wanted, snap, "snap.json", map[string]bool{"minTokens": true})
	if !strings.Contains(flagErr.Error(), "drop the conflicting flags") {
		t.Errorf("the command line message does not say what to do:\n%v", flagErr)
	}
	_, configErr := ReconcileConfigWithSnapshot(wanted, snap, "snap.json")
	if !strings.Contains(configErr.Error(), "config run --update-snapshot") {
		t.Errorf("the config message does not say what to do:\n%v", configErr)
	}
}

func asSettingsConflict(err error, target **SettingsConflictError) bool {
	if c, ok := err.(*SettingsConflictError); ok {
		*target = c
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// config-level file selection
// ---------------------------------------------------------------------------

// A config file's top-level ignoreFiles are relative to the CONFIG, while a workspace is
// analysed with Cwd set to the workspace. The same pattern string therefore means two
// different sets of files depending on which end it is read from, and a snapshot that
// recorded only the pattern would send a command-line run at the wrong files - reporting
// copies the config run had excluded, as a regression nobody caused.
//
// So the pattern is stored verbatim and the ROOT travels with it, as a path from the
// snapshot to the config.

func TestConfigLevelSettingsAreRecordedRelativeToTheSnapshot(t *testing.T) {
	opts := Options{
		Cwd:          "/repo/packages/app",
		SnapshotPath: "/repo/packages/app/dups.json",
		ConfigLevel: ConfigLevelFilters{
			ConfigPath:          "/repo/rev-dep.config.json",
			IgnoreFiles:         []string{"packages/app/generated/**"},
			ProcessIgnoredFiles: []string{"dist/**"},
		},
	}

	got := SnapshotParametersFrom(opts).ConfigLevelSettings
	if got == nil {
		t.Fatal("the config-level settings were not recorded")
	}
	if got.ConfigPath != "../../rev-dep.config.json" {
		t.Errorf("configPath = %q, want it relative to the snapshot's own directory", got.ConfigPath)
	}
	// Verbatim: rewriting a glob to another root is where the edge cases are.
	if len(got.IgnoreFiles) != 1 || got.IgnoreFiles[0] != "packages/app/generated/**" {
		t.Errorf("patterns were rewritten instead of kept as the config wrote them: %v", got.IgnoreFiles)
	}
}

func TestConfigLevelSettingsResolveBackToTheSameConfig(t *testing.T) {
	original := Options{
		Cwd:          "/repo/packages/app",
		SnapshotPath: "/repo/packages/app/dups.json",
		ConfigLevel: ConfigLevelFilters{
			ConfigPath:          "/repo/rev-dep.config.json",
			IgnoreFiles:         []string{"packages/app/generated/**"},
			ProcessIgnoredFiles: []string{"dist/**"},
		},
	}
	params := SnapshotParametersFrom(original)

	// A later run knows only where the snapshot is.
	restored := params.Apply(Options{Cwd: "/repo/packages/app", SnapshotPath: "/repo/packages/app/dups.json"})

	if restored.ConfigLevel.ConfigPath != "/repo/rev-dep.config.json" {
		t.Errorf("ConfigPath = %q, want the original config", restored.ConfigLevel.ConfigPath)
	}
	if restored.ConfigLevel.Root() != "/repo" {
		t.Errorf("Root() = %q, want the config's directory", restored.ConfigLevel.Root())
	}
	if diffs := params.DiffSettings(SnapshotParametersFrom(restored), nil); len(diffs) > 0 {
		t.Errorf("config-level settings did not survive the round trip: %+v", diffs)
	}
}

// The whole checkout moving is not a change to the project, so it must not invalidate the
// snapshot: both paths are recorded relative to each other.
func TestConfigLevelSettingsSurviveTheTreeMoving(t *testing.T) {
	at := func(root string) SnapshotParameters {
		return SnapshotParametersFrom(Options{
			Cwd:          root + "/packages/app",
			SnapshotPath: root + "/packages/app/dups.json",
			ConfigLevel: ConfigLevelFilters{
				ConfigPath:  root + "/rev-dep.config.json",
				IgnoreFiles: []string{"packages/app/generated/**"},
			},
		})
	}
	if diffs := at("/home/a/repo").DiffSettings(at("/mnt/build/checkout"), nil); len(diffs) > 0 {
		t.Errorf("moving the checkout changed the recorded settings: %+v", diffs)
	}
}

// A config-level pattern that differs is a difference like any other, so the settings check
// catches it rather than letting two runs disagree about which files exist.
func TestConfigLevelSettingsAreComparedLikeAnyOtherSetting(t *testing.T) {
	base := Options{
		Cwd: "/repo/pkg", SnapshotPath: "/repo/pkg/dups.json",
		ConfigLevel: ConfigLevelFilters{
			ConfigPath:  "/repo/rev-dep.config.json",
			IgnoreFiles: []string{"pkg/generated/**"},
		},
	}
	changed := base
	changed.ConfigLevel.IgnoreFiles = []string{"pkg/other/**"}

	diffs := SnapshotParametersFrom(base).DiffSettings(SnapshotParametersFrom(changed), nil)
	if len(diffs) != 1 || diffs[0].Setting != "configIgnoreFiles" {
		t.Errorf("a changed config-level pattern was not caught: %+v", diffs)
	}
}

// Nothing is recorded for a run that had no config, which is every run started from the
// command line without one.
func TestNoConfigLevelSettingsWithoutAConfig(t *testing.T) {
	params := SnapshotParametersFrom(Options{Cwd: "/repo", SnapshotPath: "/repo/dups.json"})
	if params.ConfigLevelSettings != nil {
		t.Errorf("a run with no config recorded config-level settings: %+v", params.ConfigLevelSettings)
	}
}
