package pathutil

import (
	"os"
	"path/filepath"
)

var OSSeparator = string(os.PathSeparator)

func StandardiseDirPath(p string) string {
	if string(p[len(p)-1]) == OSSeparator {
		return p
	}
	return p + OSSeparator
}

func StandardiseDirPathInternal(p string) string {
	if len(p) == 0 {
		return "/"
	}
	if string(p[len(p)-1]) == "/" {
		return p
	}
	return p + "/"
}

func ResolveAbsoluteCwd(cwd string) string {
	if filepath.IsAbs(cwd) {
		return StandardiseDirPath(cwd)
	}
	binaryExecDir, _ := os.Getwd()
	return StandardiseDirPath(filepath.Join(binaryExecDir, cwd))
}

func JoinWithCwd(cwd string, filePath string) string {
	if filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(cwd, filePath)
}
