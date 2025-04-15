package srv

import (
	//"archive/zip"
	//"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
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

	//"github.com/pierrec/lz4/v4"
	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/zip"
	"github.com/pierrec/lz4/v4"
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
		return err
		pe.mu.Lock()
		pe.checkpointStatus = 2
		pe.mu.Unlock()
	}
	if err := ps.writeCheckpoint(imgDir, req.PathName, CKPTFULL); err != nil {
		pe.mu.Lock()
		pe.checkpointStatus = 2
		pe.mu.Unlock()
		db.DPrintf(db.PROCD, "writeCheckpoint full %v\n", spid, err)
		return err
	}
	if err := ps.writeCheckpoint2(imgDir, req.PathName, CKPTLAZY); err != nil {
		//if err := ps.writeCheckpointlz4(imgDir, req.PathName, CKPTLAZY); err != nil {
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
		//db.DPrintf(db.PROCD, "UploadingFile %v\n", file.Name())
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

func (ps *ProcSrv) writeCheckpointlz4(chkptLocalDir string, chkptSimgaDir string, ckpt string) error {
	ps.ssrv.MemFs.SigmaClnt().MkDir(chkptSimgaDir, 0777)
	pn := filepath.Join(chkptSimgaDir, ckpt)
	db.DPrintf(db.PROCD, "writeCheckpoint: create dir: %v\n", pn)
	if err := ps.ssrv.MemFs.SigmaClnt().MkDir(pn, 0777); err != nil {
		return err
	}

	//destRegZip := filepath.Join(chkptLocalDir, "dumpreg.zip")
	//zipRegFile, err := os.Create(destRegZip)
	//if err != nil {
	//		return err
	//}

	//zipRegWriter := zip.NewWriter(zipRegFile)

	destlz4 := filepath.Join(chkptLocalDir, "dump.lz4")
	lz4File, err := os.Create(destlz4)
	if err != nil {
		return err
	}
	lz4Writer := lz4.NewWriter(lz4File)
	defer lz4Writer.Close()

	// Set compression level
	//lz4.CompressionLevelOption(lz4.Level9)(lz4Writer)

	files, err := os.ReadDir(chkptLocalDir)
	if err != nil {
		db.DPrintf(db.PROCD, "writeCheckpoint: reading local dir err %\n", err)
		return err
	}
	buffer := make([]byte, bufferSize)

	// Track archive stats
	var totalOriginalSize int64
	var filesAdded int
	for _, file := range files {
		if file.Name() == "dump.lz4" {
			continue
		}
		if ckpt == CKPTLAZY && strings.HasPrefix(file.Name(), "pages-") {
			continue
		}

		filePath := chkptLocalDir + "/" + file.Name()
		db.DPrintf(db.PROCD, "compressing %v err %v\n", file.Name(), err)
		if strings.HasPrefix(file.Name(), "pagemap") || strings.HasPrefix(file.Name(), "mm-") {
			if err := ps.ssrv.MemFs.SigmaClnt().UploadFile(filepath.Join(chkptLocalDir, file.Name()), filepath.Join(pn, file.Name())); err != nil {
				db.DPrintf(db.PROCD, "UploadFile %v err %v\n", file.Name(), err)
				return err
			}
			continue
		}
		inFile, err := os.Open(filePath)
		if err != nil {
			db.DPrintf(db.CKPT, "Warning: skipping %s: %v\n", filePath, err)
			continue
		}

		// Get file info for size
		fileInfo, err := inFile.Stat()
		if err != nil {
			inFile.Close()
			db.DPrintf(db.CKPT, "Warning: skipping %s: %v\n", filePath, err)
			continue
		}

		// Use relative path for storage
		relPath := file.Name()

		// Write file header
		nameBytes := []byte(relPath)
		header := FileHeader{
			NameLength: uint32(len(nameBytes)),
			FileSize:   uint64(fileInfo.Size()),
		}

		// Write header fields
		if err := binary.Write(lz4Writer, binary.LittleEndian, header.NameLength); err != nil {
			inFile.Close()
			return fmt.Errorf("failed to write header for %s: %w", file.Name(), err)
		}
		if err := binary.Write(lz4Writer, binary.LittleEndian, header.FileSize); err != nil {
			inFile.Close()
			return fmt.Errorf("failed to write header for %s: %w", file.Name(), err)
		}

		// Write file name
		if _, err := lz4Writer.Write(nameBytes); err != nil {
			inFile.Close()
			return fmt.Errorf("failed to write filename for %s: %w", file.Name(), err)
		}

		// Copy file contents
		var fileSize int64
		for {
			n, err := inFile.Read(buffer)
			if err != nil && err != io.EOF {
				inFile.Close()
				return fmt.Errorf("error reading from %s: %w", file.Name(), err)
			}
			if n == 0 {
				break
			}

			if _, err := lz4Writer.Write(buffer[:n]); err != nil {
				inFile.Close()
				return fmt.Errorf("error compressing data from %s: %w", file.Name(), err)
			}

			fileSize += int64(n)
		}

		inFile.Close()
		totalOriginalSize += fileSize
		filesAdded++
		db.DPrintf(db.CKPT, "Added %s (%.2f MB)\n", file.Name(), float64(fileSize)/1024/1024)
	}

	// Write end-of-archive marker (zero-length filename)
	endMarker := uint32(0)
	if err := binary.Write(lz4Writer, binary.LittleEndian, endMarker); err != nil {
		return fmt.Errorf("failed to write end-of-archive marker: %w", err)
	}

	// Flush the writer to ensure all data is written
	if err := lz4Writer.Close(); err != nil {
		return fmt.Errorf("error finalizing archive: %w", err)
	}

	// Get final compressed size
	finalInfo, err := os.Stat(destlz4)
	if err != nil {
		fmt.Println("Warning: couldn't get final archive size")
	} else {
		compressionRatio := float64(totalOriginalSize) / float64(finalInfo.Size())
		db.DPrintf(db.CKPT,
			"Archive created: %s (%.2f KB) containing %d files\n",
			destlz4,
			float64(finalInfo.Size())/1024,
			filesAdded,
		)
		db.DPrintf(db.CKPT,
			"Original size: %.2f KB, Compression ratio: %.2fx\n",
			float64(totalOriginalSize)/1024,
			compressionRatio,
		)
	}
	if err := ps.ssrv.MemFs.SigmaClnt().UploadFile(destlz4, filepath.Join(pn, "dump.lz4")); err != nil {
		db.DPrintf(db.PROCD, "UploadFile %v err %v\n", destlz4, err)
		return err
	}

	db.DPrintf(db.PROCD, "writeCheckpoint: copied %d files %s", len(files), filepath.Join(pn, "dump.lz4"))
	return nil
}

// Copy the checkpoint img. Depending on <ckpt> name, copy only "pagesnonlazy-<n>.img"
func (ps *ProcSrv) writeCheckpoint2(chkptLocalDir string, chkptSimgaDir string, ckpt string) error {
	ps.ssrv.MemFs.SigmaClnt().MkDir(chkptSimgaDir, 0777)
	pn := filepath.Join(chkptSimgaDir, ckpt)
	db.DPrintf(db.PROCD, "writeCheckpoint: create dir: %v\n", pn)
	if err := ps.ssrv.MemFs.SigmaClnt().MkDir(pn, 0777); err != nil {
		return err
	}

	//destRegZip := filepath.Join(chkptLocalDir, "dumpreg.zip")
	//zipRegFile, err := os.Create(destRegZip)
	//if err != nil {
	//		return err
	//}

	//zipRegWriter := zip.NewWriter(zipRegFile)

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

		// if strings.HasPrefix(file.Name(), "pagemap") || strings.HasPrefix(file.Name(), "mm-") {
		// 	w, err := zipRegWriter.Create(file.Name())
		// 	if err != nil {
		// 		return err
		// 	}

		// 	_, err = io.Copy(w, srcFile)
		// 	if err != nil {
		// 		return err
		// 	}
		// 	continue
		// }
		//w, err := zipWriter.Create(file.Name())
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
		//db.DPrintf(db.PROCD, "UploadingFile %v\n", file.Name())
		//if err := ps.ssrv.MemFs.SigmaClnt().UploadFile(filepath.Join(chkptLocalDir, file.Name()), filepath.Join(pn, file.Name())); err != nil {
		//	db.DPrintf(db.PROCD, "UploadFile %v err %v\n", file.Name(), err)
		//	return err
		//}
	}
	// zipRegWriter.Close()
	// zipRegFile.Close()
	// if err := ps.ssrv.MemFs.SigmaClnt().UploadFile(destRegZip, filepath.Join(pn, "dumpreg.zip")); err != nil {
	// 	db.DPrintf(db.PROCD, "UploadFile %v err %v\n", destRegZip, err)
	// 	return err
	// }

	zipWriter.Close()
	zipFile.Close()
	if err := ps.ssrv.MemFs.SigmaClnt().UploadFile(destZip, filepath.Join(pn, "dump.zip")); err != nil {
		db.DPrintf(db.PROCD, "UploadFile %v err %v\n", destZip, err)
		return err
	}

	db.DPrintf(db.PROCD, "writeCheckpoint: copied %d files %s", len(files), filepath.Join(pn, "dump.zip"))
	return nil
}

