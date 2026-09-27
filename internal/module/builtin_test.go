package module

import "testing"

func TestIsBuiltInModule(t *testing.T) {
	builtIn := []string{
		"fs", "node:fs", "fs/promises", "node:fs/promises", "path", "node:path/posix",
		"punycode", "sys", "_http_agent", "_stream_readable",
		"node:test", "node:test/reporters", "node:sqlite", "node:sea", "node:quic", "node:ffi", "node:vfs",
		"_stream_wrap", "node:_stream_duplex", "_tls_legacy", "_debugger", "_debug_agent", "_linklist",
		"freelist", "smalloc", "buffer_ieee754",
		"bun", "bun:bundle", "bun:ffi", "bun:jsc", "bun:sqlite", "bun:test",
		"node:future_module", "node:future_module/sub", "bun:future_module", "bun:wrap", "node:fss", "node:buffer/",
	}
	for _, request := range builtIn {
		if !IsBuiltInModule(request) {
			t.Errorf("%q should be a built-in module", request)
		}
	}

	notBuiltIn := []string{
		"test", "sqlite", "sea", "quic", "ffi", "vfs", "test/reporters",
		"ws", "undici",
		"buffer/", "punycode/", "string_decoder/",
		"fs-extra", "fss", "bunyan", "@types/node", "node", "deno", "node-inspect", "./fs", "",
		"npm:lodash", "jsr:@std/path",
	}
	for _, request := range notBuiltIn {
		if IsBuiltInModule(request) {
			t.Errorf("%q should not be a built-in module", request)
		}
	}
}
