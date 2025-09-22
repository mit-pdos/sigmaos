package srv

import (
	//"archive/zip"
	//"compress/flate"
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	db "sigmaos/debug"
	"sigmaos/fs"
	lazypagessrv "sigmaos/lazypages/srv"
	"sigmaos/proc"
	"sigmaos/sched/msched/proc/proto"
	"sigmaos/scontainer"
	sp "sigmaos/sigmap"

	criu "github.com/checkpoint-restore/go-criu/v7"
	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/zip"
	"golang.org/x/sys/unix"
)

const (
	DUMPDIR    = "/home/sigmaos/dump-"
	RESTOREDIR = "/home/sigmaos/restore-"

	CKPTLAZY = "ckptlazy"
	CKPTFULL = "ckptfull"
)

func (ps *ProcSrv) Checkpoint(ctx fs.CtxI, req proto.CheckpointProcRequest, res *proto.CheckpointProcResponse) error {
	db.DPrintf(db.CKPT, "Checkpointing uproc %v %q", req.PidStr, req.PathName)
	defer db.DPrintf(db.CKPT, "Checkpoint done uproc %v %q", req.PidStr, req.PathName)
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
	pe.mu.Lock()
	pe.checkpointStatus = 1
	pe.mu.Unlock()
	imgDir := DUMPDIR + spid.String()
	db.DPrintf(db.CKPT, "making %v,%v", imgDir, os.ModePerm)
	err := os.MkdirAll(imgDir, os.ModePerm)
	if err != nil {
		db.DPrintf(db.CKPT, "CheckpointProc: error creating %v err %v", imgDir, err)
		pe.mu.Lock()
		pe.checkpointStatus = 2
		pe.mu.Unlock()
		return err
	}
	if err := scontainer.CheckpointProc(ps.criuclnt, pid, imgDir, spid, pe.ino); err != nil {
		pe.mu.Lock()
		pe.checkpointStatus = 2
		pe.mu.Unlock()
		return err
	}
	if err := ps.writeCheckpoint(imgDir, req.PathName, CKPTFULL); err != nil {
		pe.mu.Lock()
		pe.checkpointStatus = 2
		pe.mu.Unlock()
		db.DPrintf(db.PROCD, "writeCheckpoint full %v err\n", spid, err)
		return err
	}
	if err := ps.writeCheckpointLazy(imgDir, req.PathName, CKPTLAZY); err != nil {
		pe.mu.Lock()
		pe.checkpointStatus = 2
		pe.mu.Unlock()
		db.DPrintf(db.PROCD, "writeCheckpoint lazy %v err %v\n", spid, err)
		return err
	}

	pe.mu.Lock()
	pe.checkpointStatus = 0
	pe.mu.Unlock()
	return nil
}
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

const bufferSize = 1024 * 1024

// FileHeader represents metadata for each file in the archive
type FileHeader struct {
	NameLength uint32
	FileSize   uint64
	// Followed by filename (variable length) in the archive
}

