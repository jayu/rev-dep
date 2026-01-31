package main

import (
	"bytes"
	"os"
	"sync"
)

type ImportKind int

const (
	NotTypeOrMixedImport ImportKind = iota
	OnlyTypeImport
)

type ResolvedImportType int

const (
	UserModule ResolvedImportType = iota
	NodeModule
	BuiltInModule
	ExcludedByUser
	NotResolvedModule
	AssetModule
	MonorepoModule
)

type Import struct {
	Request      string             `json:"request"`
	Kind         ImportKind         `json:"kind"`
	PathOrName   string             `json:"path"`
	ResolvedType ResolvedImportType `json:"resolvedType"`
	RequestStart int                `json:"requestStart"`
	RequestEnd   int                `json:"requestEnd"`
}

type FileImports struct {
	FilePath string   `json:"filePath"`
	Imports  []Import `json:"imports"`
}

func ParseImportsForTests(code string) []Import {
	return ParseImportsByte([]byte(code), false)
}

func isWhiteSpace(char byte) bool {
	return (char == ' ' || char == '\t' || char == '\n' || char == '\r')
}

// skipSpaces skips spaces, tabs, and newlines, returns new index
func skipSpaces(code []byte, i int) int {
	for i < len(code) && isWhiteSpace(code[i]) {
		i++
	}
	return i
}

func isByteIdentifierChar(char byte) bool {
	// 0-9 || A-Z || a-z || _
	return (char >= 48 && char <= 57) || (char >= 65 && char <= 90) || (char >= 97 && char <= 122) || char == 95
}

// parseStringLiteral extracts the string literal at position i (' or ")
func parseStringLiteral(code []byte, i int) (string, int, int, int) {
	quote := code[i]
	i++
	start := i
	for i < len(code) && code[i] != quote {
		i++
	}
	if i >= len(code) {
		return "", i, 0, 0
	}
	return string(code[start:i]), i + 1, start, i
}

func parseExpression(code []byte, i int) (string, int, int, int) {
	i = skipSpaces(code, i)
	if code[i] != '(' {
		return "", i + 1, 0, 0
	}
	i++
	parenthesisStack := 1
	stringContext := false
	stringChar := byte(0)
	module := make([]byte, 0)
	moduleStart := -1
	moduleEnd := -1
	j := 0
	for i < len(code) {
		if j > 1000 {
			panic("Too many expression parse iterations")
		}
		j++
		if code[i] == '(' {
			parenthesisStack++
			i++
			continue
		}
		if code[i] == ')' {
			parenthesisStack--
			i++
			if parenthesisStack == 0 {
				break
			} else {
				continue
			}
		}
		if !stringContext && (code[i] == '\'' || code[i] == '"') {
			stringContext = true
			stringChar = code[i]
			moduleStart = i + 1
			i++
			continue
		}
		if stringContext && (code[i] == stringChar) {
			stringContext = false
			moduleEnd = i
			stringChar = 0
			i++
			continue
		}
		if stringContext {
			module = append(module, code[i])
			i++
			continue
		}

		skippedSpacesIndex := skipSpaces(code, i)
		if skippedSpacesIndex == i {
			// If there was any valid import the loop should break already
			return "", i, 0, 0
		}
		i = skippedSpacesIndex
	}
	if moduleStart == -1 || moduleEnd == -1 {
		return string(module), i, 0, 0
	}
	return string(module), i, moduleStart, moduleEnd
}

// areAllImportsInBracesTypes checks if a named import block { ... } contains only "type" imports.
// It assumes code[i] is pointing at '{'.
func areAllImportsInBracesTypes(code []byte, i int) bool {
	i++ // skip '{'
	for i < len(code) {
		i = skipSpaces(code, i)
		if i >= len(code) {
			return false
		}
		if code[i] == '}' {
			return true // End of block, all checked items were types
		}

		// We expect "type" keyword followed by a whitespace (inside braces, 'type' must be separated from Identifier)
		// "type" is 4 chars.
		if len(code) > i+4 && bytes.HasPrefix(code[i:], []byte("type")) && isWhiteSpace(code[i+4]) {
			i += 4 // skip "type"
			i = skipSpaces(code, i)

			// Consume the identifier (and potential "as Alias") until we hit a comma or closing brace
			for i < len(code) && code[i] != ',' && code[i] != '}' {
				i++
			}
		} else {
			// Found an element that is NOT a type
			return false
		}

		// Skip comma if present
		if i < len(code) && code[i] == ',' {
			i++
		}
	}
	return false
}

