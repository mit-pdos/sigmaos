package procd

import (
	"encoding/json"
	"log"
	"math"

	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fs"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/inode"
	"ulambda/memfs"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/proc"
)

type readRunqFn func(procdPath string, queueName string) ([]*np.Stat, error)
type readProcFn func(procdPath string, queueName string, pid string) (*proc.Proc, error)
type claimProcFn func(procdPath string, queueName string, p *proc.Proc) bool

type ProcdFs struct {
	pd      *Procd
	root    fs.Dir
	run     fs.Dir
	runqs   map[string]fs.Dir
	ctlFile fs.FsObj
}

// Set up this procd instance's FS
func (pd *Procd) makeFs() {
	var err error
	pd.fs = &ProcdFs{}
	pd.fs.pd = pd
	pd.fs.root, pd.fsrv, pd.FsLib, pd.procclnt, err = fslibsrv.MakeMemFs(named.PROCD, named.PROCDDIR)
	if err != nil {
		log.Fatalf("MakeSrvFsLib %v\n", err)
	}

	// Set up ctl file
	pd.fs.ctlFile = makeCtlFile(pd, "", pd.fs.root)
	err = dir.MkNod(fssrv.MkCtx(""), pd.fs.root, named.PROC_CTL_FILE, pd.fs.ctlFile)
	if err != nil {
		log.Fatalf("Error MkNod in RunProcd: %v", err)
	}

	// Set up running dir
	runningi := inode.MakeInode("", np.DMDIR, pd.fs.root)
	running := dir.MakeDir(runningi)
	err = dir.MkNod(fssrv.MkCtx(""), pd.fs.root, named.PROCD_RUNNING, running)
	if err != nil {
		log.Fatalf("Error creating running dir: %v", err)
	}
	pd.fs.run = running

	// Set up runq dir
	pd.fs.runqs = make(map[string]fs.Dir)
	runqs := []string{named.PROCD_RUNQ_LC, named.PROCD_RUNQ_BE}
	for _, q := range runqs {
		runqi := inode.MakeInode("", np.DMDIR, pd.fs.root)
		runq := dir.MakeDir(runqi)
		err = dir.MkNod(fssrv.MkCtx(""), pd.fs.root, q, runq)
		if err != nil {
			log.Fatalf("Error creating running dir: %v", err)
		}
		pd.fs.runqs[q] = runq
	}
}

func (pfs *ProcdFs) readRunq(procdPath string, queueName string) ([]*np.Stat, error) {
	rq, err := pfs.runqs[queueName].ReadDir(fssrv.MkCtx(""), 0, 0, np.NoV)
	if err != nil {
		log.Printf("Error ReadDir in ProcdFs.readRunq: %v", err)
		return nil, err
	}
	return rq, nil
}

func (pfs *ProcdFs) readRunqProc(procdPath string, queueName string, name string) (*proc.Proc, error) {
	os, _, err := pfs.runqs[queueName].Lookup(fssrv.MkCtx(""), []string{name})
	if err != nil {
		log.Printf("Error Lookup in ProcdFs.getRunqProc: %v", err)
		return nil, err
	}
	b, err := os[0].(fs.File).Read(fssrv.MkCtx(""), 0, math.MaxUint32, np.NoV)
	if err != nil {
		log.Printf("Error Read in ProcdFs.getRunqProc: %v", err)
		return nil, err
	}
	p := proc.MakeEmptyProc()
	err = json.Unmarshal(b, p)
	if err != nil {
		log.Fatalf("Error Unmarshal in ProcdFs.getRunqProc: %v", err)
		return nil, err
	}
	return p, nil
}

// Remove from the runq. May race with other (work-stealing) procds.
func (pfs *ProcdFs) claimProc(procdPath string, queueName string, p *proc.Proc) bool {
	err := pfs.runqs[queueName].Remove(fssrv.MkCtx(""), p.Pid)
	if err != nil {
		db.DLPrintf("PDFS", "Error ProcdFs.claimProc: %v", err)
		return false
	}
	return true
}

// Publishes a proc as running
func (pfs *ProcdFs) running(p *Proc) error {
	p.FsObj = inode.MakeInode("", np.DMDEVICE, pfs.run)
	f := memfs.MakeFile(p.FsObj)
	// Make sure we write the proc description before we publish it.
	b, err := json.Marshal(p.attr)
	if err != nil {
		log.Fatalf("Error Marshalling proc in ProcdFs.running: %v", err)
	}
	f.Write(fssrv.MkCtx(""), 0, b, np.NoV)
	err = dir.MkNod(fssrv.MkCtx(""), pfs.run, p.Pid, p)
	if err != nil {
		log.Printf("Error ProcdFs.run: %v", err)
		return err
	}
	return nil
}

// Publishes a proc as done running (NOT running yet)
func (pfs *ProcdFs) finish(p *Proc) error {
	err := pfs.run.Remove(fssrv.MkCtx(""), p.Pid)
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
		runq = pfs.runqs[named.PROCD_RUNQ_LC]
	default:
		runq = pfs.runqs[named.PROCD_RUNQ_BE]
	}
	ino := inode.MakeInode("", 0, runq)
	f := memfs.MakeFile(ino)
	// Make sure we write the proc description before we publish it.
	f.Write(fssrv.MkCtx(""), 0, b, np.NoV)
	err := dir.MkNod(fssrv.MkCtx(""), runq, a.Pid, f)
	if err != nil {
		log.Printf("Error ProcdFs.spawn: %v", err)
		return err
	}
	pfs.pd.spawnChan <- true
	return nil
}
