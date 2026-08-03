package dupcode

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"rev-dep-go/internal/fs"
	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/parser"
	"rev-dep-go/internal/pathutil"
)

// Two phases: scan every file in parallel into hashed block records, then compare only the
// blocks that share a hash.
//
// The naive shape of this problem is a pairwise sweep over files with early-bail heuristics.
// The hash removes it: two blocks can only be duplicates if their canonical forms are
// byte-identical, so blocks sharing a hash are the ONLY pairs worth comparing.
//
// The hash decides what to compare; it never decides a match. Every bucket is confirmed by
// Compare, so a collision costs a wasted comparison rather than a wrong result.

// Occurrence is one place a duplicated chunk appears.
type Occurrence struct {
	File string // relative to the analysed directory
	// FileID identifies the file even when File is empty, so counting distinct files never
	// has to build path strings.
	FileID int32
	Start  uint32
	End    uint32 // exclusive
	// Line/Col locate Start, EndLine/EndCol the last character. 1-based, zero under CountOnly.
	Line    int
	Col     int
	EndLine int
	EndCol  int
}

// Duplication is one chunk of code together with every place it was found.
type Duplication struct {
	// Snippet is the first occurrence verbatim; the copies may be spaced differently, so
	// only one can be shown.
	Snippet     string
	Occurrences []Occurrence
	FileCount   int
	// NormLen is the canonical size - unlike a character count it does not move when the
	// formatting does.
	NormLen int
	Lines   int
	Kind    parser.BlockKind
	Depth   int
	// Statements is meaningful only when IsStatementBlock: an object literal or JSX element
	// has structure but no statements.
	Statements       int
	Tokens           int
	IsStatementBlock bool
	IsObjectLiteral  bool
	// Hash is the identity a snapshot records - see CanonicalHash.
	Hash string
	// Label is the first line of the first occurrence, for a human reading a snapshot diff.
	// It takes no part in identity.
	Label string
}

// ConfigLevelFilters are file-selection patterns plus the file they were written relative to.
//
// The patterns are never rewritten to another root - a bare `packages/app` means "this
// workspace" read from the config and "any nested directory called that" read from elsewhere,
// and no rewrite makes those the same pattern. The root travels instead.
type ConfigLevelFilters struct {
	// ConfigPath is the config FILE, not its directory, so a snapshot records which config
	// produced it.
	ConfigPath          string
	IgnoreFiles         []string
	ProcessIgnoredFiles []string
}

func (f ConfigLevelFilters) Root() string {
	if f.ConfigPath == "" {
		return ""
	}
	return filepath.Dir(f.ConfigPath)
}

func (f ConfigLevelFilters) IsSet() bool {
	return f.ConfigPath != "" && (len(f.IgnoreFiles) > 0 || len(f.ProcessIgnoredFiles) > 0)
}

