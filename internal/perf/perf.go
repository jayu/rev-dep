// Package perf provides opt-in timing instrumentation for the processing pipeline.
//
// It is enabled with the REVDEP_PERF environment variable and is a no-op otherwise, so an
// instrumented call site costs one boolean check in a normal run:
//
//	REVDEP_PERF=1   phase-level spans (cheap; a few dozen timer calls per run)
//	REVDEP_PERF=2   adds deep spans on hot inner loops (per-file / per-import). These call
//	                time.Now() often enough to inflate the very numbers they measure, so
//	                treat level 2 as a relative breakdown, not an absolute one.
//
// Spans are identified by a slash-separated path ("resolve-imports/resolver-manager"),
// which Report renders as a tree. Paths are used instead of goroutine-local call-stack
// nesting because the pipeline fans out across goroutines, where a stack cannot be
// tracked reliably. A span is registered when it starts, so the tree renders in
// execution order.
package perf

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	levelOff = iota
	levelPhases
	levelDeep
)

var (
	level int

	mu    sync.Mutex
	spans = map[string]*span{}
	seq   int
)

// span aggregates every timing recorded for one path. sum can exceed wall-clock time when
// the path is measured from several goroutines at once, so max and count are kept too:
// together they distinguish "one slow call" from "many concurrent calls".
//
// sum/max/count are atomics rather than plain fields guarded by the package mutex. Deep
// spans are recorded from every worker goroutine several times per file, and a global
// mutex there produced a lock convoy large enough to distort the measurements it was
// taking. Only registration (touch) still takes the mutex, and that happens once per path.
type span struct {
	path  string
	sum   atomic.Int64
	max   atomic.Int64
	count atomic.Int64
	order int
}

// Init reads REVDEP_PERF and sets the instrumentation level. Call it once at startup.
func Init() {
	switch strings.TrimSpace(os.Getenv("REVDEP_PERF")) {
	case "", "0", "false", "off":
		level = levelOff
	case "2", "deep":
		level = levelDeep
	default:
		level = levelPhases
	}
}

// Enabled reports whether any timing is being collected.
func Enabled() bool { return level >= levelPhases }

// Deep reports whether hot-loop instrumentation is enabled (REVDEP_PERF=2).
func Deep() bool { return level >= levelDeep }

var noop = func() {}

// Track starts a span for path and returns the function that ends it, so a whole phase
// reads as `defer perf.Track("discover")()`.
func Track(path string) func() {
	if level < levelPhases {
		return noop
	}
	return start(path)
}

// TrackDeep is Track for hot inner loops: it only measures at REVDEP_PERF=2.
func TrackDeep(path string) func() {
	if level < levelDeep {
		return noop
	}
	return start(path)
}

// Record adds an already-measured duration, for regions that cannot be expressed as a
// single deferred call (e.g. time accumulated across the iterations of a loop).
func Record(path string, duration time.Duration) {
	if level < levelPhases {
		return
	}
	touch(path).add(duration)
}

// RecordDeep is Record for hot inner loops: it only records at REVDEP_PERF=2.
func RecordDeep(path string, duration time.Duration) {
	if level < levelDeep {
		return
	}
	touch(path).add(duration)
}

func start(path string) func() {
	entry := touch(path)
	startedAt := time.Now()
	return func() { entry.add(time.Since(startedAt)) }
}

// touch returns the span for path, registering it on first use. Registering at span start
// (rather than at completion) is what makes Report's ordering match execution order:
// a parent always starts before the children it wraps.
func touch(path string) *span {
	mu.Lock()
	defer mu.Unlock()
	entry, exists := spans[path]
	if !exists {
		entry = &span{path: path, order: seq}
		seq++
		spans[path] = entry
	}
	return entry
}

func (s *span) add(duration time.Duration) {
	s.sum.Add(int64(duration))
	s.count.Add(1)
	for {
		current := s.max.Load()
		if int64(duration) <= current || s.max.CompareAndSwap(current, int64(duration)) {
			return
		}
	}
}

// Reset drops all collected spans. Used by tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	spans = map[string]*span{}
	seq = 0
}

type node struct {
	name     string
	span     *span
	children []*node
	order    int
}

