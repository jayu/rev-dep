package main

import (
	"testing"
)

func TestRemoveCommentsFromCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no comments",
			input:    "const x = 5;\nconst y = 'hello';\nconst z = `template`;",
			expected: "const x = 5;\nconst y = 'hello';\nconst z = `template`;",
		},
		{
			name:     "single line comments",
			input:    "// This is a comment\nconst x = 5; // inline comment\nconst y = 'hello'; // another comment",
			expected: "\nconst x = 5; \nconst y = 'hello'; ",
		},
		{
			name:     "multi-line comments",
			input:    "someCode;/* This is a\n   multi-line comment */\nconst x = 5;\nconst y = 'hello';",
			expected: "someCode;\nconst x = 5;\nconst y = 'hello';",
		},
		{
			name:     "comments in strings",
			input:    "const x = '// not a comment';\nconst y = \"/* not a comment */\";\nconst z = `// also not a comment`;",
			expected: "const x = '// not a comment';\nconst y = \"/* not a comment */\";\nconst z = `// also not a comment`;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveCommentsFromCode([]byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("RemoveCommentsFromCode() = %v, want %v", string(result), tt.expected)
			}
		})
	}
}
