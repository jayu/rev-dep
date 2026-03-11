package main

import (
	"os"
	"path/filepath"
	"strconv"
)

// fileLocationResolver maps violations to source locations for JSON output without re-parsing.
// It uses the already-built MinimalDependencyTree (from detailed parsing) and only reads file
// contents to convert stored byte offsets into line/column, while disambiguating duplicates by
// returning subsequent matches for identical requests/module names/exports within a file.
type fileLocationResolver struct {
	cwd       string
	tree      MinimalDependencyTree
	codeCache map[string][]byte
	indexes   map[string]*fileLocationIndex
	counters  map[string]int
}

type fileLocationIndex struct {
	byRequest    map[string][]SimpleLocation
	byModuleName map[string][]SimpleLocation
	byExportName map[string][]SimpleLocation
}

func newFileLocationResolver(cwd string, tree MinimalDependencyTree) *fileLocationResolver {
	return &fileLocationResolver{
		cwd:       cwd,
		tree:      tree,
		codeCache: make(map[string][]byte),
		indexes:   make(map[string]*fileLocationIndex),
		counters:  make(map[string]int),
	}
}

func (r *fileLocationResolver) getDeps(filePath string) ([]MinimalDependency, bool) {
	if filePath == "" {
		return nil, false
	}
	if deps, ok := r.tree[filePath]; ok {
		return deps, true
	}
	normalized := NormalizePathForInternal(filePath)
	if deps, ok := r.tree[normalized]; ok {
		return deps, true
	}
	if !filepath.IsAbs(filePath) {
		abs := NormalizePathForInternal(JoinWithCwd(r.cwd, filePath))
		if deps, ok := r.tree[abs]; ok {
			return deps, true
		}
	}
	return nil, false
}

func (r *fileLocationResolver) getFileCode(filePath string) ([]byte, bool) {
	if filePath == "" {
		return nil, false
	}
	absPath := filePath
	if !filepath.IsAbs(absPath) {
		absPath = JoinWithCwd(r.cwd, absPath)
	}
	if cached, ok := r.codeCache[absPath]; ok {
		return cached, true
	}
	code, err := os.ReadFile(absPath)
	if err != nil {
		return nil, false
	}
	r.codeCache[absPath] = code
	return code, true
}

func (r *fileLocationResolver) locationForRequest(filePath string, request string) jsonLocationFields {
	loc := r.nextLocation(filePath, "request", request)
	return locationFieldsFromSimple(loc)
}

func (r *fileLocationResolver) locationForModuleName(filePath string, moduleName string) jsonLocationFields {
	loc := r.nextLocation(filePath, "module", moduleName)
	return locationFieldsFromSimple(loc)
}

func (r *fileLocationResolver) locationForExport(filePath string, exportName string) jsonLocationFields {
	loc := r.nextLocation(filePath, "export", exportName)
	return locationFieldsFromSimple(loc)
}

func (r *fileLocationResolver) locationForPackageJsonDependency(filePath string, moduleName string) jsonLocationFields {
	loc := r.packageJsonLocation(filePath, moduleName)
	return locationFieldsFromSimple(loc)
}

func (r *fileLocationResolver) nextLocation(filePath string, kind string, key string) *SimpleLocation {
	index, ok := r.getIndex(filePath)
	if !ok {
		return nil
	}

	cacheKey := kind + ":" + filePath + ":" + key
	idx := r.counters[cacheKey]
	r.counters[cacheKey] = idx + 1

	var list []SimpleLocation
	switch kind {
	case "request":
		list = index.byRequest[key]
	case "module":
		list = index.byModuleName[key]
	case "export":
		list = index.byExportName[key]
	default:
		return nil
	}

	if idx < 0 || idx >= len(list) {
		return nil
	}
	return &list[idx]
}

func (r *fileLocationResolver) packageJsonLocation(filePath string, moduleName string) *SimpleLocation {
	code, ok := r.getFileCode(filePath)
	if !ok {
		return nil
	}
	return findPackageJsonDependencyLocation(code, moduleName)
}

func (r *fileLocationResolver) getIndex(filePath string) (*fileLocationIndex, bool) {
	if filePath == "" {
		return nil, false
	}
	if idx, ok := r.indexes[filePath]; ok {
		return idx, true
	}
	normalized := NormalizePathForInternal(filePath)
	if idx, ok := r.indexes[normalized]; ok {
		return idx, true
	}
	if !filepath.IsAbs(filePath) {
		abs := NormalizePathForInternal(JoinWithCwd(r.cwd, filePath))
		if idx, ok := r.indexes[abs]; ok {
			return idx, true
		}
	}

	deps, ok := r.getDeps(filePath)
	if !ok {
		return nil, false
	}
	code, ok := r.getFileCode(filePath)
	if !ok {
		return nil, false
	}

	index := &fileLocationIndex{
		byRequest:    make(map[string][]SimpleLocation),
		byModuleName: make(map[string][]SimpleLocation),
		byExportName: make(map[string][]SimpleLocation),
	}

	for _, dep := range deps {
		if dep.Request != "" {
			loc := primaryFromDependency(code, dep)
			index.byRequest[dep.Request] = append(index.byRequest[dep.Request], loc)
			moduleName := GetNodeModuleName(dep.Request)
			if moduleName != "" {
				index.byModuleName[moduleName] = append(index.byModuleName[moduleName], loc)
			}
		}
		if dep.IsLocalExport || dep.ExportKeyEnd != 0 {
			if dep.Keywords == nil {
				continue
			}
			for _, kw := range dep.Keywords.Keywords {
				name := kw.Name
				if kw.Alias != "" {
					name = kw.Alias
				}
				if name == "" {
					continue
				}
				index.byExportName[name] = append(index.byExportName[name], keywordLocation(code, kw))
			}
		}
	}

	r.indexes[filePath] = index
	r.indexes[normalized] = index
	if !filepath.IsAbs(filePath) {
		abs := NormalizePathForInternal(JoinWithCwd(r.cwd, filePath))
		r.indexes[abs] = index
	}
	return index, true
}

