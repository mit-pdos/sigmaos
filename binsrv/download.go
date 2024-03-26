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

	SEEK_DATA = 3
	SEEK_HOLE = 4
)

func index(o int64) int { return int(o / CHUNKSZ) }
func ckoff(i int) int64 { return int64(i * CHUNKSZ) }

type chksT []int64 // bytes present

func (chks chksT) String() string {
	s := fmt.Sprintf("[")
	n := int64(0)
	for i, l := range chks {
		if l != 0 {
			n += l
			s += fmt.Sprintf("%d(%d)[%d, %d) ", i, l, ckoff(i), ckoff(i)+int64(l))
		}
	}
	s += fmt.Sprintf("] tot %d", n)
	return s
}

type reader struct {
	cond     *sync.Cond
	nreaders int
	ck       int
	ch       chan error
	cached   bool
}

type readersT []*reader

func newReader(dl *downloader, ck int) *reader {
	return &reader{cond: sync.NewCond(&dl.mu), ck: ck, ch: make(chan error)}
}

type downloader struct {
	mu       sync.Mutex
	pn       string
	kernelId string
	sc       *sigmaclnt.SigmaClnt
	sz       int64
	chunks   chksT
	readers  readersT
	n        int64
	sfd      int // sigma fd
	ch       chan *reader
	err      error
	tot      time.Duration
}

func newDownload(pn string, sc *sigmaclnt.SigmaClnt, kernelId string, sfd int, sz int64) *downloader {
	dl := &downloader{
		pn:       pn,
		sc:       sc,
		kernelId: kernelId,
		chunks:   make(chksT, index(sz)+1),
		readers:  make(readersT, index(sz)+1),
		sfd:      sfd,
		sz:       sz,
		ch:       make(chan *reader),
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
	dl := newDownload(pn, sc, kernelId, sfd, sz)
	go dl.downloader()
	return dl, nil
}

func (dl *downloader) String() string {
	return fmt.Sprintf("{pn %q sz %d chunks %d n %d sfd %d}", dl.pn, dl.sz, len(dl.chunks), dl.n, dl.sfd)
}

func (dl *downloader) writeCache(off int64, b []byte) error {
	s := time.Now()
	ufd, err := os.OpenFile(binCachePath(dl.pn), os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		return err
	}
	defer ufd.Close()
	if _, err := ufd.Seek(off, 0); err != nil {
		return err
	}
	nn, err := ufd.Write(b)
	if nn != len(b) {
		return err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] writeCache %v", dl.pn, time.Since(s))
	return nil
}

func (dl *downloader) downloader() {
	for r := range dl.ch {
		sz, ok := dl.readLocal(r.ck)
		if ok {
			dl.chunks[r.ck] = sz
			db.DPrintf(db.BINSRV, "readLocal chunks %q %d %v\n", dl.pn, sz, dl.chunks)
			r.ch <- nil
			continue
		}
		err := dl.cacheRemoteChunk(r.ck)
		r.ch <- err
	}
}

func (dl *downloader) readLocal(ck int) (int64, bool) {
	db.DPrintf(db.BINSRV, "readLocal %q %d sz %d\n", dl.pn, ck, dl.sz)
	f, err := os.OpenFile(binCachePath(dl.pn), os.O_RDONLY, 0777)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sz := int64(0)
	ok := false
	for off := int64(0); off < dl.sz; {
		o1, err := f.Seek(off, SEEK_DATA)
		if err != nil {
			break
		}
		o2, err := f.Seek(o1, SEEK_HOLE)
		if err != nil {
			db.DFatalf("Seek hole %q %d err %v\n", dl.pn, o2, err)
		}
		for o := o1; o < o2; o += CHUNKSZ {
			if o%CHUNKSZ != 0 {
				db.DFatalf("offset %d\n", o)
			}
			if o+CHUNKSZ <= o2 || o2 >= dl.sz { // a complete chunk?
				i := index(o)
				if i == ck {
					db.DPrintf(db.BINSRV, "readLocal: %q read chunk %d(%d)\n", dl.pn, i, o)
					ok = true
					sz = CHUNKSZ
					if o2 >= dl.sz {
						sz = dl.sz - o1
					}
					break
				}
			}
		}
		off = o2
	}
	return sz, ok
}

func (dl *downloader) cacheRemoteChunk(ck int) error {
	s := time.Now()
	b := make([]byte, CHUNKSZ)
	sz, err := dl.readRemoteChunk(ckoff(ck), b)
	db.DPrintf(db.SPAWN_LAT, "[%v] readRemoteChunk %v dur %v tot %v", dl.pn, ck, time.Since(s), dl.tot)
	if err == nil {
		if err := dl.writeCache(ckoff(ck), b[0:sz]); err == nil {
			dl.register(ck, sz)
			d := time.Since(s)
			dl.tot += d
			db.DPrintf(db.SPAWN_LAT, "[%v] cacheRemoteChunk %v chunkDur %v tot %v", dl.pn, ck, d, dl.tot)
		}
	}
	return nil
}

// Note: don't invoke readChunk concurrently
func (dl *downloader) readRemoteChunk(off int64, b []byte) (int64, error) {
	if err := dl.sc.Seek(dl.sfd, sp.Toffset(off)); err != nil {
		return 0, err
	}
	db.DPrintf(db.BINSRV, "readRemoteChunk %q %d(%d)", dl.pn, index(off), off)
	sz, err := dl.sc.Read(dl.sfd, b)
	if err != nil {
		db.DPrintf(db.ERROR, "readRemoteChunk %q %d(%d) err %v\n", dl.pn, index(off), off, err)
		return 0, err
	}
	db.DPrintf(db.BINSRV, "readRemoteChunk %q read %d\n", dl.pn, sz)
	return int64(sz), nil
}

func (dl *downloader) register(ck int, len int64) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	db.DPrintf(db.BINSRV, "register %q i %d len %d tot %d\n", dl.pn, ck, len, dl.n)
	dl.n += len
	dl.chunks[ck] = len
}

func (dl *downloader) getReader(i int) *reader {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	r := dl.readers[i]
	if r == nil {
		r = newReader(dl, i)
		dl.readers[i] = r
		return r
	}
	if !r.cached {
		r.nreaders++
		db.DPrintf(db.BINSRV, "getReader: wait chunk %d nw %d", i, r.nreaders)
		r.cond.Wait()
		r.nreaders--
	}
	return r
}

func (dl *downloader) signal(i int) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	r := dl.readers[i]
	r.cached = true
	r.cond.Broadcast()
}

func min(n int64, l int) int {
	if int64(l) < n {
		return l
	} else {
		return int(n)
	}
}

func (dl *downloader) read(off int64, len int) (int, error) {
	var err error
	i := index(off)
	o := off - ckoff(i)
	j := index(off+int64(len)) + 1
	n := int64(0)
	db.DPrintf(db.BINSRV, "read %d %d: chunks [%d,%d)", off, len, i, j)
	for c := i; c < j; c++ {
		r := dl.getReader(c)
		if !r.cached {
			dl.ch <- r
			err = <-r.ch
			dl.signal(c)
		}
		n += dl.chunks[c]
	}
	return min(n-o, len), err
}

func downloadPaths(pn, kernelId string) []string {
	buildTag := ""
	pn, buildTag = binPathParse(pn)
	name := filepath.Base(pn)
	paths := []string{
		filepath.Join(sp.UX, kernelId, "bin/user/common", name),
		filepath.Join(sp.S3, "~local", buildTag, "bin", name),
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
