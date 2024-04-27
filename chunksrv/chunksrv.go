package chunksrv

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	"sigmaos/chunk"
	proto "sigmaos/chunk/proto"
	"sigmaos/chunkclnt"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/keys"
	"sigmaos/proc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

const (
	SEEK_DATA = 3
	SEEK_HOLE = 4

	ROOTCHUNKD = sp.SIGMAHOME + "/bin/user/realms"
)

func Index(o int64) int { return int(o / chunk.CHUNKSZ) }
func Ckoff(i int) int64 { return int64(i * chunk.CHUNKSZ) }

func BinPathChunkd(realm sp.Trealm, prog string) string {
	return path.Join(ROOTCHUNKD, realm.String(), prog)
}

func IsChunkSrvPath(path string) bool {
	return strings.Contains(path, sp.CHUNKD)
}

type ckclntEntry struct {
	mu     sync.Mutex
	ckclnt *chunkclnt.ChunkClnt
}

type ChunkSrv struct {
	sc        *sigmaclnt.SigmaClnt
	kernelId  string
	path      string
	ckclnt    *chunkclnt.ChunkClnt
	realmbins *realmBinEntry
}

func newChunkSrv(kernelId string, sc *sigmaclnt.SigmaClnt) *ChunkSrv {
	cksrv := &ChunkSrv{
		sc:        sc,
		kernelId:  kernelId,
		path:      chunk.ChunkdPath(kernelId),
		realmbins: newRealmBinEntry(),
		ckclnt:    chunkclnt.NewChunkClnt(sc.FsLib),
	}
	return cksrv
}

func (cksrv *ChunkSrv) getBin(r sp.Trealm, prog string, st *sp.Stat, paths []string) (*binEntry, error) {
	be := cksrv.realmbins.getBin(r, prog, st)
	be.mu.Lock()
	defer be.mu.Unlock()

	// Fill in stats
	if be.st == nil {
		st, err := Lookup(cksrv.sc, prog, paths)
		if err != nil {
			return nil, err
		}
		be.st = st
	}
	return be, nil
}

func (cksrv *ChunkSrv) fetchCache(req proto.FetchChunkRequest, res *proto.FetchChunkResponse) (bool, error) {
	r := sp.Trealm(req.Realm)
	ckid := int(req.ChunkId)
	reqsz := sp.Tsize(req.Size)

	pn := BinPathChunkd(r, req.Prog)
	if sz, ok := IsPresent(pn, ckid, reqsz); ok {
		b := make([]byte, sz)
		db.DPrintf(db.CHUNKSRV, "%v: FetchCache %q ckid %d present %d", cksrv.kernelId, pn, ckid, sz)
		if err := ReadChunk(pn, ckid, b); err != nil {
			return false, err
		}
		if req.Data {
			res.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
		}
		res.Size = uint64(sz)
		be, err := cksrv.getBin(r, req.GetProg(), nil, req.GetSigmaPath())
		if err != nil {
			db.DPrintf(db.ERROR, "Error fetchCache getBin: %v", err)
			return false, err
		}
		res.Stat = be.st
		return true, nil
	}
	db.DPrintf(db.CHUNKSRV, "%v: FetchCache: %q pid %v ck %d not present\n", cksrv.kernelId, pn, req.Pid, ckid)
	return false, nil
}

func (cksrv *ChunkSrv) fetchChunkd(r sp.Trealm, prog string, pid sp.Tpid, paths []string, ck int, reqsz sp.Tsize, b []byte) (sp.Tsize, *sp.Stat, error) {
	chunkdID := path.Base(paths[0])
	db.DPrintf(db.CHUNKSRV, "%v: fetchChunkd: %v ck %d %v", cksrv.kernelId, prog, ck, paths)
	sz, st, err := cksrv.ckclnt.FetchChunk(chunkdID, prog, pid, r, ck, reqsz, paths, b)
	if err != nil {
		return 0, nil, err
	}
	return sz, st, nil
}

