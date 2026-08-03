package parser

import "strings"

// Block scanning extends the character-loop parser with a second thing it can report: where
// every copy-pasteable chunk of code starts and ends.
//
// Normalisation and boundary detection happen in ONE pass. As the loop walks the source it
// writes a canonical form into a buffer - comments gone, whitespace collapsed, optionally
// identifiers folded - and records the canonical and original offsets of every `{...}` block
// and JSX element it closes.
//
// Doing both at once is what makes duplicate detection cheap: the canonical buffer is exactly
// the byte sequence two chunks must share to be duplicates, so a substring hash over it answers
// "could these match" without re-reading the source, and a block nested five levels deep costs
// nothing extra.

// Blinding is what a comparison ignores. It is applied DURING canonicalisation, not afterwards:
// a blinded token is replaced by a sentinel byte as the scanner passes it, so two blindings
// produce two different canonical buffers and two different hashes.
//
// Comments and insignificant whitespace always go. Beyond that each axis independently collapses
// one category of token, so the caller picks how much variation counts as the same code rather
// than choosing from a fixed set of modes. The zero value blinds nothing.
type Blinding struct {
	// Identifiers folds every non-keyword name - variables, functions, properties, type names,
	// JSX tags - so a renamed copy still matches.
	Identifiers bool
	Strings     bool
	Numbers     bool
}

func (b Blinding) IsExact() bool { return b == Blinding{} }

func (b Blinding) String() string {
	if b.IsExact() {
		return "exact"
	}
	parts := make([]string, 0, 3)
	if b.Identifiers {
		parts = append(parts, "identifiers")
	}
	if b.Strings {
		parts = append(parts, "strings")
	}
	if b.Numbers {
		parts = append(parts, "numbers")
	}
	return strings.Join(parts, "+")
}

// One sentinel per category: sharing one would make `f(name)`, `f("text")` and `f(1)` identical,
// and an argument being a name rather than a number is structure, not naming. All are outside
// the range of any source character.
const (
	IdentSentinel  = 0x01
	StringSentinel = 0x02
	NumberSentinel = 0x03
)

type BlockKind uint8

const (
	BlockBrace BlockKind = iota
	BlockJSX
)

// CodeBlock is one chunk of code that could have been copy-pasted somewhere else.
//
// Norm* index the canonical buffer and are what comparison and hashing use. Start/End are
// offsets in the original source, kept so the block can be pointed at and printed.
type CodeBlock struct {
	NormStart uint32
	NormEnd   uint32
	// AltNorm* locate the same block in BlockScan.NormAlt, filled only when both canonical forms
	// were asked for.
	AltNormStart uint32
	AltNormEnd   uint32
	Start        uint32
	End          uint32
	Kind         BlockKind
	// Depth is how many levels the block spans, counting itself: 1 with nothing nested inside,
	// 2 for `{ a: { b: 1 } }`. Unlike a character count it does not move when keys or string
	// values get long, which is what tells a trivial literal from real structure.
	Depth uint8
	// Role is meaningless for BlockJSX.
	Role BraceRole
	// Tokens is the size measure the floors use: unlike characters it does not move under blinding.
	Tokens uint32
	// Statements is meaningful only for a STATEMENT block. An object literal, type literal, class
	// body or JSX element is an expression or member list, so they report 0 rather than a count of
	// their entries.
	Statements uint16
}

func (b CodeBlock) IsStatementBlock() bool { return b.Role == BraceStatementList }

// IsObjectLiteral is the distinction --skip-objects needs: a brace in expression position, or a
// type literal, which is the same shape written in a type. A nested object is structurally
// repetitive by nature, so under structural comparison it produces matches nobody would call
// duplication.
func (b CodeBlock) IsObjectLiteral() bool {
	return b.Kind == BlockBrace && b.Role == BraceObjectLiteral
}

type BraceRole uint8

const (
	// BraceStatementList is the only kind that contains statements.
	BraceStatementList BraceRole = iota
	// BraceDeclarationBody holds members rather than statements, but a duplicated one is real
	// duplication.
	BraceDeclarationBody
	// BraceObjectLiteral is a brace in expression position, or a JSX expression container.
	BraceObjectLiteral
)

// BlockScan holds one file's scan result. Norm and the slices are reused across files by the
// caller, so a worker allocates once and not per file.
type BlockScan struct {
	Norm []byte
	// NormAlt is the structural canonical form, from the SAME traversal as Norm: the scanner knows
	// which byte runs are foldable identifiers as it passes them, so producing both costs a second
	// buffer rather than a second parse.
	NormAlt []byte
	Blocks  []CodeBlock
	// Imports holds exactly what ParseImportsByte would have returned for the same input.
	Imports []Import

	stack []blockFrame
}

