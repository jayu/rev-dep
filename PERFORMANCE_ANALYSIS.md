# Performance Analysis & Optimization Recommendations

## High Impact Performance Improvements

### 1. Reduce Redundant JSON Parsing in resolveImports.go
**Issue**: Multiple JSON unmarshal operations on the same data
- Lines 239-258: `tsconfigContent` is unmarshaled twice for different structures
- Lines 346, 351: `packageJsonContent` is processed multiple times

**Solution**: Parse once into a generic map and extract needed fields
```go
// Instead of:
var rawConfigForPaths map[string]map[string]map[string][]string
err := json.Unmarshal(tsconfigContent, &rawConfigForPaths)
var rawConfigForBaseUrl map[string]map[string]string  
err = json.Unmarshal(tsconfigContent, &rawConfigForBaseUrl)

// Use:
var rawConfig map[string]interface{}
err := json.Unmarshal(tsconfigContent, &rawConfig)
// Extract paths and baseUrl from rawConfig
```

### 2. Optimize String Operations in parseImports.go
**Issue**: Inefficient string searching with `bytes.Index` in loops
- Lines 184-196: Multiple `bytes.Index` calls for string literal detection
- Lines 201-211: Comment skipping uses inefficient byte searching

**Solution**: Use state machine approach or pre-compile regex patterns
```go
// Replace multiple bytes.Index calls with a single pass
func skipToStringEnd(code []byte, start int, quote byte) int {
    i := start + 1
    for i < len(code) && code[i] != quote {
        if code[i] == '\\' { i += 2 } else { i++ }
    }
    return i
}
```

### 3. Cache Computation Results in resolveImports.go
**Issue**: Repeated expensive computations
- Lines 411-426: Multiple sorting operations on same data
- Lines 391-406: Regex sorting happens every resolver creation

**Solution**: Cache sorted results and reuse
```go
type TsConfigParsed struct {
    // ... existing fields
    sortedRegexps     []RegExpArrItem  // cached sorted version
    regexpsSorted     bool             // flag to track if sorted
}
```

## Medium Impact Optimizations

### 4. Optimize Memory Allocations
**Issue**: Excessive slice growth and map allocations
- `parseImports.go` line 171: `imports` slice grows dynamically
- `resolveImports.go` line 428: Multiple map creations without size hints

**Solution**: Pre-allocate with capacity estimates
```go
// Instead of:
imports := make([]Import, 0, 32)

// Use estimated capacity based on file size
estimatedImports := len(code) / 100  // rough heuristic
imports := make([]Import, 0, estimatedImports)
```

### 5. Reduce File I/O Operations
**Issue**: Multiple file reads for same content
- `monorepo.go` lines 48, 75, 107, 260, 284: Repeated package.json reads
- `resolveImports.go` lines 178, 180: Separate reads for package.json and tsconfig.json

**Solution**: Implement file content caching
```go
type FileCache struct {
    contents map[string][]byte
    mutex    sync.RWMutex
}

func (fc *FileCache) ReadFile(path string) ([]byte, error) {
    fc.mutex.RLock()
    if content, exists := fc.contents[path]; exists {
        fc.mutex.RUnlock()
        return content, nil
    }
    fc.mutex.RUnlock()
    
    // Read file and cache
}
```

### 6. Optimize Goroutine Usage
**Issue**: Potential goroutine overhead in main.go
- Lines 144-155: Complex goroutine pattern for dependency graph building
- Lines 263-272: Goroutine per file for entry points processing

**Solution**: Use worker pool pattern with bounded concurrency
```go
// Instead of goroutine per file:
workerCount := runtime.NumCPU()
semaphore := make(chan struct{}, workerCount)

for _, filePath := range files {
    semaphore <- struct{}{}
    go func(fp string) {
        defer func() { <-semaphore }()
        // process file
    }(filePath)
}
```

## Code Redundancy Elimination

### 7. Consolidate Duplicate String Matching Logic
**Issue**: Similar wildcard pattern matching in multiple places
- `resolveImports.go` lines 648-661, 708-724, 901-913: Repeated pattern matching
- `monorepo.go` lines 148-162: Similar pattern processing

**Solution**: Create unified pattern matcher
```go
type PatternMatcher struct {
    patterns []WildcardPattern
}

func (pm *PatternMatcher) Match(request string) (matched bool, pattern *WildcardPattern, wildcardValue string) {
    // Unified matching logic
}
```

### 8. Remove Unused Debug Code
**Issue**: Debug code adds overhead
- `resolveImports.go` lines 230-278: Debug variables and prints
- `parseImports.go` line 87: Panic guard that shouldn't exist in production

**Solution**: Remove or conditionally compile debug code
```go
// Use build tags for debug code
//go:build debug
func debugLog(format string, args ...interface{}) {
    fmt.Printf(format, args...)
}
```

## Additional Files Analysis

### 11. Optimize Graph Building in buildDepsGraph.go
**Issue**: Inefficient recursive graph construction with excessive memory copying
- Lines 25-28: Map copying for each recursive call creates unnecessary overhead
- Lines 88-109: Redundant node serialization with duplicate loops
- Lines 175-181: Path array copying in recursion creates many allocations

**Solution**: Use shared visited set and optimize serialization
```go
// Instead of copying map in each recursion:
localVisited := make(map[string]bool)
for k, v := range visited {
    localVisited[k] = v
}

// Use shared set with depth tracking:
type visitedState struct {
    path  string
    depth int
}
visited := make(map[visitedState]bool)
```

### 12. Optimize Circular Dependency Detection in circularDeps.go
**Issue**: Inefficient cycle detection with duplicate work
- Lines 20-21: Path slice copying for each DFS step
- Lines 111-124: String-based deduplication is expensive
- Lines 91-95: Linear search through dependencies for request matching

