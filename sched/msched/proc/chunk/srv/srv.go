// package chunksrv caches chunks of binaries and the binary's Stat
// for procs.  It uses the proc's SigmaPath paths to locate the
// binary, if it doesn't have it cached: typically the last entry in
// SigmaPath is the origin for the binary (e.g., an S3 path) and
// earlier entries are paths to other chunksrvs (e.g., prepended by
// procclnt or procqsrv).
package srv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/sched/msched/proc/chunk"
	chunkclnt "sigmaos/sched/msched/proc/chunk/clnt"
	proto "sigmaos/sched/msched/proc/chunk/proto"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
	"sigmaos/util/perf"
)

const (
	SEEK_DATA = 3
	SEEK_HOLE = 4

	// Depending on whether developing in multi-user or single-user mode, the
	// host bin cache will be mounted at either /tmp/sigmaos-bin or
	// /tmp/$SIGMAUSER/sigmaos-bin
	ROOTHOSTCACHE_DIR = "sigmaos-bin"

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

// The path on the host where a kernel caches its binaries
func PathHostKernel(user, kernelId string) string {
	tmpBase := "/tmp"
	if user != sp.NOT_SET {
		tmpBase = filepath.Join(tmpBase, user)
	}
	rootHostCache := filepath.Join(tmpBase, ROOTHOSTCACHE_DIR)
	return filepath.Join(rootHostCache, kernelId)
}

// For testing: the path on the host where a kernel caches its
// binaries for a realm.
func PathHostKernelRealm(user, kernelId string, realm sp.Trealm) string {
	return filepath.Join(PathHostKernel(user, kernelId), realm.String())
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
func (cksrv *ChunkSrv) getRealmSigmaClnt(r sp.Trealm, s3secret *sp.SecretProto) (*sigmaclnt.SigmaClnt, error) {
	db.DPrintf(db.CHUNKSRV, "%v: Create SigmaClnt for realm %v", cksrv.kernelId, r)
	// Create a new proc env for the client
	pe := proc.NewDifferentRealmProcEnv(cksrv.sc.ProcEnv(), r)

	// Set the secrets to match those passed in by the user
	pe.SetSecrets(map[string]*sp.SecretProto{"s3": s3secret})
	// Create a sigmaclnt but only with an FsLib
	sc, err := sigmaclnt.NewSigmaClntFsLib(pe, dialproxyclnt.NewDialProxyClnt(pe))
	if err != nil {
		db.DPrintf(db.ERROR, "Error create SigmaClnt: %v", err)
		return nil, err
	}
	return sc, nil
}

func (cksrv *ChunkSrv) getSc(realm sp.Trealm, s3secret *sp.SecretProto, ep *sp.Tendpoint) (*sigmaclnt.SigmaClnt, error) {
	r, ok := cksrv.realms.Lookup(realm)
	if !ok {
		return nil, fmt.Errorf("No sc for realm %v", realm)
	}
	r.Lock()
	defer r.Unlock()
	if r.sc == nil {
		sc, err := cksrv.getRealmSigmaClnt(realm, s3secret)
		if err != nil {
			db.DPrintf(db.ERROR, "Error create SigmaClnt: %v", err)
			return nil, err
		}
		r.sc = sc
		if err := sc.MountTree(ep, "", sp.NAMED); err != nil {
			db.DPrintf(db.CHUNKSRV, "MountTree %v err %v", ep, err)
			return nil, err
		}
	}
	return r.sc, nil
}

// Get binary info of prog in realm. Creates a SigmaClnt for realm, if unknown realm.
func (cksrv *ChunkSrv) getBin(realm sp.Trealm, prog string) (*bin, error) {
	r, _ := cksrv.realms.AllocNew(realm, newRealm)
	b, _ := r.bins.AllocNew(prog, newBin)
	return b, nil
}

//
// Handle a FetchChunkReq
//

func (cksrv *ChunkSrv) fetchCache(be *bin, r sp.Trealm, pid sp.Tpid, s3secret *sp.SecretProto, ck int, size sp.Tsize, data bool) (bool, sp.Tsize, string, *rpcproto.Blob, error) {
	pn := pathBinCache(r, be.prog)
	if sz, ok := IsPresent(pn, ck, size); ok {
		b := make([]byte, sz)
		db.DPrintf(db.CHUNKSRV, "%v: FetchCache %q pid %v ckid %d hit %d", cksrv.kernelId, pn, pid, ck, sz)
		if err := ReadChunk(pn, ck, b); err != nil {
			return false, 0, "", nil, err
		}
		if data {
			return true, sp.Tsize(sz), cksrv.path, &rpcproto.Blob{Iov: [][]byte{b}}, nil
		}
		return true, sp.Tsize(sz), cksrv.path, nil, nil
	}
	db.DPrintf(db.CHUNKSRV, "%v: FetchCache: %q pid %v ckid %d not present\n", cksrv.kernelId, pn, pid, ck)
	return false, 0, "", nil, nil
}

func (cksrv *ChunkSrv) fetchOrigin(r sp.Trealm, prog string, pid sp.Tpid, s3secret *sp.SecretProto, ck int, paths []string, b []byte, ep *sp.Tendpoint) (sp.Tsize, string, error) {
	db.DPrintf(db.CHUNKSRV, "%v: fetchOrigin: pid %v %v ckid %d %v", cksrv.kernelId, pid, prog, ck, paths)
	be, err := cksrv.getBin(r, prog)
	if err != nil {
		return 0, "", err
	}
	sc, err := cksrv.getSc(r, s3secret, ep)
	if err != nil {
		return 0, "", err
	}
	// paths = replaceLocal(paths, cksrv.kernelId)
	fd, path := be.getFd(paths)
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
		perf.LogSpawnLatency("ChunkSrv.fetchOrigin.open", pid, perf.TIME_NOT_SET, s)
	}
	sz, err := sc.Pread(fd, b, sp.Toffset(chunk.ChunkOff(ck)))
	if err != nil {
		db.DPrintf(db.CHUNKSRV, "%v: fetchOrigin: pid %v read %q ckid %d err %v", cksrv.kernelId, pid, prog, ck, err)
		be.failFd()
		sc.CloseFd(fd)
		return 0, "", err
	}
	return sz, path, nil
}

func (cksrv *ChunkSrv) fetchChunk(be *bin, r sp.Trealm, pid sp.Tpid, s3secret *sp.SecretProto, ck int, size sp.Tsize, paths []string, ep *sp.Tendpoint) (sp.Tsize, string, error) {
	sz := sp.Tsize(0)
	b := make([]byte, int(sp.Conf.Chunk.CHUNK_SZ))
	var err error

	if paths[0] == cksrv.path {
		// If the first path is me, skip myself, because i don't have
		// chunk.
		paths = paths[1:]
	}

	if len(paths) == 0 {
		db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: r %v p %v pid %v err %v", cksrv.kernelId, r, be.prog, pid, err)
		return 0, "", serr.NewErr(serr.TErrNotfound, be.prog)
	}

	ok := false
	srvpath := ""
	for chunk.IsChunkSrvPath(paths[0]) {
		srvpath = paths[0]
		srv := filepath.Base(srvpath)
		db.DPrintf(db.CHUNKSRV, "%v: fetchChunk: pid %v prog %v ckid %d %v", cksrv.kernelId, pid, be.prog, ck, []string{srvpath})
		sz, _, err = cksrv.ckclnt.FetchChunk(srv, be.prog, pid, r, s3secret, ck, size, []string{}, b)
		db.DPrintf(db.CHUNKSRV, "%v: fetchChunk done: pid %v prog %v ckid %d %v err %v", cksrv.kernelId, pid, be.prog, ck, []string{srvpath}, err)
		if err == nil {
			ok = true
			break
		}
		paths = paths[1:]
	}

	if !ok {
		if len(paths) == 0 {
			db.DPrintf(db.CHUNKSRV, "%v: fetchChunk err: pid %v %v err %v", cksrv.kernelId, pid, be.prog, err)
			return 0, "", serr.NewErr(serr.TErrNotfound, be.prog)
		}
		sz, srvpath, err = cksrv.fetchOrigin(r, be.prog, pid, s3secret, ck, paths, b, ep)
		if err != nil {
			db.DPrintf(db.CHUNKSRV, "%v: fetchChunk err: pid %v origin %v err %v", cksrv.kernelId, pid, paths, err)
			return 0, "", err
		}
	}
	pn := pathBinCache(r, be.prog)
	if err := writeChunk(pn, ck, b[0:sz]); err != nil {
		db.DPrintf(db.CHUNKSRV, "fetchChunk err: Writechunk %q ckid %d err %v", pn, ck, err)
		return 0, "", err
	}
	db.DPrintf(db.CHUNKSRV, "%v: fetchChunk done: writeChunk %v pid %v ckid %d sz %d", cksrv.kernelId, pn, pid, ck, sz)
	return sz, srvpath, nil
}

func (cksrv *ChunkSrv) fetch(realm sp.Trealm, prog string, pid sp.Tpid, s3secret *sp.SecretProto, ck int, size sp.Tsize, paths []string, data bool, ep *sp.Tendpoint) (sp.Tsize, string, *rpcproto.Blob, error) {
	db.DPrintf(db.CHUNKSRV, "%v: Fetch: pid %v %v", cksrv.kernelId, pid, prog)
	s := time.Now()
	defer func() {
		perf.LogSpawnLatency("ChunkSrv.Fetch chunk %d", pid, perf.TIME_NOT_SET, s, ck)
	}()

	ckid := int(ck)
	be, err := cksrv.getBin(realm, prog)
	if err != nil {
		return 0, "", nil, err
	}

	if st, ok := be.isStatCached(); ok {
		db.DPrintf(db.CHUNKSRV, "%v: Fetch: pid %v hit stat %v %d", cksrv.kernelId, pid, prog, st.Length)
		if uint64(sp.Tsize(chunk.ChunkOff(ckid))+size) > st.Length {
			size = sp.Tsize(st.Length)
		}
	} else {
		if ep == nil {
			db.DPrintf(db.CHUNKSRV, "Stat not cached in fetch and ep not set realm %v prog %v", realm, prog)
		}
	}

	ok, sz, srvpath, blob, err := cksrv.fetchCache(be, realm, pid, s3secret, ckid, size, data)
	if ok || err != nil {
		return sz, srvpath, blob, err
	}

	if len(paths) == 0 {
		db.DPrintf(db.CHUNKSRV, "%v: Fetch: pid %v %v ok %t err %v", cksrv.kernelId, pid, prog, ok, err)
		return 0, "", nil, serr.NewErr(serr.TErrNotfound, prog)
	}

	// one outstanding fetch per chunk
	be.waitFetch(ckid)
	defer be.signalFetchWaiters(ckid)

	sz, srvpath, err = cksrv.fetchChunk(be, realm, pid, s3secret, ckid, size, paths, ep)
	if err != nil {
		db.DPrintf(db.CHUNKSRV, "%v: Fetch: pid %v %v ok %t err 2 %v", cksrv.kernelId, pid, prog, ok, err)
		return 0, "", nil, err
	}
	return sz, srvpath, nil, nil
}

func (cksrv *ChunkSrv) Fetch(ctx fs.CtxI, req proto.FetchChunkReq, res *proto.FetchChunkRep) error {
	var ep *sp.Tendpoint
	epp := req.GetNamedEndpointProto()
	if epp != nil {
		ep = sp.NewEndpointFromProto(epp)
	}
	sz, srvpath, blob, err := cksrv.fetch(sp.Trealm(req.Realm), req.Prog, sp.Tpid(req.Pid), req.GetS3Secret(), int(req.ChunkId), sp.Tsize(req.Size), req.SigmaPath, req.Data, ep)
	if err != nil {
		return err
	}
	res.Size = uint64(sz)
	res.Path = srvpath
	if blob != nil {
		res.Blob = blob
	}
	return nil
}

//
// Handle a GetFileStatReq
//

func (cksrv *ChunkSrv) getOrigin(r sp.Trealm, pid sp.Tpid, prog string, paths []string, s3secret *sp.SecretProto, ep *sp.Tendpoint) (*sp.Tstat, string, error) {
	db.DPrintf(db.CHUNKSRV, "%v: getOrigin %v %v", cksrv.kernelId, prog, paths)
	sc, err := cksrv.getSc(r, s3secret, ep)
	if err != nil {
		db.DPrintf(db.ERROR, "Error get realm (%v) sigma clnt: %v", r, err)
		return nil, "", err
	}
	st, path, err := cksrv.lookup(sc, pid, prog, paths)
	if err != nil {
		return nil, "", err
	}
	return st, path, nil
}

func (cksrv *ChunkSrv) getFileStat(r sp.Trealm, prog string, pid sp.Tpid, paths []string, s3secret *sp.SecretProto, ep *sp.Tendpoint) (*sp.Tstat, string, error) {
	db.DPrintf(db.CHUNKSRV, "%v: getFileStat pid %v", cksrv.kernelId, pid)
	start := time.Now()
	defer func() {
		db.DPrintf(db.CHUNKSRV, "%v: getFileStat done lat %v pid %v", cksrv.kernelId, time.Since(start), pid)
	}()
	if paths[0] == cksrv.path {
		// If the first path is me, skip myself, because i don't have the stat
		paths = paths[1:]
	}

	if len(paths) == 0 {
		return nil, "", serr.NewErr(serr.TErrNotfound, prog)
	}

	for chunk.IsChunkSrvPath(paths[0]) {
		s := filepath.Base(paths[0])
		srv := paths[0]
		st, _, err := cksrv.ckclnt.GetFileStat(s, prog, pid, r, s3secret, []string{}, nil)
		db.DPrintf(db.CHUNKSRV, "%v: GetFileStat: chunkd %v pid %v st %v err %v", cksrv.kernelId, paths[0], pid, st, err)
		if err == nil {
			return st, srv, nil
		}
		paths = paths[1:]
	}

	if len(paths) == 0 {
		return nil, "", serr.NewErr(serr.TErrNotfound, prog)
	}

	s := time.Now()
	// paths = replaceLocal(paths, cksrv.kernelId)
	st, srv, err := cksrv.getOrigin(r, pid, prog, paths, s3secret, ep)
	perf.LogSpawnLatency("ChunkSrv.getFileStat.getOrigin %v", pid, perf.TIME_NOT_SET, s, paths)
	if err != nil {
		return nil, "", err
	}
	return st, srv, nil
}

func (cksrv *ChunkSrv) GetFileStat(ctx fs.CtxI, req proto.GetFileStatReq, res *proto.GetFileStatRep) error {
	db.DPrintf(db.CHUNKSRV, "%v: GetFileStat: %v", cksrv.kernelId, req)
	s := time.Now()
	defer func() {
		perf.LogSpawnLatency("ChunkSrv.GetFileStat %s", sp.Tpid(req.GetPid()), perf.TIME_NOT_SET, s, cksrv.kernelId)
	}()

	r := sp.Trealm(req.GetRealmStr())
	be, err := cksrv.getBin(r, req.GetProg())
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

	// for lookup and fetches from origin
	var ep *sp.Tendpoint
	epp := req.GetNamedEndpointProto()
	if epp != nil {
		ep = sp.NewEndpointFromProto(epp)
	}

	// Prefetch first chunk
	go func() {
		perf.LogSpawnLatency("ChunkSrv.prefetchChunk start", sp.Tpid(req.GetPid()), perf.TIME_NOT_SET, perf.TIME_NOT_SET)
		cksrv.fetch(r, be.prog, sp.Tpid(req.Pid), req.GetS3Secret(), 0, sp.Tsize(sp.Conf.Chunk.CHUNK_SZ), req.GetSigmaPath(), true, ep)
	}()

	st, srv, err := cksrv.getFileStat(r, req.GetProg(), sp.Tpid(req.Pid), req.GetSigmaPath(), req.GetS3Secret(), ep)
	if err != nil {
		return err
	}
	be.st = st
	res.Stat = st.StatProto()
	res.Path = srv
	return nil
}

func (cksrv *ChunkSrv) lookup(sc *sigmaclnt.SigmaClnt, pid sp.Tpid, prog string, paths []string) (*sp.Tstat, string, error) {
	db.DPrintf(db.CHUNKSRV, "lookup %q %v", prog, paths)

	var st *sp.Tstat
	path := ""
	err := fslib.RetryPaths(paths, func(i int, pn string) error {
		db.DPrintf(db.CHUNKSRV, "Stat '%v/%v'", pn, prog)
		s := time.Now()
		sst, err := sc.Stat(pn + "/" + prog)
		perf.LogSpawnLatency("ChunkSrv.lookup.Stat", pid, perf.TIME_NOT_SET, s)
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

// lookup of sp.LOCAL is expensive because involves reading a
// directory. chunksrv knows what sp.LOCAL is, namely its kernelId.
func replaceLocal(paths []string, kernelId string) []string {
	for i, p := range paths {
		if strings.HasPrefix(p, sp.UX) {
			paths[i] = strings.Replace(p, sp.LOCAL, kernelId, 1)
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
		o1 = chunk.ChunkRound(o1)
		for o := o1; o < o2; o += sp.Conf.Chunk.CHUNK_SZ {
			if o+sp.Conf.Chunk.CHUNK_SZ <= o2 || o2 >= int64(totsz) { // a complete chunk?
				i := chunk.Index(o)
				if i == ck {
					db.DPrintf(db.CHUNKSRV, "IsPresent: %q read chunk %d(%d) o2 %d sz %d", pn, i, o, o2, totsz)
					ok = true
					sz = sp.Conf.Chunk.CHUNK_SZ
					if o+sp.Conf.Chunk.CHUNK_SZ >= int64(totsz) {
						sz = int64(totsz) - o
					}
					break
				}
			}
		}
		off = o2
	}
	if sz > sp.Conf.Chunk.CHUNK_SZ {
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
	nn, err := ufd.WriteAt(b, chunk.ChunkOff(ckid))
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
	nn, err := f.ReadAt(b, chunk.ChunkOff(ckid))
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
