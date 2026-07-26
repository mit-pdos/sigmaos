package binsrv

import (
	"fmt"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	schedproc "sigmaos/sched/msched/proc"
	"sigmaos/sched/msched/proc/chunk"
	chunksrv "sigmaos/sched/msched/proc/chunk/srv"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

const (
	SEEK_DATA = 3
	SEEK_HOLE = 4
)

type downloader struct {
	mu  sync.Mutex
	pn  string
	sz  sp.Tsize
	pid uint32
	p   *proc.Proc
	pds schedproc.ProcSrv
	err error
	tot time.Duration

	// Per-open (i.e., per-exec) counters, reported by Stats and logged when
	// the file handle is released. They measure how much FUSE work an exec
	// of this binary costs procd: how many reads the kernel sent us to page
	// the binary in, how much of the file we handed back, how much time went
	// into deciding whether chunks were present (chunksrv.IsPresent reopens
	// and scans the cache file on every read), and how many chunks we had to
	// fetch (should be 0 after the first proc for this binary on this node).
	nread     int
	nbyte     int64
	npresent  int
	presentMs time.Duration
	nfetch    int
}

type dlStats struct {
	nread     int
	nbyte     int64
	npresent  int
	presentMs time.Duration
	nfetch    int
	fetchMs   time.Duration
}

func (dl *downloader) Stats() dlStats {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	return dlStats{
		nread:     dl.nread,
		nbyte:     dl.nbyte,
		npresent:  dl.npresent,
		presentMs: dl.presentMs,
		nfetch:    dl.nfetch,
		fetchMs:   dl.tot,
	}
}

func newDownloader(pn string, pds schedproc.ProcSrv, sz sp.Tsize, p *proc.Proc, pid uint32) *downloader {
	dl := &downloader{
		pn:  pn,
		pds: pds,
		sz:  sz,
		pid: pid,
		p:   p,
	}
	return dl
}

func (dl *downloader) String() string {
	return fmt.Sprintf("{pn %q sz %d}", dl.pn, dl.sz)
}

func (dl *downloader) countRead(nbyte int64) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.nread++
	dl.nbyte += nbyte
}

func (dl *downloader) countPresent(d time.Duration) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.npresent++
	dl.presentMs += d
}

func (dl *downloader) countFetch(d time.Duration) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.nfetch++
	dl.tot += d
}

// Fetch chunk through procd, which will fill in the realm and
// write it a local file, which binsrv can read.
func (dl *downloader) fetchChunk(ck int) (int64, error) {
	db.DPrintf(db.BINSRV, "fetchChunk invoke %q ck %d\n", dl.pn, ck)
	sz, err := dl.pds.Fetch(int(dl.pid), ck, dl.pn, dl.sz)
	if err != nil {
		return 0, err
	}
	db.DPrintf(db.BINSRV, "fetchChunk done %q ck %d sz %d\n", dl.pn, ck, sz)
	return int64(sz), nil
}

func min(n int64, l int) int {
	if int64(l) < n {
		return l
	} else {
		return int(n)
	}
}

func (dl *downloader) read(off int64, nbyte int) (int, error) {
	i := chunk.Index(off)
	o := off - chunk.ChunkOff(i)
	j := chunk.Index(off+int64(nbyte)) + 1
	n := int64(0)
	db.DPrintf(db.BINSRV, "read %d %d: chunks [%d,%d)", off, nbyte, i, j)
	start := time.Now()
	for c := i; c < j; c++ {
		pn := binCachePath(dl.pn)
		presentStart := time.Now()
		sz, ok := chunksrv.IsPresent(pn, c, dl.sz)
		dl.countPresent(time.Since(presentStart))
		if !ok {
			db.DPrintf(db.BINSRV, "read %d %d: chunk %v not present, need to fetch", off, nbyte, c)
			s := time.Now()
			sz0, err := dl.fetchChunk(c)
			if err != nil {
				return 0, err
			} else {
				sz = sz0
			}
			d := time.Since(s)
			dl.countFetch(d)
			perf.LogSpawnLatencyVerbose("BinSrv.downloader.read.fetchChunk %d sz %v", dl.p.GetPid(), dl.p.GetSpawnTime(), s, c, sz)
		}
		n += sz
		db.DPrintf(db.BINSRV, "read %q ck %d sz %d", pn, c, sz)
	}
	dl.countRead(int64(min(n-o, nbyte)))
	perf.LogSpawnLatencyVerbose("BinSrv.downloader.read nbyte %v", dl.p.GetPid(), dl.p.GetSpawnTime(), start, nbyte)
	db.DPrintf(db.BINSRV, "read done %d %d: chunks [%d,%d)", off, nbyte, i, j)
	return min(n-o, nbyte), nil
}
