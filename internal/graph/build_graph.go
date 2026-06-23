package graph

import (
	"fmt"
	"os"
	"slices"

	"rev-dep-go/internal/module"
)

// moduleTargetSpec describes a node-module target to look for while building the graph.
type moduleTargetSpec struct {
	key           string // the original target string (used as the result key)
	name          string
	request       string
	isPackageName bool
}

// depsGraphBuilder is the shared core that builds the multi-entry forward dependency graph. Both
// BuildDepsGraphForMultiple (which adds resolution paths) and BuildEntryPointReachability (which adds
// per-target reachability) drive it; the only difference is what they compute from the built graph.
//
// While building, it records, for each target, a "frontier" — the set of node paths from which the
// target is immediately present: the target file node (file target) or every node importing the
// target module (module target). firstTargetNode is the first such node in DFS order, preserved for
// the single-target resolution-path callers.
type depsGraphBuilder struct {
	deps              MinimalDependencyTree
	ignoreTypeImports bool

	fileTargets   map[string]bool
	moduleTargets []moduleTargetSpec

	vertices      map[string]*SerializableNode
	roots         map[string]*SerializableNode
	sharedVisited map[string]bool

	frontiers       map[string]map[string]bool
	firstTargetNode *SerializableNode
}

func newDepsGraphBuilder(deps MinimalDependencyTree, targets []string, ignoreTypeImports bool) *depsGraphBuilder {
	b := &depsGraphBuilder{
		deps:              deps,
		ignoreTypeImports: ignoreTypeImports,
		fileTargets:       map[string]bool{},
		vertices:          make(map[string]*SerializableNode),
		roots:             make(map[string]*SerializableNode),
		sharedVisited:     make(map[string]bool),
		frontiers:         map[string]map[string]bool{},
	}
	for _, t := range targets {
		if _, ok := deps[t]; ok {
			b.fileTargets[t] = true
			continue
		}
		name := module.GetNodeModuleName(t)
		b.moduleTargets = append(b.moduleTargets, moduleTargetSpec{
			key:           t,
			name:          name,
			request:       t,
			isPackageName: name == t,
		})
	}
	return b
}

func (b *depsGraphBuilder) addFrontier(targetKey, nodePath string) {
	set := b.frontiers[targetKey]
	if set == nil {
		set = map[string]bool{}
		b.frontiers[targetKey] = set
	}
	set[nodePath] = true
}

func (b *depsGraphBuilder) inner(path string, depth int, parent *SerializableNode) *SerializableNode {
	// Check if vertex already exists
	if vertex, exists := b.vertices[path]; exists {
		if parent != nil {
			vertex.Parents = append(vertex.Parents, parent.Path)
		}
		return vertex
	}

	// Check for circular dependency - use shared visited set without copying
	if b.sharedVisited[path] {
		if vertex, exists := b.vertices[path]; exists {
			if parent != nil {
				vertex.Parents = append(vertex.Parents, parent.Path)
			}
			return vertex
		}

		// Create the circular node to maintain the cycle
		circularNode := &SerializableNode{
			Path:     path,
			Children: []string{},
			Parents:  []string{},
		}
		if parent != nil {
			circularNode.Parents = []string{parent.Path}
		}
		b.vertices[path] = circularNode
		return circularNode
	}

	b.sharedVisited[path] = true

	dep, exists := b.deps[path]
	if !exists {
		parentPath := "unknown"
		if parent != nil {
			parentPath = parent.Path
		}
		fmt.Fprintf(os.Stderr, "Dependency '%s' not found! Imported from '%s'\n", path, parentPath)
		os.Exit(1)
	}

	node := &SerializableNode{
		Path:     path,
		Children: []string{},
	}
	if parent != nil {
		node.Parents = []string{parent.Path}
	}

	nodeModulesSet := map[string]bool{}
	for _, d := range dep {
		if b.ignoreTypeImports && d.ImportKind == OnlyTypeImport {
			continue
		}

		if d.ResolvedType == NodeModule || d.ResolvedType == NotResolvedModule {
			if d.Request != "" {
				nodeModulesSet[d.Request] = true
			}
			for _, mt := range b.moduleTargets {
				matched := false
				if mt.isPackageName {
					matched = module.GetNodeModuleName(d.Request) == mt.name
				} else {
					matched = d.Request == mt.request
				}
				if matched {
					if node.LookedUpNodeModuleImportRequest == "" {
						node.LookedUpNodeModuleImportRequest = d.Request
					}
					if b.firstTargetNode == nil {
						b.firstTargetNode = node
					}
					b.addFrontier(mt.key, node.Path)
				}
			}
		}

		// Do not follow other modules than user modules and monorepo modules
		if d.ID != "" && (d.ResolvedType == UserModule || d.ResolvedType == MonorepoModule) {
			childNode := b.inner(d.ID, depth+1, node)
			node.Children = append(node.Children, childNode.Path)
		}
	}
	if len(nodeModulesSet) > 0 {
		node.Modules = make([]string, 0, len(nodeModulesSet))
		for moduleName := range nodeModulesSet {
			node.Modules = append(node.Modules, moduleName)
		}
		slices.Sort(node.Modules)
	}

	// Remove from visited set when backtracking to allow revisiting in other branches
	delete(b.sharedVisited, path)

	b.vertices[path] = node

	// File target match.
	if b.fileTargets[path] {
		if b.firstTargetNode == nil {
			b.firstTargetNode = node
		}
		b.addFrontier(path, path)
	}

	return node
}

