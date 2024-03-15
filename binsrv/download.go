package binsrv

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	N_DOWNLOAD_RETRIES = 100
)

type waiter struct {
	cond     *sync.Cond
	nwaiters int
}

type downloader struct {
	mu       sync.Mutex
	pn       string
	kernelId string
	sc       *sigmaclnt.SigmaClnt
	waiters  map[int64]*waiter
	n        int64
	eof      bool
}

func newDownload(pn string, sc *sigmaclnt.SigmaClnt, kernelId string) *downloader {
	dl := &downloader{
		pn:       pn,
		sc:       sc,
		kernelId: kernelId,
		waiters:  make(map[int64]*waiter),
	}
	return dl
}

func newDownloader(pn string, sc *sigmaclnt.SigmaClnt, kernelId string) *downloader {
	dl := newDownload(pn, sc, kernelId)
	go dl.loader()
	return dl
}

func (dl *downloader) String() string {
	return fmt.Sprintf("{pn %q nwaiter %d n %d eof %t}", dl.pn, len(dl.waiters), dl.n, dl.eof)
}

func (dl *downloader) loader() {
	db.DPrintf(db.BINSRV, "loader starting for %q", dl.pn)
	if err := dl.downloadProcBin(); err != nil {
		db.DPrintf(db.BINSRV, "download %q err %v\n", dl.pn, err)
	}
	db.DPrintf(db.BINSRV, "loader download done for %q", dl.pn)
}

func (dl *downloader) waitDownload(off int64, l int) (int64, bool) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	end := off + int64(l)
	db.DPrintf(db.BINSRV, "waitDownload %v o %d end %d\n", dl, off, end)
	if !dl.eof && end > dl.n {
		w, ok := dl.waiters[end]
		if !ok {
			w = &waiter{cond: sync.NewCond(&dl.mu)}
			dl.waiters[end] = w
		}
		w.nwaiters++
		db.DPrintf(db.BINSRV, "waitDownload: wait chunk %d nw %d", end, w.nwaiters)
		w.cond.Wait()
		delete(dl.waiters, end)
	}
	return dl.n, dl.eof
}

func (dl *downloader) signal(n int64, eof bool, err error) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	dl.n += n
	dl.eof = eof
	for e, w := range dl.waiters {
		if (dl.n > e && ONDEMAND) || eof {
			db.DPrintf(db.BINSRV, "signal end %d eof %t\n", e, eof)
			w.cond.Broadcast()
		}
	}
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
	var r error
	for {
		// start := time.Now()
		n, err := rdr.Read(b)
		if err != nil && err != io.EOF {
			r = err
			break
		}
		// Nothing left to read
		if n == 0 {
			break
		}
		//	db.DPrintf(db.ALWAYS, "Time reading in copyFile: %v", time.Since(start))
		b2 := b[:n]
		nn, err := f.Write(b2)
		if err != nil {
			r = err
			break
		}
		if nn != n {
			r = fmt.Errorf("short write %v != %v", nn, n)
			break
		}
		dl.signal(int64(n), false, nil)
	}
	db.DPrintf(db.BINSRV, "Download %q done", src)
	dl.signal(0, true, r)
	return nil
}

func (dl *downloader) download(i int, src string) error {
	start := time.Now()
	if err := dl.copyFile(src, dl.pn); err != nil {
		return err
	}
	db.DPrintf(db.SPAWN_LAT, "Took %v to download proc %v", time.Since(start), src)
	return nil
}

func downloadPaths(pn, kernelId string) []string {
	buildTag := ""
	pn, buildTag = binPathParse(pn)
	name := filepath.Base(pn)
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
