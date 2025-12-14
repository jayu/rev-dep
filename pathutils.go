package main

import (
	"path/filepath"
	"runtime"
	"strings"
)

// NormalizePathForInternal converts any OS path into a canonical internal
// representation using forward slashes and cleaned path components.
// Examples:
// - "C:\\project\\src\\file.ts" -> "C:/project/src/file.ts"
// - "./a/../b/" -> "b"
// Note: add "runtime" to the import list.
func NormalizePathForInternal(p string) string {
	if runtime.GOOS != "windows" {
		return p
	}
	if p == "" {
		return ""
	}
	cleaned := filepath.Clean(p)
	s := filepath.ToSlash(cleaned)
	// Trim trailing slash except when path is root like "/" or "C:/"
	if len(s) > 1 && strings.HasSuffix(s, "/") {
		s = strings.TrimRight(s, "/")
	}
	return s
}

// DenormalizePathForOS converts an internal forward-slash path back to the
// OS-native representation for os.* calls.
func DenormalizePathForOS(internal string) string {
	if runtime.GOOS != "windows" {
		return internal
	}
	if internal == "" {
		return ""
	}
	return filepath.FromSlash(internal)
}

// NormalizeGlobPattern normalizes glob pattern separators to forward slashes.
func NormalizeGlobPattern(pattern string) string {
	if runtime.GOOS != "windows" {
		return pattern
	}
	if pattern == "" {
		return ""
	}
	return strings.ReplaceAll(pattern, "\\\\", "/")
}

// NormalizePathsInSlice returns a new slice with each path normalized.
func NormalizePathsInSlice(xs []string) []string {
	if runtime.GOOS != "windows" {
		return xs
	}
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, NormalizePathForInternal(x))
	}
	return out
}