func (ps *ProcSrv) restoreProc(proc *proc.Proc) error {
	dst := RESTOREDIR + proc.GetPid().String()
	ckptSigmaDir := proc.GetCheckpointLocation()
	//assumePID is 1
	//if err := ps.readCheckpointAndRegisterLz4(ckptSigmaDir, dst, CKPTLAZY, 1); err != nil {
	if err := ps.readCheckpointAndRegister(ckptSigmaDir, dst, CKPTLAZY, 1); err != nil {
		db.DPrintf(db.CKPT, "LZ4 WRONG")
		return nil
	}
	// imgdir := filepath.Join(dst, CKPTLAZY)
	// pst, err := lazypagessrv.NewTpstree(imgdir)
	// if err != nil {
	// 	return nil
	// }
	// pid := pst.RootPid()
	//pages := filepath.Join(ckptSigmaDir, CKPTFULL, "pages-"+strconv.Itoa(pid)+".img")
	// Why is this a separate Goroutine?
	//	go func() {
	//	db.DPrintf(db.CKPT, "restoreProc: Register %d %v", pid, pages)
	//	if err := ps.lpc.Register(pid, imgdir, pages); err != nil {
	//		db.DPrintf(db.CKPT, "restoreProc: Register %d %v err %v", pid, pages, err)
	//	return
	//	}
	//	db.DPrintf(db.CKPT, "restoreProc: Registered %d %v", pid, pages)
	//	}()
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
	dstfn := filepath.Join(pn, "dump.zip")
	if err := os.Mkdir(pn, 0755); err != nil {
		return err
	}

	sts, err := ps.ssrv.MemFs.SigmaClnt().GetDir(filepath.Join(ckptSigmaDir, ckpt))
	// if err != nil {
	// 	db.DPrintf("GetDir %v err %v\n", ckptSigmaDir, err)
	// 	return err
	// }
	files := sp.Names(sts)
	for _, entry := range files {
		db.DPrintf(db.CKPT, "SEE %s\n", entry)
	}
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
	db.DPrintf(db.PROCD, "len %v\n", len(r.File))
	// Loop through each file in the archive
	for _, f := range r.File {
		db.DPrintf(db.CKPT, "uncompress %s\n", f.Name)
		fPath := filepath.Join(pn, f.Name)

		// Open file inside zip
		srcFile, err := f.Open()
		if err != nil {
			return err
		}
		defer srcFile.Close()

		// Create destination file
		dstFile, err := os.Create(fPath)
		if err != nil {
			dstFile.Close()
			return err
		}

		// Copy contents
		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			dstFile.Close()
			return err
		}
		dstFile.Close()
	}

	// for _, entry := range files {
	// 	db.DPrintf(db.CKPT, "copying %s\n", entry)
	// 	fn := filepath.Join(ckptSigmaDir, ckpt, entry)
	// 	dstfn := filepath.Join(pn, entry)
	// 	if err := ps.ssrv.MemFs.SigmaClnt().DownloadFile(fn, dstfn); err != nil {
	// 		db.DPrintf(db.PROCD, "DownloadFile %v err %v\n", fn, err)
	// 		return err
	// 	}
	// }
	//	db.DPrintf(db.CKPT, "Almost done %s\n", pn)
	if ckpt == CKPTLAZY {
		db.DPrintf(db.CKPT, "Expand %s\n", pn)
		if err := lazypagessrv.ExpandLazyPages(pn); err != nil {
			return err
		}
	}
	db.DPrintf(db.CKPT, "Done readCheckpoint%s\n", pn)

	return nil
}

