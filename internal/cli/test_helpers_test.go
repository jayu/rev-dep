package cli

import (
	"testing"

	"rev-dep-go/internal/testutil"
)

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	path, err := testutil.FixturePath(parts...)
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}
	return path
}

func goldenPath(t *testing.T, name string) string {
	t.Helper()
	path, err := testutil.GoldenPath(name)
	if err != nil {
		t.Fatalf("GoldenPath: %v", err)
	}
	return path
}
