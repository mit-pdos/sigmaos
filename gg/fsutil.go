package gg

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	db "ulambda/debug"
	np "ulambda/ninep"
)

// Interfaces
type FsLambda interface {
	MakeFile(string, []byte) error
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte) error
	Stat(string) (*np.Stat, error)
	Name() string
}

// Path constants
const (
	GG_DIR        = ".gg"
	GG_BLOBS      = "blobs"
	GG_REDUCTIONS = "reductions"
	GG_HASH_CACHE = "hash_cache"
	GG_LOCAL      = "/tmp/ulambda"
	GG_REMOTE     = "name/fs"
	GG_ORIG       = "orig"
)

// ========== Paths ==========

func isRemote(dir string) bool {
	return strings.Contains(dir, GG_REMOTE)
}

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

// ========== Util fns ==========

func setupLocalExecutionEnv(hash string) {
	subDirs := []string{
		ggLocalBlobs(hash, ""),
		ggLocalReductions(hash, ""),
		ggLocalHashCache(hash, ""),
	}
	for _, d := range subDirs {
		err := os.MkdirAll(d, 0777)
		if err != nil {
			log.Fatalf("Error making execution env dir [%v]: %v\n", d, err)
		}
	}
}

func downloadFile(fslambda FsLambda, src string, dest string) {
	db.DPrintf("Downloading [%v] to [%v]\n", src, dest)
	contents, err := fslambda.ReadFile(src)
	if err != nil {
		log.Printf("%v Read download file error [%v]: %v\n", fslambda.Name(), src, err)
	}
	err = ioutil.WriteFile(dest, contents, 0777)
	if err != nil {
		log.Printf("%v Couldn't write download file [%v]: %v\n", fslambda.Name(), dest, err)
	}
	// Override umask
	err = os.Chmod(dest, 0777)
	if err != nil {
		log.Printf("%v Couldn't chmod newly downloaded file [%v]: %v\n", fslambda.Name(), dest, err)
	}
}

func uploadDir(fslambda FsLambda, dir string, subDir string) {
	src := ggLocal(dir, subDir, "")
	dest := ggRemote(subDir, "")
	db.DPrintf("%v uploading dir [%v] to [%v]\n", fslambda.Name(), src, dest)
	files, err := ioutil.ReadDir(src)
	if err != nil {
		log.Fatalf("%v read upload dir error: %v\n", fslambda.Name(), err)
	}
	for _, f := range files {
		// Don't overwrite other thunks' reductions
		if subDir != "reductions" || strings.Contains(f.Name(), dir) {
			srcPath := path.Join(src, f.Name())
			dstPath := path.Join(dest, f.Name())
			contents, err := ioutil.ReadFile(srcPath)
			if err != nil {
				log.Fatalf("%v read upload dir file error[%v]: %v\n", fslambda.Name(), srcPath, err)
			}
			// Try and make a new file if one doesn't exist, else overwrite
			_, err = fslambda.Stat(dstPath)
			if err != nil {
				db.DPrintf("%v mkfile dir uploader [%v]\n", fslambda.Name(), dstPath)
				// XXX Perms?
				err = fslambda.MakeFile(dstPath, contents)
				if err != nil {
					// XXX This only occurs if someone else has written the file since we
					// last checked if it existed. Since it isn't a reduction (by the
					// check in the big if statement), this is ok. The contents will be
					// identical. Should change this to an atomic rename operation at some
					// point, though.
					log.Printf("%v couldn't make upload dir file %v: %v", fslambda.Name(), dstPath, err)
				}
			} else {
				db.DPrintf("%v file already exists [%v]\n", fslambda.Name(), dstPath)
				err = fslambda.WriteFile(dstPath, contents)
				if err != nil {
					// XXX This only occurs if someone else has written the file since we
					// last checked if it existed. Since it isn't a reduction (by the
					// check in the big if statement), this is ok. The contents will be
					// identical. Should change this to an atomic rename operation at some
					// point, though.
					log.Printf("%v couldn't write uplaod dir file [%v]: %v\n", fslambda.Name(), dstPath, err)
				}
			}
		}
	}
}

func getReductionResult(fslambda FsLambda, hash string) string {
	resultPath := ggRemoteReductions(hash)
	result, err := fslambda.ReadFile(resultPath)
	if err != nil {
		log.Fatalf("%v Error reading reduction[%v]: %v\n", fslambda.Name(), resultPath, err)
	}
	return strings.TrimSpace(string(result))
}

// Check if the output reduction exists in the global dir
func reductionExists(fslambda FsLambda, hash string) bool {
	outputPath := ggRemoteReductions(hash)
	// XXX would be nice to have a "stat" equivalent -- wstat?
	_, err := fslambda.Stat(outputPath)
	if err == nil {
		return true
	}
	return false
}