// skipToStringEnd skips to the end of a string literal
func skipToStringEnd(code []byte, start int, quote byte) int {
	i := start + 1
	for i < len(code) {
		if code[i] == quote {
			return i
		}
		if code[i] == '\\' && i+1 < len(code) {
			i += 2
		} else {
			i++
		}
	}
	return i
}

// skipLineComment skips to the end of a line comment
func skipLineComment(code []byte, start int) int {
	i := start + 2
	for i < len(code) && code[i] != '\n' {
		i++
	}
	return i
}

// skipBlockComment skips to the end of a block comment
func skipBlockComment(code []byte, start int) int {
	i := start + 2
	for i+1 < len(code) && !(code[i] == '*' && code[i+1] == '/') {
		i++
	}
	if i+1 < len(code) {
		i += 2
	}
	return i
}

// ParseImportsByte parses JS/TS code and extracts all imports/exports
func ParseImportsByte(code []byte, ignoreTypeImports bool) []Import {
	imports := make([]Import, 0, 32)
	i := 0
	n := len(code)

	for i < n {
		i = skipSpaces(code, i)
		if i >= n {
			break
		}

		// skip string context
		if code[i] == '\'' {
			i = skipToStringEnd(code, i, '\'')
		} else if code[i] == '"' {
			i = skipToStringEnd(code, i, '"')
		} else if code[i] == '`' {
			i = skipToStringEnd(code, i, '`')
		}

		// skip line comment
		if i+1 < len(code) && code[i] == '/' && code[i+1] == '/' {
			i = skipLineComment(code, i)
		}

		// skip multi-line comment
		if i+1 < len(code) && code[i] == '/' && code[i+1] == '*' {
			i = skipBlockComment(code, i)
		}

		// Detect keywords
		if bytes.HasPrefix(code[i:], []byte("import")) {
			i += len("import")
			if isWhiteSpace(code[i]) || code[i] == '{' || code[i] == '"' || code[i] == '\'' || code[i] == '*' || code[i] == '(' {
				i = skipSpaces(code, i)

				kind := NotTypeOrMixedImport

				// Fix: Instead of checking isWhiteSpace(code[i+4]), we check if the next char is NOT an identifier char.
				// This handles "import type{" correctly while rejecting "import typeScript".
				if bytes.HasPrefix(code[i:], []byte("type")) {
					isTypeKeyword := false
					if i+4 >= n {
						isTypeKeyword = true
					} else if !isByteIdentifierChar(code[i+4]) {
						isTypeKeyword = true
					}

					if isTypeKeyword {
						kind = OnlyTypeImport
						i += len("type")
						i = skipSpaces(code, i)
					}
				}

				if i < n && (code[i] == '"' || code[i] == '\'') {
					module, next, start, end := parseStringLiteral(code, i)
					if module != "" {
						imports = append(imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: start, RequestEnd: end})
					}
					i = next
				} else if i < n && code[i] == '(' {
					// dynamic import
					module, next, start, end := parseExpression(code, i)
					if module != "" {
						imports = append(imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: start, RequestEnd: end})
					}
					i = next
				} else {
					// static import: find 'from' keyword

					// Check if we have { type A, type B } case (mixed import promoted to type import)
					if kind == NotTypeOrMixedImport && code[i] == '{' {
						if areAllImportsInBracesTypes(code, i) {
							kind = OnlyTypeImport
						}
					}

					for i < n && !bytes.HasPrefix(code[i:], []byte("from")) {
						// Skip comments while looking for 'from'
						if i+1 < len(code) && code[i] == '/' && code[i+1] == '/' {
							i = skipLineComment(code, i)
							continue
						}
						if i+1 < len(code) && code[i] == '/' && code[i+1] == '*' {
							i = skipBlockComment(code, i)
							continue
						}
						i++
					}
					if i < n {
						i += len("from")
						i = skipSpaces(code, i)
						if i < n && (code[i] == '"' || code[i] == '\'') {
							module, next, start, end := parseStringLiteral(code, i)
							if module != "" {
								if !ignoreTypeImports || kind == NotTypeOrMixedImport {
									imports = append(imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: start, RequestEnd: end})
								}
							}
							i = next
						}
					}
				}
			}
		} else if bytes.HasPrefix(code[i:], []byte("export")) {
			i += len("export")
			if isWhiteSpace(code[i]) || code[i] == '{' || code[i] == '*' {
				i = skipSpaces(code, i)

				kind := NotTypeOrMixedImport
				// detect export * as
				if i < n && code[i] == '*' {
					i++
					i = skipSpaces(code, i)
					if bytes.HasPrefix(code[i:], []byte("as")) {
						i += len("as")
						i = skipSpaces(code, i)
					}
				}

				// drop processing export statement if followed by "const", "function" or "default".
				if bytes.HasPrefix(code[i:], []byte("const")) || bytes.HasPrefix(code[i:], []byte("function")) || bytes.HasPrefix(code[i:], []byte("default")) {
					continue
				}

				// drop processing export statement if followed by "type".
				// Fix: Same logic as import type - check boundaries correctly
				if bytes.HasPrefix(code[i:], []byte("type")) {
					isTypeKeyword := false
					if i+4 >= n {
						isTypeKeyword = true
					} else if !isByteIdentifierChar(code[i+4]) {
						isTypeKeyword = true
					}

					if isTypeKeyword {
						kind = OnlyTypeImport
						i += len("type")
						i = skipSpaces(code, i)
						if !bytes.HasPrefix(code[i:], []byte("{")) {
							// Drop if `export type SomeType = ...`
							continue
						}
					}
				}

				// Check if we have { type A } case in export
				if kind == NotTypeOrMixedImport && code[i] == '{' {
					if areAllImportsInBracesTypes(code, i) {
						kind = OnlyTypeImport
					}
				}

				shouldDropLookingForFrom := false
				// find from keyword
				for i < n && !bytes.HasPrefix(code[i:], []byte("from")) && !shouldDropLookingForFrom {
					// Skip comments while looking for 'from'
					if i+1 < len(code) && code[i] == '/' && code[i+1] == '/' {
						i = skipLineComment(code, i)
						continue
					}
					if i+1 < len(code) && code[i] == '/' && code[i+1] == '*' {
						i = skipBlockComment(code, i)
						continue
					}

					if kind != OnlyTypeImport {
						// skip processing current export if one of the keywords are found
						if bytes.HasPrefix(code[i:], []byte("import")) && !isByteIdentifierChar(code[i+len("import")]) {
							shouldDropLookingForFrom = true
							break
						}
						if bytes.HasPrefix(code[i:], []byte("export")) && !isByteIdentifierChar(code[i+len("export")]) {
							shouldDropLookingForFrom = true
							break
						}
						if bytes.HasPrefix(code[i:], []byte("require")) && !isByteIdentifierChar(code[i+len("require")]) {
							shouldDropLookingForFrom = true
							break
						}
					}
					i++
				}

				if shouldDropLookingForFrom {
					continue
				}

				if i < n {
					i += len("from")
					i = skipSpaces(code, i)
					if i < n && (code[i] == '"' || code[i] == '\'') {
						module, next, start, end := parseStringLiteral(code, i)
						if module != "" {
							imports = append(imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: start, RequestEnd: end})
						}
						i = next
					}
				}
			}
		} else if bytes.HasPrefix(code[i:], []byte("require")) {
			i += len("require")
			if bytes.HasPrefix(code[i:], []byte("(")) || skipSpaces(code, i) > i {
				module, next, start, end := parseExpression(code, i)
				if module != "" {
					imports = append(imports, Import{Request: module, Kind: NotTypeOrMixedImport, ResolvedType: NotResolvedModule, RequestStart: start, RequestEnd: end})
				}
				i = next
			}
		} else {
			// skip non-keyword
			i++
		}
	}

	return imports
}

func ParseImportsFromFiles(filePaths []string, ignoreTypeImports bool) ([]FileImports, int) {
	results := make([]FileImports, 0, len(filePaths))
	var mu sync.Mutex
	var wg sync.WaitGroup

	errCount := 0

	for _, filePath := range filePaths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			// path is internal-normalized (forward slashes); convert to OS-native for file IO
			fileContent, err := os.ReadFile(DenormalizePathForOS(path))
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
				return
			}

			imports := ParseImportsByte(fileContent, ignoreTypeImports)

			mu.Lock()
			results = append(results, FileImports{
				FilePath: filePath,
				Imports:  imports,
			})
			mu.Unlock()
		}(filePath)
	}

	wg.Wait()
	return results, errCount
}
