package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// sha256Hex returns the lowercase hex-encoded SHA-256 of s.
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// machineID returns a stable, non-reversible identifier for the current machine, used only to
// approximate how many distinct machines run the tool. It combines coarse, stable hardware/OS facts
// and hashes them, so the raw values never leave the machine.
func machineID() string {
	hostname, _ := os.Hostname()
	parts := []string{
		runtime.GOOS,
		runtime.GOARCH,
		strconv.Itoa(runtime.NumCPU()),
		hostname,
		firstMAC(),
	}
	return sha256Hex(strings.Join(parts, "|"))
}

// firstMAC returns the lexicographically-first non-loopback, non-zero hardware (MAC) address, or ""
// if none is available. Sorting keeps the value stable across runs regardless of interface order.
func firstMAC() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	macs := make([]string, 0, len(ifaces))
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		hw := ifc.HardwareAddr.String()
		if hw == "" || hw == "00:00:00:00:00:00" {
			continue
		}
		macs = append(macs, hw)
	}
	if len(macs) == 0 {
		return ""
	}
	slices.Sort(macs)
	return macs[0]
}

// projectID returns a stable, non-reversible identifier for the project rooted at cwd, used only to
// approximate how many distinct projects run the tool. It hashes the root package.json name
// together with the normalized repository URL; including the URL makes the hash far harder to
// reverse than a bare package name. Returns "" when neither is available.
func projectID(cwd string) string {
	name, repoURL := rootProjectIdentity(cwd)
	if name == "" && repoURL == "" {
		return ""
	}
	return sha256Hex(repoURL + "\x00" + name)
}

// rootProjectIdentity reads the package.json at cwd and returns its name and normalized repository
// URL. Missing or malformed files yield empty strings.
func rootProjectIdentity(cwd string) (name string, repoURL string) {
	content, err := os.ReadFile(filepath.Join(cwd, "package.json"))
	if err != nil {
		return "", ""
	}

	var pkg struct {
		Name       string          `json:"name"`
		Repository json.RawMessage `json:"repository"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return "", ""
	}

	return pkg.Name, normalizeRepoURL(parseRepositoryURL(pkg.Repository))
}

// parseRepositoryURL extracts the URL from package.json's "repository" field, which may be a bare
// string or an object of the form {"type": "...", "url": "..."}.
func parseRepositoryURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		return asString
	}
	var asObject struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(raw, &asObject) == nil {
		return asObject.URL
	}
	return ""
}

// normalizeRepoURL reduces the various forms of a git remote (https, ssh, git+, trailing .git) to a
// single canonical string so the same repository hashes identically regardless of how it is
// referenced.
func normalizeRepoURL(url string) string {
	u := strings.TrimSpace(strings.ToLower(url))
	if u == "" {
		return ""
	}
	for _, prefix := range []string{"git+", "git://", "https://", "http://", "ssh://", "git@"} {
		u = strings.TrimPrefix(u, prefix)
	}
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, ".git")
	u = strings.TrimRight(u, "/")
	// Normalize the scp-like "host:owner/repo" form to "host/owner/repo".
	u = strings.Replace(u, ":", "/", 1)
	return u
}

// isCI reports whether the tool appears to be running in a CI environment. Most providers set CI;
// a few provider-specific variables cover the rest.
func isCI() bool {
	if v := os.Getenv("CI"); v != "" && v != "false" && v != "0" {
		return true
	}
	for _, key := range []string{
		"GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "BUILDKITE",
		"JENKINS_URL", "TEAMCITY_VERSION", "TF_BUILD", "APPVEYOR", "BITBUCKET_BUILD_NUMBER",
	} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}
