package main

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestGetFollowMonorepoPackagesValue(t *testing.T) {
	t.Run("flag absent should be disabled", func(t *testing.T) {
		followMonorepoPackages = nil
		cmd := &cobra.Command{Use: "test"}
		addSharedFlags(cmd)

		if err := cmd.ParseFlags([]string{}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}

		got, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.IsEnabled() {
			t.Fatalf("expected disabled follow mode, got %+v", got)
		}
	})

	t.Run("bare flag should follow all", func(t *testing.T) {
		followMonorepoPackages = nil
		cmd := &cobra.Command{Use: "test"}
		addSharedFlags(cmd)

		if err := cmd.ParseFlags([]string{"--follow-monorepo-packages"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}

		got, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !got.ShouldFollowAll() {
			t.Fatalf("expected follow all mode, got %+v", got)
		}
	})

	t.Run("flag with values should be selective", func(t *testing.T) {
		followMonorepoPackages = nil
		cmd := &cobra.Command{Use: "test"}
		addSharedFlags(cmd)

		if err := cmd.ParseFlags([]string{"--follow-monorepo-packages=pkg-a,@scope/*"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}

		got, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := FollowMonorepoPackagesValue{Packages: map[string]bool{"pkg-a": true, "@scope/*": true}}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected %+v, got %+v", expected, got)
		}
	})

	t.Run("explicit star value should not mean follow all", func(t *testing.T) {
		followMonorepoPackages = nil
		cmd := &cobra.Command{Use: "test"}
		addSharedFlags(cmd)

		if err := cmd.ParseFlags([]string{"--follow-monorepo-packages=*"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}

		got, err := getFollowMonorepoPackagesValue(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := FollowMonorepoPackagesValue{Packages: map[string]bool{"*": true}}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected %+v, got %+v", expected, got)
		}
	})
}
