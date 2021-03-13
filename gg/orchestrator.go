package gg

import (
	"io/ioutil"
	"log"
	"path"
	//	"runtime/debug"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
	np "ulambda/ninep"
)

const (
	ORCHESTRATOR              = "name/gg/orchestrator"
	THUNK_OUTPUTS_SUFFIX      = ".thunk-outputs"
	EXIT_DEPENDENCIES_SUFFIX  = ".exit-dependencies"
	INPUT_DEPENDENCIES_SUFFIX = ".input-dependencies"
	SHEBANG_DIRECTIVE         = "#!/usr/bin/env gg-force-and-run"
)

type ExecutorLauncher interface {
	FsLambda
	Spawn(*fslib.Attr) error
	SpawnNoOp(string, []string) error
	Started(string) error
}

type OrchestratorDev struct {
	orc *Orchestrator
}

func (orcdev *OrchestratorDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	return np.Tsize(len(data)), nil
}

func (orcdev *OrchestratorDev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
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
	memfsd := memfsd.MakeFsd("orchestrator", ":0", nil)
	fls, err := fslib.InitFs(ORCHESTRATOR, memfsd, &OrchestratorDev{orc})
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
	origDirUploaders := []string{spawnOrigDirUploader(orc, orc.cwd, GG_BLOBS)}
	orc.waitPids(origDirUploaders)
	for i, target := range orc.targets {
		targetHash := orc.getTargetHash(target)
		orc.targetHashes = append(orc.targetHashes, targetHash)
		g := orc.ingestStaticGraph(targetHash)
		orc.executeStaticGraph(g, origDirUploaders)
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
		exitDeps := outputHandlerPids(thunk.deps)
		exPid := spawnExecutor(orc, thunk.hash, exitDeps)
		outputHandlerPid := spawnThunkOutputHandler(orc, []string{exPid}, thunk.hash, []string{thunk.hash})
		spawnNoOp(orc, outputHandlerPid)
	}
}

func (orc *Orchestrator) waitPids(pids []string) {
	for _, p := range pids {
		orc.Wait(p)
	}
}

func (orc *Orchestrator) getExitDependencies(targetHash string) []string {
	dependencies := []string{}
	dependenciesFilePath := ggRemoteBlobs(targetHash + EXIT_DEPENDENCIES_SUFFIX)
	f, err := orc.ReadFile(dependenciesFilePath)
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
		targetReduction := ggRemoteReductions(orc.targetHashes[i])
		f, err := orc.ReadFile(targetReduction)
		if err != nil {
			log.Fatalf("Error reading target reduction: %v\n", err)
		}
		outputHash := strings.TrimSpace(string(f))
		outputPath := ggRemoteBlobs(outputHash)
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

func (orc *Orchestrator) mkdirOpt(path string) {
	_, err := orc.FsLib.Stat(path)
	if err != nil {
		db.DPrintf("Mkdir [%v]\n", path)
		// XXX Perms?
		err = orc.FsLib.Mkdir(path, np.DMDIR)
		if err != nil {
			log.Fatalf("Couldn't mkdir %v: %v", path, err)
		}
	} else {
		db.DPrintf("Already exists [%v]\n", path)
	}
}

func (orc *Orchestrator) setUpRemoteDirs() {
	orc.mkdirOpt(ggRemote("", ""))
	orc.mkdirOpt(ggRemoteBlobs(""))
	orc.mkdirOpt(ggRemoteReductions(""))
	orc.mkdirOpt(ggRemoteHashCache(""))
}

func getInputDependencies(launch ExecutorLauncher, targetHash string, srcDir string) []string {
	dependencies := []string{}
	dependenciesFilePath := path.Join(
		srcDir,
		targetHash+INPUT_DEPENDENCIES_SUFFIX,
	)
	// Read either from local or remote storage
	var f []byte
	var err error
	if isRemote(srcDir) {
		f, err = launch.ReadFile(dependenciesFilePath)
	} else {
		f, err = ioutil.ReadFile(dependenciesFilePath)
	}
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

// XXX This doesn't actually do what I think it does :P
// XXX Should really check if it's in the queue as well (not just if it's
//  actively running. Also, there may be a slight race
func currentlyExecuting(launch ExecutorLauncher, thunkHash string) bool {
	err := launch.Started(executorPid(thunkHash))
	if err == nil {
		return true
	}
	return false
}

func (orc *Orchestrator) Name() string {
	return "Orchestrator"
}
