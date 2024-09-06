package chunksrv

import (
	"sync"

	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type bin struct {
	sync.Mutex
	cond            *sync.Cond
	condFetch       *sync.Cond
	fd              int
	prog            string
	st              *sp.Tstat
	statInProgress  bool
	openInProgress  bool
	fetchInProgress map[int]bool
	path            string
}

func newBin(prog string) *bin {
	b := &bin{
		prog:            prog,
		fd:              -1,
		fetchInProgress: make(map[int]bool),
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
func (be *bin) waitFetch(ckid int) {
	be.Lock()
	defer be.Unlock()

	for {
		_, ok := be.fetchInProgress[ckid]
		if !ok {
			be.fetchInProgress[ckid] = true
			return
		}
		// wait until outstanding fetch returns
		be.condFetch.Wait()
	}
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
