package dupcode

import (
	"fmt"
	"strings"
	"testing"
)

// Detection is parallel and hash-bucketed, so nothing about the pipeline guarantees an
// order on its own: groups are collected by ranging over a map of hash buckets, and both
// sorts that follow are stable, which means anything the sort keys cannot separate comes
// out in whatever order the map handed it over. Go randomises that per run.
//
// The counts are safe either way and a snapshot sorts by hash before it is written, so this
// only shows up where the ORDER is the answer: `--format json` diffs against itself, and
// `--limit N` picks its N off the top of a list whose tail is arbitrary.

// tiedProject builds n duplications that no sort key can tell apart: each appears exactly
// twice and each has the same canonical length, the identifiers being padded to a fixed
// width so that folding or not folding them changes nothing.
func tiedProject(t *testing.T, n int) string {
	t.Helper()
	files := make(map[string]string, n)
	for i := 0; i < n; i++ {
		tag := fmt.Sprintf("%04d", i)
		body := "{\n" +
			"  const requestFor" + tag + " = buildRequest(endpoint, payload);\n" +
			"  const responseOf" + tag + " = transport.send(requestFor" + tag + ");\n" +
			"  if (!responseOf" + tag + ".ok) {\n" +
			"    throw new TransportFailure" + tag + "(responseOf" + tag + ".status);\n" +
			"  }\n" +
			"  return responseOf" + tag + ".body;\n" +
			"}"
		files["mod"+tag+".ts"] =
			"export function sendFirst" + tag + "(endpoint, payload) " + body + "\n" +
				"export function sendSecond" + tag + "(endpoint, payload) " + body + "\n"
	}
	return writeProject(t, files)
}

// orderSignature identifies the result by sequence, and fingerprint identifies it by
// content. Comparing both is what separates "the order moved" from "the findings moved".
func orderSignature(dups []Duplication) string {
	parts := make([]string, 0, len(dups))
	for _, d := range dups {
		parts = append(parts, d.Occurrences[0].File)
	}
	return strings.Join(parts, ",")
}

func contentFingerprint(dups []Duplication) string {
	parts := make([]string, 0, len(dups))
	for _, d := range dups {
		parts = append(parts, d.Occurrences[0].File)
	}
	// Sorted, so this is the SET of findings regardless of the order they came out in.
	for i := 1; i < len(parts); i++ {
		for j := i; j > 0 && parts[j] < parts[j-1]; j-- {
			parts[j], parts[j-1] = parts[j-1], parts[j]
		}
	}
	return strings.Join(parts, ",")
}

// TestDetectOrdersTiedFindingsDeterministically requires repeated runs over unchanged input
// to produce the identical report.
//
// The findings here are deliberately indistinguishable to the sort - same occurrence count,
// same canonical length - because that is the case the sort cannot decide and something
// else therefore has to. Without a final tiebreak the order is the map's.
func TestDetectOrdersTiedFindingsDeterministically(t *testing.T) {
	// Enough groups that several land in the same hash shard; a handful would each get a
	// shard of their own and the map order would never be consulted.
	dir := tiedProject(t, 400)
	opts := Options{Blinding: Blinding{}, MinTokens: 10, MinLines: 1}

	first := detect(t, dir, opts)
	if len(first) < 100 {
		t.Fatalf("expected the tied project to produce plenty of findings, got %d", len(first))
	}
	wantOrder := orderSignature(first)
	wantContent := contentFingerprint(first)

	for run := 2; run <= 6; run++ {
		got := detect(t, dir, opts)
		if contentFingerprint(got) != wantContent {
			t.Fatalf("run %d found a different SET of duplications - this is worse than an "+
				"ordering problem", run)
		}
		if orderSignature(got) != wantOrder {
			t.Fatalf("run %d reported the same findings in a different order; "+
				"tied findings need a deterministic final tiebreak (hash, or first "+
				"occurrence file+offset)", run)
		}
	}
}

// TestDetectReportsTheSameFindingsEveryRun asks the same question of the whole result.
//
// The order test above pins the sequence; this pins the CONTENT, occurrence by occurrence. A
// run whose findings are the same set in the same order can still differ in which copy of a
// tied group was chosen as the representative, and that is what a reader sees printed.
func TestDetectReportsTheSameFindingsEveryRun(t *testing.T) {
	dir := tiedProject(t, 400)
	opts := Options{Blinding: Blinding{}, MinTokens: 10, MinLines: 1}

	want := contentFingerprint(detect(t, dir, opts))
	for run := 2; run <= 6; run++ {
		if got := contentFingerprint(detect(t, dir, opts)); got != want {
			t.Fatalf("run %d returned different findings for unchanged sources:\n  first: %s\n  now:   %s",
				run, want, got)
		}
	}
}

// TestSnapshotIsStableAcrossRuns is the same question asked of the artefact that gets
// committed. It should already hold - BuildSnapshot sorts by hash - and pinning it says
// that a fix for the ordering above is not what is keeping snapshots reviewable.
func TestSnapshotIsStableAcrossRuns(t *testing.T) {
	dir := tiedProject(t, 200)
	opts := Options{Blinding: Blinding{}, MinTokens: 10, MinLines: 1, NeedHashes: true, Cwd: dir}

	var want []string
	for run := 1; run <= 4; run++ {
		dups := detect(t, dir, opts)
		snap := BuildSnapshot(dups, opts)
		got := make([]string, 0, len(snap.Entries))
		for _, e := range snap.Entries {
			got = append(got, e.Hash)
		}
		if run == 1 {
			want = got
			if len(want) < 50 {
				t.Fatalf("expected a meaningful snapshot, got %d entries", len(want))
			}
			continue
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("run %d wrote a different snapshot for unchanged sources", run)
		}
	}
}
