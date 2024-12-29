package srv_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/api/fs"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/proc"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv/memfs"
	"sigmaos/sigmasrv/memfssrv/memfs/dir"
	"sigmaos/sigmasrv/stats"
	"sigmaos/spproto/srv"
)

func TestCompile(t *testing.T) {
}

type tstate struct {
	t   *testing.T
	srv *srv.ProtSrv
}

func newTstate(t *testing.T) *tstate {
	ctx := ctx.NewCtx(sp.NoPrincipal(), nil, 0, sp.NoClntId, nil, nil)
	root := dir.NewRootDir(ctx, memfs.NewInode, nil)
	stats := stats.NewStatsDev(root)
	pps := srv.NewProtSrvState(stats)
	grf := func(*sp.Tprincipal, map[string]*sp.SecretProto, string, sessp.Tsession, sp.TclntId) (fs.Dir, fs.CtxI) {
		return root, ctx
	}
	aaf := srv.AttachAllowAllToAll
	pe := proc.NewTestProcEnv(sp.ROOTREALM, nil, nil, sp.NO_IP, sp.NO_IP, "", false, false)
	srv := srv.NewProtSrv(pe, pps, sp.NoPrincipal(), 0, grf, aaf)
	srv.NewRootFid(0, ctx, root, path.Tpathname{})
	return &tstate{t, srv}
}

func (ts *tstate) walk(fid, nfid sp.Tfid) {
	args := sp.NewTwalk(fid, nfid, path.Tpathname{})
	rets := sp.Rwalk{}
	rerr := ts.srv.Walk(args, &rets)
	assert.Nil(ts.t, rerr, "rerror %v", rerr)
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

func TestCreate(t *testing.T) {
	// ns := []int{10, 100, 1000, 10_000, 100_000, 1_000_000}
	ns := []int{100_000}
	for _, n := range ns {
		ts := newTstate(t)
		s := time.Now()
		twalk := sp.NewTwalk(0, 0, path.Tpathname{})
		rwalk := &sp.Rwalk{}
		tclunk := sp.NewTclunk(0)
		rclunk := &sp.Rclunk{}
		tcreate := sp.NewTcreate(0, "", 0777, sp.ORDWR, sp.NoLeaseId, sp.NullFence())
		rcreate := &sp.Rcreate{}
		//tremove := sp.NewTremove(0, sp.NullFence())
		//rremove := &sp.Rremove{}
		for i := 1; i < n; i++ {
			twalk.NewFid = uint32(i)
			rerr := ts.srv.Walk(twalk, rwalk)
			assert.Nil(ts.t, rerr, "walk rerror %v", rerr)
			tcreate.Fid = uint32(i)
			tcreate.Name = "fff" + strconv.Itoa(i)
			rerr = ts.srv.Create(tcreate, rcreate)
			assert.Nil(ts.t, rerr, "create rerror %v", rerr)
			tclunk.Fid = uint32(i)
			rerr = ts.srv.Clunk(tclunk, rclunk)
			assert.Nil(ts.t, rerr, "clunk rerror %v", rerr)
			//tremove.Fid = uint32(i)
			//rerr = ts.srv.Remove(tremove, rremove)
			//assert.Nil(ts.t, rerr, "clunk rerror %v", rerr)
		}
		t := time.Since(s)
		db.DPrintf(db.TEST, "%d creates %v us/op %f", n, t, float64(t.Microseconds())/float64(n))
	}
}
