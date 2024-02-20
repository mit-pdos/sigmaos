package netclnt_test

import (
	"bufio"
	// "bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/netclnt"
	"sigmaos/netsrv"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
	"sigmaos/test"
)

const (
	BUFSZ = 100           // 64 * sp.KBYTE
	TOTAL = 10 * sp.MBYTE // 1000 * sp.MBYTE
)

func TestProto(t *testing.T) {
	const N = 1000

	req := sp.NewTheartbeat(map[uint64]bool{uint64(1): true})
	fc := sessp.NewFcallMsg(req, sessp.IoVec{test.NewBuf(BUFSZ)}, 1, &seqno)

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
	db.DPrintf(db.ALWAYS, "proto %v usec\n", time.Since(t0).Microseconds())
}

type call struct {
	buf []byte
}

func (c *call) Tag() sessp.Ttag {
	return 0
}

func ReadCall(rdr io.Reader) (demux.CallI, *serr.Err) {
	var l uint32

	if err := binary.Read(rdr, binary.LittleEndian, &l); err != nil {
		return nil, serr.NewErr(serr.TErrUnreachable, err)
	}
	l = l - 4
	if l < 0 {
		return nil, serr.NewErr(serr.TErrUnreachable, "readMsg too short")
	}
	frame := make(sessp.Tframe, l)
	n, e := io.ReadFull(rdr, frame)
	if n != len(frame) {
		return nil, serr.NewErr(serr.TErrUnreachable, e)
	}
	return &call{buf: frame}, nil
}

func WriteCall(wr io.Writer, c demux.CallI) *serr.Err {
	call := c.(*call)
	wrt := wr.(*bufio.Writer)

	l := uint32(len(call.buf) + 4) // +4 because that is how 9P wants it
	if err := binary.Write(wrt, binary.LittleEndian, l); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	if n, err := wrt.Write(call.buf); err != nil || n != len(call.buf) {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	if err := wrt.Flush(); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}

var seqno sessp.Tseqno

func ReadFcall(rdr io.Reader) (demux.CallI, *serr.Err) {
	c, err := spcodec.ReadCall(rdr)
	return c, err
}

func WriteFcall(wr io.Writer, c demux.CallI) *serr.Err {
	fc := c.(*sessp.FcallMsg)
	pmfc := spcodec.NewPartMarshaledMsg(fc)
	return spcodec.WriteCall(wr, pmfc)
}

func ReadFcall1(rdr io.Reader) (demux.CallI, *serr.Err) {
	f, err := frame.ReadFrame(rdr)
	if err != nil {
		return nil, err
	}
	fm := sessp.NewFcallMsgNull()
	if err := proto.Unmarshal(f, fm.Fc); err != nil {
		db.DFatalf("error decoding fcall %v", err)
	}

	// db.DPrintf(db.TEST, "unmarshall %v %d\n", fm.Fc, len(f))

	msg, error1 := spcodec.NewMsg(fm.Type())
	if error1 != nil {
		db.DFatalf("error type %v %v", msg, error1)
	}
	m := msg.(proto.Message)

	b := make(sessp.Tframe, fm.Fc.Len)
	n, error := io.ReadFull(rdr, b)
	if n != len(b) {
		return nil, serr.NewErr(serr.TErrUnreachable, error)
	}
	if err := proto.Unmarshal(b, m); err != nil {
		db.DFatalf("error decoding msg %v", err)
	}

	// db.DPrintf(db.TEST, "unmarshall %v msg %v\n", fm, m)

	iov, err := frame.ReadFramesN(rdr, fm.Fc.Nvec)
	if err != nil {
		return nil, err
	}

	fm.Msg = msg
	fm.Iov = iov
	return fm, nil
}

func WriteFcall1(wr io.Writer, c demux.CallI) *serr.Err {
	wrt := wr.(*bufio.Writer)
	fc := c.(*sessp.FcallMsg)

	msg, err := proto.Marshal(fc.Msg.(proto.Message))
	if err != nil {
		db.DFatalf("error encoding msg %v", err)
	}
	fc.Fc.Len = uint32(len(msg))
	fc.Fc.Nvec = uint32(len(fc.Iov))

	b, err := proto.Marshal(fc.Fc)
	if err != nil {
		db.DFatalf("error encoding fcall %v", err)
	}

	// db.DPrintf(db.TEST, "marshall %v %d\n", fc.Fc, len(b))

	if err := binary.Write(wrt, binary.LittleEndian, uint32(len(b)+4)); err != nil {
		db.DFatalf("error write %v", err)
	}

	if _, err := wrt.Write(b); err != nil {
		db.DFatalf("error write %v", err)
	}

	if _, err := wrt.Write(msg); err != nil {
		db.DFatalf("error write %v", err)
	}

	//this write will be copied (< 4096)
	for _, f := range fc.Iov {
		if err := frame.WriteFrame(wrt, f); err != nil {
			return err
		}
	}

	// db.DPrintf(db.TEST, "buf %d\n", wrt.Buffered())

	// writes all bytes
	if err := wrt.Flush(); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}

type netConn struct {
	conn net.Conn
}

func (nc *netConn) ServeRequest(req demux.CallI) (demux.CallI, *serr.Err) {
	var rep demux.CallI
	switch r := req.(type) {
	case *call:
		rep = &call{buf: r.buf}
	case *sessp.FcallMsg:
		rep = r
	}
	return rep, nil
}

func (nc *netConn) ReportError(err error) {
}

type TstateNet struct {
	*test.TstateMin
	srv  *netsrv.NetServer
	clnt *netclnt.NetClnt
	dmx  *demux.DemuxClnt
	rf   demux.ReadCallF
	wf   demux.WriteCallF
}

func (ts *TstateNet) NewConn(conn net.Conn) *demux.DemuxSrv {
	br := bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN)
	wr := bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN)
	nc := &netConn{conn}
	return demux.NewDemuxSrv(br, wr, ts.rf, ts.wf, nc)
}

