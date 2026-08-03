package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rev-dep-go/internal/dupcode"
)

// What happens to a snapshot must not depend on the output format. It used to: the snapshot
// decisions lived inside the human branch, so `--update-snapshot --format json` wrote no file
// and a deleted snapshot exited 0. Both silent - the JSON on stdout looked correct.

// captureStdout redirects os.Stdout, which is what the command writes to - a version taking an
// io.Writer would pass while the real one still printed elsewhere.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	restore := os.Stdout
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				b.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		done <- b.String()
	}()

	runErr := fn()
	w.Close()
	os.Stdout = restore
	return <-done, runErr
}

func runDuplicatedCode(t *testing.T, project, snapshot string, update bool, format string) (string, error) {
	t.Helper()
	return captureStdout(t, func() error {
		return duplicatedCodeCmdFn(project, dupcode.Blinding{},
			0, 0, 0, 0, 0, false, nil, nil, snapshot, update, format, false, nil)
	})
}

func TestUpdateSnapshotWritesTheFileInEveryFormat(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			project := filepath.Join(dir, "project")
			writeDupProject(t, project, "sendA.ts", "sendB.ts")
			snapshot := filepath.Join(dir, "snapshot.json")

			out, err := runDuplicatedCode(t, project, snapshot, true, format)
			if err != nil {
				t.Fatalf("--update-snapshot returned an error: %v", err)
			}
			if _, err := os.Stat(snapshot); err != nil {
				t.Fatalf("--update-snapshot --format %s wrote no file: %v", format, err)
			}

			snap, err := dupcode.LoadSnapshot(snapshot)
			if err != nil {
				t.Fatalf("the file it wrote is not a snapshot: %v", err)
			}
			if len(snap.Entries) == 0 {
				t.Error("the snapshot acknowledged nothing, though the project is duplicated")
			}

			if format != "json" {
				return
			}
			// A baseline written from this run matches this run exactly, so it is reported as
			// a clean comparison rather than as a separate shape a consumer has to handle.
			var report struct {
				Snapshot *struct {
					Path  string `json:"path"`
					Clean bool   `json:"clean"`
				} `json:"snapshot"`
			}
			if err := json.Unmarshal([]byte(out), &report); err != nil {
				t.Fatalf("output is not valid JSON: %v\n%s", err, out)
			}
			if report.Snapshot == nil {
				t.Fatal("JSON output does not mention the snapshot it just wrote")
			}
			if !report.Snapshot.Clean {
				t.Error("a snapshot written from this run should compare clean against it")
			}
			if report.Snapshot.Path != snapshot {
				t.Errorf("reported path %q, wrote %q", report.Snapshot.Path, snapshot)
			}
		})
	}
}

// A configured baseline that does not exist is a broken setup, not an empty one. Passing
// would let a deleted file turn the check green - and in JSON it did.
func TestMissingSnapshotFailsInEveryFormat(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			project := filepath.Join(dir, "project")
			writeDupProject(t, project, "sendA.ts", "sendB.ts")
			missing := filepath.Join(dir, "never-written.json")

			out, err := runDuplicatedCode(t, project, missing, false, format)
			if err == nil {
				t.Fatalf("--format %s reported success for a snapshot that does not exist", format)
			}
			if !strings.Contains(err.Error(), "does not exist") {
				t.Errorf("the error does not say what is wrong: %v", err)
			}
			// The report is still printed, so the reader can see what they would acknowledge.
			if strings.TrimSpace(out) == "" {
				t.Error("nothing was reported alongside the error")
			}
			if format == "json" {
				if err := json.Unmarshal([]byte(out), &struct{}{}); err != nil {
					t.Errorf("stdout is not valid JSON: %v", err)
				}
			}
		})
	}
}

// The whole point of writing a baseline is that the next run compares clean against it. This
// closes the loop the two tests above open, in both formats.
func TestSnapshotWrittenInOneFormatIsCleanInTheOther(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	writeDupProject(t, project, "sendA.ts", "sendB.ts")
	snapshot := filepath.Join(dir, "snapshot.json")

	if _, err := runDuplicatedCode(t, project, snapshot, true, "json"); err != nil {
		t.Fatalf("writing the snapshot as JSON failed: %v", err)
	}
	if _, err := runDuplicatedCode(t, project, snapshot, false, "human"); err != nil {
		t.Errorf("a snapshot written by --format json is not clean when read back: %v", err)
	}
}
