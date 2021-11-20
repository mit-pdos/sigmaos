package procd

import (
	"encoding/json"
	"log"
	"math"

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

type ProcdFs struct {
	pd      *Procd
	root    fs.Dir
	running fs.Dir
	runq    fs.Dir
	ctlFile fs.FsObj
}

// Set up this procd instance's FS
func (pd *Procd) makeFs() {
	var err error
	pd.fs = &ProcdFs{}
	pd.fs.pd = pd
	pd.fs.root, pd.FsServer, pd.FsLib, err = fslibsrv.MakeMemFs(named.PROCD, named.PROCDDIR)
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
	pd.fs.running = running

	// Set up runq dir
	runqi := inode.MakeInode("", np.DMDIR, pd.fs.root)
	runq := dir.MakeDir(runqi)
	err = dir.MkNod(fssrv.MkCtx(""), pd.fs.root, named.PROCD_RUNQ, runq)
	if err != nil {
		log.Fatalf("Error creating running dir: %v", err)
	}
	pd.fs.runq = runq
}

func (pfs *ProcdFs) readRunq(procdPath string) ([]*np.Stat, error) {
	rq, err := pfs.runq.ReadDir(fssrv.MkCtx(""), 0, 0, np.NoV)
	if err != nil {
		log.Fatalf("Error ReadDir in Procd.getProc: %v", err)
		return nil, err
	}
	return rq, nil
}

func (pfs *ProcdFs) readRunqProc(procdPath string, name string) (*proc.Proc, error) {
	os, _, err := pfs.runq.Lookup(fssrv.MkCtx(""), []string{name})
	if err != nil {
		log.Fatalf("Error Lookup in ProcdFs.getRunqProc: %v", err)
		return nil, err
	}
	b, err := os[0].(fs.File).Read(fssrv.MkCtx(""), 0, math.MaxUint32, np.NoV)
	if err != nil {
		log.Fatalf("Error Read in ProcdFs.getRunqProc: %v", err)
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
func (pfs *ProcdFs) claimProc(procdPath string, p *proc.Proc) bool {
	err := pfs.runq.Remove(fssrv.MkCtx(""), p.Pid)
	if err != nil {
		log.Printf("Error ProcdFs.claimProc: %v", err)
		return false
	}
	return true
}

// Publishes a proc as running
func (pfs *ProcdFs) pubRunning(p *Proc) error {
	p.FsObj = inode.MakeInode("", np.DMDEVICE, pfs.running)
	f := memfs.MakeFile(p.FsObj)
	// Make sure we write the proc description before we publish it.
	b, err := json.Marshal(p.attr)
	if err != nil {
		log.Fatalf("Error Marshalling proc in ProcdFs.pubRunning: %v", err)
	}
	f.Write(fssrv.MkCtx(""), 0, b, np.NoV)
	err = dir.MkNod(fssrv.MkCtx(""), pfs.running, p.Pid, p)
	if err != nil {
		log.Printf("Error ProcdFs.pubRunning: %v", err)
		return err
	}
	return nil
}

// Publishes a proc as done running (NOT running yet)
func (pfs *ProcdFs) pubFinished(p *Proc) error {
	err := pfs.running.Remove(fssrv.MkCtx(""), p.Pid)
	if err != nil {
		log.Printf("Error ProcdFs.pubFinished: %v", err)
		return err
	}
	return nil
}

// Publishes a proc as spawned (NOT running yet)
func (pfs *ProcdFs) pubSpawned(a *proc.Proc, b []byte) error {
	ino := inode.MakeInode("", 0, pfs.runq)
	f := memfs.MakeFile(ino)
	// Make sure we write the proc description before we publish it.
	f.Write(fssrv.MkCtx(""), 0, b, np.NoV)
	err := dir.MkNod(fssrv.MkCtx(""), pfs.runq, a.Pid, f)
	if err != nil {
		log.Printf("Error ProcdFs.pubSpawned: %v", err)
		return err
	}
	pfs.pd.spawnChan <- true
	return nil
}
