package netclnt_test

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/netclnt"
	"sigmaos/netsrv"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
	"sigmaos/test"
)

const (
	BUFSZ = 100          // 64 * sp.KBYTE
	TOTAL = 1 * sp.MBYTE // 1000 * sp.MBYTE
)

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
	fcm := c.(*sessp.FcallMsg)
	// db.DPrintf(db.TEST, "ReadFcall %v\n", fcm)
	return &call{buf: fcm.Iov[0]}, err
}

func WriteFcall(wr io.Writer, c demux.CallI) *serr.Err {
	call := c.(*call)
	req := sp.NewTheartbeat(map[uint64]bool{uint64(1): true})
	fcm := sessp.NewFcallMsg(req, sessp.IoVec{call.buf}, 1, &seqno)
	pmfc := spcodec.NewPartMarshaledMsg(fcm)
	// db.DPrintf(db.TEST, "fcall %v\n", fcm)
	return spcodec.WriteCall(wr, pmfc)
}

type netConn struct {
	conn net.Conn
}

func (nc *netConn) ServeRequest(req demux.CallI) (demux.CallI, *serr.Err) {
	r := req.(*call)
	rep := &call{buf: r.buf}
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

func TestNetClntPerfFcall(t *testing.T) {
	ts := newTstateNet(t, ReadFcall, WriteFcall)
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
