package binsrv

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	N_DOWNLOAD_RETRIES = 100
)

type downloader struct {
	mu       sync.Mutex
	pn       string
	kernelId string
	sc       *sigmaclnt.SigmaClnt
	waiters  *sync.Cond
	nwaiter  int
	done     bool
}

func newDownloader(pn string, sc *sigmaclnt.SigmaClnt, kernelId string) *downloader {
	dl := &downloader{pn: pn, sc: sc, kernelId: kernelId}
	dl.waiters = sync.NewCond(&dl.mu)
	go dl.loader()
	return dl
}

func newDownloaderPresent(pn string, sc *sigmaclnt.SigmaClnt, kernelId string) *downloader {
	dl := &downloader{pn: pn, sc: sc, kernelId: kernelId}
	dl.waiters = sync.NewCond(&dl.mu)
	dl.done = true
	return dl
}

func (dl *downloader) String() string {
	return fmt.Sprintf("{pn %q nwaiter %d done %t}", dl.pn, dl.nwaiter, dl.done)
}

func (dl *downloader) loader() {
	db.DPrintf(db.BINSRV, "loader starting for %q", dl.pn)
	if err := dl.downloadProcBin(); err != nil {
		db.DPrintf(db.BINSRV, "download %q err %v\n", dl.pn, err)
	}
	db.DPrintf(db.BINSRV, "loader download done for %q", dl.pn)
	// time.Sleep(2 * time.Second)
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.done = true
	dl.waiters.Broadcast()
}

func (dl *downloader) waitDownload() int {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	db.DPrintf(db.BINSRV, "waitDownload %v\n", dl)
	if !dl.done {
		dl.nwaiter++
		db.DPrintf(db.BINSRV, "nwaiters %d", dl.nwaiter)
		dl.waiters.Wait()
		dl.nwaiter--
	}
	fd, err := syscall.Open(dl.pn, syscall.O_RDONLY, 0)
	if err != nil {
		db.DFatalf("open %q err %v", dl.pn, err)
	}
	return fd
}

func (dl *downloader) copyFile(src string, dst string) error {
	rdr, err := dl.sc.OpenAsyncReader(src, 0)
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

func (dl *downloader) download(i int, src string) error {
	tmpdst := dl.pn + rand.String(8)
	start := time.Now()
	if err := dl.copyFile(src, tmpdst); err != nil {
		return err
	}
	if err := os.Rename(tmpdst, dl.pn); err != nil {
		return err
	}
	db.DPrintf(db.BINSRV, "Took %v to download proc %v", time.Since(start), src)
	db.DPrintf(db.SPAWN_LAT, "Took %v to download proc %v", time.Since(start), src)
	return nil
}

func downloadPaths(pn, kernelId string) []string {
	name := filepath.Base(pn)
	buildTag := "TODO XXX" // don't have the proc here
	paths := []string{
		filepath.Join(sp.UX, kernelId, "bin/user/common", name),
		filepath.Join(sp.S3, "~local", buildTag, "bin"),
	}
	// For user bins, go straight to S3 instead of checking locally first.
	if sp.Target != "local" && name != "named" && name != "spawn-latency-ux" {
		paths = paths[1:]
	}
	return paths
}

func (dl *downloader) downloadProcBin() error {
	paths := downloadPaths(dl.pn, dl.kernelId)
	return retryPaths(paths, dl.download)
}

func retryLoop(i int, f func(i int, pn string) error, src string) error {
	var r error
	for i := 0; i < N_DOWNLOAD_RETRIES; i++ {
		// Return if successful. Else, retry
		if err := f(i, src); err == nil {
			return nil
		} else {
			db.DPrintf(db.BINSRV, "download %q err %v", src, err)
			r = err
			if serr.IsErrCode(err, serr.TErrNotfound) {
				break
			}
		}
	}
	return fmt.Errorf("retryLoop: couldn't do %T for %q in %d retries err %v", f, src, N_DOWNLOAD_RETRIES, r)

}

func retryPaths(paths []string, f func(i int, pn string) error) error {
	var r error
	for i, pp := range paths {
		if err := retryLoop(i, f, pp); err == nil {
			return nil
		} else {
			db.DPrintf(db.BINSRV, "download pp %q err %v", pp, err)
			r = err
		}
	}
	return r
}
