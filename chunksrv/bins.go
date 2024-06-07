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
	mu       sync.Mutex
	cond     *sync.Cond
	getSC    func(sp.Trealm, *sp.SecretProto) (*sigmaclnt.SigmaClnt, error)
	fd       int
	prog     string
	realm    sp.Trealm
	s3secret *sp.SecretProto
	st       *sp.Stat
	path     string
}

func newBinEntry(prog string, realm sp.Trealm, s3secret *sp.SecretProto, getSC func(sp.Trealm, *sp.SecretProto) (*sigmaclnt.SigmaClnt, error)) *binEntry {
	return &binEntry{
		prog:     prog,
		realm:    realm,
		s3secret: s3secret,
		getSC:    getSC,
		fd:       -1,
	}
}

func (be *binEntry) signal() {
	be.mu.Lock()
	defer be.mu.Unlock()

	if be.cond != nil {
		be.cond.Broadcast()
	}
}
func (be *binEntry) getFd(paths []string) (*sigmaclnt.SigmaClnt, int, string, error) {
	// Get or create SigmaClnt using client's s3 secrets
	sc, err := be.getSC(be.realm, be.s3secret)
	if err != nil {
		return nil, -1, "", err
	}

	be.mu.Lock()
	defer be.mu.Unlock()

	if be.fd != -1 {
		return sc, be.fd, be.path, nil
	}
	s := time.Now()
	fd, path, err := open(sc, be.prog, paths)
	if err != nil {
		return sc, -1, "", err
	}
	be.fd = fd
	be.path = path
	db.DPrintf(db.SPAWN_LAT, "[%v] getFd %q open lat %v", be.prog, paths, time.Since(s))
	return sc, be.fd, path, nil
}

type progBins struct {
	bins *syncmap.SyncMap[string, *binEntry]
}

func newProgBins() *progBins {
	return &progBins{
		bins: syncmap.NewSyncMap[string, *binEntry](),
	}
}

type realmBinEntry struct {
	getSC     func(sp.Trealm, *sp.SecretProto) (*sigmaclnt.SigmaClnt, error)
	realmbins *syncmap.SyncMap[sp.Trealm, *progBins]
}

func newRealmBinEntry(getSC func(sp.Trealm, *sp.SecretProto) (*sigmaclnt.SigmaClnt, error)) *realmBinEntry {
	return &realmBinEntry{
		getSC:     getSC,
		realmbins: syncmap.NewSyncMap[sp.Trealm, *progBins](),
	}
}

func (rb *realmBinEntry) getBin(r sp.Trealm, prog string, s3secret *sp.SecretProto) *binEntry {
	re, ok := rb.realmbins.Lookup(r)
	if !ok {
		re, _ = rb.realmbins.Alloc(r, newProgBins())
	}
	be, ok := re.bins.Lookup(prog)
	if !ok {
		db.DPrintf(db.CHUNKSRV, "getBin: bin state not present r %v prog %v", r, prog)
		be, _ = re.bins.Alloc(prog, newBinEntry(prog, r, s3secret, rb.getSC))
	}
	return be
}
