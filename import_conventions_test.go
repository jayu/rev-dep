package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportConventions_ObjectDomains_TsWildcardAlias(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "TestImportConventions_ObjectDomains_TsWildcardAlias")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create package.json
	packageJson := `{
		"name": "test-project",
		"dependencies": {}
	}`
	os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJson), 0644)

	// Create tsconfig.json with alias
	tsconfig := `{
		"compilerOptions": {
			"baseUrl": ".",
			"paths": {
				"@/auth/*": ["./src/auth/*"],
				"@/settings/*": ["./src/settings/*"],
				"@/chat/*": ["./src/chat/*"],
				"@/chat-another/*": ["./src/chat/*"],
				"*": ["./*"],
			}
		}
	}`

	os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(tsconfig), 0644)

	config := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{
			{
				Path: ".",
				ImportConventions: []ImportConventionRule{
					{
						Rule:    "relative-internal-absolute-external",
						Autofix: true,
						Domains: []ImportConventionDomain{
							{
								Path:    "src/auth",
								Alias:   "@/auth",
								Enabled: true,
							},
							{
								Path: "src/users",
								// Matches TS config wildcard alias
								Enabled: true,
							},
							{
								Path: "src/settings",
								// Matches TS config @/settings alias
								Enabled: true,
							},
							{
								Path:    "src/chat",
								Alias:   "@/chat/",
								Enabled: false,
							},
						},
					},
				},
			},
		},
	}

	// == Auth domain ==
	authDir := filepath.Join(tempDir, "src", "auth")
	os.MkdirAll(authDir, 0755)

	authFile := filepath.Join(authDir, "file.ts")
	authFile2 := filepath.Join(authDir, "file2.ts") // we need a file that will be resolved

	authFileContent := `
		import authFile2 from "@/auth/file2.ts";
		import chatFile1 from "../chat/file";
		import settingsFile1 from "../settings/file";
		import usersFile1 from "../users/file";
		import chatFile2 from "src/chat/file";
		import chatFile3 from "@/chat-another/file";
	`

	os.WriteFile(authFile, []byte(authFileContent), 0644)
	os.WriteFile(authFile2, []byte(""), 0644)

	// == Users domain ==
	usersDir := filepath.Join(tempDir, "src", "users")
	os.MkdirAll(usersDir, 0755)

	usersFile := filepath.Join(usersDir, "file.ts")
	usersFileContent := `
		
	`

	os.WriteFile(usersFile, []byte(usersFileContent), 0644)

	// == Settings domain ==
	settingsDir := filepath.Join(tempDir, "src", "settings")
	os.MkdirAll(settingsDir, 0755)

	settingsFile := filepath.Join(settingsDir, "file.ts")
	settingsFileContent := `
		
	`

	os.WriteFile(settingsFile, []byte(settingsFileContent), 0644)

	// == Chat domain ==
	chatDir := filepath.Join(tempDir, "src", "chat")
	os.MkdirAll(chatDir, 0755)

	chatFile := filepath.Join(chatDir, "file.ts")
	chatFileContent := `
		
	`

	os.WriteFile(chatFile, []byte(chatFileContent), 0644)

	result, err := ProcessConfig(&config, tempDir, "package.json", "tsconfig.json", true)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	// Check that we have a violation and it was fixed
	if len(result.RuleResults) == 0 {
		t.Fatalf("Expected at least 1 rule result")
	}

	ruleResult := result.RuleResults[0]

	if len(ruleResult.ImportConventionViolations) != 6 {
		t.Fatalf("Expected 3 violations, got %d", len(ruleResult.ImportConventionViolations))
	}

	// == should-be-relative + persist extension ==
	violation1 := ruleResult.ImportConventionViolations[0]

	if !strings.HasSuffix(violation1.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation1.FilePath)
	}

	if violation1.ViolationType != "should-be-relative" {
		t.Errorf("Expected violation type 'should-be-relative', got '%s'", violation1.ViolationType)
	}

	if violation1.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation1.Fix != nil && violation1.Fix.Text != "./file2.ts" {
		t.Errorf("Expected violation fix text to be './file2.ts', got '%s'", violation1.Fix.Text)
	}

	violation1fileAfterFix, _ := os.ReadFile(violation1.FilePath)

	if !strings.Contains(string(violation1fileAfterFix), `import authFile2 from "./file2.ts";`) {
		t.Errorf("Expected file content to be fixed to use relative import, got:\n%s", string(violation1fileAfterFix))
	}

	// == should-be-aliased - alias defined in domains config ==
	violation2 := ruleResult.ImportConventionViolations[1]

	if !strings.HasSuffix(violation2.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation2.FilePath)
	}

	if violation2.ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got '%s'", violation2.ViolationType)
	}

	if violation2.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation2.Fix != nil && violation2.Fix.Text != "@/chat/file" {
		t.Errorf("Expected violation fix text to be '@/chat/file', got '%s'", violation2.Fix.Text)
	}

	violation2fileAfterFix, _ := os.ReadFile(violation2.FilePath)

	if !strings.Contains(string(violation2fileAfterFix), `import chatFile1 from "@/chat/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation2fileAfterFix))
	}

	// == should-be-aliased - alias not defined in domains config, resolves to non base url TS alias  ==
	violation3 := ruleResult.ImportConventionViolations[2]

	if !strings.HasSuffix(violation3.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation3.FilePath)
	}

	if violation3.ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got '%s'", violation3.ViolationType)
	}

	if violation3.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation3.Fix != nil && violation3.Fix.Text != "@/settings/file" {
		t.Errorf("Expected violation fix text to be '@/settings/file', got '%s'", violation3.Fix.Text)
	}

	violation3fileAfterFix, _ := os.ReadFile(violation3.FilePath)

	if !strings.Contains(string(violation3fileAfterFix), `import settingsFile1 from "@/settings/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation3fileAfterFix))
	}

	// == should-be-aliased - alias not defined in domains config, resolves to base url TS alias  ==
	violation4 := ruleResult.ImportConventionViolations[3]

	if !strings.HasSuffix(violation4.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation4.FilePath)
	}

	if violation4.ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got '%s'", violation4.ViolationType)
	}

	if violation4.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation4.Fix != nil && violation4.Fix.Text != "src/users/file" {
		t.Errorf("Expected violation fix text to be 'src/users/file', got '%s'", violation4.Fix.Text)
	}

	violation4fileAfterFix, _ := os.ReadFile(violation4.FilePath)

	if !strings.Contains(string(violation4fileAfterFix), `import usersFile1 from "src/users/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation4fileAfterFix))
	}

	// == wrong-alias - current alias is baseUrl wildcard alias ("*": ["./*"],)  ==
	violation5 := ruleResult.ImportConventionViolations[4]

	if !strings.HasSuffix(violation5.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation5.FilePath)
	}

	if violation5.ViolationType != "wrong-alias" {
		t.Errorf("Expected violation type 'wrong-alias', got '%s'", violation5.ViolationType)
	}

	if violation5.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation5.Fix != nil && violation5.Fix.Text != "@/chat/file" {
		t.Errorf("Expected violation fix text to be '@/chat/file', got '%s'", violation5.Fix.Text)
	}

	violation5fileAfterFix, _ := os.ReadFile(violation5.FilePath)

	if !strings.Contains(string(violation5fileAfterFix), `import chatFile2 from "@/chat/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation5fileAfterFix))
	}

	// == wrong-alias - current alias is non-empty string alias  ==
	violation6 := ruleResult.ImportConventionViolations[5]

	if !strings.HasSuffix(violation6.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation6.FilePath)
	}

	if violation6.ViolationType != "wrong-alias" {
		t.Errorf("Expected violation type 'wrong-alias', got '%s'", violation6.ViolationType)
	}

	if violation6.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation6.Fix != nil && violation6.Fix.Text != "@/chat/file" {
		t.Errorf("Expected violation fix text to be '@/chat/file', got '%s'", violation6.Fix.Text)
	}

	violation6fileAfterFix, _ := os.ReadFile(violation6.FilePath)

	if !strings.Contains(string(violation6fileAfterFix), `import chatFile3 from "@/chat/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation6fileAfterFix))
	}

	// == Disabled domain == 0 violations ==

	config.Rules[0].ImportConventions[0].Domains[0].Enabled = false // disable auth domain

	// Reset auth file content
	os.WriteFile(authFile, []byte(authFileContent), 0644)

	result, err = ProcessConfig(&config, tempDir, "package.json", "tsconfig.json", true)

	if result.HasFailures {
		t.Fatalf("Expected no violations for disabled domain, got %d violations", len(result.RuleResults[0].ImportConventionViolations))
	}
}

func TestImportConventions_ObjectDomains_NoTsWildcardAlias(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "TestImportConventions_ObjectDomains_NoTsWildcardAlias")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create package.json
	packageJson := `{
		"name": "test-project",
		"dependencies": {}
	}`
	os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJson), 0644)

	// Create tsconfig.json with alias
	tsconfig := `{
		"compilerOptions": {
			"paths": {
				"@/auth/*": ["./src/auth/*"],
				"@/settings/*": ["./src/settings/*"],
				"@/chat/*": ["./src/chat/*"],
				"@/chat-another/*": ["./src/chat/*"],
			}
		}
	}`

	os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(tsconfig), 0644)

	config := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{
			{
				Path: ".",
				ImportConventions: []ImportConventionRule{
					{
						Rule:    "relative-internal-absolute-external",
						Autofix: true,
						Domains: []ImportConventionDomain{
							{
								Path:    "src/auth",
								Alias:   "@/auth",
								Enabled: true,
							},
							{
								Path: "src/users",
								// Matches TS config wildcard alias
								Enabled: true,
							},
							{
								Path: "src/settings",
								// Matches TS config @/settings alias
								Enabled: true,
							},
							{
								Path:    "src/chat",
								Alias:   "@/chat/",
								Enabled: false,
							},
						},
					},
				},
			},
		},
	}

	// == Auth domain ==
	authDir := filepath.Join(tempDir, "src", "auth")
	os.MkdirAll(authDir, 0755)

	authFile := filepath.Join(authDir, "file.ts")
	authFile2 := filepath.Join(authDir, "file2.ts") // we need a file that will be resolved

	authFileContent := `
		import authFile2 from "@/auth/file2.ts";
		import chatFile1 from "../chat/file";
		import settingsFile1 from "../settings/file";
		import usersFile1 from "../users/file";
		import chatFile2 from "src/chat/file";
		import chatFile3 from "@/chat-another/file";
	`

	os.WriteFile(authFile, []byte(authFileContent), 0644)
	os.WriteFile(authFile2, []byte(""), 0644)

	// == Users domain ==
	usersDir := filepath.Join(tempDir, "src", "users")
	os.MkdirAll(usersDir, 0755)

	usersFile := filepath.Join(usersDir, "file.ts")
	usersFileContent := `
		
	`

	os.WriteFile(usersFile, []byte(usersFileContent), 0644)

	// == Settings domain ==
	settingsDir := filepath.Join(tempDir, "src", "settings")
	os.MkdirAll(settingsDir, 0755)

	settingsFile := filepath.Join(settingsDir, "file.ts")
	settingsFileContent := `
		
	`

	os.WriteFile(settingsFile, []byte(settingsFileContent), 0644)

	// == Chat domain ==
	chatDir := filepath.Join(tempDir, "src", "chat")
	os.MkdirAll(chatDir, 0755)

	chatFile := filepath.Join(chatDir, "file.ts")
	chatFileContent := `
		
	`

	os.WriteFile(chatFile, []byte(chatFileContent), 0644)

	result, err := ProcessConfig(&config, tempDir, "package.json", "tsconfig.json", true)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	// Check that we have a violation and it was fixed
	if len(result.RuleResults) == 0 {
		t.Fatalf("Expected at least 1 rule result")
	}

	ruleResult := result.RuleResults[0]

	if len(ruleResult.ImportConventionViolations) != 5 {
		t.Fatalf("Expected 5 violations, got %d", len(ruleResult.ImportConventionViolations))
	}

	// == should-be-relative + persist extension ==
	violation1 := ruleResult.ImportConventionViolations[0]

	if !strings.HasSuffix(violation1.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation1.FilePath)
	}

	if violation1.ViolationType != "should-be-relative" {
		t.Errorf("Expected violation type 'should-be-relative', got '%s'", violation1.ViolationType)
	}

	if violation1.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation1.Fix != nil && violation1.Fix.Text != "./file2.ts" {
		t.Errorf("Expected violation fix text to be './file2.ts', got '%s'", violation1.Fix.Text)
	}

	violation1fileAfterFix, _ := os.ReadFile(violation1.FilePath)

	if !strings.Contains(string(violation1fileAfterFix), `import authFile2 from "./file2.ts";`) {
		t.Errorf("Expected file content to be fixed to use relative import, got:\n%s", string(violation1fileAfterFix))
	}

	// == should-be-aliased - alias defined in domains config ==
	violation2 := ruleResult.ImportConventionViolations[1]

	if !strings.HasSuffix(violation2.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation2.FilePath)
	}

	if violation2.ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got '%s'", violation2.ViolationType)
	}

	if violation2.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation2.Fix != nil && violation2.Fix.Text != "@/chat/file" {
		t.Errorf("Expected violation fix text to be '@/chat/file', got '%s'", violation2.Fix.Text)
	}

	violation2fileAfterFix, _ := os.ReadFile(violation2.FilePath)

	if !strings.Contains(string(violation2fileAfterFix), `import chatFile1 from "@/chat/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation2fileAfterFix))
	}

	// == should-be-aliased - alias not defined in domains config, resolves to non base url TS alias  ==
	violation3 := ruleResult.ImportConventionViolations[2]

	if !strings.HasSuffix(violation3.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation3.FilePath)
	}

	if violation3.ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got '%s'", violation3.ViolationType)
	}

	if violation3.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation3.Fix != nil && violation3.Fix.Text != "@/settings/file" {
		t.Errorf("Expected violation fix text to be '@/settings/file', got '%s'", violation3.Fix.Text)
	}

	violation3fileAfterFix, _ := os.ReadFile(violation3.FilePath)

	if !strings.Contains(string(violation3fileAfterFix), `import settingsFile1 from "@/settings/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation3fileAfterFix))
	}

	// == should-be-aliased - alias not defined in domains config, no matching alias defined in ts config  ==
	violation4 := ruleResult.ImportConventionViolations[3]

	if !strings.HasSuffix(violation4.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation4.FilePath)
	}

	if violation4.ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got '%s'", violation4.ViolationType)
	}

	if violation4.Fix != nil {
		t.Errorf("Expected violation fix to not exist, got %s", fmt.Sprintf("%v", violation4.Fix.Text))
	}

	violation4fileAfterFix, _ := os.ReadFile(violation4.FilePath)

	if !strings.Contains(string(violation4fileAfterFix), `import usersFile1 from "../users/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation4fileAfterFix))
	}

	// == wrong-alias - current alias is baseUrl wildcard alias ("*": ["./*"],)  ==
	// Ts config does not have such alias, so import is not resolved, no violation matched

	// == wrong-alias - current alias is non-empty string alias  ==
	violation5 := ruleResult.ImportConventionViolations[4]

	if !strings.HasSuffix(violation5.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation5.FilePath)
	}

	if violation5.ViolationType != "wrong-alias" {
		t.Errorf("Expected violation type 'wrong-alias', got '%s'", violation5.ViolationType)
	}

	if violation5.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation5.Fix != nil && violation5.Fix.Text != "@/chat/file" {
		t.Errorf("Expected violation fix text to be '@/chat/file', got '%s'", violation5.Fix.Text)
	}

	violation5fileAfterFix, _ := os.ReadFile(violation5.FilePath)

	if !strings.Contains(string(violation5fileAfterFix), `import chatFile3 from "@/chat/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation5fileAfterFix))
	}

	// == Disabled domain == 0 violations ==

	config.Rules[0].ImportConventions[0].Domains[0].Enabled = false // disable auth domain

	// Reset auth file content
	os.WriteFile(authFile, []byte(authFileContent), 0644)

	result, err = ProcessConfig(&config, tempDir, "package.json", "tsconfig.json", true)

	if result.HasFailures {
		t.Fatalf("Expected no violations for disabled domain, got %d violations", len(result.RuleResults[0].ImportConventionViolations))
	}
}

func TestImportConventions_StringDomains_TsWildcardAlias(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "TestImportConventions_StringDomains_TsWildcardAlias")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create package.json
	packageJson := `{
		"name": "test-project",
		"dependencies": {}
	}`
	os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJson), 0644)

	// Create tsconfig.json with alias
	tsconfig := `{
		"compilerOptions": {
			"baseUrl": ".", // 👈 test this way of defining wildcard alias
			"paths": {
				"@/auth/*": ["./src/auth/*"],
				"@/utils/*": ["./utils/*"],
			}
		}
	}`

	os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(tsconfig), 0644)

	config := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{
			{
				Path: ".",
				ImportConventions: []ImportConventionRule{
					{
						Rule:    "relative-internal-absolute-external",
						Autofix: true,
						Domains: []ImportConventionDomain{
							{
								Path:    "src/*",
								Enabled: true,
							},
							{
								Path:    "utils",
								Enabled: true,
							},
						},
					},
				},
			},
		},
	}

	// == src/auth domain ==
	authDir := filepath.Join(tempDir, "src", "auth")
	os.MkdirAll(authDir, 0755)

	authFile := filepath.Join(authDir, "file.ts")
	authFile2 := filepath.Join(authDir, "file2.ts") // we need a file that will be resolved

	authFileContent := `
		import authFile2 from "@/auth/file2.ts";
		import utilsFile1 from "../../utils/file";
		import usersFile1 from "../users/file";
	`

	os.WriteFile(authFile, []byte(authFileContent), 0644)
	os.WriteFile(authFile2, []byte(""), 0644)

	// == src/users domain ==
	usersDir := filepath.Join(tempDir, "src", "users")
	os.MkdirAll(usersDir, 0755)

	usersFile := filepath.Join(usersDir, "file.ts")
	usersFileContent := ""

	os.WriteFile(usersFile, []byte(usersFileContent), 0644)

	// == Utils domain ==
	settingsDir := filepath.Join(tempDir, "utils")
	os.MkdirAll(settingsDir, 0755)

	settingsFile := filepath.Join(settingsDir, "file.ts")
	settingsFileContent := ""

	os.WriteFile(settingsFile, []byte(settingsFileContent), 0644)

	result, err := ProcessConfig(&config, tempDir, "package.json", "tsconfig.json", true)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	// Check that we have a violation and it was fixed
	if len(result.RuleResults) == 0 {
		t.Fatalf("Expected at least 1 rule result")
	}

	ruleResult := result.RuleResults[0]

	if len(ruleResult.ImportConventionViolations) != 3 {
		t.Fatalf("Expected 3 violations, got %d", len(ruleResult.ImportConventionViolations))
	}

	// == should-be-relative + persist extension ==
	violation1 := ruleResult.ImportConventionViolations[0]

	if !strings.HasSuffix(violation1.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation1.FilePath)
	}

	if violation1.ViolationType != "should-be-relative" {
		t.Errorf("Expected violation type 'should-be-relative', got '%s'", violation1.ViolationType)
	}

	if violation1.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation1.Fix != nil && violation1.Fix.Text != "./file2.ts" {
		t.Errorf("Expected violation fix text to be './file2.ts', got '%s'", violation1.Fix.Text)
	}

	violation1fileAfterFix, _ := os.ReadFile(violation1.FilePath)

	if !strings.Contains(string(violation1fileAfterFix), `import authFile2 from "./file2.ts";`) {
		t.Errorf("Expected file content to be fixed to use relative import, got:\n%s", string(violation1fileAfterFix))
	}

	// == should-be-aliased - alias not defined in domains config, resolves to non base url TS alias  ==
	violation2 := ruleResult.ImportConventionViolations[1]

	if !strings.HasSuffix(violation2.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation2.FilePath)
	}

	if violation2.ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got '%s'", violation2.ViolationType)
	}

	if violation2.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation2.Fix != nil && violation2.Fix.Text != "@/utils/file" {
		t.Errorf("Expected violation fix text to be '@/utils/file', got '%s'", violation2.Fix.Text)
	}

	violation2fileAfterFix, _ := os.ReadFile(violation2.FilePath)

	if !strings.Contains(string(violation2fileAfterFix), `import utilsFile1 from "@/utils/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation2fileAfterFix))
	}

	// == should-be-aliased - alias not defined in domains config, resolves to base url TS alias  ==
	violation3 := ruleResult.ImportConventionViolations[2]

	if !strings.HasSuffix(violation3.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation3.FilePath)
	}

	if violation3.ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got '%s'", violation3.ViolationType)
	}

	if violation3.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation3.Fix != nil && violation3.Fix.Text != "src/users/file" {
		t.Errorf("Expected violation fix text to be 'src/users/file', got '%s'", violation3.Fix.Text)
	}

	violation3fileAfterFix, _ := os.ReadFile(violation3.FilePath)

	if !strings.Contains(string(violation3fileAfterFix), `import usersFile1 from "src/users/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation3fileAfterFix))
	}

}

func TestImportConventions_StringDomains_NoTsWildcardAlias(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "TestImportConventions_StringDomains_NoTsWildcardAlias")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create package.json
	packageJson := `{
		"name": "test-project",
		"dependencies": {}
	}`
	os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJson), 0644)

	// Create tsconfig.json with alias
	tsconfig := `{
		"compilerOptions": {
			"paths": {
				"@/auth/*": ["./src/auth/*"],
				"@/utils/*": ["./utils/*"],
			}
		}
	}`

	os.WriteFile(filepath.Join(tempDir, "tsconfig.json"), []byte(tsconfig), 0644)

	config := RevDepConfig{
		ConfigVersion: "1.0",
		Rules: []Rule{
			{
				Path: ".",
				ImportConventions: []ImportConventionRule{
					{
						Rule:    "relative-internal-absolute-external",
						Autofix: true,
						Domains: []ImportConventionDomain{
							{
								Path:    "src/*",
								Enabled: true,
							},
							{
								Path:    "utils",
								Enabled: true,
							},
						},
					},
				},
			},
		},
	}

	// == src/auth domain ==
	authDir := filepath.Join(tempDir, "src", "auth")
	os.MkdirAll(authDir, 0755)

	authFile := filepath.Join(authDir, "file.ts")
	authFile2 := filepath.Join(authDir, "file2.ts") // we need a file that will be resolved

	authFileContent := `
		import authFile2 from "@/auth/file2.ts";
		import utilsFile1 from "../../utils/file";
		import usersFile1 from "../users/file";
	`

	os.WriteFile(authFile, []byte(authFileContent), 0644)
	os.WriteFile(authFile2, []byte(""), 0644)

	// == src/users domain ==
	usersDir := filepath.Join(tempDir, "src", "users")
	os.MkdirAll(usersDir, 0755)

	usersFile := filepath.Join(usersDir, "file.ts")
	usersFileContent := ""

	os.WriteFile(usersFile, []byte(usersFileContent), 0644)

	// == Utils domain ==
	settingsDir := filepath.Join(tempDir, "utils")
	os.MkdirAll(settingsDir, 0755)

	settingsFile := filepath.Join(settingsDir, "file.ts")
	settingsFileContent := ""

	os.WriteFile(settingsFile, []byte(settingsFileContent), 0644)

	result, err := ProcessConfig(&config, tempDir, "package.json", "tsconfig.json", true)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	// Check that we have a violation and it was fixed
	if len(result.RuleResults) == 0 {
		t.Fatalf("Expected at least 1 rule result")
	}

	ruleResult := result.RuleResults[0]

	if len(ruleResult.ImportConventionViolations) != 3 {
		t.Fatalf("Expected 3 violations, got %d", len(ruleResult.ImportConventionViolations))
	}

	// == should-be-relative + persist extension ==
	violation1 := ruleResult.ImportConventionViolations[0]

	if !strings.HasSuffix(violation1.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation1.FilePath)
	}

	if violation1.ViolationType != "should-be-relative" {
		t.Errorf("Expected violation type 'should-be-relative', got '%s'", violation1.ViolationType)
	}

	if violation1.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation1.Fix != nil && violation1.Fix.Text != "./file2.ts" {
		t.Errorf("Expected violation fix text to be './file2.ts', got '%s'", violation1.Fix.Text)
	}

	violation1fileAfterFix, _ := os.ReadFile(violation1.FilePath)

	if !strings.Contains(string(violation1fileAfterFix), `import authFile2 from "./file2.ts";`) {
		t.Errorf("Expected file content to be fixed to use relative import, got:\n%s", string(violation1fileAfterFix))
	}

	// == should-be-aliased - alias not defined in domains config, resolves to non base url TS alias  ==
	violation2 := ruleResult.ImportConventionViolations[1]

	if !strings.HasSuffix(violation2.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation2.FilePath)
	}

	if violation2.ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got '%s'", violation2.ViolationType)
	}

	if violation2.Fix == nil {
		t.Errorf("Expected violation fix to exist, got nil")
	}

	if violation2.Fix != nil && violation2.Fix.Text != "@/utils/file" {
		t.Errorf("Expected violation fix text to be '@/utils/file', got '%s'", violation2.Fix.Text)
	}

	violation2fileAfterFix, _ := os.ReadFile(violation2.FilePath)

	if !strings.Contains(string(violation2fileAfterFix), `import utilsFile1 from "@/utils/file";`) {
		t.Errorf("Expected file content to be fixed to use aliased import, got:\n%s", string(violation2fileAfterFix))
	}

	// == should-be-aliased - alias not defined in domains config, no alias to resolve to ==
	violation3 := ruleResult.ImportConventionViolations[2]

	if !strings.HasSuffix(violation3.FilePath, "src/auth/file.ts") {
		t.Errorf("Expected violation file path to be 'src/auth/file.ts', got '%s'", violation3.FilePath)
	}

	if violation3.ViolationType != "should-be-aliased" {
		t.Errorf("Expected violation type 'should-be-aliased', got '%s'", violation3.ViolationType)
	}

	if violation3.Fix != nil {
		t.Errorf("Expected violation fix to be nil, got %v", violation3.Fix.Text)
	}

	violation3fileAfterFix, _ := os.ReadFile(violation3.FilePath)

	if !strings.Contains(string(violation3fileAfterFix), `import usersFile1 from "../users/file";`) {
		t.Errorf("Expected file content to not be fixed to use aliased import, got:\n%s", string(violation3fileAfterFix))
	}

}
