package gg

import (
	"path"
)

func isThunk(hash string) bool {
	return hash[0] == 'T'
}

// ========== Paths ==========

func ggOrigBlobs(dir string, file string) string {
	return ggOrig(dir, GG_BLOBS, file)
}

func ggOrigReductions(dir string, file string) string {
	return ggOrig(dir, GG_REDUCTIONS, file)
}

func ggOrigHashCache(dir string, file string) string {
	return ggOrig(dir, GG_REDUCTIONS, file)
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
	return ggLocal(dir, GG_REDUCTIONS, file)
}

func ggLocal(dir string, subDir string, file string) string {
	return ggDir(GG_LOCAL_DIR, dir, subDir, file)
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
	return ggDir(GG_REMOTE_DIR, "", subDir, file)
}

func ggDir(ggDir string, dir string, subDir string, file string) string {
	return path.Join(
		ggDir,
		dir,
		subDir,
		file,
	)
}
