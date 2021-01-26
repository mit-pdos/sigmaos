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
  ORCHESTRATOR = "name/gg/orchestrator"
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
  pid string
  targets []string
  *fslib.FsLibSrv
}

func MakeOrchestrator(args []string, debug bool) (*Orchestrator, error) {
  log.Printf("Orchestrator: %v\n", args)
  orc := &Orchestrator{}
  orc.targets = args[1:]
  orc.pid = args[0]
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
  db.DPrintf("Orchestrator working!\n");
}
