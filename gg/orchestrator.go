package gg

import (
//  "errors"
//  "fmt"
  "strings"
  "log"
  "path"
  "io/ioutil"

  "ulambda/fslib"
  np "ulambda/ninep"
  db "ulambda/debug"
)

const (
  GG_TOP_DIR = "name/gg"
// XXX eventually make GG dirs configurable, both here & in GG
  GG_DIR = "name/fs/.gg"
  GG_BLOB_DIR = GG_DIR +  "/blobs"
//  GG_DIR = GG_TOP_DIR + ".gg"
  ORCHESTRATOR = GG_TOP_DIR +  "/orchestrator"
  UPLOAD_SUFFIX = ".upload"
  EXECUTOR_SUFFIX = ".executor"
  SHEBANG_DIRECTIVE = "#!/usr/bin/env gg-force-and-run"
)

type OrchestratorDev struct {
  orc *Orchestrator
}

func (orcdev *OrchestratorDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
//  t := string(data)
//  db.DPrintf("OrchestratorDev.write %v\n", t)
//  if strings.HasPrefix(t, "Join") {
//    orcdev.orc.join(t[len("Join "):])
//  } else if strings.HasPrefix(t, "Leave") {
//    orcdev.orc.leave(t[len("Leave"):])
//  } else if strings.HasPrefix(t, "Add") {
//    orcdev.orc.add()
//  } else if strings.HasPrefix(t, "Resume") {
//    orcdev.orc.resume(t[len("Resume "):])
//  } else {
//    return 0, fmt.Errorf("Write: unknown command %v\n", t)
//  }
  return np.Tsize(len(data)), nil
}

func (orcdev *OrchestratorDev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
  //  if off == 0 {
  //  s := orcdev.sd.ps()
  //return []byte(s), nil
  //}
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
  fls, err := fslib.InitFs(ORCHESTRATOR, &OrchestratorDev{orc})
  if err != nil {
    return nil, err
  }
  orc.FsLibSrv = fls
  db.SetDebug(debug)
  orc.Started(orc.pid)
  return orc, nil
}

func (orc *Orchestrator) Exit() {
  orc.Exiting(orc.pid)
}

func (orc *Orchestrator) Work() {
  orc.setUpDirs()
  executables := orc.getExecutableDependencies()
  exUpPids := orc.uploadExecutableDependencies(executables)
  for _, target := range orc.targets {
    // XXX handle non-thunk targets
    db.DPrintf("Spawning upload worker [%v]\n", target);
    targetHash := orc.getTargetHash(target)
    orc.targetHashes = append(orc.targetHashes, targetHash)
    upPids := []string{orc.spawnUploader(targetHash)}
    upPids = append(upPids, exUpPids...)
    orc.spawnExecutor(targetHash, upPids)
  }
}

func (orc *Orchestrator) uploadExecutableDependencies(execs []string) []string {
  pids := []string{}
  for _, exec := range execs {
    pids = append(pids, orc.spawnUploader(exec))
  }
  return pids
}

func (orc *Orchestrator) getExecutableDependencies() []string {
  execsPath := path.Join(orc.cwd, ".gg", "blobs", "executables.txt")
  f, err := ioutil.ReadFile(execsPath)
  if err != nil {
    log.Fatalf("Error reading exec dependencies: %v\n", err)
  }
  trimmed_f := strings.TrimRight(string(f), "\n")
  return strings.Split(trimmed_f, "\n")
}

func (orc *Orchestrator) getTargetHash(target string) string {
  // XXX support non-placeholders
  f, err := ioutil.ReadFile(path.Join(orc.cwd, target))
  contents := string(f)
  if err != nil {
    log.Fatalf("Error reading target [%v]: %v\n", target, err)
  }
  shebang := strings.Split(contents, "\n")[0]
  if shebang != SHEBANG_DIRECTIVE {
    log.Fatalf("Error: not a placeholder")
  }
  hash := strings.Split(contents, "\n")[1]
  return hash
}

func (orc *Orchestrator) mkdirOpt(path string) {
  _, err := orc.FsLib.Stat(path)
  if err != nil {
    db.DPrintf("Mkdir [%v]\n", path)
    // XXX Perms?
    err = orc.FsLib.Mkdir(path, np.DMDIR)
    if err != nil {
      log.Fatalf("Couldn't mkdir %v", GG_DIR)
    }
  } else {
    db.DPrintf("Already exists [%v]\n", path)
  }
}

func (orc *Orchestrator) setUpDirs() {
  orc.mkdirOpt(GG_DIR)
  orc.mkdirOpt(GG_BLOB_DIR)
}

func (orc *Orchestrator) spawnUploader(targetHash string) string {
  a := fslib.Attr{}
  a.Pid = targetHash + UPLOAD_SUFFIX
  a.Program = "./bin/fsuploader"
  a.Args = []string{
    path.Join(orc.cwd, ".gg", "blobs", targetHash),
    path.Join(GG_BLOB_DIR, targetHash),
  }
  a.Env = []string{}
  a.PairDep = []fslib.PDep{}
  a.ExitDep = nil
  err := orc.Spawn(&a)
  if err != nil {
    log.Fatalf("Error spawning upload worker [%v]: %v\n", targetHash, err);
  }
  return a.Pid
}

func (orc *Orchestrator) spawnExecutor(targetHash string, upPids []string) string {
  a := fslib.Attr{}
  a.Pid = targetHash + EXECUTOR_SUFFIX
  a.Program = "gg-execute" // XXX
  a.Args = []string{
    "--timelog",
    "--ninep",
    targetHash,
  }
  a.Env = []string{
    "GG_STORAGE_URI=9p://mnt/9p/fs GG_DIR=/mnt/9p/fs/.gg",
    "GG_DIR=/mnt/9p/fs/.gg", // XXX Make this configurable
    "GG_VERBOSE=1",
  }
  a.PairDep = []fslib.PDep{}
  a.ExitDep = upPids
  err := orc.Spawn(&a)
  if err != nil {
    log.Fatalf("Error spawning upload worker [%v]: %v\n", targetHash, err);
  }
  return a.Pid
}

//echo $SPID,"gg-execute","[--timelog --ninep ${hashes[@]}]","[GG_STORAGE_URI=9p://mnt/9p/fs GG_DIR=/mnt/9p/fs/.gg]","",""
