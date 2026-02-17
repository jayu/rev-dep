package main

import (
	"fmt"
	"os"
)

func buildDepsGraphForMultiple(deps MinimalDependencyTree, entryPoints []string, filePathOrNodeModuleName *string, allPaths bool) BuildDepsGraphResultMultiple {
	vertices := make(map[string]*SerializableNode)
	roots := make(map[string]*SerializableNode)
	resolutionPaths := make(map[string][][]string)
	var fileOrNodeModuleNode *SerializableNode
	sharedVisited := make(map[string]bool)

	var inner func(path string, visited map[string]bool, depth int, parent *SerializableNode) *SerializableNode
	inner = func(path string, visited map[string]bool, depth int, parent *SerializableNode) *SerializableNode {
		// Check if vertex already exists
		if vertex, exists := vertices[path]; exists {
			// Add parent to existing vertex
			if parent != nil {
				vertex.Parents = append(vertex.Parents, parent.Path)
			}
			return vertex
		}

		// Check for circular dependency - use shared visited set without copying
		if visited[path] {
			// For circular dependencies, we still create the node and link it to maintain the actual cycle
			// but we don't recurse further to prevent infinite loops
			if vertex, exists := vertices[path]; exists {
				// Node already exists, just add parent relationship
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
			vertices[path] = circularNode
			return circularNode
		}

		// Add to visited set
		visited[path] = true

		// Get dependencies for this path
		dep, exists := deps[path]
		if !exists {
			// Return error node or panic - following JS implementation that throws
			parentPath := "unknown"
			if parent != nil {
				parentPath = parent.Path
			}
			fmt.Fprintf(os.Stderr, "Dependency '%s' not found! Imported from '%s'\n", path, parentPath)
			os.Exit(1)
		}

		// Create new node
		node := &SerializableNode{
			Path:     path,
			Children: []string{},
		}

		if parent != nil {
			node.Parents = []string{parent.Path}
		}

		for _, d := range dep {
			// Do not follow other modules than user modules and monorepo modules
			if d.ID != "" && (d.ResolvedType == UserModule || d.ResolvedType == MonorepoModule) {
				childNode := inner(d.ID, visited, depth+1, node)
				node.Children = append(node.Children, childNode.Path)
			}
		}

		// Remove from visited set when backtracking to allow revisiting in other branches
		delete(visited, path)

		// Store vertex
		vertices[path] = node

		// Check if this is the file we're looking for
		if filePathOrNodeModuleName != nil && path == *filePathOrNodeModuleName {
			fileOrNodeModuleNode = node
		}

		return node
	}

	// Build graph for each entry point using shared visited set
	for _, entryPoint := range entryPoints {
		root := inner(entryPoint, sharedVisited, 1, nil)
		roots[entryPoint] = root

		// Compute resolution paths if a specific file was found for this entry point
		if fileOrNodeModuleNode != nil {
			// Initialize with empty path array for the resolvePathsToRoot function
			initialPaths := [][]string{{}}
			resolutionPaths[entryPoint] = ResolvePathsToRoot(fileOrNodeModuleNode, vertices, allPaths, initialPaths, 0)
		}
	}

	return BuildDepsGraphResultMultiple{
		Roots:                roots,
		FileOrNodeModuleNode: fileOrNodeModuleNode,
		ResolutionPaths:      resolutionPaths,
		Vertices:             vertices,
	}
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
	Path     string   `json:"path"`
	Parents  []string `json:"parents,omitempty"`
	Children []string `json:"children,omitempty"`
}

// bst collects a list of all vertices starting from the root SerializableNode
func bst(root *SerializableNode, vertices map[string]*SerializableNode) []string {
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
