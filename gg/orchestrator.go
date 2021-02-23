package gg

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	//	"runtime/debug"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

// XXX eventually make GG dirs configurable, both here & in GG
const (
	GG_TOP_DIR                = "name/gg"
	GG_DIR                    = "name/fs/.gg"
	GG_DIR_TMP                = ".gg"
	GG_BLOB_DIR               = GG_DIR + "/blobs"
	GG_BLOBS                  = ".gg/blobs"
	GG_REDUCTION_DIR          = GG_DIR + "/reductions"
	GG_REDUCTIONS             = ".gg/reductions"
	GG_HASH_CACHE_DIR         = GG_DIR + "/hash_cache"
	GG_HASH_CACHE             = ".gg/hash_cache"
	GG_LOCAL_DIR              = "/tmp/ulambda"
	GG_REMOTE_DIR             = "name/fs"
	ORCHESTRATOR              = GG_TOP_DIR + "/orchestrator"
	UPLOADER_SUFFIX           = ".uploader"
	DOWNLOADER_SUFFIX         = ".downloader"
	EXECUTOR_SUFFIX           = ".executor"
	TARGET_WRITER_SUFFIX      = ".target-writer"
	OUTPUT_HANDLER_SUFFIX     = ".output-handler"
	THUNK_OUTPUTS_SUFFIX      = ".thunk-outputs"
	EXIT_DEPENDENCIES_SUFFIX  = ".exit-dependencies"
	INPUT_DEPENDENCIES_SUFFIX = ".input-dependencies"
	SHEBANG_DIRECTIVE         = "#!/usr/bin/env gg-force-and-run"
)

type ExecutorLauncher interface {
	Spawn(*fslib.Attr) error
	getCwd() string
}

type OrchestratorDev struct {
	orc *Orchestrator
}

func (orcdev *OrchestratorDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	//  t := string(data)
	//  db.DPrintf("OrchestratorDev.write %v\n", t)
	//  if strings.HasPrefix(t, "Join") {
	//    orcdev.orc.join(t[len("Join "):])
	//  } else if strings.HasPrefix(t, "Leave") {
	//    orcdev.orc.leave(t[len("Leave"):])
	//  } else if strings.HasPrefix(t, "Add") {
	//    orcdev.orc.add()
	//  } else if strings.HasPrefix(t, "Resume") {
	//    orcdev.orc.resume(t[len("Resume "):])
	//  } else {
	//    return 0, fmt.Errorf("Write: unknown command %v\n", t)
	//  }
	return np.Tsize(len(data)), nil
}

func (orcdev *OrchestratorDev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	//  if off == 0 {
	//  s := orcdev.sd.ps()
	//return []byte(s), nil
	//}
	return nil, nil
}

func (orcdev *OrchestratorDev) Len() np.Tlength {
	return 0
}

type Orchestrator struct {
	pid          string
	cwd          string
	targets      []string
	targetHashes []string
	*fslib.FsLibSrv
}

func MakeOrchestrator(args []string, debug bool) (*Orchestrator, error) {
	log.Printf("Orchestrator: %v\n", args)
	orc := &Orchestrator{}

	orc.pid = args[0]
	orc.cwd = args[1]
	orc.targets = args[2:]
	fls, err := fslib.InitFs(ORCHESTRATOR, &OrchestratorDev{orc})
	if err != nil {
		return nil, err
	}
	orc.FsLibSrv = fls
	//  db.SetDebug(true)
	orc.Started(orc.pid)
	return orc, nil
}

func (orc *Orchestrator) Exit() {
	orc.Exiting(orc.pid, "OK")
}

