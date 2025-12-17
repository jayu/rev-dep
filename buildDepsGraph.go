package main

import (
	"fmt"
	"os"
)

// buildDepsGraph builds a dependency graph from the minimal dependency tree
func buildDepsGraph(deps MinimalDependencyTree, entryPoint string, filePathOrNodeModuleName *string, allPaths bool) BuildDepsGraphResult {
	vertices := make(map[string]*Node)
	var fileOrNodeModuleNode *Node

	var inner func(path string, visited map[string]bool, depth int, parent *Node) *Node
	inner = func(path string, visited map[string]bool, depth int, parent *Node) *Node {
		// Check if vertex already exists
		if vertex, exists := vertices[path]; exists {
			// Add parent to existing vertex
			if parent != nil {
				vertex.Parents = append(vertex.Parents, parent)
			}
			return vertex
		}

		// Create local visited set to track circular dependencies
		localVisited := make(map[string]bool)
		for k, v := range visited {
			localVisited[k] = v
		}

		// Check for circular dependency
		if localVisited[path] {
			circularNode := &Node{
				Path:     "CIRCULAR",
				Children: []*Node{},
			}
			if parent != nil {
				circularNode.Parents = []*Node{parent}
			}
			return circularNode
		}

		localVisited[path] = true

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
		node := &Node{
			Path:     path,
			Children: []*Node{},
		}

		if parent != nil {
			node.Parents = []*Node{parent}
		}

		for _, d := range dep {
			// Do not follow other modules than user modules
			if d.ID != nil && *d.ID != "" && d.ResolvedType == UserModule {
				childNode := inner(*d.ID, localVisited, depth+1, node)
				node.Children = append(node.Children, childNode)
			}
		}

		// Store vertex
		vertices[path] = node

		// Check if this is the file we're looking for
		if filePathOrNodeModuleName != nil && path == *filePathOrNodeModuleName {
			fileOrNodeModuleNode = node
		}

		return node
	}

	root := inner(entryPoint, make(map[string]bool), 1, nil)

	// Convert to serializable format
	serializableVertices := make(map[string]*SerializableNode)
	for path, node := range vertices {
		serializableNode := &SerializableNode{
			Path:     node.Path,
			Parents:  make([]string, 0, len(node.Parents)),
			Children: make([]string, 0, len(node.Children)),
		}

		for _, parent := range node.Parents {
			if parent != nil {
				serializableNode.Parents = append(serializableNode.Parents, parent.Path)
			}
		}

		for _, child := range node.Children {
			if child != nil {
				serializableNode.Children = append(serializableNode.Children, child.Path)
			}
		}

		serializableVertices[path] = serializableNode
	}

	// Convert root to serializable format
	var serializableRoot *SerializableNode
	if root != nil {
		serializableRoot = &SerializableNode{
			Path:     root.Path,
			Parents:  make([]string, 0, len(root.Parents)),
			Children: make([]string, 0, len(root.Children)),
		}

		for _, parent := range root.Parents {
			if parent != nil {
				serializableRoot.Parents = append(serializableRoot.Parents, parent.Path)
			}
		}

		for _, child := range root.Children {
			if child != nil {
				serializableRoot.Children = append(serializableRoot.Children, child.Path)
			}
		}
	}

	// Convert fileOrNodeModuleNode to serializable format
	var serializableFileOrNodeModuleNode *SerializableNode
	if fileOrNodeModuleNode != nil {
		serializableFileOrNodeModuleNode = &SerializableNode{
			Path:     fileOrNodeModuleNode.Path,
			Parents:  make([]string, 0, len(fileOrNodeModuleNode.Parents)),
			Children: make([]string, 0, len(fileOrNodeModuleNode.Children)),
		}

		for _, parent := range fileOrNodeModuleNode.Parents {
			if parent != nil {
				serializableFileOrNodeModuleNode.Parents = append(serializableFileOrNodeModuleNode.Parents, parent.Path)
			}
		}

		for _, child := range fileOrNodeModuleNode.Children {
			if child != nil {
				serializableFileOrNodeModuleNode.Children = append(serializableFileOrNodeModuleNode.Children, child.Path)
			}
		}
	}

	// Compute resolution paths if a specific file was found
	var resolutionPaths [][]string
	if fileOrNodeModuleNode != nil {
		// Initialize with empty path array for the resolvePathsToRoot function
		initialPaths := [][]string{{}}
		resolutionPaths = ResolvePathsToRoot(fileOrNodeModuleNode, allPaths, initialPaths, 0)
	}

	return BuildDepsGraphResult{
		Root:                 serializableRoot,
		FileOrNodeModuleNode: serializableFileOrNodeModuleNode,
		ResolutionPaths:      resolutionPaths,
		Vertices:             serializableVertices,
	}
}

// ResolvePathsToRoot resolves all paths from a node to the root(s)
func ResolvePathsToRoot(node *Node, all bool, resolvedPaths [][]string, depth int) [][]string {

	// Create new paths by prepending current node path to each resolved path
	newPaths := make([][]string, len(resolvedPaths))
	for i, resolvedPath := range resolvedPaths {
		newPath := make([]string, len(resolvedPath)+1)
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
		for _, parent := range node.Parents {
			// fmt.Println("check parent", parent.Path, depth)
			parentPaths := ResolvePathsToRoot(parent, all, newPaths, depth+1)
			allPaths = append(allPaths, parentPaths...)
			if len(allPaths) > 1000 {
				fmt.Println("Resolving all paths hard stop on 1000 paths")
				break
			}
		}
		return allPaths
	}

	// Only follow the first parent
	return ResolvePathsToRoot(node.Parents[0], false, newPaths, depth)
}

// Node represents a node in the dependency graph
type Node struct {
	Path     string  `json:"path"`
	Parents  []*Node `json:"parents,omitempty"`
	Children []*Node `json:"children,omitempty"`
}

// SerializableNode represents a node that can be safely JSON marshaled
type SerializableNode struct {
	Path     string   `json:"path"`
	Parents  []string `json:"parents,omitempty"`
	Children []string `json:"children,omitempty"`
}

// BuildDepsGraphResult represents the result of building the dependency graph
type BuildDepsGraphResult struct {
	Root                 *SerializableNode            `json:"root"`
	FileOrNodeModuleNode *SerializableNode            `json:"fileOrNodeModuleNode"`
	ResolutionPaths      [][]string                   `json:"resolutionPaths,omitempty"`
	Vertices             map[string]*SerializableNode `json:"vertices"`
}