// Options configures Detect.
type Options struct {
	Cwd string
	// Files, when set, is scanned instead of walking Cwd, so a config run reuses the list
	// every other check saw. Internal-normalised, as fs.GetFiles returns them.
	Files []string
	// Blinding is what this comparison ignores. Zero value is exact.
	Blinding Blinding
	// MinTokens is the size floor. Tokens rather than characters because a token count does
	// not move when a blinding is applied - one number means the same amount of code
	// whatever is ignored.
	MinTokens int
	MinLines  int
	// MinDepth and MinStatements filter on complexity rather than size: a three-key object
	// stays trivial however long its string values are, and no character count can tell.
	//
	// MinDepth counts the block itself, so 1 admits everything. MinStatements applies only to
	// statement blocks - an object literal or JSX element has no statements to be short of.
	MinDepth      int
	MinStatements int
	// SkipObjects drops duplications that are only object literals: their shape recurs across
	// unrelated code, so under structural comparison they are the main source of matches that
	// read as false positives.
	SkipObjects bool
	// MinDuplicates is how many copies a chunk needs to be reported. Raise to 3 to tolerate a
	// single copy-paste.
	MinDuplicates int
	// ProcessIgnoredFiles puts gitignored files back in scope. The config run's discovery
	// honours these, so this must too or the two see different files.
	ProcessIgnoredFiles []string
	// IgnoreFiles are applied BEFORE anything is counted, not to the finished report. A chunk
	// in three files with one ignored is a TWO-copy duplicate here, so at MinDuplicates 3 it
	// is not reported at all. Filtering the output instead would claim three copies with one
	// hidden - a different claim.
	IgnoreFiles []string
	// ConfigLevel are patterns written relative to somewhere other than Cwd, which is how a
	// run reproduces the file set a config run saw. See ConfigLevelFilters.
	ConfigLevel ConfigLevelFilters
	// SnapshotPath is not a detection setting, but it is the directory
	// ConfigLevel.ConfigPath is recorded relative to.
	SnapshotPath string
	// There is deliberately no file-size ceiling and no result cap.
	//
	// The ceiling was enforced here but not in the collector, so the two entry points disagreed
	// about which files were analysed and a snapshot from one read as resolved by the other. A
	// result cap broke a snapshot comparison the same way.

	// NeedHashes computes the digest and label of every reported duplication. It normalises
	// each block a second time - 44% of the reporting phase - so it is off unless a snapshot
	// or JSON output will read them.
	NeedHashes bool
	// trackBelowThreshold keeps chunks that ARE duplicated but have fewer copies than
	// MinDuplicates, for one reader: a snapshot delta.
	//
	// At MinDuplicates 3, deleting one of three copies makes a pattern vanish from the
	// report, and a delta with nothing to compare against can only call that "no longer
	// duplicated" - wrong, and it hides the two copies still there.
	//
	// Unexported: switched on by DetectWithBelowThreshold and nothing else.
	trackBelowThreshold bool
	// CountOnly drops the snippet text and per-occurrence line/column. Grouping and nesting
	// collapse still run, so the counts are unchanged - it is the presentation that goes.
	// File paths ARE still populated, because a snapshot records which files a duplicate
	// lives in and that has to work from a config run.
	CountOnly bool
}

// Stats is what a report says about the run beyond the findings themselves.
type Stats struct {
	Files        int
	IgnoredFiles int
	// Composition by block kind, which is what says whether SkipObjects is the right lever
	// for a project: if object literals are a few percent of the findings, it is not.
	ObjectFindings    int
	StatementFindings int
	JSXFindings       int
	OtherFindings     int

	Total time.Duration
}

const (
	// DefaultMinTokens is where the old 150-character default lands: over a 22k-file corpus
	// the median block runs 3.36 canonical characters per token, so the two floors admit the
	// same code. It keeps 18% of all repeated blocks.
	DefaultMinTokens = 50

	// DefaultMinLines is the smallest duplication worth reporting, in lines.
	DefaultMinLines = 3

	// hashShards splits the block table so it can be built and walked in parallel
	// without a lock. It must be a power of two.
	hashShards = 64
)

// blockRec is one scanned block, flattened so the matcher never touches the scan structures.
type blockRec struct {
	hash    uint64
	file    int32
	start   uint32
	end     uint32
	normLen uint32
	kind    parser.BlockKind
	depth   uint8
	stmts   uint16
	tokens  uint32
	isStmt  bool
	isObj   bool
}

type fileEntry struct {
	path    string // relative to cwd, for display
	content []byte
}

// Detect finds duplicated code under opts.Cwd.
func Detect(opts Options) ([]Duplication, Stats, error) {
	dups, _, stats, err := runDetect(opts)
	return dups, stats, err
}

// DetectWithBelowThreshold is Detect plus the chunks duplicated by fewer copies than
// MinDuplicates required. Those are not findings; they let a snapshot delta tell "this code
// is gone" from "this code is still here twice, and you asked to hear about three".
//
// At the default MinDuplicates of 2 the second result is always empty.
func DetectWithBelowThreshold(opts Options) ([]Duplication, []Duplication, Stats, error) {
	opts.trackBelowThreshold = true
	return runDetect(opts)
}

