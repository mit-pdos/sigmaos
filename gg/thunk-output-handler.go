package gg

import (
//  "errors"
//  "fmt"
  "strings"
  "log"
  "path"

  "ulambda/fslib"
//  np "ulambda/ninep"
  db "ulambda/debug"
)

type ThunkOutputHandler struct {
  pid          string
  thunkHash    string
  outputFiles  []string
  *fslib.FsLib
}

func MakeThunkOutputHandler(args []string, debug bool) (*ThunkOutputHandler, error) {
  db.DPrintf("ThunkOutputHandler: %v\n", args)
  toh := &ThunkOutputHandler{}

  toh.pid = args[0]
  toh.thunkHash = args[1]
  toh.outputFiles = args[2:]
  fls := fslib.MakeFsLib("gg-thunk-output-handler")
  toh.FsLib = fls
  db.SetDebug(debug)
  toh.Started(toh.pid)
  return toh, nil
}

func (toh *ThunkOutputHandler) Exit() {
  toh.Exiting(toh.pid)
}

// XXX Check cache
func (toh *ThunkOutputHandler) Work() {
  // Read the thunk output file
  thunkOutput := toh.readThunkOutput()
  newThunks := toh.getNewThunks(thunkOutput)
  outputFiles := toh.getOutputFiles(thunkOutput)
  if len(newThunks) == 0 {
    // We have produced a value, and need to propagate it down to functions which
    // depend on us.
    toh.propagateResultUpstream()
  } else {
    downstreamThunks := toh.spawnDownstreamThunks(newThunks, outputFiles)
    for _, t := range downstreamThunks {
      toh.Wait(t)
    }
    db.DPrintf("Handler [%v] done waiting\n", toh.pid)
  }
}

func (toh *ThunkOutputHandler) spawnDownstreamThunks(newThunks map[string][]string, outputFiles map[string][]string) []string {
  children := []string{}
  for thunkHash, deps := range newThunks {
    db.DPrintf("Handler [%v] spawning [%v], depends on [%v]\n", toh.thunkHash, thunkHash, deps)
    exPid := spawnExecutor(toh, thunkHash, deps)
    child := spawnThunkOutputHandler(toh, exPid, thunkHash, outputFiles[thunkHash])
    children = append(children, child)
  }
  return children
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
    // XXX remove this sanity check
    if _, ok := outputFiles[hash]; ok {
      log.Fatalf("output file was already in map when parsing thunk output")
    }
    outputFiles[hash] = []string{ toh.thunkHash + "#" + tag }
    if first {
      first = false
      outputFiles[hash] = append(outputFiles[hash], toh.thunkHash)
    }
  }
  return outputFiles
}

// TODO: make this more readable
func (toh *ThunkOutputHandler) getNewThunks(thunkOutput []string) map[string][]string {
  // Maps of new thunks to their dependencies
  newThunks := make(map[string][]string)
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
  }
  return newThunks
}

func (toh *ThunkOutputHandler) readThunkOutput() []string {
  outputThunksPath := path.Join(
    GG_BLOB_DIR,
    toh.thunkHash + THUNK_OUTPUTS_SUFFIX,
  )
  contents, err := toh.ReadFile(outputThunksPath)
  if err != nil {
    // XXX switch to db
    log.Fatalf("Error reading thunk outputs [%v]: %v\n", toh.thunkHash, err)
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
