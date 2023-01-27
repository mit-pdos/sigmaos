package procd

import (
	"encoding/json"
	"fmt"
	"path"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type ProcdFs struct {
	pd      *Procd
	run     fs.Dir
	ctlFile fs.Inode
}

// Set up this procd instance's FS
func (pd *Procd) makeFs() {
	pd.fs = &ProcdFs{}
	pd.fs.pd = pd
	var err error
	pd.memfssrv, pd.sc, err = memfssrv.MakeMemFs(sp.PROCD, sp.PROCDREL)
	if err != nil {
		db.DFatalf("%v: MakeMemFs %v\n", proc.GetProgram(), err)
	}
	procclnt.MountPids(pd.sc.FsLib, pd.sc.NamedAddr())

	// Set up runq dir
	dirs := []string{sp.PROCD_RUNNING, proc.PIDS}
	for _, d := range dirs {
		if err := pd.sc.MkDir(path.Join(sp.PROCD, pd.memfssrv.MyAddr(), d), 0777); err != nil {
			db.DFatalf("Error creating dir: %v", err)
		}
	}
}

// Publishes a proc as running
func (pfs *ProcdFs) running(p *LinuxProc) *serr.Err {
	// For convenience.
	b, error := json.Marshal(p.attr)
	if error != nil {
		return serr.MkErrError(fmt.Errorf("running marshal err %v", error))
	}
	_, err := pfs.pd.sc.PutFile(path.Join(sp.PROCD, pfs.pd.memfssrv.MyAddr(), sp.PROCD_RUNNING, p.attr.GetPid().String()), 0777, sp.OREAD|sp.OWRITE, b)
	if err != nil {
		pfs.pd.perf.Done()
		db.DFatalf("Error ProcdFs.spawn: %v", err)
	}
	return nil
}

// Publishes a proc as done running
func (pfs *ProcdFs) finish(p *LinuxProc) error {
	err := pfs.pd.sc.Remove(path.Join(sp.PROCD, pfs.pd.memfssrv.MyAddr(), sp.PROCD_RUNNING, p.attr.GetPid().String()))
	if err != nil {
		db.DFatalf("Error ProcdFs.finish: %v", err)
		return err
	}
	return nil
}
