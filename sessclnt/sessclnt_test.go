package sessclnt_test

import (
	"flag"
	"net"
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
	"sigmaos/keys"
	"sigmaos/memfs"
	"sigmaos/netproxyclnt"
	"sigmaos/netsrv"
	"sigmaos/path"
	"sigmaos/rand"
	"sigmaos/serr"
	"sigmaos/sessclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/sigmapsrv"
	"sigmaos/spcodec"
	"sigmaos/test"
)

const (
	READ_TEST_STR = "PDOS!"
)

var srvaddr string

func init() {
	flag.StringVar(&srvaddr, "srvaddr", sp.NOT_SET, "service addr")
}

type SessSrv struct {
	crash int
	conn  net.Conn
}

func (ss *SessSrv) ReportError(err error) {
	db.DPrintf(db.TEST, "Server ReportError sid %v err %v\n", ss, err)
}

func (ss *SessSrv) ServeRequest(req demux.CallI) (demux.CallI, *serr.Err) {
	fcm := req.(*sessp.PartMarshaledMsg)

	if err := spcodec.UnmarshalMsg(fcm); err != nil {
		return nil, err
	}

	//	db.DPrintf(db.TEST, "serve %v\n", fcm)

	qid := sp.NewQidPerm(0777, 0, 0)
	var rep *sessp.FcallMsg
	switch fcm.Fcm.Type() {
	case sessp.TTwatch:
		time.Sleep(1 * time.Second)
		ss.conn.Close()
		msg := &sp.Ropen{Qid: qid}
		rep = sessp.NewFcallMsgReply(fcm.Fcm, msg)
	case sessp.TTwrite:
		msg := &sp.Rwrite{Count: uint32(len(fcm.Fcm.Iov[0]))}
		rep = sessp.NewFcallMsgReply(fcm.Fcm, msg)
	case sessp.TTreadF:
		msg := &sp.Rread{}
		rep = sessp.NewFcallMsgReply(fcm.Fcm, msg)
		rep.Iov = sessp.IoVec{[]byte(READ_TEST_STR)}
	case sessp.TTwriteread:
		msg := &sp.Rread{}
		rep = sessp.NewFcallMsgReply(fcm.Fcm, msg)
		rep.Iov = sessp.IoVec{fcm.Fcm.Iov[0][0:REPBUFSZ]}
	default:
		msg := &sp.Rattach{Qid: qid}
		rep = sessp.NewFcallMsgReply(fcm.Fcm, msg)
		r := rand.Int64(100)
		if r < uint64(ss.crash) {
			ss.conn.Close()
		}
	}
	pmfc := spcodec.NewPartMarshaledMsg(rep)
	return pmfc, nil
}

type TstateSrv struct {
	*test.TstateMin
	srv   *netsrv.NetServer
	clnt  *sessclnt.Mgr
	crash int
}

func newTstateClntAddr(t *testing.T, addr *sp.Taddr, crash int) *TstateSrv {
	ts := &TstateSrv{TstateMin: test.NewTstateMinAddr(t, addr), crash: crash}
	ts.clnt = sessclnt.NewMgr(ts.PE, netproxyclnt.NewNetProxyClnt(ts.PE, nil))
	return ts
}

func newTstateSrvAddr(t *testing.T, addr *sp.Taddr, crash int) *TstateSrv {
	ts := newTstateClntAddr(t, addr, crash)
	db.DPrintf(db.ALWAYS, "pe: %v", ts.PE)
	ts.srv = netsrv.NewNetServer(ts.PE, netproxyclnt.NewNetProxyClnt(ts.PE, ts.AMgr), ts.Addr, ts)
	db.DPrintf(db.TEST, "srv %v\n", ts.srv.GetEndpoint())
	return ts
}

func newTstateSrv(t *testing.T, crash int) *TstateSrv {
	addr := sp.NewTaddr(sp.NO_IP, sp.INNER_CONTAINER_IP, 1110)
	return newTstateSrvAddr(t, addr, crash)
}

