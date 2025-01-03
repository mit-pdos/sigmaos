package srv_test

import (
	"flag"
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
	"sigmaos/sigmasrv/stats"
	"sigmaos/spproto/srv"
	"sigmaos/test"
)

var srvname string // e.g., memfs

func init() {
	flag.StringVar(&srvname, "server", sp.MEMFSREL, "server")
}

func TestCompile(t *testing.T) {
}

type tstate struct {
	t   *testing.T
	srv *srv.ProtSrv
}

func setupNamed(pe *proc.ProcEnv) fs.Dir {
	stats := fsetcd.NewPstatsDev()
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
		root = dir.NewRootDir(ctx, memfs.NewInode)
	case sp.NAMEDREL:
		root = setupNamed(pe)
	default:
		db.DFatalf("newTstate: Unknown srv %v", srvname)
	}
	stats := stats.NewStatsDev()
	pps := srv.NewProtSrvState(stats)
	grf := func(*sp.Tprincipal, map[string]*sp.SecretProto, string, sessp.Tsession, sp.TclntId) (fs.Dir, fs.CtxI) {
		return root, ctx
	}
	aaf := srv.AttachAllowAllToAll
	srv := srv.NewProtSrv(pe, pps, sp.NoPrincipal(), 0, grf, aaf)
	srv.NewRootFid(0, ctx, root, "")
	return &tstate{t, srv}
}

func (ts *tstate) walkPath(fid, nfid sp.Tfid, path path.Tpathname) {
	args := sp.NewTwalk(fid, nfid, path)
	rets := sp.Rwalk{}
	rerr := ts.srv.Walk(args, &rets)
	assert.Nil(ts.t, rerr, "rerror %v", rerr)
}

func (ts *tstate) walk(fid, nfid sp.Tfid) {
	ts.walkPath(fid, nfid, path.Tpathname{})
}

func (ts *tstate) clunk(fid sp.Tfid) {
	args := sp.NewTclunk(fid)
	rets := sp.Rclunk{}
	rerr := ts.srv.Clunk(args, &rets)
	assert.Nil(ts.t, rerr, "rerror %v", rerr)
}

func (ts *tstate) create(fid sp.Tfid, n string) {
	args := sp.NewTcreate(fid, n, 0777, sp.ORDWR, sp.NoLeaseId, sp.NullFence())
	rets := sp.Rcreate{}
	rerr := ts.srv.Create(args, &rets)
	assert.Nil(ts.t, rerr, "rerror %v", rerr)
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

func TestCreateMany(t *testing.T) {
	//ns := []int{10, 100, 1000, 10_000, 100_000, 1_000_000}
	ns := []int{2}
	for _, n := range ns {
		ts := newTstate(t)
		s := time.Now()
		for i := 1; i < n; i++ {
			ts.walk(0, sp.Tfid(i))
			ts.create(sp.Tfid(i), "fff"+strconv.Itoa(i))
			ts.clunk(sp.Tfid(i))
		}
		t := time.Since(s)
		db.DPrintf(db.TEST, "%d creates %v us/op %f", n, t, float64(t.Microseconds())/float64(n))
	}
}

func TestCreateRemove(t *testing.T) {
	// ns := []int{10, 100, 1000, 10_000, 100_000, 1_000_000}
	//ns := []int{10_000}
	ns := []int{10}
	for _, n := range ns {
		ts := newTstate(t)
		s := time.Now()
		for i := 1; i < n; i++ {
			ts.walk(0, sp.Tfid(i))
			ts.create(sp.Tfid(i), "fff"+strconv.Itoa(i))
			ts.remove(sp.Tfid(i))
		}
		t := time.Since(s)
		db.DPrintf(db.TEST, "%d create+remove %v us/op %f", n, t, float64(t.Microseconds())/float64(n))
		db.DPrintf(db.TEST, "len freelist %d", ts.srv.Stats())
	}
}

func TestWalkClunk(t *testing.T) {
	// ns := []int{10, 100, 1000, 10_000, 100_000, 1_000_000}
	ns := []int{100_000}
	//ns := []int{10}
	for _, n := range ns {
		ts := newTstate(t)
		s := time.Now()
		for i := 1; i < n; i++ {
			ts.walk(0, sp.Tfid(i))
			ts.clunk(sp.Tfid(i))
		}
		t := time.Since(s)
		db.DPrintf(db.TEST, "%d walk+clunk %v us/op %f", n, t, float64(t.Microseconds())/float64(n))
		db.DPrintf(db.TEST, "len freelist %v", ts.srv.Stats())
	}
}

func TestWalkStat(t *testing.T) {
	// ns := []int{10, 100, 1000, 10_000, 100_000, 1_000_000}
	// ns := []int{100_000}
	ns := []int{10}
	for _, n := range ns {
		ts := newTstate(t)
		for i := 1; i < n; i++ {
			ts.walk(0, sp.Tfid(i))
			ts.create(sp.Tfid(i), "fff"+strconv.Itoa(i))
			ts.clunk(sp.Tfid(i))
		}
		s := time.Now()
		nfid := sp.Tfid(1)
		ts.walkPath(0, nfid, path.Split("fff1"))
		st := ts.stat(nfid)
		ts.clunk(nfid)
		t := time.Since(s)
		db.DPrintf(db.TEST, "%v for walk+stat in dir w. %d files st %v", t, n, st)
		db.DPrintf(db.TEST, "len freelist %v", ts.srv.Stats())
	}
}
