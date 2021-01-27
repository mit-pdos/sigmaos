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
  *fslib.FsLib
}

func MakeThunkOutputHandler(args []string, debug bool) (*ThunkOutputHandler, error) {
  log.Printf("ThunkOutputHandler: %v\n", args)
  toh := &ThunkOutputHandler{}

  toh.pid = args[0]
  toh.thunkHash = args[1]
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
  newThunkHashes := toh.getOutputThunks()
  for _, thunkHash := range newThunkHashes {
    exPid := spawnExecutor(toh, thunkHash, []string{})
    spawnThunkOutputHandler(toh, exPid, thunkHash)
  }
}

func (toh *ThunkOutputHandler) getOutputThunks() []string {
  outputThunksPath := path.Join(
    GG_BLOB_DIR,
    toh.thunkHash + THUNK_OUTPUTS_SUFFIX,
  )
  f, err := toh.ReadFile(outputThunksPath)
  if err != nil {
    log.Fatalf("Error reading thunk outputs [%v]: %v\n", toh.thunkHash, err)
  }
  trimmed_f := strings.TrimSpace(string(f))
  return strings.Split(trimmed_f, "\n")
}
