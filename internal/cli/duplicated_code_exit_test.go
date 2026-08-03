package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"rev-dep-go/internal/dupcode"
)

// A failing check is not a failing command. Every other check command here says so by
// printing its report and calling os.Exit: circular (root.go), node-modules unused and
// missing (root.go), config run (config_run.go). duplicated-code used to return an empty
// error instead, and cobra turned that into a bare `Error:` line on stderr.
//
// That line is not invisible. The npm wrapper (npm/rev-dep/bin.js) forwards stderr whenever
// the exit code is non-zero, so it lands under a report that had already said everything.
//
// os.Exit cannot be observed from inside a test, so the check below re-runs this test binary
// as a child process - which is also what the wrapper does, and therefore what is actually
// worth asserting.

const dupExitSubprocessEnv = "REV_DEP_TEST_DUPCODE_EXIT"

// duplicatedBlock is long enough to clear the default thresholds.
const duplicatedBlock = `{
  const request = buildRequest(endpoint, payload);
  const response = await transport.send(request);
  if (!response.ok) {
    throw new TransportFailure(response.status, response.body);
  }
  return response.body;
}`

func writeDupProject(t *testing.T, dir string, names ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		src := "export async function " + strings.TrimSuffix(name, ".ts") +
			"(endpoint, payload) " + duplicatedBlock + "\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDuplicatedCodeSnapshotFailureExitsWithoutACobraError(t *testing.T) {
	// Child half: run the check that is expected to fail and let os.Exit do its work.
	if os.Getenv(dupExitSubprocessEnv) != "" {
		err := duplicatedCodeCmdFn(
			os.Getenv(dupExitSubprocessEnv), dupcode.Blinding{},
			0, 0, 0, 0, 0, false, nil, nil,
			os.Getenv(dupExitSubprocessEnv+"_SNAPSHOT"), false, "human", false, nil,
		)
		if err != nil {
			// Reaching here means the command returned an error rather than exiting, which
			// is the regression. Let cobra's path run so the parent sees the difference.
			t.Fatalf("returned an error instead of exiting: %v", err)
		}
		return
	}

	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	snapshot := filepath.Join(dir, "snapshot.json")

	// Two copies, acknowledged in a snapshot.
	writeDupProject(t, project, "sendA.ts", "sendB.ts")
	if err := duplicatedCodeCmdFn(project, dupcode.Blinding{},
		0, 0, 0, 0, 0, false, nil, nil, snapshot, true, "human", false, nil); err != nil {
		t.Fatalf("writing the snapshot failed: %v", err)
	}

	// A third copy, which the snapshot does not know about.
	writeDupProject(t, project, "sendC.ts")

	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	cmd.Env = append(os.Environ(),
		dupExitSubprocessEnv+"="+project,
		dupExitSubprocessEnv+"_SNAPSHOT="+snapshot,
	)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected a non-zero exit when the run no longer matches the snapshot, got %v\n"+
			"stdout:\n%s", err, stdout.String())
	}
	if code := exitErr.ExitCode(); code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}

	// The report is the output, and it belongs on stdout like config run's.
	if !strings.Contains(stdout.String(), "Duplication increased.") {
		t.Errorf("the delta was not printed to stdout:\n%s", stdout.String())
	}

	// Nothing on stderr: the wrapper forwards it verbatim on a non-zero exit, so anything
	// here is appended to a report that is already complete.
	if got := strings.TrimSpace(stderr.String()); got != "" {
		t.Errorf("stderr should be empty on a failing check, got:\n%s", got)
	}
}

// TestDuplicatedCodeInvalidFlagStillReturnsAnError is the other side of the rule: a bad
// invocation IS a command error, so it keeps going through cobra and keeps its message.
func TestDuplicatedCodeInvalidFlagStillReturnsAnError(t *testing.T) {
	err := duplicatedCodeCmdFn(t.TempDir(), dupcode.Blinding{},
		0, 0, 0, 0, 0, false, nil, nil, "", false, "bogus", false, nil)
	if err == nil {
		t.Fatal("an invalid --format must be reported as an error")
	}
	if !strings.Contains(err.Error(), "invalid --format") {
		t.Errorf("error message does not name the problem: %v", err)
	}
}

// TestDuplicatedCodeMissingSnapshotIsAnError pins the third case: a configured baseline that
// is not on disk is a broken setup rather than a failing check, so it keeps the error path
// and the message telling the reader how to create it.
func TestDuplicatedCodeMissingSnapshotIsAnError(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	writeDupProject(t, project, "sendA.ts", "sendB.ts")

	err := duplicatedCodeCmdFn(project, dupcode.Blinding{},
		0, 0, 0, 0, 0, false, nil, nil,
		filepath.Join(dir, "absent.json"), false, "human", false, nil)
	if err == nil {
		t.Fatal("a missing snapshot must not pass silently")
	}
	if !strings.Contains(err.Error(), "--update-snapshot") {
		t.Errorf("the error should say how to create the snapshot: %v", err)
	}
}

// TestSnapshotPathIsRelativeToCwd covers where the baseline file ends up.
//
// Every other path the command takes is relative to --cwd, and the snapshot is a file that
// belongs to the project being analysed - so it has to be too. Resolving it against the
// process's own directory instead put the file wherever the shell happened to be, and
// --update-snapshot wrote it there without saying anything.
func TestSnapshotPathIsRelativeToCwd(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	writeDupProject(t, project, "sendA.ts", "sendB.ts")

	// Run from somewhere that is NOT the project, exactly as a user pointing --cwd at
	// another tree would be.
	elsewhere := filepath.Join(dir, "elsewhere")
	if err := os.MkdirAll(elsewhere, 0o755); err != nil {
		t.Fatal(err)
	}
	restore, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(elsewhere); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(restore)

	if err := duplicatedCodeCmdFn(project, dupcode.Blinding{},
		0, 0, 0, 0, 0, false, nil, nil, "dups.json", true, "human", false, nil); err != nil {
		t.Fatalf("writing the snapshot failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(project, "dups.json")); err != nil {
		t.Errorf("the snapshot was not written under --cwd: %v", err)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, "dups.json")); err == nil {
		t.Error("the snapshot was written next to the process instead of under --cwd")
	}

	// And reading it back finds the same file, so a check run agrees with the write.
	if err := duplicatedCodeCmdFn(project, dupcode.Blinding{},
		0, 0, 0, 0, 0, false, nil, nil, "dups.json", false, "human", false, nil); err != nil {
		t.Errorf("the snapshot just written was not found on the next run: %v", err)
	}
}

// TestAbsoluteSnapshotPathIsUnchanged: only a relative path is anchored to --cwd.
func TestAbsoluteSnapshotPathIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project")
	writeDupProject(t, project, "sendA.ts", "sendB.ts")
	absolute := filepath.Join(dir, "outside.json")

	if err := duplicatedCodeCmdFn(project, dupcode.Blinding{},
		0, 0, 0, 0, 0, false, nil, nil, absolute, true, "human", false, nil); err != nil {
		t.Fatalf("writing the snapshot failed: %v", err)
	}
	if _, err := os.Stat(absolute); err != nil {
		t.Errorf("an absolute --snapshot was not honoured: %v", err)
	}
}
