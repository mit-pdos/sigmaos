package gg

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
)

type Executor struct {
	pid       string
	thunkHash string
	*fslib.FsLib
}

func MakeExecutor(args []string, debug bool) (*Executor, error) {
	db.DPrintf("Executor: %v\n", args)
	ex := &Executor{}
	ex.pid = args[0]
	ex.thunkHash = args[1]
	fls := fslib.MakeFsLib("executor")
	ex.FsLib = fls
	db.SetDebug(false)
	ex.Started(ex.pid)
	return ex, nil
}

func (ex *Executor) Work() {
	setupLocalExecutionEnv(ex, ex.thunkHash)
	ex.downloadInputFiles()
	ex.exec()
	ex.uploadOutputFiles()
}

func (ex *Executor) downloadInputFiles() {
	// Download the thunk itself
	ex.downloadFile(GG_BLOBS, ex.thunkHash)
	inputDeps := getInputDependencies(ex, ex.thunkHash, ggRemoteBlobs(""))
	for _, dep := range inputDeps {
		if isThunk(dep) {
			// If it's a thunk, download the reduction file
			ex.downloadFile(GG_REDUCTIONS, dep)
			// And download the reduction itself
			reduction := ex.readReduction(dep)
			ex.downloadFile(GG_BLOBS, reduction)
		} else {
			ex.downloadFile(GG_BLOBS, dep)
		}
	}
}

func (ex *Executor) exec() error {
	args := []string{
		"--ninep",
		ex.thunkHash,
	}
	env := append(os.Environ(), []string{
		"GG_DIR=" + ggLocal(ex.thunkHash, "", ""),
		"GG_NINEP=true",
		"GG_VERBOSE=1",
	}...)
	cmd := exec.Command("gg-execute", args...)
	cmd.Env = env
	cmd.Dir = ggLocal(ex.thunkHash, "", "")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Fatalf("Executor error when starting command %v: %v\n", ex.thunkHash, err)
		return err
	}
	err = cmd.Wait()
	if err != nil {
		log.Fatalf("Executor error when waiting for command %v: %v\n", ex.thunkHash, err)
		return err
	}
	return nil
}

func (ex *Executor) uploadOutputFiles() {
	subDirs := []string{GG_BLOBS, GG_REDUCTIONS, GG_HASH_CACHE}

	// Upload contents of each subdir (blobs, reductions, hash_cache) to 9P remote
	// server
	for _, subDir := range subDirs {
		ex.uploadDir(subDir)
	}
}

func (ex *Executor) downloadFile(subDir string, file string) {
	src := ggRemote(subDir, file)
	dest := ggLocal(ex.thunkHash, subDir, file)
	db.DPrintf("Executor downloading [%v] to [%v]\n", src, dest)
	contents, err := ex.ReadFile(src)
	if err != nil {
		log.Printf("Read download file error [%v]: %v\n", src, err)
	}
	err = ioutil.WriteFile(dest, contents, 0777)
	if err != nil {
		log.Printf("Executor couldn't write download file [%v]: %v\n", dest, err)
	}
	// Override umask
	err = os.Chmod(dest, 0777)
	if err != nil {
		log.Printf("Executor couldn't chmod newly downloaded file")
	}
}

func (ex *Executor) readReduction(reductionHash string) string {
	reductionPath := ggRemoteReductions(reductionHash)
	f, err := ex.ReadFile(reductionPath)
	if err != nil {
		log.Fatalf("Executor couldn't read target reduction [%v]: %v\n", reductionPath, err)
	}
	return strings.TrimSpace(string(f))
}

func (ex *Executor) uploadDir(subDir string) {
	src := ggLocal(ex.thunkHash, subDir, "")
	dest := ggRemote(subDir, "")
	db.DPrintf("Executor uploading dir [%v] to [%v]\n", src, dest)
	files, err := ioutil.ReadDir(src)
	if err != nil {
		log.Fatalf("Executor read upload dir error: %v\n", err)
	}
	for _, f := range files {
		// Don't overwrite other thunks' reductions
		if subDir != "reductions" || strings.Contains(f.Name(), ex.thunkHash) {
			srcPath := path.Join(src, f.Name())
			dstPath := path.Join(dest, f.Name())
			contents, err := ioutil.ReadFile(srcPath)
			if err != nil {
				log.Fatalf("Executor read upload dir file error[%v]: %v\n", srcPath, err)
			}
			// Try and make a new file if one doesn't exist, else overwrite
			_, err = ex.Stat(dstPath)
			if err != nil {
				db.DPrintf("Executor mkfile dir uploader [%v]\n", dstPath)
				// XXX Perms?
				err = ex.MakeFile(dstPath, contents)
				if err != nil {
					// XXX This only occurs if someone else has written the file since we
					// last checked if it existed. Since it isn't a reduction (by the
					// check in the big if statement), this is ok. The contents will be
					// identical. Should change this to an atomic rename operation at some
					// point, though.
					log.Printf("Executor couldn't make upload dir file %v: %v", dstPath, err)
				}
			} else {
				db.DPrintf("Executor file already exists [%v]\n", dstPath)
				err = ex.WriteFile(dstPath, contents)
				if err != nil {
					// XXX This only occurs if someone else has written the file since we
					// last checked if it existed. Since it isn't a reduction (by the
					// check in the big if statement), this is ok. The contents will be
					// identical. Should change this to an atomic rename operation at some
					// point, though.
					log.Printf("Executor couldn't write uplaod dir file [%v]: %v\n", dstPath, err)
				}
			}
		}
	}

}

func (ex *Executor) Exit() {
	ex.Exiting(ex.pid, "OK")
}

// XXX Unsure if we really need this
func (ex *Executor) getCwd() string {
	return "DIDN'T IMPLEMENT EXECUTOR GETCWD"
}

//echo $SPID,"gg-execute","[--timelog --ninep ${thunkHashes[@]}]","[GG_STORAGE_URI=9p://mnt/9p/fs GG_DIR=/mnt/9p/fs/.gg]","",""
