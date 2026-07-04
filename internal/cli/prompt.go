package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Interactive terminal prompts (stdlib only). These are generic CLI helpers, independent of
// any particular command.

// stdinIsInteractive reports whether stdin is connected to a terminal (a character device)
// rather than a pipe or file. Used to skip interactive prompts in CI / piped invocations.
func stdinIsInteractive() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

// selectOne presents a numbered menu and returns the 0-based index and value of the chosen
// option. It re-prompts until a valid choice is entered. An empty line selects defaultIndex
// when that is a valid index; otherwise an empty line re-prompts. It reads from in and writes
// prompts to out so it can be unit-tested without a real terminal.
func selectOne(in io.Reader, out io.Writer, prompt string, options []string, defaultIndex int) (int, string, error) {
	if len(options) == 0 {
		return -1, "", fmt.Errorf("no options to select from")
	}
	hasDefault := defaultIndex >= 0 && defaultIndex < len(options)
	reader := bufio.NewReader(in)

	for {
		fmt.Fprintln(out, prompt)
		for i, opt := range options {
			marker := " "
			if hasDefault && i == defaultIndex {
				marker = "*"
			}
			fmt.Fprintf(out, " %s %2d) %s\n", marker, i+1, opt)
		}
		if hasDefault {
			fmt.Fprintf(out, "Select 1-%d [default: %d]: ", len(options), defaultIndex+1)
		} else {
			fmt.Fprintf(out, "Select 1-%d: ", len(options))
		}

		line, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(line) == "" {
			return -1, "", err // EOF with no input
		}
		line = strings.TrimSpace(line)

		if line == "" {
			if hasDefault {
				return defaultIndex, options[defaultIndex], nil
			}
			fmt.Fprintf(out, "  ⚠️  please choose a number between 1 and %d.\n\n", len(options))
			continue
		}

		n, convErr := strconv.Atoi(line)
		if convErr != nil || n < 1 || n > len(options) {
			fmt.Fprintf(out, "  ⚠️  %q is not a valid choice (1-%d).\n\n", line, len(options))
			continue
		}
		return n - 1, options[n-1], nil
	}
}
