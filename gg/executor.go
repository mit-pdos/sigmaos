package gg

import (
	"log"
	"os"
	"os/exec"

	db "ulambda/debug"
	"ulambda/depproc"
	"ulambda/fslib"
)

type Executor struct {
	pid       string
	thunkHash string
	*fslib.FsLib
	*depproc.DepProcCtl
}

func MakeExecutor(args []string, debug bool) (*Executor, error) {
	db.DPrintf("Executor: %v\n", args)
	ex := &Executor{}
	ex.pid = args[0]
	ex.thunkHash = args[1]
	fls := fslib.MakeFsLib("executor")
	ex.FsLib = fls
	ex.DepProcCtl = depproc.MakeDepProcCtl(fls, depproc.DEFAULT_JOB_ID)
	ex.Started(ex.pid)
	return ex, nil
}

func (ex *Executor) Work() {
	setupLocalExecutionEnv(ex.thunkHash)
	ex.downloadInputFiles()
	ex.exec()
	ex.uploadOutputFiles()
}

func (ex *Executor) downloadInputFiles() {
	// Download the thunk itself
	downloadFile(ex, ggRemoteBlobs(ex.thunkHash), ggLocalBlobs(ex.thunkHash, ex.thunkHash))
	inputDeps := getInputDependencies(ex, ex.thunkHash, ggRemoteBlobs(""))
	for _, dep := range inputDeps {
		if isThunk(dep) {
			// If it's a thunk, download the reduction file
			downloadFile(ex, ggRemoteReductions(dep), ggLocalReductions(ex.thunkHash, dep))
			// And download the reduction itself
			reduction := getReductionResult(ex, dep)
			downloadFile(ex, ggRemoteBlobs(reduction), ggLocalBlobs(ex.thunkHash, reduction))
		} else {
			downloadFile(ex, ggRemoteBlobs(dep), ggLocalBlobs(ex.thunkHash, dep))
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
		uploadDir(ex, ex.thunkHash, subDir)
	}
}

func (ex *Executor) Name() string {
	return "Executor " + ex.pid + " "
}

func (ex *Executor) Exit() {
	ex.Exited(ex.pid)
}
