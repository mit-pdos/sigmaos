package binsrv

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"sigmaos/chunkclnt"
	"sigmaos/chunksrv"
	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

const (
	SEEK_DATA = 3
	SEEK_HOLE = 4
)

type chksT []int64 // bytes present

func (chks chksT) String() string {
	s := fmt.Sprintf("[")
	n := int64(0)
	for i, l := range chks {
		if l != 0 {
			n += l
			s += fmt.Sprintf("%d(%d)[%d, %d) ", i, l, chunksrv.Ckoff(i), chunksrv.Ckoff(i)+int64(l))
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
	sz       sp.Tsize
	chunks   chksT
	readers  readersT
	n        int64
	ckclnt   *chunkclnt.ChunkClnt
	ch       chan *reader
	err      error
	tot      time.Duration
}

func newDownload(pn string, sc *sigmaclnt.SigmaClnt, ckclnt *chunkclnt.ChunkClnt, sz sp.Tsize) *downloader {
	dl := &downloader{
		pn:      pn,
		sc:      sc,
		chunks:  make(chksT, chunksrv.Index(int64(sz))+1),
		readers: make(readersT, chunksrv.Index(int64(sz))+1),
		ckclnt:  ckclnt,
		sz:      sz,
		ch:      make(chan *reader),
	}
	return dl
}

func newDownloader(pn string, sc *sigmaclnt.SigmaClnt, ckclnt *chunkclnt.ChunkClnt, sz sp.Tsize) (*downloader, error) {
	dl := newDownload(pn, sc, ckclnt, sz)
	go dl.downloader()
	return dl, nil
}

func (dl *downloader) String() string {
	return fmt.Sprintf("{pn %q sz %d chunks %d n %d ckclnt %v}", dl.pn, dl.sz, len(dl.chunks), dl.n, dl.ckclnt)
}

func (dl *downloader) downloader() {
	for r := range dl.ch {
		pn := binCachePath(dl.pn)
		sz, ok := chunksrv.ReadChunk(pn, r.ck, dl.sz)
		if ok {
			dl.chunks[r.ck] = sz
			db.DPrintf(db.BINSRV, "readLocal %q %d cks %v\n", pn, sz, dl.chunks)
			r.ch <- nil
			continue
		}
		err := dl.cacheRemoteChunk(r.ck)
		r.ch <- err
	}
}

func (dl *downloader) cacheRemoteChunk(ck int) error {
	s := time.Now()
	sz, err := dl.readRemoteChunk(ck)
	d := time.Since(s)
	db.DPrintf(db.SPAWN_LAT, "[%v] readRemoteChunk %v dur %v tot %v", dl.pn, ck, d, dl.tot)
	if err == nil {
		dl.register(ck, sz)
		dl.tot += d
	}
	return nil
}

// Fetch chunk through uprocd, which will fill in the realm and
// write it a local file, which binsrv can read.
func (dl *downloader) readRemoteChunk(ck int) (int64, error) {
	prog, paths := binPathParse(dl.pn)
	sz, err := dl.ckclnt.UprocdFetch(prog, sp.NOREALM, ck, dl.sz, paths, nil)
	if err != nil {
		db.DPrintf(db.ERROR, "readRemoteChunk %q fetch %d err %v\n", dl.pn, ck, err)
		return 0, err
	}
	db.DPrintf(db.BINSRV, "readRemoteChunk %q read ck %d sz %d\n", dl.pn, ck, sz)
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
	i := chunksrv.Index(off)
	o := off - chunksrv.Ckoff(i)
	j := chunksrv.Index(off+int64(len)) + 1
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

func downloadPaths(pn, kernelId string) (string, []string) {
	bin, paths := binPathParse(pn)

	// XXX hack; how to handle ~local?
	for i, p := range paths {
		if strings.HasPrefix(p, sp.UX) {
			paths[i] = strings.Replace(p, "~local", kernelId, 1)
		}
	}
	return bin, paths
}
