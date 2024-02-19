package sessclnt_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"

	"sigmaos/auth"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/dir"
	"sigmaos/ephemeralmap"
	"sigmaos/keys"
	"sigmaos/memfs"
	"sigmaos/netsrv"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/protsrv"
	"sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sessclnt"
	"sigmaos/sessp"
	"sigmaos/sesssrv"
	sp "sigmaos/sigmap"
	"sigmaos/sigmaprotsrv"
	"sigmaos/spcodec"
)

type SessSrv struct {
	crash int
}

func (ss *SessSrv) ReportError(conn sigmaprotsrv.Conn, err error) {
	db.DPrintf(db.TEST, "Server ReportError sid %v err %v\n", conn.GetSessId(), err)
}

func (ss *SessSrv) ServeRequest(conn sigmaprotsrv.Conn, req demux.CallI) (demux.CallI, *serr.Err) {
	fcm := req.(*sessp.FcallMsg)
	qid := sp.NewQidPerm(0777, 0, 0)
	if fcm.Type() == sessp.TTwatch {
		time.Sleep(1 * time.Second)
		conn.CloseConnTest()
		msg := &sp.Ropen{Qid: qid}
		rep := sessp.NewFcallMsgReply(fcm, msg)
		return rep, nil
	} else {
		msg := &sp.Rattach{Qid: qid}
		rep := sessp.NewFcallMsgReply(fcm, msg)
		r := rand.Int64(100)
		if r < uint64(ss.crash) {
			conn.CloseConnTest()
		}
		return rep, nil
	}
}

type Tstate struct {
	T    *testing.T
	pe   *proc.ProcEnv
	addr *sp.Taddr
}

func newTstate(t *testing.T) (*Tstate, error) {
	key, err := keys.NewSymmetricKey(sp.KEY_LEN)
	assert.Nil(t, err, "Err NewKey: %v", err)
	pubkey, err := auth.NewPublicKey[*jwt.SigningMethodHMAC](jwt.SigningMethodHS256, key.B64())
	assert.Nil(t, err, "Err NewPublicKey: %v", err)
	kmgr := keys.NewKeyMgr(keys.WithConstGetKeyFn(pubkey))
	as, err1 := auth.NewAuthSrv[*jwt.SigningMethodHMAC](jwt.SigningMethodHS256, "test", proc.NOT_SET, kmgr)
	if err1 != nil {
		return nil, err1
	}
	s3secrets, err1 := auth.GetAWSSecrets()
	if err1 != nil {
		db.DPrintf(db.ERROR, "Failed to load AWS secrets %v", err1)
		return nil, err1
	}
	secrets := map[string]*proc.ProcSecretProto{"s3": s3secrets}
	pe := proc.NewTestProcEnv(sp.ROOTREALM, secrets, sp.LOCALHOST, sp.LOCALHOST, sp.LOCALHOST, "", false, false)
	pe.Program = "srv"
	proc.SetSigmaDebugPid(pe.GetPID().String())
	if err := as.MintAndSetToken(pe); err != nil {
		db.DPrintf(db.ERROR, "Error NewToken: %v", err)
		return nil, err
	}
	addr := sp.NewTaddr(sp.NO_IP, sp.INNER_CONTAINER_IP, 1110)
	proc.SetSigmaDebugPid(pe.GetPID().String())
	return &Tstate{
		T:    t,
		pe:   pe,
		addr: addr,
	}, nil
}

type TstateSrv struct {
	*Tstate
	srv  *netsrv.NetServer
	clnt *sessclnt.Mgr
}

func newTstateSrv(t *testing.T, crash int) (*TstateSrv, error) {
	tsi, err := newTstate(t)
	if err != nil {
		return nil, err
	}
	ts := &TstateSrv{Tstate: tsi}
	ss := &SessSrv{crash}
	ts.srv = netsrv.NewNetServer(ts.pe, ss, ts.addr, spcodec.ReadCall, spcodec.WriteCall)
	db.DPrintf(db.TEST, "srv %v\n", ts.srv.MyAddr())
	ts.clnt = sessclnt.NewMgr(sp.ROOTREALM.String())
	return ts, nil
}

func TestCompile(t *testing.T) {
}

