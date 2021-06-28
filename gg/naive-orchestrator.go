package gg

import (
	"log"
	"path"
	"sync"

	// db "ulambda/debug"
	"ulambda/fslib"
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
	orc.Started(orc.pid)
	return orc, nil
}

func (orc *NaiveOrchestrator) Exit() {
	//	orc.Exiting(orc.pid, "OK")
}

func (orc *NaiveOrchestrator) stillProcessing() bool {
	orc.mu.Lock()
	defer orc.mu.Unlock()
	//	log.Printf("Remaining: %v", orc.nRemaining)
	return orc.nRemaining > 0
}

func (orc *NaiveOrchestrator) workerThread() {
	defer orc.wg.Done()
	for orc.stillProcessing() {
		thunk, ok := <-orc.thunkQ
		if !ok {
			break
		}
		//		log.Printf("thunk to be executed: %v", thunk)
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
		//		log.Printf("========== Graph Pre %v ========== \n\n %v \n\n ========== Graph Pre %v ==========", thunk.hash, orc.g, thunk.hash)
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
			//			log.Printf("Adding thunk %v deps %v", t.hash, deps)
			orc.g.AddThunk(t.hash, deps, t.outputFiles)
		}
		//		log.Printf("========== Graph Post1 %v ========== \n\n %v \n\n ========== Graph Post1 %v ==========", thunk.hash, orc.g, thunk.hash)
		// Add newly runnable thunks to the queue
		orc.updateThunkQL()
		//		log.Printf("========== Graph Post2 %v ========== \n\n %v \n\n ========== Graph Post2 %v ==========", thunk.hash, orc.g, thunk.hash)
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

	log.Printf("\nTargets: %v\nHashes: %v\n", orc.targets, orc.targetHashes)

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
		// XXX should I add the thunk's has as an output file?
		orc.g.AddThunk(current, exitDeps, []string{current})
		queue = append(queue, exitDeps...)
	}
}

//func (orc *NaiveOrchestrator) executeStaticGraph(targetHash string, g *Graph) {
//	thunks := g.GetThunks()
//	for _, thunk := range thunks {
//		exitDeps := outputHandlerPids(thunk.deps)
//		if reductionExists(orc, thunk.hash) || currentlyExecuting(orc, thunk.hash) || isReduction(thunk.hash) {
//			continue
//		}
//		exPid := spawnExecutor(orc, thunk.hash, exitDeps)
//		outputHandlerPid := spawnThunkOutputHandler(orc, []string{exPid}, thunk.hash, []string{thunk.hash})
//		spawnNoOp(orc, outputHandlerPid)
//	}
//}

func (orc *NaiveOrchestrator) waitPids(pids []string) {
	for _, p := range pids {
		orc.Wait(p)
	}
}

func (orc *NaiveOrchestrator) Name() string {
	return "NaiveOrchestrator"
}
