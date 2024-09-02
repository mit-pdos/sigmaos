package chunksrv

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type fetchInfo struct {
	paths []string // Paths to try fetching from
}

func newFetchInfo(paths []string) *fetchInfo {
	return &fetchInfo{
		paths: paths,
	}
}

type bin struct {
	sync.Mutex
	cond            *sync.Cond
	condFetch       *sync.Cond
	fd              int
	prog            string
	st              *sp.Tstat
	statInProgress  bool
	openInProgress  bool
	fetchInProgress map[int]*fetchInfo
	path            string
}

func newBin(prog string) *bin {
	b := &bin{
		prog:            prog,
		fd:              -1,
		fetchInProgress: make(map[int]*fetchInfo),
	}
	b.cond = sync.NewCond(&b.Mutex)
	b.condFetch = sync.NewCond(&b.Mutex)
	return b
}

func (be *bin) isStatCached() (*sp.Tstat, bool) {
	be.Lock()
	defer be.Unlock()

	return be.st, be.st != nil
}

// Caller should stat file if getStat() returns nil
func (be *bin) waitStat() *sp.Tstat {
	be.Lock()
	defer be.Unlock()

	for {
		if be.st != nil {
			return be.st
		}
		if !be.statInProgress {
			be.statInProgress = true
			return nil
		}
		be.cond.Wait()
	}
	return be.st
}

func (be *bin) signalStatWaiters() {
	be.Lock()
	defer be.Unlock()

	be.statInProgress = false
	be.cond.Broadcast()
}

// Caller should open file if getFd returns -1
func (be *bin) getFd(sc *sigmaclnt.SigmaClnt, paths []string) (int, string) {
	be.Lock()
	defer be.Unlock()

	for {
		if be.fd != -1 {
			return be.fd, be.path
		}
		if !be.openInProgress {
			be.openInProgress = true
			return -1, ""
		}
		be.cond.Wait()
	}
}

func (be *bin) signalFdWaiters() {
	be.Lock()
	defer be.Unlock()

	be.openInProgress = false
	be.cond.Broadcast()
}

// If returns true, fetch should skip straight to origin
func (be *bin) waitFetch(fetcherPath string, paths []string, ckid int) bool {
	be.Lock()
	defer be.Unlock()

	for {
		fi, ok := be.fetchInProgress[ckid]
		if !ok {
			be.fetchInProgress[ckid] = newFetchInfo(paths)
			return false
		} else {
			if len(fi.paths) >= 2 && fi.paths[0] == fetcherPath {
				db.DPrintf(db.CHUNKSRV, "Fetcher %v circular fetch paths %v", fetcherPath, fi.paths)
				// Circular fetch. Fetcher is waiting for a fetch, which is in turn
				// fetching from the fetcher, so, bail out and fetch from origin.
				return true
			}
		}
		// wait until outstanding fetch returns
		be.condFetch.Wait()
	}
	return false
}

func (be *bin) signalFetchWaiters(ckid int) {
	be.Lock()
	defer be.Unlock()

	delete(be.fetchInProgress, ckid)
	be.condFetch.Broadcast()
}

type realm struct {
	sync.Mutex
	sc       *sigmaclnt.SigmaClnt
	s3secret *sp.SecretProto
	bins     *syncmap.SyncMap[string, *bin]
}

func (r *realm) getSc() *sigmaclnt.SigmaClnt {
	return r.sc
}

func newRealm(r sp.Trealm) *realm {
	return &realm{
		bins: syncmap.NewSyncMap[string, *bin](),
	}
}

type realms struct {
	*syncmap.SyncMap[sp.Trealm, *realm]
}

func newRealms() *realms {
	return &realms{
		SyncMap: syncmap.NewSyncMap[sp.Trealm, *realm](),
	}
}

func (rs *realms) InitRoot(sc *sigmaclnt.SigmaClnt) {
	r, _ := rs.AllocNew(sp.ROOTREALM, newRealm)
	r.sc = sc
}