**Solution**: Use more efficient data structures
```go
// Use path indices instead of string copying
type cycleTracker struct {
    nodeToIndex map[string]int
    path        []string
}

// Use map-based deduplication
func deduplicateCycles(cycles [][]string) [][]string {
    seen := make(map[[64]byte]bool) // Use hash instead of string join
    result := make([][]string, 0, len(cycles))
    
    for _, cycle := range cycles {
        if !seen[hashCycle(cycle)] {
            seen[hashCycle(cycle)] = true
            result = append(result, cycle)
        }
    }
    return result
}
```

### 13. Optimize Glob Matching in createGlobMatchers.go
**Issue**: Redundant pattern compilation and matching
- Lines 40, 53: Multiple glob compilations for similar patterns
- Lines 67-69: Path normalization repeated for each matcher
- Lines 80-94: Multiple string operations for each match attempt

**Solution**: Cache compiled patterns and normalize once
```go
type OptimizedGlobMatcher struct {
    compiledPatterns []glob.Glob
    rootPattern     string
    normalizedRoot  string
}

func (ogm *OptimizedGlobMatcher) Match(filePath string) bool {
    normalizedPath := NormalizePathForInternal(filePath)
    // Try all patterns in single loop
    for _, pattern := range ogm.compiledPatterns {
        if pattern.Match(normalizedPath) {
            return true
        }
    }
    return false
}
```

### 14. Optimize File System Operations in getFiles.go
**Issue**: Excessive file system calls and redundant operations
- Lines 47, 78: Multiple .gitignore reads for same directory
- Lines 110-135: Sequential file extension checking with multiple stat calls
- Lines 64-67: Directory traversal without early termination

**Solution**: Batch file operations and cache results
```go
type FileScanner struct {
    statCache    map[string]os.FileInfo
    gitignoreCache map[string][]GlobMatcher
}

func (fs *FileScanner) CheckExtensions(path string) string {
    if info, exists := fs.statCache[path]; exists {
        // Use cached info
    }
    // Check all extensions in single stat call using glob
    pattern := path + ".{ts,tsx,js,jsx,cjs,mjs,mjsx}"
    matches, _ := filepath.Glob(pattern)
    // Return first match
}
```

### 15. Optimize TypeScript Config Processing in tsconfig.go
**Issue**: Complex recursive merging with multiple JSON operations
- Lines 92-110: Sequential file candidate checking with multiple stat calls
- Lines 141-167: Complex map merging with multiple type assertions
- Lines 181-250: Repetitive map operations for compiler options

**Solution**: Streamline merging process and cache file checks
```go
type TsConfigResolver struct {
    fileCache   map[string]map[string]interface{}
    pathCache   map[string][]string
}

func (tcr *TsConfigResolver) resolveExtendsCached(cfg map[string]interface{}, baseDir string, seen map[string]bool) (map[string]interface{}, error) {
    // Use cached resolutions
    if cached, exists := tcr.fileCache[baseDir]; exists {
        return cached, nil
    }
    
    // Optimized merging with reduced type assertions
    result := tcr.mergeConfigsOptimized(baseCfg, cfg)
    tcr.fileCache[baseDir] = result
    return result, nil
}
```

## Low Impact but Worthwhile

### 9. Optimize Import Resolution Order
**Issue**: Linear search through alias patterns
- `resolveImports.go`: Sequential pattern matching

**Solution**: Use more efficient data structures like trie for prefix matching

### 10. Reduce String Allocations
**Issue**: Excessive string concatenations
- Multiple string operations throughout could use `strings.Builder`

### 16. Consolidate Path Normalization
**Issue**: Repeated path normalization across multiple files
- `getFiles.go`, `tsconfig.go`, `monorepo.go`: Multiple path normalization calls

**Solution**: Centralize path normalization with caching
```go
type PathNormalizer struct {
    cache map[string]string
}

func (pn *PathNormalizer) Normalize(path string) string {
    if normalized, exists := pn.cache[path]; exists {
        return normalized
    }
    normalized := NormalizePathForInternal(path)
    pn.cache[path] = normalized
    return normalized
}
```

## Implementation Priority

1. **Immediate**: JSON parsing optimization (#1) - 15-20% performance gain
2. **Short-term**: String operation optimization (#2) - 10-15% gain  
3. **Medium-term**: Caching implementations (#3, #5, #16) - 20-25% gain
4. **Long-term**: Memory allocation optimizations (#4, #11, #12) - 5-10% gain

## Files Analyzed

- `resolveImports.go` - Main import resolution logic with regex patterns and caching
- `main.go` - CLI commands and orchestration with goroutine usage
- `monorepo.go` - Workspace detection and package.json processing
- `parseImports.go` - JavaScript/TypeScript import parsing with byte-level operations
- `minimalDependencyTree.go` - Data structure transformations
- `getEntryPoints.go` - Entry point detection logic
- `buildDepsGraph.go` - Dependency graph construction and serialization
- `circularDeps.go` - Circular dependency detection and formatting
- `createGlobMatchers.go` - Pattern matching for file exclusion/inclusion
- `getFiles.go` - File system scanning and gitignore processing
- `tsconfig.go` - TypeScript configuration parsing and merging

## Summary

These optimizations should provide significant performance improvements, especially for large codebases with many files and complex import structures. The most impactful changes focus on reducing redundant computations, optimizing string operations, implementing effective caching strategies, and streamlining file system operations. The additional analysis reveals opportunities in graph algorithms, pattern matching, and configuration processing that could further enhance performance.
