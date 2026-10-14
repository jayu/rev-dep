// Package baseline stores and compares semantic finding sets. It knows nothing
// about config processing, detector options, source positions, or output formats.
package baseline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

const SchemaVersion = "1.0"

// Issue contains only fields that identify a problem. Validate constrains the
// shape separately for each detector; diagnostic evidence is never persisted.
type Issue struct {
	File          string   `json:"file,omitempty"`
	Target        string   `json:"target,omitempty"`
	Module        string   `json:"module,omitempty"`
	PackageJSON   string   `json:"packageJson,omitempty"`
	Export        string   `json:"export,omitempty"`
	IsType        *bool    `json:"isType,omitempty"`
	Request       string   `json:"request,omitempty"`
	Kind          string   `json:"kind,omitempty"`
	ViolationType string   `json:"violationType,omitempty"`
	EntryPoint    string   `json:"entryPoint,omitempty"`
	Files         []string `json:"files,omitempty"`
	// Preserve input keys: omitempty must not hide unexpected empty fields when
	// checking the detector-specific shape of a loaded record.
	inputFields []string
}

func (issue *Issue) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		return fmt.Errorf("baseline finding must be an object")
	}
	type plainIssue Issue
	var decoded plainIssue
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	decoded.inputFields = make([]string, 0, len(fields))
	for key, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("baseline finding field %s must not be null", key)
		}
		decoded.inputFields = append(decoded.inputFields, key)
	}
	*issue = Issue(decoded)
	return nil
}

type Snapshot struct {
	SchemaVersion string                        `json:"schemaVersion"`
	Workspaces    map[string]map[string][]Issue `json:"workspaces"`
}

type Finding struct {
	Workspace string `json:"workspace"`
	Detector  string `json:"detector"`
	Issue     Issue  `json:"issue"`
}

type Counts struct {
	Acknowledged int `json:"acknowledged"`
	New          int `json:"new"`
	Obsolete     int `json:"obsolete"`
}

// Result is the effective comparison, independent of raw detector failures.
type Result struct {
	Path       string    `json:"path"`
	Status     string    `json:"status"` // matched, different, written
	Counts     Counts    `json:"counts"`
	New        []Finding `json:"new"`
	Obsolete   []Finding `json:"obsolete"`
	Workspaces []string  `json:"workspaces,omitempty"` // present for a partial validation
}

func New() *Snapshot {
	return &Snapshot{SchemaVersion: SchemaVersion, Workspaces: map[string]map[string][]Issue{}}
}

func (s *Snapshot) Add(workspace, detector string, issue Issue) {
	if s.Workspaces[workspace] == nil {
		s.Workspaces[workspace] = map[string][]Issue{}
	}
	s.Workspaces[workspace][detector] = append(s.Workspaces[workspace][detector], issue)
}

func IssueKey(issue Issue) string {
	b, _ := json.Marshal(issue)
	return string(b)
}

func FindingKey(f Finding) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func isCanonicalPath(value string) bool {
	return value != "" && !strings.ContainsAny(value, "\\\x00") && !path.IsAbs(value) &&
		!(len(value) >= 2 && value[1] == ':') && path.Clean(value) == value
}