// BlockScanOptions configures ScanCodeBlocks.
type BlockScanOptions struct {
	Blinding Blinding
	// AllowJSX must be off for .ts/.mts, where `<T>` is a type argument or assertion.
	AllowJSX bool
	// MinTokens filters here rather than in the caller, keeping the block slice small - a deeply
	// nested file produces a block per level. Tokens rather than characters: the count does not
	// move under blinding, so one floor means the same amount of code whatever is ignored.
	MinTokens int
	// See CodeBlock.Depth.
	MinDepth int
	// MinStatements does not apply to expressions - object literals, JSX - which pass through
	// whatever its value. See CodeBlock.Statements.
	MinStatements int
	// See CodeBlock.IsObjectLiteral.
	SkipObjectLiterals bool
	// AltIdentifiers builds a SECOND canonical form into NormAlt from the same traversal, identical
	// to the primary except that identifiers are blinded. Two comparisons then cost one parse.
	//
	// Only this axis is available: the alternate has to be a COARSENING of the primary, produced by
	// collapsing runs the primary emitted, never by expanding a byte it already collapsed. Any other
	// pair of blindings costs a pass each. MinTokens serves both.
	AltIdentifiers bool

	// SkipBlocks leaves only import collection, so a caller wanting imports alone does not pay for
	// the canonical buffer.
	SkipBlocks bool
	// CollectImports is the point of the unified scan: a caller needing both gets them from one
	// traversal.
	CollectImports    bool
	IgnoreTypeImports bool
	ImportMode        ParseMode
}

// ---------------------------------------------------------------------------
// normalising scanner
// ---------------------------------------------------------------------------

// tokenClass tells the block state machine whether a byte is code to interpret, or opaque
// payload from inside a string or template that it must not read braces out of.
type tokenClass uint8

const (
	tokCode tokenClass = iota
	tokText
)

const (
	stCode uint8 = iota
	stTemplate
)

type normScanner struct {
	code     []byte
	i, n     int
	blinding Blinding
	// altIdentifiers applies only where the primary kept a name as written.
	altIdentifiers bool
	// skim trades byte-level output for speed. The rules are identical - the same state machine
	// decides what is a string, comment, regex or identifier - but each is reported as one
	// representative byte and the cursor jumps to its end.
	//
	// A caller that only wants imports needs to know WHERE tokens begin, not what they
	// canonicalise to, and paying a call per byte for that was three to four times slower.
	skim bool

	st      uint8
	escaped bool

	// rawEnd/rawClass emit an already-decided run verbatim, one byte per next() call - regex
	// literals, and identifiers kept as written.
	rawEnd   int
	rawClass tokenClass
	// rawSentinel is what the ALTERNATE form collapses this run to, or zero if it keeps the run.
	// rawStart is where the run began, so the first byte carries the sentinel and the rest are
	// dropped from the alternate stream.
	rawSentinel byte
	rawStart    int

	// altByte/altSkip describe how the byte just returned appears in the structural form: usually
	// itself, once as IdentSentinel for a folded identifier, absent for the rest of that run. They
	// are what lets one pass produce both canonical forms.
	//
	// next() defaults them to "the byte itself" so no emit path has to remember; altSet is how the
	// one path that differs opts out.
	altByte byte
	altSkip bool
	altSet  bool

	// tokStart marks the first byte of a token, which is how blocks count tokens. False for
	// separators, for continuation bytes, and inside template text - so the count is the same
	// whatever is blinded, because blinding swaps one token for another rather than removing it.
	tokStart bool

	last byte // last byte emitted, which decides whether a separator is needed

	// braceDepth and tmplReturn exist only to find the `}` closing a `${`.
	braceDepth int
	tmplReturn []int
}

// beginRawRun emits [s.i, end) verbatim, one byte per next() call, and settles how they appear
// in the structural form.
//
// Every site that wants a verbatim run goes through here. Setting rawEnd directly produced a bug
// where a `${` inherited foldability from the identifier before it and lost its brace from the
// structural stream - a corrupt second canonical form with no crash to point at it.
func (s *normScanner) beginRawRun(end int, class tokenClass, sentinel byte) {
	s.rawEnd = end
	s.rawClass = class
	s.rawSentinel = sentinel
	s.rawStart = s.i
}

// isTagAbortByte: seeing one means the '<' that opened the tag was really a comparison or a
// TypeScript type argument.
func isTagAbortByte(c byte) bool {
	switch c {
	case ',', ';', '(', ')', '[', ']', '}', '?', '&', '|', '+', '*', '%', '!', '~', '^':
		return true
	}
	return false
}

var statementBlockKeywords = map[string]struct{}{
	"else": {}, "do": {}, "try": {}, "finally": {},
}

var declarationKeywords = map[string]struct{}{
	"class": {}, "interface": {}, "enum": {}, "namespace": {}, "module": {},
}

