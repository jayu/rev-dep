package main

import (
	"os"
	"path/filepath"
	"slices"
)

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
