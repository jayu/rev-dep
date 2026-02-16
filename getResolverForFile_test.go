package main

import (
	"fmt"
	"slices"
	"testing"
)

func TestGetResolverForFile_NestedMonorepo(t *testing.T) {
	// Test case for nested monorepo packages
	// Structure:
	// /workspace/
	//   package.json
	//   packages/
	//     core/
	//       package.json
	//       src/
	//         index.ts
	//     core/utils/
	//       package.json
	//       src/
	//         helper.ts
	//     ui/
	//       package.json
	//       src/
	//         button.ts

	// Create a mock monorepo context
	monorepoCtx := &MonorepoContext{
		WorkspaceRoot: "/workspace",
		PackageToPath: map[string]string{
			"core":  "/workspace/packages/core",
			"utils": "/workspace/packages/core/utils", // More specific path
			"ui":    "/workspace/packages/ui",
		},
	}

	// Create resolver manager
	rm := &ResolverManager{
		monorepoContext: monorepoCtx,
		subpackageResolvers: []SubpackageResolver{
			{PkgPath: "/workspace/packages/core", Resolver: &ModuleResolver{}},
			{PkgPath: "/workspace/packages/core/utils", Resolver: &ModuleResolver{}}, // More specific
			{PkgPath: "/workspace/packages/ui", Resolver: &ModuleResolver{}},
		},
		rootResolver: &ModuleResolver{},
	}

	// Sort by path length descending (as done in NewResolverManager)
	// This ensures most specific paths are checked first
	sortedResolvers := []SubpackageResolver{
		{PkgPath: "/workspace/packages/core/utils", Resolver: &ModuleResolver{}}, // Longest path
		{PkgPath: "/workspace/packages/core", Resolver: &ModuleResolver{}},       // Medium path
		{PkgPath: "/workspace/packages/ui", Resolver: &ModuleResolver{}},         // Medium path
	}
	rm.subpackageResolvers = sortedResolvers

	tests := []struct {
		name     string
		filePath string
		expected string // Expected resolver path
	}{
		{
			name:     "File in most specific nested package",
			filePath: "/workspace/packages/core/utils/src/helper.ts",
			expected: "/workspace/packages/core/utils",
		},
		{
			name:     "File in parent package",
			filePath: "/workspace/packages/core/src/index.ts",
			expected: "/workspace/packages/core",
		},
		{
			name:     "File in sibling package",
			filePath: "/workspace/packages/ui/src/button.ts",
			expected: "/workspace/packages/ui",
		},
		{
			name:     "File in workspace root",
			filePath: "/workspace/some-root-file.ts",
			expected: "root", // Should return root resolver
		},
		{
			name:     "File outside all packages",
			filePath: "/some/other/path/file.ts",
			expected: "root", // Should return root resolver
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := rm.GetResolverForFile(tt.filePath)

			if tt.expected == "root" {
				if resolver != rm.rootResolver {
					t.Errorf("GetResolverForFile(%s) = %v, want root resolver", tt.filePath, resolver)
				}
			} else {
				found := false
				for _, subPkg := range rm.subpackageResolvers {
					if resolver == subPkg.Resolver && subPkg.PkgPath == tt.expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("GetResolverForFile(%s) did not return resolver for path %s", tt.filePath, tt.expected)
				}
			}
		})
	}
}