func (cksrv *ChunkSrv) fetchOrigin(r sp.Trealm, prog string, paths []string, ck int, b []byte) (sp.Tsize, *sp.Stat, error) {
	db.DPrintf(db.CHUNKSRV, "%v: fetchOrigin: %v ck %d %v", cksrv.kernelId, prog, ck, paths)
	be, err := cksrv.getBin(r, prog, nil, paths)
	if err != nil {
		db.DPrintf(db.ERROR, "Error fetchOrigin getBin: %v", err)
		return 0, nil, err
	}
	fd, err := be.getFd(cksrv.sc, paths)
	if err != nil {
		return 0, nil, err
	}
	sz, err := cksrv.sc.Pread(fd, b, sp.Toffset(Ckoff(ck)))
	if err != nil {
		db.DPrintf(db.CHUNKSRV, "%v: FetchOrigin: read %q ck %d err %v", cksrv.kernelId, prog, ck, err)
		return 0, nil, err
	}
	return sz, be.st, nil
}

func (cksrv *ChunkSrv) fetchChunk(req proto.FetchChunkRequest, res *proto.FetchChunkResponse) error {
	sz := sp.Tsize(0)
	r := sp.Trealm(req.Realm)
	b := make([]byte, chunk.CHUNKSZ)
	ck := int(req.ChunkId)
	var st *sp.Stat
	var err error

	paths := req.SigmaPath
	if req.SigmaPath[0] == cksrv.path {
		// If the first path is me, skip myself, because i don't have
		// chunk.
		paths = req.SigmaPath[1:]
	}

	if len(paths) == 0 {
		db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: %v err %v", cksrv.kernelId, req, err)
		return serr.NewErr(serr.TErrNotfound, req.Prog)
	}

	ok := false
	for IsChunkSrvPath(paths[0]) {
		sz, st, err = cksrv.fetchChunkd(r, req.Prog, sp.Tpid(req.Pid), []string{paths[0]}, ck, sp.Tsize(req.Size), b)
		db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: chunkd %v st %v err %v", cksrv.kernelId, paths[0], st, err)
		if err == nil {
			ok = true
			// Create a bin entry, so that this chunkd won't have to fetch stats from
			// the origin if another chunkd queries it for stats
			// XXX paths messed up
			_, err := cksrv.getBin(r, req.Prog, st, paths[1:])
			if err != nil {
				db.DPrintf(db.ERROR, "Error fetchChunk getBin: %v", err)
				return err
			}
			break
		}
		paths = paths[1:]
	}

	if !ok {
		sz, st, err = cksrv.fetchOrigin(r, req.Prog, paths, ck, b)
		if err != nil {
			db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: origin %v err %v", cksrv.kernelId, paths, err)
			return err
		}
	}
	pn := BinPathChunkd(r, req.Prog)
	if err := writeChunk(pn, int(req.ChunkId), b[0:sz]); err != nil {
		db.DPrintf(db.CHUNKSRV, "fetchChunk: Writechunk %q ck %d err %v", pn, req.ChunkId, err)
		return err
	}
	db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: writeChunk %v pid %v ck %d sz %d", cksrv.kernelId, pn, req.Pid, req.ChunkId, sz)
	res.Size = uint64(sz)
	res.Stat = st
	return nil
}

func (cksrv *ChunkSrv) GetFileStat(ctx fs.CtxI, req proto.GetFileStatRequest, res *proto.GetFileStatResponse) error {
	db.DPrintf(db.CHUNKSRV, "%v: GetFileStat: %v", cksrv.kernelId, req)
	defer db.DPrintf(db.CHUNKSRV, "%v: GetFileStat done: %v", cksrv.kernelId, req)

	paths := req.GetSigmaPath()
	// Skip chunksrv paths
	for IsChunkSrvPath(paths[0]) {
		paths = paths[1:]
	}
	if len(paths) < 1 {
		return fmt.Errorf("Error no paths left")
	}

	be, err := cksrv.getBin(sp.Trealm(req.GetRealmStr()), req.GetProg(), nil, paths)
	if err != nil {
		db.DPrintf(db.ERROR, "Error getBin: %v", err)
		return err
	}
	res.Stat = be.st
	return nil
}

func (cksrv *ChunkSrv) Fetch(ctx fs.CtxI, req proto.FetchChunkRequest, res *proto.FetchChunkResponse) error {
	db.DPrintf(db.CHUNKSRV, "%v: Fetch: %v", cksrv.kernelId, req)

	//be := cksrv.getBin(r, req.Prog)
	//be.mu.Lock()
	//defer be.mu.Unlock()

	ok, err := cksrv.fetchCache(req, res)
	if ok || err != nil {
		return err
	}
	return cksrv.fetchChunk(req, res)
}

