package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// Interactive terminal prompts. These are generic CLI helpers, independent of any particular
// command.

const (
	// boxBorderCost is the number of columns the box frame consumes on each rendered line:
	// "│ " on the left and " │" on the right.
	boxBorderCost = 4
	// boxMinContentWidth keeps the text area usable on very narrow terminals; there is no maximum,
	// so the box uses the full terminal width.
	boxMinContentWidth = 16
	// fallbackTerminalWidth is assumed when the real width cannot be determined (e.g. output is
	// not a terminal).
	fallbackTerminalWidth = 80
)

// terminalWidth returns the width of the controlling terminal in columns, or fallbackTerminalWidth
// when stdout is not a terminal or the size cannot be determined.
func terminalWidth() int {
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		return width
	}
	return fallbackTerminalWidth
}

// boxContentWidth is the text area width for boxed prompts: the full terminal width minus the
// frame, floored so the box stays usable on very narrow terminals.
func boxContentWidth() int {
	width := terminalWidth() - boxBorderCost
	if width < boxMinContentWidth {
		width = boxMinContentWidth
	}
	return width
}

// wrapText word-wraps a single line of text to width columns. Words longer than width are hard-split
// so nothing overflows. An empty string yields a single empty line (preserving blank separators).
func wrapText(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := ""
	for _, word := range words {
		for utf8.RuneCountInString(word) > width {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			runes := []rune(word)
			lines = append(lines, string(runes[:width]))
			word = string(runes[width:])
		}
		switch {
		case current == "":
			current = word
		case utf8.RuneCountInString(current)+1+utf8.RuneCountInString(word) <= width:
			current += " " + word
		default:
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// wrapLabeled wraps a prefixed line (e.g. an option "  1) …") to width, hanging-indenting
// continuation lines so they align under the label text rather than the marker.
func wrapLabeled(prefix, text string, width int) []string {
	indent := utf8.RuneCountInString(prefix)
	available := width - indent
	if available < 1 {
		available = 1
	}
	wrapped := wrapText(text, available)
	lines := []string{prefix + wrapped[0]}
	pad := strings.Repeat(" ", indent)
	for _, continuation := range wrapped[1:] {
		lines = append(lines, pad+continuation)
	}
	return lines
}

// fprintBox draws lines inside a rounded box, left-aligned and padded to the widest line, to
// visually group a prompt (question, options, and the select line) and separate it from the
// surrounding output. Lines are expected to be pre-wrapped to boxContentWidth; the box width is
// capped to it defensively so the frame never overflows the terminal.
func fprintBox(out io.Writer, lines []string) {
	width := 0
	for _, line := range lines {
		if runes := utf8.RuneCountInString(line); runes > width {
			width = runes
		}
	}
	if cap := boxContentWidth(); width > cap {
		width = cap
	}
	border := strings.Repeat("─", width+2)
	fmt.Fprintf(out, "╭%s╮\n", border)
	for _, line := range lines {
		pad := width - utf8.RuneCountInString(line)
		if pad < 0 {
			pad = 0
		}
		fmt.Fprintf(out, "│ %s%s │\n", line, strings.Repeat(" ", pad))
	}
	fmt.Fprintf(out, "╰%s╯\n", border)
}

// buildPromptLines assembles the boxed prompt content — the (possibly multi-line) question, a
// blank separator, the numbered options, another blank, and the "Type 1-N" line — with every line
// word-wrapped to contentWidth. Options hang-indent their continuation lines under the label.
func buildPromptLines(prompt string, options []string, defaultIndex, contentWidth int) []string {
	hasDefault := defaultIndex >= 0 && defaultIndex < len(options)

	var lines []string
	// A prompt may span several lines (a question plus an explanation); wrap each segment.
	for _, segment := range strings.Split(prompt, "\n") {
		lines = append(lines, wrapText(segment, contentWidth)...)
	}
	lines = append(lines, "")
	for i, opt := range options {
		marker := " "
		if hasDefault && i == defaultIndex {
			marker = "*"
		}
		prefix := fmt.Sprintf(" %s %2d) ", marker, i+1)
		lines = append(lines, wrapLabeled(prefix, opt, contentWidth)...)
	}
	if hasDefault {
		lines = append(lines, "", fmt.Sprintf("Type 1-%d [default: %d]:", len(options), defaultIndex+1))
	} else {
		lines = append(lines, "", fmt.Sprintf("Type 1-%d:", len(options)))
	}
	return lines
}

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
		// Build the whole prompt (question, options, and select line) as one boxed block, then
		// read the answer on the line below the box.
		fprintBox(out, buildPromptLines(prompt, options, defaultIndex, boxContentWidth()))
		fmt.Fprint(out, "> ")

		line, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(line) == "" {
			return -1, "", err // EOF with no input
		}
		line = strings.TrimSpace(line)

		if line == "" {
			if hasDefault {
				fmt.Fprintln(out)
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
		fmt.Fprintln(out)
		return n - 1, options[n-1], nil
	}
}
