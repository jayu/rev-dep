package parser

import (
	"fmt"
	"strings"
	"testing"
)

func makeSimpleSvelteFixture() []byte {
	return []byte(`<script lang="ts">
  export let title = "Hello";
  import { format } from "./format";
  const message = format(title);
  void message;
</script>

<h1>{title}</h1>
`)
}

func makeComplexSvelteFixture500Lines() []byte {
	var sb strings.Builder
	sb.Grow(64 * 1024)

	sb.WriteString("<script context=\"module\" lang=\"ts\">\n")
	for i := 0; i < 120; i++ {
		sb.WriteString(fmt.Sprintf("import { m%d } from \"./module/m%d\";\n", i, i))
	}
	sb.WriteString("export const moduleVersion = m0;\n")
	sb.WriteString("</script>\n")

	sb.WriteString("<script lang=\"ts\">\n")
	for i := 0; i < 260; i++ {
		sb.WriteString(fmt.Sprintf("export    \n\tlet prop%d = \"p%d\";\n", i, i))
	}
	for i := 0; i < 160; i++ {
		sb.WriteString(fmt.Sprintf("import { helper%d } from \"./helpers/h%d\";\n", i, i))
	}
	for i := 0; i < 40; i++ {
		sb.WriteString(fmt.Sprintf("const v%d = helper%d(prop0);\n", i, i))
	}
	sb.WriteString("void v0;\n")
	sb.WriteString("</script>\n")

	sb.WriteString("<main>\n")
	for i := 0; i < 120; i++ {
		sb.WriteString(fmt.Sprintf("  <p>{prop%d}</p>\n", i))
	}
	sb.WriteString("</main>\n")

	return []byte(sb.String())
}

func BenchmarkNormalizeSvelte_Simple(b *testing.B) {
	code := makeSimpleSvelteFixture()
	b.ReportAllocs()
	b.SetBytes(int64(len(code)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeSvelteForParsing(code)
	}
}

func BenchmarkNormalizeSvelte_Complex500Lines(b *testing.B) {
	code := makeComplexSvelteFixture500Lines()
	b.ReportAllocs()
	b.SetBytes(int64(len(code)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeSvelteForParsing(code)
	}
}