func runDetect(opts Options) ([]Duplication, []Duplication, Stats, error) {
	start := time.Now()
	var stats Stats

	opts = opts.Normalized()

	paths := opts.Files
	if paths == nil {
		// Each set of patterns is matched from the directory it was written in, which is
		// what makes a config's top-level patterns mean the same thing here as they did
		// there.
		processIgnored := globutil.CreateGlobMatchers(opts.ProcessIgnoredFiles, opts.Cwd)
		if opts.ConfigLevel.IsSet() {
			processIgnored = append(processIgnored,
				globutil.CreateGlobMatchers(opts.ConfigLevel.ProcessIgnoredFiles, opts.ConfigLevel.Root())...)
		}
		paths = fs.GetFiles(
			opts.Cwd,
			[]string{},
			fs.FindAndProcessGitIgnoreFilesUpToRepoRoot(opts.Cwd),
			processIgnored,
		)
	}
	paths, stats.IgnoredFiles = applyIgnoreFiles(paths, opts.IgnoreFiles, opts.Cwd)
	if opts.ConfigLevel.IsSet() {
		var configIgnored int
		paths, configIgnored = applyIgnoreFiles(paths, opts.ConfigLevel.IgnoreFiles, opts.ConfigLevel.Root())
		stats.IgnoredFiles += configIgnored
	}
	if len(paths) == 0 {
		stats.Total = time.Since(start)
		return nil, nil, stats, nil
	}
	files, shards, scannedFiles := scanAll(paths, opts)
	stats.Files = scannedFiles
	groups, belowGroups := matchAll(files, shards, opts)
	dups := buildReport(files, groups, opts)
	below := buildBelowThresholdReport(files, belowGroups, opts)
	countComposition(dups, &stats)

	stats.Total = time.Since(start)
	return dups, below, stats, nil
}

// buildBelowThresholdReport renders the sub-threshold groups in a separate pass, so a chunk
// nobody asked to hear about never joins the nesting collapse and marks a real finding
// redundant.
func buildBelowThresholdReport(files []fileEntry, groups []group, opts Options) []Duplication {
	if len(groups) == 0 {
		return nil
	}
	return buildReport(files, groups, opts)
}

// ---------------------------------------------------------------------------
// phase 1: read, canonicalise, hash
// ---------------------------------------------------------------------------

// allowsJSX is false for .ts/.mts/.cts, where '<' is always a type argument or assertion and
// treating it as JSX would wreck the scan.
func allowsJSX(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".mts", ".cts":
		return false
	}
	return true
}

// scanAll returns the file entries, the blocks sharded by hash, and how many files were read -
// which can be short of paths, since an unreadable file is skipped.
func scanAll(paths []string, opts Options) ([]fileEntry, [][]blockRec, int) {
	workers := runtime.GOMAXPROCS(0)
	// paths is never empty - runDetect returns before this - so workers is at least 1.
	if workers > len(paths) {
		workers = len(paths)
	}

	files := make([]fileEntry, len(paths))

	// Each worker fills its own per-shard buckets, so nothing needs a lock until they are
	// stitched together below.
	perWorker := make([][][]blockRec, workers)
	var totalFiles int64

	// A shared cursor rather than a fixed split: files differ in size by orders of
	// magnitude, so a worker drawing small files simply comes back sooner.
	var cursor int64
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()

			buckets := make([][]blockRec, hashShards)
			scan := &parser.BlockScan{}
			hasher := &blockHasher{}
			var hashes []uint64
			var localFiles int64

			for {
				idx := int(atomic.AddInt64(&cursor, 1)) - 1
				if idx >= len(paths) {
					break
				}
				path := paths[idx]

				content, err := os.ReadFile(pathutil.DenormalizePathForOS(path))
				if err != nil {
					continue
				}
				files[idx] = fileEntry{path: path, content: content}
				localFiles++

				parser.ScanCodeBlocks(content, parser.BlockScanOptions{
					Blinding:           opts.Blinding,
					AllowJSX:           allowsJSX(path),
					MinTokens:          opts.MinTokens,
					MinDepth:           opts.MinDepth,
					MinStatements:      opts.MinStatements,
					SkipObjectLiterals: opts.SkipObjects,
				}, scan)
				if len(scan.Blocks) == 0 {
					continue
				}

				hashes = hasher.hashSpans(scan.Norm, scan.Blocks, false, hashes)

				for i, b := range scan.Blocks {
					h := hashes[i]
					shard := h & (hashShards - 1)
					buckets[shard] = append(buckets[shard], blockRec{
						hash:    h,
						file:    int32(idx),
						start:   b.Start,
						end:     b.End,
						normLen: b.NormEnd - b.NormStart,
						kind:    b.Kind,
						depth:   b.Depth,
						stmts:   b.Statements,
						tokens:  b.Tokens,
						isStmt:  b.IsStatementBlock(),
						isObj:   b.IsObjectLiteral(),
					})
				}
			}

			perWorker[w] = buckets
			atomic.AddInt64(&totalFiles, localFiles)
		}(w)
	}
	wg.Wait()

	// Concatenate each shard across workers. Only the slice headers move.
	shards := make([][]blockRec, hashShards)
	for s := 0; s < hashShards; s++ {
		n := 0
		for w := range perWorker {
			if perWorker[w] != nil {
				n += len(perWorker[w][s])
			}
		}
		if n == 0 {
			continue
		}
		merged := make([]blockRec, 0, n)
		for w := range perWorker {
			if perWorker[w] != nil {
				merged = append(merged, perWorker[w][s]...)
			}
		}
		shards[s] = merged
	}

	return files, shards, int(totalFiles)
}

