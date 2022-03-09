package procd

import (
	"encoding/json"
	"fmt"
	"log"
	"math"

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

type readRunqFn func(procdPath string, queueName string) ([]*np.Stat, error)
type readProcFn func(procdPath string, queueName string, pid string) (*proc.Proc, error)
type claimProcFn func(procdPath string, queueName string, p *proc.Proc) bool

type ProcdFs struct {
	pd      *Procd
	run     fs.Dir
	runqs   map[string]fs.Dir
	ctlFile fs.FsObj
}

// Set up this procd instance's FS
func (pd *Procd) makeFs() {
	pd.fs = &ProcdFs{}
	pd.fs.pd = pd
	var err error
	pd.MemFs, pd.FsLib, pd.procclnt, err = fslibsrv.MakeMemFs(np.PROCD, np.PROCDREL)
	if err != nil {
		log.Fatalf("FATAL %v: MakeMemFs %v\n", proc.GetProgram(), err)
	}
	procclnt.MountPids(pd.FsLib, fslib.Named())

	// Set up ctl file
	pd.fs.ctlFile = makeCtlFile(pd, nil, pd.Root())
	err1 := dir.MkNod(ctx.MkCtx("", 0, nil), pd.Root(), np.PROC_CTL_FILE, pd.fs.ctlFile)
	if err1 != nil {
		log.Fatalf("FATAL Error MkNod in RunProcd: %v", err1)
	}

	// Set up running dir
	runningi := inode.MakeInode(nil, np.DMDIR, pd.Root())
	running := dir.MakeDir(runningi)
	err1 = dir.MkNod(ctx.MkCtx("", 0, nil), pd.Root(), np.PROCD_RUNNING, running)
	if err1 != nil {
		log.Fatalf("FATAL Error creating running dir: %v", err1)
	}
	pd.fs.run = running

	// Set up runq dir
	pd.fs.runqs = make(map[string]fs.Dir)
	runqs := []string{np.PROCD_RUNQ_LC, np.PROCD_RUNQ_BE}
	for _, q := range runqs {
		runqi := inode.MakeInode(nil, np.DMDIR, pd.Root())
		runq := dir.MakeDir(runqi)
		err1 = dir.MkNod(ctx.MkCtx("", 0, nil), pd.Root(), q, runq)
		if err1 != nil {
			log.Fatalf("FATAL Error creating running dir: %v", err1)
		}
		pd.fs.runqs[q] = runq
	}

	// Set up pids dir
	pidsi := inode.MakeInode(nil, np.DMDIR, pd.Root())
	pids := dir.MakeDir(pidsi)
	err1 = dir.MkNod(ctx.MkCtx("", 0, nil), pd.Root(), proc.PIDS, pids)
	if err1 != nil {
		log.Fatalf("FATAL Error creating pids dir: %v", err1)
	}
}

func (pfs *ProcdFs) readRunq(procdPath string, queueName string) ([]*np.Stat, error) {
	rq, err := pfs.runqs[queueName].ReadDir(ctx.MkCtx("", 0, nil), 0, 0, np.NoV)
	if err != nil {
		log.Printf("Error ReadDir in ProcdFs.readRunq: %v", err)
		return nil, err
	}
	return rq, nil
}

func (pfs *ProcdFs) readRunqProc(procdPath string, queueName string, name string) (*proc.Proc, error) {
	os, _, err := pfs.runqs[queueName].Lookup(ctx.MkCtx("", 0, nil), []string{name})
	if err != nil {
		log.Printf("Error Lookup in ProcdFs.getRunqProc: %v", err)
		return nil, err
	}
	b, err := os[0].(fs.File).Read(ctx.MkCtx("", 0, nil), 0, math.MaxUint32, np.NoV)
	if err != nil {
		log.Printf("Error Read in ProcdFs.getRunqProc: %v", err)
		return nil, err
	}
	p := proc.MakeEmptyProc()
	error := json.Unmarshal(b, p)
	if error != nil {
		log.Fatalf("FATAL Error Unmarshal in ProcdFs.getRunqProc: %v", err)
		return nil, np.MkErrError(error)
	}
	return p, nil
}

// Remove from the runq. May race with other (work-stealing) procds.
func (pfs *ProcdFs) claimProc(procdPath string, queueName string, p *proc.Proc) bool {
	err := pfs.runqs[queueName].Remove(ctx.MkCtx("", 0, nil), p.Pid)
	if err != nil {
		db.DLPrintf("PDFS", "Error ProcdFs.claimProc: %v", err)
		return false
	}
	return true
}

// Publishes a proc as running
func (pfs *ProcdFs) running(p *Proc) *np.Err {
	p.FsObj = inode.MakeInode(nil, np.DMDEVICE, pfs.run)
	f := memfs.MakeFile(p.FsObj)
	// Make sure we write the proc description before we publish it.
	b, error := json.Marshal(p.attr)
	if error != nil {
		return np.MkErrError(fmt.Errorf("running marshal err %v", error))
	}
	_, err := f.Write(ctx.MkCtx("", 0, nil), 0, b, np.NoV)
	if err != nil {
		return err
	}
	err = dir.MkNod(ctx.MkCtx("", 0, nil), pfs.run, p.Pid, p)
	if err != nil {
		log.Printf("Error ProcdFs.run: %v", err)
		return err
	}
	return nil
}

// Publishes a proc as done running
func (pfs *ProcdFs) finish(p *Proc) error {
	err := pfs.run.Remove(ctx.MkCtx("", 0, nil), p.Pid)
	if err != nil {
		log.Printf("Error ProcdFs.finish: %v", err)
		return err
	}
	return nil
}

// Publishes a proc as spawned (NOT running yet)
func (pfs *ProcdFs) spawn(a *proc.Proc, b []byte) error {
	var runq fs.Dir
	switch {
	case a.Ncore > 0:
		runq = pfs.runqs[np.PROCD_RUNQ_LC]
	default:
		runq = pfs.runqs[np.PROCD_RUNQ_BE]
	}
	ino := inode.MakeInode(nil, 0, runq)
	f := memfs.MakeFile(ino)
	// Make sure we write the proc description before we publish it.
	f.Write(ctx.MkCtx("", 0, nil), 0, b, np.NoV)
	err := dir.MkNod(ctx.MkCtx("", 0, nil), runq, a.Pid, f)
	if err != nil {
		log.Printf("Error ProcdFs.spawn: %v", err)
		return err
	}
	pfs.pd.spawnChan <- true
	return nil
}