// build runs the DFS from each entry point. perEntry, if non-nil, is invoked after each entry
// point's subtree is built (used to compute resolution paths against the shared graph state).
func (b *depsGraphBuilder) build(entryPoints []string, perEntry func(entryPoint string, root *SerializableNode)) {
	for _, entryPoint := range entryPoints {
		root := b.inner(entryPoint, 1, nil)
		b.roots[entryPoint] = root
		if perEntry != nil {
			perEntry(entryPoint, root)
		}
	}
}

// BuildDepsGraphForMultiple builds the forward dependency graph from the given entry points. When a
// target file/module is provided it also resolves, per entry point, the paths from that entry point
// to the target (ResolutionPaths) and exposes the matched target node (FileOrNodeModuleNode).
//
// NOTE: resolution paths follow only the first parent and are only correct for a single entry point
// at a time (see the resolve command in internal/cli/root.go). For "which of many entry points reach
// a target" use BuildEntryPointReachability instead.
func BuildDepsGraphForMultiple(deps MinimalDependencyTree, entryPoints []string, filePathOrNodeModuleName *string, allPaths bool, ignoreTypeImports bool) BuildDepsGraphResultMultiple {
	var targets []string
	if filePathOrNodeModuleName != nil {
		targets = []string{*filePathOrNodeModuleName}
	}

	b := newDepsGraphBuilder(deps, targets, ignoreTypeImports)
	resolutionPaths := make(map[string][][]string)
	b.build(entryPoints, func(entryPoint string, root *SerializableNode) {
		if b.firstTargetNode != nil {
			initialPaths := [][]string{{}}
			resolutionPaths[entryPoint] = ResolvePathsToRoot(b.firstTargetNode, b.vertices, allPaths, initialPaths, 0)
		}
	})

	return BuildDepsGraphResultMultiple{
		Roots:                b.roots,
		FileOrNodeModuleNode: b.firstTargetNode,
		ResolutionPaths:      resolutionPaths,
		Vertices:             b.vertices,
	}
}

// EntryPointReachability reports, for each target, which entry-point roots transitively reach it.
type EntryPointReachability struct {
	// RootReachesTarget[target][entryPoint] is true when entryPoint transitively reaches target.
	RootReachesTarget map[string]map[string]bool
}