// CanonicalIssue also validates identities captured by adapters, keeping writer
// and reader requirements identical.
func CanonicalIssue(detector string, issue Issue) (Issue, error) {
	if len(issue.Files) > 0 {
		issue.Files = slices.Clone(issue.Files)
		slices.Sort(issue.Files)
		issue.Files = slices.Compact(issue.Files)
	}
	var required []string
	switch detector {
	case "circular-imports":
		required = []string{"files"}
	case "orphan-files":
		required = []string{"file"}
	case "missing-node-modules", "dev-deps-usage-on-prod":
		required = []string{"file", "module"}
	case "unused-node-modules":
		required = []string{"packageJson", "module"}
	case "unused-exports":
		required = []string{"file", "export", "isType"}
	case "unresolved-imports":
		required = []string{"file", "request", "kind"}
		if issue.Kind != "literal" && issue.Kind != "nonliteral" {
			return Issue{}, fmt.Errorf("invalid unresolved import kind %q", issue.Kind)
		}
	case "module-boundaries":
		required = []string{"file", "target"}
	case "import-conventions":
		required = []string{"file", "request", "violationType"}
		if !slices.Contains([]string{"should-be-relative", "should-be-aliased", "wrong-alias"}, issue.ViolationType) {
			return Issue{}, fmt.Errorf("invalid import convention violation %q", issue.ViolationType)
		}
	case "restricted-imports":
		if issue.Kind == "file" {
			required = []string{"kind", "target"}
		} else if issue.Kind == "module" {
			required = []string{"kind", "request"}
		}
	case "restricted-importers":
		if issue.Kind == "file" {
			required = []string{"entryPoint", "kind", "target"}
		} else if issue.Kind == "module" {
			required = []string{"entryPoint", "kind", "module"}
		}
	case "restricted-direct-importers":
		if issue.Kind == "file" {
			required = []string{"file", "kind", "target"}
		} else if issue.Kind == "module" {
			required = []string{"file", "kind", "request"}
		}
	default:
		return Issue{}, fmt.Errorf("unsupported baseline detector %q", detector)
	}
	if len(required) == 0 {
		return Issue{}, fmt.Errorf("invalid %s target kind %q", detector, issue.Kind)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(IssueKey(issue)), &fields); err != nil {
		return Issue{}, err
	}
	for _, key := range required {
		if _, ok := fields[key]; !ok {
			return Issue{}, fmt.Errorf("%s finding requires %s", detector, key)
		}
	}
	for key := range fields {
		if !slices.Contains(required, key) {
			return Issue{}, fmt.Errorf("unexpected %s in %s finding", key, detector)
		}
	}
	for _, key := range issue.inputFields {
		if !slices.Contains(required, key) {
			return Issue{}, fmt.Errorf("unexpected %s in %s finding", key, detector)
		}
	}
	for _, p := range []string{issue.File, issue.Target, issue.PackageJSON, issue.EntryPoint} {
		if p != "" && !isCanonicalPath(p) {
			return Issue{}, fmt.Errorf("invalid baseline path %q", p)
		}
	}
	for _, p := range issue.Files {
		if !isCanonicalPath(p) {
			return Issue{}, fmt.Errorf("invalid component path %q", p)
		}
	}
	return issue, nil
}

func (s *Snapshot) Canonicalize() error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported baseline schema version %q; run rev-dep config run --update-snapshot", s.SchemaVersion)
	}
	if s.Workspaces == nil {
		return fmt.Errorf("baseline requires a workspaces object")
	}
	for workspace, checks := range s.Workspaces {
		if !isCanonicalPath(workspace) {
			return fmt.Errorf("invalid workspace path %q", workspace)
		}
		if checks == nil {
			return fmt.Errorf("workspace %q requires a checks object", workspace)
		}
		for detector, issues := range checks {
			// Validate the detector even for an empty group.
			if !slices.Contains(Detectors(), detector) {
				return fmt.Errorf("unsupported baseline detector %q", detector)
			}
			if issues == nil {
				return fmt.Errorf("%s.%s requires a findings array", workspace, detector)
			}
			for i, issue := range issues {
				normalized, err := CanonicalIssue(detector, issue)
				if err != nil {
					return fmt.Errorf("%s.%s: %w", workspace, detector, err)
				}
				issues[i] = normalized
			}
			slices.SortFunc(issues, func(a, b Issue) int { return strings.Compare(IssueKey(a), IssueKey(b)) })
			issues = slices.CompactFunc(issues, func(a, b Issue) bool { return IssueKey(a) == IssueKey(b) })
			if len(issues) == 0 {
				delete(checks, detector)
			} else {
				checks[detector] = issues
			}
		}
		if len(checks) == 0 {
			delete(s.Workspaces, workspace)
		}
	}
	return nil
}