// ---------------------------------------------------------------------------
// rolling hash over the canonical buffer
// ---------------------------------------------------------------------------

const hashBase uint64 = 1099511628211

// blockHasher computes the polynomial hash of every block in a file.
//
// The obvious implementation tabulates the prefix hash of the whole canonical buffer, making
// any span an O(1) subtraction. That costs sixteen bytes of table per canonical byte and was
// 58% of everything this package allocated.
//
// A block's hash needs the running prefix at two positions, its start and its end - two per
// block, not one per byte. So collect those positions, walk the buffer once accumulating a
// single running value, and remember it only where asked. Scratch shrinks from O(file) to
// O(blocks).
type blockHasher struct {
	positions []uint32
	values    []uint64
}

// hashSpans hashes every block against one canonical form. alt selects the structural
// span, so a single block list can be hashed against both forms of the same file.
func (h *blockHasher) hashSpans(norm []byte, blocks []parser.CodeBlock, alt bool, out []uint64) []uint64 {
	if len(blocks) == 0 {
		return out[:0]
	}

	h.positions = h.positions[:0]
	for _, b := range blocks {
		if alt {
			h.positions = append(h.positions, b.AltNormStart, b.AltNormEnd)
		} else {
			h.positions = append(h.positions, b.NormStart, b.NormEnd)
		}
	}
	slices.Sort(h.positions)
	h.positions = slices.Compact(h.positions)

	// One pass over the canonical bytes, remembering the running hash only at the
	// positions some block starts or ends at.
	h.values = h.values[:0]
	var running uint64
	next := 0
	for i := 0; i <= len(norm) && next < len(h.positions); i++ {
		for next < len(h.positions) && int(h.positions[next]) == i {
			h.values = append(h.values, running)
			next++
		}
		if i < len(norm) {
			running = running*hashBase + uint64(norm[i]) + 1
		}
	}

	out = out[:0]
	for _, b := range blocks {
		start, end := b.NormStart, b.NormEnd
		if alt {
			start, end = b.AltNormStart, b.AltNormEnd
		}
		out = append(out, spanHash(h.valueAt(start), h.valueAt(end), int(end-start)))
	}
	return out
}

func (h *blockHasher) valueAt(position uint32) uint64 {
	idx, _ := slices.BinarySearch(h.positions, position)
	return h.values[idx]
}

func spanHash(from, to uint64, length int) uint64 {
	h := to - from*powBase(length)
	// Fold the length in so two spans cannot collide just because the polynomial wrapped,
	// then avalanche: the low bits pick the shard.
	return mix64(h ^ (uint64(length) * 0x9e3779b97f4a7c15))
}
func powBase(n int) uint64 {
	result := uint64(1)
	base := hashBase
	for n > 0 {
		if n&1 == 1 {
			result *= base
		}
		base *= base
		n >>= 1
	}
	return result
}

func mix64(h uint64) uint64 {
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return h
}

// ---------------------------------------------------------------------------
// phase 2: bucket and confirm
// ---------------------------------------------------------------------------

// group is a confirmed set of blocks that compare equal.
type group []blockRec

