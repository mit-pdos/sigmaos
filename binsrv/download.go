package binsrv

import (
	"fmt"
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
	CHUNKSZ            = 1 * sp.MBYTE
)

func index(o int64) int { return int(o / CHUNKSZ) }
func ckoff(i int) int64 { return int64(i * CHUNKSZ) }

type chunk struct {
	cond     *sync.Cond
	nwaiters int
	i        int
	n        int // bytes read
	b        []byte
	ch       chan error
}

type downloader struct {
	mu       sync.Mutex
	pn       string
	kernelId string
	sc       *sigmaclnt.SigmaClnt
	sz       int64
	chunks   []*chunk
	n        int64
	sfd      int      // sigma fd
	ufd      *os.File // unix fd
	ch       chan *chunk
	err      error
	tot      time.Duration
}

func newDownload(pn string, sc *sigmaclnt.SigmaClnt, kernelId string, sfd int, ufd *os.File, sz int64) *downloader {
	dl := &downloader{
		pn:       pn,
		sc:       sc,
		kernelId: kernelId,
		chunks:   make([]*chunk, index(sz)+1),
		sfd:      sfd,
		ufd:      ufd,
		sz:       sz,
		ch:       make(chan *chunk),
	}
	return dl
}

func newDownloader(pn string, sc *sigmaclnt.SigmaClnt, kernelId string, sz int64) (*downloader, error) {
	sfd := 0
	s := time.Now()
	paths := downloadPaths(pn, kernelId)
	if err := retryPaths(paths, func(i int, pn string) error {
		db.DPrintf(db.BINSRV, "open %q\n", pn)
		fd, err := sc.Open(pn, sp.OREAD)
		if err == nil {
			sfd = fd
			return nil
		}
		return err
	}); err != nil {
		return nil, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] SigmaOS Open %d %v", pn, sfd, time.Since(s))
	s = time.Now()
	ufd, err := os.OpenFile(pn, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Ux Open %d %v", pn, ufd, time.Since(s))
	dl := newDownload(pn, sc, kernelId, sfd, ufd, sz)
	go dl.downloader()
	return dl, nil
}

func (dl *downloader) String() string {
	return fmt.Sprintf("{pn %q sz %d chunks %d n %d sfd %d}", dl.pn, dl.sz, len(dl.chunks), dl.n, dl.sfd)
}

func (dl *downloader) file() string {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	s := fmt.Sprintf("%d [", dl.sz)
	n := 0
	for _, ck := range dl.chunks {
		if ck != nil {
			n += ck.n
			s += fmt.Sprintf("%d(%d)[%d, %d) ", ck.i, ck.n, ckoff(ck.i), ckoff(ck.i)+int64(ck.n))
		}
	}
	s += fmt.Sprintf("] tot %d", n)
	return s
}

func (dl *downloader) write(off int64, b []byte) error {
	if _, err := dl.ufd.Seek(off, 0); err != nil {
		return err
	}
	db.DPrintf(db.BINSRV, "write %q %d %d\n", dl.pn, off, len(b))
	nn, err := dl.ufd.Write(b)
	if nn != len(b) {
		return err
	}
	return nil
}

func (dl *downloader) downloader() {
	for ck := range dl.ch {
		s := time.Now()
		b, err := dl.readChunk(ckoff(ck.i), CHUNKSZ)
		if err == nil {
			err = dl.write(ckoff(ck.i), b)
			if err == nil {
				dl.insert(ck.i, len(b), b)
				d := time.Since(s)
				dl.tot += d
				db.DPrintf(db.SPAWN_LAT, "[%v] Chunk %v %v tot %v", dl.pn, ck.i, d, dl.tot)

			}
		}
		ck.ch <- err
	}
}

// Note: don't invoke readChunk concurrently
func (dl *downloader) readChunk(off int64, l int) ([]byte, error) {
	if err := dl.sc.Seek(dl.sfd, sp.Toffset(off)); err != nil {
		return nil, err
	}
	db.DPrintf(db.BINSRV, "read %q %d %d\n", dl.pn, off, l)
	b, err := dl.sc.Read(dl.sfd, sp.Tsize(l))
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (dl *downloader) insert(i int, len int, b []byte) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	dl.n += int64(len)
	db.DPrintf(db.BINSRV, "insert %q i %d len %d tot %d\n", dl.pn, i, len, dl.n)
	ck := dl.chunks[i]
	if ck == nil {
		db.DFatalf("end: unknown %d %d\n", i)
	}
	ck.b = b
	ck.n = len
}

func (dl *downloader) getChunk(i int) *chunk {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	db.DPrintf(db.BINSRV, "getChunk %d", i)

	ck := dl.chunks[i]
	if ck == nil {
		ck = &chunk{cond: sync.NewCond(&dl.mu), i: i, ch: make(chan error)}
		dl.chunks[i] = ck
		return ck
	}
	if ck.n == 0 {
		ck.nwaiters++
		db.DPrintf(db.BINSRV, "waitDownload: wait chunk %d nw %d", i, ck.nwaiters)
		ck.cond.Wait()
		ck.nwaiters--
	}
	return ck
}

func (dl *downloader) signal(i int) {
	c := dl.chunks[i]
	if c == nil {
		db.DFatalf("signal: unknown %d\n", i)
	}
	c.cond.Broadcast()
}

func (dl *downloader) read(off int64, len int) error {
	var err error
	i := index(off)
	j := index(off+int64(len)) + 1
	db.DPrintf(db.BINSRV, "read %d %d: chunks [%d,%d)", off, len, i, j)
	for c := i; c < j; c++ {
		ck := dl.getChunk(c)
		if ck.n == 0 {
			dl.ch <- ck
			err = <-ck.ch
			dl.signal(c)
		}
	}
	return err
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
