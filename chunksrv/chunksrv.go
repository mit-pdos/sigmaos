// package chunksrv caches chunks of binaries and the binary's Stat
// for procs.  It uses the proc's SigmaPath paths to locate the
// binary, if it doesn't have it cached: typically the last entry in
// SigmaPath is the origin for the binary (e.g., an S3 path) and
// earlier entries are paths to other chunksrvs (e.g., prepended by
// procclnt or procqsrv).
package chunksrv

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sigmaos/chunk"
	proto "sigmaos/chunk/proto"
	"sigmaos/chunkclnt"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslib"
	"sigmaos/netproxyclnt"
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

func Index(o int64) int     { return int(o / chunk.CHUNKSZ) }
func Ckoff(i int) int64     { return int64(i * chunk.CHUNKSZ) }
func CkRound(o int64) int64 { return (o + chunk.CHUNKSZ - 1) &^ (chunk.CHUNKSZ - 1) }

func IsChunkSrvPath(path string) bool {
	return strings.Contains(path, sp.CHUNKD)
}

// The path on the host where a kernel caches its binaries
func PathHostKernel(kernelId string) string {
	return filepath.Join(ROOTHOSTCACHE, kernelId)
}

// For testing: the path on the host where a kernel caches its
// binaries for a realm.
func PathHostKernelRealm(kernelId string, realm sp.Trealm) string {
	return filepath.Join(PathHostKernel(kernelId), realm.String())
}

// The path where chunksrv caches binaries in the local file system.
func pathBinCache(realm sp.Trealm, prog string) string {
	return filepath.Join(ROOTBINCACHE, realm.String(), prog)
}

// The pathname that uprocsrv uses for the directory with cached
// binaries.
func pathBinRealm(realm sp.Trealm) string {
	return filepath.Join(ROOTBINCONTAINER, realm.String())
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
	mu       sync.Mutex
	sc       *sigmaclnt.SigmaClnt
	kernelId string
	path     string
	ckclnt   *chunkclnt.ChunkClnt
	realms   *realms
}

func newChunkSrv(kernelId string, sc *sigmaclnt.SigmaClnt) *ChunkSrv {
	cksrv := &ChunkSrv{
		sc:       sc,
		kernelId: kernelId,
		path:     chunk.ChunkdPath(kernelId),
		ckclnt:   chunkclnt.NewChunkClnt(sc.FsLib, true),
	}
	cksrv.realms = newRealms()
	cksrv.realms.InitRoot(sc)
	return cksrv
}

// Create a new sigmaclnt for a realm, with given s3 secrets
// XXX sigmaclnt is overkill; we only need fslib
func (cksrv *ChunkSrv) getRealmSigmaClnt(r sp.Trealm, s3secret *sp.SecretProto) (*sigmaclnt.SigmaClnt, error) {
	db.DPrintf(db.CHUNKSRV, "%v: Create SigmaClnt for realm %v", cksrv.kernelId, r)
	// Create a new proc env for the client
	pe := proc.NewDifferentRealmProcEnv(cksrv.sc.ProcEnv(), r)

	// Set the secrets to match those passed in by the user
	pe.SetSecrets(map[string]*sp.SecretProto{"s3": s3secret})
	// Create a sigmaclnt but only with an FsLib
	sc, err := sigmaclnt.NewSigmaClntFsLib(pe, netproxyclnt.NewNetProxyClnt(pe))
	if err != nil {
		db.DPrintf(db.ERROR, "Error create SigmaClnt: %v", err)
		return nil, err
	}
	return sc, nil
}

// Get binary info of prog in realm. Creates a SigmaClnt for realm, if unknown realm.
func (cksrv *ChunkSrv) getBin(realm sp.Trealm, prog string, s3secret *sp.SecretProto) (*bin, error) {
	r, _ := cksrv.realms.AllocNew(realm, newRealm)

	r.Lock()
	defer r.Unlock()

	if r.sc == nil {
		sc, err := cksrv.getRealmSigmaClnt(realm, s3secret)
		if err != nil {
			db.DPrintf(db.ERROR, "Error create SigmaClnt: %v", err)
			return nil, err
		}
		r.sc = sc
	}
	b, _ := r.bins.AllocNew(prog, newBin)
	b.Lock()
	defer b.Unlock()
	b.sc = r.sc
	return b, nil
}

