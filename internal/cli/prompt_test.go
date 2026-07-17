package cli

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWrapText(t *testing.T) {
	if got := wrapText("", 10); len(got) != 1 || got[0] != "" {
		t.Errorf("empty text should yield one empty line, got %v", got)
	}
	if got := wrapText("one two three four", 8); !strings.EqualFold(strings.Join(got, "|"), "one two|three|four") {
		t.Errorf("word wrap: got %v", got)
	}
	// A word longer than the width is hard-split.
	got := wrapText("supercalifragilistic", 6)
	for _, line := range got {
		if utf8.RuneCountInString(line) > 6 {
			t.Errorf("hard-split line %q exceeds width 6", line)
		}
	}
	if strings.Join(got, "") != "supercalifragilistic" {
		t.Errorf("hard-split lost characters: %v", got)
	}
}

func TestWrapLabeledHangingIndent(t *testing.T) {
	lines := wrapLabeled("  1) ", "alpha beta gamma delta", 12)
	if len(lines) < 2 {
		t.Fatalf("expected wrapping into multiple lines, got %v", lines)
	}
	if !strings.HasPrefix(lines[0], "  1) ") {
		t.Errorf("first line should carry the prefix, got %q", lines[0])
	}
	indent := strings.Repeat(" ", utf8.RuneCountInString("  1) "))
	for _, cont := range lines[1:] {
		if !strings.HasPrefix(cont, indent) {
			t.Errorf("continuation %q should be hang-indented by %d spaces", cont, len(indent))
		}
	}
	for _, line := range lines {
		if utf8.RuneCountInString(line) > 12 {
			t.Errorf("line %q exceeds width 12", line)
		}
	}
}

func TestBuildPromptLinesFitsWidth(t *testing.T) {
	prompt := "A long question that certainly needs to wrap across several lines to fit.\nAnd a second explanatory sentence here."
	options := []string{
		"A short option",
		"A considerably longer option label that must be wrapped and hang-indented under its own text",
	}
	const width = 30
	lines := buildPromptLines(prompt, options, 0, width)
	for _, line := range lines {
		if utf8.RuneCountInString(line) > width {
			t.Errorf("line exceeds content width %d: %q (%d)", width, line, utf8.RuneCountInString(line))
		}
	}
	// The default marker appears on the chosen option.
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "*  1)") {
		t.Errorf("expected default marker on option 1, got:\n%s", joined)
	}
	if !strings.Contains(joined, "Type 1-2 [default: 1]:") {
		t.Errorf("expected type line, got:\n%s", joined)
	}
}

func TestSelectOne(t *testing.T) {
	options := []string{"Option 1", "Option 2", "Option 3"}

	t.Run("valid numeric choice", func(t *testing.T) {
		var out bytes.Buffer
		idx, val, err := selectOne(strings.NewReader("2\n"), &out, "Pick:", options, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx != 1 || val != "Option 2" {
			t.Fatalf("expected (1, Option 2), got (%d, %s)", idx, val)
		}
	})

	t.Run("empty line uses default", func(t *testing.T) {
		var out bytes.Buffer
		idx, val, err := selectOne(strings.NewReader("\n"), &out, "Pick:", options, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx != 2 || val != "Option 3" {
			t.Fatalf("expected default (2, Option 3), got (%d, %s)", idx, val)
		}
	})

	t.Run("re-prompts on invalid then accepts valid", func(t *testing.T) {
		var out bytes.Buffer
		idx, _, err := selectOne(strings.NewReader("nope\n9\n1\n"), &out, "Pick:", options, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx != 0 {
			t.Fatalf("expected index 0, got %d", idx)
		}
		if strings.Count(out.String(), "not a valid choice") != 2 {
			t.Fatalf("expected two invalid-choice warnings, got output:\n%s", out.String())
		}
	})

	t.Run("empty line without default re-prompts", func(t *testing.T) {
		var out bytes.Buffer
		idx, _, err := selectOne(strings.NewReader("\n3\n"), &out, "Pick:", options, -1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx != 2 {
			t.Fatalf("expected index 2, got %d", idx)
		}
		if !strings.Contains(out.String(), "please choose a number") {
			t.Fatalf("expected a re-prompt warning, got:\n%s", out.String())
		}
	})

	t.Run("EOF with no input returns error", func(t *testing.T) {
		var out bytes.Buffer
		if _, _, err := selectOne(strings.NewReader(""), &out, "Pick:", options, -1); err == nil {
			t.Fatalf("expected error on EOF with no input")
		}
	})

	t.Run("no options returns error", func(t *testing.T) {
		var out bytes.Buffer
		if _, _, err := selectOne(strings.NewReader("1\n"), &out, "Pick:", nil, -1); err == nil {
			t.Fatalf("expected error when there are no options")
		}
	})
}