func TestGetResolverForFile_SortingDeterminism(t *testing.T) {
	// Test that resolver selection is deterministic regardless of insertion order
	monorepoCtx := &MonorepoContext{
		WorkspaceRoot: "/workspace",
		PackageToPath: map[string]string{
			"a": "/workspace/packages/a",
			"b": "/workspace/packages/b/c", // More specific
			"c": "/workspace/packages/b",
		},
	}

	// Test with different insertion orders
	orders := [][]SubpackageResolver{
		// Order 1: a, b/c, b
		{
			{PkgPath: "/workspace/packages/a", Resolver: &ModuleResolver{}},
			{PkgPath: "/workspace/packages/b/c", Resolver: &ModuleResolver{}},
			{PkgPath: "/workspace/packages/b", Resolver: &ModuleResolver{}},
		},
		// Order 2: b, a, b/c
		{
			{PkgPath: "/workspace/packages/b", Resolver: &ModuleResolver{}},
			{PkgPath: "/workspace/packages/a", Resolver: &ModuleResolver{}},
			{PkgPath: "/workspace/packages/b/c", Resolver: &ModuleResolver{}},
		},
		// Order 3: b/c, b, a
		{
			{PkgPath: "/workspace/packages/b/c", Resolver: &ModuleResolver{}},
			{PkgPath: "/workspace/packages/b", Resolver: &ModuleResolver{}},
			{PkgPath: "/workspace/packages/a", Resolver: &ModuleResolver{}},
		},
	}

	testFilePath := "/workspace/packages/b/c/src/file.ts"

	for i, order := range orders {
		t.Run(fmt.Sprintf("Order_%d", i+1), func(t *testing.T) {
			rm := &ResolverManager{
				monorepoContext:     monorepoCtx,
				subpackageResolvers: order,
				rootResolver:        &ModuleResolver{},
			}

			// Sort by path length descending (as done in NewResolverManager)
			slices.SortFunc(rm.subpackageResolvers, func(a, b SubpackageResolver) int {
				return len(b.PkgPath) - len(a.PkgPath)
			})

			resolver := rm.GetResolverForFile(testFilePath)

			// Should always return the most specific resolver (/workspace/packages/b/c)
			expectedResolver := rm.subpackageResolvers[0].Resolver // After sorting, this should be the longest path
			if resolver != expectedResolver {
				t.Errorf("GetResolverForFile returned unexpected resolver for order %d", i+1)
			}
		})
	}
}

func TestGetResolverForFile_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		subpackagePaths []string
		filePath        string
		expectedPath    string
		expectRoot      bool
	}{
		{
			name:            "Empty subpackage resolvers",
			subpackagePaths: []string{},
			filePath:        "/any/path/file.ts",
			expectRoot:      true,
		},
		{
			name:            "Exact path match",
			subpackagePaths: []string{"/workspace/packages/core"},
			filePath:        "/workspace/packages/core/index.ts",
			expectedPath:    "/workspace/packages/core",
		},
		{
			name:            "Prefix but not full path match",
			subpackagePaths: []string{"/workspace/packages/core"},
			filePath:        "/workspace/packages/core-extra/file.ts",
			expectRoot:      true, // Should not match sibling path with shared prefix
		},
		{
			name: "Multiple potential matches, choose most specific",
			subpackagePaths: []string{
				"/workspace/packages",
				"/workspace/packages/core",
				"/workspace/packages/core/utils",
			},
			filePath:     "/workspace/packages/core/src/file.ts",
			expectedPath: "/workspace/packages/core", // Most specific that matches
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var subpackageResolvers []SubpackageResolver
			for _, path := range tt.subpackagePaths {
				subpackageResolvers = append(subpackageResolvers, SubpackageResolver{
					PkgPath:  path,
					Resolver: &ModuleResolver{},
				})
			}

			// Sort by path length descending
			slices.SortFunc(subpackageResolvers, func(a, b SubpackageResolver) int {
				return len(b.PkgPath) - len(a.PkgPath)
			})

			rm := &ResolverManager{
				subpackageResolvers: subpackageResolvers,
				rootResolver:        &ModuleResolver{},
			}

			resolver := rm.GetResolverForFile(tt.filePath)

			if tt.expectRoot {
				if resolver != rm.rootResolver {
					t.Errorf("Expected root resolver, got %v", resolver)
				}
			} else {
				found := false
				for _, subPkg := range rm.subpackageResolvers {
					if resolver == subPkg.Resolver && subPkg.PkgPath == tt.expectedPath {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected resolver for path %s, got %v", tt.expectedPath, resolver)
				}
			}
		})
	}
}
