// package chunksrv caches chunks of binaries and the binary's Stat
// for procs.  It uses the proc's SigmaPath paths to locate the
// binary, if it doesn't have it cached: typically the last entry in
// SigmaPath is the origin for the binary (e.g., an S3 path) and
// earlier entries are paths to other chunksrvs (e.g., prepended by
// procclnt or procqsrv).
package chunksrv

import (
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

	ROOTHOSTCACHE = "/tmp/sigmaos-bin"

	// start-kernel.sh mounts /tmp/sigmaos-bin/${KERNELID} as
	// ROOTBINCACHE, given each kernel its own directory in the local
	// file system to cache binaries.  chunksrv runs as part of the
	// kernel and uses this pathname to cache binaries.
	ROOTBINCACHE = sp.SIGMAHOME + "/bin/user/realms"

	// The kernel's scheduler starts a uprocsrv container per realm
	// and proc type and mounts "/tmp/sigmaos-bin/<kernelid>" at
	// ROOTBINCONTAINER for uprocsrv.
	ROOTBINCONTAINER = sp.SIGMAHOME + "all-realm-bin"

	// The directory ROOTBINCONTAINER/<realm> is mounted here by
	// uprocsrv:
	BINPROC = sp.SIGMAHOME + "/bin/user/"
)

func Index(o int64) int { return int(o / chunk.CHUNKSZ) }
func Ckoff(i int) int64 { return int64(i * chunk.CHUNKSZ) }

func IsChunkSrvPath(path string) bool {
	return strings.Contains(path, sp.CHUNKD)
}

// The path on the host where a kernel caches its binaries
func PathHostKernel(kernelId string) string {
	return path.Join(ROOTHOSTCACHE, kernelId)
}

// For testing: the path on the host where a kernel caches its
// binaries for a realm.
func PathHostKernelRealm(kernelId string, realm sp.Trealm) string {
	return path.Join(PathHostKernel(kernelId), realm.String())
}

// The path where chunksrv caches binaries in the local file system.
func pathBinCache(realm sp.Trealm, prog string) string {
	return path.Join(ROOTBINCACHE, realm.String(), prog)
}

// The pathname that uprocsrv uses for the directory with cached
// binaries.
func pathBinRealm(realm sp.Trealm) string {
	return path.Join(ROOTBINCONTAINER, realm.String())
}

// Uprocsrv mounts PathBinRealm at PathBinProc() providing a proc
// accesses to its binaries through PathBinProc().
func MkPathBinRealm(realm sp.Trealm) string {
	dir := pathBinRealm(realm)
	// fails is already exist and if it fails for another reason Mount will fail
	if err := os.Mkdir(dir, 0750); err != nil {
		db.DPrintf(db.CHUNKSRV, "Mkdir %q err %v\n", dir, err)
	}
	return dir
}