// unlike writing full checkpoint, writeCheckpointLazy does not fill out the memory image (pages.img) and also compresses the metadata
func (ps *ProcSrv) writeCheckpointLazy(chkptLocalDir string, chkptSimgaDir string, ckpt string) error {
	ps.ssrv.MemFs.SigmaClnt().MkDir(chkptSimgaDir, 0777)
	pn := filepath.Join(chkptSimgaDir, ckpt)
	db.DPrintf(db.PROCD, "writeCheckpoint: create dir: %v\n", pn)
	if err := ps.ssrv.MemFs.SigmaClnt().MkDir(pn, 0777); err != nil {
		return err
	}

	destZip := filepath.Join(chkptLocalDir, "dump.zip")
	zipFile, err := os.Create(destZip)
	if err != nil {
		return err
	}

	zipWriter := zip.NewWriter(zipFile)
	zipWriter.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression) // Best compression, slower but smaller
	})
	files, err := os.ReadDir(chkptLocalDir)
	if err != nil {
		db.DPrintf(db.PROCD, "writeCheckpoint: reading local dir err %\n", err)
		return err
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "criu.log") || strings.HasPrefix(file.Name(), "stats-") {
			continue
		}
		if strings.HasPrefix(file.Name(), "dump") {
			continue
		}
		if ckpt == CKPTLAZY && strings.HasPrefix(file.Name(), "pages-") {
			continue
		}

		filePath := chkptLocalDir + "/" + file.Name()
		db.DPrintf(db.PROCD, "compressing %v err %v\n", file.Name(), err)
		srcFile, err := os.Open(filePath)
		if err != nil {
			return err
		}
		if strings.HasPrefix(file.Name(), "pagemap") || strings.HasPrefix(file.Name(), "mm-") {
			if err := ps.ssrv.MemFs.SigmaClnt().UploadFile(filepath.Join(chkptLocalDir, file.Name()), filepath.Join(pn, file.Name())); err != nil {
				db.DPrintf(db.PROCD, "UploadFile %v err %v\n", file.Name(), err)
				return err
			}
			continue
		}

		w, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:   file.Name(),
			Method: zip.Deflate, // DEFLATE with best compression
		})
		if err != nil {
			return err
		}

		_, err = io.Copy(w, srcFile)
		if err != nil {
			return err
		}

	}

	zipWriter.Close()
	zipFile.Close()
	if err := ps.ssrv.MemFs.SigmaClnt().UploadFile(destZip, filepath.Join(pn, "dump.zip")); err != nil {
		db.DPrintf(db.PROCD, "UploadFile %v err %v\n", destZip, err)
		return err
	}

	db.DPrintf(db.PROCD, "writeCheckpoint: copied %d files %s", len(files), filepath.Join(pn, "dump.zip"))
	return nil
}

// formate is <lazypagesid>pid.fifo
func (ps *ProcSrv) getRestoredPid(proc *proc.Proc, fifoDir string, lazypagesid int) error {
	fifo := filepath.Join(fifoDir, strconv.Itoa(lazypagesid)+"pid.fifo")

	// Clean up any stale FIFO from prior runs (ignore errors).
	_ = os.Remove(fifo)

	// Create the FIFO: mode 0666 so other processes/users can write.
	if err := unix.Mkfifo(fifo, 0666); err != nil {
		db.DPrintf(db.ERROR, "mkfifo: %v", err)
	}
	db.DPrintf(db.CKPT, "pid pipe created at %v", fifo)

	// Open read-only; this blocks until someone opens for write.
	f, err := os.OpenFile(fifo, os.O_RDONLY, 0)
	if err != nil {
		db.DPrintf(db.ERROR, "open read: %v", err)
	}
	defer f.Close()

	// Read text lines until writer closes (EOF.
	sc := bufio.NewScanner(f)
	if sc.Scan() {
		db.DPrintf(db.CKPT, "received restore proc pid: %v", sc.Text())
		pid, err := strconv.Atoi(sc.Text())
		if err != nil {
			db.DPrintf(db.ERROR, "pid pipe format error: %v", err)

		}
		pe, alloc := ps.procs.Alloc(pid, newProcEntry(proc))
		if !alloc { // it was already inserted
			pe.insertSignal(proc)
		}

	} else {
		db.DPrintf(db.ERROR, "read error: %v", err)
	}
	return nil
}
func (ps *ProcSrv) restoreProc(proc *proc.Proc) error {
	dst := RESTOREDIR + proc.GetPid().String()
	ckptSigmaDir := proc.GetCheckpointLocation()
	lazypagesid := rand.Intn(100000) // pick a large enough range to avoid collisions
	if err := ps.readCheckpointAndRegister(ckptSigmaDir, dst, CKPTLAZY, 1, lazypagesid); err != nil {
		db.DPrintf(db.CKPT, "LZ4 WRONG")
		return nil
	}
	go ps.getRestoredPid(proc, ps.lpc.WorkDir(), lazypagesid)
	criuclnt := criu.MakeCriu()
	criuclnt.SetCriuPath("/criu/criu/criu")
	// XXX delete dst dir when done
	if err := scontainer.RestoreProc(criuclnt, proc, filepath.Join(dst, CKPTLAZY), ps.lpc.WorkDir(), lazypagesid); err != nil {
		return err
	}
	return nil
}

