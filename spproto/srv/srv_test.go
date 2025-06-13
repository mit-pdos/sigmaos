package srv_test

import (
	"flag"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/api/fs"
	"sigmaos/ctx"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/namesrv"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/path"
	"sigmaos/proc"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv/memfs"
	"sigmaos/sigmasrv/memfssrv/memfs/dir"
	"sigmaos/sigmasrv/memfssrv/memfs/inode"
	"sigmaos/sigmasrv/stats"
	"sigmaos/spproto/srv"
	"sigmaos/test"
)

var srvname string // e.g., memfs or named
var N int
var D int

func init() {
	flag.StringVar(&srvname, "server", sp.MEMFSREL, "server")
	flag.IntVar(&N, "N", 1000, "N iterations")
	flag.IntVar(&D, "D", 1, "Create test directory w D directory entries")
}

func TestCompile(t *testing.T) {
}

type tstate struct {
	t    *testing.T
	srv  *srv.ProtSrv
	nfid sp.Tfid
}

func setupNamed(pe *proc.ProcEnv) fs.Dir {
	stats := fsetcd.NewPstatsDev(inode.NewInodeAlloc(sp.DEV_STATFS))
	npc := dialproxyclnt.NewDialProxyClnt(pe)
	fs, err := fsetcd.NewFsEtcd(npc.Dial, pe.GetEtcdEndpoints(), sp.ROOTREALM, stats)
	if err != nil {
		db.DFatalf("setupNamed: NewFsEtcd err %v", err)
	}
	_, elect, err := namesrv.Elect(fs, pe, sp.ROOTREALM)
	if err != nil {
		db.DFatalf("setupNamed: Elect err %v", err)
	}
	fs.Fence(elect.Key(), elect.Rev())
	return namesrv.RootDir(fs, sp.ROOTREALM)
}

func newTstate(t *testing.T) *tstate {
	lip := sp.Tip("127.0.0.1")
	etcdMnt, err := fsetcd.NewFsEtcdEndpoint(sp.Tip(test.EtcdIP))
	if err != nil {
		db.DFatalf("newTstate: NewFsEtcdEndpoints err %v", err)
	}
	pe := proc.NewTestProcEnv(sp.ROOTREALM, nil, etcdMnt, lip, lip, "", false, false)
	ctx := ctx.NewCtx(sp.NoPrincipal(), nil, 0, sp.NoClntId, nil, nil)
	var root fs.Dir
	switch srvname {
	case sp.MEMFSREL:
		root = dir.NewRootDir(ctx, memfs.NewNewInode(sp.DEV_MEMFS))
	case sp.NAMEDREL:
		root = setupNamed(pe)
	default:
		db.DFatalf("newTstate: Unknown srv %v", srvname)
	}
	stats := stats.NewStatsDev(inode.NewInodeAlloc(sp.DEV_STATFS))
	pps := srv.NewProtSrvState(stats)
	grf := func(*sp.Tprincipal, map[string]*sp.SecretProto, string, sessp.Tsession, sp.TclntId) (fs.Dir, fs.CtxI) {
		return root, ctx
	}
	aaf := srv.AttachAllowAllToAll
	srv := srv.NewProtSrv(pe, pps, sp.NoPrincipal(), 0, grf, aaf)
	srv.NewRootFid(0, ctx, root, "")
	return &tstate{t, srv, sp.Tfid(2)}
}

func (ts *tstate) newFid() (fid sp.Tfid) {
	fid = ts.nfid
	ts.nfid += 1
	return
}

func (ts *tstate) walkPath(pn string) sp.Tfid {
	nfid := ts.newFid()
	args := sp.NewTwalk(0, nfid, path.Split(pn))
	rets := sp.Rwalk{}
	rerr := ts.srv.Walk(args, &rets)
	assert.Nil(ts.t, rerr, "rerror %v", rerr)
	return nfid
}

func (ts *tstate) walk() sp.Tfid {
	return ts.walkPath("")
}

func (ts *tstate) clunk(fid sp.Tfid) {
	args := sp.NewTclunk(fid)
	rets := sp.Rclunk{}
	rerr := ts.srv.Clunk(args, &rets)
	assert.Nil(ts.t, rerr, "rerror %v", rerr)
}

func (ts *tstate) create(n string) sp.Tfid {
	nfid := ts.walkPath(filepath.Dir(n))
	args := sp.NewTcreate(nfid, filepath.Base(n), 0777, sp.ORDWR, sp.NoLeaseId, sp.NullFence())
	rets := sp.Rcreate{}
	rerr := ts.srv.Create(args, &rets)
	assert.Nil(ts.t, rerr, "rerror %v", rerr)
	return nfid
}

func (ts *tstate) mkdir(n string) {
	nfid := ts.walk()
	args := sp.NewTcreate(nfid, n, 0777|sp.DMDIR, sp.ORDWR, sp.NoLeaseId, sp.NullFence())
	rets := sp.Rcreate{}
	rerr := ts.srv.Create(args, &rets)
	assert.Nil(ts.t, rerr, "rerror %v", rerr)
	ts.clunk(nfid)
}

