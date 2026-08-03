// Package shell quotes values for command lines rev-dep prints for a human to copy and run.
package shell

import "strings"

// Globs need quoting as much as spaces do: an unquoted `src/**/*.test.ts` is expanded by the
// shell before rev-dep sees it.
const metacharacters = " \t\"'$`\\*?[]{}()!~#&;<>|"

// Quote wraps a value in single quotes when it needs them.
//
// Single quotes, not %q: that is Go quoting, and inside the double quotes it produces a shell
// still expands $ and executes backticks.
func Quote(value string) string {
	if !strings.ContainsAny(value, metacharacters) {
		return value
	}
	// A single quote cannot appear inside single quotes, so close, escape, reopen.
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