// Reads checkpoint from S3 and registers the proc with lazypagesrv
func (ps *ProcSrv) readCheckpointAndRegister(ckptSigmaDir, localDir, ckpt string, pid int, lazypagesid int) error {
	db.DPrintf(db.CKPT, "readCheckpoint %v %v %v", ckptSigmaDir, localDir, ckpt)

	os.Mkdir(localDir, 0755)
	pn := filepath.Join(localDir, ckpt)
	dstfn := filepath.Join(pn, "dump.zip")
	if err := os.Mkdir(pn, 0755); err != nil {
		return err
	}

	sts, err := ps.ssrv.MemFs.SigmaClnt().GetDir(filepath.Join(ckptSigmaDir, ckpt))
	if err != nil {
		return err
	}
	files := sp.Names(sts)
	firstInstance := true

	for _, entry := range files {
		//preloads file is working set from a past restore
		//If it exists in the checkpoint dir, we need to use it to prefetch the working set
		if strings.HasPrefix(entry, "preloads") {
			firstInstance = false

		}
		if strings.HasPrefix(entry, "pagemap") || strings.HasPrefix(entry, "mm-") || strings.HasPrefix(entry, "preloads") {
			fn := filepath.Join(ckptSigmaDir, ckpt, entry)
			dstfn := filepath.Join(pn, entry)
			if err := ps.ssrv.MemFs.SigmaClnt().DownloadFile(fn, dstfn); err != nil {
				return err
			}
		}

	}
	//Register in parallel with reading in the rest of the metadata
	go func() {
		pages := filepath.Join(ckptSigmaDir, CKPTFULL, "pages-"+strconv.Itoa(pid)+".img")

		if err := ps.lpc.Register(lazypagesid, pn, pages, filepath.Join(ckptSigmaDir, ckpt), firstInstance); err != nil {
			return
		}
		db.DPrintf(db.CKPT, "restoreProc: Registered %d %v", pid, pages)
	}()

	fn := filepath.Join(ckptSigmaDir, ckpt, "dump.zip")
	if err := ps.ssrv.MemFs.SigmaClnt().DownloadFile(fn, dstfn); err != nil {
		db.DPrintf(db.PROCD, "DownloadFile %v err %v\n", fn, err)
		return err
	}
	r, err := zip.OpenReader(dstfn)
	if err != nil {
		db.DPrintf(db.PROCD, "openzip %v err %v\n", fn, err)
		return err
	}
	defer r.Close()
	db.DPrintf(db.CKPT, "decompressing %v\n", dstfn)
	// Loop through each file in the archive
	for _, f := range r.File {
		fPath := filepath.Join(pn, f.Name)

		// Open file inside zip
		srcFile, err := f.Open()
		if err != nil {
			db.DPrintf(db.CKPT, "decompressing err %v\n", err)
			return err
		}
		defer srcFile.Close()

		// Create destination file
		dstFile, err := os.Create(fPath)
		if err != nil {
			db.DPrintf(db.CKPT, "destination err %v\n", err)
			dstFile.Close()
			return err
		}

		// Copy contents
		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			db.DPrintf(db.CKPT, "copy err %v\n", err)
			dstFile.Close()
			return err
		}
		dstFile.Close()
	}
	db.DPrintf(db.CKPT, "decompressed %v\n", dstfn)
	if ckpt == CKPTLAZY {
		db.DPrintf(db.CKPT, "Expand %s\n", pn)
		if err := lazypagessrv.ExpandLazyPages(pn); err != nil {
			return err
		}
	}
	db.DPrintf(db.CKPT, "Done readCheckpoint%s\n", pn)

	return nil
}
