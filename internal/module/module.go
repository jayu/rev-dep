package module

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/jsonc"
)

var BuiltInModules = map[string]bool{
	"node:assert":              true,
	"assert":                   true,
	"node:async_hooks":         true,
	"async_hooks":              true,
	"node:buffer":              true,
	"buffer":                   true,
	"node:child_process":       true,
	"child_process":            true,
	"node:cluster":             true,
	"cluster":                  true,
	"node:console":             true,
	"console":                  true,
	"node:constants":           true,
	"constants":                true,
	"node:crypto":              true,
	"crypto":                   true,
	"node:dgram":               true,
	"dgram":                    true,
	"node:diagnostics_channel": true,
	"diagnostics_channel":      true,
	"node:dns":                 true,
	"dns":                      true,
	"node:domain":              true,
	"domain":                   true,
	"node:events":              true,
	"events":                   true,
	"node:fs":                  true,
	"fs":                       true,
	"node:http":                true,
	"http":                     true,
	"node:http2":               true,
	"http2":                    true,
	"node:https":               true,
	"https":                    true,
	"node:inspector":           true,
	"inspector":                true,
	"node:module":              true,
	"module":                   true,
	"node:net":                 true,
	"net":                      true,
	"node:os":                  true,
	"os":                       true,
	"node:path":                true,
	"path":                     true,
	"node:perf_hooks":          true,
	"perf_hooks":               true,
	"node:process":             true,
	"process":                  true,
	"node:querystring":         true,
	"querystring":              true,
	"node:quic":                true,
	"node:readline":            true,
	"readline":                 true,
	"node:repl":                true,
	"repl":                     true,
	"node:sea":                 true,
	"node:sqlite":              true,
	"node:stream":              true,
	"stream":                   true,
	"node:string_decoder":      true,
	"string_decoder":           true,
	"node:test":                true,
	"node:timers":              true,
	"timers":                   true,
	"node:tls":                 true,
	"tls":                      true,
	"node:trace_events":        true,
	"trace_events":             true,
	"node:tty":                 true,
	"tty":                      true,
	"node:url":                 true,
	"url":                      true,
	"node:util":                true,
	"util":                     true,
	"node:v8":                  true,
	"v8":                       true,
	"node:vm":                  true,
	"vm":                       true,
	"node:wasi":                true,
	"wasi":                     true,
	"node:worker_threads":      true,
	"worker_threads":           true,
	"node:zlib":                true,
	"zlib":                     true,
}

func GetNodeModuleName(request string) string {
	splitCount := 2
	if strings.HasPrefix(request, "@") {
		splitCount = 3
	}
	parts := strings.SplitN(request, "/", splitCount)
	return strings.Join(parts[:splitCount-1], "/")
}

func GetNodeModulesFromPkgJson(packageJsonContent []byte) (map[string]bool, map[string]bool) {
	packageJsonContent = jsonc.ToJSON(packageJsonContent)

	var rawPackageJson map[string]map[string]string

	err := json.Unmarshal(packageJsonContent, &rawPackageJson)

	if err != nil {
		// fmt.Printf("Failed to parse package json : %s\n", err)
	}

	deps := map[string]bool{}
	devDeps := map[string]bool{}

	rawDeps, ok := rawPackageJson["dependencies"]

	if ok {
		for dep := range rawDeps {
			deps[dep] = true
		}
	}
	rawDevDeps, ok2 := rawPackageJson["devDependencies"]

	if ok2 {
		for dep := range rawDevDeps {
			devDeps[dep] = true
		}
	}

	return deps, devDeps
}

func IsValidNodeModuleName(name string) bool {
	// There are more restrictions on node module name than starting with dot, but for now we just check against that
	return !strings.HasPrefix(name, ".")
}