func matchAll(files []fileEntry, shards [][]blockRec, opts Options) ([]group, []group) {
	workers := runtime.GOMAXPROCS(0)

	results := make([][]group, hashShards)
	belowResults := make([][]group, hashShards)

	// Shards are drawn from a shared cursor for the same reason files are: their sizes
	// are not known ahead of time and a worker that finishes early takes the next one.
	var cursor int64
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// first maps a hash to the only block seen with it so far. Most hashes end
			// there, so the slice-allocating map is only paid for by hashes that repeat.
			first := make(map[uint64]int32)
			repeats := make(map[uint64][]int32)

			for {
				s := int(atomic.AddInt64(&cursor, 1)) - 1
				if s >= hashShards {
					break
				}
				recs := shards[s]
				if len(recs) < 2 {
					continue
				}

				clear(first)
				clear(repeats)
				for i := range recs {
					h := recs[i].hash
					if prev, ok := first[h]; ok {
						list, seen := repeats[h]
						if !seen {
							list = append(list, prev)
						}
						repeats[h] = append(list, int32(i))
						continue
					}
					first[h] = int32(i)
				}
				if len(repeats) == 0 {
					continue
				}

				var out, below []group
				for _, idxs := range repeats {
					out, below = confirmBucket(files, recs, idxs, opts, out, below)
				}
				results[s] = out
				belowResults[s] = below
			}
		}()
	}
	wg.Wait()

	var all, below []group
	for s := 0; s < hashShards; s++ {
		all = append(all, results[s]...)
		below = append(below, belowResults[s]...)
	}
	return all, below
}

// confirmBucket splits one hash bucket into sets that genuinely compare equal. Sharing a hash
// is evidence, not proof, so a collision produces a second set rather than a wrong group.
func confirmBucket(files []fileEntry, recs []blockRec, idxs []int32, opts Options, out, below []group) ([]group, []group) {
	// Nearly every bucket is a single set, so a slice scanned linearly beats a map.
	sets := make([]group, 0, 1)

	for _, i := range idxs {
		r := recs[i]
		text := files[r.file].content[r.start:r.end]

		placed := false
		for si := range sets {
			rep := sets[si][0]
			if Compare(files[rep.file].content[rep.start:rep.end], text, opts.Blinding) {
				sets[si] = append(sets[si], r)
				placed = true
				break
			}
		}
		if !placed {
			sets = append(sets, group{r})
		}
	}

	for _, s := range sets {
		switch {
		case len(s) >= opts.MinDuplicates:
			out = append(out, s)
		// A set of one is a hash collision, not a duplicate, so the floor here is two
		// however low MinDuplicates is set - and at the default of two nothing lands here
		// at all.
		case opts.trackBelowThreshold && len(s) >= 2:
			below = append(below, s)
		}
	}
	return out, below
}

// ---------------------------------------------------------------------------
// phase 3: shape the result
// ---------------------------------------------------------------------------