func (orc *Orchestrator) Work() {
	orc.setUpRemoteDirs()
	// XXX  need to change how we intake the initial graph
	executables := orc.getExecutableDependencies()
	exUpPids := orc.uploadExecutableDependencies(executables)
	for i, target := range orc.targets {
		targetHash := orc.getTargetHash(target)
		orc.targetHashes = append(orc.targetHashes, targetHash)
		g := orc.ingestStaticGraph(targetHash)
		orc.executeStaticGraph(g, exUpPids)
		spawnReductionWriter(orc, orc.targets[i], targetHash, orc.cwd, "", []string{})
	}
}

func (orc *Orchestrator) ingestStaticGraph(targetHash string) *Graph {
	g := MakeGraph()
	orc.targetHashes = append(orc.targetHashes, targetHash)
	var current string
	queue := []string{targetHash}
	// Will loop inifinitely if the graph is circular
	for len(queue) > 0 {
		current, queue = queue[0], queue[1:]
		exitDeps := orc.getExitDependencies(current)
		g.AddThunk(current, exitDeps)
		queue = append(queue, exitDeps...)
	}
	return g
}

func (orc *Orchestrator) executeStaticGraph(g *Graph, uploadDeps []string) {
	thunks := g.GetThunks()
	for _, thunk := range thunks {
		// XXX handle non-thunk targets
		exitDeps := outputHandlerPids(thunk.deps)
		inputDependencies := getInputDependencies(orc, thunk.hash)
		uploaderPids := []string{
			spawnUploader(orc, thunk.hash, orc.cwd, "blobs"),
		}
		uploaderPids = append(uploaderPids, uploadDeps...)
		oldCwd := orc.cwd
		orc.cwd = path.Join(GG_LOCAL_DIR, thunk.hash)
		// XXX A dirty hack... I should do something more principled
		inputDownloaderPids := spawnInputDownloaders(orc, thunk.hash, path.Join(GG_LOCAL_DIR, thunk.hash), inputDependencies, uploaderPids)
		orc.cwd = oldCwd
		exitDeps = append(exitDeps, uploaderPids...)
		exitDeps = append(exitDeps, inputDownloaderPids...)
		exPid := spawnExecutor(orc, thunk.hash, exitDeps)
		spawnThunkOutputHandler(orc, exPid, thunk.hash, []string{thunk.hash})
	}
}

func (orc *Orchestrator) getExitDependencies(targetHash string) []string {
	dependencies := []string{}
	dependenciesFilePath := path.Join(
		orc.cwd,
		".gg",
		"blobs",
		targetHash+EXIT_DEPENDENCIES_SUFFIX,
	)
	f, err := ioutil.ReadFile(dependenciesFilePath)
	if err != nil {
		db.DPrintf("No exit dependencies file for [%v]: %v\n", targetHash, err)
		return dependencies
	}
	f_trimmed := strings.TrimSpace(string(f))
	if len(f_trimmed) > 0 {
		for _, d := range strings.Split(f_trimmed, "\n") {
			dependencies = append(dependencies, d)
		}
	}
	db.DPrintf("Got exit dependencies for [%v]: %v\n", targetHash, dependencies)
	return dependencies
}

func (orc *Orchestrator) writeTargets() {
	for i, target := range orc.targets {
		targetReduction := path.Join(
			GG_REDUCTION_DIR,
			orc.targetHashes[i],
		)
		f, err := orc.ReadFile(targetReduction)
		if err != nil {
			log.Fatalf("Error reading target reduction: %v\n", err)
		}
		outputHash := strings.TrimSpace(string(f))
		outputPath := path.Join(
			GG_BLOB_DIR,
			outputHash,
		)
		outputValue, err := orc.ReadFile(outputPath)
		if err != nil {
			log.Fatalf("Error reading value path: %v\n", err)
		}
		err = ioutil.WriteFile(target, outputValue, 0777)
		if err != nil {
			log.Fatalf("Error writing output file: %v\n", err)
		}
		db.DPrintf("Wrote output file [%v]\n", target)
	}
}

func (orc *Orchestrator) uploadExecutableDependencies(execs []string) []string {
	pids := []string{}
	for _, exec := range execs {
		pids = append(pids, spawnUploader(orc, exec, orc.cwd, "blobs"))
	}
	return pids
}