func PathBinProc() string {
	return BINPROC
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

func (cksrv *ChunkSrv) getBin(r sp.Trealm, prog string) *binEntry {
	be := cksrv.realmbins.getBin(r, prog)
	return be
}

func (cksrv *ChunkSrv) fetchCache(req proto.FetchChunkRequest, res *proto.FetchChunkResponse) (bool, error) {
	r := sp.Trealm(req.Realm)
	ckid := int(req.ChunkId)
	reqsz := sp.Tsize(req.Size)

	pn := pathBinCache(r, req.Prog)
	if sz, ok := IsPresent(pn, ckid, reqsz); ok {
		b := make([]byte, sz)
		db.DPrintf(db.CHUNKSRV, "%v: FetchCache %q ckid %d hit %d", cksrv.kernelId, pn, ckid, sz)
		if err := ReadChunk(pn, ckid, b); err != nil {
			return false, err
		}
		if req.Data {
			res.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
		}
		res.Size = uint64(sz)
		return true, nil
	}
	db.DPrintf(db.CHUNKSRV, "%v: FetchCache: %q pid %v ckid %d not present\n", cksrv.kernelId, pn, req.Pid, ckid)
	return false, nil
}

func (cksrv *ChunkSrv) fetchChunkd(r sp.Trealm, prog string, pid sp.Tpid, paths []string, ck int, reqsz sp.Tsize, b []byte) (sp.Tsize, error) {
	chunkdID := path.Base(paths[0])
	db.DPrintf(db.CHUNKSRV, "%v: fetchChunkd: %v ckid %d %v", cksrv.kernelId, prog, ck, paths)
	sz, err := cksrv.ckclnt.FetchChunk(chunkdID, prog, pid, r, ck, reqsz, paths, b)
	if err != nil {
		return 0, err
	}
	return sz, nil
}

func (cksrv *ChunkSrv) fetchOrigin(r sp.Trealm, prog string, paths []string, ck int, b []byte) (sp.Tsize, error) {
	db.DPrintf(db.CHUNKSRV, "%v: fetchOrigin: %v ckid %d %v", cksrv.kernelId, prog, ck, paths)
	be := cksrv.getBin(r, prog)
	fd, err := be.getFd(cksrv.sc, paths)
	if err != nil {
		return 0, err
	}
	sz, err := cksrv.sc.Pread(fd, b, sp.Toffset(Ckoff(ck)))
	if err != nil {
		db.DPrintf(db.CHUNKSRV, "%v: FetchOrigin: read %q ckid %d err %v", cksrv.kernelId, prog, ck, err)
		return 0, err
	}
	return sz, nil
}

func (cksrv *ChunkSrv) fetchChunk(req proto.FetchChunkRequest, res *proto.FetchChunkResponse) error {
	sz := sp.Tsize(0)
	r := sp.Trealm(req.Realm)
	b := make([]byte, chunk.CHUNKSZ)
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
		sz, err = cksrv.fetchChunkd(r, req.Prog, sp.Tpid(req.Pid), []string{paths[0]}, ck, sp.Tsize(req.Size), b)
		db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: chunkd %v err %v", cksrv.kernelId, paths[0], err)
		if err == nil {
			ok = true
			break
		}
		paths = paths[1:]
	}

	if !ok {
		sz, err = cksrv.fetchOrigin(r, req.Prog, paths, ck, b)
		if err != nil {
			db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: origin %v err %v", cksrv.kernelId, paths, err)
			return err
		}
	}
	pn := pathBinCache(r, req.Prog)
	if err := writeChunk(pn, int(req.ChunkId), b[0:sz]); err != nil {
		db.DPrintf(db.CHUNKSRV, "fetchChunk: Writechunk %q ckid %d err %v", pn, req.ChunkId, err)
		return err
	}
	db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: writeChunk %v pid %v ckid %d sz %d", cksrv.kernelId, pn, req.Pid, req.ChunkId, sz)
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

func (cksrv *ChunkSrv) getStatCache(req proto.GetFileStatRequest, res *proto.GetFileStatResponse) (*sp.Stat, bool) {
	be := cksrv.getBin(sp.Trealm(req.GetRealmStr()), req.GetProg())
	be.mu.Lock()
	defer be.mu.Unlock()
	if be.st == nil {
		return nil, false
	}
	db.DPrintf(db.CHUNKSRV, "%v: getStatCache: hit %v", cksrv.kernelId, req.GetProg())
	return be.st, true
}

func (cksrv *ChunkSrv) getFileStat(req proto.GetFileStatRequest, res *proto.GetFileStatResponse) error {
	r := sp.Trealm(req.GetRealmStr())
	paths := req.GetSigmaPath()
	if req.SigmaPath[0] == cksrv.path {
		// If the first path is me, skip myself, because i don't have the stat
		paths = req.SigmaPath[1:]
	}

	if len(paths) == 0 {
		return serr.NewErr(serr.TErrNotfound, req.Prog)
	}

	ok := false
	var st *sp.Stat
	var err error
	for IsChunkSrvPath(paths[0]) {
		srv := path.Base(paths[0])
		st, err = cksrv.ckclnt.GetFileStat(srv, req.Prog, sp.Tpid(req.Pid), r, []string{paths[0]})
		db.DPrintf(db.CHUNKSRV, "%v: GetFileStat: chunkd %v st %v err %v", cksrv.kernelId, paths[0], st, err)
		if err == nil {
			ok = true
			break
		}
		paths = paths[1:]
	}
	if !ok {
		st, err = cksrv.getOrigin(r, req.Prog, paths)
		if err != nil {
			db.DPrintf(db.CHUNKSRV, "%v: getFileStat: origin %v err %v", cksrv.kernelId, paths, err)
			return err
		}
	}
	db.DPrintf(db.CHUNKSRV, "%v: getFileStat pid %v st %v", cksrv.kernelId, req.Pid, st)
	be := cksrv.getBin(r, req.GetProg())
	be.mu.Lock()
	defer be.mu.Unlock()
	be.st = st
	res.Stat = be.st
	return nil
}

func (cksrv *ChunkSrv) getOrigin(r sp.Trealm, prog string, paths []string) (*sp.Stat, error) {
	st, err := lookup(cksrv.sc, prog, paths)
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (cksrv *ChunkSrv) GetFileStat(ctx fs.CtxI, req proto.GetFileStatRequest, res *proto.GetFileStatResponse) error {
	db.DPrintf(db.CHUNKSRV, "%v: GetFileStat: %v", cksrv.kernelId, req)
	defer db.DPrintf(db.CHUNKSRV, "%v: GetFileStat done: %v", cksrv.kernelId, req)

	st, ok := cksrv.getStatCache(req, res)
	if ok {
		res.Stat = st
		return nil
	}
	return cksrv.getFileStat(req, res)
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

func lookup(sc *sigmaclnt.SigmaClnt, prog string, paths []string) (*sp.Stat, error) {
	db.DPrintf(db.CHUNKSRV, "lookup %q %v", prog, paths)

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
	db.DPrintf(db.CHUNKSRV, "lookup done %q %v st %v err %v", prog, paths, st, err)
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
