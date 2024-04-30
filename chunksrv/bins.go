package chunksrv

import (
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type binEntry struct {
	mu    sync.Mutex
	cond  *sync.Cond
	fd    int
	prog  string
	realm sp.Trealm
	st    *sp.Stat
}

func newBinEntry(prog string, realm sp.Trealm) *binEntry {
	return &binEntry{
		prog:  prog,
		realm: realm,
		fd:    -1,
	}
}

func (be *binEntry) signal() {
	be.mu.Lock()
	defer be.mu.Unlock()

	if be.cond != nil {
		be.cond.Broadcast()
	}
}
func (be *binEntry) getFd(sc *sigmaclnt.SigmaClnt, paths []string) (int, error) {
	be.mu.Lock()
	defer be.mu.Unlock()
	if be.fd != -1 {
		return be.fd, nil
	}
	s := time.Now()
	fd, err := open(sc, be.prog, paths)
	if err != nil {
		return -1, err
	}
	be.fd = fd
	db.DPrintf(db.SPAWN_LAT, "[%v] getFd %q spawn %v", be.prog, paths, time.Since(s))
	return be.fd, nil
}

type progBins struct {
	bins *syncmap.SyncMap[string, *binEntry]
}

func newProgBins() *progBins {
	return &progBins{bins: syncmap.NewSyncMap[string, *binEntry]()}
}

type realmBinEntry struct {
	realmbins *syncmap.SyncMap[sp.Trealm, *progBins]
}

func newRealmBinEntry() *realmBinEntry {
	return &realmBinEntry{realmbins: syncmap.NewSyncMap[sp.Trealm, *progBins]()}
}

func (rb *realmBinEntry) getBin(r sp.Trealm, prog string) *binEntry {
	re, ok := rb.realmbins.Lookup(r)
	if !ok {
		re, _ = rb.realmbins.Alloc(r, newProgBins())
	}
	be, ok := re.bins.Lookup(prog)
	if !ok {
		db.DPrintf(db.CHUNKSRV, "getBin: bin state not present r %v prog %v", r, prog)
		be, _ = re.bins.Alloc(prog, newBinEntry(prog, r))
	}
	return be
}