// buildTree assembles the recorded paths into a tree, synthesising any parent that was
// never measured directly so that a child path like "a/b" still renders under "a".
func buildTree() *node {
	mu.Lock()
	defer mu.Unlock()

	root := &node{order: -1}
	byPath := map[string]*node{"": root}

	paths := make([]string, 0, len(spans))
	for path := range spans {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool { return spans[paths[i]].order < spans[paths[j]].order })

	for _, path := range paths {
		segments := strings.Split(path, "/")
		parentPath := ""
		for depth, segment := range segments {
			currentPath := segment
			if parentPath != "" {
				currentPath = parentPath + "/" + segment
			}
			current, exists := byPath[currentPath]
			if !exists {
				// A synthesised parent inherits the order of the first child that needed
				// it, keeping it next to the subtree it introduces.
				current = &node{name: segment, order: spans[path].order}
				byPath[currentPath] = current
				parent := byPath[parentPath]
				parent.children = append(parent.children, current)
			}
			if depth == len(segments)-1 {
				current.span = spans[path]
				current.order = spans[path].order
			}
			parentPath = currentPath
		}
	}

	sortChildren(root)
	return root
}

func sortChildren(parent *node) {
	sort.SliceStable(parent.children, func(i, j int) bool {
		return parent.children[i].order < parent.children[j].order
	})
	for _, child := range parent.children {
		sortChildren(child)
	}
}

// Report prints the collected spans as a tree, with each span's share of total. It is a
// no-op when instrumentation is disabled.
func Report(total time.Duration) {
	if level < levelPhases {
		return
	}

	root := buildTree()
	if len(root.children) == 0 {
		return
	}

	labelWidth := 0
	var measureLabels func(parent *node, depth int)
	measureLabels = func(parent *node, depth int) {
		for _, child := range parent.children {
			if width := depth*2 + len(child.name); width > labelWidth {
				labelWidth = width
			}
			measureLabels(child, depth+1)
		}
	}
	measureLabels(root, 0)
	if labelWidth < 24 {
		labelWidth = 24
	}

	fmt.Printf("\n⏱  Timing breakdown (REVDEP_PERF=%d)\n\n", level)
	fmt.Printf("  %-*s  %10s  %7s  %6s  %10s\n", labelWidth, "PHASE", "TIME", "SHARE", "CALLS", "SLOWEST")
	fmt.Printf("  %s\n", strings.Repeat("-", labelWidth+40))

	topLevelSum := time.Duration(0)
	for _, child := range root.children {
		if child.span != nil {
			topLevelSum += time.Duration(child.span.sum.Load())
		}
	}

	var print func(parent *node, depth int)
	print = func(parent *node, depth int) {
		for _, child := range parent.children {
			label := strings.Repeat("  ", depth) + child.name
			if child.span == nil {
				fmt.Printf("  %-*s  %10s  %7s  %6s  %10s\n", labelWidth, label, "-", "-", "-", "-")
			} else {
				spanSum := time.Duration(child.span.sum.Load())
				spanCount := child.span.count.Load()
				share := "-"
				if total > 0 {
					share = fmt.Sprintf("%.1f%%", 100*float64(spanSum)/float64(total))
				}
				calls := fmt.Sprintf("%d", spanCount)
				slowest := "-"
				if spanCount > 1 {
					slowest = formatDuration(time.Duration(child.span.max.Load()))
				}
				fmt.Printf("  %-*s  %10s  %7s  %6s  %10s\n",
					labelWidth, label, formatDuration(spanSum), share, calls, slowest)
			}
			print(child, depth+1)
		}
	}
	print(root, 0)

	fmt.Printf("  %s\n", strings.Repeat("-", labelWidth+40))
	fmt.Printf("  %-*s  %10s\n", labelWidth, "measured (top level)", formatDuration(topLevelSum))
	fmt.Printf("  %-*s  %10s\n", labelWidth, "total run", formatDuration(total))
	if unaccounted := total - topLevelSum; unaccounted > 0 {
		fmt.Printf("  %-*s  %10s\n", labelWidth, "unaccounted", formatDuration(unaccounted))
	}

	fmt.Printf("\n  TIME sums every call, so a phase whose CALLS run concurrently can exceed\n")
	fmt.Printf("  wall-clock time; compare it against SLOWEST (the longest single call).\n")
	if level >= levelDeep {
		fmt.Printf("  REVDEP_PERF=2 instruments hot loops and inflates absolute numbers — read it as a ratio.\n")
	}
}

func formatDuration(duration time.Duration) string {
	switch {
	case duration >= time.Second:
		return fmt.Sprintf("%.2fs", duration.Seconds())
	case duration >= time.Millisecond:
		return fmt.Sprintf("%.1fms", float64(duration)/float64(time.Millisecond))
	case duration >= time.Microsecond:
		return fmt.Sprintf("%.0fµs", float64(duration)/float64(time.Microsecond))
	default:
		return fmt.Sprintf("%dns", duration.Nanoseconds())
	}
}
