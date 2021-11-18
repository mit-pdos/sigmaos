package procd

import (
	"log"

	"ulambda/dir"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/inode"
	"ulambda/named"
	np "ulambda/ninep"
)

// Set up this procd instance's FS
func (pd *Procd) makeFs() {
	var err error
	pd.fs = &ProcdFs{}
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
