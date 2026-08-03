package shell

import (
	"os/exec"
	"strings"
	"testing"
)

// The point of quoting is that a value survives being pasted into a shell. So the cases below
// are checked by actually running sh, not by comparing against an expected quoting - the
// expected quoting is what was wrong before.
func TestQuotedValueSurvivesTheShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	for _, value := range []string{
		"src/gen/**",
		"**/*.test.ts",
		"a b/*.ts",
		"src/o'brien/*.ts",
		`dist/$OUT/**`,   // $ expands inside double quotes - %q got this wrong
		"src/`id`/**",    // backticks EXECUTE inside double quotes - %q got this wrong
		`src/back\slash`, // \ is an escape inside double quotes
		"weird!name#{}",
	} {
		out, err := exec.Command("sh", "-c", "printf '%s' "+Quote(value)).Output()
		if err != nil {
			t.Errorf("%q: the shell rejected the quoted form %s: %v", value, Quote(value), err)
			continue
		}
		if string(out) != value {
			t.Errorf("%q survived as %q (quoted as %s)", value, string(out), Quote(value))
		}
	}
}

// Quoting only where it is needed keeps the common command readable.
func TestPlainValuesAreNotQuoted(t *testing.T) {
	for _, value := range []string{"src/index.ts", "packages/app", ".", "@scope/pkg", "a-b_c.ts"} {
		if got := Quote(value); got != value {
			t.Errorf("Quote(%q) = %q, want it left alone", value, got)
		}
	}
}

func TestValuesNeedingQuotesGetThem(t *testing.T) {
	for _, value := range []string{"a b", "src/**", "x?y", "$HOME", "a`b`", "a'b"} {
		if got := Quote(value); !strings.HasPrefix(got, "'") {
			t.Errorf("Quote(%q) = %q, want it quoted", value, got)
		}
	}
}