// BuildEntryPointReachability builds the forward graph from entryPoints ONCE and answers, for every
// target, which entry points transitively reach it. The forward graph is target-independent, so the
// only per-target work is a reverse BFS from that target's frontier back to the roots — cycle-safe
// and visiting only the target's ancestors. This is the optimal shape for "which of many entry
// points reach these targets", and unlike resolution paths it stays correct with many entry points.
func BuildEntryPointReachability(deps MinimalDependencyTree, entryPoints []string, targets []string, ignoreTypeImports bool) EntryPointReachability {
	b := newDepsGraphBuilder(deps, targets, ignoreTypeImports)
	b.build(entryPoints, nil)

	rootSet := make(map[string]bool, len(entryPoints))
	for _, ep := range entryPoints {
		rootSet[ep] = true
	}

	rootReaches := make(map[string]map[string]bool, len(targets))
	for _, t := range targets {
		reaches := map[string]bool{}
		rootReaches[t] = reaches
		frontier := b.frontiers[t]
		if len(frontier) == 0 {
			continue
		}
		visited := make(map[string]bool, len(frontier))
		queue := make([]string, 0, len(frontier))
		for p := range frontier {
			visited[p] = true
			queue = append(queue, p)
		}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			if rootSet[current] {
				reaches[current] = true
			}
			node, ok := b.vertices[current]
			if !ok {
				continue
			}
			for _, parent := range node.Parents {
				if !visited[parent] {
					visited[parent] = true
					queue = append(queue, parent)
				}
			}
		}
	}

	return EntryPointReachability{RootReachesTarget: rootReaches}
}

// ResolvePathsToRoot resolves all paths from a node to the root(s)
func ResolvePathsToRoot(node *SerializableNode, vertices map[string]*SerializableNode, all bool, resolvedPaths [][]string, depth int) [][]string {

	// Create new paths by prepending current node path to each resolved path
	// Optimize by preallocating and copying in place
	newPaths := make([][]string, len(resolvedPaths))
	for i, resolvedPath := range resolvedPaths {
		// Preallocate with exact size to avoid multiple allocations
		newPath := make([]string, 1+len(resolvedPath))
		newPath[0] = node.Path
		copy(newPath[1:], resolvedPath)
		newPaths[i] = newPath
	}

	// If no parents, we've reached the root
	if len(node.Parents) == 0 {
		// If there is only one path, and has length of 1, it's file self reference
		// It's invalid result, so we return empty paths in that case
		if len(newPaths) == 1 && len(newPaths[0]) == 1 {
			return [][]string{}
		}
		return newPaths
	}

	if all {
		// Collect paths from all parents
		var allPaths [][]string
		for _, parentPath := range node.Parents {
			parent, exists := vertices[parentPath]
			if !exists {
				continue
			}
			// fmt.Println("check parent", parent.Path, depth)
			parentPaths := ResolvePathsToRoot(parent, vertices, all, newPaths, depth+1)
			allPaths = append(allPaths, parentPaths...)
			if len(allPaths) > 1000 {
				fmt.Println("Resolving all paths hard stop on 1000 paths")
				break
			}
		}
		return allPaths
	}

	// Only follow the first parent
	if len(node.Parents) > 0 {
		if parent, exists := vertices[node.Parents[0]]; exists {
			return ResolvePathsToRoot(parent, vertices, false, newPaths, depth)
		}
	}
	return newPaths
}

// SerializableNode represents a node that can be safely JSON marshaled
type SerializableNode struct {
	Path                            string   `json:"path"`
	Parents                         []string `json:"parents,omitempty"`
	Children                        []string `json:"children,omitempty"`
	Modules                         []string `json:"modules,omitempty"`
	LookedUpNodeModuleImportRequest string   `json:"lookedUpNodeModuleImportRequest,omitempty"`
}

// bst collects a list of all vertices starting from the root SerializableNode
func BST(root *SerializableNode, vertices map[string]*SerializableNode) []string {
	if root == nil {
		return []string{}
	}

	visited := make(map[string]bool)
	queue := []*SerializableNode{root}
	var result []string

	for len(queue) > 0 {
		// Dequeue the first node
		current := queue[0]
		queue = queue[1:]

		// Skip if already visited (prevents infinite loops in circular dependencies)
		if visited[current.Path] {
			continue
		}

		// Mark as visited and add to result
		visited[current.Path] = true
		result = append(result, current.Path)

		// Add all children to the queue
		for _, childPath := range current.Children {
			if child, exists := vertices[childPath]; exists && !visited[childPath] {
				queue = append(queue, child)
			}
		}
	}

	return result
}

// BuildDepsGraphResultMultiple represents the result of building dependency graphs for multiple entry points
type BuildDepsGraphResultMultiple struct {
	Roots                map[string]*SerializableNode `json:"roots"`
	FileOrNodeModuleNode *SerializableNode            `json:"fileOrNodeModuleNode"`
	ResolutionPaths      map[string][][]string        `json:"resolutionPaths,omitempty"`
	Vertices             map[string]*SerializableNode `json:"vertices"`
}