//
// Handle a FetchChunkRequest
//

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
		res.Path = cksrv.path
		return true, nil
	}
	db.DPrintf(db.CHUNKSRV, "%v: FetchCache: %q pid %v ckid %d not present\n", cksrv.kernelId, pn, req.Pid, ckid)
	return false, nil
}

func (cksrv *ChunkSrv) fetchOrigin(r sp.Trealm, prog string, s3secret *sp.SecretProto, ck int, paths []string, b []byte) (sp.Tsize, string, error) {
	db.DPrintf(db.CHUNKSRV, "%v: fetchOrigin: %v ckid %d %v", cksrv.kernelId, prog, ck, paths)
	be, err := cksrv.getBin(r, prog, s3secret)
	if err != nil {
		return 0, "", err
	}
	sc, err := cksrv.realms.getSc(r)
	if err != nil {
		return 0, "", err
	}
	// paths = replaceLocal(paths, cksrv.kernelId)
	fd, path := be.getFd(sc, paths)
	if fd == -1 {
		defer be.signalFdWaiters()
		s := time.Now()
		fd0, path0, err := open(sc, be.prog, paths)
		if err != nil {
			return 0, "", err
		}
		be.fd = fd0
		be.path = path0
		fd = fd0
		path = path0
		db.DPrintf(db.SPAWN_LAT, "[%v] getFd %q open lat %v", be.prog, paths, time.Since(s))
	}
	sz, err := sc.Pread(fd, b, sp.Toffset(Ckoff(ck)))
	if err != nil {
		db.DPrintf(db.CHUNKSRV, "%v: FetchOrigin: read %q ckid %d err %v", cksrv.kernelId, prog, ck, err)
		return 0, "", err
	}
	return sz, path, nil
}

func (cksrv *ChunkSrv) fetchChunk(r sp.Trealm, prog string, pid sp.Tpid, s3secret *sp.SecretProto, ck int, size sp.Tsize, paths []string) (sp.Tsize, string, error) {
	sz := sp.Tsize(0)
	b := make([]byte, chunk.CHUNKSZ)
	var err error

	if paths[0] == cksrv.path {
		// If the first path is me, skip myself, because i don't have
		// chunk.
		paths = paths[1:]
	}

	if len(paths) == 0 {
		db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: %v err %v", cksrv.kernelId, prog, err)
		return 0, "", serr.NewErr(serr.TErrNotfound, prog)
	}

	ok := false
	srvpath := ""
	for IsChunkSrvPath(paths[0]) {
		srvpath = paths[0]
		srv := filepath.Base(srvpath)
		db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: %v ckid %d %v", cksrv.kernelId, prog, ck, []string{srvpath})
		sz, _, err = cksrv.ckclnt.FetchChunk(srv, prog, pid, r, s3secret, ck, size, []string{}, b)
		db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: %v ckid %d %v err %v", cksrv.kernelId, prog, ck, []string{srvpath}, err)
		if err == nil {
			ok = true
			break
		}
		paths = paths[1:]
	}

	if !ok {
		if len(paths) == 0 {
			db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: %v err %v", cksrv.kernelId, prog, err)
			return 0, "", serr.NewErr(serr.TErrNotfound, prog)
		}
		sz, srvpath, err = cksrv.fetchOrigin(r, prog, s3secret, ck, paths, b)
		if err != nil {
			db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: origin %v err %v", cksrv.kernelId, paths, err)
			return 0, "", err
		}
	}
	pn := pathBinCache(r, prog)
	if err := writeChunk(pn, ck, b[0:sz]); err != nil {
		db.DPrintf(db.CHUNKSRV, "fetchChunk: Writechunk %q ckid %d err %v", pn, ck, err)
		return 0, "", err
	}
	db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: writeChunk %v pid %v ckid %d sz %d", cksrv.kernelId, pn, pid, ck, sz)
	return sz, srvpath, nil
}

