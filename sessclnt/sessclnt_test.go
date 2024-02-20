package sessclnt_test

import (
	"sync"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
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
	"sigmaos/protsrv"
	"sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sessclnt"
	"sigmaos/sessp"
	"sigmaos/sesssrv"
	sp "sigmaos/sigmap"
	"sigmaos/sigmaprotsrv"
	"sigmaos/spcodec"
	"sigmaos/test"
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
	var rep *sessp.FcallMsg
	switch fcm.Type() {
	case sessp.TTwatch:
		time.Sleep(1 * time.Second)
		conn.CloseConnTest()
		msg := &sp.Ropen{Qid: qid}
		rep = sessp.NewFcallMsgReply(fcm, msg)
	case sessp.TTwrite:
		msg := &sp.Rwrite{Count: uint32(len(fcm.Iov[0]))}
		rep = sessp.NewFcallMsgReply(fcm, msg)
		return rep, nil
	default:
		msg := &sp.Rattach{Qid: qid}
		rep = sessp.NewFcallMsgReply(fcm, msg)
		r := rand.Int64(100)
		if r < uint64(ss.crash) {
			conn.CloseConnTest()
		}
	}
	pmfc := spcodec.NewPartMarshaledMsg(rep)
	return pmfc, nil
}

type TstateSrv struct {
	*test.TstateMin
	srv  *netsrv.NetServer
	clnt *sessclnt.Mgr
}

func newTstateSrv(t *testing.T, crash int) *TstateSrv {
	ts := &TstateSrv{TstateMin: test.NewTstateMin(t)}
	ss := &SessSrv{crash}
	ts.srv = netsrv.NewNetServer(ts.PE, ss, ts.Addr, spcodec.ReadCall, spcodec.WriteCall)
	db.DPrintf(db.TEST, "srv %v\n", ts.srv.MyAddr())
	ts.clnt = sessclnt.NewMgr(sp.ROOTREALM.String())
	return ts
}

func TestCompile(t *testing.T) {
}

func TestConnectSessSrv(t *testing.T) {
	ts := newTstateSrv(t, 0)
	req := sp.NewTattach(0, sp.NoFid, ts.PE.GetPrincipal(), 0, path.Path{})
	rep, err := ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)
	ts.srv.CloseListener()
}

