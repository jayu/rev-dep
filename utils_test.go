package main

import (
	"testing"
)

func TestRemoveTaggedTemplateLiterals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no tagged templates",
			input:    "const x = 5;\nconst y = 'hello';\nconst z = `template`;",
			expected: "const x = 5;\nconst y = 'hello';\nconst z = `template`;",
		},
		{
			name:     "simple tagged template",
			input:    "const x = css`color: red;`;\nconst y = 'hello';",
			expected: "const x = css;\nconst y = 'hello';",
		},
		{
			name:     "nested tagged templates",
			input:    "const Button = styled.button`\n  color: ${props => props.primary ? 'white' : 'black'};\n  background: ${props => props.primary ? 'blue' : 'gray'};\n`;",
			expected: "const Button = styled.button;",
		},
		{
			name:     "multiple tagged templates",
			input:    "const x = css`color: red;`;\nconst y = 'hello';\nconst z = styled.div`display: flex;`;",
			expected: "const x = css;\nconst y = 'hello';\nconst z = styled.div;",
		},
		{
			name:     "tagged template with backticks inside",
			input:    "const x = html`<div>Hello \\`world\\`</div>`;\nconst y = 'hello';",
			expected: "const x = html;\nconst y = 'hello';",
		},
		{
			name:     "tagged template with comments",
			input:    "// This is a comment\nconst x = css`color: red;`; // inline comment\n/* multi-line\n   comment */\nconst y = 'hello';",
			expected: "\nconst x = css; \n\nconst y = 'hello';",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveTaggedTemplateLiterals([]byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("RemoveTaggedTemplateLiterals() = %v, want %v", string(result), tt.expected)
			}
		})
	}
}

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
