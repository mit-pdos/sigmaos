package chunksrv

import (
	"fmt"
	"sync"

	// db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type bin struct {
	sync.Mutex
	cond           *sync.Cond
	fd             int
	prog           string
	st             *sp.Stat
	statInProgress bool
	openInProgress bool
	path           string
	sc             *sigmaclnt.SigmaClnt
}

func newBin(prog string) *bin {
	b := &bin{
		prog: prog,
		fd:   -1,
	}
	b.cond = sync.NewCond(&b.Mutex)
	return b
}

func (be *bin) getStat() *sp.Stat {
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

func (be *bin) getFd(sc *sigmaclnt.SigmaClnt, paths []string) (int, string) {
	be.Lock()
	defer be.Unlock()

	for {
		if be.fd != -1 {
			return be.fd, be.path
		}
		if !be.openInProgress {
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

func (rs *realms) getSc(realm sp.Trealm) (*sigmaclnt.SigmaClnt, error) {
	r, ok := rs.Lookup(realm)
	if !ok {
		return nil, fmt.Errorf("No sc for realm %v", realm)
	}
	return r.getSc(), nil
}

func (rs *realms) InitRoot(sc *sigmaclnt.SigmaClnt) {
	r, _ := rs.AllocNew(sp.ROOTREALM, newRealm)
	r.sc = sc
}
