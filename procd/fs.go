package procd

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"ulambda/ctx"
	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fslib"
	"ulambda/fslibsrv"
	"ulambda/inode"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/procclnt"
)

type ProcdFs struct {
	pd      *Procd
	run     fs.Dir
	runqs   map[string]fs.Dir
	ctlFile fs.Inode
}

// Set up this procd instance's FS
func (pd *Procd) makeFs() {
	pd.fs = &ProcdFs{}
	pd.fs.pd = pd
	var err error
	pd.MemFs, pd.FsLib, pd.procclnt, err = fslibsrv.MakeMemFs(np.PROCD, np.PROCDREL)
	if err != nil {
		db.DFatalf("%v: MakeMemFs %v\n", proc.GetProgram(), err)
	}
	procclnt.MountPids(pd.FsLib, fslib.Named())

	// Set up ctl file
	pd.fs.ctlFile = makeCtlFile(pd, nil, pd.Root())
	err1 := dir.MkNod(ctx.MkCtx("", 0, nil), pd.Root(), np.PROC_CTL_FILE, pd.fs.ctlFile)
	if err1 != nil {
		db.DFatalf("Error MkNod in RunProcd: %v", err1)
	}

	// Set up running dir
	runningi := inode.MakeInode(nil, np.DMDIR, pd.Root())
	running := dir.MakeDir(runningi, memfs.MakeInode)
	err1 = dir.MkNod(ctx.MkCtx("", 0, nil), pd.Root(), np.PROCD_RUNNING, running)
	if err1 != nil {
		db.DFatalf("Error creating running dir: %v", err1)
	}
	pd.fs.run = running

	// Set up runq dir
	pd.fs.runqs = make(map[string]fs.Dir)
	runqs := []string{np.PROCD_RUNQ_LC, np.PROCD_RUNQ_BE}
	for _, q := range runqs {
		runqi := inode.MakeInode(nil, np.DMDIR, pd.Root())
		runq := dir.MakeDir(runqi, memfs.MakeInode)
		err1 = dir.MkNod(ctx.MkCtx("", 0, nil), pd.Root(), q, runq)
		if err1 != nil {
			db.DFatalf("Error creating running dir: %v", err1)
		}
		pd.fs.runqs[q] = runq
	}

	// Set up pids dir
	pidsi := inode.MakeInode(nil, np.DMDIR, pd.Root())
	pids := dir.MakeDir(pidsi, memfs.MakeInode)
	err1 = dir.MkNod(ctx.MkCtx("", 0, nil), pd.Root(), proc.PIDS, pids)
	if err1 != nil {
		db.DFatalf("Error creating pids dir: %v", err1)
	}
}

// Publishes a proc as running
func (pfs *ProcdFs) running(p *Proc) *np.Err {
	// Make sure we write the proc description before we publish it.
	b, error := json.Marshal(p.attr)
	if error != nil {
		return np.MkErrError(fmt.Errorf("running marshal err %v", error))
	}
	_, err := pfs.pd.PutFile(path.Join(np.PROCD, pfs.pd.MyAddr(), np.PROCD_RUNNING, p.Pid.String()), 0777, np.OREAD|np.OWRITE, b)
	if err != nil {
		db.DFatalf("Error ProcdFs.spawn: %v", err)
		// TODO: return an np.Err return err
	}
	return nil
}

// Publishes a proc as done running
func (pfs *ProcdFs) finish(p *Proc) error {
	err := pfs.pd.Remove(path.Join(np.PROCD, pfs.pd.MyAddr(), np.PROCD_RUNNING, p.Pid.String()))
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