func buildReport(files []fileEntry, groups []group, opts Options) []Duplication {
	if len(groups) == 0 {
		return nil
	}

	// Order the copies within each group first, so the group sort below can fall back on a
	// property of the group itself. Nothing upstream provides one: matchAll ranges over a map
	// and Go randomises that per run.
	for _, g := range groups {
		sortGroup(files, g)
	}

	// Largest first, so a duplicated function is reported ahead of the blocks nested inside it,
	// which lets those be recognised as redundant below. slices.SortStableFunc rather than
	// sort.SliceStable: the latter's reflection-based swapper was a fifth of the reporting phase
	// at a few thousand findings.
	slices.SortStableFunc(groups, func(a, b group) int {
		if a[0].normLen != b[0].normLen {
			return int(b[0].normLen) - int(a[0].normLen)
		}
		if len(a) != len(b) {
			return len(b) - len(a)
		}
		// A stable sort leaves ties in arrival order, which here is the map's. Position decides
		// which of two equal-sized groups is accepted first, and so which collapses the other.
		return comparePosition(files, a[0], b[0])
	})

	type span struct {
		start, end uint32
		group      int
	}
	covered := make(map[int32][]span)
	enclosing := make(map[int]int) // accepted group -> how many of this group's copies it contains

	dups := make([]Duplication, 0, len(groups))

	for _, g := range groups {
		// A block appearing only inside a larger reported block, in the same number of copies, is
		// the same copy-paste seen from closer up.
		//
		// Every enclosing span is counted, not just the first: a copy can sit inside several
		// accepted blocks, and not the same ones from file to file. What matters is whether ONE
		// accepted group accounts for every copy.
		clear(enclosing)
		for _, r := range g {
			for _, sp := range covered[r.file] {
				if sp.start <= r.start && r.end <= sp.end {
					enclosing[sp.group]++
				}
			}
		}
		redundant := false
		for gid, n := range enclosing {
			if n == len(g) && len(dups[gid].Occurrences) >= len(g) {
				redundant = true
				break
			}
		}
		if redundant {
			continue
		}

		firstRec := g[0]
		body := files[firstRec.file].content[firstRec.start:firstRec.end]
		// Counting newlines inside the block is bounded by the block; finding a line NUMBER is
		// bounded by the file, which is why that is optional below.
		lines := 1 + bytes.Count(body, []byte{'\n'})
		if lines < opts.MinLines {
			continue
		}
		snippet := ""
		if !opts.CountOnly {
			snippet = string(body)
		}
		// Both exist for snapshots alone, and each costs a walk of the block.
		hash, label := "", ""
		if opts.NeedHashes {
			hash = CanonicalHash(body, opts.Blinding)
			label = firstLineLabel(body)
		}

		occ := make([]Occurrence, 0, len(g))
		distinctFiles := 0
		var prevFile int32 = -1
		for _, r := range g {
			occ = append(occ, Occurrence{
				FileID: r.file,
				File:   relativePath(opts.Cwd, files[r.file].path),
				Start:  r.start,
				End:    r.end,
			})
			if r.file != prevFile {
				distinctFiles++
				prevFile = r.file
			}
		}

		idx := len(dups)
		dups = append(dups, Duplication{
			Snippet:          snippet,
			Occurrences:      occ,
			FileCount:        distinctFiles,
			NormLen:          int(firstRec.normLen),
			Lines:            lines,
			Kind:             firstRec.kind,
			Depth:            int(firstRec.depth),
			Statements:       int(firstRec.stmts),
			Tokens:           int(firstRec.tokens),
			IsStatementBlock: firstRec.isStmt,
			IsObjectLiteral:  firstRec.isObj,
			Hash:             hash,
			Label:            label,
		})
		for _, r := range g {
			covered[r.file] = append(covered[r.file], span{r.start, r.end, idx})
		}
	}

	// Most copies first, then the biggest. The last comparison is not cosmetic: hundreds of
	// findings tie on the first two, and a stable sort would leave them in map order, so the
	// tail of the report reshuffles on every run over unchanged code. Position is the tiebreak
	// because it is the one thing that does not move when the run does.
	slices.SortStableFunc(dups, func(a, b Duplication) int {
		if len(a.Occurrences) != len(b.Occurrences) {
			return len(b.Occurrences) - len(a.Occurrences)
		}
		if a.NormLen != b.NormLen {
			return b.NormLen - a.NormLen
		}
		if c := strings.Compare(a.Occurrences[0].File, b.Occurrences[0].File); c != 0 {
			return c
		}
		return int(a.Occurrences[0].Start) - int(b.Occurrences[0].Start)
	})

	if !opts.CountOnly {
		fillLineNumbers(files, dups)
	}
	return dups
}

