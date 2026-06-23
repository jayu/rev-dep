// Package telemetry reports a single anonymous event per `config run` invocation so the
// maintainers can understand how often and in what shape the tool is used.
//
// What is collected is fully described by the Payload and Metrics structs in this package - inspect
// them to see exactly what leaves your machine. It is intentionally limited to:
//   - two non-reversible hashes (an approximate machine id and an approximate project id), and
//   - counts and environment facts (OS, arch, CI, tool/config versions, per-detector usage counts).
//
// No file names, paths, source code, dependency names, or URLs are ever transmitted in the clear.
//
// Telemetry is fully opt-out: set REV_DEP_TELEMETRY_OFF=true to disable it. It is also a no-op
// under `go test` and in any build that did not bake in an Application Insights connection string.
//
// Delivery never slows the tool down: the parent process spawns a fully detached reporter (a hidden
// subcommand of this same binary) that performs all fingerprinting and the network request on its
// own and outlives the parent, so even sub-second runs are reported reliably. Every error is
// silently ignored.
package telemetry

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"rev-dep-go/internal/config"
)

// telemetrySubcommand is the hidden CLI subcommand the detached reporter process runs.
const telemetrySubcommand = "__telemetry"

// connectionString is the Application Insights connection string, injected at build time via
//
//	-ldflags "-X rev-dep-go/internal/telemetry.connectionString=InstrumentationKey=...;IngestionEndpoint=..."
//
// When empty (local/dev builds and the open-source checkout) telemetry is a complete no-op.
var connectionString string

// dispatchInput is the data the parent hands to the detached reporter over stdin. The reporter
// augments it with machine/project fingerprints and environment details to form the Payload that is
// actually sent. Kept separate so the parent does zero fingerprinting or network work on the hot
// path.
type dispatchInput struct {
	Cwd           string  `json:"cwd"`
	ConfigVersion string  `json:"configVersion"`
	Metrics       Metrics `json:"metrics"`
}

// Dispatch fires telemetry for a `config run` invocation without blocking the caller. It spawns a
// fully detached reporter process (the hidden subcommand of this same binary) that performs the
// fingerprinting and the network request on its own, then returns immediately. The reporter
// outlives this process, so even very fast runs are reported. Any failure is silently ignored.
//
// Telemetry is suppressed entirely when REV_DEP_TELEMETRY_OFF=true, under `go test`, or when no
// Application Insights connection string was baked into the build.
func Dispatch(cwd string, cfg *config.RevDepConfig, fileCount int) {
	if !enabled() {
		return
	}

	input := dispatchInput{
		Cwd:           cwd,
		ConfigVersion: cfg.ConfigVersion,
		Metrics:       BuildMetrics(cfg, fileCount),
	}
	data, err := json.Marshal(input)
	if err != nil {
		return
	}

	exe, err := os.Executable()
	if err != nil {
		return
	}

	cmd := exec.Command(exe, telemetrySubcommand)
	// Detach stdout/stderr: nil connects the child fds to the null device. This is REQUIRED - the
	// npm wrapper captures this process's stdout/stderr and waits for EOF, so a detached child that
	// inherited those pipes would hang the wrapper after we exit.
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = detachSysProcAttr()

	// Hand the payload to the child over an explicit *os.File pipe (no copier goroutine, so it
	// survives our exit). The payload is tiny - well under the pipe buffer - so the write completes
	// immediately and the child reads it from the kernel buffer even after we are gone.
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return
	}

	if err := cmd.Start(); err != nil {
		_ = stdinPipe.Close()
		return
	}

	_, _ = stdinPipe.Write(data)
	_ = stdinPipe.Close()
	// Intentionally do NOT Wait: the reporter is detached and manages its own lifetime.
}

// enabled reports whether telemetry should run for this process.
func enabled() bool {
	if connectionString == "" {
		return false
	}
	if os.Getenv("REV_DEP_TELEMETRY_OFF") == "true" {
		return false
	}
	if testing.Testing() {
		return false
	}
	return true
}
