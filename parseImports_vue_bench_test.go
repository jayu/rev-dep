package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func makeSimpleVueFixture() []byte {
	return []byte(`<template>
  <button class="btn">{{ label }}</button>
</template>
<script setup lang="ts">
import { ref } from "vue"
import { format } from "./format"

const label = ref(format("Hello"))
</script>
<style scoped>
.btn { padding: 8px 12px; }
</style>
`)
}

func makeComplexVueFixture500Lines() []byte {
	var sb strings.Builder
	sb.Grow(64 * 1024)

	sb.WriteString("<template>\n")
	for i := 0; i < 120; i++ {
		sb.WriteString(fmt.Sprintf("  <li class=\"row\">{{ items[%d] }}</li>\n", i))
	}
	sb.WriteString("</template>\n")
	sb.WriteString("<script setup lang=\"ts\">\n")
	sb.WriteString("import { computed, ref } from \"vue\"\n")
	sb.WriteString("import { normalize } from \"./normalize\"\n")
	for i := 0; i < 420; i++ {
		sb.WriteString(fmt.Sprintf("import { helper%d } from \"./helpers/h%d\"\n", i, i))
	}
	sb.WriteString("const base = ref(\"value\")\n")
	for i := 0; i < 40; i++ {
		sb.WriteString(fmt.Sprintf("const v%d = helper%d(base.value)\n", i, i))
	}
	sb.WriteString("const result = computed(() => normalize(v0))\n")
	sb.WriteString("void result\n")
	sb.WriteString("</script>\n")
	sb.WriteString("<style scoped>\n")
	for i := 0; i < 60; i++ {
		sb.WriteString(fmt.Sprintf(".row-%d { color: #333; }\n", i))
	}
	sb.WriteString("</style>\n")

	return []byte(sb.String())
}

func normalizeVueSFCForParsingBaseline(code []byte) []byte {
	masked := make([]byte, len(code))
	for i, b := range code {
		switch b {
		case '\n', '\r':
			masked[i] = b
		default:
			masked[i] = ' '
		}
	}

	lower := bytes.ToLower(code)
	searchFrom := 0
	for searchFrom < len(lower) {
		scriptOpenRel := bytes.Index(lower[searchFrom:], []byte("<script"))
		if scriptOpenRel == -1 {
			break
		}
		scriptOpen := searchFrom + scriptOpenRel

		tagEndRel := bytes.IndexByte(lower[scriptOpen:], '>')
		if tagEndRel == -1 {
			break
		}
		contentStart := scriptOpen + tagEndRel + 1

		scriptCloseRel := bytes.Index(lower[contentStart:], []byte("</script>"))
		if scriptCloseRel == -1 {
			break
		}
		contentEnd := contentStart + scriptCloseRel

		copy(masked[contentStart:contentEnd], code[contentStart:contentEnd])
		searchFrom = contentEnd + len("</script>")
	}

	return masked
}

func BenchmarkNormalizeVueSFCOptimized_Simple(b *testing.B) {
	code := makeSimpleVueFixture()
	b.ReportAllocs()
	b.SetBytes(int64(len(code)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeVueSFCForParsing(code)
	}
}

func BenchmarkNormalizeVueSFCBaseline_Simple(b *testing.B) {
	code := makeSimpleVueFixture()
	b.ReportAllocs()
	b.SetBytes(int64(len(code)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeVueSFCForParsingBaseline(code)
	}
}

func BenchmarkNormalizeVueSFCOptimized_Complex500Lines(b *testing.B) {
	code := makeComplexVueFixture500Lines()
	b.ReportAllocs()
	b.SetBytes(int64(len(code)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeVueSFCForParsing(code)
	}
}

func BenchmarkNormalizeVueSFCBaseline_Complex500Lines(b *testing.B) {
	code := makeComplexVueFixture500Lines()
	b.ReportAllocs()
	b.SetBytes(int64(len(code)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeVueSFCForParsingBaseline(code)
	}
}
