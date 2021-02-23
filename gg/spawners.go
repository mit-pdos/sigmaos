package gg

import (
	"log"
	"path"
	//	"runtime/debug"

	"ulambda/fslib"
)

func spawnInputDownloaders(launch ExecutorLauncher, targetHash string, dstDir string, inputs []string, exitDeps []string) []string {
	downloaders := []string{}
	// XXX A dirty hack... I should do something more principled
	// Make sure to download target thunk file as well
	// XXX Should get the right exitDeps from the fn call...
	// XXX Should not manually calc uploader PID here
	//	uploaderPid := "[" + targetHash + ".blobs]" + targetHash + UPLOADER_SUFFIX
	//	downloaders = append(downloaders, spawnDownloader(launch, targetHash, dstDir, "blobs", append(exitDeps, uploaderPid)))
	downloaders = append(downloaders, spawnDownloader(launch, targetHash, dstDir, "blobs", exitDeps))
	for _, input := range inputs {
		if isThunk(input) {
			//			inputPidReduction := "[" + input + ".reductions]" + input + UPLOADER_SUFFIX
			// Download the thunk reduction file as well as the value it points to
			//		reductionDownloader := spawnDownloader(launch, input, dstDir, "reductions", append(exitDeps, inputPidReduction))
			reductionDownloader := spawnDownloader(launch, input, dstDir, "reductions", exitDeps)

			// set target == targetReduction to preserve target name
			// XXX waiting for reductionDownloader is a hack, and needs to be replaced
			//		reductionWriter := spawnReductionWriter(launch, input, input, dstDir, path.Join(".gg", "blobs"), append(exitDeps, reductionDownloader))
			reductionWriter := spawnReductionWriter(launch, input, input, dstDir, path.Join(".gg", "blobs"), exitDeps)
			downloaders = append(downloaders, reductionDownloader, reductionWriter)
		} else {
			downloaders = append(downloaders, spawnDownloader(launch, input, dstDir, "blobs", exitDeps))
		}
	}
	return downloaders
}

func spawnDownloader(launch ExecutorLauncher, targetHash string, dstDir string, subDir string, exitDeps []string) string {
	a := fslib.Attr{}
	a.Pid = uploaderPid(dstDir, subDir, targetHash)
	//	log.Printf("Spawning d %v deps %v", a.Pid, exitDeps)
	//	debug.PrintStack()
	a.Program = "./bin/fsdownloader"
	a.Args = []string{
		ggRemote(subDir, targetHash),
		path.Join(dstDir, ".gg", subDir, targetHash),
	}
	a.Env = []string{}
	a.PairDep = []fslib.PDep{}
	a.ExitDep = exitDeps
	err := launch.Spawn(&a)
	if err != nil {
		log.Fatalf("Error spawning download worker [%v]: %v\n", targetHash, err)
	}
	return a.Pid
}

func spawnUploader(launch ExecutorLauncher, targetHash string, srcDir string, subDir string) string {
	a := fslib.Attr{}
	a.Pid = uploaderPid(srcDir, subDir, targetHash)
	//	log.Printf("Spawned uploader %v\n", a.Pid)
	a.Program = "./bin/fsuploader"
	a.Args = []string{
		path.Join(srcDir, ".gg", subDir, targetHash),
		ggRemote(subDir, targetHash),
	}
	a.Env = []string{}
	a.PairDep = []fslib.PDep{}
	a.ExitDep = nil
	err := launch.Spawn(&a)
	if err != nil {
		log.Fatalf("Error spawning upload worker [%v]: %v\n", targetHash, err)
	}
	return a.Pid
}

func spawnReductionWriter(launch ExecutorLauncher, target string, targetReduction string, dstDir string, subDir string, deps []string) string {
	a := fslib.Attr{}
	a.Pid = reductionWriterPid(dstDir, subDir, target)
	a.Program = "./bin/gg-target-writer"
	a.Args = []string{
		path.Join(dstDir, subDir),
		target,
		targetReduction,
	}
	a.Env = []string{}
	a.PairDep = []fslib.PDep{}
	reductionPid := outputHandlerPid(targetReduction)
	deps = append(deps, reductionPid)
	a.ExitDep = deps
	err := launch.Spawn(&a)
	if err != nil {
		log.Fatalf("Error spawning target writer [%v]: %v\n", target, err)
	}
	return a.Pid
}

func spawnExecutor(launch ExecutorLauncher, targetHash string, depPids []string) string {
	setupLocalExecutionEnv(launch, targetHash)
	a := fslib.Attr{}
	a.Pid = executorPid(targetHash)
	a.Program = "gg-execute"
	a.Args = []string{
		"--ninep",
		targetHash,
	}
	a.Dir = ggLocal(targetHash, "", "")
	a.Env = []string{
		"GG_DIR=" + a.Dir,
		"GG_NINEP=true",
		"GG_VERBOSE=1",
	}
	a.PairDep = []fslib.PDep{}
	a.ExitDep = depPids
	err := launch.Spawn(&a)
	if err != nil {
		// XXX Clean this up better with caching
		//    log.Fatalf("Error spawning executor [%v]: %v\n", targetHash, err);
	}
	return a.Pid
}

func spawnThunkOutputHandler(launch ExecutorLauncher, exPid string, thunkHash string, outputFiles []string) string {
	a := fslib.Attr{}
	a.Pid = outputHandlerPid(thunkHash)
	a.Program = "./bin/gg-thunk-output-handler"
	a.Args = []string{
		thunkHash,
	}
	a.Args = append(a.Args, outputFiles...)
	a.Env = []string{}
	a.PairDep = []fslib.PDep{}
	a.ExitDep = []string{
		exPid,
	}
	err := launch.Spawn(&a)
	if err != nil {
		// XXX Clean this up better with caching
		//    log.Fatalf("Error spawning output handler [%v]: %v\n", thunkHash, err);
	}
	return a.Pid
}
