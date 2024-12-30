package clnt_test

import (
	"bufio"
	"encoding/binary"
	"flag"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/util/io/demux"
	"sigmaos/netclnt"
	"sigmaos/netsrv"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	spcodec "sigmaos/session/codec"
	"sigmaos/test"
)

var srvaddr string

func init() {
	flag.StringVar(&srvaddr, "srvaddr", sp.NOT_SET, "service addr")
}

const (
	// For latency measurements
	// REQBUFSZ = 100
	// REPBUFSZ = 100
	// TOTAL    = 10 * sp.MBYTE

	// For tput measurements
	REQBUFSZ = 1 * sp.MBYTE // 128 * sp.KBYTE
	REPBUFSZ = 10
	TOTAL    = 1000 * sp.MBYTE
)

func measureProtobuf(t *testing.T, fc *sessp.FcallMsg) {
	const N = 100000
	t0 := time.Now()
	for i := 0; i < N; i++ {
		m, err := proto.Marshal(fc.Msg.(proto.Message))
		assert.Nil(t, err)
		f, err := proto.Marshal(fc.Fc)
		assert.Nil(t, err)
		fm := sessp.NewFcallMsgNull()
		err = proto.Unmarshal(f, fm.Fc)
		assert.Nil(t, err)
		msg, error := spcodec.NewMsg(fm.Type())
		assert.Nil(t, error)
		msg0 := msg.(proto.Message)
		err = proto.Unmarshal(m, msg0)
		assert.Nil(t, err)
		fm.Msg = msg
		// db.DPrintf(db.TEST, "fm %v\n", fm)
	}
	db.DPrintf(db.ALWAYS, "proto %v %v usec (%d iter)\n", fc.Type(), time.Since(t0).Microseconds(), N)
}

func TestProtobuf(t *testing.T) {
	seqcntr := new(sessp.Tseqcntr)
	req := sp.NewTwriteread(sp.NoFid)
	fc := sessp.NewFcallMsg(req, sessp.IoVec{test.NewBuf(REQBUFSZ)}, 1, seqcntr)
	measureProtobuf(t, fc)
	f := sp.NullFence()
	req1 := sp.NewTwriteF(sp.NoFid, 0, f)
	fc = sessp.NewFcallMsg(req1, sessp.IoVec{test.NewBuf(REQBUFSZ)}, 1, seqcntr)
	measureProtobuf(t, fc)
}

type data struct {
	a0 uint64
	a1 uint64
	a2 uint64
	a3 uint64
	a4 uint64
}

func f(n uint64, d data) uint64 {
	if n == 0 {
		return n + d.a0 + d.a1 + d.a2 + d.a3 + d.a4
	} else {
		return f(n-1, data{d.a0 + n, d.a1 + n, d.a2 + 2*n, d.a3 + 3*n, d.a4 + 4*n})
	}
}

func TestStack(t *testing.T) {
	const N = 1000000
	const M = 9 // with 8 no stack grows

	ch := make(chan uint64, N)
	t0 := time.Now()
	for i := 0; i < N; i++ {
		go func(i int) {
			r := f(M, data{uint64(i), 1, 1, 1, 1})
			ch <- r
		}(i)
	}
	for i := 0; i < N; i++ {
		<-ch
	}
	db.DPrintf(db.ALWAYS, "stack %v usec\n", time.Since(t0).Microseconds())
}

type call struct {
	buf []byte
}

func (c *call) Tag() sessp.Ttag {
	return 0
}

type transport struct {
	rdr  io.Reader
	wrt  *bufio.Writer
	iovm *demux.IoVecMap
}

func newTransport(conn net.Conn, iovm *demux.IoVecMap) demux.TransportI {
	return &transport{
		rdr:  bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN),
		wrt:  bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN),
		iovm: iovm,
	}
}

