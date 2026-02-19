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

func hasPrefixAt(code []byte, i int, s string) bool {
	if i < 0 || i+len(s) > len(code) {
		return false
	}
	for j := 0; j < len(s); j++ {
		if code[i+j] != s[j] {
			return false
		}
	}
	return true
}

func hasWordAt(code []byte, i int, s string) bool {
	if !hasPrefixAt(code, i, s) {
		return false
	}
	end := i + len(s)
	return end >= len(code) || !isByteIdentifierChar(code[end])
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

type parseState struct {
	code              []byte
	n                 int
	ignoreTypeImports bool
	mode              ParseMode
	imports           []Import
}

func (s *parseState) skipDeclareAmbientBlock(i int) (int, bool) {
	if !hasWordAt(s.code, i, "declare") {
		return i, false
	}

	j := i + 7
	j = skipSpaces(s.code, j)
	isDeclareBlock := false
	if hasWordAt(s.code, j, "module") || hasWordAt(s.code, j, "global") || hasWordAt(s.code, j, "namespace") {
		isDeclareBlock = true
	}
	if !isDeclareBlock {
		return i, false
	}

	// Find the opening brace and skip to matching closing brace
	for j < s.n && s.code[j] != '{' {
		if j+1 < s.n && s.code[j] == '/' && s.code[j+1] == '/' {
			j = skipLineComment(s.code, j)
			continue
		}
		if j+1 < s.n && s.code[j] == '/' && s.code[j+1] == '*' {
			j = skipBlockComment(s.code, j)
			continue
		}
		j++
	}
	if j < s.n && s.code[j] == '{' {
		depth := 1
		j++
		for j < s.n && depth > 0 {
			if s.code[j] == '{' {
				depth++
			} else if s.code[j] == '}' {
				depth--
			} else if s.code[j] == '\'' || s.code[j] == '"' || s.code[j] == '`' {
				j = skipToStringEnd(s.code, j, s.code[j])
				if j < s.n {
					j++ // advance past closing quote
				}
				continue
			} else if j+1 < s.n && s.code[j] == '/' && s.code[j+1] == '/' {
				j = skipLineComment(s.code, j)
				continue
			} else if j+1 < s.n && s.code[j] == '/' && s.code[j+1] == '*' {
				j = skipBlockComment(s.code, j)
				continue
			}
			j++
		}
	}

	return j, true
}

func (s *parseState) parseImportStatement(i int) (int, bool) {
	if !hasPrefixAt(s.code, i, "import") {
		return i, false
	}

	i += len("import")
	if i >= s.n {
		return i, true
	}
	if !(isWhiteSpace(s.code[i]) || s.code[i] == '{' || s.code[i] == '"' || s.code[i] == '\'' || s.code[i] == '*' || s.code[i] == '(') {
		return i, true
	}

	i = skipSpaces(s.code, i)
	kind := NotTypeOrMixedImport
	isWholeStatementType := false

	if bytes.HasPrefix(s.code[i:], []byte("type")) {
		isTypeKeyword := false
		if i+4 >= s.n {
			isTypeKeyword = true
		} else if !isByteIdentifierChar(s.code[i+4]) {
			isTypeKeyword = true
		}
		if isTypeKeyword {
			kind = OnlyTypeImport
			isWholeStatementType = true
			i += len("type")
			i = skipSpaces(s.code, i)
		}
	}

	if i < s.n && (s.code[i] == '"' || s.code[i] == '\'') {
		module, next, start, end := parseStringLiteral(s.code, i)
		if module != "" {
			s.imports = append(s.imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end)})
		}
		return next, true
	}
	if i < s.n && s.code[i] == '(' {
		module, next, start, end := parseExpression(s.code, i)
		if module != "" {
			s.imports = append(s.imports, Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end), IsDynamicImport: true})
		}
		return next, true
	}

	var detailedKeywords *KeywordMap
	var detailedNext int
	if s.mode == ParseModeDetailed {
		detailedKeywords, detailedNext = parseImportKeywords(s.code, i, isWholeStatementType)
	}

	if kind == NotTypeOrMixedImport && s.code[i] == '{' {
		if areAllImportsInBracesTypes(s.code, i) {
			kind = OnlyTypeImport
		}
	}
	if s.mode == ParseModeDetailed && detailedKeywords != nil {
		i = detailedNext
		i = skipSpacesAndComments(s.code, i)
	}

	foundFrom := false
	scanStart := i
	for i < s.n {
		if hasWordAt(s.code, i, "from") {
			foundFrom = true
			break
		}
		if i+1 < s.n && s.code[i] == '/' && s.code[i+1] == '/' {
			i = skipLineComment(s.code, i)
			continue
		}
		if i+1 < s.n && s.code[i] == '/' && s.code[i+1] == '*' {
			i = skipBlockComment(s.code, i)
			continue
		}
		i++
	}

	if foundFrom {
		i += len("from")
		i = skipSpaces(s.code, i)
		if i < s.n && (s.code[i] == '"' || s.code[i] == '\'') {
			module, next, start, end := parseStringLiteral(s.code, i)
			if module != "" && (!s.ignoreTypeImports || kind == NotTypeOrMixedImport) {
				imp := Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end)}
				if s.mode == ParseModeDetailed && detailedKeywords != nil && detailedKeywords.Len() > 0 {
					imp.Keywords = detailedKeywords
				}
				s.imports = append(s.imports, imp)
			}
			return next, true
		}
		return i, true
	}

	// Fallback for malformed static imports like `import X form "mod"`.
	j := scanStart
	for j < s.n {
		if j+1 < s.n && s.code[j] == '/' && s.code[j+1] == '/' {
			j = skipLineComment(s.code, j)
			continue
		}
		if j+1 < s.n && s.code[j] == '/' && s.code[j+1] == '*' {
			j = skipBlockComment(s.code, j)
			continue
		}
		if s.code[j] == ';' || s.code[j] == '\n' || s.code[j] == '\r' {
			break
		}
		if s.code[j] == '"' || s.code[j] == '\'' {
			module, next, start, end := parseStringLiteral(s.code, j)
			if module != "" {
				imp := Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end)}
				if s.mode == ParseModeDetailed && detailedKeywords != nil && detailedKeywords.Len() > 0 {
					imp.Keywords = detailedKeywords
				}
				s.imports = append(s.imports, imp)
			}
			j = next
			break
		}
		j++
	}
	return j, true
}

