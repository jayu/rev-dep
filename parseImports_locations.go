package main

import "sort"

type LineCol struct {
	Line int
	Col  int
}

type LineColRange struct {
	Start LineCol
	End   LineCol
}

type SimpleLocation struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

type lineIndex struct {
	starts []int
	length int
}

func newLineIndex(code []byte) lineIndex {
	starts := []int{0}
	for i, b := range code {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return lineIndex{starts: starts, length: len(code)}
}

func (li lineIndex) toLineCol(offset uint32) LineCol {
	off := int(offset)
	if off < 0 {
		off = 0
	}
	if off > li.length {
		off = li.length
	}
	lineIdx := sort.Search(len(li.starts), func(i int) bool {
		return li.starts[i] > off
	}) - 1
	if lineIdx < 0 {
		lineIdx = 0
	}
	col := off - li.starts[lineIdx] + 1
	return LineCol{Line: lineIdx + 1, Col: col}
}

func (li lineIndex) rangeFromOffsets(start uint32, end uint32) LineColRange {
	return LineColRange{
		Start: li.toLineCol(start),
		End:   li.toLineCol(end),
	}
}

func ResolvePrimaryLocation(code []byte, imp Import) SimpleLocation {
	index := newLineIndex(code)
	start := imp.RequestStart
	end := imp.RequestEnd
	isExportLike := imp.IsLocalExport || imp.ExportKeyEnd != 0
	if isExportLike {
		if imp.Keywords != nil && imp.Keywords.Len() > 0 {
			kw := imp.Keywords.Keywords[0]
			start = kw.Start
			end = kw.End
		} else if imp.ExportKeyStart != 0 || imp.ExportKeyEnd != 0 {
			start = imp.ExportKeyStart
			end = imp.ExportKeyEnd
		} else if imp.ExportDeclStart != 0 {
			start = imp.ExportDeclStart
			end = imp.ExportDeclStart
		}
	}
	rng := index.rangeFromOffsets(start, end)
	return SimpleLocation{
		StartLine: rng.Start.Line,
		StartCol:  rng.Start.Col,
		EndLine:   rng.End.Line,
		EndCol:    rng.End.Col,
	}
}
