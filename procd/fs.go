package procd

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/memfssrv"
	np "sigmaos/ninep"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/resource"
)

type ProcdFs struct {
	pd        *Procd
	run       fs.Dir
	spawnFile fs.Inode
	ctlFile   fs.Inode
}

// Set up this procd instance's FS
func (pd *Procd) makeFs() {
	pd.fs = &ProcdFs{}
	pd.fs.pd = pd
	var err error
	pd.MemFs, pd.FsLib, pd.procclnt, err = memfssrv.MakeMemFs(np.PROCD, np.PROCDREL)
	if err != nil {
		db.DFatalf("%v: MakeMemFs %v\n", proc.GetProgram(), err)
	}
	procclnt.MountPids(pd.FsLib, fslib.Named())

	// Set up spawn file
	pd.fs.spawnFile = makeSpawnFile(pd, nil, pd.Root())
	err1 := dir.MkNod(ctx.MkCtx("", 0, nil), pd.Root(), np.PROCD_SPAWN_FILE, pd.fs.spawnFile)
	if err1 != nil {
		db.DFatalf("Error MkNod in RunProcd: %v", err1)
	}

	// Set up ctl file
	resource.MakeCtlFile(pd.addCores, pd.removeCores, pd.Root(), np.RESOURCE_CTL)

	// Set up runq dir
	dirs := []string{np.PROCD_RUNQ_LC, np.PROCD_RUNQ_BE, np.PROCD_RUNNING, proc.PIDS}
	for _, d := range dirs {
		if err := pd.MkDir(path.Join(np.PROCD, pd.MyAddr(), d), 0777); err != nil {
			db.DFatalf("Error creating dir: %v", err1)
		}
	}
}

// Publishes a proc as running
func (pfs *ProcdFs) running(p *LinuxProc) *np.Err {
	// Make sure we write the proc description before we publish it.
	b, error := json.Marshal(p.attr)
	if error != nil {
		return np.MkErrError(fmt.Errorf("running marshal err %v", error))
	}
	_, err := pfs.pd.PutFile(path.Join(np.PROCD, pfs.pd.MyAddr(), np.PROCD_RUNNING, p.attr.Pid.String()), 0777, np.OREAD|np.OWRITE, b)
	if err != nil {
		db.DFatalf("Error ProcdFs.spawn: %v", err)
		// TODO: return an np.Err return err
	}
	return nil
}

// Publishes a proc as done running
func (pfs *ProcdFs) finish(p *LinuxProc) error {
	err := pfs.pd.Remove(path.Join(np.PROCD, pfs.pd.MyAddr(), np.PROCD_RUNNING, p.attr.Pid.String()))
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
	case a.Ncore > 0:
		runq = np.PROCD_RUNQ_LC
	default:
		runq = np.PROCD_RUNQ_BE
	}
	_, err := pfs.pd.PutFile(path.Join(np.PROCD, pfs.pd.MyAddr(), runq, a.Pid.String()), 0777, np.OREAD|np.OWRITE, b)
	if err != nil {
		log.Printf("Error ProcdFs.spawn: %v", err)
		return err
	}
	pfs.pd.spawnProc(a)
	return nil
}
