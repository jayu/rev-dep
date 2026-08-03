package dupcode

import (
	"sync"
	"time"

	"rev-dep-go/internal/parser"
)

// A Collector lets duplicate detection ride along on a traversal someone else is already doing.
//
// In a config run the import pass has already read and tokenised the same files. Collector
// implements parser.BlockSink so the scan results come out of that pass, leaving only matching
// and reporting.
//
// One Collector serves every rule. Rules differ in their thresholds, so the scan uses the most
// permissive settings any of them asked for and each rule filters down to its own at match time.
type Collector struct {
	// One spec per distinct comparison mode in use.
	specs []collectorSpec
	files []fileEntry

	hasherPool sync.Pool
}

type collectorSpec struct {
	blinding Blinding
	// The loosest any rule asked for, so a rule with stricter ones is still served from the same
	// blocks.
	minTokens     int
	minDepth      int
	minStatements int
	// Indexed by file, so the goroutine handling a file writes only its own slot.
	recs [][]blockRec
}

type CollectorSpec struct {
	Blinding      Blinding
	MinTokens     int
	MinDepth      int
	MinStatements int
	SkipObjects   bool
}

// NewCollector merges specs sharing a mode, keeping the loosest floor of each, so two rules
// differing only in strictness cost one scan.
func NewCollector(specs []CollectorSpec, fileCount int) *Collector {
	c := &Collector{}
	for _, s := range specs {
		if s.MinTokens <= 0 {
			s.MinTokens = DefaultMinTokens
		}
		if existing := c.specIndex(s.Blinding); existing >= 0 {
			spec := &c.specs[existing]
			spec.minTokens = min(spec.minTokens, s.MinTokens)
			spec.minDepth = min(spec.minDepth, s.MinDepth)
			spec.minStatements = min(spec.minStatements, s.MinStatements)
			continue
		}
		c.specs = append(c.specs, collectorSpec{
			blinding:      s.Blinding,
			minTokens:     s.MinTokens,
			minDepth:      s.MinDepth,
			minStatements: s.MinStatements,
			recs:          make([][]blockRec, fileCount),
		})
	}
	c.files = make([]fileEntry, fileCount)
	return c
}

func (c *Collector) specIndex(blinding Blinding) int {
	for i := range c.specs {
		if c.specs[i].blinding == blinding {
			return i
		}
	}
	return -1
}

// pairedSpecs returns the two specs one scan can serve together: they must differ only in
// whether identifiers are blinded, since the alternate form can only ever be a coarsening of
// the primary. primary is the one that keeps names.
func (c *Collector) pairedSpecs() (primary, alt int) {
	if len(c.specs) != 2 {
		return -1, -1
	}
	a, b := c.specs[0].blinding, c.specs[1].blinding
	if a.Identifiers == b.Identifiers || a.Strings != b.Strings || a.Numbers != b.Numbers {
		return -1, -1
	}
	if a.Identifiers {
		return 1, 0
	}
	return 0, 1
}

// Passes implements parser.BlockSink. Two comparisons differing only in name blinding share one
// pass; any other pair costs a pass each.
func (c *Collector) Passes() []parser.BlockScanOptions {
	// SkipObjectLiterals is deliberately NOT set. One scan serves every detection in the run and
	// they need not agree, so either answer would be wrong for one of them. Objects are dropped per
	// detection in Detect instead, exactly as the size and complexity floors are.
	if p, a := c.pairedSpecs(); p >= 0 {
		return []parser.BlockScanOptions{{
			Blinding:       c.specs[p].blinding,
			AltIdentifiers: true,
			// One floor serves both forms.
			MinTokens:     min(c.specs[p].minTokens, c.specs[a].minTokens),
			MinDepth:      min(c.specs[p].minDepth, c.specs[a].minDepth),
			MinStatements: min(c.specs[p].minStatements, c.specs[a].minStatements),
		}}
	}

	out := make([]parser.BlockScanOptions, 0, len(c.specs))
	for _, s := range c.specs {
		out = append(out, parser.BlockScanOptions{
			Blinding:      s.blinding,
			MinTokens:     s.minTokens,
			MinDepth:      s.minDepth,
			MinStatements: s.minStatements,
		})
	}
	return out
}

func (c *Collector) AllowJSX(path string) bool { return allowsJSX(path) }

