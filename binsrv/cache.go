package binsrv

import (
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
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
	ust.Size = int64(sst.Length)
	ust.Blocks = int64(sst.Length/BLOCKSIZE + 1)
	ust.Atim.Sec = int64(sst.Atime)
	ust.Mtim.Sec = int64(sst.Mtime)
	ust.Ctim.Sec = int64(sst.Mtime)
	ust.Mode = 0777
	ust.Nlink = 1
	ust.Blksize = BLOCKSIZE
}

type entry struct {
	st *sp.Stat
	dl *downloader
}

type bincache struct {
	mu       sync.Mutex
	sc       *sigmaclnt.SigmaClnt
	kernelId string
	cache    map[string]*entry
	updc     *uprocclnt.UprocdClnt
}

func (bc *bincache) sStat(pn string) (*sp.Stat, error) {
	bin, paths := downloadPaths(pn, bc.kernelId)
	db.DPrintf(db.BINSRV, "sStat %q %v\n", bin, paths)

	s := time.Now()
	db.DPrintf(db.SPAWN_LAT, "[%v] sStat %v", bin, paths)

	var st *sp.Stat
	err := fslib.RetryPaths(paths, func(i int, pn string) error {
		db.DPrintf(db.BINSRV, "Stat %q/%q\n", pn, bin)
		sst, err := bc.sc.Stat(pn + "/" + bin)
		if err == nil {
			sst.Dev = uint32(i)
			st = sst
			return nil
		}
		return err
	})

	db.DPrintf(db.SPAWN_LAT, "[%v] sStat %v %v", bin, paths, time.Since(s))

	return st, err
}

func newBinCache(kernelId string, sc *sigmaclnt.SigmaClnt, updc *uprocclnt.UprocdClnt) *bincache {
	bc := &bincache{
		cache:    make(map[string]*entry),
		sc:       sc,
		kernelId: kernelId,
		updc:     updc,
	}
	return bc
}

// Check cache first. If not present, Stat file in sigmaos.
func (bc *bincache) lookup(pn string) (*sp.Stat, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	e, ok := bc.cache[pn]
	if ok {
		return e.st, nil
	}
	st, err := bc.sStat(pn)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.BINSRV, "lookup %q %v err %v\n", pn, st, err)
	bc.cache[pn] = &entry{st, nil}
	return st, nil
}

func (bc *bincache) getDownload(pn string, sz sp.Tsize) (*downloader, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	e, ok := bc.cache[pn]
	if !ok {
		db.DFatalf("getDownload %q not present", pn)
	}

	if e.dl == nil {
		db.DPrintf(db.BINSRV, "getDownload: new downloader %q\n", pn)
		if dl, err := newDownloader(pn, bc.sc, bc.updc, bc.kernelId, sz); err != nil {
			return nil, err
		} else {
			e.dl = dl
		}
	} else {
		db.DPrintf(db.BINSRV, "getDownload: %q downloader %v\n", pn, e.dl)
	}
	return e.dl, nil
}