func keywordLocation(code []byte, kw KeywordInfo) SimpleLocation {
	index := newLineIndex(code)
	rng := index.rangeFromOffsets(kw.Start, kw.End)
	return SimpleLocation{
		StartLine: rng.Start.Line,
		StartCol:  rng.Start.Col,
		EndLine:   rng.End.Line,
		EndCol:    rng.End.Col,
	}
}

func primaryFromDependency(code []byte, dep MinimalDependency) SimpleLocation {
	imp := Import{
		Request:            dep.Request,
		Keywords:           dep.Keywords,
		RequestStart:       dep.RequestStart,
		RequestEnd:         dep.RequestEnd,
		IsDynamicImport:    dep.IsDynamicImport,
		IsLocalExport:      dep.IsLocalExport,
		ExportKeyStart:     dep.ExportKeyStart,
		ExportKeyEnd:       dep.ExportKeyEnd,
		ExportDeclStart:    dep.ExportDeclStart,
		ExportBraceStart:   dep.ExportBraceStart,
		ExportBraceEnd:     dep.ExportBraceEnd,
		ExportStatementEnd: dep.ExportStatementEnd,
	}
	return ResolvePrimaryLocation(code, imp)
}

func intPtr(v int) *int {
	return &v
}

func locationFieldsFromSimple(loc *SimpleLocation) jsonLocationFields {
	if loc == nil {
		return jsonLocationFields{}
	}
	return jsonLocationFields{
		StartLine: intPtr(loc.StartLine),
		StartCol:  intPtr(loc.StartCol),
		EndLine:   intPtr(loc.EndLine),
		EndCol:    intPtr(loc.EndCol),
	}
}

func findPackageJsonDependencyLocation(code []byte, moduleName string) *SimpleLocation {
	if len(code) == 0 || moduleName == "" {
		return nil
	}

	type jsonCtx struct {
		kind         byte
		expectingKey bool
		isDepsObject bool
	}

	depSection := map[string]bool{
		"dependencies":         true,
		"devDependencies":      true,
		"peerDependencies":     true,
		"optionalDependencies": true,
	}

	stack := make([]jsonCtx, 0, 4)
	pendingDepsObject := false
	lastKey := ""
	lastKeyStart := 0
	lastKeyEnd := 0

	for i := 0; i < len(code); {
		i = skipJSONSpacesAndComments(code, i)
		if i >= len(code) {
			break
		}

		if pendingDepsObject && code[i] != '{' {
			pendingDepsObject = false
		}

		switch code[i] {
		case '{':
			isDeps := pendingDepsObject
			pendingDepsObject = false
			stack = append(stack, jsonCtx{kind: '{', expectingKey: true, isDepsObject: isDeps})
			i++
		case '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			i++
		case '[':
			pendingDepsObject = false
			stack = append(stack, jsonCtx{kind: '[', expectingKey: false})
			i++
		case ']':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			i++
		case ',':
			if len(stack) > 0 && stack[len(stack)-1].kind == '{' {
				stack[len(stack)-1].expectingKey = true
			}
			pendingDepsObject = false
			i++
		case ':':
			if len(stack) > 0 && stack[len(stack)-1].kind == '{' {
				stack[len(stack)-1].expectingKey = false
			}
			if depSection[lastKey] {
				pendingDepsObject = true
			} else {
				pendingDepsObject = false
			}
			i++
		case '"':
			str, next, start, end := parseJSONStringLiteral(code, i)
			if len(stack) > 0 && stack[len(stack)-1].kind == '{' && stack[len(stack)-1].expectingKey {
				lastKey = str
				lastKeyStart = start
				lastKeyEnd = end
				if stack[len(stack)-1].isDepsObject && lastKey == moduleName {
					index := newLineIndex(code)
					rng := index.rangeFromOffsets(uint32(lastKeyStart), uint32(lastKeyEnd))
					return &SimpleLocation{
						StartLine: rng.Start.Line,
						StartCol:  rng.Start.Col,
						EndLine:   rng.End.Line,
						EndCol:    rng.End.Col,
					}
				}
			}
			i = next
		default:
			pendingDepsObject = false
			i++
		}
	}

	return nil
}

func skipJSONSpacesAndComments(code []byte, i int) int {
	for i < len(code) {
		switch code[i] {
		case ' ', '\t', '\r', '\n':
			i++
		case '/':
			if i+1 >= len(code) {
				return i
			}
			if code[i+1] == '/' {
				i += 2
				for i < len(code) && code[i] != '\n' {
					i++
				}
			} else if code[i+1] == '*' {
				i += 2
				for i+1 < len(code) && !(code[i] == '*' && code[i+1] == '/') {
					i++
				}
				if i+1 < len(code) {
					i += 2
				}
			} else {
				return i
			}
		default:
			return i
		}
	}
	return i
}

func parseJSONStringLiteral(code []byte, i int) (string, int, int, int) {
	if i >= len(code) || code[i] != '"' {
		return "", i, 0, 0
	}
	start := i + 1
	j := start
	for j < len(code) {
		if code[j] == '\\' {
			j += 2
			continue
		}
		if code[j] == '"' {
			break
		}
		j++
	}
	if j >= len(code) {
		return "", j, 0, 0
	}
	raw := code[i : j+1]
	key := ""
	if unquoted, err := strconv.Unquote(string(raw)); err == nil {
		key = unquoted
	} else {
		key = string(code[start:j])
	}
	return key, j + 1, start, j
}