func (ts *TstateNet) ReportError(err error) {
}

func newTstateNet(t *testing.T, rf demux.ReadCallF, wf demux.WriteCallF) *TstateNet {
	ts := &TstateNet{rf: rf, wf: wf}
	ts.TstateMin = test.NewTstateMin(t)

	ts.srv = netsrv.NewNetServer(ts.Pcfg, ts.Addr, ts)

	db.DPrintf(db.TEST, "srv %v\n", ts.srv.MyAddr())

	nc, err := netclnt.NewNetClnt(sp.ROOTREALM.String(), sp.Taddrs{ts.srv.MyAddr()})
	assert.Nil(t, err)
	br := bufio.NewReaderSize(nc.Conn(), sp.Conf.Conn.MSG_LEN)
	bw := bufio.NewWriterSize(nc.Conn(), sp.Conf.Conn.MSG_LEN)
	ts.dmx = demux.NewDemuxClnt(bw, br, rf, wf, ts)
	return ts
}

func TestNetClntPerfFrame(t *testing.T) {
	ts := newTstateNet(t, ReadCall, WriteCall)
	c := &call{buf: test.NewBuf(BUFSZ)}

	t0 := time.Now()
	n := TOTAL / BUFSZ
	for i := 0; i < n; i++ {
		d, err := ts.dmx.SendReceive(c)
		assert.Nil(t, err)
		call := d.(*call)
		assert.True(t, len(call.buf) == BUFSZ)
	}
	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms (%v us per iter, %d iter) tput %v\n", humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/BUFSZ), n, test.TputStr(TOTAL, ms))

	ts.srv.CloseListener()
}

func TestNetClntPerfFcall1(t *testing.T) {
	ts := newTstateNet(t, ReadFcall1, WriteFcall1)
	req := sp.NewTheartbeat(map[uint64]bool{uint64(1): true})
	fc := sessp.NewFcallMsg(req, sessp.IoVec{test.NewBuf(BUFSZ)}, 1, &seqno)

	t0 := time.Now()
	n := TOTAL / BUFSZ
	for i := 0; i < n; i++ {
		c, err := ts.dmx.SendReceive(fc)
		assert.Nil(t, err)
		fcm := c.(*sessp.FcallMsg)
		assert.True(t, len(fcm.Iov[0]) == BUFSZ)
	}
	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms (%v us per iter, %d iter) tput %v\n", humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/BUFSZ), n, test.TputStr(TOTAL, ms))

	ts.srv.CloseListener()
}

func TestNetClntPerfFcall(t *testing.T) {
	ts := newTstateNet(t, ReadFcall, WriteFcall)
	req := sp.NewTheartbeat(map[uint64]bool{uint64(1): true})
	fc := sessp.NewFcallMsg(req, sessp.IoVec{test.NewBuf(BUFSZ)}, 1, &seqno)

	t0 := time.Now()
	n := TOTAL / BUFSZ
	for i := 0; i < n; i++ {
		c, err := ts.dmx.SendReceive(fc)
		assert.Nil(t, err)
		fcm := c.(*sessp.FcallMsg)
		assert.True(t, len(fcm.Iov[0]) == BUFSZ)
	}
	tot := uint64(TOTAL)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "wrote %v bytes in %v ms (%v us per iter, %d iter) tput %v\n", humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/BUFSZ), n, test.TputStr(TOTAL, ms))

	ts.srv.CloseListener()
}

func testLocalPerf(t *testing.T, typ, arg string) {
	var socket net.Listener
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

	ch := make(chan bool)
	// Serve requests in another thread
	go func() {
		conn, err := socket.Accept()
		assert.Nil(t, err)
		tot := 0
		for {
			rb := make([]byte, BUFSZ)
			n, err := io.ReadFull(conn, rb)
			if err == io.EOF {
				db.DPrintf(db.TEST, "tot %d\n", tot)
				break
			}
			tot += n
			if n != len(rb) || err != nil {
				db.DFatalf("Err read: len %v err %v", n, err)
			}
			conn.Write(rb)
		}
		ch <- true

	}()

	time.Sleep(1 * time.Second)
	conn, err := net.Dial(typ, arg)
	assert.Nil(t, err)

	sz := sp.Tlength(TOTAL)
	buf := test.NewBuf(BUFSZ)
	t0 := time.Now()
	n := TOTAL / BUFSZ
	for i := 0; i < n; i++ {
		n, err := conn.Write(buf)
		assert.Nil(t, err)
		assert.Equal(t, BUFSZ, n)
		rb := make([]byte, BUFSZ)
		m, err := io.ReadFull(conn, rb)
		assert.Nil(t, err)
		assert.True(t, m == BUFSZ)
	}

	conn.Close()

	<-ch

	tot := uint64(sz)
	ms := time.Since(t0).Milliseconds()
	db.DPrintf(db.ALWAYS, "%v wrote %v bytes in %v ms (%v us per iter, %d iter) tput %v\n", typ, humanize.Bytes(tot), ms, (ms*1000)/(TOTAL/BUFSZ), n, test.TputStr(sz, ms))

	if typ == "unix" {
		err = os.Remove(arg)
		assert.True(t, err == nil || os.IsNotExist(err), "Err remove sock: %v", err)
	}

	socket.Close()
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
