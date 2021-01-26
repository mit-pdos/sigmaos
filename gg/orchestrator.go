package gg

import (
//  "errors"
//  "strings"
//  "fmt"
  "log"

  "ulambda/fslib"
  np "ulambda/ninep"
  db "ulambda/debug"
)

const (
  GG_TOP_DIR = "name/gg"
// XXX eventually make GG dirs configurable, both here & in GG
  GG_DIR = "name/fs/.gg"
  GG_BLOB_DIR = GG_DIR + "/blobs"
//  GG_DIR = GG_TOP_DIR + ".gg"
  ORCHESTRATOR = GG_TOP_DIR + "/orchestrator"
  UPLOAD_SUFFIX = ".upload"
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
  pid     string
  cwd     string
  targets []string
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
  for _, target := range orc.targets {
    db.DPrintf("Spawning upload worker [%v]\n", target);
    err := orc.spawnUploader(target)
    if err != nil {
      db.DPrintf("Error spawning upload worker [%v]: %v\n", target, err);
    }
  }
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
  }
}

func (orc *Orchestrator) setUpDirs() {
  orc.mkdirOpt(GG_DIR)
  orc.mkdirOpt(GG_BLOB_DIR)
}

func (orc *Orchestrator) spawnUploader(target string) error {
  a := fslib.Attr{}
  a.Pid = target + UPLOAD_SUFFIX
  a.Program = "./bin/fsuploader"
  a.Args = []string{orc.cwd + "/" + target, GG_BLOB_DIR + "/" + target}
  a.PairDep = []fslib.PDep{}
  a.ExitDep = nil
  return orc.Spawn(&a)
}