// braceRoleAt decides what a `{` is from the token in front of it.
//
// After `)` it closes a header - `if (...)`, `function f(...)`. After `;`, `{`, `}` or nothing
// it sits where a statement may start. After `=>` it is an arrow body, since an arrow returning
// an object needs parentheses. Everything else - `=`, `(`, `,`, `:`, `[`, an operator, or a
// plain identifier as in `class A {` - is expression or member-list position.
func braceRoleAt(norm []byte, bracePos int, prev, prev2 byte, word []byte) BraceRole {
	switch {
	case prev == 0:
		return BraceStatementList
	case prev == ')' || prev == ';' || prev == '{' || prev == '}':
		return BraceStatementList
	case prev == '>' && prev2 == '=':
		return BraceStatementList // an arrow body; one returning an object needs parens
	}
	if identLikeByte(prev) {
		if _, ok := statementBlockKeywords[string(word)]; ok {
			return BraceStatementList
		}
	}
	// A TypeScript return type sits between the parameter list and the body, so the byte before the
	// brace ends a type rather than being the `)` that would answer: `function f(): Promise<User> {`.
	// The same walk correctly refuses an object entry - `a: { b: 1 }` reaches the `{` or `,` that
	// introduced the key.
	kind, found := walkBackFromBrace(norm, bracePos)
	if found {
		return kind
	}
	return BraceObjectLiteral
}

// walkBackFromBrace steps back over a return type, heritage clause or name to find what
// introduced the brace. Bounded: a long enough run of type-ish characters is not a signature,
// and leaving it unbounded would make a pathological file quadratic.
func walkBackFromBrace(norm []byte, bracePos int) (BraceRole, bool) {
	const maxLookback = 256
	limit := bracePos - maxLookback
	if limit < 0 {
		limit = 0
	}

	wordEnd := -1
	for i := bracePos - 1; i >= limit; i-- {
		c := norm[i]
		if identLikeByte(c) {
			if wordEnd < 0 {
				wordEnd = i + 1
			}
			continue
		}
		// An identifier run just ended: `class`, `interface` and friends settle it.
		if wordEnd >= 0 {
			if _, ok := declarationKeywords[string(norm[i+1:wordEnd])]; ok {
				return BraceDeclarationBody, true
			}
			wordEnd = -1
		}
		if c == ')' {
			return BraceStatementList, true
		}
		if !typeAnnotationByte(c) {
			return BraceObjectLiteral, true
		}
	}
	if wordEnd >= 0 {
		if _, ok := declarationKeywords[string(norm[limit:wordEnd])]; ok {
			return BraceDeclarationBody, true
		}
	}
	return BraceObjectLiteral, false
}

// typeAnnotationByte is what the walk above steps over.
func typeAnnotationByte(c byte) bool {
	if identLikeByte(c) {
		return true
	}
	switch c {
	case '.', '<', '>', '[', ']', '|', '&', ':', ',', ' ', '\'', '"':
		return true
	}
	return false
}

func identLikeByte(c byte) bool {
	return isByteIdentifierChar(c) || c == IdentSentinel
}

// needsSeparator: `const x` must keep its space, `} else` must not lose it, `a + +b` must not
// become `a++b`.
func needsSeparator(prev, next byte) bool {
	if prev == 0 {
		return false
	}
	if identLikeByte(prev) && identLikeByte(next) {
		return true
	}
	if (prev == '+' && next == '+') || (prev == '-' && next == '-') {
		return true
	}
	return false
}

// next returns the next canonical byte, its offset in the original source, and its class.
//
// It also settles how that byte appears in the structural form. Doing it here rather than at
// each of the dozen return sites in nextToken is not tidiness: a site that forgot would emit
// whatever the previous one left behind - a corrupt second canonical form, not a crash.
func (s *normScanner) next() (byte, int, tokenClass, bool) {
	s.altSet = false
	s.tokStart = true
	inTemplate := s.st == stTemplate
	b, orig, class, ok := s.nextToken()
	if ok && !s.altSet {
		s.altByte, s.altSkip = b, false
	}
	// Template text is one literal however many bytes it takes, so only the opening backtick
	// counts.
	if inTemplate && s.st == stTemplate {
		s.tokStart = false
	}
	return b, orig, class, ok
}

