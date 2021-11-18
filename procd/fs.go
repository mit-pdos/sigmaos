package procd

import (
	"encoding/json"
	"log"

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
	pd.fs.root, pd.FsServer, pd.FsLib, err = fslibsrv.MakeMemFs(named.PROCD, "procd")
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
	return nil
}

func (pfs *ProcdFs) pubClaimed(p *proc.Proc) error {
	err := pfs.runq.Remove(fssrv.MkCtx(""), p.Pid)
	if err != nil {
		log.Printf("Error ProcdFs.pubClaimed: %v", err)
		return err
	}
	return nil
}
