package gg

import (
	"log"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/proc"
)

type ThunkOutputHandler struct {
	pid                    string
	thunkHash              string
	primaryOutputThunkHash string
	primaryOutputThunkPid  string
	outputFiles            []string
	*fslib.FsLib
	*proc.ProcCtl
}

func MakeThunkOutputHandler(args []string, debug bool) (*ThunkOutputHandler, error) {
	db.DPrintf("ThunkOutputHandler: %v\n", args)

	toh := mkThunkOutputHandler(args[0], args[1], args[2:])
	toh.Started(toh.pid)
	return toh, nil
}

func mkThunkOutputHandler(pid string, thunkHash string, outputFiles []string) *ThunkOutputHandler {
	toh := &ThunkOutputHandler{}
	toh.pid = pid
	toh.thunkHash = thunkHash
	toh.outputFiles = outputFiles
	fls := fslib.MakeFsLib("gg-thunk-output-handler")
	toh.FsLib = fls
	toh.ProcCtl = proc.MakeProcCtl(fls)
	return toh
}

func (toh *ThunkOutputHandler) Exit() {
	//	toh.Exiting(toh.pid, "OK")
}

func (toh *ThunkOutputHandler) Work() {
	newThunks := toh.processOutput()
	for _, thunk := range newThunks {
		// Avoid doing redundant work
		if reductionExists(toh, thunk.hash) || currentlyExecuting(toh, thunk.hash) {
			continue
		}
		exitDeps := outputHandlerPids(thunk.deps)
		toh.initDownstreamThunk(thunk.hash, exitDeps, thunk.outputFiles)
	}
	if len(newThunks) != 0 {
		toh.adjustExitDependencies()
	}
}

func (toh *ThunkOutputHandler) processOutput() []*Thunk {
	// Read the thunk output file
	thunkOutput := toh.readThunkOutputs()
	outputFiles := toh.getOutputFiles(thunkOutput)
	newThunks := toh.getNewThunks(thunkOutput, outputFiles)
	if len(newThunks) == 0 {
		// We have produced a value, and need to propagate it upstream to functions
		// which depend on us.
		toh.propagateResultUpstream()
	}
	return newThunks
}

func (toh *ThunkOutputHandler) initDownstreamThunk(thunkHash string, deps []string, outputFiles []string) string {
	db.DPrintf("Handler [%v] spawning [%v], depends on [%v]\n", toh.thunkHash, thunkHash, deps)
	exPid, err := spawnExecutor(toh, thunkHash, deps)
	if err != nil {
		log.Printf("%v", err)
	}
	outputHandlerPid := spawnThunkOutputHandler(toh, []string{exPid}, thunkHash, outputFiles)
	return spawnNoOp(toh, outputHandlerPid)
}

func (toh *ThunkOutputHandler) propagateResultUpstream() {
	reduction := getReductionResult(toh, toh.thunkHash)
	db.DPrintf("Thunk [%v] got value [%v], propagating back to [%v]\n", toh.thunkHash, reduction, toh.outputFiles)
	for _, outputFile := range toh.outputFiles {
		outputPath := ggRemoteReductions(outputFile)
		toh.WriteFile(outputPath, []byte(reduction))
	}
}

func (toh *ThunkOutputHandler) adjustExitDependencies() {
	exitDepSwaps := []string{
		toh.pid,
		toh.primaryOutputThunkPid,
	}
	db.DPrintf("Updating exit dependencies for [%v]\n", toh.pid)
	err := toh.SwapExitDependency(exitDepSwaps)
	if err != nil {
		log.Fatalf("Couldn't swap exit dependencies %v: %v\n", exitDepSwaps, err)
	}
}

func (toh *ThunkOutputHandler) getOutputFiles(thunkOutput []string) map[string][]string {
	outputFiles := make(map[string][]string)
	first := true
	for _, line := range thunkOutput {
		// Get output file name this thunk corresponds to
		result := strings.Split(strings.TrimSpace(line), "=")
		tag := result[0]
		// Get output thunk's hash
		hash := strings.Split(result[1], " ")[0]
		if _, ok := outputFiles[hash]; ok {
			log.Fatalf("output file was already in map when parsing thunk output")
		}
		outputFiles[hash] = []string{toh.thunkHash + "#" + tag}
		if first {
			first = false
			outputFiles[hash] = append(outputFiles[hash], toh.thunkHash)
		}
	}
	return outputFiles
}

func (toh *ThunkOutputHandler) getNewThunks(thunkOutput []string, outputFiles map[string][]string) []*Thunk {
	// Maps of new thunks to their dependencies
	g := MakeGraph()
	first := true
	for _, line := range thunkOutput {
		thunkLine := strings.Split(strings.TrimSpace(line), "=")[1]
		// Compute new thunk's dependencies
		hashes := strings.Split(thunkLine, " ")
		g.AddThunk(hashes[0], hashes[1:], outputFiles[hashes[0]])
		if first {
			// XXX I'm actually not sure if I should just wait on the output handler
			// here, but for consistency, I'll wait on the no-op
			toh.primaryOutputThunkHash = hashes[0]
			outputHandlerPid := outputHandlerPid(hashes[0])
			noOpPid := noOpPid(outputHandlerPid)
			toh.primaryOutputThunkPid = noOpPid
			first = false
		}
	}
	return g.GetThunks()
}

func (toh *ThunkOutputHandler) readThunkOutputs() []string {
	outputThunksPath := ggRemoteBlobs(toh.thunkHash + THUNK_OUTPUTS_SUFFIX)
	contents, err := toh.ReadFile(outputThunksPath)
	if err != nil {
		log.Fatalf("Error reading thunk outputs [%v]: %v\n", outputThunksPath, err)
	}
	trimmedContents := strings.TrimSpace(string(contents))
	if len(trimmedContents) > 0 {
		return strings.Split(trimmedContents, "\n")
	} else {
		return []string{}
	}
}

func (toh *ThunkOutputHandler) Name() string {
	return "ThunkOutputHandler " + toh.pid + " "
}
