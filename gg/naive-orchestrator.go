package gg

import (
	"log"
	"path"
	"sync"
	"time"

	"ulambda/fslib"
	"ulambda/proc"
)

const (
	N_WORKERS = 10
)

type NaiveOrchestrator struct {
	mu           sync.Mutex
	thunkQ       chan *Thunk
	nRemaining   int
	done         bool
	wg           *sync.WaitGroup
	g            *Graph
	pid          string
	cwd          string
	targets      []string
	targetHashes []string
	*fslib.FsLib
	*proc.ProcCtl
}

func MakeNaiveOrchestrator(args []string, debug bool) (*NaiveOrchestrator, error) {
	log.Printf("NaiveOrchestrator: %v\n", args)
	orc := &NaiveOrchestrator{}
	orc.thunkQ = make(chan *Thunk)
	orc.nRemaining = 0
	orc.done = false
	orc.wg = &sync.WaitGroup{}
	orc.g = MakeGraph()
	orc.pid = args[0]
	orc.cwd = args[1]
	orc.targets = args[2:]
	fls := fslib.MakeFsLib("orchestrator")
	orc.FsLib = fls
	orc.ProcCtl = proc.MakeProcCtl(fls, orc.pid)
	orc.Started(orc.pid)
	return orc, nil
}

func (orc *NaiveOrchestrator) Exit() {
	//	orc.Exiting(orc.pid, "OK")
}

func (orc *NaiveOrchestrator) stillProcessing() bool {
	orc.mu.Lock()
	defer orc.mu.Unlock()
	return orc.nRemaining > 0
}

func (orc *NaiveOrchestrator) workerThread() {
	defer orc.wg.Done()
	for orc.stillProcessing() {
		thunk, ok := <-orc.thunkQ
		if !ok {
			break
		}
		if reductionExists(orc, thunk.hash) || currentlyExecuting(orc, thunk.hash) || isReduction(thunk.hash) {
			orc.mu.Lock()
			orc.nRemaining -= 1
			orc.mu.Unlock()
			continue
		}
		// Spawn an executor and wait for the result...
		pid, err := spawnExecutor(orc, thunk.hash, []string{})
		if err != nil {
			orc.mu.Lock()
			orc.nRemaining -= 1
			orc.mu.Unlock()
			continue
		}
		orc.Wait(pid)
		toh := mkThunkOutputHandler("", thunk.hash, thunk.outputFiles)
		newThunks := toh.processOutput()
		orc.mu.Lock()
		// Force thunk we just executed
		orc.g.ForceThunk(thunk.hash)
		orc.nRemaining -= 1
		if toh.primaryOutputThunkHash != "" {
			orc.g.SwapDeps(thunk.hash, toh.primaryOutputThunkHash)
		}
		for _, t := range newThunks {
			deps := []string{}
			for dep, _ := range t.deps {
				deps = append(deps, dep)
			}
			orc.g.AddThunk(t.hash, deps, t.outputFiles)
		}
		// Add newly runnable thunks to the queue
		orc.updateThunkQL()
		orc.mu.Unlock()
	}
	orc.mu.Lock()
	if !orc.done {
		close(orc.thunkQ)
		orc.done = true
	}
	orc.mu.Unlock()
}

func (orc *NaiveOrchestrator) Work() {
	setUpRemoteDirs(orc)
	copyRemoteDirTree(orc, path.Join(orc.cwd, ".gg"), ggRemote("", ""))
	start := time.Now()
	for _, target := range orc.targets {
		targetHash := getTargetHash(orc, orc.cwd, target)
		orc.targetHashes = append(orc.targetHashes, targetHash)
		orc.ingestStaticGraph(targetHash)
		// Ignore reductions, which aren't actually executable
		if !isReduction(targetHash) {
			orc.updateThunkQ()
		}
	}
	// Spawn worker threads (contexts to wait on running lambdas)
	// XXX make N_WORKERS configurable...
	for i := 0; i < N_WORKERS; i++ {
		orc.wg.Add(1)
		go orc.workerThread()
	}
	orc.wg.Wait()

	// Write back targets
	// XXX eventually get rid of this...
	var targetsWritten sync.WaitGroup
	for i, _ := range orc.targets {
		if isReduction(orc.targets[i]) {
			continue
		}
		targetsWritten.Add(1)
		go func(i int, wg *sync.WaitGroup) {
			defer wg.Done()
			pid := spawnReductionWriter(orc, orc.targets[i], orc.targetHashes[i], path.Join(orc.cwd, "results"), "", []string{})
			orc.Wait(pid)
		}(i, &targetsWritten)
	}
	targetsWritten.Wait()
	end := time.Now()
	elapsed := end.Sub(start)
	log.Printf("Elapsed time: %v ms", elapsed.Milliseconds())
}

func (orc *NaiveOrchestrator) updateThunkQ() {
	orc.mu.Lock()
	defer orc.mu.Unlock()

	orc.updateThunkQL()
}

func (orc *NaiveOrchestrator) updateThunkQL() {
	newRunnable := orc.g.GetRunnableThunks()
	// Update number of thunks in the pipeline
	orc.nRemaining += len(newRunnable)
	go func() {
		for _, t := range newRunnable {
			orc.thunkQ <- t
		}
	}()
}

func (orc *NaiveOrchestrator) ingestStaticGraph(targetHash string) {
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
		orc.g.AddThunk(current, exitDeps, []string{current})
		queue = append(queue, exitDeps...)
	}
}

func (orc *NaiveOrchestrator) waitPids(pids []string) {
	for _, p := range pids {
		orc.Wait(p)
	}
}

func (orc *NaiveOrchestrator) Name() string {
	return "NaiveOrchestrator"
}
