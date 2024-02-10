package sessclnt_test

import (
	"bufio"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
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

type TstateSrv struct {
	*Tstate
	srv  *netsrv.NetServer
	clnt *sessclnt.Mgr
}

func newTstateSrv(t *testing.T, crash int) *TstateSrv {
	ts := &TstateSrv{Tstate: newTstate(t)}
	ss := &SessSrv{crash}
	ts.srv = netsrv.NewNetServer(ts.pcfg, ss, ts.addr, spcodec.ReadCall, spcodec.WriteCall)
	db.DPrintf(db.TEST, "srv %v\n", ts.srv.MyAddr())
	ts.clnt = sessclnt.NewMgr(sp.ROOTREALM.String())
	return ts
}

func TestConnectSessSrv(t *testing.T) {
	ts := newTstateSrv(t, 0)

	req := sp.NewTattach(0, sp.NoFid, "clnt", 0, path.Path{})
	rep, err := ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req, nil)
	assert.Nil(t, err)
	db.DPrintf(db.TEST, "fcall %v\n", rep)
	ts.srv.CloseListener()
}

func TestDisconnectSessSrv(t *testing.T) {
	ts := newTstateSrv(t, 0)
	req := sp.NewTattach(0, sp.NoFid, "clnt", 0, path.Path{})
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
					req := sp.NewTattach(sp.Tfid(j), sp.NoFid, "clnt", sp.TclntId(i), path.Path{})
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

	req1 := sp.NewTwriteread(sp.NoFid)
	iov := sessp.NewIoVec([][]byte{make([]byte, 10)})
	rep, err = ts.clnt.RPC(sp.Taddrs{ts.srv.MyAddr()}, req1, iov)
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

func TestWriteSocketPerfSingle(t *testing.T) {
	const (
		SOCKPATH = "/tmp/test-perf-socket"
		WRITESZ  = 4096
	)

	err := os.Remove(SOCKPATH)
	assert.True(t, err == nil || os.IsNotExist(err), "Err remove sock: %v", err)

	socket, err := net.Listen("unix", SOCKPATH)
	assert.Nil(t, err)
	err = os.Chmod(SOCKPATH, 0777)
	assert.Nil(t, err)

	buf := test.NewBuf(WRITESZ)
	ch := make(chan bool)
	// Serve requests in another thread
	go func() {
		conn, err := socket.Accept()
		assert.Nil(t, err)
		rdr := bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN)
		rb := test.NewBuf(WRITESZ)
		for {
			n, err := io.ReadFull(rdr, rb)
			if err == io.EOF {
				break
			}
			if n != len(rb) || err != nil {
				db.DFatalf("Err read: len %v err %v", n, err)
			}
		}
		ch <- true

	}()

	conn, err := net.Dial("unix", SOCKPATH)
	assert.Nil(t, err)

	sz := sp.Tlength(100 * sp.MBYTE)
	wrt := bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN)
	t0 := time.Now()
	err = test.Writer(t, wrt, buf, sz)
	assert.Nil(t, err)
	err = wrt.Flush()
	assert.Nil(t, err)

	conn.Close()

	<-ch

	tot := uint64(sz)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms tput %v\n", humanize.Bytes(tot), ms, test.TputStr(sz, ms))

	err = os.Remove(SOCKPATH)
	assert.True(t, err == nil || os.IsNotExist(err), "Err remove sock: %v", err)
}
