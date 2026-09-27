package module

import "strings"

const nodePrefix = "node:"

const bunPrefix = "bun:"

var nodeBuiltInModules = []string{
	"_debug_agent",
	"_debugger",
	"_http_agent",
	"_http_client",
	"_http_common",
	"_http_incoming",
	"_http_outgoing",
	"_http_server",
	"_linklist",
	"_stream_duplex",
	"_stream_passthrough",
	"_stream_readable",
	"_stream_transform",
	"_stream_wrap",
	"_stream_writable",
	"_tls_common",
	"_tls_legacy",
	"_tls_wrap",
	"assert",
	"async_hooks",
	"buffer",
	"buffer_ieee754",
	"child_process",
	"cluster",
	"console",
	"constants",
	"crypto",
	"dgram",
	"diagnostics_channel",
	"dns",
	"domain",
	"events",
	"freelist",
	"fs",
	"http",
	"http2",
	"https",
	"inspector",
	"module",
	"net",
	"os",
	"path",
	"perf_hooks",
	"process",
	"punycode",
	"querystring",
	"readline",
	"repl",
	"smalloc",
	"stream",
	"string_decoder",
	"sys",
	"timers",
	"tls",
	"trace_events",
	"tty",
	"url",
	"util",
	"v8",
	"vm",
	"wasi",
	"worker_threads",
	"zlib",
}

var bunBuiltInModules = []string{
	"bun",
}

var bareBuiltInModules = func() map[string]bool {
	names := make(map[string]bool, len(nodeBuiltInModules)+len(bunBuiltInModules))
	for _, name := range nodeBuiltInModules {
		names[name] = true
	}
	for _, name := range bunBuiltInModules {
		names[name] = true
	}
	return names
}()

func IsBuiltInModule(request string) bool {
	if strings.HasPrefix(request, nodePrefix) || strings.HasPrefix(request, bunPrefix) {
		return true
	}
	name := GetNodeModuleName(request)
	if request == name+"/" {
		return false
	}
	return bareBuiltInModules[name]
}