func (s *normScanner) nextToken() (byte, int, tokenClass, bool) {
	for {
		if s.i < s.rawEnd {
			c := s.code[s.i]
			p := s.i
			s.tokStart = p == s.rawStart
			if s.rawSentinel != 0 {
				// One sentinel stands for the whole run in the alternate form: the
				// first byte carries it, the rest are absent. altSet opts out of the
				// default the wrapper would otherwise apply.
				s.altByte, s.altSkip, s.altSet = s.rawSentinel, p != s.rawStart, true
			}
			s.i++
			s.last = c
			return c, p, s.rawClass, true
		}

		switch s.st {
		case stCode:
			// Collapse whitespace and comments. Comments are dropped outright; the run leaves a single
			// space only where it separates two tokens that would otherwise merge.
			j := s.i
			for j < s.n {
				c := s.code[j]
				if isWhiteSpace(c) {
					j++
					continue
				}
				if c == '/' && j+1 < s.n && s.code[j+1] == '/' {
					j = skipLineComment(s.code, j)
					continue
				}
				if c == '/' && j+1 < s.n && s.code[j+1] == '*' {
					j = skipBlockComment(s.code, j)
					continue
				}
				break
			}
			if j > s.i {
				p := s.i
				s.i = j
				if !s.skim && j < s.n && needsSeparator(s.last, s.code[j]) {
					s.last = ' '
					return ' ', p, tokCode, true
				}
				continue
			}
			if s.i >= s.n {
				return 0, 0, tokCode, false
			}

			c := s.code[s.i]
			switch {
			case c == '\'' || c == '"':
				p := s.i
				end := skipToStringEnd(s.code, s.i, c)
				if end < s.n {
					end++
				}
				// Blinded or skimmed, the literal collapses to one byte; otherwise it is emitted verbatim as
				// one run. Either way it is one token.
				if s.skim || s.blinding.Strings {
					s.i = end
					out := c
					if s.blinding.Strings {
						out = StringSentinel
					}
					s.last = out
					return out, p, tokText, true
				}
				s.beginRawRun(end, tokText, 0)
				continue

			case c == '`':
				p := s.i
				if s.skim {
					s.i = skipTemplateLiteral(s.code, s.i)
					s.last = c
					return c, p, tokText, true
				}
				s.st = stTemplate
				s.escaped = false
				s.i++
				s.last = c
				return c, p, tokText, true

			case c == '{':
				s.braceDepth++
				p := s.i
				s.i++
				s.last = c
				return c, p, tokCode, true

			case c == '}':
				p := s.i
				s.i++
				s.last = c
				// A `}` closing a `${` is template punctuation, not a block.
				if len(s.tmplReturn) > 0 && s.braceDepth-1 == s.tmplReturn[len(s.tmplReturn)-1] {
					s.tmplReturn = s.tmplReturn[:len(s.tmplReturn)-1]
					s.braceDepth--
					s.st = stTemplate
					return c, p, tokText, true
				}
				s.braceDepth--
				return c, p, tokCode, true

			// `</` closing a JSX element is handled inside regexLiteralAllowedBefore, which
			// refuses a regex after '<'. It has to be settled there rather than here: this
			// scanner emits a run of punctuation in one step when it is skimming, so the '<'
			// may already be behind the cursor by the time the '/' is reached, and a guard on
			// the previously emitted byte would silently stop firing in that mode.
			case c == '/' && regexLiteralAllowedBefore(s.code, s.i):
				// A regex body is opaque: its slashes, braces and quotes must not reach the block state
				// machine.
				if end, ok := skipRegexLiteral(s.code, s.i); ok {
					if s.skim {
						p := s.i
						s.i = end
						s.last = c
						return c, p, tokText, true
					}
					s.beginRawRun(end, tokText, 0)
					continue
				}
				p := s.i
				s.i++
				s.last = c
				return c, p, tokCode, true

			case isByteIdentifierChar(c):
				start := s.i
				j := start
				for j < s.n && isByteIdentifierChar(s.code[j]) {
					j++
				}
				if s.skim {
					s.i = j
					s.last = c
					return c, start, tokCode, true
				}
				// A run starting with a digit is a number, not a name, so the two axes decide it separately.
				isNumber := c >= '0' && c <= '9'
				// Keywords are never folded - but only where they could BE keywords. Straight after `.` a run
				// is a property name, and no spelling makes that a keyword. Without the position check,
				// `.keys` folded and `.from` did not, so `Object.keys(a)` matched `Foo.bar(b)` and
				// `Array.from(a)` did not.
				name := !isNumber && (s.last == '.' || !isReservedWord(s.code[start:j]))

				sentinel := byte(0)
				switch {
				case isNumber:
					if s.blinding.Numbers {
						sentinel = NumberSentinel
					}
				case name:
					if s.blinding.Identifiers {
						sentinel = IdentSentinel
					}
				}
				if sentinel != 0 {
					s.i = j
					s.last = sentinel
					return sentinel, start, tokCode, true
				}
				// Kept as written here; the alternate form may still fold it.
				alt := byte(0)
				if s.altIdentifiers && name {
					alt = IdentSentinel
				}
				s.beginRawRun(j, tokCode, alt)
				continue

			default:
				p := s.i
				s.i++
				if s.skim {
					// A skimming caller needs identifier starts, brace depth and literal boundaries; operators
					// and punctuation can go by in a run rather than a call each.
					for s.i < s.n && !skimInteresting[s.code[s.i]] {
						s.i++
					}
				}
				s.last = c
				return c, p, tokCode, true
			}

		case stTemplate:
			if s.i >= s.n {
				s.st = stCode
				continue
			}
			c := s.code[s.i]
			if s.escaped {
				p := s.i
				s.i++
				s.escaped = false
				s.last = c
				return c, p, tokText, true
			}
			switch {
			case c == '\\':
				p := s.i
				s.i++
				s.escaped = true
				s.last = c
				return c, p, tokText, true

			case c == '`':
				p := s.i
				s.i++
				s.st = stCode
				s.last = c
				return c, p, tokText, true

			case c == '$' && s.i+1 < s.n && s.code[s.i+1] == '{':
				// Interpolations hold real code. The `${` and its `}` stay text so the block state machine does
				// not see a brace pair that is not one.
				p := s.i
				s.i++
				s.beginRawRun(s.i+1, tokText, 0)
				s.tmplReturn = append(s.tmplReturn, s.braceDepth)
				s.braceDepth++
				s.st = stCode
				s.last = '$'
				return '$', p, tokText, true

			case isWhiteSpace(c):
				// Template whitespace collapses to one space, so a re-indented styled-component or GraphQL
				// document still matches.
				p := s.i
				s.i = skipSpaces(s.code, s.i)
				s.last = ' '
				return ' ', p, tokText, true

			default:
				p := s.i
				if s.blinding.Strings {
					// Collapse the run of literal text to one sentinel, stopping at anything that ends it: an
					// interpolation still holds real code.
					for s.i < s.n {
						d := s.code[s.i]
						if d == '`' || d == '\\' || isWhiteSpace(d) ||
							(d == '$' && s.i+1 < s.n && s.code[s.i+1] == '{') {
							break
						}
						s.i++
					}
					s.last = StringSentinel
					return StringSentinel, p, tokText, true
				}
				s.i++
				s.last = c
				return c, p, tokText, true
			}
		}
	}
}

