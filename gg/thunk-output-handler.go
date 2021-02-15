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
		GG_LOCAL_ENV_BASE,
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
		for newThunk, thunkDeps := range newThunks {
			inputDependencies := getInputDependencies(toh, newThunk)
			log.Printf("Got input dependencies %v for [%v]\n", inputDependencies, newThunk)
			// XXX A dirty hack.. should definitely do something more principled
			oldCwd := toh.cwd
			toh.cwd = path.Join(GG_LOCAL_ENV_BASE, newThunk)
			// XXX Waiting for all uploaders is overly conservative... perhaps not necessary
			downloaders := spawnInputDownloaders(toh, newThunk, inputDependencies, uploaders)
			toh.cwd = oldCwd
			exitDeps := []string{}
			exitDeps = append(exitDeps, thunkDeps...)
			exitDeps = append(exitDeps, uploaders...)
			exitDeps = append(exitDeps, downloaders...)
			toh.spawnDownstreamThunk(newThunk, exitDeps, outputFiles)
		}
		exitDepSwaps := []string{
			toh.pid,
			toh.primaryOutputThunk,
		}
		db.DPrintf("Updating exit dependencies for [%v]\n", toh.pid)
		err := toh.SwapExitDependencies(exitDepSwaps)
		if err != nil {
			log.Fatal("Couldn't swap exit dependencies\n")
		}
	}
}

func (toh *ThunkOutputHandler) spawnResultUploaders() []string {
	uploaders := []string{}
	subDirs, err := ioutil.ReadDir(path.Join(toh.cwd, ".gg"))
	if err != nil {
		log.Fatalf("Couldn't read local dir [%v] contents: %v\n", toh.cwd, err)
	}

	// Upload contents of each subdir (blobs, reductions, hash_cache) to 9P remote
	// server
	for _, subDir := range subDirs {
		subdirPath := path.Join(toh.cwd, ".gg", subDir.Name())
		files, err := ioutil.ReadDir(subdirPath)
		if err != nil {
			log.Fatalf("Couldn't read subdir [%v] contents: %v\n", subdirPath, err)
		}
		for _, f := range files {
			uploaders = append(uploaders, spawnUploader(toh, f.Name(), subDir.Name()))
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
		outputPath := path.Join(
			GG_REDUCTION_DIR,
			outputFile,
		)
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

func (toh *ThunkOutputHandler) getNewThunks(thunkOutput []string) map[string][]string {
	// Maps of new thunks to their dependencies
	newThunks := make(map[string][]string)
	first := true
	for _, line := range thunkOutput {
		thunkLine := strings.Split(strings.TrimSpace(line), "=")[1]
		// Compute new thunk's dependencies
		hashes := strings.Split(thunkLine, " ")
		for i, h := range hashes {
			// Thunk actually depends on the output handler of its dependency
			if i > 0 {
				hashes[i] = h + OUTPUT_HANDLER_SUFFIX
			}
		}
		newThunks[hashes[0]] = hashes[1:]
		if first {
			toh.primaryOutputThunk = hashes[0] + OUTPUT_HANDLER_SUFFIX
			first = false
		}
	}
	return newThunks
}

func (toh *ThunkOutputHandler) readThunkOutput() []string {
	outputThunksPath := path.Join(
		toh.cwd,
		".gg",
		"blobs",
		toh.thunkHash+THUNK_OUTPUTS_SUFFIX,
	)
	contents, err := ioutil.ReadFile(outputThunksPath)
	if err != nil {
		// XXX switch to db
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
	filePath := path.Join(
		GG_BLOB_DIR,
		toh.getReduction(),
	)
	contents, err := toh.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Error reading value file[%v]: %v\n", filePath, err)
	}
	return strings.TrimSpace(string(contents))
}

func (toh *ThunkOutputHandler) getReduction() string {
	thunkOutputPath := path.Join(
		GG_REDUCTION_DIR,
		toh.thunkHash,
	)
	valueFile, err := toh.ReadFile(thunkOutputPath)
	if err != nil {
		log.Fatalf("Error reading thunk outputs [%v]: %v\n", toh.thunkHash, err)
	}
	return strings.TrimSpace(string(valueFile))
}

func (toh *ThunkOutputHandler) getCwd() string {
	return toh.cwd
}