func (t *transport) ReadCall() (demux.CallI, *serr.Err) {
	var l uint32

	if err := binary.Read(t.rdr, binary.LittleEndian, &l); err != nil {
		return nil, serr.NewErr(serr.TErrUnreachable, err)
	}
	l = l - 4
	if l < 0 {
		return nil, serr.NewErr(serr.TErrUnreachable, "readMsg too short")
	}
	frame := make(sessp.Tframe, l)
	n, e := io.ReadFull(t.rdr, frame)
	if n != len(frame) {
		return nil, serr.NewErr(serr.TErrUnreachable, e)
	}
	return &call{buf: frame}, nil
}

func (t *transport) WriteCall(c demux.CallI) *serr.Err {
	call := c.(*call)

	l := uint32(len(call.buf) + 4) // +4 because that is how 9P wants it
	if err := binary.Write(t.wrt, binary.LittleEndian, l); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	if n, err := t.wrt.Write(call.buf); err != nil || n != len(call.buf) {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	if err := t.wrt.Flush(); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}

type netConn struct {
	conn net.Conn
}

func (nc *netConn) ServeRequest(req demux.CallI) (demux.CallI, *serr.Err) {
	//	db.DPrintf(db.TEST, "serve %v\n", req)
	var rep demux.CallI
	switch r := req.(type) {
	case *call:
		rep = &call{buf: r.buf[0:REPBUFSZ]}
	case *sessp.FcallMsg:
		rep = r
	case *sessp.PartMarshaledMsg:
		fcm := &sessp.FcallMsg{
			Fc:  r.Fcm.Fc,
			Msg: r.Fcm.Msg,
			Iov: sessp.IoVec{r.Fcm.Iov[0][0:REPBUFSZ]},
		}
		rep = &sessp.PartMarshaledMsg{
			Fcm:          fcm,
			MarshaledFcm: r.MarshaledFcm,
		}
	default:
		panic("ServeRequest")
	}
	return rep, nil
}

func (nc *netConn) ReportError(err error) {
}

type TstateNet struct {
	*test.TstateMin
	srv     *netsrv.NetServer
	clnt    *netclnt.NetClnt
	dmx     *demux.DemuxClnt
	mktrans func(net.Conn, *demux.IoVecMap) demux.TransportI
}

func (ts *TstateNet) NewConn(conn net.Conn) *demux.DemuxSrv {
	nc := &netConn{conn}
	iovm := demux.NewIoVecMap()
	return demux.NewDemuxSrv(nc, ts.mktrans(conn, iovm))
}

func newTstateNet(t *testing.T, mktrans func(net.Conn, *demux.IoVecMap) demux.TransportI) *TstateNet {
	ts := &TstateNet{
		TstateMin: test.NewTstateMin(t),
		mktrans:   mktrans,
	}
	ts.srv = netsrv.NewNetServer(ts.PE, ts.Addr, ts)

	db.DPrintf(db.TEST, "srv %v\n", ts.srv.MyAddr())

	nc, err := netclnt.NewNetClnt(sp.ROOTREALM.String(), sp.Taddrs{ts.srv.MyAddr()})
	assert.Nil(t, err)
	iovm := demux.NewIoVecMap()
	ts.dmx = demux.NewDemuxClnt(mktrans(nc.Conn(), iovm), iovm)
	return ts
}

func TestNetClntPerfFrame(t *testing.T) {
	ts := newTstateNet(t, newTransport)
	c := &call{buf: test.NewBuf(REQBUFSZ)}

	t0 := time.Now()
	n := TOTAL / REQBUFSZ
	for i := 0; i < n; i++ {
		d, err := ts.dmx.SendReceive(c, nil)
		assert.Nil(t, err)
		call := d.(*call)
		assert.True(t, len(call.buf) == REPBUFSZ)
	}
	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms (%v us per iter, %d iter) tput %v\n", humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/REQBUFSZ), n, test.TputStr(TOTAL, ms))

	ts.srv.CloseListener()
}

func TestNetClntPerfFcall(t *testing.T) {
	ts := newTstateNet(t, spcodec.NewTransport)
	req := sp.NewTwriteread(sp.NoFid)
	seqcntr := new(sessp.Tseqcntr)
	fcm := sessp.NewFcallMsg(req, sessp.IoVec{test.NewBuf(REQBUFSZ)}, 1, seqcntr)
	pfcm := spcodec.NewPartMarshaledMsg(fcm)
	t0 := time.Now()
	n := TOTAL / REQBUFSZ
	for i := 0; i < n; i++ {
		c, err := ts.dmx.SendReceive(pfcm, nil)
		assert.Nil(t, err)
		fcm := c.(*sessp.PartMarshaledMsg)
		assert.True(t, len(fcm.Fcm.Iov[0]) == REPBUFSZ)
	}
	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms (%v us per iter, %d iter) tput %v\n", humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/REQBUFSZ), n, test.TputStr(TOTAL, ms))

	ts.srv.CloseListener()
}

func runSrv(t *testing.T, typ, arg string) func() {
	var socket net.Listener
	ch := make(chan bool)
	if typ == "unix" {
		db.DPrintf(db.TEST, "local %v %v\n", typ, arg)
		err := os.Remove(arg)
		assert.True(t, err == nil || os.IsNotExist(err), "Err remove sock: %v", err)
		socket, err = net.Listen("unix", arg)
		assert.Nil(t, err)
		err = os.Chmod(arg, 0777)
		assert.Nil(t, err)
	} else {
		var err error
		socket, err = net.Listen(typ, arg)
		assert.Nil(t, err)
	}

	// Serve requests in another thread
	go func() {
		conn, err := socket.Accept()
		assert.Nil(t, err)
		tot := 0
		for {
			rb := make([]byte, REQBUFSZ)
			n, err := io.ReadFull(conn, rb)
			if err == io.EOF {
				db.DPrintf(db.TEST, "tot %d\n", tot)
				break
			}
			tot += n
			if n != len(rb) || err != nil {
				db.DFatalf("Err read: len %v err %v", n, err)
			}
			conn.Write(rb[0:REPBUFSZ])
		}
		ch <- true
	}()
	return func() {
		<-ch
		socket.Close()
	}
}

func runClnt(t *testing.T, typ, arg string) {
	conn, err := net.Dial(typ, arg)
	assert.Nil(t, err)

	sz := sp.Tlength(TOTAL)
	buf := test.NewBuf(REQBUFSZ)
	t0 := time.Now()
	n := TOTAL / REQBUFSZ
	for i := 0; i < n; i++ {
		n, err := conn.Write(buf)
		assert.Nil(t, err)
		assert.Equal(t, REQBUFSZ, n)
		rb := make([]byte, REPBUFSZ)
		m, err := io.ReadFull(conn, rb)
		assert.Nil(t, err)
		assert.True(t, m == REPBUFSZ)
	}
	conn.Close()

	tot := uint64(sz)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "%v wrote %v bytes in %v ms (%v us per iter, %d iter) tput %v\n", typ, humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/REQBUFSZ), n, test.TputStr(sz, ms))
}

func testLocalPerf(t *testing.T, typ, arg string) {
	waitf := runSrv(t, typ, arg)
	defer waitf()
	time.Sleep(1 * time.Second)
	runClnt(t, typ, arg)
	if typ == "unix" {
		err := os.Remove(arg)
		assert.True(t, err == nil || os.IsNotExist(err), "Err remove sock: %v", err)
	}
}

func TestSocketPerf(t *testing.T) {
	const (
		SOCKPATH = "/tmp/test-perf-socket"
	)
	testLocalPerf(t, "unix", SOCKPATH)
}

func TestTCPPerf(t *testing.T) {
	testLocalPerf(t, "tcp", "127.0.0.1:4444")
}

func TestTCPPerfClnt(t *testing.T) {
	if srvaddr == sp.NOT_SET {
		db.DPrintf(db.TEST, "Srv addr not set. Skipping test.")
		return
	}
	// Run the client
	runClnt(t, "tcp", srvaddr)
}

func TestTCPPerfSrv(t *testing.T) {
	if srvaddr == sp.NOT_SET {
		db.DPrintf(db.TEST, "Srv addr not set. Skipping test.")
		return
	}
	// Run the server
	waitf := runSrv(t, "tcp", srvaddr)
	defer waitf()
}