// Collect implements parser.BlockSink. It runs on the parsing goroutine, so it touches only its
// own slot.
func (c *Collector) Collect(pass, idx int, path string, content []byte, scan *parser.BlockScan) {
	if idx < 0 || idx >= len(c.files) {
		return
	}
	paired, pairedAlt := c.pairedSpecs()
	if paired < 0 && (pass < 0 || pass >= len(c.specs)) {
		return
	}
	// Further passes are the same file under another mode.
	if pass == 0 {
		c.files[idx] = fileEntry{path: path, content: content}
	}
	if len(scan.Blocks) == 0 {
		return
	}

	// The hasher's scratch is pooled: Collect runs on whichever parsing goroutine read the file, so
	// there is no worker to hang it off.
	hasher, _ := c.hasherPool.Get().(*blockHasher)
	if hasher == nil {
		hasher = &blockHasher{}
	}
	if paired >= 0 {
		// One scan, two canonical forms: a record under each spec from the same blocks.
		c.fileRecords(paired, idx, hasher, scan.Norm, scan.Blocks, false)
		c.fileRecords(pairedAlt, idx, hasher, scan.NormAlt, scan.Blocks, true)
	} else {
		c.fileRecords(pass, idx, hasher, scan.Norm, scan.Blocks, false)
	}

	c.hasherPool.Put(hasher)
}

// Detect matches and reports over the already-collected blocks, restricted to files: blocks
// from anything outside the rule's own list are ignored.
func (c *Collector) Detect(opts Options, files []string) ([]Duplication, Stats, error) {
	dups, _, stats, err := c.runDetect(opts, files)
	return dups, stats, err
}

// See the standalone DetectWithBelowThreshold.
func (c *Collector) DetectWithBelowThreshold(opts Options, files []string) ([]Duplication, []Duplication, Stats, error) {
	opts.trackBelowThreshold = true
	return c.runDetect(opts, files)
}

func (c *Collector) runDetect(opts Options, files []string) ([]Duplication, []Duplication, Stats, error) {
	start := time.Now()
	var stats Stats

	spec := c.specIndex(opts.Blinding)
	if spec < 0 {
		// Nothing collected for this mode; fall back rather than silently reporting zero.
		opts.Files = files
		return runDetect(opts)
	}

	opts = opts.Normalized()

	// Ignored files leave the rule's scope before any block is bucketed. Blocks were still scanned
	// for them during the shared pass, because another rule may not ignore the same paths - dropping
	// them here is what keeps the rules independent while sharing one scan.
	files, ignored := applyIgnoreFiles(files, opts.IgnoreFiles, opts.Cwd)
	stats.IgnoredFiles = ignored

	inScope := make(map[string]struct{}, len(files))
	for _, f := range files {
		inScope[f] = struct{}{}
	}

	// Re-sharding walks block records rather than file bytes, a fraction of a fresh scan.
	shards := make([][]blockRec, hashShards)
	scanned := 0
	for idx, recs := range c.specs[spec].recs {
		entry := c.files[idx]
		if entry.path == "" {
			continue
		}
		if _, ok := inScope[entry.path]; !ok {
			continue
		}
		scanned++
		for _, r := range recs {
			// A block below this rule's floors was collected for a looser rule.
			if int(r.tokens) < opts.MinTokens || int(r.depth) < opts.MinDepth {
				continue
			}
			// The statement floor asks about statement blocks only, as the scanner does for a standalone
			// run.
			if r.isStmt && int(r.stmts) < opts.MinStatements {
				continue
			}
			// Excluded before sharding, so an object literal neither gets reported nor suppresses a real
			// duplication nested inside it.
			if opts.SkipObjects && r.isObj {
				continue
			}
			shard := r.hash & (hashShards - 1)
			shards[shard] = append(shards[shard], r)
		}
	}

	stats.Files = scanned
	groups, belowGroups := matchAll(c.files, shards, opts)
	dups := buildReport(c.files, groups, opts)
	below := buildBelowThresholdReport(c.files, belowGroups, opts)
	countComposition(dups, &stats)
	stats.Total = time.Since(start)

	return dups, below, stats, nil
}

// fileRecords hashes one file's blocks against one canonical form. alt selects which span, so
// the same block list serves both modes.
func (c *Collector) fileRecords(
	spec, idx int,
	hasher *blockHasher,
	norm []byte,
	blocks []parser.CodeBlock,
	alt bool,
) {
	if spec < 0 {
		return
	}
	s := &c.specs[spec]

	hashes := hasher.hashSpans(norm, blocks, alt, nil)
	recs := make([]blockRec, 0, len(blocks))
	for i, b := range blocks {
		normLen := b.NormEnd - b.NormStart
		if alt {
			normLen = b.AltNormEnd - b.AltNormStart
		}
		// The shared scan admits a block clearing the loosest floor asked for, so each spec drops what
		// is below its own.
		if int(b.Tokens) < s.minTokens {
			continue
		}
		recs = append(recs, blockRec{
			hash:    hashes[i],
			file:    int32(idx),
			start:   b.Start,
			end:     b.End,
			normLen: normLen,
			kind:    b.Kind,
			depth:   b.Depth,
			stmts:   b.Statements,
			tokens:  b.Tokens,
			isStmt:  b.IsStatementBlock(),
			isObj:   b.IsObjectLiteral(),
		})
	}
	s.recs[idx] = recs
}
