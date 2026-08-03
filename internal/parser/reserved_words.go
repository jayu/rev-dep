package parser

// reservedWords are the identifier-shaped tokens that carry meaning of their own rather
// than naming something. Structural normalisation keeps these as written and folds
// everything else to IdentSentinel, so `if`, `return` and `await` still have to line up
// while `userCount` may be `total` in the copy.
//
// The list is deliberately wider than the ECMAScript reserved list: TypeScript modifiers
// and built-in type names are shape, not naming, and treating `readonly string[]` as
// interchangeable with `foo bar[]` would match code that has nothing in common.
var reservedWords = map[string]struct{}{
	// declarations and control flow
	"var": {}, "let": {}, "const": {}, "function": {}, "class": {}, "extends": {},
	"return": {}, "if": {}, "else": {}, "for": {}, "while": {}, "do": {}, "switch": {},
	"case": {}, "default": {}, "break": {}, "continue": {}, "try": {}, "catch": {},
	"finally": {}, "throw": {}, "with": {}, "debugger": {},

	// operators and expression keywords
	"new": {}, "delete": {}, "typeof": {}, "instanceof": {}, "in": {}, "of": {},
	"void": {}, "yield": {}, "await": {}, "async": {}, "this": {}, "super": {},

	// literals
	"true": {}, "false": {}, "null": {}, "undefined": {}, "NaN": {}, "Infinity": {},

	// modules
	"import": {}, "export": {}, "from": {}, "as": {}, "require": {},

	// class and member modifiers
	"static": {}, "get": {}, "set": {}, "public": {}, "private": {}, "protected": {},
	"readonly": {}, "abstract": {}, "override": {}, "constructor": {},

	// TypeScript type-level keywords
	"interface": {}, "type": {}, "enum": {}, "namespace": {}, "module": {},
	"declare": {}, "implements": {}, "keyof": {}, "infer": {}, "satisfies": {},
	"is": {}, "asserts": {}, "out": {}, "unique": {},

	// TypeScript built-in types
	"any": {}, "unknown": {}, "never": {}, "string": {}, "number": {}, "boolean": {},
	"symbol": {}, "object": {}, "bigint": {},
}

// isReservedWord reports whether word is a keyword rather than a name. The
// map[string(bytes)] form is compiled to a lookup that does not allocate a string.
func isReservedWord(word []byte) bool {
	_, ok := reservedWords[string(word)]
	return ok
}
