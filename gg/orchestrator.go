package gg

import (
	"log"
	"path"

	// db "ulambda/debug"
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
	orc.Started(orc.pid)
	return orc, nil
}

func (orc *Orchestrator) Exit() {
	orc.Exiting(orc.pid, "OK")
}

func (orc *Orchestrator) Work() {
	setUpRemoteDirs(orc)
	copyRemoteDirTree(orc, path.Join(orc.cwd, ".gg"), ggRemote("", ""))
	for i, target := range orc.targets {
		targetHash := getTargetHash(orc, orc.cwd, target)
		orc.targetHashes = append(orc.targetHashes, targetHash)
		g := orc.ingestStaticGraph(targetHash)
		// Ignore reductions, which aren't actually executable
		if !isReduction(targetHash) {
			orc.executeStaticGraph(targetHash, g)
			// TODO: How do I make sure the reduction writer properly waits in the other case?
			spawnReductionWriter(orc, orc.targets[i], targetHash, path.Join(orc.cwd, "results"), "", []string{})
		}
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
		if isReduction(current) {
			current = thunkHashFromReduction(current)
		}
		exitDeps := getExitDependencies(orc, current)
		exitDeps = thunkHashesFromReductions(exitDeps)
		g.AddThunk(current, exitDeps)
		queue = append(queue, exitDeps...)
	}
	return g
}

// XXX If it is a reduction, we should make sure to wait on the right thing...
func (orc *Orchestrator) executeStaticGraph(targetHash string, g *Graph) {
	thunks := g.GetThunks()
	for _, thunk := range thunks {
		exitDeps := outputHandlerPids(thunk.deps)
		if reductionExists(orc, thunk.hash) || currentlyExecuting(orc, thunk.hash) || isReduction(thunk.hash) {
			continue
		}
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

func (orc *Orchestrator) Name() string {
	return "Orchestrator"
}
