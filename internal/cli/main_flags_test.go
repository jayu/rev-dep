package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"rev-dep-go/internal/model"
)

func TestGetFollowMonorepoPackagesValue(t *testing.T) {
	t.Run("flag absent should be disabled", func(t *testing.T) {
		SetFollowMonorepoPackages(nil)
		cmd := &cobra.Command{Use: "test"}
		AddSharedFlags(cmd)

		if err := cmd.ParseFlags([]string{}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}

		got, err := GetFollowMonorepoPackagesValue(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.IsEnabled() {
			t.Fatalf("expected disabled follow mode, got %+v", got)
		}
	})

	t.Run("bare flag should follow all", func(t *testing.T) {
		SetFollowMonorepoPackages(nil)
		cmd := &cobra.Command{Use: "test"}
		AddSharedFlags(cmd)

		if err := cmd.ParseFlags([]string{"--follow-monorepo-packages"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}

		got, err := GetFollowMonorepoPackagesValue(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !got.ShouldFollowAll() {
			t.Fatalf("expected follow all mode, got %+v", got)
		}
	})

	t.Run("flag with values should be selective", func(t *testing.T) {
		SetFollowMonorepoPackages(nil)
		cmd := &cobra.Command{Use: "test"}
		AddSharedFlags(cmd)

		if err := cmd.ParseFlags([]string{"--follow-monorepo-packages=pkg-a,@scope/*"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}

		got, err := GetFollowMonorepoPackagesValue(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := model.FollowMonorepoPackagesValue{Packages: map[string]bool{"pkg-a": true, "@scope/*": true}}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected %+v, got %+v", expected, got)
		}
	})

	t.Run("explicit star value should not mean follow all", func(t *testing.T) {
		SetFollowMonorepoPackages(nil)
		cmd := &cobra.Command{Use: "test"}
		AddSharedFlags(cmd)

		if err := cmd.ParseFlags([]string{"--follow-monorepo-packages=*"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}

		got, err := GetFollowMonorepoPackagesValue(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := model.FollowMonorepoPackagesValue{Packages: map[string]bool{"*": true}}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("expected %+v, got %+v", expected, got)
		}
	})
}

func TestSanitizeFlagSentinelInHelpOutput(t *testing.T) {
	rawOutput := "      --follow-monorepo-packages strings[=" + FollowMonorepoPackagesAllSentinel() + "]   test"
	sanitizedOutput := SanitizeFlagSentinelInHelpOutput(rawOutput)

	if strings.Contains(sanitizedOutput, FollowMonorepoPackagesAllSentinel()) {
		t.Fatalf("sanitized output should not expose internal sentinel, got:\n%s", sanitizedOutput)
	}

	if !strings.Contains(sanitizedOutput, "--follow-monorepo-packages strings") {
		t.Fatalf("sanitized output should keep the flag usage, got:\n%s", sanitizedOutput)
	}
}
