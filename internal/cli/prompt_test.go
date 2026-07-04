package cli

import (
	"bytes"
	"strings"
	"testing"
)

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