func Detectors() []string {
	return []string{"circular-imports", "orphan-files", "module-boundaries", "unused-node-modules", "missing-node-modules", "unused-exports", "unresolved-imports", "dev-deps-usage-on-prod", "restricted-imports", "restricted-importers", "restricted-direct-importers", "import-conventions"}
}

func (s *Snapshot) Findings(workspaces []string) []Finding {
	findings := []Finding{}
	for workspace, checks := range s.Workspaces {
		if len(workspaces) > 0 && !slices.Contains(workspaces, workspace) {
			continue
		}
		for detector, issues := range checks {
			for _, issue := range issues {
				findings = append(findings, Finding{workspace, detector, issue})
			}
		}
	}
	slices.SortFunc(findings, func(a, b Finding) int { return strings.Compare(FindingKey(a), FindingKey(b)) })
	return findings
}

func Compare(current, previous *Snapshot, workspaces []string) *Result {
	r := &Result{Status: "matched", New: []Finding{}, Obsolete: []Finding{}, Workspaces: slices.Clone(workspaces)}
	slices.Sort(r.Workspaces)
	r.Workspaces = slices.Compact(r.Workspaces)
	was := map[string]Finding{}
	for _, f := range previous.Findings(workspaces) {
		was[FindingKey(f)] = f
	}
	for _, f := range current.Findings(workspaces) {
		key := FindingKey(f)
		if _, ok := was[key]; ok {
			r.Counts.Acknowledged++
			delete(was, key)
		} else {
			r.New = append(r.New, f)
		}
	}
	for _, f := range was {
		r.Obsolete = append(r.Obsolete, f)
	}
	slices.SortFunc(r.Obsolete, func(a, b Finding) int { return strings.Compare(FindingKey(a), FindingKey(b)) })
	r.Counts.New, r.Counts.Obsolete = len(r.New), len(r.Obsolete)
	if r.Counts.New+r.Counts.Obsolete > 0 {
		r.Status = "different"
	}
	return r
}

func Load(filename string) (*Snapshot, error) {
	raw, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read baseline snapshot %s: %w; run rev-dep config run --update-snapshot to record it", filename, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("parse baseline snapshot %s: %w", filename, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("baseline snapshot %s contains trailing data", filename)
	}
	if err := snapshot.Canonicalize(); err != nil {
		return nil, fmt.Errorf("baseline snapshot %s: %w", filename, err)
	}
	return &snapshot, nil
}

// CheckDestination permits a new file or a recognizable findings baseline. It
// checks the envelope only, so unsupported versions or invalid findings can be
// regenerated without allowing updates to replace unrelated project files.
func CheckDestination(filename string) error {
	raw, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read baseline destination %s: %w", filename, err)
	}
	var envelope struct {
		SchemaVersion string          `json:"schemaVersion"`
		Workspaces    json.RawMessage `json:"workspaces"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.SchemaVersion == "" || !bytes.HasPrefix(bytes.TrimSpace(envelope.Workspaces), []byte("{")) {
		return fmt.Errorf("refusing to overwrite %s: existing file is not a recognizable findings baseline; remove or rename it before running rev-dep config run --update-snapshot", filename)
	}
	return nil
}

// Save replaces a complete file atomically; a failed write preserves the old one.
func Save(filename string, snapshot *Snapshot) error {
	if err := snapshot.Canonicalize(); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snapshot); err != nil {
		return err
	}
	if err := CheckDestination(filename); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(filename), ".baseline-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := tmp.Chmod(0644); err != nil {
		return err
	}
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), filename); err != nil {
		return fmt.Errorf("replace baseline snapshot %s: %w", filename, err)
	}
	return nil
}
