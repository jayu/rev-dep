package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"rev-dep-go/internal/version"
)

// reporterTimeout bounds the detached reporter's whole network attempt. It is generous because the
// reporter is detached and does not block anything.
const reporterTimeout = 5 * time.Second

// Payload is the exact anonymous record sent to the telemetry collector for a `config run`. Inspect
// this struct to see everything that is shared. It contains only two non-reversible hashes plus
// counts and environment facts; no file names, paths, code, dependency names, or URLs are ever
// transmitted in the clear.
type Payload struct {
	MachineID     string  `json:"machineId"`     // sha256(hardware/OS facts); approximate unique machine
	ProjectID     string  `json:"projectId"`     // sha256(repo URL + root package.json name); approximate unique project
	ToolVersion   string  `json:"toolVersion"`   // rev-dep version
	ConfigVersion string  `json:"configVersion"` // config schema version in use
	OS            string  `json:"os"`            // GOOS: darwin / linux / windows
	Arch          string  `json:"arch"`          // GOARCH
	IsCI          bool    `json:"isCI"`          // best-effort CI detection
	Metrics       Metrics `json:"metrics"`       // anonymous usage counts
}

// Configured reports whether this build has a usable telemetry connection string baked in - a
// non-empty value that parses to a non-empty InstrumentationKey. The production build uses this
// (via `__telemetry --check`) to verify the key was actually injected, catching a typo'd `-X`
// import path, which the linker silently ignores.
func Configured() bool {
	iKey, _ := parseConnectionString(connectionString)
	return iKey != ""
}

// RunReporter is the entry point for the hidden `__telemetry` subcommand. It reads a dispatchInput
// from stdin, builds the full Payload (computing the machine/project fingerprints here, off the
// parent's hot path), and sends it. Every error is silently ignored.
func RunReporter() {
	iKey, endpoint := parseConnectionString(connectionString)
	if iKey == "" {
		return
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		return
	}

	var input dispatchInput
	if err := json.Unmarshal(data, &input); err != nil {
		return
	}

	payload := Payload{
		MachineID:     machineID(),
		ProjectID:     projectID(input.Cwd),
		ToolVersion:   version.Version,
		ConfigVersion: input.ConfigVersion,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		IsCI:          isCI(),
		Metrics:       input.Metrics,
	}

	ctx, cancel := context.WithTimeout(context.Background(), reporterTimeout)
	defer cancel()
	_ = send(ctx, iKey, endpoint, payload)
}

// send posts the payload to the Application Insights ingestion endpoint as a custom event.
func send(ctx context.Context, iKey, endpoint string, p Payload) error {
	envelope := map[string]any{
		"name": "Microsoft.ApplicationInsights.Event",
		"time": time.Now().UTC().Format(time.RFC3339),
		"iKey": iKey,
		"tags": map[string]string{"ai.cloud.role": "rev-dep-cli"},
		"data": map[string]any{
			"baseType": "EventData",
			"baseData": map[string]any{
				"ver":  2,
				"name": "config-run",
				"properties": map[string]string{
					"machineId":     p.MachineID,
					"projectId":     p.ProjectID,
					"toolVersion":   p.ToolVersion,
					"configVersion": p.ConfigVersion,
					"os":            p.OS,
					"arch":          p.Arch,
					"isCI":          strconv.FormatBool(p.IsCI),
				},
				"measurements": p.Metrics.asMeasurements(),
			},
		},
	}

	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/v2/track", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// parseConnectionString extracts the instrumentation key and ingestion endpoint from an Application
// Insights connection string. The endpoint defaults to the classic global collector when not
// specified.
func parseConnectionString(cs string) (iKey string, endpoint string) {
	endpoint = "https://dc.services.visualstudio.com"
	for _, part := range strings.Split(cs, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(kv[0]))
		value := strings.TrimSpace(kv[1])
		switch key {
		case "instrumentationkey":
			iKey = value
		case "ingestionendpoint":
			endpoint = strings.TrimRight(value, "/")
		}
	}
	return iKey, endpoint
}
