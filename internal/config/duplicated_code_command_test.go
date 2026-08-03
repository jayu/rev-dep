package config

import (
	"strings"
	"testing"
)

func TestDuplicatedCodeCommand(t *testing.T) {
	cases := []struct {
		name           string
		opts           *DuplicatedCodeOptions
		rulePath       string
		globalIgnore   []string
		processIgnored []string
		want           string
	}{
		{
			name:     "root rule with no options set",
			opts:     &DuplicatedCodeOptions{Enabled: true},
			rulePath: ".",
			want:     "rev-dep duplicated-code",
		},
		{
			name:     "subdirectory rule needs --cwd or it would scan the whole repo",
			opts:     &DuplicatedCodeOptions{Enabled: true},
			rulePath: "./packages/app",
			want:     "rev-dep duplicated-code --cwd ./packages/app",
		},
		{
			name:     "trailing slash is not a different directory",
			opts:     &DuplicatedCodeOptions{Enabled: true},
			rulePath: "packages/app/",
			want:     "rev-dep duplicated-code --cwd packages/app",
		},
		{
			name:     "empty path is the root",
			opts:     &DuplicatedCodeOptions{Enabled: true},
			rulePath: "",
			want:     "rev-dep duplicated-code",
		},
		{
			name:     "every configured option is carried over",
			opts:     &DuplicatedCodeOptions{Enabled: true, BlindIdentifiers: true, MinTokens: 22, MinLines: 4, MinDepth: 1, MinStatements: 2, MinDuplicates: 3},
			rulePath: ".",
			want:     "rev-dep duplicated-code --blind-identifiers --min-tokens 22 --min-lines 4 --min-depth 1 --min-statements 2 --min-duplicates 3",
		},
		{
			// An exact comparison is the default, so there is nothing to spell out.
			name:     "options and a subdirectory together",
			opts:     &DuplicatedCodeOptions{Enabled: true, BlindIdentifiers: false, MinDepth: 1},
			rulePath: "apps/web",
			want:     "rev-dep duplicated-code --cwd apps/web --min-depth 1",
		},
		{
			name:     "each blinded axis is named on its own",
			opts:     &DuplicatedCodeOptions{Enabled: true, BlindIdentifiers: true, BlindStrings: true, BlindNumbers: true},
			rulePath: ".",
			want:     "rev-dep duplicated-code --blind-identifiers --blind-strings --blind-numbers",
		},
		{
			name:     "blinding only literals leaves names alone",
			opts:     &DuplicatedCodeOptions{Enabled: true, BlindStrings: true, BlindNumbers: true},
			rulePath: ".",
			want:     "rev-dep duplicated-code --blind-strings --blind-numbers",
		},
		{
			name:     "unset options are not spelled out",
			opts:     &DuplicatedCodeOptions{Enabled: true, MinDepth: 2},
			rulePath: ".",
			want:     "rev-dep duplicated-code --min-depth 2",
		},
		{
			name:     "a path with spaces stays one argument",
			opts:     &DuplicatedCodeOptions{Enabled: true},
			rulePath: "packages/my app",
			want:     "rev-dep duplicated-code --cwd 'packages/my app'",
		},
		{
			name:     "ignore patterns are carried over and quoted against the shell",
			opts:     &DuplicatedCodeOptions{Enabled: true, IgnoreFiles: []string{"**/*.test.ts", "src/generated/**"}},
			rulePath: ".",
			want:     "rev-dep duplicated-code --ignore-files '**/*.test.ts' --ignore-files 'src/generated/**'",
		},
		{
			name:     "a plain ignore path needs no quoting",
			opts:     &DuplicatedCodeOptions{Enabled: true, IgnoreFiles: []string{"src/a.ts"}},
			rulePath: ".",
			want:     "rev-dep duplicated-code --ignore-files src/a.ts",
		},
		{
			name:     "no detection still yields a runnable command",
			opts:     nil,
			rulePath: "apps/web",
			want:     "rev-dep duplicated-code --cwd apps/web",
		},
		{
			// The config's top-level ignoreFiles shrinks what the run ever discovered,
			// so a command without it reports a different number.
			name:         "top-level ignoreFiles is carried over",
			opts:         &DuplicatedCodeOptions{Enabled: true},
			rulePath:     ".",
			globalIgnore: []string{"**/*.test.ts"},
			want:         "rev-dep duplicated-code --ignore-files '**/*.test.ts'",
		},
		{
			name:         "top-level and detection patterns are combined, top-level first",
			opts:         &DuplicatedCodeOptions{Enabled: true, IgnoreFiles: []string{"src/gen/**"}},
			rulePath:     ".",
			globalIgnore: []string{"**/*.test.ts"},
			want:         "rev-dep duplicated-code --ignore-files '**/*.test.ts' --ignore-files 'src/gen/**'",
		},
		{
			name:         "a pattern written in both places is not repeated",
			opts:         &DuplicatedCodeOptions{Enabled: true, IgnoreFiles: []string{"src/gen/**"}},
			rulePath:     ".",
			globalIgnore: []string{"src/gen/**"},
			want:         "rev-dep duplicated-code --ignore-files 'src/gen/**'",
		},
		{
			// processIgnoredFiles puts gitignored files BACK in scope, so omitting it
			// would make the command see fewer files than the rule did.
			name:           "processIgnoredFiles is carried over",
			opts:           &DuplicatedCodeOptions{Enabled: true},
			rulePath:       ".",
			processIgnored: []string{"dist/**"},
			want:           "rev-dep duplicated-code --process-ignored-files 'dist/**'",
		},
	}

	for _, tc := range cases {
		if got := DuplicatedCodeCommand(tc.opts, tc.rulePath, tc.globalIgnore, tc.processIgnored); got != tc.want {
			t.Errorf("%s:\n  got  %s\n  want %s", tc.name, got, tc.want)
		}
	}
}

// TestDuplicatedCodeCommandCoversEveryOption fails when a new option is added to the
// config without being reflected in the hint, which would otherwise print a command that
// quietly does something different from the rule it came from.
func TestDuplicatedCodeCommandCoversEveryOption(t *testing.T) {
	full := &DuplicatedCodeOptions{
		Enabled: true, BlindIdentifiers: true, BlindStrings: true, BlindNumbers: true,
		MinTokens: 1, MinLines: 2, MinDepth: 3, MinStatements: 4, MinDuplicates: 5,
		IgnoreFiles: []string{"src/gen/**"},
	}
	got := DuplicatedCodeCommand(full, ".", nil, nil)

	for _, want := range []string{
		"--blind-identifiers", "--blind-strings", "--blind-numbers",
		"--min-tokens 1", "--min-lines 2",
		"--min-depth 3", "--min-statements 4", "--min-duplicates 5",
		"--ignore-files 'src/gen/**'",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("command is missing %q: %s", want, got)
		}
	}
}