func TestDisconnectSessSrv(t *testing.T) {
	ts := newTstateSrv(t, 0)
	req := sp.NewTattach(0, sp.NoFid, ts.PE.GetPrincipal(), 0, path.Path{})
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
	ts := newTstateSrv(t, crash)
	ch := make(chan bool)
	for i := 0; i < NCLNT; i++ {
		go func(i int) {
			for j := 0; true; j++ {
				select {
				case <-ch:
					ch <- true
					break
				default:
					req := sp.NewTattach(sp.Tfid(j), sp.NoFid, ts.PE.GetPrincipal(), sp.TclntId(i), path.Path{})
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
	*test.TstateMin
	srv  *sesssrv.SessSrv
	clnt *sessclnt.Mgr
}

func newTstateSp(t *testing.T) (*TstateSp, error) {
	ts := &TstateSp{}
	ts.TstateMin = test.NewTstateMin(t)
	et := ephemeralmap.NewEphemeralMap()
	root := dir.NewRootDir(ctx.NewCtxNull(), memfs.NewInode, nil)
	pubkey, privkey, err := keys.NewECDSAKey()
	if err != nil {
		db.DPrintf(db.ERROR, "Error NewECDSAKey: %v", err)
		return nil, err
	}
	kmgr := keys.NewKeyMgr(keys.WithConstGetKeyFn(pubkey))
	kmgr.AddPrivateKey(auth.SIGMA_DEPLOYMENT_MASTER_SIGNER, privkey)
	as, err1 := auth.NewAuthSrv[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, auth.SIGMA_DEPLOYMENT_MASTER_SIGNER, sp.NOT_SET, kmgr)
	if err1 != nil {
		db.DPrintf(db.ERROR, "Error NewAuthSrv: %v", err1)
		return nil, err1
	}
	ts.srv = sesssrv.NewSessSrv(ts.PE, as, root, ts.Addr, protsrv.NewProtServer, et, nil)
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
	req := sp.NewTattach(0, sp.NoFid, ts.PE.GetPrincipal(), 0, path.Path{})
	rep, err := ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)

	req1 := sp.NewTwriteread(sp.NoFid)
	iov := sessp.NewIoVec([][]byte{make([]byte, 10)})
	rep, err = ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req1, iov)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)

	ts.srv.StopServing()
}

func TestDisconnectMfsSrv(t *testing.T) {
	ts, err1 := newTstateSp(t)
	if !assert.Nil(t, err1, "Error New Tstate: %v", err1) {
		return
	}
	req := sp.NewTattach(0, sp.NoFid, ts.PE.GetPrincipal(), 0, path.Path{})
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

	ts.srv.StopServing()
}

const (
	BUFSZ = 64 * sp.KBYTE
	TOTAL = 1000 * sp.MBYTE
)

type Awriter struct {
	nthread int
	clnt    *sessclnt.Mgr
	addr    *sp.Taddr
	req     chan sessp.IoVec
	rep     chan error
	err     error
	wg      sync.WaitGroup
}

func NewAwriter(n int, clnt *sessclnt.Mgr, addr *sp.Taddr) *Awriter {
	req := make(chan sessp.IoVec)
	rep := make(chan error)
	awrt := &Awriter{nthread: n, clnt: clnt, addr: addr, req: req, rep: rep}
	for i := 0; i < n; i++ {
		go awrt.Writer()
	}
	go awrt.Collector()
	return awrt
}

func (awrt *Awriter) Writer() {
	for {
		iov, ok := <-awrt.req
		if !ok {
			return
		}
		req := sp.NewTwriteF(sp.NoFid, 0, sp.NullFence())
		_, err := awrt.clnt.RPC(sp.Taddrs{awrt.addr}, req, iov)
		if err != nil {
			awrt.rep <- err
		} else {
			awrt.rep <- nil
		}
	}
	db.DPrintf(db.TEST, "Writer closed\n")
}

func (awrt *Awriter) Collector() {
	for {
		r, ok := <-awrt.rep
		if !ok {
			return
		}
		awrt.wg.Done()
		if r != nil {
			db.DPrintf(db.TEST, "Writer call %v\n", r)
			awrt.err = r
		}
	}
}

func (awrt *Awriter) Write(iov sessp.IoVec) chan error {
	awrt.wg.Add(1)
	awrt.req <- iov
	return nil
}

func (awrt *Awriter) Close() error {
	db.DPrintf(db.TEST, "Close %v\n", awrt.wg)
	awrt.wg.Wait()
	close(awrt.req)
	close(awrt.rep)
	return nil
}

func TestPerfSessSrvAsync(t *testing.T) {
	const (
		TOTAL = 1000 * sp.MBYTE
	)
	ts := newTstateSrv(t, 0)
	buf := test.NewBuf(BUFSZ)

	aw := NewAwriter(1, ts.clnt, ts.srv.MyAddr())

	t0 := time.Now()

	for i := 0; i < TOTAL/BUFSZ; i++ {
		err := aw.Write(sessp.IoVec{buf})
		assert.Nil(t, err)
	}

	aw.Close()

	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms tput %v\n", humanize.Bytes(tot), ms, test.TputStr(TOTAL, ms))

	ts.srv.CloseListener()
}

func TestPerfSessSrvSync(t *testing.T) {
	ts := newTstateSrv(t, 0)
	buf := test.NewBuf(BUFSZ)

	t0 := time.Now()

	for i := 0; i < TOTAL/BUFSZ; i++ {
		req := sp.NewTwriteF(sp.NoFid, 0, sp.NullFence())
		iov := sessp.IoVec{buf}
		_, err := ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req, iov)
		assert.Nil(t, err)
	}

	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms tput %v\n", humanize.Bytes(tot), ms, test.TputStr(TOTAL, ms))

	ts.srv.CloseListener()
}
