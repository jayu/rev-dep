package resolve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseTsConfig_ExtendsArray(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name: "single parent",
			files: map[string]string{
				"tsconfig.json": `{"extends":["./base.json"]}`,
				"base.json":     `{"compilerOptions":{"paths":{"@base/*":["src/*"]},"types":["node"]}}`,
			},
			want: `{"compilerOptions":{"paths":{"@base/*":["src/*"]},"types":["node"]}}`,
		},
		{
			name: "later parents override earlier parents and child overrides both",
			files: map[string]string{
				"tsconfig.json": `{
					"extends":["./first.json","./second.json"],
					"include":["child"],
					"compilerOptions":{
						"baseUrl":"./child",
						"strict":false,
						"paths":{"@shared/*":["child/*"],"@child/*":["src/*"]},
						"types":["child","shared"]
					}
				}`,
				"first.json": `{
					"include":["first"],
					"compilerOptions":{
						"baseUrl":"./first","strict":true,"target":"ES2020","module":"CommonJS",
						"paths":{"@first/*":["first/*"],"@shared/*":["first/shared/*"],"@parent/*":["first/parent/*"]},
						"types":["first","shared"]
					}
				}`,
				"second.json": `{
					"include":["second"],
					"exclude":["dist"],
					"compilerOptions":{
						"baseUrl":"./second","strict":true,"target":"ES2022",
						"paths":{"@second/*":["second/*"],"@shared/*":["second/shared/*"],"@parent/*":["second/parent/*"]},
						"types":["second","shared"]
					}
				}`,
			},
			want: `{
				"include":["child"],"exclude":["dist"],
				"compilerOptions":{
					"baseUrl":"./child","strict":false,"target":"ES2022","module":"CommonJS",
					"paths":{"@shared/*":["child/*"],"@child/*":["src/*"]},
					"types":["child","shared"]
				}
			}`,
		},
		{
			name: "parent order determines precedence without child compiler options",
			files: map[string]string{
				"tsconfig.json": `{"extends":["./second.json","./first.json"]}`,
				"first.json":    `{"include":["first"],"compilerOptions":{"baseUrl":"./first","strict":false}}`,
				"second.json":   `{"include":["second"],"compilerOptions":{"baseUrl":"./second","strict":true,"target":"ES2022"}}`,
			},
			want: `{"include":["first"],"compilerOptions":{"baseUrl":"./first","strict":false,"target":"ES2022"}}`,
		},
		{
			name: "nested arrays and strings rebase paths from each parent directory",
			files: map[string]string{
				"tsconfig.json": `{"extends":["./configs/first.json","./configs/nested/second.json"]}`,
				"configs/first.json": `{
					"extends":["./common.json","./extra.json"],
					"compilerOptions":{"paths":{"@first/*":["../src/first/*"]}}
				}`,
				"configs/common.json": `{"compilerOptions":{"paths":{"@common/*":["../src/common/*"]}}}`,
				"configs/extra.json":  `{"compilerOptions":{"paths":{"@extra/*":["../src/extra/*"]}}}`,
				"configs/nested/second.json": `{
					"extends":"../third.json",
					"compilerOptions":{"strict":true}
				}`,
				"configs/third.json": `{"compilerOptions":{"paths":{"@third/*":["../src/third/*"]}}}`,
			},
			want: `{"compilerOptions":{"strict":true,"paths":{"@third/*":["src/third/*"]}}}`,
		},
		{
			name: "package parents support direct files and package main",
			files: map[string]string{
				"tsconfig.json": `{"extends":["@tsconfig/node26/tsconfig.json","@tsconfig/strictest"]}`,
				"node_modules/@tsconfig/node26/tsconfig.json": `{
					"compilerOptions":{"target":"ES2026","paths":{"@node/*":["./node/*"]}}
				}`,
				"node_modules/@tsconfig/strictest/package.json": `{"main":"configs/base.json"}`,
				"node_modules/@tsconfig/strictest/configs/base.json": `{
					"compilerOptions":{"strict":true,"paths":{"@strict/*":["../strict/*"]}}
				}`,
			},
			want: `{"compilerOptions":{"target":"ES2026","strict":true,"paths":{
				"@strict/*":["node_modules/@tsconfig/strictest/strict/*"]
			}}}`,
		},
		{
			name: "shared ancestor is resolved independently for each branch",
			files: map[string]string{
				"tsconfig.json": `{"extends":["./first.json","./second.json"]}`,
				"first.json": `{
					"extends":"./configs/shared.json",
					"compilerOptions":{"strict":false,"paths":{"@shared/*":["first/*"],"@first/*":["first/*"]}}
				}`,
				"second.json": `{
					"extends":["./configs/shared.json"],
					"compilerOptions":{"target":"ES2022"}
				}`,
				"configs/shared.json": `{"compilerOptions":{"strict":true,"paths":{"@shared/*":["./shared/*"]}}}`,
			},
			want: `{"compilerOptions":{"strict":true,"target":"ES2022","paths":{"@shared/*":["configs/shared/*"]}}}`,
		},
		{
			name: "repeated parent can override an intervening parent",
			files: map[string]string{
				"tsconfig.json": `{"extends":["./first.json","./second.json","./first.json"]}`,
				"first.json":    `{"compilerOptions":{"strict":false,"types":["first"]}}`,
				"second.json":   `{"compilerOptions":{"strict":true,"types":["second"]}}`,
			},
			want: `{"compilerOptions":{"strict":false,"types":["first"]}}`,
		},
		{
			name: "unresolvable parents do not discard valid siblings",
			files: map[string]string{
				"tsconfig.json": `{
					"extends":["./missing.json","./first.json","./malformed.json","./null.json","./second.json"],
					"compilerOptions":{"types":["child"]}
				}`,
				"first.json":     `{"compilerOptions":{"strict":true,"types":["first"]}}`,
				"malformed.json": `{"compilerOptions":`,
				"null.json":      `null`,
				"second.json":    `{"compilerOptions":{"target":"ES2022","types":["second"]}}`,
			},
			want: `{"compilerOptions":{"strict":true,"target":"ES2022","types":["child"]}}`,
		},
		{
			name: "invalid entries do not discard valid siblings",
			files: map[string]string{
				"tsconfig.json": `{"extends":[null,"./first.json",12,"",{},[],"  ","./second.json",false]}`,
				"first.json":    `{"compilerOptions":{"strict":true}}`,
				"second.json":   `{"compilerOptions":{"target":"ES2022"}}`,
			},
			want: `{"compilerOptions":{"strict":true,"target":"ES2022"}}`,
		},
		{
			name: "empty array keeps child settings",
			files: map[string]string{
				"tsconfig.json": `{"extends":[],"compilerOptions":{"strict":true}}`,
			},
			want: `{"compilerOptions":{"strict":true}}`,
		},
		{
			name: "empty array ensures compiler options exist",
			files: map[string]string{
				"tsconfig.json": `{"extends":[]}`,
			},
			want: `{"compilerOptions":{}}`,
		},
		{
			name: "circular branch keeps other parents and child settings",
			files: map[string]string{
				"tsconfig.json": `{"extends":["./first.json","./second.json"],"compilerOptions":{"types":["child"]}}`,
				"first.json":    `{"extends":["./loop.json"],"compilerOptions":{"strict":true,"types":["first"]}}`,
				"loop.json":     `{"extends":"./first.json","compilerOptions":{"target":"ES2022","types":["loop"]}}`,
				"second.json":   `{"compilerOptions":{"module":"ESNext","types":["second"]}}`,
			},
			want: `{"compilerOptions":{"strict":true,"target":"ES2022","module":"ESNext","types":["child"]}}`,
		},
		{
			name: "cycle back to root does not inherit other root parents into the branch",
			files: map[string]string{
				"tsconfig.json": `{"extends":["./first.json","./second.json"],"compilerOptions":{"types":["child"]}}`,
				"first.json":    `{"extends":"./tsconfig.json","compilerOptions":{"strict":true,"types":["first"]}}`,
				"second.json":   `{"compilerOptions":{"target":"ES2022","types":["second"]}}`,
			},
			want: `{"compilerOptions":{"strict":true,"target":"ES2022","types":["child"]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertTsConfigFiles(t, tt.files, tt.want)
		})
	}
}

func TestParseTsConfig_CompilerOptionReplacement(t *testing.T) {
	tests := []struct {
		name    string
		options string
		want    string
	}{
		{
			name:    "omitted options inherit",
			options: `{}`,
			want:    `{"strict":true,"paths":{"@base/*":["src/base/*"]},"types":["base","shared"]}`,
		},
		{
			name:    "paths replace the whole map",
			options: `{"paths":{"@later/*":["src/later/*"]}}`,
			want:    `{"strict":true,"paths":{"@later/*":["src/later/*"]},"types":["base","shared"]}`,
		},
		{
			name:    "empty paths clear inherited aliases",
			options: `{"paths":{}}`,
			want:    `{"strict":true,"paths":{},"types":["base","shared"]}`,
		},
		{
			name:    "types replace the whole array",
			options: `{"types":["later","shared"]}`,
			want:    `{"strict":true,"paths":{"@base/*":["src/base/*"]},"types":["later","shared"]}`,
		},
		{
			name:    "empty types clear inherited entries",
			options: `{"types":[]}`,
			want:    `{"strict":true,"paths":{"@base/*":["src/base/*"]},"types":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, mode := range []string{"string", "array child", "array parent"} {
				t.Run(mode, func(t *testing.T) {
					files := map[string]string{
						"base.json": `{"compilerOptions":{"strict":true,"paths":{"@base/*":["src/base/*"]},"types":["base","shared"]}}`,
					}
					switch mode {
					case "string":
						files["tsconfig.json"] = `{"extends":"./base.json","compilerOptions":` + tt.options + `}`
					case "array child":
						files["tsconfig.json"] = `{"extends":["./base.json"],"compilerOptions":` + tt.options + `}`
					case "array parent":
						files["tsconfig.json"] = `{"extends":["./base.json","./later.json"]}`
						files["later.json"] = `{"compilerOptions":` + tt.options + `}`
					}
					assertTsConfigFiles(t, files, `{"compilerOptions":`+tt.want+`}`)
				})
			}
		})
	}
}

func assertTsConfigFiles(t *testing.T, files map[string]string, expected string) {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		filename := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	merged, err := ParseTsConfig(filepath.Join(root, "tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]interface{}
	if err := json.Unmarshal(merged, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged config = %s\nwant %s", merged, expected)
	}
}