// XXX hack; how to handle ~local?
func downloadPaths(paths []string, kernelId string) []string {
	for i, p := range paths {
		if strings.HasPrefix(p, sp.UX) {
			paths[i] = strings.Replace(p, "~local", kernelId, 1)
		}
	}
	return paths
}

func Lookup(sc *sigmaclnt.SigmaClnt, prog string, paths []string) (*sp.Stat, error) {
	db.DPrintf(db.CHUNKSRV, "Lookup %q %v", prog, paths)

	var st *sp.Stat
	err := fslib.RetryPaths(paths, func(i int, pn string) error {
		db.DPrintf(db.CHUNKSRV, "Stat %q/%q", pn, prog)
		sst, err := sc.Stat(pn + "/" + prog)
		if err == nil {
			sst.Dev = uint32(i)
			st = sst
			return nil
		}
		return err
	})
	db.DPrintf(db.CHUNKSRV, "Lookup done %q %v st %v err %v", prog, paths, st, err)
	return st, err
}

func open(sc *sigmaclnt.SigmaClnt, prog string, paths []string) (int, error) {
	sfd := -1
	if err := fslib.RetryPaths(paths, func(i int, pn string) error {
		db.DPrintf(db.CHUNKSRV, "sOpen %q/%v", pn, prog)
		fd, err := sc.Open(pn+"/"+prog, sp.OREAD)
		if err == nil {
			sfd = fd
			return nil
		}
		return err
	}); err != nil {
		return sfd, err
	}
	return sfd, nil
}

func IsPresent(pn string, ck int, totsz sp.Tsize) (int64, bool) {
	f, err := os.OpenFile(pn, os.O_RDONLY, 0777)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sz := int64(0)
	ok := false
	for off := int64(0); off < int64(totsz); {
		o1, err := f.Seek(off, SEEK_DATA)
		if err != nil {
			break
		}
		o2, err := f.Seek(o1, SEEK_HOLE)
		if err != nil {
			db.DFatalf("Seek hole %q %d err %v", pn, o2, err)
		}
		for o := o1; o < o2; o += chunk.CHUNKSZ {
			if o%chunk.CHUNKSZ != 0 {
				db.DFatalf("offset %d", o)
			}
			if o+chunk.CHUNKSZ <= o2 || o2 >= int64(totsz) { // a complete chunk?
				i := Index(o)
				if i == ck {
					db.DPrintf(db.CHUNKSRV, "IsPresent: %q read chunk %d(%d) o2 %d sz %d", pn, i, o, o2, totsz)
					ok = true
					sz = chunk.CHUNKSZ
					if o+chunk.CHUNKSZ >= int64(totsz) {
						sz = int64(totsz) - o
					}
					break
				}
			}
		}
		off = o2
	}
	if sz > chunk.CHUNKSZ {
		db.DFatalf("IsPresent %d sz", sz)
	}
	return sz, ok
}

func writeChunk(pn string, ckid int, b []byte) error {
	ufd, err := os.OpenFile(pn, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		return err
	}
	defer ufd.Close()
	nn, err := ufd.WriteAt(b, Ckoff(ckid))
	if nn != len(b) {
		return err
	}
	return nil
}

func ReadChunk(pn string, ckid int, b []byte) error {
	f, err := os.OpenFile(pn, os.O_RDONLY, 0777)
	if err != nil {
		return err
	}
	defer f.Close()
	nn, err := f.ReadAt(b, Ckoff(ckid))
	if nn != len(b) {
		return err
	}
	return nil
}

func Run(kernelId string, masterPubKey auth.PublicKey, pubkey auth.PublicKey, privkey auth.PrivateKey) {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}

	kmgr := keys.NewKeyMgrWithBootstrappedKeys(
		keys.WithSigmaClntGetKeyFn[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sc),
		masterPubKey,
		nil,
		sp.Tsigner(pe.GetPID()),
		pubkey,
		privkey,
	)
	as, err := auth.NewAuthSrv[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sp.Tsigner(pe.GetPID()), sp.NOT_SET, kmgr)
	if err != nil {
		db.DFatalf("Error NewAuthSrv %v", err)
	}
	sc.SetAuthSrv(as)
	cksrv := newChunkSrv(kernelId, sc)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(path.Join(sp.CHUNKD, sc.ProcEnv().GetKernelID()), sc, cksrv)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	// Start monitoring for other chunkds, and establishing clients to them
	cksrv.ckclnt.UpdateChunkds()
	ssrv.RunServer()
}
