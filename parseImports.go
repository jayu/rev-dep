package main

import (
	"bytes"
	"os"
	"runtime"
	"sync"
)

type ImportKind uint8

const (
	NotTypeOrMixedImport ImportKind = iota
	OnlyTypeImport
)

type ResolvedImportType uint8

const (
	UserModule ResolvedImportType = iota
	NodeModule
	BuiltInModule
	ExcludedByUser
	NotResolvedModule
	AssetModule
	MonorepoModule
	LocalExportDeclaration
)

type ParseMode uint8

const (
	ParseModeBasic    ParseMode = iota // Current behavior
	ParseModeDetailed                  // Keyword tracking + local exports
)

type NodeModulesMatchingStrategy uint8

const (
	NodeModulesMatchingStrategySelfResolver NodeModulesMatchingStrategy = iota
	NodeModulesMatchingStrategyRootResolver
	NodeModulesMatchingStrategyCwdResolver
)

type KeywordInfo struct {
	Name       string // Original name ("default" for default imports, "*" for namespace)
	Alias      string // Local alias if "as" used, empty otherwise
	Start      uint32 // Byte offset of identifier start in source
	End        uint32 // Byte offset of identifier end in source
	Position   uint32 // 0-based position in the import/export list
	CommaAfter uint32 // Byte offset of `,` after this entry (0 if no trailing comma)
	IsType     bool   // true if inline "type" keyword precedes this identifier
}

type KeywordMap struct {
	Keywords []KeywordInfo  // Insertion-ordered slice; primary storage
	index    map[string]int // Name -> index; lazily built on first Get() call
}

func (km *KeywordMap) Get(name string) (KeywordInfo, bool) {
	if km.index == nil {
		km.index = make(map[string]int, len(km.Keywords))
		for i, kw := range km.Keywords {
			km.index[kw.Name] = i
		}
	}
	idx, ok := km.index[name]
	if !ok {
		return KeywordInfo{}, false
	}
	return km.Keywords[idx], true
}

func (km *KeywordMap) Add(kw KeywordInfo) {
	km.Keywords = append(km.Keywords, kw)
	km.index = nil // invalidate index
}

func (km *KeywordMap) Len() int {
	return len(km.Keywords)
}

type Import struct {
	Request      string             `json:"request"`
	PathOrName   string             `json:"path"`
	Keywords     *KeywordMap        `json:"-"` // nil in basic mode
	RequestStart uint32             `json:"requestStart"`
	RequestEnd   uint32             `json:"requestEnd"`
	Kind         ImportKind         `json:"kind"`
	ResolvedType ResolvedImportType `json:"resolvedType"`

	IsDynamicImport bool `json:"-"` // true for `import('...')`
	IsLocalExport   bool `json:"-"` // true for `export const/default/function/...` without `from`

	// New fields — populated only in ParseModeDetailed
	ExportKeyStart     uint32 `json:"-"` // Byte offset where `export` keyword starts
	ExportKeyEnd       uint32 `json:"-"` // Byte offset right after `export `
	ExportDeclStart    uint32 `json:"-"` // After `export [default] ` — where the declaration starts
	ExportBraceStart   uint32 `json:"-"` // Position of `{` in brace-list exports (0 if not brace-list)
	ExportBraceEnd     uint32 `json:"-"` // Position after `}` in brace-list exports
	ExportStatementEnd uint32 `json:"-"` // Position after full statement including optional `;`
}

type FileImports struct {
	FilePath string   `json:"filePath"`
	Imports  []Import `json:"imports"`
}

func ParseImportsForTests(code string) []Import {
	return ParseImportsByte([]byte(code), false, ParseModeBasic)
}

