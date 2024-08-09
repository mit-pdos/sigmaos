package binsrv

import (
	"sync"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
	"sigmaos/uprocclnt"
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

func toUstatfs(sst *sp.Stat, ust *syscall.Statfs_t) {
	const BLOCKSIZE = 4096

	ust.Bsize = BLOCKSIZE
	ust.Blocks = uint64(sst.Tlength()/BLOCKSIZE + 1)
}

type entry struct {
	mu sync.Mutex
	st *sp.Stat
	dl *downloader
}

type bincache struct {
	kernelId string
	cache    *syncmap.SyncMap[string, *entry]
	upds     uprocclnt.UprocSrv
}

func newBinCache(upds uprocclnt.UprocSrv) *bincache {
	bc := &bincache{
		cache: syncmap.NewSyncMap[string, *entry](),
		upds:  upds,
	}
	return bc
}

// Check cache first. If not present, get stat from uprocd
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
		st, err := bc.upds.Lookup(int(pid), pn)
		if err != nil {
			db.DPrintf(db.ERROR, "Error upds lookup bin: %v", err)
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
	return newDownloader(pn, bc.upds, sz, pid)
}