func (ps *ProcSrv) readCheckpoint2(ckptSigmaDir, localDir, ckpt string) error {
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
		db.DPrintf(db.CKPT, "copying %s\n", entry)
		fn := filepath.Join(ckptSigmaDir, ckpt, entry)
		dstfn := filepath.Join(pn, entry)
		if err := ps.ssrv.MemFs.SigmaClnt().DownloadFile(fn, dstfn); err != nil {
			db.DPrintf(db.PROCD, "DownloadFile %v err %v\n", fn, err)
			return err
		}
	}
	//	db.DPrintf(db.CKPT, "Almost done %s\n", pn)
	if ckpt == CKPTLAZY {
		db.DPrintf(db.CKPT, "Expand %s\n", pn)
		if err := lazypagessrv.ExpandLazyPages(pn); err != nil {
			return err
		}
	}
	db.DPrintf(db.CKPT, "Done readCheckpoint%s\n", pn)

	return nil
}

func (ps *ProcSrv) readCheckpointAndRegister(ckptSigmaDir, localDir, ckpt string, pid int) error {
	db.DPrintf(db.CKPT, "readCheckpoint %v %v %v", ckptSigmaDir, localDir, ckpt)

	os.Mkdir(localDir, 0755)
	pn := filepath.Join(localDir, ckpt)
	//dstRegfn := filepath.Join(pn, "dumpreg.zip")
	dstfn := filepath.Join(pn, "dump.zip")
	if err := os.Mkdir(pn, 0755); err != nil {
		return err
	}

	// fnReg := filepath.Join(ckptSigmaDir, ckpt, "dumpreg.zip")
	// if err := ps.ssrv.MemFs.SigmaClnt().DownloadFile(fnReg, dstRegfn); err != nil {
	// 	db.DPrintf(db.PROCD, "DownloadFile %v err %v\n", fnReg, err)
	// 	return err
	// }
	// rReg, err := zip.OpenReader(dstRegfn)
	// if err != nil {
	// 	db.DPrintf(db.PROCD, "openzip %v err %v\n", fnReg, err)
	// 	return err
	// }
	// defer rReg.Close()
	// db.DPrintf(db.PROCD, "len %v\n", len(rReg.File))
	// // Loop through each file in the archive
	// for _, f := range rReg.File {
	// 	db.DPrintf(db.CKPT, "uncompressReg %s\n", f.Name)
	// 	fPath := filepath.Join(pn, f.Name)

	// 	// Open file inside zip
	// 	srcFile, err := f.Open()
	// 	if err != nil {
	// 		return err
	// 	}
	// 	defer srcFile.Close()

	// 	// Create destination file
	// 	dstFile, err := os.Create(fPath)
	// 	if err != nil {
	// 		dstFile.Close()
	// 		return err
	// 	}

	// 	// Copy contents
	// 	_, err = io.Copy(dstFile, srcFile)
	// 	if err != nil {
	// 		dstFile.Close()
	// 		return err
	// 	}
	// 	dstFile.Close()
	// }

	sts, err := ps.ssrv.MemFs.SigmaClnt().GetDir(filepath.Join(ckptSigmaDir, ckpt))
	files := sp.Names(sts)
	for _, entry := range files {
		db.DPrintf(db.CKPT, "SEE %s\n", entry)
		if strings.HasPrefix(entry, "pagemap") || strings.HasPrefix(entry, "mm-") {
			db.DPrintf(db.PROCD, "DownloadFile %s\n", entry)
			fn := filepath.Join(ckptSigmaDir, ckpt, entry)
			dstfn := filepath.Join(pn, entry)
			if err := ps.ssrv.MemFs.SigmaClnt().DownloadFile(fn, dstfn); err != nil {
				db.DPrintf(db.PROCD, "DownloadFile %v err %v\n", fn, err)
				return err
			}
		}
	}
	go func() {
		pages := filepath.Join(ckptSigmaDir, CKPTFULL, "pages-"+strconv.Itoa(pid)+".img")
		db.DPrintf(db.CKPT, "restoreProc: Register %d %v", pid, pages)
		if err := ps.lpc.Register(pid, pn, pages); err != nil {
			db.DPrintf(db.CKPT, "restoreProc: Register %d %v err %v", pid, pages, err)
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
		//db.DPrintf(db.CKPT, "uncompress %s\n", f.Name)
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

func (ps *ProcSrv) readCheckpointAndRegisterLz4(ckptSigmaDir, localDir, ckpt string, pid int) error {
	db.DPrintf(db.CKPT, "readCheckpoint %v %v %v", ckptSigmaDir, localDir, ckpt)

	os.Mkdir(localDir, 0755)
	pn := filepath.Join(localDir, ckpt)
	//dstRegfn := filepath.Join(pn, "dumpreg.zip")
	dstfn := filepath.Join(pn, "dump.lz4")
	if err := os.Mkdir(pn, 0755); err != nil {
		return err
	}

	sts, err := ps.ssrv.MemFs.SigmaClnt().GetDir(filepath.Join(ckptSigmaDir, ckpt))
	files := sp.Names(sts)
	for _, entry := range files {
		db.DPrintf(db.CKPT, "SEE %s\n", entry)
		if strings.HasPrefix(entry, "pagemap") || strings.HasPrefix(entry, "mm-") {
			db.DPrintf(db.PROCD, "DownloadFile %s\n", entry)
			fn := filepath.Join(ckptSigmaDir, ckpt, entry)
			dstfn := filepath.Join(pn, entry)
			if err := ps.ssrv.MemFs.SigmaClnt().DownloadFile(fn, dstfn); err != nil {
				db.DPrintf(db.PROCD, "DownloadFile %v err %v\n", fn, err)
				return err
			}
		}
	}
	go func() {
		pages := filepath.Join(ckptSigmaDir, CKPTFULL, "pages-"+strconv.Itoa(pid)+".img")
		db.DPrintf(db.CKPT, "restoreProc: Register %d %v", pid, pages)
		if err := ps.lpc.Register(pid, pn, pages); err != nil {
			db.DPrintf(db.CKPT, "restoreProc: Register %d %v err %v", pid, pages, err)
			return
		}
		db.DPrintf(db.CKPT, "restoreProc: Registered %d %v", pid, pages)
	}()

	fn := filepath.Join(ckptSigmaDir, ckpt, "dump.lz4")
	if err := ps.ssrv.MemFs.SigmaClnt().DownloadFile(fn, dstfn); err != nil {
		db.DPrintf(db.PROCD, "DownloadFile %v err %v\n", fn, err)
		return err
	}
	lz4File, err := os.Open(dstfn)
	if err != nil {
		return err
	}
	defer lz4File.Close()
	// Create LZ4 reader
	lz4Reader := lz4.NewReader(lz4File)

	// Buffer for reading file data
	buffer := make([]byte, bufferSize)

	// Track extraction stats
	var totalExtractedSize int64
	var filesExtracted int
	db.DPrintf(db.CKPT, "Start uncompress %s", fn)
	// Process files until end of archive
	for {
		// Read file header
		var nameLength uint32
		if err := binary.Read(lz4Reader, binary.LittleEndian, &nameLength); err != nil {
			if err == io.EOF {
				break // End of file reached normally
			}
			return fmt.Errorf("error reading header: %w", err)
		}

		// Check for end-of-archive marker
		if nameLength == 0 {
			break
		}

		// Read file size
		var fileSize uint64
		if err := binary.Read(lz4Reader, binary.LittleEndian, &fileSize); err != nil {
			return fmt.Errorf("error reading file size: %w", err)
		}

		// Read filename
		nameBytes := make([]byte, nameLength)
		if _, err := io.ReadFull(lz4Reader, nameBytes); err != nil {
			return fmt.Errorf("error reading filename: %w", err)
		}
		fileName := string(nameBytes)

		// Create output file path
		outPath := filepath.Join(pn, fileName)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", outPath, err)
		}

		// Create output file
		outFile, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("failed to create output file %s: %w", outPath, err)
		}

		// Copy file data
		bytesRemaining := fileSize
		for bytesRemaining > 0 {
			// Calculate how much to read in this iteration
			toRead := uint64(bufferSize)
			if bytesRemaining < toRead {
				toRead = bytesRemaining
			}

			// Read compressed data
			n, err := io.ReadAtLeast(lz4Reader, buffer[:toRead], int(toRead))
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				outFile.Close()
				return fmt.Errorf("error reading file data for %s: %w", fileName, err)
			}

			// Write to output file
			if _, err := outFile.Write(buffer[:n]); err != nil {
				outFile.Close()
				return fmt.Errorf("error writing to %s: %w", outPath, err)
			}

			bytesRemaining -= uint64(n)
			if n < int(toRead) {
				if bytesRemaining > 0 {
					outFile.Close()
					return fmt.Errorf("unexpected end of data for %s", fileName)
				}
				break
			}
		}

		outFile.Close()
		totalExtractedSize += int64(fileSize)
		filesExtracted++
		//db.DPrintf(db.CKPT, "Extracted %s (%.2f MB)\n", fileName, float64(fileSize)/1024/1024)
	}

	db.DPrintf(db.CKPT,
		"Extraction complete: %d files (%.2f KB total) extracted to %s\n",
		filesExtracted,
		float64(totalExtractedSize)/1024,
		pn,
	)

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
