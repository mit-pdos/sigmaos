package chunksrv

import (
	"fmt"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type bin struct {
	sync.Mutex
	cond *sync.Cond
	fd   int
	prog string
	st   *sp.Stat
	path string
	sc   *sigmaclnt.SigmaClnt
}

func newBin(prog string) *bin {
	return &bin{
		prog: prog,
		fd:   -1,
	}
}

func (be *bin) getStat() (*sp.Tstat, bool) {
	be.Lock()
	defer be.Unlock()
	if be.st == nil {
		return nil, false
	}
	return be.st, true
}

func (be *bin) signal() {
	be.Lock()
	defer be.Unlock()

	if be.cond != nil {
		be.cond.Broadcast()
	}
}

func (be *bin) getFd(sc *sigmaclnt.SigmaClnt, paths []string) (int, string, error) {
	be.Lock()
	defer be.Unlock()

	if be.fd != -1 {
		return be.fd, be.path, nil
	}
	s := time.Now()
	fd, path, err := open(sc, be.prog, paths)
	if err != nil {
		return -1, "", err
	}
	be.fd = fd
	be.path = path
	db.DPrintf(db.SPAWN_LAT, "[%v] getFd %q open lat %v", be.prog, paths, time.Since(s))
	return be.fd, path, nil
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