func (ts *tstate) remove(fid sp.Tfid) {
	args := sp.NewTremove(fid, sp.NullFence())
	rets := sp.Rremove{}
	rerr := ts.srv.Remove(args, &rets)
	assert.Nil(ts.t, rerr, "rerror %v", rerr)
}

func (ts *tstate) stat(fid sp.Tfid) *sp.Tstat {
	args := sp.NewTrstat(fid)
	rets := sp.Rrstat{}
	rerr := ts.srv.Stat(args, &rets)
	assert.Nil(ts.t, rerr, "rerror %v", rerr)
	return &sp.Tstat{rets.Stat}
}

func (ts *tstate) rename(dsrc, src, ddst, dst string) {
	d0fid := ts.walkPath(dsrc)
	d1fid := ts.walkPath(ddst)
	args := sp.NewTrenameat(d0fid, src, d1fid, dst, sp.NullFence())
	rets := sp.Rrenameat{}
	rerr := ts.srv.Renameat(args, &rets)
	assert.Nil(ts.t, rerr, "rerror %v", rerr)
	ts.clunk(d0fid)
	ts.clunk(d1fid)
}

func (ts *tstate) setupDir(dir string) {
	s := time.Now()
	for i := 0; i < D; i++ {
		fid := ts.create(filepath.Join(dir, "ggg"+strconv.Itoa(i)))
		ts.clunk(fid)
	}
	t := time.Since(s)
	if D > 0 {
		db.DPrintf(db.TEST, "setupDir: %d creates %v us/op %f", D, t, float64(t.Microseconds())/float64(D))
		db.DPrintf(db.TEST, "setupDir: len freelist %d", ts.srv.Stats())
	}
}

func TestWalkClunk(t *testing.T) {
	ns := []int{N}
	for _, n := range ns {
		ts := newTstate(t)
		s := time.Now()
		for i := 0; i < n; i++ {
			fid := ts.walk()
			ts.clunk(fid)
		}
		t := time.Since(s)
		db.DPrintf(db.TEST, "%v: %d walk+clunk %v us/op %f", srvname, n, t, float64(t.Microseconds())/float64(n))
		db.DPrintf(db.TEST, "len freelist %v", ts.srv.Stats())
	}
}

func TestWalkPathClunk(t *testing.T) {
	ns := []int{N}
	for _, n := range ns {
		ts := newTstate(t)
		ts.setupDir("")
		ts.mkdir("d0")
		fid := ts.create("d0/fff0")
		ts.clunk(fid)
		s := time.Now()
		for i := 0; i < n; i++ {
			fid := ts.walkPath("d0/fff0")
			ts.clunk(fid)
		}
		t := time.Since(s)
		db.DPrintf(db.TEST, "%v: %d walk+clunk %v us/op %f", srvname, n, t, float64(t.Microseconds())/float64(n))
		db.DPrintf(db.TEST, "len freelist %v", ts.srv.Stats())
	}
}

func TestWalkStat(t *testing.T) {
	ns := []int{N}
	for _, n := range ns {
		ts := newTstate(t)
		ts.setupDir("")
		s := time.Now()
		for i := 0; i < n+1; i++ {
			nfid := ts.walkPath("ggg0")
			ts.stat(nfid)
			ts.clunk(nfid)
		}
		t := time.Since(s)
		db.DPrintf(db.TEST, "%d walk+stat %v us/op %f", n, t, float64(t.Microseconds())/float64(n))
		db.DPrintf(db.TEST, "len freelist %v", ts.srv.Stats())
	}
}

func TestCreateRemove(t *testing.T) {
	ns := []int{N}
	for _, n := range ns {
		ts := newTstate(t)
		ts.setupDir("")
		s := time.Now()
		for i := 0; i < n; i++ {
			fid := ts.create("fff" + strconv.Itoa(i))
			ts.remove(fid)
		}
		t := time.Since(s)
		db.DPrintf(db.TEST, "%d create+remove %v us/op %f", n, t, float64(t.Microseconds())/float64(n))
		db.DPrintf(db.TEST, "len freelist %d", ts.srv.Stats())
	}
}
func TestCreateRenameat(t *testing.T) {
	ns := []int{N}
	for _, n := range ns {
		ts := newTstate(t)
		ts.mkdir("d0")
		ts.mkdir("d1")
		ts.setupDir("d0")
		ts.setupDir("d1")
		s := time.Now()
		for i := 0; i < n; i++ {
			fid := ts.create("d0/fff1")
			ts.clunk(fid)

			ts.rename("d0", "fff1", "d1", "fff2")

			//nfid := ts.walkPath("d1/fff2")
			//ts.remove(nfid)
		}
		t := time.Since(s)
		db.DPrintf(db.TEST, "%d create+renameat+remove %v us/op %f", n, t, float64(t.Microseconds())/float64(n))
		db.DPrintf(db.TEST, "len freelist %d", ts.srv.Stats())
	}
}