// NormIterator yields the canonical bytes of source under a blinding, applying exactly the
// rules ScanCodeBlocks uses to fill BlockScan.Norm.
//
// It exists so comparing two raw chunks and hashing a scanned block cannot drift apart: both go
// through this state machine, so "these normalise identically" means the same to both.
type NormIterator struct {
	s normScanner
}

func NewNormIterator(code []byte, blinding Blinding) NormIterator {
	return NormIterator{s: normScanner{
		code:     code,
		n:        len(code),
		blinding: blinding,
	}}
}

// Next calls nextToken directly rather than next: the alternate-form bookkeeping exists for the
// dual-mode scan, and nobody iterating this way reads it. It also removes a call level from the
// hottest loop in matching, which walks two of these in lockstep per candidate pair.
func (it *NormIterator) Next() (b byte, orig int, ok bool) {
	b, orig, _, ok = it.s.nextToken()
	return b, orig, ok
}

// ---------------------------------------------------------------------------
// block state machine
// ---------------------------------------------------------------------------

const (
	fBrace uint8 = iota
	fJSXTag
	fJSXChildren
	fJSXExpr
)

type blockFrame struct {
	kind         uint8
	normStart    uint32
	altNormStart uint32
	origStart    uint32

	// tokenStart makes the frame's token count a subtraction at close rather than a rescan.
	tokenStart uint32
	// maxSeen is carried upward on every pop, which makes nesting depth O(1) per block.
	maxSeen uint8
	// See CodeBlock.Statements.
	units uint16
	// unitOpen stops a trailing `;` counting as an extra statement.
	unitOpen bool
	// parens: separators only delimit statements at depth zero - the semicolons in `for (a; b; c)`
	// belong to that header.
	parens uint16
	// role decides whether counting statements means anything and whether --skip-objects drops it.
	role BraceRole
}

// countsAsDepth: a `{...}` in JSX only wraps an expression, so counting it would make
// `<Icon name={x} />` look as nested as `<Icon><Badge /></Icon>`.
func countsAsDepth(kind uint8) bool { return kind != fJSXExpr }

