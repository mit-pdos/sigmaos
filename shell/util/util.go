package util

import (
	"path/filepath"
	"sigmaos/shell/shellctx"
	"strings"
)

func IsLocalFile(path string) bool {
	return strings.HasPrefix(path, shellctx.LOCAL_FILE_PREFIX)
}

func ResolvePath(ctx *shellctx.ShellContext, path string) string {
	var resolvedPath string
	if filepath.IsAbs(path) {
		resolvedPath = filepath.Clean(path)
	} else {
		resolvedPath = filepath.Clean(filepath.Join(ctx.CurrentDir, path))
	}
	//if path is directory append /, to make things easier for autocomplete
	isDir, err := ctx.Tstate.IsDir(shellctx.FILEPATH_OFFSET + resolvedPath)
	if isDir && err == nil && resolvedPath[len(resolvedPath)-1] != '/' {
		resolvedPath += "/"
	}
	return resolvedPath
}
