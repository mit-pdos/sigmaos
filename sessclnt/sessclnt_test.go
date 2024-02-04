package sessclnt_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/dir"
	"sigmaos/ephemeralmap"
	"sigmaos/memfs"
	"sigmaos/netsrv"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/protsrv"
	"sigmaos/serr"
	"sigmaos/sessclnt"
	"sigmaos/sessp"
	"sigmaos/sesssrv"
	sp "sigmaos/sigmap"
	"sigmaos/sigmaprotsrv"
	"sigmaos/spcodec"
)

type SessSrv struct {
}

func (ss *SessSrv) ReportError(conn sigmaprotsrv.Conn, err error) {
}

func (ss *SessSrv) ServeRequest(conn sigmaprotsrv.Conn, req demux.CallI) (demux.CallI, *serr.Err) {
	fcm := req.(*sessp.FcallMsg)
	db.DPrintf(db.TEST, "fcall %v\n", fcm)
	msg := &sp.Rattach{Qid: sp.NewQidPerm(0777, 0, 0)}
	rep := sessp.NewFcallMsgReply(fcm, msg)
	return rep, nil
}

type Tstate struct {
	T    *testing.T
	lip  sp.Tip
	pcfg *proc.ProcEnv
	addr *sp.Taddr
}

func newTstate(t *testing.T) *Tstate {
	lip := sp.Tip("127.0.0.1")
	pcfg := proc.NewTestProcEnv(sp.ROOTREALM, lip, lip, lip, "", false, false)
	pcfg.Program = "srv"
	pcfg.SetUname("srv")
	addr := sp.NewTaddr(sp.NO_IP, sp.INNER_CONTAINER_IP, 1110)
	proc.SetSigmaDebugPid(pcfg.GetPID().String())
	return &Tstate{T: t, lip: lip, pcfg: pcfg, addr: addr}
}

func TestConnectSessSrv(t *testing.T) {
	ts := newTstate(t)
	ss := &SessSrv{}

	srv := netsrv.NewNetServer(ts.pcfg, ss, ts.addr, spcodec.ReadCall, spcodec.WriteCall)
	db.DPrintf(db.TEST, "srv %v\n", srv.MyAddr())

	smgr := sessclnt.NewMgr(sp.ROOTREALM.String())
	req := sp.NewTattach(0, sp.NoFid, "clnt", 0, path.Path{})
	rep, err := smgr.RPC(sp.Taddrs{srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)
	srv.CloseListener()
}

type TstateSp struct {
	*Tstate
	srv  *sesssrv.SessSrv
	clnt *sessclnt.Mgr
}

func newTstateSp(t *testing.T) *TstateSp {
	ts := &TstateSp{}
	ts.Tstate = newTstate(t)
	et := ephemeralmap.NewEphemeralMap()
	root := dir.NewRootDir(ctx.NewCtxNull(), memfs.NewInode, nil)
	ts.srv = sesssrv.NewSessSrv(ts.pcfg, root, ts.addr, protsrv.NewProtServer, et, nil)
	ts.clnt = sessclnt.NewMgr(sp.ROOTREALM.String())
	return ts
}

func (ts *TstateSp) shutdown() {
	scs := ts.clnt.SessClnts()
	for _, sc := range scs {
		err := sc.Close()
		assert.Nil(ts.T, err)
	}
}

func TestConnectMfsSrv(t *testing.T) {
	ts := newTstateSp(t)
	req := sp.NewTattach(0, sp.NoFid, "clnt", 0, path.Path{})
	rep, err := ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)
	ts.srv.StopServing()
}

func TestDisconnectMfsSrv(t *testing.T) {
	ts := newTstateSp(t)
	req := sp.NewTattach(0, sp.NoFid, "clnt", 0, path.Path{})
	rep, err := ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)

	sess, err := ts.clnt.LookupSessClnt(sp.Taddrs{ts.srv.MyAddr()})
	assert.Nil(t, err)
	assert.True(t, sess.IsConnected())

	ssess, ok := ts.srv.GetSessionTable().Lookup(sess.SessId())
	assert.True(t, ok)
	assert.True(t, ssess.IsConnected())

	// check if session isn't timed out
	time.Sleep(3 * sp.Conf.Session.TIMEOUT)

	assert.True(t, sess.IsConnected())

	// client disconnects session
	ts.shutdown()

	assert.False(t, sess.IsConnected())

	// allow server session to timeout
	time.Sleep(2 * sp.Conf.Session.TIMEOUT)

	assert.False(t, sess.IsConnected())
}
