//go:build dev
// +build dev

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"time"
)

func getEntryPointFiles(minimalTree MinimalDependencyTree, mainEntryPoint string, rootFilePath string) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		startTime := time.Now()

		query := req.URL.Query()
		filePath := query.Get("filePath")

		if filePath == "" {
			filePath = mainEntryPoint
		} else {
			filePath = filepath.Join(rootFilePath, filePath)
		}

		fmt.Println("Request: ", filePath)

		graphMultiple := buildDepsGraphForMultiple(minimalTree, []string{filePath}, nil, false)

		// Extract the single result for compatibility
		var graph BuildDepsGraphResult
		if root, exists := graphMultiple.Roots[filePath]; exists {
			graph = BuildDepsGraphResult{
				Root:                 root,
				FileOrNodeModuleNode: graphMultiple.FileOrNodeModuleNode,
				ResolutionPaths:      graphMultiple.ResolutionPaths[filePath],
				Vertices:             graphMultiple.Vertices,
			}
		}

		files := make([]string, 0, len(graph.Vertices))

		for filePath := range graph.Vertices {
			rel, _ := filepath.Rel(rootFilePath, filePath)
			files = append(files, rel)
		}

		slices.Sort(files)

		jsonResponse, _ := json.Marshal(files)

		w.Write([]byte(jsonResponse))
		fmt.Printf("GetEntryPoints Served in %v\n", time.Since(startTime))

	}
}

func explainDependency(minimalTree MinimalDependencyTree, mainEntryPoint string, rootFilePath string) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		startTime := time.Now()
		query := req.URL.Query()
		filePath := query.Get("filePath")
		entryPoint := query.Get("entryPoint")

		filePath = filepath.Join(rootFilePath, filePath)

		if entryPoint == "" {
			entryPoint = mainEntryPoint
		} else {
			entryPoint = filepath.Join(rootFilePath, entryPoint)
		}

		fmt.Printf("Request: filePath=%s; entryPoint=%s\n", filePath, entryPoint)

		graphMultiple := buildDepsGraphForMultiple(minimalTree, []string{entryPoint}, &filePath, false)

		// Extract the single result for compatibility
		var graph BuildDepsGraphResult
		if root, exists := graphMultiple.Roots[entryPoint]; exists {
			graph = BuildDepsGraphResult{
				Root:                 root,
				FileOrNodeModuleNode: graphMultiple.FileOrNodeModuleNode,
				ResolutionPaths:      graphMultiple.ResolutionPaths[entryPoint],
				Vertices:             graphMultiple.Vertices,
			}
		}

		resolutionPaths := make([]string, 0, len(graph.ResolutionPaths[0]))

		for _, filePath := range graph.ResolutionPaths[0] {
			rel, _ := filepath.Rel(rootFilePath, filePath)
			resolutionPaths = append(resolutionPaths, rel)
		}

		jsonResponse, _ := json.Marshal(resolutionPaths)

		w.Write([]byte(jsonResponse))

		fmt.Printf("Explain Served in %v\n", time.Since(startTime))
	}
}

func StartServer(minimalTree MinimalDependencyTree, mainEntryPoint string, rootFilePath string) {
	fs := http.FileServer(http.Dir("./browser/dist"))
	http.Handle("/", fs)
	http.HandleFunc("/getEntryPointFiles", getEntryPointFiles(minimalTree, mainEntryPoint, rootFilePath))
	http.HandleFunc("/explainDependency", explainDependency(minimalTree, mainEntryPoint, rootFilePath))

	fmt.Println("Server running at http://localhost:8008")
	http.ListenAndServe(":8008", nil)
}
