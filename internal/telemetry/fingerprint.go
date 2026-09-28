package telemetry

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/url"
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

// repoID returns a stable, non-reversible identifier for the closest Git repository that contains
// cwd. Git does not have an immutable repository UUID, so a configured remote is the only stable
// repository-level identity shared by its clones. Repositories without a usable remote return "":
// hashing a local path or the current commit would be less stable and less useful.
func repoID(cwd string) string {
	gitDir := closestGitDir(cwd)
	if gitDir == "" {
		return ""
	}

	remoteURL := gitRemoteURL(gitDir)
	if remoteURL == "" {
		return ""
	}
	return sha256Hex(remoteURL)
}

// closestGitDir finds the nearest containing Git worktree or bare repository without invoking Git.
// A worktree's .git entry can be either a directory or a file that points to its actual Git dir.
func closestGitDir(cwd string) string {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}

	for {
		if gitDir := gitDirAt(dir); gitDir != "" {
			return gitDir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func gitDirAt(dir string) string {
	gitEntry := filepath.Join(dir, ".git")
	info, err := os.Stat(gitEntry)
	if err == nil {
		switch {
		case info.IsDir() && isGitDir(gitEntry):
			return gitEntry
		case info.Mode().IsRegular():
			return gitDirFromFile(gitEntry)
		}
	}

	// A bare repository has the Git directory at its root rather than in a .git entry.
	if isGitDir(dir) {
		return dir
	}
	return ""
}

func gitDirFromFile(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	line, _, _ := strings.Cut(string(content), "\n")
	line = strings.TrimSpace(line)
	if len(line) < len("gitdir:") || !strings.EqualFold(line[:len("gitdir:")], "gitdir:") {
		return ""
	}

	gitDir := strings.TrimSpace(line[len("gitdir:"):])
	if gitDir == "" {
		return ""
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(filepath.Dir(path), gitDir)
	}
	gitDir = filepath.Clean(gitDir)
	if !isGitDir(gitDir) {
		return ""
	}
	return gitDir
}

func isGitDir(path string) bool {
	head, err := os.Stat(filepath.Join(path, "HEAD"))
	if err != nil || head.IsDir() {
		return false
	}
	objects, err := os.Stat(filepath.Join(path, "objects"))
	if err == nil && objects.IsDir() {
		return true
	}

	// Linked worktrees store objects in their common Git directory and mark that relationship with
	// a commondir file in the worktree-specific Git dir.
	commonDir, err := os.Stat(filepath.Join(path, "commondir"))
	return err == nil && !commonDir.IsDir()
}

// gitRemoteURL returns a canonical URL for a repository remote. origin is preferred because it is
// the conventional clone source; if it is absent, the lexicographically first usable remote keeps
// the result deterministic. Git worktrees share config through the common Git directory.
func gitRemoteURL(gitDir string) string {
	var originURLs, otherURLs []string
	for _, configPath := range gitConfigPaths(gitDir) {
		content, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}
		for _, remote := range gitRemoteURLs(content) {
			url := normalizeRepoURL(remote.url)
			if url == "" {
				continue
			}
			if strings.EqualFold(remote.name, "origin") {
				originURLs = append(originURLs, url)
			} else {
				otherURLs = append(otherURLs, url)
			}
		}
	}

	if len(originURLs) > 0 {
		slices.Sort(originURLs)
		return originURLs[0]
	}
	if len(otherURLs) > 0 {
		slices.Sort(otherURLs)
		return otherURLs[0]
	}
	return ""
}

func gitConfigPaths(gitDir string) []string {
	commonDir := gitDir
	if content, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		commonDir = strings.TrimSpace(string(content))
		if !filepath.IsAbs(commonDir) {
			commonDir = filepath.Join(gitDir, commonDir)
		}
		commonDir = filepath.Clean(commonDir)
	}

	if commonDir == gitDir {
		return []string{filepath.Join(gitDir, "config")}
	}
	return []string{
		filepath.Join(gitDir, "config.worktree"),
		filepath.Join(commonDir, "config"),
	}
}

type gitRemote struct {
	name string
	url  string
}

// gitRemoteURLs extracts remote URLs from a local Git config. The config files where Git stores
// remotes are small and use a simple section/key format, so reading them avoids an extra process in
// the detached reporter and works even when the Git executable is unavailable.
func gitRemoteURLs(content []byte) []gitRemote {
	var remotes []gitRemote
	remoteName := ""

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			remoteName = gitRemoteSectionName(line)
			continue
		}
		if remoteName == "" {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "url") {
			continue
		}
		if value = gitConfigValue(value); value != "" {
			remotes = append(remotes, gitRemote{name: remoteName, url: value})
		}
	}
	return remotes
}

func gitRemoteSectionName(line string) string {
	end := strings.IndexByte(line, ']')
	if end == -1 {
		return ""
	}
	header := strings.TrimSpace(line[1:end])
	if len(header) < len("remote") || !strings.EqualFold(header[:len("remote")], "remote") {
		return ""
	}

	name := strings.TrimSpace(header[len("remote"):])
	if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
		if unquoted, err := strconv.Unquote(name); err == nil {
			return unquoted
		}
		return name[1 : len(name)-1]
	}
	return ""
}

func gitConfigValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
		return value[1 : len(value)-1]
	}
	return value
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

// normalizeRepoURL reduces supported network Git URL forms to a single canonical string and strips
// user information, query parameters, and fragments. Local-path remotes are intentionally ignored:
// their paths are not a repository identity shared by clones.
func normalizeRepoURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || filepath.IsAbs(rawURL) || filepath.VolumeName(rawURL) != "" {
		return ""
	}
	rawURL = strings.TrimPrefix(rawURL, "git+")

	// Git accepts scp-style remotes such as git@github.com:owner/repo.git.
	if hostPart, repoPath, ok := strings.Cut(rawURL, ":"); ok && !strings.Contains(rawURL, "://") &&
		!strings.Contains(hostPart, "/") && !strings.Contains(hostPart, "\\") {
		host := hostPart
		if at := strings.LastIndexByte(host, '@'); at >= 0 {
			host = host[at+1:]
		}
		return normalizeRepoAddress(host, repoPath, "")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || parsed.Scheme == "file" {
		return ""
	}
	return normalizeRepoAddress(parsed.Hostname(), parsed.EscapedPath(), parsed.Port())
}

func normalizeRepoAddress(host, repoPath, port string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	repoPath = strings.Trim(strings.TrimSpace(repoPath), "/")
	if host == "" || repoPath == "" {
		return ""
	}
	if len(repoPath) >= len(".git") && strings.EqualFold(repoPath[len(repoPath)-len(".git"):], ".git") {
		repoPath = strings.TrimRight(repoPath[:len(repoPath)-len(".git")], "/")
	}
	if repoPath == "" {
		return ""
	}
	if port != "" {
		host += ":" + port
	}
	return host + "/" + repoPath
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
