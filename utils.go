package main

import (
	"os"
	"path/filepath"
	"slices"
)

var osSeparator = string(os.PathSeparator)

func StandardiseDirPath(cwd string) string {
	if string(cwd[len(cwd)-1]) == osSeparator {
		return cwd
	} else {
		return cwd + osSeparator
	}
}

func ResolveAbsoluteCwd(cwd string) string {
	if filepath.IsAbs(cwd) {
		return StandardiseDirPath(cwd)
	} else {
		binaryExecDir, _ := os.Getwd()
		return StandardiseDirPath(filepath.Join(binaryExecDir, cwd))
	}
}

// RemoveTaggedTemplateLiterals removes tagged template literals (e.g., `styled.div`\`...\“ or `css`\`...\“) from the code.
// It first removes comments to avoid false positives. It preserves regular template literals (without a tag).
func RemoveTaggedTemplateLiterals(code []byte) []byte {
	// First remove comments to avoid false positives
	code = RemoveCommentsFromCode(code)

	var result []byte
	i := 0
	n := len(code)

	for i < n {
		// Look for an identifier followed by a backtick
		if i+1 < n && (isValidIdentifierChar(code[i]) || code[i] == '.') {
			// Save the current position for potential rollback
			savePos := i

			// Find the end of the identifier
			j := i
			for j < n && (isValidIdentifierChar(code[j]) || code[j] == '.') {
				j++
			}

			// Check if this is a tagged template (identifier followed by backtick)
			// and not a regular template literal (just a backtick)
			if j < n && code[j] == '`' {
				// Check if the character before the identifier is whitespace or start of line
				isTagged := false
				if savePos == 0 {
					isTagged = true
				} else {
					// Look backwards to find the first non-whitespace character
					k := savePos - 1
					for k >= 0 && (code[k] == ' ' || code[k] == '\t' || code[k] == '\n' || code[k] == '\r') {
						k--
					}
					if k < 0 || !isValidIdentifierChar(code[k]) {
						isTagged = true
					}
				}

				if isTagged {
					// Add the identifier to the result (we'll remove the template part)
					result = append(result, code[i:j]...)

					// Skip the backtick
					i = j + 1

					// Skip the entire template literal content
					for i < n && code[i] != '`' {
						if code[i] == '\\' && i+1 < n {
							i += 2 // Skip escaped characters
						} else {
							i++
						}
					}

					// Skip the closing backtick if found
					if i < n && code[i] == '`' {
						i++
					}
					continue
				}
			}

			// If we get here, it wasn't a tagged template literal, so reset position
			i = savePos
		}

		// Handle regular template literals (without a tag)
		if i < n && code[i] == '`' {
			// Add the backtick to the result
			result = append(result, code[i])
			i++

			// Copy the entire template literal content
			for i < n && code[i] != '`' {
				if code[i] == '\\' && i+1 < n {
					// Copy escaped characters as-is
					result = append(result, code[i], code[i+1])
					i += 2
				} else {
					result = append(result, code[i])
					i++
				}
			}

			// Add the closing backtick if found
			if i < n && code[i] == '`' {
				result = append(result, code[i])
				i++
			}
			continue
		}

		// If we get here, it's not part of a template literal, so add to result
		if i < n {
			result = append(result, code[i])
			i++
		}
	}

	return result
}

func isValidIdentifierChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '$'
}

// RemoveCommentsFromCode removes all comments from the given code while preserving string literals and template literals.
func RemoveCommentsFromCode(code []byte) []byte {
	var result []byte
	i := 0
	n := len(code)

	inSingleQuoteString := false
	inDoubleQuoteString := false
	inTemplateLiteral := false
	inLineComment := false
	inBlockComment := false

	for i < n {
		// Handle comment endings first
		if inLineComment && code[i] == '\n' {
			inLineComment = false
			// Keep the newline character
			result = append(result, '\n')
			i++
			continue
		}

		if inBlockComment && i+1 < n && code[i] == '*' && code[i+1] == '/' {
			inBlockComment = false
			i += 2
			continue
		}

		// If we're in any comment, skip the character
		if inLineComment || inBlockComment {
			i++
			continue
		}

		// Handle string and template literal contexts
		if code[i] == '`' && (i == 0 || code[i-1] != '\\') {
			inTemplateLiteral = !inTemplateLiteral
			result = append(result, code[i])
			i++
			continue
		} else if !inTemplateLiteral {
			if code[i] == '\'' && (i == 0 || code[i-1] != '\\') {
				inSingleQuoteString = !inSingleQuoteString
			} else if code[i] == '"' && (i == 0 || code[i-1] != '\\') {
				inDoubleQuoteString = !inDoubleQuoteString
			}
		}

		// Only process comments when not in any string/template literal
		if !inSingleQuoteString && !inDoubleQuoteString && !inTemplateLiteral {
			// Check for line comment start
			if i+1 < n && code[i] == '/' && code[i+1] == '/' {
				inLineComment = true
				i += 2
				continue
			}
			// Check for block comment start
			if i+1 < n && code[i] == '/' && code[i+1] == '*' {
				inBlockComment = true
				i += 2
				continue
			}
		}

		// Add the character to the result if we're not in a comment
		result = append(result, code[i])
		i++
	}

	return result
}

type KV[K any, V any] struct {
	k K
	v V
}

func GetSortedMap[K string | int, V any](m map[K]V) []KV[K, V] {
	result := make([]KV[K, V], 0, len(m))

	for k, v := range m {
		result = append(result, KV[K, V]{k, v})
	}

	slices.SortFunc(result, func(a KV[K, V], b KV[K, V]) int {

		if a.k > b.k {
			return 1
		}
		if a.k < b.k {
			return -1
		}
		return 0
	})

	return result
}

func Abs(val int) int {
	if val >= 0 {
		return val
	}
	return -val
}

func PadRight(text string, char byte, length int) string {
	prefixLen := Abs(length - len(text))
	prefix := make([]byte, prefixLen)
	for range prefixLen {
		prefix = append(prefix, char)
	}

	return text + string(prefix)
}