func (cksrv *ChunkSrv) Fetch(ctx fs.CtxI, req proto.FetchChunkRequest, res *proto.FetchChunkResponse) error {
	db.DPrintf(db.CHUNKSRV, "%v: Fetch: %v", cksrv.kernelId, req)

	r := sp.Trealm(req.GetRealm())
	be, err := cksrv.getBin(r, req.GetProg(), req.GetS3Secret())
	if err != nil {
		return err
	}

	if st, ok := be.isStatCached(); ok {
		db.DPrintf(db.CHUNKSRV, "%v: Fetch: hit stat %v %v %d", cksrv.kernelId, req.GetProg(), req.GetPid(), st.Length)
		if uint64(Ckoff(int(req.ChunkId)))+req.Size > st.Length {
			req.Size = st.Length
		}
	}

	ok, err := cksrv.fetchCache(req, res)
	if ok || err != nil {
		return err
	}

	if len(req.GetSigmaPath()) == 0 {
		db.DPrintf(db.CHUNKSRV, "%v: Fetch: %v ok %t err %v", cksrv.kernelId, req.Prog, ok, err)
		return serr.NewErr(serr.TErrNotfound, req.Prog)
	}

	sz, srvpath, err := cksrv.fetchChunk(sp.Trealm(req.Realm), req.Prog, sp.Tpid(req.Pid), req.GetS3Secret(), int(req.ChunkId), sp.Tsize(req.Size), req.SigmaPath)
	if err != nil {
		return err
	}
	res.Size = uint64(sz)
	res.Path = srvpath
	return nil
}

//
// Handle a GetFileStatRequest
//

func (cksrv *ChunkSrv) getOrigin(r sp.Trealm, prog string, paths []string, s3secret *sp.SecretProto) (*sp.Stat, string, error) {
	db.DPrintf(db.CHUNKSRV, "%v: getOrigin %v %v", cksrv.kernelId, prog, paths)
	sc, err := cksrv.realms.getSc(r)
	if err != nil {
		db.DPrintf(db.ERROR, "Error get realm (%v) sigma clnt: %v", r, err)
		return nil, "", err
	}
	st, path, err := cksrv.lookup(sc, prog, paths)
	if err != nil {
		return nil, "", err
	}
	return st, path, nil
}

func (cksrv *ChunkSrv) getFileStat(req proto.GetFileStatRequest) (*sp.Tstat, string, error) {
	db.DPrintf(db.CHUNKSRV, "%v: getFileStat pid %v", cksrv.kernelId, req.GetPid())
	start := time.Now()
	defer func() {
		db.DPrintf(db.CHUNKSRV, "%v: getFileStat done lat %v pid %v", cksrv.kernelId, time.Since(start), req.Pid)
	}()
	r := sp.Trealm(req.GetRealmStr())
	paths := req.GetSigmaPath()
	if req.SigmaPath[0] == cksrv.path {
		// If the first path is me, skip myself, because i don't have the stat
		paths = req.SigmaPath[1:]
	}

	if len(paths) == 0 {
		return nil, "", serr.NewErr(serr.TErrNotfound, req.GetProg())
	}

	for IsChunkSrvPath(paths[0]) {
		s := filepath.Base(paths[0])
		srv := paths[0]
		st, _, err := cksrv.ckclnt.GetFileStat(s, req.Prog, sp.Tpid(req.Pid), r, req.GetS3Secret(), []string{}, nil)
		db.DPrintf(db.CHUNKSRV, "%v: GetFileStat: chunkd %v pid %v st %v err %v", cksrv.kernelId, paths[0], req.GetPid(), st, err)
		if err == nil {
			return st, srv, nil
		}
		paths = paths[1:]
	}

	if len(paths) == 0 {
		return nil, "", serr.NewErr(serr.TErrNotfound, req.GetProg())
	}

	s := time.Now()
	// paths = replaceLocal(paths, cksrv.kernelId)
	st, srv, err := cksrv.getOrigin(r, req.GetProg(), paths, req.GetS3Secret())
	db.DPrintf(db.SPAWN_LAT, "[%v] getFileStat lat %v: origin %v err %v", req.GetPid(), time.Since(s), paths, err)
	if err != nil {
		return nil, "", err
	}
	return st, srv, nil
}

