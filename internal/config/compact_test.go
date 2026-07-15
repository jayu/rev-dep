package config

import "testing"

func TestCompactConfigText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "enabled-only object folds to boolean",
			in:   `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":{"enabled":true}}]}`,
			want: `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":true}]}`,
		},
		{
			name: "disabled-only object folds to false",
			in:   `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":{"enabled":false}}]}`,
			want: `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":false}]}`,
		},
		{
			name: "empty object folds to true",
			in:   `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":{}}]}`,
			want: `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":true}]}`,
		},
		{
			name: "redundant enabled true dropped when other options present",
			in:   `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":{"enabled":true,"validEntryPoints":["a.ts"]}}]}`,
			want: `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":{"validEntryPoints":["a.ts"]}}]}`,
		},
		{
			name: "disabled object with options left unchanged",
			in:   `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":{"enabled":false,"validEntryPoints":["a.ts"]}}]}`,
			want: `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":{"enabled":false,"validEntryPoints":["a.ts"]}}]}`,
		},
		{
			name: "array element never folds to bare boolean",
			in:   `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":[{"enabled":true},{"enabled":true,"validEntryPoints":["a.ts"]}]}]}`,
			want: `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":[{"enabled":true},{"validEntryPoints":["a.ts"]}]}]}`,
		},
		{
			name: "already boolean left unchanged",
			in:   `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":true}]}`,
			want: `{"configVersion":"1.11","rules":[{"path":"src","orphanFilesDetection":true}]}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := CompactConfigText([]byte(c.in))
			if err != nil {
				t.Fatalf("CompactConfigText: %v", err)
			}
			if string(got) != c.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}

func TestCompactConfigText_PreservesCommentsAndFormatting(t *testing.T) {
	in := `{
  // top-level comment
  "configVersion": "1.11",
  "rules": [
    {
      "path": "src",
      // detector below should fold
      "circularImportsDetection": { "enabled": true },
      "orphanFilesDetection": {
        "enabled": true, // redundant, should drop
        "validEntryPoints": ["index.ts"]
      }
    }
  ]
}`
	want := `{
  // top-level comment
  "configVersion": "1.11",
  "rules": [
    {
      "path": "src",
      // detector below should fold
      "circularImportsDetection": true,
      "orphanFilesDetection": {
        "validEntryPoints": ["index.ts"]
      }
    }
  ]
}`
	got, err := CompactConfigText([]byte(in))
	if err != nil {
		t.Fatalf("CompactConfigText: %v", err)
	}
	if string(got) != want {
		t.Fatalf("got:\n%s\n\nwant:\n%s", got, want)
	}
}
