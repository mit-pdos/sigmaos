package uprocsrv

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/lazypagessrv"
	"sigmaos/proc"
	"sigmaos/scontainer"
	sp "sigmaos/sigmap"
	"sigmaos/uprocsrv/proto"
)

const (
	DUMPDIR    = "/home/sigmaos/dump-"
	RESTOREDIR = "/home/sigmaos/restore-"

	CKPTLAZY = "ckptlazy"
	CKPTFULL = "ckptfull"
)

func (ups *UprocSrv) Checkpoint(ctx fs.CtxI, req proto.CheckpointProcRequest, res *proto.CheckpointProcResponse) error {
	db.DPrintf(db.UPROCD, "Checkpointing uproc %v %q", req.PidStr, req.PathName)
	spid := sp.Tpid(req.PidStr)
	pid, ok := ups.pids.Lookup(spid)
	if !ok {
		db.DPrintf(db.UPROCD, "Checkpoint no pid for %v\n", spid)
		return fmt.Errorf("no proc %v\n", spid)
	}
	pe, ok := ups.procs.Lookup(pid)
	if !ok {
		db.DPrintf(db.UPROCD, "Checkpoint no proc for %v\n", pid)
		return fmt.Errorf("no proc %v\n", spid)
	}
	imgDir := DUMPDIR + spid.String()
	err := os.MkdirAll(imgDir, os.ModePerm)
	if err != nil {
		db.DPrintf(db.CKPT, "CheckpointProc: error creating %v err %v", imgDir, err)
		return err
	}
	if err := scontainer.CheckpointProc(ups.criuclnt, pid, imgDir, spid, pe.ino); err != nil {
		return err
	}
	if err := ups.writeCheckpoint(imgDir, req.PathName, CKPTFULL); err != nil {
		db.DPrintf(db.UPROCD, "writeCheckpoint full %v\n", spid, err)
		return err
	}
	if err := ups.writeCheckpoint(imgDir, req.PathName, CKPTLAZY); err != nil {
		db.DPrintf(db.UPROCD, "writeCheckpoint lazy %v err %v\n", spid, err)
		return err
	}
	return nil
}

// Copy the checkpoint img. Depending on <ckpt> name, copy only "pagesnonlazy-<n>.img"
func (ups *UprocSrv) writeCheckpoint(chkptLocalDir string, chkptSimgaDir string, ckpt string) error {
	ups.ssrv.MemFs.SigmaClnt().MkDir(chkptSimgaDir, 0777)
	pn := filepath.Join(chkptSimgaDir, ckpt)
	db.DPrintf(db.UPROCD, "writeCheckpoint: create dir: %v\n", pn)
	if err := ups.ssrv.MemFs.SigmaClnt().MkDir(pn, 0777); err != nil {
		return err
	}
	files, err := os.ReadDir(chkptLocalDir)
	if err != nil {
		db.DPrintf(db.UPROCD, "writeCheckpoint: reading local dir err %\n", err)
		return err
	}
	for _, file := range files {
		if ckpt == CKPTLAZY && strings.HasPrefix(file.Name(), "pages-") {
			continue
		}
		if err := ups.ssrv.MemFs.SigmaClnt().UploadFile(filepath.Join(chkptLocalDir, file.Name()), filepath.Join(pn, file.Name())); err != nil {
			db.DPrintf(db.UPROCD, "UploadFile %v err %v\n", file.Name(), err)
			return err
		}
	}
	db.DPrintf(db.UPROCD, "writeCheckpoint: copied %d files", len(files))
	return nil
}

func (ups *UprocSrv) restoreProc(proc *proc.Proc) error {
	dst := RESTOREDIR + proc.GetPid().String()
	ckptSigmaDir := proc.GetCheckpointLocation()
	if err := ups.readCheckpoint(ckptSigmaDir, dst, CKPTLAZY); err != nil {
		return nil
	}
	imgdir := filepath.Join(dst, CKPTLAZY)
	ps, err := lazypagessrv.NewTpstree(imgdir)
	if err != nil {
		return nil
	}
	pid := ps.RootPid()
	pages := filepath.Join(ckptSigmaDir, CKPTFULL, "pages-"+strconv.Itoa(pid)+".img")
	go func() {
		db.DPrintf(db.CKPT, "restoreProc: Register %d %v", pid, pages)
		if err := ups.lpc.Register(pid, imgdir, pages); err != nil {
			db.DPrintf(db.CKPT, "restoreProc: Register %d %v err %v", pid, pages, err)
			return
		}
	}()
	// XXX delete dst dir when done
	if err := scontainer.RestoreProc(ups.criuclnt, proc, filepath.Join(dst, CKPTLAZY), ups.lpc.WorkDir()); err != nil {
		return err
	}
	return nil
}

func (ups *UprocSrv) readCheckpoint(ckptSigmaDir, localDir, ckpt string) error {
	db.DPrintf(db.CKPT, "readCheckpoint %v %v %v", ckptSigmaDir, localDir, ckpt)

	os.Mkdir(localDir, 0755)
	pn := filepath.Join(localDir, ckpt)
	if err := os.Mkdir(pn, 0755); err != nil {
		return err
	}

	sts, err := ups.ssrv.MemFs.SigmaClnt().GetDir(filepath.Join(ckptSigmaDir, ckpt))
	if err != nil {
		db.DPrintf("GetDir %v err %v\n", ckptSigmaDir, err)
		return err
	}
	files := sp.Names(sts)
	db.DPrintf(db.UPROCD, "Copy file %v to %s\n", files, pn)
	for _, entry := range files {
		fn := filepath.Join(ckptSigmaDir, ckpt, entry)
		dstfn := filepath.Join(pn, entry)
		if err := ups.ssrv.MemFs.SigmaClnt().DownloadFile(fn, dstfn); err != nil {
			db.DPrintf("DownloadFile %v err %v\n", fn, err)
			return err
		}
	}
	if ckpt == CKPTLAZY {
		db.DPrintf(db.CKPT, "Expand %s\n", pn)
		if err := lazypagessrv.ExpandLazyPages(pn); err != nil {
			return err
		}
	}
	return nil
}
