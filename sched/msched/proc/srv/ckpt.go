package srv

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
	"sigmaos/sched/msched/proc/proto"
	"sigmaos/scontainer"
	sp "sigmaos/sigmap"
)

const (
	DUMPDIR    = "/home/sigmaos/dump-"
	RESTOREDIR = "/home/sigmaos/restore-"

	CKPTLAZY = "ckptlazy"
	CKPTFULL = "ckptfull"
)

func (ps *ProcSrv) Checkpoint(ctx fs.CtxI, req proto.CheckpointProcRequest, res *proto.CheckpointProcResponse) error {
	db.DPrintf(db.PROCD, "Checkpointing uproc %v %q", req.PidStr, req.PathName)
	spid := sp.Tpid(req.PidStr)
	pid, ok := ps.pids.Lookup(spid)
	if !ok {
		db.DPrintf(db.PROCD, "Checkpoint no pid for %v\n", spid)
		return fmt.Errorf("no proc %v\n", spid)
	}
	pe, ok := ps.procs.Lookup(pid)
	if !ok {
		db.DPrintf(db.PROCD, "Checkpoint no proc for %v\n", pid)
		return fmt.Errorf("no proc %v\n", spid)
	}
	imgDir := DUMPDIR + spid.String()
	err := os.MkdirAll(imgDir, os.ModePerm)
	if err != nil {
		db.DPrintf(db.CKPT, "CheckpointProc: error creating %v err %v", imgDir, err)
		return err
	}
	if err := scontainer.CheckpointProc(ps.criuclnt, pid, imgDir, spid, pe.ino); err != nil {
		return err
	}
	if err := ps.writeCheckpoint(imgDir, req.PathName, CKPTFULL); err != nil {
		db.DPrintf(db.PROCD, "writeCheckpoint full %v\n", spid, err)
		return err
	}
	if err := ps.writeCheckpoint(imgDir, req.PathName, CKPTLAZY); err != nil {
		db.DPrintf(db.PROCD, "writeCheckpoint lazy %v err %v\n", spid, err)
		return err
	}
	return nil
}

// Copy the checkpoint img. Depending on <ckpt> name, copy only "pagesnonlazy-<n>.img"
func (ps *ProcSrv) writeCheckpoint(chkptLocalDir string, chkptSimgaDir string, ckpt string) error {
	ps.ssrv.MemFs.SigmaClnt().MkDir(chkptSimgaDir, 0777)
	pn := filepath.Join(chkptSimgaDir, ckpt)
	db.DPrintf(db.PROCD, "writeCheckpoint: create dir: %v\n", pn)
	if err := ps.ssrv.MemFs.SigmaClnt().MkDir(pn, 0777); err != nil {
		return err
	}
	files, err := os.ReadDir(chkptLocalDir)
	if err != nil {
		db.DPrintf(db.PROCD, "writeCheckpoint: reading local dir err %\n", err)
		return err
	}
	for _, file := range files {
		if ckpt == CKPTLAZY && strings.HasPrefix(file.Name(), "pages-") {
			continue
		}
		if err := ps.ssrv.MemFs.SigmaClnt().UploadFile(filepath.Join(chkptLocalDir, file.Name()), filepath.Join(pn, file.Name())); err != nil {
			db.DPrintf(db.PROCD, "UploadFile %v err %v\n", file.Name(), err)
			return err
		}
	}
	db.DPrintf(db.PROCD, "writeCheckpoint: copied %d files", len(files))
	return nil
}

func (ps *ProcSrv) restoreProc(proc *proc.Proc) error {
	dst := RESTOREDIR + proc.GetPid().String()
	ckptSigmaDir := proc.GetCheckpointLocation()
	if err := ps.readCheckpoint(ckptSigmaDir, dst, CKPTLAZY); err != nil {
		return nil
	}
	imgdir := filepath.Join(dst, CKPTLAZY)
	pst, err := lazypagessrv.NewTpstree(imgdir)
	if err != nil {
		return nil
	}
	pid := pst.RootPid()
	pages := filepath.Join(ckptSigmaDir, CKPTFULL, "pages-"+strconv.Itoa(pid)+".img")
	go func() {
		db.DPrintf(db.CKPT, "restoreProc: Register %d %v", pid, pages)
		if err := ps.lpc.Register(pid, imgdir, pages); err != nil {
			db.DPrintf(db.CKPT, "restoreProc: Register %d %v err %v", pid, pages, err)
			return
		}
	}()
	// XXX delete dst dir when done
	if err := scontainer.RestoreProc(ps.criuclnt, proc, filepath.Join(dst, CKPTLAZY), ps.lpc.WorkDir()); err != nil {
		return err
	}
	return nil
}

func (ps *ProcSrv) readCheckpoint(ckptSigmaDir, localDir, ckpt string) error {
	db.DPrintf(db.CKPT, "readCheckpoint %v %v %v", ckptSigmaDir, localDir, ckpt)

	os.Mkdir(localDir, 0755)
	pn := filepath.Join(localDir, ckpt)
	if err := os.Mkdir(pn, 0755); err != nil {
		return err
	}

	sts, err := ps.ssrv.MemFs.SigmaClnt().GetDir(filepath.Join(ckptSigmaDir, ckpt))
	if err != nil {
		db.DPrintf("GetDir %v err %v\n", ckptSigmaDir, err)
		return err
	}
	files := sp.Names(sts)
	db.DPrintf(db.PROCD, "Copy file %v to %s\n", files, pn)
	for _, entry := range files {
		fn := filepath.Join(ckptSigmaDir, ckpt, entry)
		dstfn := filepath.Join(pn, entry)
		if err := ps.ssrv.MemFs.SigmaClnt().DownloadFile(fn, dstfn); err != nil {
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
