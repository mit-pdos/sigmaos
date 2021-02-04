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
  newThunks, outputNames := toh.getOutputThunksAndFiles()
  if len(newThunks) == 0 {
    // We have produced a value, and need to propagate it down to functions which
    // depend on us.
    toh.propagateResultUpstream()
  } else {
    downstreamThunks := toh.spawnDownstreamThunks(newThunks, outputNames)
    for _, t := range downstreamThunks {
      toh.Wait(t)
    }
    db.DPrintf("Handler [%v] done waiting\n", toh.pid)
  }
}

func (toh *ThunkOutputHandler) spawnDownstreamThunks(newThunks map[string][]string, outputNames map[string][]string) []string {
  children := []string{}
  for thunkHash, deps := range newThunks {
    db.DPrintf("Handler [%v] spawning [%v], depends on [%v]\n", toh.thunkHash, thunkHash, deps)
    exPid := spawnExecutor(toh, thunkHash, deps)
    child := spawnThunkOutputHandler(toh, exPid, thunkHash, outputNames[thunkHash])
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

// TODO: make this more readable
func (toh *ThunkOutputHandler) getOutputThunksAndFiles() (map[string][]string, map[string][]string) {
  thunksAndDeps := make(map[string][]string)
  outputNames := make(map[string][]string)
  outputThunksPath := path.Join(
    GG_BLOB_DIR,
    toh.thunkHash + THUNK_OUTPUTS_SUFFIX,
  )
  f, err := toh.ReadFile(outputThunksPath)
  if err != nil {
    // XXX switch to db
    log.Fatalf("Error reading thunk outputs [%v]: %v\n", toh.thunkHash, err)
  }
  trimmed_f := strings.TrimSpace(string(f))
  if len(trimmed_f) > 0 {
    first := true
    // Read the thunk output file
    for _, line := range strings.Split(trimmed_f, "\n") {
      // Store output file name this thunk corresponds to
      result := strings.Split(strings.TrimSpace(line), "=")
      tag := result[0]
      // Store output thunk & its dependencies
      hashes := strings.Split(result[1], " ")
      for i, h := range hashes {
        if i > 0 {
          hashes[i] = h + OUTPUT_HANDLER_SUFFIX
        }
      }
      thunksAndDeps[hashes[0]] = hashes[1:]
      // XXX remove this sanity check
      if _, ok := outputNames[hashes[0]]; ok {
        log.Fatalf("output file was already in map when parsing thunk output")
      }
      outputNames[hashes[0]] = []string{ toh.thunkHash + "#" + tag }
      if first {
        first = false
        outputNames[hashes[0]] = append(outputNames[hashes[0]], toh.thunkHash)
      }
    }
  }
  return thunksAndDeps, outputNames
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