func (s *parseState) parseRequireStatement(i int) (int, bool) {
	if !hasPrefixAt(s.code, i, "require") {
		return i, false
	}
	i += len("require")
	if i < s.n && (bytes.HasPrefix(s.code[i:], []byte("(")) || skipSpaces(s.code, i) > i) {
		module, next, start, end := parseExpression(s.code, i)
		if module != "" {
			s.imports = append(s.imports, Import{Request: module, Kind: NotTypeOrMixedImport, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end), IsDynamicImport: true})
		}
		return next, true
	}
	return i, true
}

func (s *parseState) parseExportStatement(i int) (int, bool) {
	if !hasPrefixAt(s.code, i, "export") {
		return i, false
	}

	exportKeyStart := i
	i += len("export")
	if i >= s.n || !(isWhiteSpace(s.code[i]) || s.code[i] == '{' || s.code[i] == '*') {
		return i, true
	}

	exportKeyEnd := i
	// Skip whitespace after `export` to find where content starts
	for exportKeyEnd < s.n && isWhiteSpace(s.code[exportKeyEnd]) {
		exportKeyEnd++
	}
	i = skipSpaces(s.code, i)

	kind := NotTypeOrMixedImport
	isWholeStatementType := false

	// Save position before consuming * for detailed mode
	preStarPos := i

	// detect export * as
	if i < s.n && s.code[i] == '*' {
		i++
		i = skipSpaces(s.code, i)
		if bytes.HasPrefix(s.code[i:], []byte("as")) {
			i += len("as")
			i = skipSpaces(s.code, i)
		}
	}

	// Check for export namespace/module — skip body (inner exports are namespace members)
	isExportNamespace := false
	if hasWordAt(s.code, i, "namespace") {
		isExportNamespace = true
	} else if hasWordAt(s.code, i, "module") {
		// Check it's `export module Foo {`, not `export ... from 'module'`
		mj := i + 6
		mj = skipSpacesAndComments(s.code, mj)
		if mj < s.n && isByteIdentifierChar(s.code[mj]) {
			isExportNamespace = true
		}
	}
	if isExportNamespace {
		if s.mode == ParseModeDetailed {
			kw, _ := parseLocalExportKeyword(s.code, i)
			if kw.Name != "" {
				km := &KeywordMap{Keywords: make([]KeywordInfo, 0, 1)}
				km.Add(kw)
				s.imports = append(s.imports, Import{
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
		for i < s.n && s.code[i] != '{' {
			i++
		}
		if i < s.n && s.code[i] == '{' {
			depth := 1
			i++
			for i < s.n && depth > 0 {
				if s.code[i] == '{' {
					depth++
				} else if s.code[i] == '}' {
					depth--
				} else if s.code[i] == '\'' || s.code[i] == '"' || s.code[i] == '`' {
					i = skipToStringEnd(s.code, i, s.code[i])
					if i < s.n {
						i++
					}
					continue
				} else if i+1 < s.n && s.code[i] == '/' && s.code[i+1] == '/' {
					i = skipLineComment(s.code, i)
					continue
				} else if i+1 < s.n && s.code[i] == '/' && s.code[i+1] == '*' {
					i = skipBlockComment(s.code, i)
					continue
				}
				i++
			}
		}
		return i, true
	}

	// Check for local export keywords that need detailed-mode handling
	isLocalExportKw := bytes.HasPrefix(s.code[i:], []byte("const")) ||
		bytes.HasPrefix(s.code[i:], []byte("function")) ||
		bytes.HasPrefix(s.code[i:], []byte("default")) ||
		bytes.HasPrefix(s.code[i:], []byte("class")) ||
		bytes.HasPrefix(s.code[i:], []byte("async")) ||
		bytes.HasPrefix(s.code[i:], []byte("let")) ||
		bytes.HasPrefix(s.code[i:], []byte("var")) ||
		bytes.HasPrefix(s.code[i:], []byte("enum")) ||
		bytes.HasPrefix(s.code[i:], []byte("interface"))

	if isLocalExportKw {
		if s.mode == ParseModeDetailed {
			kw, kwNext := parseLocalExportKeyword(s.code, i)
			if kw.Name != "" {
				declStart := exportKeyEnd
				if kw.Name == "default" {
					declStart = skipSpacesAndComments(s.code, kwNext)
				}
				km := &KeywordMap{Keywords: make([]KeywordInfo, 0, 1)}
				km.Add(kw)
				s.imports = append(s.imports, Import{
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
		return i, true
	}

	// drop processing export statement if followed by "type".
	if bytes.HasPrefix(s.code[i:], []byte("type")) {
		isTypeKeyword := false
		if i+4 >= s.n {
			isTypeKeyword = true
		} else if !isByteIdentifierChar(s.code[i+4]) {
			isTypeKeyword = true
		}

		if isTypeKeyword {
			kind = OnlyTypeImport
			isWholeStatementType = true
			i += len("type")
			i = skipSpaces(s.code, i)
			if !bytes.HasPrefix(s.code[i:], []byte("{")) && !bytes.HasPrefix(s.code[i:], []byte("*")) {
				// `export type SomeType = ...` is a local export
				if s.mode == ParseModeDetailed {
					emitLocalTypeExport := true
					stmtEnd := i
					braceDepth, parenDepth, bracketDepth := 0, 0, 0
					for stmtEnd < s.n {
						if stmtEnd+1 < s.n && s.code[stmtEnd] == '/' && s.code[stmtEnd+1] == '/' {
							stmtEnd = skipLineComment(s.code, stmtEnd)
							continue
						}
						if stmtEnd+1 < s.n && s.code[stmtEnd] == '/' && s.code[stmtEnd+1] == '*' {
							stmtEnd = skipBlockComment(s.code, stmtEnd)
							continue
						}
						if s.code[stmtEnd] == '\'' || s.code[stmtEnd] == '"' || s.code[stmtEnd] == '`' {
							stmtEnd = skipToStringEnd(s.code, stmtEnd, s.code[stmtEnd])
							if stmtEnd < s.n {
								stmtEnd++
							}
							continue
						}

						if s.code[stmtEnd] == '{' {
							braceDepth++
						} else if s.code[stmtEnd] == '}' && braceDepth > 0 {
							braceDepth--
						} else if s.code[stmtEnd] == '(' {
							parenDepth++
						} else if s.code[stmtEnd] == ')' && parenDepth > 0 {
							parenDepth--
						} else if s.code[stmtEnd] == '[' {
							bracketDepth++
						} else if s.code[stmtEnd] == ']' && bracketDepth > 0 {
							bracketDepth--
						}

						if braceDepth == 0 && parenDepth == 0 && bracketDepth == 0 {
							if s.code[stmtEnd] == ';' {
								stmtEnd++
								break
							}
							if s.code[stmtEnd] == '\n' || s.code[stmtEnd] == '\r' {
								break
							}
						}

						stmtEnd++
					}

					nextTopLevel := skipSpacesAndComments(s.code, stmtEnd)
					if nextTopLevel < s.n &&
						hasWordAt(s.code, nextTopLevel, "import") {
						emitLocalTypeExport = false
					}

					// Parse `type SomeType` - rewind to `type`
					kw, _ := parseLocalExportKeyword(s.code, i-5) // back to 'type' start
					if emitLocalTypeExport && kw.Name != "" {
						km := &KeywordMap{Keywords: make([]KeywordInfo, 0, 1)}
						km.Add(kw)
						s.imports = append(s.imports, Import{
							Kind:            kind,
							ResolvedType:    LocalExportDeclaration,
							Keywords:        km,
							IsLocalExport:   true,
							ExportKeyStart:  uint32(exportKeyStart),
							ExportKeyEnd:    uint32(exportKeyEnd),
							ExportDeclStart: uint32(exportKeyEnd),
						})
					}
					i = stmtEnd
				}
				return i, true
			}
		}
	}

	// In detailed mode, parse export keywords for re-exports and local exports
	var detailedExportKeywords *KeywordMap
	var detailedBraceStart, detailedBraceEnd int
	if s.mode == ParseModeDetailed {
		// Star was consumed above - parse from saved position
		if preStarPos != i && s.code[preStarPos] == '*' {
			detailedExportKeywords, _, _, _ = parseExportKeywords(s.code, preStarPos, isWholeStatementType)
		} else if i < s.n && s.code[i] == '{' {
			// Brace export: check if re-export or local
			savedI := i
			detailedKw, brStart, brEnd, afterKw := parseExportKeywords(s.code, i, isWholeStatementType)
			checkI := skipSpacesAndComments(s.code, afterKw)
			if hasWordAt(s.code, checkI, "from") {
				// This is a re-export, save keywords and continue normal processing below
				detailedExportKeywords = detailedKw
				detailedBraceStart = brStart
				detailedBraceEnd = brEnd
				i = savedI
			} else {
				// This is a local export: `export { A, B }`.
				// Skip `export type { ... }` local declarations in detailed mode.
				if !isWholeStatementType && detailedKw != nil && detailedKw.Len() > 0 {
					stmtEnd := skipOptionalSemicolon(s.code, afterKw)
					s.imports = append(s.imports, Import{
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
				return afterKw, true
			}
		}
	}

	// Check if we have { type A } case in export
	if kind == NotTypeOrMixedImport && s.code[i] == '{' {
		if areAllImportsInBracesTypes(s.code, i) {
			kind = OnlyTypeImport
		}
	}

	shouldDropLookingForFrom := false
	foundFrom := false
	for i < s.n && !shouldDropLookingForFrom {
		if hasWordAt(s.code, i, "from") {
			foundFrom = true
			break
		}
		if i+1 < s.n && s.code[i] == '/' && s.code[i+1] == '/' {
			i = skipLineComment(s.code, i)
			continue
		}
		if i+1 < s.n && s.code[i] == '/' && s.code[i+1] == '*' {
			i = skipBlockComment(s.code, i)
			continue
		}
		if hasWordAt(s.code, i, "import") || hasWordAt(s.code, i, "export") || hasWordAt(s.code, i, "require") {
			shouldDropLookingForFrom = true
			break
		}
		i++
	}

	if shouldDropLookingForFrom {
		return i, true
	}

	if foundFrom {
		i += len("from")
		i = skipSpaces(s.code, i)
		if i < s.n && (s.code[i] == '"' || s.code[i] == '\'') {
			module, next, start, end := parseStringLiteral(s.code, i)
			if module != "" {
				imp := Import{Request: module, Kind: kind, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end)}
				if s.mode == ParseModeDetailed {
					imp.ExportKeyStart = uint32(exportKeyStart)
					imp.ExportKeyEnd = uint32(exportKeyEnd)
					imp.ExportDeclStart = uint32(exportKeyEnd)
					imp.ExportBraceStart = uint32(detailedBraceStart)
					imp.ExportBraceEnd = uint32(detailedBraceEnd)
					imp.ExportStatementEnd = uint32(skipOptionalSemicolon(s.code, next))
					if detailedExportKeywords != nil && detailedExportKeywords.Len() > 0 {
						imp.Keywords = detailedExportKeywords
					}
				}
				s.imports = append(s.imports, imp)
			}
			return next, true
		}
	}

	return i, true
}

// ParseImportsByte parses JS/TS code and extracts all imports/exports
func ParseImportsByte(code []byte, ignoreTypeImports bool, mode ParseMode) []Import {
	state := parseState{
		code:              code,
		n:                 len(code),
		ignoreTypeImports: ignoreTypeImports,
		mode:              mode,
		imports:           make([]Import, 0, 32),
	}
	i := 0
	n := state.n
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
							state.imports = append(state.imports, Import{Request: module, Kind: NotTypeOrMixedImport, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end), IsDynamicImport: true})
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
							state.imports = append(state.imports, Import{Request: module, Kind: NotTypeOrMixedImport, ResolvedType: NotResolvedModule, RequestStart: uint32(start), RequestEnd: uint32(end), IsDynamicImport: true})
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

		switch code[i] {
		case 'd':
			if next, ok := state.skipDeclareAmbientBlock(i); ok {
				i = next
				continue
			}
		case 'i':
			if next, ok := state.parseImportStatement(i); ok {
				i = next
				continue
			}
		case 'e':
			if next, ok := state.parseExportStatement(i); ok {
				i = next
				continue
			}
		case 'r':
			if next, ok := state.parseRequireStatement(i); ok {
				i = next
				continue
			}
		}

		// Track brace depth for non-keyword bytes at depth 0.
		// Opening braces enter depth > 0, enabling the fast scan path.
		if code[i] == '{' {
			depth++
		}
		i++
	}

	return state.imports
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
