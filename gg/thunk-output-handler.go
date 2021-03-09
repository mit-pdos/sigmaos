package gg

import (
	"log"
	"path"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
)

type ThunkOutputHandler struct {
	pid                   string
	thunkHash             string
	primaryOutputThunkPid string
	outputFiles           []string
	cwd                   string
	*fslib.FsLib
}

func MakeThunkOutputHandler(args []string, debug bool) (*ThunkOutputHandler, error) {
	db.DPrintf("ThunkOutputHandler: %v\n", args)
	toh := &ThunkOutputHandler{}

	toh.pid = args[0]
	toh.thunkHash = args[1]
	toh.outputFiles = args[2:]
	toh.cwd = path.Join(
		GG_LOCAL,
		toh.thunkHash,
	)
	fls := fslib.MakeFsLib("gg-thunk-output-handler")
	toh.FsLib = fls
	db.SetDebug(debug)
	toh.Started(toh.pid)
	return toh, nil
}

func (toh *ThunkOutputHandler) Exit() {
	toh.Exiting(toh.pid, "OK")
}

// XXX Memoize to avoid redundant work
func (toh *ThunkOutputHandler) Work() {
	// Read the thunk output file
	thunkOutput := toh.readThunkOutput()
	newThunks := toh.getNewThunks(thunkOutput)
	outputFiles := toh.getOutputFiles(thunkOutput)
	if len(newThunks) == 0 {
		// We have produced a value, and need to propagate it upstream to functions
		// which depend on us.
		toh.propagateResultUpstream()
	} else {
		for _, thunk := range newThunks {
			//			inputDependencies := getInputDependencies(toh, thunk.hash, ggRemoteBlobs(""))
			//			depPids := outputHandlerPids(thunk.deps)
			//			downloaders := spawnInputDownloaders(toh, thunk.hash, path.Join(GG_LOCAL, thunk.hash), inputDependencies, depPids)
			//			exitDeps := []string{}
			//			exitDeps = append(exitDeps, downloaders...)
			// Avoid doing redundant work
			if doneExecuting(toh, thunk.hash) || currentlyExecuting(toh, thunk.hash) {
				continue
			}
			exitDeps := outputHandlerPids(thunk.deps)
			toh.spawnDownstreamThunk(thunk.hash, exitDeps, outputFiles)
		}
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
}

func (toh *ThunkOutputHandler) spawnDownstreamThunk(thunkHash string, deps []string, outputFiles map[string][]string) string {
	db.DPrintf("Handler [%v] spawning [%v], depends on [%v]\n", toh.thunkHash, thunkHash, deps)
	//	exPid := spawnExecutor(toh, thunkHash, deps)
	//	resUploaders := spawnThunkResultUploaders(toh, thunkHash)
	//	outputHandlerPid := spawnThunkOutputHandler(toh, append(resUploaders, exPid), thunkHash, outputFiles[thunkHash])
	exPid := spawnExecutor(toh, thunkHash, deps)
	outputHandlerPid := spawnThunkOutputHandler(toh, []string{exPid}, thunkHash, outputFiles[thunkHash])
	return spawnNoOp(toh, outputHandlerPid)
}

func (toh *ThunkOutputHandler) propagateResultUpstream() {
	reduction := toh.getReduction()
	db.DPrintf("Thunk [%v] got value [%v], propagating back to [%v]\n", toh.thunkHash, reduction, toh.outputFiles)
	for _, outputFile := range toh.outputFiles {
		outputPath := ggRemoteReductions(outputFile)
		toh.WriteFile(outputPath, []byte(reduction))
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

func (toh *ThunkOutputHandler) getNewThunks(thunkOutput []string) []Thunk {
	// Maps of new thunks to their dependencies
	g := MakeGraph()
	first := true
	for _, line := range thunkOutput {
		thunkLine := strings.Split(strings.TrimSpace(line), "=")[1]
		// Compute new thunk's dependencies
		hashes := strings.Split(thunkLine, " ")
		g.AddThunk(hashes[0], hashes[1:])
		if first {
			// XXX I'm actually not sure if I should just wait on the output handler
			// here, but for consistency, I'll wait on the no-op
			outputHandlerPid := outputHandlerPid(hashes[0])
			noOpPid := noOpPid(outputHandlerPid)
			toh.primaryOutputThunkPid = noOpPid
			first = false
		}
	}
	return g.GetThunks()
}

func (toh *ThunkOutputHandler) readThunkOutput() []string {
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

func (toh *ThunkOutputHandler) getValue() string {
	filePath := ggRemoteBlobs(toh.getReduction())
	contents, err := toh.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error reading value file[%v]: %v\n", filePath, err)
	}
	return strings.TrimSpace(string(contents))
}

func (toh *ThunkOutputHandler) getReduction() string {
	thunkOutputPath := ggRemoteReductions(toh.thunkHash)
	valueFile, err := toh.ReadFile(thunkOutputPath)
	if err != nil {
		log.Fatalf("Error reading reduction in TOH [%v]: %v\n", thunkOutputPath, err)
	}
	return strings.TrimSpace(string(valueFile))
}

func (toh *ThunkOutputHandler) getCwd() string {
	return toh.cwd
}

func outputHandlerPids(deps map[string]bool) []string {
	out := []string{}
	for d, _ := range deps {
		pid := outputHandlerPid(d)
		noOpPid := noOpPid(pid)
		out = append(out, noOpPid)
	}
	return out
}
