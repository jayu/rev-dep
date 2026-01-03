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
)

type Import struct {
	Request      string             `json:"request"`
	Kind         ImportKind         `json:"kind"`
	PathOrName   string             `json:"path"`
	ResolvedType ResolvedImportType `json:"resolvedType"`
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
func parseStringLiteral(code []byte, i int) (string, int) {
	quote := code[i]
	i++
	start := i
	for i < len(code) && code[i] != quote {
		i++
	}
	if i >= len(code) {
		return "", i
	}
	return string(code[start:i]), i + 1
}

func parseExpression(code []byte, i int) (string, int) {
	i = skipSpaces(code, i)
	if code[i] != '(' {
		return "", i + 1
	}
	i++
	parenthesisStack := 1
	stringContext := false
	stringChar := byte(0)
	module := make([]byte, 0)
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
			i++
			continue
		}
		if stringContext && (code[i] == stringChar) {
			stringContext = false
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
			return "", i
		}
		i = skippedSpacesIndex
	}
	return string(module), i
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
		if bytes.HasPrefix(code[i:], []byte("'")) {
			i++
			endOfString := bytes.Index(code[i:], []byte("'"))
			i += endOfString
		}
		if bytes.HasPrefix(code[i:], []byte("\"")) {
			i++
			endOfString := bytes.Index(code[i:], []byte("\""))
			i += endOfString
		}
		if bytes.HasPrefix(code[i:], []byte("`")) {
			i++
			endOfString := bytes.Index(code[i:], []byte("`"))
			i += endOfString
		}

		// skip line comment
		if bytes.HasPrefix(code[i:], []byte("//")) {
			i += 2
			endOfLineIndex := bytes.Index(code[i:], []byte("\n"))

			i += endOfLineIndex
		}

		// skip multi-line comment
		if bytes.HasPrefix(code[i:], []byte("/*")) {
			i += 2
			endOfCommentIndex := bytes.Index(code[i:], []byte("*/"))

			i += endOfCommentIndex
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
					module, next := parseStringLiteral(code, i)
					if module != "" {
						imports = append(imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule})
					}
					i = next
				} else if i < n && code[i] == '(' {
					// dynamic import
					module, next := parseExpression(code, i)
					if module != "" {
						imports = append(imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule})
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
						i++
					}
					if i < n {
						i += len("from")
						i = skipSpaces(code, i)
						if i < n && (code[i] == '"' || code[i] == '\'') {
							module, next := parseStringLiteral(code, i)
							if module != "" {
								if !ignoreTypeImports || kind == NotTypeOrMixedImport {
									imports = append(imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule})
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
						module, next := parseStringLiteral(code, i)
						if module != "" {
							imports = append(imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule})
						}
						i = next
					}
				}
			}
		} else if bytes.HasPrefix(code[i:], []byte("require")) {
			i += len("require")
			if bytes.HasPrefix(code[i:], []byte("(")) || skipSpaces(code, i) > i {
				module, next := parseExpression(code, i)
				if module != "" {
					imports = append(imports, Import{Request: module, Kind: NotTypeOrMixedImport, ResolvedType: NotResolvedModule})
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

func ImportKindToString(kind ImportKind) string {
	switch kind {
	case NotTypeOrMixedImport:
		return "NotTypeOrMixedImport"
	case OnlyTypeImport:
		return "OnlyTypeImport"
	default:
		return "Unknown"
	}
}

func ResolvedImportTypeToString(resolvedType ResolvedImportType) string {
	switch resolvedType {
	case UserModule:
		return "UserModule"
	case NodeModule:
		return "NodeModule"
	case BuiltInModule:
		return "BuiltInModule"
	case ExcludedByUser:
		return "ExcludedByUser"
	case NotResolvedModule:
		return "NotResolvedModule"
	case AssetModule:
		return "AssetModule"
	default:
		return "Unknown"
	}
}

/*
All export from examples

export * from "module-name";
export * as name1 from "module-name";
export { name1,  nameN } from "module-name";
export { import1 as name1, import2 as name2,  nameN } from "module-name";
export { default } from "module-name";
export { default as name1 } from "module-name";
export { type MyType } from "./types";
export type { MyType } from "./types";
export { default, type MyType2 } from "./types";

All import examples

import defaultExport from "module-name";
import * as name from "module-name";
import { export1 } from "module-name";
import { export1 as alias1 } from "module-name";
import { default as alias } from "module-name";
import { export1, export2 } from "module-name";
import { export1, export2 as alias2 } from "module-name";
import { "string name" as alias } from "module-name";
import defaultExport, { export1 } from "module-name";
import defaultExport, * as name from "module-name";
import "module-name";
import { type MyType2 } from "./types";
import type {  MyType2 } from "./types";
import fnA, { type MyType3 } from "./types";
import fnA, { type MyType3, MyVal } from "./types";

*/