func ParseImportsForTestsDetailed(code string) []Import {
	return ParseImportsByte([]byte(code), false, ParseModeDetailed)
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

// skipSpacesAndComments skips whitespace, line comments, and block comments
func skipSpacesAndComments(code []byte, i int) int {
	n := len(code)
	for i < n {
		i = skipSpaces(code, i)
		if i+1 < n && code[i] == '/' && code[i+1] == '/' {
			i = skipLineComment(code, i)
			continue
		}
		if i+1 < n && code[i] == '/' && code[i+1] == '*' {
			i = skipBlockComment(code, i)
			continue
		}
		break
	}
	return i
}

// skipOptionalSemicolon skips whitespace (spaces/tabs only) then `;` if present.
// Returns position after `;` if found, or the original position i if not.
func skipOptionalSemicolon(code []byte, i int) int {
	n := len(code)
	j := i
	for j < n && (code[j] == ' ' || code[j] == '\t') {
		j++
	}
	if j < n && code[j] == ';' {
		return j + 1
	}
	return i
}

// parseIdentifier extracts a single identifier token starting at position i.
// Returns the identifier string, next position, and byte offsets (start, end).
func parseIdentifier(code []byte, i int) (name string, next int, start int, end int) {
	n := len(code)
	if i >= n || !isByteIdentifierChar(code[i]) {
		return "", i, i, i
	}
	start = i
	for i < n && isByteIdentifierChar(code[i]) {
		i++
	}
	return string(code[start:i]), i, start, i
}

// parseImportKeywords parses the identifier portion of an import statement
// (everything between `import [type]` and `from`).
// isWholeStatementType is true if the import statement has `import type` prefix.
func parseImportKeywords(code []byte, i int, isWholeStatementType bool) (keywords *KeywordMap, next int) {
	n := len(code)
	keywords = &KeywordMap{Keywords: make([]KeywordInfo, 0, 2)}
	position := 0

	i = skipSpacesAndComments(code, i)
	if i >= n {
		return keywords, i
	}

	// Side-effect import: `import "mod"` or dynamic import: `import("mod")`
	if code[i] == '"' || code[i] == '\'' || code[i] == '(' {
		return nil, i
	}

	// Namespace import: `* as Name`
	if code[i] == '*' {
		starStart := i
		i++ // skip *
		i = skipSpacesAndComments(code, i)
		// expect "as"
		if i+2 <= n && bytes.HasPrefix(code[i:], []byte("as")) && (i+2 >= n || !isByteIdentifierChar(code[i+2])) {
			i += 2
			i = skipSpacesAndComments(code, i)
			alias, next, _, aliasEnd := parseIdentifier(code, i)
			if alias != "" {
				keywords.Add(KeywordInfo{
					Name:     "*",
					Alias:    alias,
					IsType:   isWholeStatementType,
					Start:    uint32(starStart),
					End:      uint32(aliasEnd),
					Position: uint32(position),
				})
			}
			return keywords, next
		}
		return keywords, i
	}

	// Named import: `{ A, B as C, type D }`
	if code[i] == '{' {
		i++ // skip '{'
		for i < n {
			i = skipSpacesAndComments(code, i)
			if i >= n || code[i] == '}' {
				i++ // skip '}'
				break
			}

			kwIsType := isWholeStatementType
			kwStart := i

			// Check for inline `type` keyword
			if i+4 < n && bytes.HasPrefix(code[i:], []byte("type")) && !isByteIdentifierChar(code[i+4]) {
				// Lookahead: is this `type` as inline type modifier or identifier named "type"?
				// If next non-space token is an identifier or string, it's a type modifier
				saved := i
				i += 4
				i = skipSpacesAndComments(code, i)
				if i < n && (isByteIdentifierChar(code[i]) || code[i] == '"' || code[i] == '\'') {
					kwIsType = true
					kwStart = saved
				} else {
					// "type" is the identifier itself (e.g. `{ type }` or `{ type as X }`)
					i = saved
				}
			}

			// Check for string name: `"string name" as alias`
			if i < n && (code[i] == '"' || code[i] == '\'') {
				strName, strNext, _, _ := parseStringLiteral(code, i)
				i = strNext
				i = skipSpacesAndComments(code, i)
				// expect "as"
				alias := ""
				aliasEnd := i
				if i+2 <= n && bytes.HasPrefix(code[i:], []byte("as")) && (i+2 >= n || !isByteIdentifierChar(code[i+2])) {
					i += 2
					i = skipSpacesAndComments(code, i)
					alias, i, _, aliasEnd = parseIdentifier(code, i)
				}
				keywords.Add(KeywordInfo{
					Name:     strName,
					Alias:    alias,
					IsType:   kwIsType,
					Start:    uint32(kwStart),
					End:      uint32(aliasEnd),
					Position: uint32(position),
				})
				position++
			} else {
				// Regular identifier
				name, next, _, nameEnd := parseIdentifier(code, i)
				if name == "" {
					i = next + 1 // skip unexpected char
					continue
				}
				i = next
				i = skipSpacesAndComments(code, i)

				alias := ""
				aliasEnd := nameEnd
				if i+2 <= n && bytes.HasPrefix(code[i:], []byte("as")) && (i+2 >= n || !isByteIdentifierChar(code[i+2])) {
					i += 2
					i = skipSpacesAndComments(code, i)
					alias, i, _, aliasEnd = parseIdentifier(code, i)
				}
				keywords.Add(KeywordInfo{
					Name:     name,
					Alias:    alias,
					IsType:   kwIsType,
					Start:    uint32(kwStart),
					End:      uint32(aliasEnd),
					Position: uint32(position),
				})
				position++
			}

			// Skip comma
			i = skipSpacesAndComments(code, i)
			if i < n && code[i] == ',' {
				i++
			}
		}
		// After closing brace, check for `from`
		i = skipSpacesAndComments(code, i)
		return keywords, i
	}

	// Default import: `Default` or `Default, { A }` or `Default, * as Ns`
	name, next, nameStart, nameEnd := parseIdentifier(code, i)
	if name == "" {
		return nil, i
	}
	i = next

	keywords.Add(KeywordInfo{
		Name:     "default",
		Alias:    name,
		IsType:   isWholeStatementType,
		Start:    uint32(nameStart),
		End:      uint32(nameEnd),
		Position: uint32(position),
	})
	position++

	i = skipSpacesAndComments(code, i)

	// Check for comma (mixed import)
	if i < n && code[i] == ',' {
		i++ // skip comma
		i = skipSpacesAndComments(code, i)

		if i < n && code[i] == '*' {
			// Default + namespace: `Default, * as Ns`
			starStart := i
			i++
			i = skipSpacesAndComments(code, i)
			if i+2 <= n && bytes.HasPrefix(code[i:], []byte("as")) && (i+2 >= n || !isByteIdentifierChar(code[i+2])) {
				i += 2
				i = skipSpacesAndComments(code, i)
				alias, next, _, aliasEnd := parseIdentifier(code, i)
				if alias != "" {
					keywords.Add(KeywordInfo{
						Name:     "*",
						Alias:    alias,
						IsType:   isWholeStatementType,
						Start:    uint32(starStart),
						End:      uint32(aliasEnd),
						Position: uint32(position),
					})
				}
				i = next
			}
		} else if i < n && code[i] == '{' {
			// Default + named: `Default, { A, B }`
			innerKw, innerNext := parseImportKeywords(code, i, isWholeStatementType)
			if innerKw != nil {
				for _, kw := range innerKw.Keywords {
					kw.Position = uint32(position)
					keywords.Add(kw)
					position++
				}
			}
			i = innerNext
		}
	}

	return keywords, i
}

// parseExportKeywords parses the identifier portion of an export statement.
// Handles: `{ A, B as C, type D }`, `* as Name`, `*`
// Returns brace positions (0 if no braces, e.g. star exports).
func parseExportKeywords(code []byte, i int, isWholeStatementType bool) (keywords *KeywordMap, braceStart int, braceEnd int, next int) {
	n := len(code)
	keywords = &KeywordMap{Keywords: make([]KeywordInfo, 0, 2)}
	position := 0

	i = skipSpacesAndComments(code, i)
	if i >= n {
		return keywords, 0, 0, i
	}

	// Star export: `*` or `* as Name`
	if code[i] == '*' {
		starStart := i
		i++
		i = skipSpacesAndComments(code, i)
		alias := ""
		aliasEnd := starStart + 1
		if i+2 <= n && bytes.HasPrefix(code[i:], []byte("as")) && (i+2 >= n || !isByteIdentifierChar(code[i+2])) {
			i += 2
			i = skipSpacesAndComments(code, i)
			alias, i, _, aliasEnd = parseIdentifier(code, i)
		}
		keywords.Add(KeywordInfo{
			Name:     "*",
			Alias:    alias,
			IsType:   isWholeStatementType,
			Start:    uint32(starStart),
			End:      uint32(aliasEnd),
			Position: uint32(position),
		})
		return keywords, 0, 0, i
	}

	// Named export: `{ A, B as C, type D }`
	if code[i] == '{' {
		braceStart = i
		i++ // skip '{'
		for i < n {
			i = skipSpacesAndComments(code, i)
			if i >= n || code[i] == '}' {
				i++ // skip '}'
				braceEnd = i
				break
			}

			kwIsType := isWholeStatementType
			kwStart := i

			// Check for inline `type` keyword
			if i+4 < n && bytes.HasPrefix(code[i:], []byte("type")) && !isByteIdentifierChar(code[i+4]) {
				saved := i
				i += 4
				i = skipSpacesAndComments(code, i)
				if i < n && (isByteIdentifierChar(code[i]) || code[i] == '"' || code[i] == '\'') {
					kwIsType = true
					kwStart = saved
				} else {
					i = saved
				}
			}

			// Check for string name
			if i < n && (code[i] == '"' || code[i] == '\'') {
				strName, strNext, _, _ := parseStringLiteral(code, i)
				i = strNext
				i = skipSpacesAndComments(code, i)
				alias := ""
				aliasEnd := i
				if i+2 <= n && bytes.HasPrefix(code[i:], []byte("as")) && (i+2 >= n || !isByteIdentifierChar(code[i+2])) {
					i += 2
					i = skipSpacesAndComments(code, i)
					alias, i, _, aliasEnd = parseIdentifier(code, i)
				}
				keywords.Add(KeywordInfo{
					Name:     strName,
					Alias:    alias,
					IsType:   kwIsType,
					Start:    uint32(kwStart),
					End:      uint32(aliasEnd),
					Position: uint32(position),
				})
				position++
			} else {
				name, next, _, nameEnd := parseIdentifier(code, i)
				if name == "" {
					i = next + 1
					continue
				}
				i = next
				i = skipSpacesAndComments(code, i)

				alias := ""
				aliasEnd := nameEnd
				if i+2 <= n && bytes.HasPrefix(code[i:], []byte("as")) && (i+2 >= n || !isByteIdentifierChar(code[i+2])) {
					i += 2
					i = skipSpacesAndComments(code, i)
					alias, i, _, aliasEnd = parseIdentifier(code, i)
				}
				keywords.Add(KeywordInfo{
					Name:     name,
					Alias:    alias,
					IsType:   kwIsType,
					Start:    uint32(kwStart),
					End:      uint32(aliasEnd),
					Position: uint32(position),
				})
				position++
			}

			i = skipSpacesAndComments(code, i)
			if i < n && code[i] == ',' {
				keywords.Keywords[len(keywords.Keywords)-1].CommaAfter = uint32(i)
				i++
			}
		}
		return keywords, braceStart, braceEnd, i
	}

	return keywords, 0, 0, i
}

// parseLocalExportKeyword parses a single exported keyword from local export statements.
// Handles: default, const/let/var, function, async function, class, type, interface, enum
func parseLocalExportKeyword(code []byte, i int) (keyword KeywordInfo, next int) {
	n := len(code)
	i = skipSpacesAndComments(code, i)
	if i >= n {
		return KeywordInfo{}, i
	}

	// `default`
	if bytes.HasPrefix(code[i:], []byte("default")) && (i+7 >= n || !isByteIdentifierChar(code[i+7])) {
		return KeywordInfo{Name: "default", Start: uint32(i), End: uint32(i + 7)}, i + 7
	}

	// `const`, `let`, `var`
	for _, kw := range []string{"const", "let", "var"} {
		kwLen := len(kw)
		if bytes.HasPrefix(code[i:], []byte(kw)) && (i+kwLen >= n || !isByteIdentifierChar(code[i+kwLen])) {
			j := i + kwLen
			j = skipSpacesAndComments(code, j)
			name, next, nameStart, nameEnd := parseIdentifier(code, j)
			if name != "" {
				return KeywordInfo{Name: name, Start: uint32(nameStart), End: uint32(nameEnd)}, next
			}
			return KeywordInfo{}, j
		}
	}

	// `async function`
	if bytes.HasPrefix(code[i:], []byte("async")) && (i+5 >= n || !isByteIdentifierChar(code[i+5])) {
		j := i + 5
		j = skipSpacesAndComments(code, j)
		if bytes.HasPrefix(code[j:], []byte("function")) && (j+8 >= n || !isByteIdentifierChar(code[j+8])) {
			j += 8
			j = skipSpacesAndComments(code, j)
			// Skip optional *
			if j < n && code[j] == '*' {
				j++
				j = skipSpacesAndComments(code, j)
			}
			name, next, nameStart, nameEnd := parseIdentifier(code, j)
			if name != "" {
				return KeywordInfo{Name: name, Start: uint32(nameStart), End: uint32(nameEnd)}, next
			}
			return KeywordInfo{}, j
		}
	}

	// `function`
	if bytes.HasPrefix(code[i:], []byte("function")) && (i+8 >= n || !isByteIdentifierChar(code[i+8])) {
		j := i + 8
		j = skipSpacesAndComments(code, j)
		// Skip optional *
		if j < n && code[j] == '*' {
			j++
			j = skipSpacesAndComments(code, j)
		}
		name, next, nameStart, nameEnd := parseIdentifier(code, j)
		if name != "" {
			return KeywordInfo{Name: name, Start: uint32(nameStart), End: uint32(nameEnd)}, next
		}
		return KeywordInfo{}, j
	}

	// `class`
	if bytes.HasPrefix(code[i:], []byte("class")) && (i+5 >= n || !isByteIdentifierChar(code[i+5])) {
		j := i + 5
		j = skipSpacesAndComments(code, j)
		name, next, nameStart, nameEnd := parseIdentifier(code, j)
		if name != "" {
			return KeywordInfo{Name: name, Start: uint32(nameStart), End: uint32(nameEnd)}, next
		}
		return KeywordInfo{}, j
	}

	// `namespace`, `module`
	for _, kw := range []string{"namespace", "module"} {
		kwLen := len(kw)
		if bytes.HasPrefix(code[i:], []byte(kw)) && (i+kwLen >= n || !isByteIdentifierChar(code[i+kwLen])) {
			j := i + kwLen
			j = skipSpacesAndComments(code, j)
			name, next, nameStart, nameEnd := parseIdentifier(code, j)
			if name != "" {
				return KeywordInfo{Name: name, Start: uint32(nameStart), End: uint32(nameEnd)}, next
			}
			return KeywordInfo{}, j
		}
	}

	// `type`, `interface`, `enum` (IsType = true)
	for _, kw := range []string{"type", "interface", "enum"} {
		kwLen := len(kw)
		if bytes.HasPrefix(code[i:], []byte(kw)) && (i+kwLen >= n || !isByteIdentifierChar(code[i+kwLen])) {
			j := i + kwLen
			j = skipSpacesAndComments(code, j)
			name, next, nameStart, nameEnd := parseIdentifier(code, j)
			if name != "" {
				return KeywordInfo{Name: name, IsType: true, Start: uint32(nameStart), End: uint32(nameEnd)}, next
			}
			return KeywordInfo{}, j
		}
	}

	return KeywordInfo{}, i
}

// ParseImportsByte parses JS/TS code and extracts all imports/exports
func ParseImportsByte(code []byte, ignoreTypeImports bool, mode ParseMode) []Import {
	imports := make([]Import, 0, 32)
	i := 0
	n := len(code)
	depth := 0 // brace depth: static import/export can only appear at depth 0

	for i < n {
		// Fast path: when inside braces (depth > 0), static import/export/declare
		// cannot appear. Only dynamic import() and require() are possible.
		// Use a tight switch-based scan instead of the full keyword detection.
		if depth > 0 {
			b := code[i]
			switch b {
			case '{':
				depth++
				i++
			case '}':
				depth--
				i++
			case '\'', '"', '`':
				i = skipToStringEnd(code, i, b)
				if i < n {
					i++ // advance past closing quote
				}
			case '/':
				if i+1 < n && code[i+1] == '/' {
					i = skipLineComment(code, i)
				} else if i+1 < n && code[i+1] == '*' {
					i = skipBlockComment(code, i)
				} else {
					i++
				}
			case 'i':
				// Check for dynamic import: import(
				if i+6 < n && code[i+1] == 'm' && code[i+2] == 'p' && code[i+3] == 'o' && code[i+4] == 'r' && code[i+5] == 't' && !isByteIdentifierChar(code[i+6]) {
					i += 6
					i = skipSpaces(code, i)
					if i < n && code[i] == '(' {
						module, next, start, end := parseExpression(code, i)
						if module != "" {
							imports = append(imports, Import{Request: module, Kind: NotTypeOrMixedImport, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end), IsDynamicImport: true})
						}
						i = next
					}
				} else {
					i++
				}
			case 'r':
				// Check for require(
				if i+7 < n && code[i+1] == 'e' && code[i+2] == 'q' && code[i+3] == 'u' && code[i+4] == 'i' && code[i+5] == 'r' && code[i+6] == 'e' && !isByteIdentifierChar(code[i+7]) {
					i += 7
					if i < n && (code[i] == '(' || skipSpaces(code, i) > i) {
						module, next, start, end := parseExpression(code, i)
						if module != "" {
							imports = append(imports, Import{Request: module, Kind: NotTypeOrMixedImport, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end), IsDynamicImport: true})
						}
						i = next
					}
				} else {
					i++
				}
			default:
				i++
			}
			continue
		}

		// Depth == 0: full keyword scanning for static import/export/declare/require

		i = skipSpaces(code, i)
		if i >= n {
			break
		}

		// skip string context
		if code[i] == '\'' {
			i = skipToStringEnd(code, i, '\'')
			if i < n {
				i++ // advance past closing quote
			}
			continue
		} else if code[i] == '"' {
			i = skipToStringEnd(code, i, '"')
			if i < n {
				i++ // advance past closing quote
			}
			continue
		} else if code[i] == '`' {
			i = skipToStringEnd(code, i, '`')
			if i < n {
				i++ // advance past closing quote
			}
			continue
		}

		// skip line comment
		if i+1 < n && code[i] == '/' && code[i+1] == '/' {
			i = skipLineComment(code, i)
			continue
		}

		// skip multi-line comment
		if i+1 < n && code[i] == '/' && code[i+1] == '*' {
			i = skipBlockComment(code, i)
			continue
		}

		// Skip declare module/global/namespace blocks — exports inside are ambient declarations
		if bytes.HasPrefix(code[i:], []byte("declare")) && (i+7 >= n || !isByteIdentifierChar(code[i+7])) {
			j := i + 7
			j = skipSpaces(code, j)
			isDeclareBlock := false
			if bytes.HasPrefix(code[j:], []byte("module")) && (j+6 >= n || !isByteIdentifierChar(code[j+6])) {
				isDeclareBlock = true
			} else if bytes.HasPrefix(code[j:], []byte("global")) && (j+6 >= n || !isByteIdentifierChar(code[j+6])) {
				isDeclareBlock = true
			} else if bytes.HasPrefix(code[j:], []byte("namespace")) && (j+9 >= n || !isByteIdentifierChar(code[j+9])) {
				isDeclareBlock = true
			}
			if isDeclareBlock {
				// Find the opening brace and skip to matching closing brace
				for j < n && code[j] != '{' {
					if j+1 < n && code[j] == '/' && code[j+1] == '/' {
						j = skipLineComment(code, j)
						continue
					}
					if j+1 < n && code[j] == '/' && code[j+1] == '*' {
						j = skipBlockComment(code, j)
						continue
					}
					j++
				}
				if j < n && code[j] == '{' {
					depth := 1
					j++
					for j < n && depth > 0 {
						if code[j] == '{' {
							depth++
						} else if code[j] == '}' {
							depth--
						} else if code[j] == '\'' || code[j] == '"' || code[j] == '`' {
							j = skipToStringEnd(code, j, code[j])
							if j < n {
								j++ // advance past closing quote
							}
							continue
						} else if j+1 < n && code[j] == '/' && code[j+1] == '/' {
							j = skipLineComment(code, j)
							continue
						} else if j+1 < n && code[j] == '/' && code[j+1] == '*' {
							j = skipBlockComment(code, j)
							continue
						}
						j++
					}
				}
				i = j
				continue
			}
		}

		// Detect keywords
		if bytes.HasPrefix(code[i:], []byte("import")) {
			i += len("import")
			if isWhiteSpace(code[i]) || code[i] == '{' || code[i] == '"' || code[i] == '\'' || code[i] == '*' || code[i] == '(' {
				i = skipSpaces(code, i)

				kind := NotTypeOrMixedImport
				isWholeStatementType := false

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
						isWholeStatementType = true
						i += len("type")
						i = skipSpaces(code, i)
					}
				}

				if i < n && (code[i] == '"' || code[i] == '\'') {
					module, next, start, end := parseStringLiteral(code, i)
					if module != "" {
						imports = append(imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end)})
					}
					i = next
				} else if i < n && code[i] == '(' {
					// dynamic import
					module, next, start, end := parseExpression(code, i)
					if module != "" {
						imports = append(imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end), IsDynamicImport: true})
					}
					i = next
				} else {
					// static import: find 'from' keyword
					var detailedKeywords *KeywordMap
					var detailedNext int

					if mode == ParseModeDetailed {
						detailedKeywords, detailedNext = parseImportKeywords(code, i, isWholeStatementType)
					}

					// Check if we have { type A, type B } case (mixed import promoted to type import)
					if kind == NotTypeOrMixedImport && code[i] == '{' {
						if areAllImportsInBracesTypes(code, i) {
							kind = OnlyTypeImport
						}
					}

					if mode == ParseModeDetailed && detailedKeywords != nil {
						// In detailed mode, we already parsed past the keywords; skip to 'from'
						i = detailedNext
						i = skipSpacesAndComments(code, i)
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
									imp := Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end)}
									if mode == ParseModeDetailed && detailedKeywords != nil && detailedKeywords.Len() > 0 {
										imp.Keywords = detailedKeywords
									}
									imports = append(imports, imp)
								}
							}
							i = next
						}
					}
				}
			}
		} else if bytes.HasPrefix(code[i:], []byte("export")) {
			exportKeyStart := i
			i += len("export")
			if isWhiteSpace(code[i]) || code[i] == '{' || code[i] == '*' {
				exportKeyEnd := i
				// Skip whitespace after `export` to find where content starts
				for exportKeyEnd < n && isWhiteSpace(code[exportKeyEnd]) {
					exportKeyEnd++
				}
				i = skipSpaces(code, i)

				kind := NotTypeOrMixedImport
				isWholeStatementType := false

				// Save position before consuming * for detailed mode
				preStarPos := i

				// detect export * as
				if i < n && code[i] == '*' {
					i++
					i = skipSpaces(code, i)
					if bytes.HasPrefix(code[i:], []byte("as")) {
						i += len("as")
						i = skipSpaces(code, i)
					}
				}

				// Check for export namespace/module — skip body (inner exports are namespace members)
				isExportNamespace := false
				if bytes.HasPrefix(code[i:], []byte("namespace")) && (i+9 >= n || !isByteIdentifierChar(code[i+9])) {
					isExportNamespace = true
				} else if bytes.HasPrefix(code[i:], []byte("module")) && (i+6 >= n || !isByteIdentifierChar(code[i+6])) {
					// Check it's `export module Foo {`, not `export ... from 'module'`
					mj := i + 6
					mj = skipSpacesAndComments(code, mj)
					if mj < n && isByteIdentifierChar(code[mj]) {
						isExportNamespace = true
					}
				}
				if isExportNamespace {
					if mode == ParseModeDetailed {
						kw, _ := parseLocalExportKeyword(code, i)
						if kw.Name != "" {
							km := &KeywordMap{Keywords: make([]KeywordInfo, 0, 1)}
							km.Add(kw)
							imports = append(imports, Import{
								Kind:            kind,
								ResolvedType:    LocalExportDeclaration,
								Keywords:        km,
								IsLocalExport:   true,
								ExportKeyStart:  uint32(exportKeyStart),
								ExportKeyEnd:    uint32(exportKeyEnd),
								ExportDeclStart: uint32(exportKeyEnd),
							})
						}
					}
					// Skip to opening brace and past matching closing brace
					for i < n && code[i] != '{' {
						i++
					}
					if i < n && code[i] == '{' {
						depth := 1
						i++
						for i < n && depth > 0 {
							if code[i] == '{' {
								depth++
							} else if code[i] == '}' {
								depth--
							} else if code[i] == '\'' || code[i] == '"' || code[i] == '`' {
								i = skipToStringEnd(code, i, code[i])
								if i < n {
									i++
								}
								continue
							} else if i+1 < n && code[i] == '/' && code[i+1] == '/' {
								i = skipLineComment(code, i)
								continue
							} else if i+1 < n && code[i] == '/' && code[i+1] == '*' {
								i = skipBlockComment(code, i)
								continue
							}
							i++
						}
					}
					continue
				}

				// Check for local export keywords that need detailed-mode handling
				isLocalExportKw := bytes.HasPrefix(code[i:], []byte("const")) ||
					bytes.HasPrefix(code[i:], []byte("function")) ||
					bytes.HasPrefix(code[i:], []byte("default")) ||
					bytes.HasPrefix(code[i:], []byte("class")) ||
					bytes.HasPrefix(code[i:], []byte("async")) ||
					bytes.HasPrefix(code[i:], []byte("let")) ||
					bytes.HasPrefix(code[i:], []byte("var")) ||
					bytes.HasPrefix(code[i:], []byte("enum")) ||
					bytes.HasPrefix(code[i:], []byte("interface"))

				if isLocalExportKw {
					if mode == ParseModeDetailed {
						kw, kwNext := parseLocalExportKeyword(code, i)
						if kw.Name != "" {
							declStart := exportKeyEnd
							if kw.Name == "default" {
								declStart = skipSpacesAndComments(code, kwNext)
							}
							km := &KeywordMap{Keywords: make([]KeywordInfo, 0, 1)}
							km.Add(kw)
							imports = append(imports, Import{
								Kind:            kind,
								ResolvedType:    LocalExportDeclaration,
								Keywords:        km,
								IsLocalExport:   true,
								ExportKeyStart:  uint32(exportKeyStart),
								ExportKeyEnd:    uint32(exportKeyEnd),
								ExportDeclStart: uint32(declStart),
							})
						}
					}
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
						isWholeStatementType = true
						i += len("type")
						i = skipSpaces(code, i)
						if !bytes.HasPrefix(code[i:], []byte("{")) {
							// `export type SomeType = ...` is a local export
							if mode == ParseModeDetailed {
								// Parse `type SomeType` - rewind to `type`
								kw, _ := parseLocalExportKeyword(code, i-5) // back to 'type' start
								if kw.Name != "" {
									km := &KeywordMap{Keywords: make([]KeywordInfo, 0, 1)}
									km.Add(kw)
									imports = append(imports, Import{
										Kind:            kind,
										ResolvedType:    LocalExportDeclaration,
										Keywords:        km,
										IsLocalExport:   true,
										ExportKeyStart:  uint32(exportKeyStart),
										ExportKeyEnd:    uint32(exportKeyEnd),
										ExportDeclStart: uint32(exportKeyEnd),
									})
								}
							}
							continue
						}
					}
				}

				// In detailed mode, parse export keywords for re-exports and local exports
				var detailedExportKeywords *KeywordMap
				var detailedBraceStart, detailedBraceEnd int
				if mode == ParseModeDetailed {
					// Star was consumed above - parse from saved position
					if preStarPos != i && code[preStarPos] == '*' {
						detailedExportKeywords, _, _, _ = parseExportKeywords(code, preStarPos, isWholeStatementType)
					} else if i < n && code[i] == '{' {
						// Brace export: check if re-export or local
						savedI := i
						detailedKw, brStart, brEnd, afterKw := parseExportKeywords(code, i, isWholeStatementType)
						checkI := skipSpacesAndComments(code, afterKw)
						if checkI < n && bytes.HasPrefix(code[checkI:], []byte("from")) && (checkI+4 >= n || !isByteIdentifierChar(code[checkI+4])) {
							// This is a re-export, save keywords and continue normal processing below
							detailedExportKeywords = detailedKw
							detailedBraceStart = brStart
							detailedBraceEnd = brEnd
							i = savedI
						} else {
							// This is a local export: `export { A, B }`
							if detailedKw != nil && detailedKw.Len() > 0 {
								stmtEnd := skipOptionalSemicolon(code, afterKw)
								imports = append(imports, Import{
									Kind:               kind,
									ResolvedType:       LocalExportDeclaration,
									Keywords:           detailedKw,
									IsLocalExport:      true,
									ExportKeyStart:     uint32(exportKeyStart),
									ExportKeyEnd:       uint32(exportKeyEnd),
									ExportDeclStart:    uint32(exportKeyEnd),
									ExportBraceStart:   uint32(brStart),
									ExportBraceEnd:     uint32(brEnd),
									ExportStatementEnd: uint32(stmtEnd),
								})
							}
							i = afterKw
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
							imp := Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end)}
							if mode == ParseModeDetailed {
								imp.ExportKeyStart = uint32(exportKeyStart)
								imp.ExportKeyEnd = uint32(exportKeyEnd)
								imp.ExportDeclStart = uint32(exportKeyEnd)
								imp.ExportBraceStart = uint32(detailedBraceStart)
								imp.ExportBraceEnd = uint32(detailedBraceEnd)
								imp.ExportStatementEnd = uint32(skipOptionalSemicolon(code, next))
								if detailedExportKeywords != nil && detailedExportKeywords.Len() > 0 {
									imp.Keywords = detailedExportKeywords
								}
							}
							imports = append(imports, imp)
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
					imports = append(imports, Import{Request: module, Kind: NotTypeOrMixedImport, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end), IsDynamicImport: true})
				}
				i = next
			}
		} else {
			// Track brace depth for non-keyword bytes at depth 0.
			// Opening braces enter depth > 0, enabling the fast scan path.
			if code[i] == '{' {
				depth++
			}
			i++
		}
	}

	return imports
}

func ParseImportsFromFiles(filePaths []string, ignoreTypeImports bool, mode ParseMode) ([]FileImports, int) {
	results := make([]FileImports, 0, len(filePaths))
	var mu sync.Mutex
	var wg sync.WaitGroup

	errCount := 0

	// Limit concurrency to avoid memory spikes
	maxConcurrency := runtime.GOMAXPROCS(0) * 2
	sem := make(chan struct{}, maxConcurrency)

	for _, filePath := range filePaths {
		wg.Add(1)
		// Acquire semaphore
		sem <- struct{}{}

		go func(path string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			// path is internal-normalized (forward slashes); convert to OS-native for file IO
			fileContent, err := os.ReadFile(DenormalizePathForOS(path))
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
				return
			}

			imports := ParseImportsByte(fileContent, ignoreTypeImports, mode)

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