func (orc *Orchestrator) getExecutableDependencies() []string {
	execsPath := path.Join(orc.cwd, ".gg", "blobs", "executables.txt")
	f, err := ioutil.ReadFile(execsPath)
	if err != nil {
		log.Fatalf("Error reading exec dependencies: %v\n", err)
	}
	trimmed_f := strings.TrimSpace(string(f))
	return strings.Split(trimmed_f, "\n")
}

func (orc *Orchestrator) getTargetHash(target string) string {
	// XXX support non-placeholders
	targetPath := path.Join(orc.cwd, target)
	f, err := ioutil.ReadFile(targetPath)
	contents := string(f)
	if err != nil {
		log.Fatalf("Error reading target [%v]: %v\n", target, err)
	}
	shebang := strings.Split(contents, "\n")[0]
	if shebang != SHEBANG_DIRECTIVE {
		log.Fatalf("Error: [%v] is not a placeholder [%v]", targetPath, shebang)
	}
	hash := strings.Split(contents, "\n")[1]
	return hash
}

func (orc *Orchestrator) getCwd() string {
	return orc.cwd
}

func (orc *Orchestrator) mkdirOpt(path string) {
	_, err := orc.FsLib.Stat(path)
	if err != nil {
		db.DPrintf("Mkdir [%v]\n", path)
		// XXX Perms?
		err = orc.FsLib.Mkdir(path, np.DMDIR)
		if err != nil {
			log.Fatalf("Couldn't mkdir %v: %v", GG_DIR, err)
		}
	} else {
		db.DPrintf("Already exists [%v]\n", path)
	}
}

func (orc *Orchestrator) setUpRemoteDirs() {
	orc.mkdirOpt(ggRemote(GG_DIR_TMP, ""))
	orc.mkdirOpt(ggRemoteBlobs(""))
	orc.mkdirOpt(ggRemoteReductions(""))
	orc.mkdirOpt(ggRemoteHashCache(""))
}

func getInputDependencies(launch ExecutorLauncher, targetHash string) []string {
	dependencies := []string{}
	dependenciesFilePath := path.Join(
		launch.getCwd(),
		".gg",
		"blobs",
		targetHash+INPUT_DEPENDENCIES_SUFFIX,
	)
	f, err := ioutil.ReadFile(dependenciesFilePath)
	if err != nil {
		db.DPrintf("No input dependencies file for [%v]: %v\n", targetHash, err)
		return dependencies
	}
	f_trimmed := strings.TrimSpace(string(f))
	if len(f_trimmed) > 0 {
		for _, d := range strings.Split(f_trimmed, "\n") {
			dependencies = append(dependencies, d)
		}
	}
	db.DPrintf("Got input dependencies for [%v]: %v\n", targetHash, dependencies)
	return dependencies
}

func setupLocalExecutionEnv(launch ExecutorLauncher, targetHash string) string {
	envPath := path.Join(
		GG_LOCAL_DIR,
		targetHash,
		".gg",
	)
	subDirs := []string{
		path.Join(envPath, "blobs"),
		path.Join(envPath, "reductions"),
		path.Join(envPath, "hash_cache"),
	}
	for _, d := range subDirs {
		err := os.MkdirAll(d, 0777)
		if err != nil {
			log.Fatalf("Error making execution env dir [%v]: %v\n", envPath, err)
		}
	}
	return envPath
}

