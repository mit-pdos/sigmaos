package binsrv

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	db "sigmaos/debug"
	"sigmaos/rand"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	N_DOWNLOAD_RETRIES = 100
)

func (n *binFsNode) copyFile(src string, dst string) error {
	rdr, err := n.RootData.Sc.OpenAsyncReader(src, 0)
	if err != nil {
		return err
	}
	defer rdr.Close()
	f, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return err
	}
	defer f.Close()

	b := make([]byte, sp.BUFSZ)
	for {
		//		start := time.Now()
		n, err := rdr.Read(b)
		if err != nil && err != io.EOF {
			return err
		}
		// Nothing left to read
		if n == 0 {
			break
		}
		//		db.DPrintf(db.ALWAYS, "Time reading in copyFile: %v", time.Since(start))
		b2 := b[:n]
		nn, err := f.Write(b2)
		if err != nil {
			return err
		}
		if nn != n {
			return fmt.Errorf("short write %v != %v", nn, n)
		}
	}
	return nil
}

func (n *binFsNode) download(src, dst string) error {
	st, err := n.RootData.Sc.Stat(src)
	if err != nil {
		return err
	}
	db.DPrintf(db.BINSRV, "Stat %q %v\n", src, st)
	tmpdst := dst + rand.String(8)
	start := time.Now()
	if err := n.copyFile(src, tmpdst); err != nil {
		return err
	}
	if err := os.Rename(tmpdst, dst); err != nil {
		return err
	}
	db.DPrintf(db.BINSRV, "Took %v to download proc %v", time.Since(start), src)
	db.DPrintf(db.SPAWN_LAT, "Took %v to download proc %v", time.Since(start), src)
	return nil
}

func (n *binFsNode) downloadRetry(src, dst string) error {
	var r error
	for i := 0; i < N_DOWNLOAD_RETRIES; i++ {
		// Return if successful. Else, retry
		if err := n.download(src, dst); err == nil {
			return nil
		} else {
			db.DPrintf(db.BINSRV, "download %q err %v", src, err)
			r = err
			if serr.IsErrCode(err, serr.TErrNotfound) {
				break
			}
		}
	}
	return fmt.Errorf("downloadRetry: couldn't download %q in %v retries err %v", src, N_DOWNLOAD_RETRIES, r)
}

func (n *binFsNode) downloadProcBin(dst string) error {
	name := filepath.Base(dst)
	buildTag := "TODO XXX" // don't have the proc here
	paths := []string{
		filepath.Join(sp.UX, n.RootData.KernelId, "bin/user/common", name),
		filepath.Join(sp.S3, "~local", buildTag, "bin"),
	}
	// For user bins, go straight to S3 instead of checking locally first.
	if sp.Target != "local" && name != "named" && name != "spawn-latency-ux" {
		paths = paths[1:]
	}
	var r error
	for _, pp := range paths {
		if err := n.downloadRetry(pp, dst); err == nil {
			return nil
		} else {
			db.DPrintf(db.BINSRV, "download pp %q err %v", pp, err)
			r = err
		}
	}
	return r
}