// ScanCodeBlocks walks code once, writing its canonical form into dst.Norm and recording every
// brace block and JSX element into dst.Blocks. dst is reused as scratch, so keep one per worker.
func ScanCodeBlocks(code []byte, opts BlockScanOptions, dst *BlockScan) *BlockScan {
	if dst == nil {
		dst = &BlockScan{}
	}
	// The canonical form is never longer than the source - every rule drops bytes or replaces a run
	// with one - so the buffer is sized from the file. Growth stays geometric so a run of
	// ever-larger files does not reallocate on each one.
	if !opts.SkipBlocks && cap(dst.Norm) < len(code) {
		size := len(code)
		if double := cap(dst.Norm) * 2; double > size {
			size = double
		}
		dst.Norm = make([]byte, 0, size)
	}
	dst.Norm = dst.Norm[:0]
	alsoAlt := opts.AltIdentifiers && !opts.SkipBlocks
	if alsoAlt && cap(dst.NormAlt) < len(code) {
		size := len(code)
		if double := cap(dst.NormAlt) * 2; double > size {
			size = double
		}
		dst.NormAlt = make([]byte, 0, size)
	}
	dst.NormAlt = dst.NormAlt[:0]
	dst.Blocks = dst.Blocks[:0]
	dst.Imports = dst.Imports[:0]
	dst.stack = dst.stack[:0]

	s := normScanner{
		code:           code,
		n:              len(code),
		blinding:       opts.Blinding,
		altIdentifiers: opts.AltIdentifiers,
		skim:           opts.SkipBlocks,
		tmplReturn:     nil,
	}

	// Import collection rides along. The statement parsers below are the ones the import-only entry
	// point has always used and they read the original source directly, so what they accept is
	// unchanged; this loop only decides WHERE to offer them a position.
	imp := parseState{
		code:              code,
		n:                 len(code),
		ignoreTypeImports: opts.IgnoreTypeImports,
		mode:              opts.ImportMode,
		imports:           dst.Imports[:0],
	}
	// braceDepth mirrors the import-only scan: static import, export and declare are only possible
	// at zero, and inside braces only the call forms are.
	braceDepth := 0
	// importSkipUntil keeps bytes a statement parser already accounted for from being offered
	// again. Ambient `declare` blocks use the same mechanism to stay excluded.
	importSkipUntil := 0

	norm := dst.Norm
	normAlt := dst.NormAlt
	stack := dst.stack
	minTokens := uint32(opts.MinTokens)
	// A frame's token count is the difference between this index at close and at open.
	tokenIndex := uint32(0)

	var (
		lastCode  byte = 0
		lastCode2 byte = 0
		wordStart int  = 0

		// A '<' cannot be classified until the byte after it is known, so it waits here.
		pendingLT         int = -1
		pendingLTAlt      uint32
		pendingLTOrig     uint32
		pendingLTAllowed  bool
		pendingLTChildren bool

		inCloseTag         bool
		justOpenedChildren bool
	)

	// jsxAllowedNow: after a value - identifier, number, `)`, `]`, or a string - the '<' is a
	// comparison; everywhere else it can open an element. wordEnd is the offset of the '<', so
	// norm[wordStart:wordEnd] is the identifier run that ended right before it.
	jsxAllowedNow := func(wordEnd uint32) bool {
		if lastCode == 0 {
			return true
		}
		if identLikeByte(lastCode) {
			if lastCode == IdentSentinel {
				return false
			}
			return regexAllowingKeywords[string(norm[wordStart:wordEnd])]
		}
		switch lastCode {
		case ')', ']', '"', '\'', '`':
			return false
		}
		return true
	}

	// structDepth counts only frames that represent real nesting, so JSX expression containers do
	// not inflate it.
	structDepth := 0

	push := func(kind uint8, normStart, altNormStart, origStart uint32) {
		// Whatever opened this frame is part of a statement of the frame enclosing it.
		if len(stack) > 0 {
			stack[len(stack)-1].unitOpen = true
		}
		// normStart is the offset of the `{`, so this is the identifier run that ended immediately
		// before it, without the brace.
		word := norm[:0]
		if wordStart <= int(normStart) {
			word = norm[wordStart:normStart]
		}
		role := BraceObjectLiteral
		if kind == fBrace {
			role = braceRoleAt(norm, int(normStart), lastCode, lastCode2, word)
		}
		if countsAsDepth(kind) {
			structDepth++
		}
		d := structDepth
		if d > 255 {
			d = 255
		}
		stack = append(stack, blockFrame{
			kind:         kind,
			normStart:    normStart,
			altNormStart: altNormStart,
			origStart:    origStart,
			tokenStart:   tokenIndex,
			maxSeen:      uint8(d),
			role:         role,
		})
	}

	// pop carries the frame's high-water mark into its parent on the way out.
	pop := func() (blockFrame, int) {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		inner := int(f.maxSeen) - structDepth
		if countsAsDepth(f.kind) {
			structDepth--
		}
		if inner < 0 {
			inner = 0
		}
		if len(stack) > 0 {
			top := &stack[len(stack)-1]
			if f.maxSeen > top.maxSeen {
				top.maxSeen = f.maxSeen
			}
		}
		return f, inner
	}

	emit := func(f blockFrame, normEnd, altNormEnd, origEnd uint32, kind BlockKind, inner int) {
		tokens := tokenIndex - f.tokenStart
		if tokens < minTokens {
			return
		}
		// Depth counts the block itself.
		depth := inner + 1

		// A statement left open at the close is the last one, which no `;` terminated.
		statements := 0
		if f.role == BraceStatementList {
			statements = int(f.units)
			if f.unitOpen {
				statements++
			}
		}
		if depth < opts.MinDepth {
			return
		}
		// The statement floor is a question about statement blocks. Applying it to an object literal or
		// JSX element would silently turn a complexity floor into "code blocks only".
		if f.role == BraceStatementList && statements < opts.MinStatements {
			return
		}
		// See CodeBlock.IsObjectLiteral.
		if opts.SkipObjectLiterals && kind == BlockBrace && f.role == BraceObjectLiteral {
			return
		}
		if depth > 255 {
			depth = 255
		}
		if statements > 65535 {
			statements = 65535
		}
		dst.Blocks = append(dst.Blocks, CodeBlock{
			NormStart:    f.normStart,
			NormEnd:      normEnd,
			AltNormStart: f.altNormStart,
			AltNormEnd:   altNormEnd,
			Start:        f.origStart,
			End:          origEnd,
			Kind:         kind,
			Depth:        uint8(depth),
			Role:         f.role,
			Tokens:       tokens,
			Statements:   uint16(statements),
		})
	}

	for {
		b, orig, class, ok := s.next()
		if !ok {
			break
		}
		if s.tokStart {
			tokenIndex++
		}
		p := uint32(len(norm))
		pAlt := uint32(len(normAlt))
		if !opts.SkipBlocks {
			norm = append(norm, b)
			if alsoAlt && !s.altSkip {
				normAlt = append(normAlt, s.altByte)
			}
		}

		// Track the current identifier run so a keyword before '<' can be recognised. In skim mode a
		// run arrives as one byte, so every identifier byte starts a run.
		startsWord := class == tokCode && identLikeByte(b) &&
			(s.skim || !identLikeByte(lastCode))
		if startsWord {
			wordStart = int(p)
		}

		// A statement can only begin where an identifier begins, and only in code - not inside a string,
		// template text, a comment or a regex, all already classified tokText.
		if opts.CollectImports && startsWord && orig >= importSkipUntil && !s.inTemplateInterpolation() {
			if next, ok := detectImportAt(&imp, orig, braceDepth); ok {
				importSkipUntil = next
			}
		}

		if class == tokCode {
			switch b {
			case '{':
				braceDepth++
			case '}':
				if braceDepth > 0 {
					braceDepth--
				}
			}
		}

		// Import-only callers stop here.
		if opts.SkipBlocks {
			lastCode2, lastCode = lastCode, b
			continue
		}

		if pendingLT >= 0 {
			startNorm := uint32(pendingLT)
			startAlt := pendingLTAlt
			startOrig := pendingLTOrig
			allowed := pendingLTAllowed
			children := pendingLTChildren
			pendingLT = -1

			if class == tokCode {
				if children && b == '/' {
					inCloseTag = true
					lastCode = b
					continue
				}
				if (allowed || children) && (identLikeByte(b) || b == '>') {
					push(fJSXTag, startNorm, startAlt, startOrig)
					if b == '>' {
						// `<>` - a fragment whose opening tag is already complete.
						stack[len(stack)-1].kind = fJSXChildren
					}
					lastCode2, lastCode = lastCode, b
					continue
				}
			}
			// Not a tag after all; fall through and treat b as ordinary code.
		}

		if inCloseTag {
			if class == tokCode && b == '>' {
				inCloseTag = false
				for len(stack) > 0 {
					top, inner := pop()
					if top.kind == fJSXChildren {
						emit(top, p+1, pAlt+1, uint32(orig)+1, BlockJSX, inner)
						break
					}
				}
			}
			lastCode2, lastCode = lastCode, b
			continue
		}

		if class != tokCode {
			lastCode2, lastCode = lastCode, b
			continue
		}

		// A generic arrow such as `<T,>(v) => v` looks like an opening tag until its first `(`.
		// Abandoning the frame here stops it swallowing the rest of the file.
		if justOpenedChildren {
			justOpenedChildren = false
			if b == '(' && len(stack) > 0 && stack[len(stack)-1].kind == fJSXChildren {
				pop()
			}
		}

		// Statement counting for the frame directly enclosing this byte: a separator closes the current
		// unit, anything else opens one.
		if len(stack) > 0 && b != '}' { // '}' closes this frame; it is not content of it
			top := &stack[len(stack)-1]
			if top.role == BraceStatementList {
				switch b {
				case '(', '[':
					top.parens++
					top.unitOpen = true
				case ')', ']':
					if top.parens > 0 {
						top.parens--
					}
					top.unitOpen = true
				case ';':
					// A `;` inside `for (a; b; c)` belongs to the header.
					if top.parens == 0 && top.unitOpen {
						top.units++
						top.unitOpen = false
					}
				default:
					if b != ' ' {
						top.unitOpen = true
					}
				}
			}
		}

		// dispatch re-runs when a frame turns out to be bogus, so the byte that exposed it is still
		// handled by the mode that really owns it.
	dispatch:
		for {
			mode := uint8(fBrace) // stands in for "plain JS"
			if len(stack) > 0 {
				switch stack[len(stack)-1].kind {
				case fJSXTag:
					mode = fJSXTag
				case fJSXChildren:
					mode = fJSXChildren
				}
			}

			switch mode {
			case fJSXTag:
				if isTagAbortByte(b) {
					// None of these can appear in an opening tag outside a `{...}` or a string, so the '<' was a
					// comparison or a type argument.
					pop()
					continue dispatch
				}
				switch b {
				case '{':
					push(fJSXExpr, p, pAlt, uint32(orig))
				case '>':
					if lastCode == '/' {
						top, inner := pop()
						emit(top, p+1, pAlt+1, uint32(orig)+1, BlockJSX, inner)
					} else {
						stack[len(stack)-1].kind = fJSXChildren
						justOpenedChildren = true
					}
				}

			case fJSXChildren:
				switch b {
				case '{':
					push(fJSXExpr, p, pAlt, uint32(orig))
				case '<':
					pendingLT = int(p)
					pendingLTAlt = pAlt
					pendingLTOrig = uint32(orig)
					pendingLTAllowed = true
					pendingLTChildren = true
				case '}':
					// Children never contain a bare '}', so the element frame is spurious.
					pop()
					continue dispatch
				}

			default:
				switch b {
				case '{':
					push(fBrace, p, pAlt, uint32(orig))
				case '}':
					if len(stack) > 0 {
						top, inner := pop()
						if top.kind == fBrace {
							emit(top, p+1, pAlt+1, uint32(orig)+1, BlockBrace, inner)
						}
						// `if (x) { ... }` is one statement of the block around it, terminated by nothing but the
						// closing brace. Inside parens the brace is an argument, so it ends nothing.
						if len(stack) > 0 {
							parent := &stack[len(stack)-1]
							if parent.role == BraceStatementList && parent.parens == 0 && parent.unitOpen {
								parent.units++
								parent.unitOpen = false
							}
						}
					}
				case '<':
					if opts.AllowJSX {
						pendingLT = int(p)
						pendingLTAlt = pAlt
						pendingLTOrig = uint32(orig)
						pendingLTAllowed = jsxAllowedNow(p)
						pendingLTChildren = false
					}
				}
			}
			break
		}

		lastCode2, lastCode = lastCode, b
	}

	dst.Norm = norm
	dst.NormAlt = normAlt
	dst.stack = stack[:0]
	dst.Imports = imp.imports
	return dst
}

