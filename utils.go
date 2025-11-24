package main

import (
	"bytes"
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
	result := make([]byte, 0, len(code))
	i := 0
	n := len(code)

	inStringContent := false
	for i < n {
		// detect string content
		if bytes.HasPrefix(code[i:], []byte("\"")) {
			inStringContent = !inStringContent
		}

		if !inStringContent {
			if bytes.HasPrefix(code[i:], []byte("//")) {
				i += 2
				endOfLineIndex := bytes.Index(code[i:], []byte("\n"))
				i += endOfLineIndex
			}

			if bytes.HasPrefix(code[i:], []byte("/*")) {
				i += 2
				endOfLineIndex := bytes.Index(code[i:], []byte("*/"))
				i += endOfLineIndex + 2
			}
		}

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
