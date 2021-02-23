package gg

import (
	"io/ioutil"
	"log"
	"path"
	"strings"

	db "ulambda/debug"
	"ulambda/fslib"
)

type ThunkOutputHandler struct {
	pid                string
	thunkHash          string
	primaryOutputThunk string
	outputFiles        []string
	cwd                string
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
	// Upload results from local execution dir
	uploaders := toh.spawnResultUploaders()
	if len(newThunks) == 0 {
		// We have produced a value, and need to propagate it upstream to functions
		// which depend on us.
		toh.propagateResultUpstream()
	} else {
		_ = outputFiles
		for _, thunk := range newThunks {
			inputDependencies := getInputDependencies(toh, thunk.hash, ggLocalBlobs(toh.thunkHash, ""))
			depPids := outputHandlerPids(thunk.deps)
			// XXX Waiting for all uploaders is overly conservative... perhaps not necessary
			downloaders := spawnInputDownloaders(toh, thunk.hash, path.Join(GG_LOCAL, thunk.hash), inputDependencies, append(depPids, uploaders...))
			exitDeps := []string{}
			exitDeps = append(exitDeps, downloaders...)
			toh.spawnDownstreamThunk(thunk.hash, exitDeps, outputFiles)
		}
		exitDepSwaps := []string{
			toh.pid,
			toh.primaryOutputThunk,
		}
		db.DPrintf("Updating exit dependencies for [%v]\n", toh.pid)
		err := toh.SwapExitDependency(exitDepSwaps)
		if err != nil {
			log.Fatalf("Couldn't swap exit dependencies %v: %v\n", exitDepSwaps, err)
		}
	}
}

func (toh *ThunkOutputHandler) spawnResultUploaders() []string {
	uploaders := []string{}
	subDirs, err := ioutil.ReadDir(ggLocal(toh.thunkHash, "", ""))
	if err != nil {
		log.Fatalf("Couldn't read local dir [%v] contents: %v\n", toh.cwd, err)
	}

	// Upload contents of each subdir (blobs, reductions, hash_cache) to 9P remote
	// server
	for _, subDir := range subDirs {
		subdirPath := path.Join(ggLocal(toh.thunkHash, subDir.Name(), ""))
		files, err := ioutil.ReadDir(subdirPath)
		if err != nil {
			log.Fatalf("Couldn't read subdir [%v] contents: %v\n", subdirPath, err)
		}
		for _, f := range files {
			uploaders = append(uploaders, spawnUploader(toh, f.Name(), toh.cwd, subDir.Name()))
		}
	}
	return uploaders
}

func (toh *ThunkOutputHandler) spawnDownstreamThunk(thunkHash string, deps []string, outputFiles map[string][]string) string {
	db.DPrintf("Handler [%v] spawning [%v], depends on [%v]\n", toh.thunkHash, thunkHash, deps)
	exPid := spawnExecutor(toh, thunkHash, deps)
	return spawnThunkOutputHandler(toh, exPid, thunkHash, outputFiles[thunkHash])
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
			toh.primaryOutputThunk = outputHandlerPid(hashes[0])
			first = false
		}
	}
	return g.GetThunks()
}

func (toh *ThunkOutputHandler) readThunkOutput() []string {
	outputThunksPath := ggLocalBlobs(toh.thunkHash, toh.thunkHash+THUNK_OUTPUTS_SUFFIX)
	contents, err := ioutil.ReadFile(outputThunksPath)
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
	thunkOutputPath := ggLocalReductions(toh.thunkHash, toh.thunkHash)
	valueFile, err := ioutil.ReadFile(thunkOutputPath)
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
		out = append(out, outputHandlerPid(d))
	}
	return out
}