func TestConnectSessSrv(t *testing.T) {
	ts, err1 := newTstateSrv(t, 0)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	req := sp.NewTattach(0, sp.NoFid, ts.pe.GetPrincipal(), 0, path.Path{})
	rep, err := ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)
	ts.srv.CloseListener()
}

func TestDisconnectSessSrv(t *testing.T) {
	ts, err1 := newTstateSrv(t, 0)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}

	req := sp.NewTattach(0, sp.NoFid, ts.pe.GetPrincipal(), 0, path.Path{})
	_, err := ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	ch := make(chan *serr.Err)
	go func() {
		req := sp.NewTwatch(sp.NoFid)
		_, err := ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req, nil)
		ch <- err
	}()
	time.Sleep(1 * time.Second)
	r := <-ch
	assert.NotNil(t, r)
	ts.srv.CloseListener()
}

func testManyClients(t *testing.T, crash int) {
	const (
		NCLNT = 50
	)
	ts, err1 := newTstateSrv(t, crash)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	ch := make(chan bool)
	for i := 0; i < NCLNT; i++ {
		go func(i int) {
			for j := 0; true; j++ {
				select {
				case <-ch:
					ch <- true
					break
				default:
					req := sp.NewTattach(sp.Tfid(j), sp.NoFid, ts.pe.GetPrincipal(), sp.TclntId(i), path.Path{})
					_, err := ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req, nil)
					if err != nil && crash > 0 && serr.IsErrCode(err, serr.TErrUnreachable) {
						// wait for stop signal
						<-ch
						ch <- true
						break
					}
					assert.True(t, err == nil)
				}
			}
		}(i)
	}

	time.Sleep(1 * time.Second)

	for i := 0; i < NCLNT; i++ {
		ch <- true
		<-ch
	}
	ts.srv.CloseListener()
}

func TestManyClientsOK(t *testing.T) {
	testManyClients(t, 0)
}

func TestManyClientsCrash(t *testing.T) {
	const (
		N     = 20
		CRASH = 10
	)
	for i := 0; i < N; i++ {
		db.DPrintf(db.TEST, "=== TestManyClientsCrash: %d\n", i)
		testManyClients(t, CRASH)
	}
}

type TstateSp struct {
	*Tstate
	srv  *sesssrv.SessSrv
	clnt *sessclnt.Mgr
}

func newTstateSp(t *testing.T) (*TstateSp, error) {
	tsi, err := newTstate(t)
	if err != nil {
		return nil, err
	}
	ts := &TstateSp{}
	ts.Tstate = tsi
	et := ephemeralmap.NewEphemeralMap()
	root := dir.NewRootDir(ctx.NewCtxNull(), memfs.NewInode, nil)
	key, err := keys.NewSymmetricKey(sp.KEY_LEN)
	assert.Nil(t, err, "Err NewKey: %v", err)
	pubkey, err := auth.NewPublicKey[*jwt.SigningMethodHMAC](jwt.SigningMethodHS256, key.B64())
	assert.Nil(t, err, "Err NewPublicKey: %v", err)
	kmgr := keys.NewKeyMgr(keys.WithConstGetKeyFn(pubkey))
	as, err1 := auth.NewAuthSrv[*jwt.SigningMethodHMAC](jwt.SigningMethodHS256, "test", proc.NOT_SET, kmgr)
	if err1 != nil {
		return nil, err1
	}
	ts.srv = sesssrv.NewSessSrv(ts.pe, as, root, ts.addr, protsrv.NewProtServer, et, nil)
	ts.clnt = sessclnt.NewMgr(sp.ROOTREALM.String())
	return ts, nil
}

func (ts *TstateSp) shutdown() {
	scs := ts.clnt.SessClnts()
	for _, sc := range scs {
		err := sc.Close()
		assert.Nil(ts.T, err)
	}
}

func TestConnectMfsSrv(t *testing.T) {
	ts, err1 := newTstateSp(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	req := sp.NewTattach(0, sp.NoFid, ts.pe.GetPrincipal(), 0, path.Path{})
	rep, err := ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)
	ts.srv.StopServing()
}

func TestDisconnectMfsSrv(t *testing.T) {
	ts, err1 := newTstateSp(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	req := sp.NewTattach(0, sp.NoFid, ts.pe.GetPrincipal(), 0, path.Path{})
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
