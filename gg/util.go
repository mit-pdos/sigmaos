package gg

import (
	"path"
)

// PID constants
const (
	UPLOADER_SUFFIX       = ".uploader"
	DOWNLOADER_SUFFIX     = ".downloader"
	EXECUTOR_SUFFIX       = ".executor"
	TARGET_WRITER_SUFFIX  = ".target-writer"
	OUTPUT_HANDLER_SUFFIX = ".output-handler"
)

// Path constants
const (
	GG_DIR        = ".gg"
	GG_BLOBS      = "blobs"
	GG_REDUCTIONS = "reductions"
	GG_HASH_CACHE = "hash_cache"
	GG_LOCAL      = "/tmp/ulambda"
	GG_REMOTE     = "name/fs"
)

func isThunk(hash string) bool {
	return hash[0] == 'T'
}

// ========== Pids ==========

func executorPid(hash string) string {
	return ggPid("", "", hash, EXECUTOR_SUFFIX)
}

func outputHandlerPid(hash string) string {
	return ggPid("", "", hash, OUTPUT_HANDLER_SUFFIX)
}

func reductionWriterPid(dir string, subDir string, hash string) string {
	return ggPid(path.Base(dir), path.Base(subDir), hash, TARGET_WRITER_SUFFIX)
}

func uploaderPid(dir string, subDir string, hash string) string {
	return ggPid(path.Base(dir), subDir, hash, UPLOADER_SUFFIX)
}

func downloaderPid(dir string, subDir string, hash string) string {
	return ggPid(path.Base(dir), subDir, hash, DOWNLOADER_SUFFIX)
}

func ggPid(dir string, subDir string, hash string, suffix string) string {
	return "[" + dir + "." + subDir + "]" + hash + suffix
}

// ========== Paths ==========

func ggOrigBlobs(dir string, file string) string {
	return ggOrig(dir, GG_BLOBS, file)
}

func ggOrigReductions(dir string, file string) string {
	return ggOrig(dir, GG_REDUCTIONS, file)
}

func ggOrigHashCache(dir string, file string) string {
	return ggOrig(dir, GG_HASH_CACHE, file)
}

func ggOrig(dir string, subDir string, file string) string {
	return ggDir(dir, "", subDir, file)
}

func ggLocalBlobs(dir string, file string) string {
	return ggLocal(dir, GG_BLOBS, file)
}

func ggLocalReductions(dir string, file string) string {
	return ggLocal(dir, GG_REDUCTIONS, file)
}

func ggLocalHashCache(dir string, file string) string {
	return ggLocal(dir, GG_HASH_CACHE, file)
}

func ggLocal(dir string, subDir string, file string) string {
	return ggDir(GG_LOCAL, dir, subDir, file)
}

func ggRemoteBlobs(file string) string {
	return ggRemote(GG_BLOBS, file)
}

func ggRemoteReductions(file string) string {
	return ggRemote(GG_REDUCTIONS, file)
}

func ggRemoteHashCache(file string) string {
	return ggRemote(GG_HASH_CACHE, file)
}

func ggRemote(subDir string, file string) string {
	return ggDir(GG_REMOTE, "", subDir, file)
}

func ggDir(env string, dir string, subDir string, file string) string {
	return path.Join(
		env,
		dir,
		GG_DIR,
		subDir,
		file,
	)
}