func (cksrv *ChunkSrv) GetFileStat(ctx fs.CtxI, req proto.GetFileStatRequest, res *proto.GetFileStatResponse) error {
	db.DPrintf(db.CHUNKSRV, "%v: GetFileStat: %v", cksrv.kernelId, req)
	db.DPrintf(db.SPAWN_LAT, "%v: GetFileStat: %v", cksrv.kernelId, req)
	s := time.Now()
	defer func() {
		db.DPrintf(db.SPAWN_LAT, "%v: GetFileStat done: %v lat %v", cksrv.kernelId, req, time.Since(s))
	}()

	r := sp.Trealm(req.GetRealmStr())
	be, err := cksrv.getBin(r, req.GetProg(), req.GetS3Secret())
	if err != nil {
		return err
	}

	if st, ok := be.isStatCached(); ok {
		db.DPrintf(db.CHUNKSRV, "%v: GetFileStat: hit %v %v", cksrv.kernelId, req.GetProg(), req.GetPid())
		res.Stat = st.StatProto()
		res.Path = cksrv.path
		return nil
	}

	if len(req.GetSigmaPath()) == 0 {
		return serr.NewErr(serr.TErrNotfound, req.GetProg())
	}

	if st := be.waitStat(); st != nil {
		db.DPrintf(db.CHUNKSRV, "%v: GetFileStat: wait hit %v %v", cksrv.kernelId, req.GetProg(), req.GetPid())
		res.Stat = st.StatProto()
		res.Path = cksrv.path
		return nil
	}

	defer be.signalStatWaiters()

	epp := req.GetNamedEndpointProto()
	if epp != nil {
		ep := sp.NewEndpointFromProto(epp)
		if ep.IsValidEP() {
			if err := be.sc.MountTree(ep, "", sp.NAMED); err != nil {
				db.DPrintf(db.CHUNKSRV, "MountTree %v err %v", ep, err)
				return err
			}
		}
	}

	// Prefetch first chunk
	go func() {
		db.DPrintf(db.SPAWN_LAT, "Prefetch chunk 0 %v %v %v", req.GetProg(), req.GetPid(), req.GetSigmaPath())
		s := time.Now()
		_, _, err := cksrv.fetchChunk(r, req.GetProg(), sp.Tpid(req.Pid), req.GetS3Secret(), 0, chunk.CHUNKSZ, req.GetSigmaPath())
		db.DPrintf(db.SPAWN_LAT, "GetFileStat: prefetch fetchChunk %v err %v lat %v", req.GetProg(), err, time.Since(s))
	}()

	st, srv, err := cksrv.getFileStat(req)
	if err != nil {
		return err
	}
	be.st = st
	res.Stat = st.StatProto()
	res.Path = srv
	return nil
}

func (cksrv *ChunkSrv) lookup(sc *sigmaclnt.SigmaClnt, prog string, paths []string) (*sp.Stat, string, error) {
	db.DPrintf(db.CHUNKSRV, "lookup %q %v", prog, paths)

	var st *sp.Stat
	path := ""
	err := fslib.RetryPaths(paths, func(i int, pn string) error {
		db.DPrintf(db.CHUNKSRV, "Stat '%v/%v'", pn, prog)
		s := time.Now()
		sst, err := sc.Stat(pn + "/" + prog)
		db.DPrintf(db.SPAWN_LAT, "Stat '%v/%v' lat %v", pn, prog, time.Since(s))
		if err == nil {
			sst.Dev = uint32(i)
			st = sst
			path = pn
			return nil
		}
		return err
	})
	db.DPrintf(db.CHUNKSRV, "lookup done %q %v st %v err %v", prog, paths, st, err)
	return st, path, err
}

func open(sc *sigmaclnt.SigmaClnt, prog string, paths []string) (int, string, error) {
	sfd := -1
	path := ""
	if err := fslib.RetryPaths(paths, func(i int, pn string) error {
		db.DPrintf(db.CHUNKSRV, "sOpen %q/%v", pn, prog)
		fd, err := sc.Open(pn+"/"+prog, sp.OREAD)
		if err == nil {
			sfd = fd
			path = pn
			return nil
		}
		return err
	}); err != nil {
		return sfd, "", err
	}
	return sfd, path, nil
}

// lookup of ~local is expensive because involves reading a
// directory. chunksrv knows what ~local is, namely its kernelId.
func replaceLocal(paths []string, kernelId string) []string {
	for i, p := range paths {
		if strings.HasPrefix(p, sp.UX) {
			paths[i] = strings.Replace(p, "~local", kernelId, 1)
		}
	}
	return paths
}

func IsPresent(pn string, ck int, totsz sp.Tsize) (int64, bool) {
	f, err := os.OpenFile(pn, os.O_RDONLY, 0777)
	if err != nil {
		db.DPrintf(db.CHUNKSRV, "IsPresent: open %v err %v", pn, err)
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
		o1 = CkRound(o1)
		for o := o1; o < o2; o += chunk.CHUNKSZ {
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

func Run(kernelId string) {
	pe := proc.GetProcEnv()
	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}
	cksrv := newChunkSrv(kernelId, sc)
	ssrv, err := sigmasrv.NewSigmaSrvClnt(filepath.Join(sp.CHUNKD, sc.ProcEnv().GetKernelID()), sc, cksrv)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}
	ssrv.RunServer()
}
