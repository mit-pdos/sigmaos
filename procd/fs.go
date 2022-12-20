package procd

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/memfssrv"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/resource"
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
	pd.memfssrv, pd.FsLib, pd.procclnt, err = memfssrv.MakeMemFs(sp.PROCD, sp.PROCDREL)
	if err != nil {
		db.DFatalf("%v: MakeMemFs %v\n", proc.GetProgram(), err)
	}
	procclnt.MountPids(pd.FsLib, fslib.Named())

	// Set up spawn file
	if err := makeSpawnFile(pd); err != nil {
		db.DFatalf("Error MkDev in RunProcd: %v", err)
	}

	// Set up ctl file
	resource.MakeCtlFile(pd.addCores, pd.removeCores, pd.memfssrv.Root(), sp.RESOURCE_CTL)

	// Set up runq dir
	dirs := []string{sp.PROCD_RUNQ_LC, sp.PROCD_RUNQ_BE, sp.PROCD_RUNNING, proc.PIDS}
	for _, d := range dirs {
		if err := pd.MkDir(path.Join(sp.PROCD, pd.memfssrv.MyAddr(), d), 0777); err != nil {
			db.DFatalf("Error creating dir: %v", err)
		}
	}
}

// Publishes a proc as running
func (pfs *ProcdFs) running(p *LinuxProc) *serr.Err {
	// Make sure we write the proc description before we publish it.
	b, error := json.Marshal(p.attr)
	if error != nil {
		return serr.MkErrError(fmt.Errorf("running marshal err %v", error))
	}
	_, err := pfs.pd.PutFile(path.Join(sp.PROCD, pfs.pd.memfssrv.MyAddr(), sp.PROCD_RUNNING, p.attr.Pid.String()), 0777, sp.OREAD|sp.OWRITE, b)
	if err != nil {
		pfs.pd.perf.Done()
		db.DFatalf("Error ProcdFs.spawn: %v", err)
		// TODO: return an serr.Err return err
	}
	return nil
}

// Publishes a proc as done running
func (pfs *ProcdFs) finish(p *LinuxProc) error {
	err := pfs.pd.Remove(path.Join(sp.PROCD, pfs.pd.memfssrv.MyAddr(), sp.PROCD_RUNNING, p.attr.Pid.String()))
	if err != nil {
		log.Printf("%v: Error ProcdFs.finish: %v", proc.GetName(), err)
		return err
	}
	return nil
}

// Publishes a proc as spawned (NOT running yet)
func (pfs *ProcdFs) spawn(a *proc.Proc, b []byte) error {
	var runq string
	switch {
	case a.Type == proc.T_LC:
		runq = sp.PROCD_RUNQ_LC
	default:
		runq = sp.PROCD_RUNQ_BE
	}
	// This procd will likely claim this proc, so cache it.
	pfs.pd.pcache.Set(a.Pid, a)
	_, err := pfs.pd.PutFile(path.Join(sp.PROCD, pfs.pd.memfssrv.MyAddr(), runq, a.Pid.String()), 0777, sp.OREAD|sp.OWRITE, b)
	if err != nil {
		log.Printf("Error ProcdFs.spawn: %v", err)
		return err
	}
	db.DPrintf(db.PROCD, "Procd created q file %v", path.Join(sp.PROCD, pfs.pd.memfssrv.MyAddr(), runq, a.Pid.String()))
	pfs.pd.spawnProc(a)
	return nil
}