func (ts *TstateSrv) NewConn(conn net.Conn) *demux.DemuxSrv {
	ss := &SessSrv{crash: ts.crash, conn: conn}
	iovm := demux.NewIoVecMap()
	return demux.NewDemuxSrv(ss, spcodec.NewTransport(conn, iovm))
}

func TestCompile(t *testing.T) {
}

func TestConnectSessSrv(t *testing.T) {
	ts := newTstateSrv(t, 0)
	req := sp.NewTattach(0, sp.NoFid, ts.PE.GetPrincipal(), 0, path.Path{})
	rep, err := ts.clnt.RPC(ts.srv.GetEndpoint(), req, nil, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)
	ts.srv.CloseListener()
}

func TestDisconnectSessSrv(t *testing.T) {
	ts := newTstateSrv(t, 0)
	req := sp.NewTattach(0, sp.NoFid, ts.PE.GetPrincipal(), 0, path.Path{})
	_, err := ts.clnt.RPC(ts.srv.GetEndpoint(), req, nil, nil)
	assert.Nil(t, err)
	ch := make(chan *serr.Err)
	go func() {
		req := sp.NewTwatch(sp.NoFid)
		_, err := ts.clnt.RPC(ts.srv.GetEndpoint(), req, nil, nil)
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
	stop := make(chan bool)
	for i := 0; i < NCLNT; i++ {
		go func(i int) {
			done := false
			for j := 0; !done; j++ {
				select {
				case <-stop:
					ch <- true
					done = true
				default:
					req := sp.NewTattach(sp.Tfid(j), sp.NoFid, ts.PE.GetPrincipal(), sp.TclntId(i), path.Path{})
					_, err := ts.clnt.RPC(ts.srv.GetEndpoint(), req, nil, nil)
					if err != nil && crash > 0 && serr.IsErrCode(err, serr.TErrUnreachable) {
						// wait for stop signal
						<-stop
						ch <- true
						done = true
					} else {
						assert.True(t, err == nil)
					}
				}
			}
		}(i)
	}

	time.Sleep(2 * time.Second)

	for i := 0; i < NCLNT; i++ {
		stop <- true
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

const (
	// For latency measurement
	//REQBUFSZ = 100           // 64 * sp.KBYTE
	//REPBUFSZ = 100           // 64 * sp.KBYTE
	//TOTAL    = 10 * sp.MBYTE // 1000 * sp.MBYTE

	// For tput measurement
	REQBUFSZ = 1 * sp.MBYTE
	REPBUFSZ = 10
	TOTAL    = 1000 * sp.MBYTE
)

type Awriter struct {
	nthread int
	clnt    *sessclnt.Mgr
	ep      *sp.Tendpoint
	req     chan sessp.IoVec
	rep     chan error
	err     error
	wg      sync.WaitGroup
}

func NewAwriter(n int, clnt *sessclnt.Mgr, ep *sp.Tendpoint) *Awriter {
	req := make(chan sessp.IoVec)
	rep := make(chan error)
	awrt := &Awriter{
		nthread: n,
		clnt:    clnt,
		ep:      ep,
		req:     req,
		rep:     rep,
	}
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
		req := sp.NewTwriteread(sp.NoFid)
		_, err := awrt.clnt.RPC(awrt.ep, req, iov, nil)
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

func TestRead(t *testing.T) {
	ts := newTstateSrv(t, 0)
	buf := make([]byte, len(READ_TEST_STR))
	req := sp.NewReadF(sp.NoFid, 0, sp.Tsize(len(buf)), sp.NullFence())
	iov := sessp.IoVec{buf}
	_, err := ts.clnt.RPC(ts.srv.GetEndpoint(), req, nil, iov)
	assert.Nil(t, err, "Err Read: %v", err)
	assert.Equal(t, READ_TEST_STR, string(buf))
	ts.srv.CloseListener()
}

func TestPerfSessSrvAsync(t *testing.T) {
	ts := newTstateSrv(t, 0)
	buf := test.NewBuf(REQBUFSZ)

	aw := NewAwriter(1, ts.clnt, ts.srv.GetEndpoint())

	t0 := time.Now()

	n := TOTAL / REQBUFSZ
	for i := 0; i < n; i++ {
		err := aw.Write(sessp.IoVec{buf})
		assert.Nil(t, err)
	}

	aw.Close()

	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms (%v us per iter, %d iter) tput %v\n", humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/REQBUFSZ), n, test.TputStr(TOTAL, ms))

	ts.srv.CloseListener()
}

func TestPerfSessSrvAsyncSrv(t *testing.T) {
	if srvaddr == sp.NOT_SET {
		db.DPrintf(db.TEST, "Srv addr not set. Skipping test.")
		return
	}

	h, pstr, err := net.SplitHostPort(srvaddr)
	assert.Nil(t, err, "Err split host port: %v", err)
	p, err := sp.ParsePort(pstr)
	assert.Nil(t, err, "Err parse port: %v", err)

	addr := sp.NewTaddr(sp.Tip(h), sp.OUTER_CONTAINER_IP, p)

	ts := newTstateSrvAddr(t, addr, 0)
	defer ts.srv.CloseListener()
	time.Sleep(20 * time.Second)

}

func TestPerfSessSrvAsyncClnt(t *testing.T) {
	if srvaddr == sp.NOT_SET {
		db.DPrintf(db.TEST, "Srv addr not set. Skipping test.")
		return
	}

	h, pstr, err := net.SplitHostPort(srvaddr)
	assert.Nil(t, err, "Err split host port: %v", err)
	p, err := sp.ParsePort(pstr)
	assert.Nil(t, err, "Err parse port: %v", err)

	addr := sp.NewTaddr(sp.Tip(h), sp.OUTER_CONTAINER_IP, p)
	ts := newTstateClntAddr(t, addr, 0)
	aw := NewAwriter(1, ts.clnt, ts.srv.GetEndpoint())
	buf := test.NewBuf(REQBUFSZ)

	t0 := time.Now()

	n := TOTAL / REQBUFSZ
	for i := 0; i < n; i++ {
		err := aw.Write(sessp.IoVec{buf})
		assert.Nil(t, err)
	}

	aw.Close()

	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms (%v us per iter, %d iter) tput %v\n", humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/REQBUFSZ), n, test.TputStr(TOTAL, ms))

}

func TestPerfSessSrvSync(t *testing.T) {
	ts := newTstateSrv(t, 0)
	buf := test.NewBuf(REQBUFSZ)

	t0 := time.Now()

	n := TOTAL / REQBUFSZ
	for i := 0; i < TOTAL/REQBUFSZ; i++ {
		req := sp.NewTwriteread(sp.NoFid)
		rep, err := ts.clnt.RPC(ts.srv.GetEndpoint(), req, sessp.IoVec{buf}, nil)
		assert.Nil(t, err)
		assert.True(t, REPBUFSZ == len(rep.Iov[0]))
	}

	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms (%v us per iter, %d iter) tput %v\n", humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/REQBUFSZ), n, test.TputStr(TOTAL, ms))

	ts.srv.CloseListener()
}

func TestPerfSessSrvSyncSrv(t *testing.T) {
	if srvaddr == sp.NOT_SET {
		db.DPrintf(db.TEST, "Srv addr not set. Skipping test.")
		return
	}

	h, pstr, err := net.SplitHostPort(srvaddr)
	assert.Nil(t, err, "Err split host port: %v", err)
	p, err := sp.ParsePort(pstr)
	assert.Nil(t, err, "Err parse port: %v", err)

	addr := sp.NewTaddr(sp.Tip(h), sp.OUTER_CONTAINER_IP, p)

	ts := newTstateSrvAddr(t, addr, 0)
	defer ts.srv.CloseListener()
	time.Sleep(20 * time.Second)
}

func TestPerfSessSrvSyncClnt(t *testing.T) {
	if srvaddr == sp.NOT_SET {
		db.DPrintf(db.TEST, "Srv addr not set. Skipping test.")
		return
	}

	h, pstr, err := net.SplitHostPort(srvaddr)
	assert.Nil(t, err, "Err split host port: %v", err)
	p, err := sp.ParsePort(pstr)
	assert.Nil(t, err, "Err parse port: %v", err)

	addr := sp.NewTaddr(sp.Tip(h), sp.OUTER_CONTAINER_IP, p)

	ts := newTstateClntAddr(t, addr, 0)
	buf := test.NewBuf(REQBUFSZ)

	t0 := time.Now()

	n := TOTAL / REQBUFSZ
	for i := 0; i < TOTAL/REQBUFSZ; i++ {
		req := sp.NewTwriteread(sp.NoFid)
		rep, err := ts.clnt.RPC(ts.srv.GetEndpoint(), req, sessp.IoVec{buf}, nil)
		assert.Nil(t, err)
		assert.True(t, REPBUFSZ == len(rep.Iov[0]))
	}

	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms (%v us per iter, %d iter) tput %v\n", humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/REQBUFSZ), n, test.TputStr(TOTAL, ms))
}

//
// sessclnt with a sigmap server
//

type TstateSp struct {
	*test.TstateMin
	srv     *sigmapsrv.SigmaPSrv
	clnt    *sessclnt.Mgr
	pubkey  auth.PublicKey
	privkey auth.PrivateKey
}

func newTstateSp(t *testing.T) *TstateSp {
	ts := &TstateSp{}
	ts.TstateMin = test.NewTstateMin(t)
	pubkey, privkey, err := keys.NewECDSAKey()
	assert.Nil(t, err, "Err NewKey: %v", err)
	ts.pubkey = pubkey
	ts.privkey = privkey
	kmgr := keys.NewKeyMgr(keys.WithConstGetKeyFn(ts.pubkey))
	kmgr.AddPrivateKey(sp.Tsigner(ts.PE.GetPID()), ts.privkey)
	as, err := auth.NewAuthMgr[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, sp.Tsigner(ts.PE.GetPID()), "", kmgr)
	assert.Nil(t, err, "Err NewAuthMgr: %v", err)
	err = as.MintAndSetProcToken(ts.PE)
	assert.Nil(t, err, "Err MintAndSetToken: %v", err)
	root := dir.NewRootDir(ctx.NewCtxNull(), memfs.NewInode, nil)
	ts.srv = sigmapsrv.NewSigmaPSrv(ts.PE, netproxyclnt.NewNetProxyClnt(ts.PE, as), root, as, ts.Addr, nil)
	ts.clnt = sessclnt.NewMgr(ts.PE, netproxyclnt.NewNetProxyClnt(ts.PE, nil))
	return ts
}

func (ts *TstateSp) shutdown() {
	scs := ts.clnt.SessClnts()
	for _, sc := range scs {
		err := sc.Close()
		assert.Nil(ts.T, err)
	}
}

func TestConnectSigmaPSrv(t *testing.T) {
	ts := newTstateSp(t)
	req := sp.NewTattach(0, sp.NoFid, ts.PE.GetPrincipal(), 0, path.Path{})
	rep, err := ts.clnt.RPC(ts.srv.GetEndpoint(), req, nil, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)

	req1 := sp.NewTwriteread(sp.NoFid)
	iov := sessp.NewIoVec([][]byte{make([]byte, 10)})
	rep, err = ts.clnt.RPC(ts.srv.GetEndpoint(), req1, iov, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)

	ts.srv.StopServing()
}

func TestDisconnectSigmaPSrv(t *testing.T) {
	ts := newTstateSp(t)
	req := sp.NewTattach(0, sp.NoFid, ts.PE.GetPrincipal(), 0, path.Path{})
	rep, err := ts.clnt.RPC(ts.srv.GetEndpoint(), req, nil, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)

	sess, err := ts.clnt.LookupSessClnt(ts.srv.GetEndpoint())
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
