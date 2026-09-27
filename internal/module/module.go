package module

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/jsonc"
)

func GetNodeModuleName(request string) string {
	splitCount := 2
	if strings.HasPrefix(request, "@") {
		splitCount = 3
	}
	parts := strings.SplitN(request, "/", splitCount)
	return strings.Join(parts[:splitCount-1], "/")
}

func GetNodeModulesFromPkgJson(packageJsonContent []byte) (map[string]bool, map[string]bool) {
	packageJsonContent = jsonc.ToJSON(packageJsonContent)

	var rawPackageJson map[string]map[string]string

	err := json.Unmarshal(packageJsonContent, &rawPackageJson)

	if err != nil {
		// fmt.Printf("Failed to parse package json : %s\n", err)
	}

	deps := map[string]bool{}
	devDeps := map[string]bool{}

	rawDeps, ok := rawPackageJson["dependencies"]

	if ok {
		for dep := range rawDeps {
			deps[dep] = true
		}
	}
	rawDevDeps, ok2 := rawPackageJson["devDependencies"]

	if ok2 {
		for dep := range rawDevDeps {
			devDeps[dep] = true
		}
	}

	return deps, devDeps
}

func IsValidNodeModuleName(name string) bool {
	// There are more restrictions on node module name than starting with dot, but for now we just check against that
	return !strings.HasPrefix(name, ".")
}