func spawnInputDownloaders(launch ExecutorLauncher, targetHash string, dstDir string, inputs []string, exitDeps []string) []string {
	downloaders := []string{}
	// XXX A dirty hack... I should do something more principled

	// Make sure to download target thunk file as well
	// XXX Should get the right exitDeps from the fn call...
	// XXX Should not manually calc uploader PID here
	uploaderPid := "[" + targetHash + ".blobs]" + targetHash + UPLOADER_SUFFIX
	downloaders = append(downloaders, spawnDownloader(launch, targetHash, dstDir, "blobs", append(exitDeps, uploaderPid)))
	//	downloaders = append(downloaders, spawnDownloader(launch, targetHash, "blobs", exitDeps))
	for _, input := range inputs {
		if isThunk(input) {
			inputPidReduction := "[" + input + ".reductions]" + input + UPLOADER_SUFFIX
			// Download the thunk reduction file as well as the value it points to
			reductionDownloader := spawnDownloader(launch, input, dstDir, "reductions", append(exitDeps, inputPidReduction))
			//			reductionDownloader := spawnDownloader(launch, input, "reductions", exitDeps)

			// set target == targetReduction to preserve target name
			// XXX waiting for reductionDownloader is a hack, and needs to be replaced
			reductionWriter := spawnReductionWriter(launch, input, input, dstDir, path.Join(".gg", "blobs"), append(exitDeps, reductionDownloader))
			//			reductionWriter := spawnReductionWriter(launch, input, input, path.Join(".gg", "blobs"), exitDeps)
			downloaders = append(downloaders, reductionDownloader, reductionWriter)
		} else {
			downloaders = append(downloaders, spawnDownloader(launch, input, dstDir, "blobs", exitDeps))
		}
	}
	return downloaders
}

// XXX Clean up naming conventions, include subdir
func spawnDownloader(launch ExecutorLauncher, targetHash string, dstDir string, subDir string, exitDeps []string) string {
	a := fslib.Attr{}
	a.Pid = "[" + path.Base(dstDir) + "." + subDir + "]" + targetHash + DOWNLOADER_SUFFIX
	//	log.Printf("Spawning d %v deps %v", a.Pid, exitDeps)
	//	debug.PrintStack()
	a.Program = "./bin/fsdownloader"
	a.Args = []string{
		path.Join(GG_DIR, subDir, targetHash),
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
	a.Pid = "[" + path.Base(srcDir) + "." + subDir + "]" + targetHash + UPLOADER_SUFFIX
	//	log.Printf("Spawned uploader %v\n", a.Pid)
	a.Program = "./bin/fsuploader"
	a.Args = []string{
		path.Join(srcDir, ".gg", subDir, targetHash),
		path.Join(GG_DIR, subDir, targetHash),
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
	a.Pid = "[" + path.Base(dstDir) + "." + path.Base(subDir) + "]" + target + TARGET_WRITER_SUFFIX
	a.Program = "./bin/gg-target-writer"
	a.Args = []string{
		path.Join(dstDir, subDir),
		target,
		targetReduction,
	}
	a.Env = []string{}
	a.PairDep = []fslib.PDep{}
	reductionPid := targetReduction + OUTPUT_HANDLER_SUFFIX
	deps = append(deps, reductionPid)
	a.ExitDep = deps
	err := launch.Spawn(&a)
	if err != nil {
		log.Fatalf("Error spawning target writer [%v]: %v\n", target, err)
	}
	return a.Pid
}

func spawnExecutor(launch ExecutorLauncher, targetHash string, depPids []string) string {
	/* envPath := */ setupLocalExecutionEnv(launch, targetHash)
	a := fslib.Attr{}
	a.Pid = targetHash + EXECUTOR_SUFFIX
	a.Program = "gg-execute"
	a.Args = []string{
		"--ninep",
		targetHash,
	}
	a.Dir = path.Join(GG_LOCAL_DIR, targetHash, ".gg")
	a.Env = []string{
		//		"GG_STORAGE_URI=9p://mnt/9p/fs",
		//		"GG_DIR=/mnt/9p/fs/.gg", // XXX Make this configurable
		"GG_DIR=" + path.Join(GG_LOCAL_DIR, targetHash, ".gg"), // XXX Make this configurable
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
	a.Pid = thunkHash + OUTPUT_HANDLER_SUFFIX
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