// fillLineNumbers resolves every occurrence to a line and column.
//
// One at a time this is the most expensive thing in reporting - a line number means counting
// newlines from the start of the file, so a bundle with a hundred occurrences gets walked a
// hundred times. Gathering per file and sorting by offset walks each file once.
func fillLineNumbers(files []fileEntry, dups []Duplication) {
	// Both ends of each occurrence are queued, so the file is still walked once.
	type ref struct {
		dup, occ int
		offset   uint32
		isEnd    bool
	}
	byFile := map[int32][]ref{}
	for di := range dups {
		for oi := range dups[di].Occurrences {
			o := dups[di].Occurrences[oi]
			byFile[o.FileID] = append(byFile[o.FileID],
				ref{di, oi, o.Start, false},
				// End is exclusive; the last character is the byte before it.
				ref{di, oi, o.End - 1, true})
		}
	}

	for id, refs := range byFile {
		content := files[id].content
		sort.Slice(refs, func(a, b int) bool { return refs[a].offset < refs[b].offset })

		line, lineStart, cursor := 1, 0, 0
		for _, r := range refs {
			o := &dups[r.dup].Occurrences[r.occ]
			target := int(r.offset)
			if target > len(content) {
				continue
			}
			// The cursor only moves forward: offsets were sorted, so the file is walked
			// once no matter how many positions it holds.
			for cursor < target {
				if content[cursor] == '\n' {
					line++
					lineStart = cursor + 1
				}
				cursor++
			}
			if r.isEnd {
				o.EndLine, o.EndCol = line, target-lineStart+1
			} else {
				o.Line, o.Col = line, target-lineStart+1
			}
		}
	}
}

// sortGroup makes g[0] the same copy on every run, giving the group a stable identity.
func sortGroup(files []fileEntry, g group) {
	slices.SortFunc(g, func(x, y blockRec) int { return comparePosition(files, x, y) })
}

// comparePosition orders by file PATH then offset. Path rather than file id: ids come from
// the walk order, which says nothing to a reader.
func comparePosition(files []fileEntry, x, y blockRec) int {
	if x.file != y.file {
		return strings.Compare(files[x.file].path, files[y.file].path)
	}
	return int(x.start) - int(y.start)
}

func relativePath(cwd, path string) string {
	cwd = pathutil.NormalizePathForInternal(cwd)
	if !strings.HasSuffix(cwd, "/") {
		cwd += "/"
	}
	return strings.TrimPrefix(path, cwd)
}

// applyIgnoreFiles drops matching paths before a byte is scanned, so an ignored file
// contributes no blocks and every count downstream - occurrences, files, MinDuplicates - is
// computed as though it were not in the project.
func applyIgnoreFiles(paths []string, patterns []string, cwd string) ([]string, int) {
	if len(patterns) == 0 {
		return paths, 0
	}
	matchers := globutil.CreateGlobMatchers(patterns, cwd)
	if len(matchers) == 0 {
		return paths, 0
	}

	kept := make([]string, 0, len(paths))
	for _, path := range paths {
		if globutil.MatchesAnyGlobMatcher(path, matchers, false) {
			continue
		}
		kept = append(kept, path)
	}
	return kept, len(paths) - len(kept)
}

// firstLineLabel is cosmetic: nothing compares against it.
func firstLineLabel(body []byte) string {
	// Walking the lines rather than splitting them - materialising every line to throw all
	// but one away showed up in a heap profile.
	for start := 0; start < len(body); {
		end := bytes.IndexByte(body[start:], '\n')
		var line []byte
		if end < 0 {
			line = body[start:]
			start = len(body)
		} else {
			line = body[start : start+end]
			start += end + 1
		}
		trimmed := bytes.TrimSpace(line)
		// The opening brace on its own says nothing; take the first line with content.
		if len(trimmed) == 0 || (len(trimmed) == 1 && trimmed[0] == '{') {
			continue
		}
		if len(trimmed) > labelMaxLen {
			return string(trimmed[:labelMaxLen]) + "..."
		}
		return string(trimmed)
	}
	return ""
}

// Normalized fills in the defaults an unset option implies.
//
// Exported because a snapshot records the settings a run ACTUALLY used: recording raw values
// would make an unset field look like a changed setting next time the two are compared.
func (o Options) Normalized() Options {
	if o.MinTokens <= 0 {
		o.MinTokens = DefaultMinTokens
	}
	if o.MinLines <= 0 {
		o.MinLines = DefaultMinLines
	}
	if o.MinDuplicates < 2 {
		o.MinDuplicates = 2
	}
	return o
}

// countComposition tallies findings by block kind.
func countComposition(dups []Duplication, stats *Stats) {
	for _, d := range dups {
		switch {
		case d.Kind == parser.BlockJSX:
			stats.JSXFindings++
		case d.IsObjectLiteral:
			stats.ObjectFindings++
		case d.IsStatementBlock:
			stats.StatementFindings++
		default:
			stats.OtherFindings++
		}
	}
}