// detectImportAt offers position i to the statement parsers that apply there and reports how far
// the accepted one consumed.
//
// The split by brace depth is not an optimisation: a static `import`/`export`/`declare` is only
// legal at the top level, so offering those parsers a nested position would accept text the
// import-only scan never accepted.
func detectImportAt(s *parseState, i int, braceDepth int) (int, bool) {
	if braceDepth > 0 {
		switch s.code[i] {
		case 'i':
			return s.parseDynamicImportAt(i)
		case 'r':
			return s.parseRequireCallAt(i)
		}
		return i, false
	}

	switch s.code[i] {
	case 'd':
		return s.skipDeclareAmbientBlock(i)
	case 'i':
		return s.parseImportStatement(i)
	case 'e':
		return s.parseExportStatement(i)
	case 'r':
		return s.parseRequireStatement(i)
	}
	return i, false
}

// inTemplateInterpolation: the import-only scan skipped template literals whole, so it never
// looked inside one, and matching that keeps the two entry points returning the same imports.
func (s *normScanner) inTemplateInterpolation() bool {
	return len(s.tmplReturn) > 0
}

// skipTemplateLiteral returns the index past the closing backtick, stepping over `${...}` so a
// backtick nested inside one does not look like the end of the outer literal.
func skipTemplateLiteral(code []byte, i int) int {
	n := len(code)
	j := i + 1
	for j < n {
		switch code[j] {
		case '\\':
			j += 2
			continue
		case '`':
			return j + 1
		case '$':
			if j+1 < n && code[j+1] == '{' {
				depth := 1
				j += 2
				// Regex literals are why this is not just quotes and braces: `${ s.replace(/"/g, '-') }` puts a
				// quote inside a regex, and reading it as a string opener sends skipToStringEnd off to the next
				// quote in the FILE - silently swallowing every import after it.
				for j < n && depth > 0 {
					switch code[j] {
					case '{':
						depth++
					case '}':
						depth--
					case '`':
						j = skipTemplateLiteral(code, j) - 1
					case '\'', '"':
						j = skipToStringEnd(code, j, code[j])
					case '/':
						if j+1 < n && code[j+1] == '/' {
							j = skipLineComment(code, j) - 1
						} else if j+1 < n && code[j+1] == '*' {
							j = skipBlockComment(code, j) - 1
						} else if regexLiteralAllowedBefore(code, j) {
							if end, ok := skipRegexLiteral(code, j); ok {
								j = end - 1
							}
						}
					}
					j++
				}
				continue
			}
		}
		j++
	}
	return n
}

// skimInteresting marks the bytes a skimming scan must stop on: identifier characters, the
// delimiters of strings, templates, comments and regex literals, the braces that carry depth,
// and whitespace.
var skimInteresting = func() [256]bool {
	var t [256]bool
	for c := 0; c < 256; c++ {
		b := byte(c)
		t[c] = isByteIdentifierChar(b) || isWhiteSpace(b) ||
			b == '\'' || b == '"' || b == '`' || b == '/' || b == '{' || b == '}'
	}
	return t
}()
