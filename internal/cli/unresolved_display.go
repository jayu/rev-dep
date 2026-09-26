package cli

import (
	"os"
	"path/filepath"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/parser"
	"rev-dep-go/internal/pathutil"
)

// maxCallTextInReport keeps one report line readable when the call is long or minified.
const maxCallTextInReport = 100

// nonLiteralTag follows the call in the formats a person reads. In JSON the same record is marked
// by the `nonLiteral` field instead.
const nonLiteralTag = "non-literal"

// unresolvedLabeller renders the text a report prints for one unresolved import. A literal request
// names itself; the `<non-literal>` stub does not, so it is shown as the call it stands for -
// without that the reader cannot tell which import of the file is meant.
type unresolvedLabeller struct {
	cwd   string
	files map[string][]byte
}

func newUnresolvedLabeller(cwd string) *unresolvedLabeller {
	return &unresolvedLabeller{cwd: cwd, files: make(map[string][]byte)}
}

func (l *unresolvedLabeller) Label(u checks.UnresolvedImport) string {
	if !u.IsNonLiteral {
		return u.Request
	}
	code, ok := l.code(u.FilePath)
	if !ok || int(u.RequestEnd) > len(code) || u.RequestEnd <= u.RequestStart {
		return nonLiteralLabelFor(u.Request)
	}
	return nonLiteralLabelFor(callTextAround(code, u.Request, u.RequestStart))
}

func (l *unresolvedLabeller) code(filePath string) ([]byte, bool) {
	if cached, ok := l.files[filePath]; ok {
		return cached, cached != nil
	}
	absPath := filePath
	if !filepath.IsAbs(absPath) {
		absPath = pathutil.JoinWithCwd(l.cwd, absPath)
	}
	code, err := os.ReadFile(pathutil.DenormalizePathForOS(absPath))
	if err != nil {
		l.files[filePath] = nil
		return nil, false
	}
	l.files[filePath] = code
	return code, true
}

// nonLiteralLabelFor puts the call first, because that is what identifies the import, and the tag
// after it.
func nonLiteralLabelFor(call string) string {
	if call == "" {
		return nonLiteralTag
	}
	return shorten(call) + " - " + nonLiteralTag
}

// callTextAround wraps the expression in the call it belongs to - `import('@/api/' + name)` - by
// reading the keyword that precedes it. The record's range covers the expression alone, and an
// expression on its own does not read like an import.
func callTextAround(code []byte, request string, start uint32) string {
	keyword := callKeywordBefore(code, start)
	if keyword == "" {
		return request
	}
	return keyword + "(" + request + ")"
}

// callKeywordBefore walks back from the expression over spaces and the `(` to the `import` or
// `require` that opened the call, and returns "" when the code in between is anything else.
func callKeywordBefore(code []byte, start uint32) string {
	i := int(start) - 1
	for i >= 0 && parser.IsWhiteSpace(code[i]) {
		i--
	}
	for i >= 0 && code[i] == '(' {
		i--
		for i >= 0 && parser.IsWhiteSpace(code[i]) {
			i--
		}
	}
	end := i + 1
	for i >= 0 && parser.IsByteIdentifierChar(code[i]) {
		i--
	}
	word := string(code[i+1 : end])
	if word == "import" || word == "require" {
		return word
	}
	return ""
}

func shorten(text string) string {
	if len(text) > maxCallTextInReport {
		return text[:maxCallTextInReport] + "..."
	}
	return text
}
