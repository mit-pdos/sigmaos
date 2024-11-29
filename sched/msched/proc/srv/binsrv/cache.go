package binsrv

import (
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"

	db "sigmaos/debug"
	"sigmaos/sched/msched/proc"
	sp "sigmaos/sigmap"
	"sigmaos/util/syncmap"
)

func idFromStat(st *syscall.Stat_t) fs.StableAttr {
	swapped := (uint64(st.Dev) << 32) | (uint64(st.Dev) >> 32)
	return fs.StableAttr{
		Mode: uint32(st.Mode),
		Gen:  1,
		Ino:  swapped ^ st.Ino,
	}
}

func toUstat(sst *sp.Stat, ust *syscall.Stat_t) {
	const BLOCKSIZE = 4096

	ust.Dev = uint64(sst.Dev)
	ust.Ino = sst.Qid.Path
	ust.Size = int64(sst.Tlength())
	ust.Blocks = int64(sst.Tlength()/BLOCKSIZE + 1)
	ust.Atim.Sec = int64(sst.Atime)
	ust.Mtim.Sec = int64(sst.Mtime)
	ust.Ctim.Sec = int64(sst.Mtime)
	ust.Mode = 0777
	ust.Nlink = 1
	ust.Blksize = BLOCKSIZE
}

type entry struct {
	mu sync.Mutex
	st *sp.Stat
	dl *downloader
}

type bincache struct {
	kernelId string
	cache    *syncmap.SyncMap[string, *entry]
	pds      proc.ProcSrv
}

func newBinCache(pds proc.ProcSrv) *bincache {
	bc := &bincache{
		cache: syncmap.NewSyncMap[string, *entry](),
		pds:   pds,
	}
	return bc
}

// Check cache first. If not present, get stat from procd
func (bc *bincache) lookup(pn string, pid uint32) (*sp.Stat, error) {
	e, ok := bc.cache.Lookup(pn)
	if ok {
		return e.st, nil
	}
	e, _ = bc.cache.Alloc(pn, &entry{})
	db.DPrintf(db.BINSRV, "alloc %q\n", pn)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.st == nil {
		st, err := bc.pds.Lookup(int(pid), pn)
		if err != nil {
			db.DPrintf(db.ERROR, "Error pds lookup bin: %v", err)
			return nil, err
		}
		if st == nil {
			db.DFatalf("Error st is nil from lookup pn %v pid %v", pn, pid)
		}
		e.st = st
	}
	return e.st, nil
}

func (bc *bincache) getDownload(pn string, sz sp.Tsize, pid uint32) *downloader {
	return newDownloader(pn, bc.pds, sz, pid)
}
