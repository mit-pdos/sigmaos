package chunksrv

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	"sigmaos/syncmap"
)

const (
	CHUNKSZ = 1 * sp.MBYTE

	SEEK_DATA = 3
	SEEK_HOLE = 4

	ROOTCHUNKD = sp.SIGMAHOME + "/bin/user/realms"
)

func Index(o int64) int { return int(o / CHUNKSZ) }
func Ckoff(i int) int64 { return int64(i * CHUNKSZ) }

//func BinPathUprocd(realm sp.Trealm, prog string) string {
//	return path.Join(ROOTPROCD, realm.String(), prog)
//}

func BinPathChunkd(realm sp.Trealm, prog string) string {
	return path.Join(ROOTCHUNKD, realm.String(), prog)
}

func IsChunkSrvPath(path string) bool {
	return strings.Contains(path, sp.CHUNKD)
}

type binEntry struct {
	mu    sync.Mutex
	cond  *sync.Cond
	fd    int
	prog  string
	realm sp.Trealm
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

type ckclntEntry struct {
	mu     sync.Mutex
	ckclnt *chunkclnt.ChunkClnt
}

type ChunkSrv struct {
	sc       *sigmaclnt.SigmaClnt
	kernelId string
	path     string
	ckclnts  *syncmap.SyncMap[string, *ckclntEntry]
	bins     *syncmap.SyncMap[string, *binEntry]
}

func newChunkSrv(kernelId string, sc *sigmaclnt.SigmaClnt) *ChunkSrv {
	cksrv := &ChunkSrv{
		sc:       sc,
		kernelId: kernelId,
		path:     chunk.ChunkdPath(kernelId),
		ckclnts:  syncmap.NewSyncMap[string, *ckclntEntry](),
		bins:     syncmap.NewSyncMap[string, *binEntry](),
	}
	return cksrv
}

func (cksrv *ChunkSrv) getClnt(pn string) (*chunkclnt.ChunkClnt, error) {
	e, ok := cksrv.ckclnts.Lookup(pn)
	if ok {
		return e.ckclnt, nil
	}
	e, _ = cksrv.ckclnts.Alloc(pn, &ckclntEntry{})
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ckclnt == nil {
		ckclnt, err := chunkclnt.NewChunkClnt(cksrv.sc.FsLib, pn)
		if err != nil {
			return nil, err
		}
		e.ckclnt = ckclnt
	}
	return e.ckclnt, nil
}

func (cksrv *ChunkSrv) getBin(r sp.Trealm, prog string) *binEntry {
	pn := filepath.Join(r.String(), prog)
	be, ok := cksrv.bins.Lookup(pn)
	if ok {
		return be
	}
	be, _ = cksrv.bins.Alloc(pn, &binEntry{prog: prog, realm: r, fd: -1})
	return be
}

func (cksrv *ChunkSrv) fetchCache(req proto.FetchChunkRequest, res *proto.FetchChunkResponse) (bool, error) {
	r := sp.Trealm(req.Realm)
	ckid := int(req.ChunkId)
	reqsz := sp.Tsize(req.Size)

	pn := BinPathChunkd(r, req.Prog)
	if sz, ok := IsPresent(pn, ckid, reqsz); ok {
		if sz > CHUNKSZ {
			sz = CHUNKSZ
		}
		b := make([]byte, sz)
		db.DPrintf(db.CHUNKSRV, "%v: FetchCache %q ckid %d present %d", cksrv.kernelId, pn, ckid, sz)
		if err := ReadChunk(pn, ckid, b); err != nil {
			return false, err
		}
		if req.Data {
			res.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
		}
		res.Size = uint64(sz)
		return true, nil
	}
	db.DPrintf(db.CHUNKSRV, "%v: FetchCache: %q pid %v ck %d not present\n", cksrv.kernelId, pn, req.Pid, ckid)
	return false, nil
}

func (cksrv *ChunkSrv) fetchChunkd(r sp.Trealm, prog, pid string, paths []string, ck int, reqsz sp.Tsize, b []byte) (sp.Tsize, error) {
	clnt, err := cksrv.getClnt(paths[0])
	if err != nil {
		return 0, err
	}
	db.DPrintf(db.CHUNKSRV, "%v: fetchChunkd: %v ck %d %v", cksrv.kernelId, prog, ck, paths)
	sz, err := clnt.FetchChunk(prog, pid, r, ck, reqsz, paths, b)
	if err != nil {
		return 0, err
	}
	return sz, nil
}

func (cksrv *ChunkSrv) fetchOrigin(r sp.Trealm, prog string, paths []string, ck int, b []byte) (sp.Tsize, error) {
	db.DPrintf(db.CHUNKSRV, "%v: fetchOrigin: %v ck %d %v", cksrv.kernelId, prog, ck, paths)
	be := cksrv.getBin(r, prog)
	fd, err := be.getFd(cksrv.sc, paths)
	if err != nil {
		return 0, err
	}
	sz, err := cksrv.sc.Pread(fd, b, sp.Toffset(Ckoff(ck)))
	if err != nil {
		db.DPrintf(db.CHUNKSRV, "%v: FetchOrigin: read %q ck %d err %v", cksrv.kernelId, prog, ck, err)
		return 0, err
	}
	return sz, nil
}

func (cksrv *ChunkSrv) fetchChunk(req proto.FetchChunkRequest, res *proto.FetchChunkResponse) error {
	sz := sp.Tsize(0)
	r := sp.Trealm(req.Realm)
	b := make([]byte, CHUNKSZ)
	ck := int(req.ChunkId)
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
		sz, err = cksrv.fetchChunkd(r, req.Prog, req.Pid, []string{paths[0]}, ck, sp.Tsize(req.Size), b)
		if err == nil {
			ok = true
			break
		}
		db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: chunkd %v err %v", cksrv.kernelId, paths[0], err)
		paths = paths[1:]
	}

	if !ok {
		sz, err = cksrv.fetchOrigin(r, req.Prog, paths, ck, b)
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
	db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: writeChunk %v pid %v ck %d", cksrv.kernelId, pn, req.Pid, req.ChunkId)
	res.Size = uint64(sz)
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
	db.DPrintf(db.CHUNKSRV, "Lookup %q %v\n", prog, paths)

	var st *sp.Stat
	err := fslib.RetryPaths(paths, func(i int, pn string) error {
		db.DPrintf(db.CHUNKSRV, "Stat %q/%q\n", pn, prog)
		sst, err := sc.Stat(pn + "/" + prog)
		if err == nil {
			sst.Dev = uint32(i)
			st = sst
			return nil
		}
		return err
	})
	return st, err
}

func open(sc *sigmaclnt.SigmaClnt, prog string, paths []string) (int, error) {
	sfd := -1
	if err := fslib.RetryPaths(paths, func(i int, pn string) error {
		db.DPrintf(db.CHUNKSRV, "sOpen %q/%v\n", pn, prog)
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
			db.DFatalf("Seek hole %q %d err %v\n", pn, o2, err)
		}
		for o := o1; o < o2; o += CHUNKSZ {
			if o%CHUNKSZ != 0 {
				db.DFatalf("offset %d\n", o)
			}
			if o+CHUNKSZ <= o2 || o2 >= int64(totsz) { // a complete chunk?
				i := Index(o)
				if i == ck {
					db.DPrintf(db.CHUNKSRV, "readLocal: %q read chunk %d(%d)\n", pn, i, o)
					ok = true
					sz = CHUNKSZ
					if o2 >= int64(totsz) {
						sz = int64(totsz) - o1
					}
					break
				}
			}
		}
		off = o2
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
	ssrv.RunServer()
}
